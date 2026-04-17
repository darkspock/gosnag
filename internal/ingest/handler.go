package ingest

import (
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/darkspock/gosnag/internal/activity"
	"github.com/darkspock/gosnag/internal/database/db"
	"github.com/darkspock/gosnag/internal/routegroup"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type Handler struct {
	queries     *db.Queries
	alertFn     func(projectID uuid.UUID, issue db.Issue, isNew bool)
	postEventFn func(projectID uuid.UUID, issue db.Issue, eventData json.RawMessage)
}

func NewHandler(queries *db.Queries, alertFn func(projectID uuid.UUID, issue db.Issue, isNew bool), postEventFn func(projectID uuid.UUID, issue db.Issue, eventData json.RawMessage)) *Handler {
	return &Handler{queries: queries, alertFn: alertFn, postEventFn: postEventFn}
}

// Store handles POST /api/{project_id}/store/ (legacy Sentry endpoint)
func (h *Handler) Store(w http.ResponseWriter, r *http.Request) {
	projectID, _, err := h.authenticate(r)
	if err != nil {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}

	project, err := h.queries.GetProject(r.Context(), projectID)
	if err != nil {
		slog.Error("failed to load project", "error", err, "project_id", projectID)
		http.Error(w, `{"error":"project not found"}`, http.StatusNotFound)
		return
	}

	body, err := readBody(r)
	if err != nil {
		http.Error(w, `{"error":"bad request"}`, http.StatusBadRequest)
		return
	}

	event, err := ParseEvent(body)
	if err != nil {
		slog.Warn("failed to parse event", "error", err)
		http.Error(w, `{"error":"invalid event"}`, http.StatusBadRequest)
		return
	}

	h.processEvent(r, project, event)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"id": event.EventID})
}

// Envelope handles POST /api/{project_id}/envelope/ (modern Sentry endpoint)
func (h *Handler) Envelope(w http.ResponseWriter, r *http.Request) {
	projectID, _, err := h.authenticate(r)
	if err != nil {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}

	project, err := h.queries.GetProject(r.Context(), projectID)
	if err != nil {
		slog.Error("failed to load project", "error", err, "project_id", projectID)
		http.Error(w, `{"error":"project not found"}`, http.StatusNotFound)
		return
	}

	header, items, err := ParseEnvelope(r)
	if err != nil {
		slog.Warn("failed to parse envelope", "error", err)
		http.Error(w, `{"error":"invalid envelope"}`, http.StatusBadRequest)
		return
	}

	eventID := header.EventID

	for _, item := range items {
		switch item.Header.Type {
		case "event":
			event, err := ParseEvent(item.Payload)
			if err != nil {
				slog.Warn("failed to parse event item", "error", err)
				continue
			}
			if event.EventID == "" {
				event.EventID = eventID
			}
			h.processEvent(r, project, event)
			eventID = event.EventID

		case "transaction":
			// Out of scope - ignore silently
			continue

		case "session", "sessions", "client_report":
			// Ignore silently
			continue

		default:
			slog.Debug("ignoring envelope item", "type", item.Header.Type)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"id": eventID})
}

func (h *Handler) authenticate(r *http.Request) (uuid.UUID, db.ProjectKey, error) {
	projectIDStr := extractProjectID(r)

	publicKey := ExtractPublicKey(r)
	if publicKey == "" {
		return uuid.Nil, db.ProjectKey{}, errUnauthorized
	}

	key, err := h.queries.GetProjectKeyByPublic(r.Context(), publicKey)
	if err != nil {
		return uuid.Nil, db.ProjectKey{}, errUnauthorized
	}

	// Try UUID first, then numeric ID
	if projectID, err := uuid.Parse(projectIDStr); err == nil {
		if key.ProjectID != projectID {
			return uuid.Nil, db.ProjectKey{}, errUnauthorized
		}
		return projectID, key, nil
	}

	// Try numeric ID (for Python SDK and others that require numeric project IDs)
	if numericID, err := strconv.Atoi(projectIDStr); err == nil {
		project, err := h.queries.GetProjectByNumericID(r.Context(), int32(numericID))
		if err != nil {
			return uuid.Nil, db.ProjectKey{}, errUnauthorized
		}
		if key.ProjectID != project.ID {
			return uuid.Nil, db.ProjectKey{}, errUnauthorized
		}
		return project.ID, key, nil
	}

	return uuid.Nil, db.ProjectKey{}, errUnauthorized
}

func isInformationalLevel(level string) bool {
	return level == "info" || level == "debug"
}

type issueSettings struct {
	WarningAsError      bool
	MaxInfoIssues       int32
	ErrorGroupingMode   string
	WarningGroupingMode string
	InfoGroupingMode    string
}

func normalizeGroupingMode(mode string) string {
	switch mode {
	case "by_url", "by_file":
		return mode
	default:
		return "normal"
	}
}

func loadIssueSettings(ctx context.Context, queries *db.Queries, project db.Project) issueSettings {
	settings := issueSettings{
		WarningAsError:      project.WarningAsError,
		MaxInfoIssues:       project.MaxInfoIssues,
		ErrorGroupingMode:   normalizeGroupingMode(project.InfoGroupingMode),
		WarningGroupingMode: normalizeGroupingMode(project.InfoGroupingMode),
		InfoGroupingMode:    normalizeGroupingMode(project.InfoGroupingMode),
	}
	if queries == nil {
		return settings
	}
	if err := queries.RawDB().QueryRowContext(ctx, `
		SELECT warning_as_error, max_info_issues, error_grouping_mode, warning_grouping_mode, info_grouping_mode
		FROM project_issue_settings
		WHERE project_id = $1
	`, project.ID).Scan(
		&settings.WarningAsError,
		&settings.MaxInfoIssues,
		&settings.ErrorGroupingMode,
		&settings.WarningGroupingMode,
		&settings.InfoGroupingMode,
	); err != nil {
		if err != sql.ErrNoRows {
			slog.Warn("failed to load issue settings", "project_id", project.ID, "error", err)
		}
		return settings
	}
	settings.ErrorGroupingMode = normalizeGroupingMode(settings.ErrorGroupingMode)
	settings.WarningGroupingMode = normalizeGroupingMode(settings.WarningGroupingMode)
	settings.InfoGroupingMode = normalizeGroupingMode(settings.InfoGroupingMode)
	return settings
}

func groupingModeForLevel(level string, settings issueSettings) string {
	switch level {
	case "error", "fatal":
		return settings.ErrorGroupingMode
	case "warning":
		return settings.WarningGroupingMode
	default:
		return settings.InfoGroupingMode
	}
}

func canonicalURLGroupingHint(event *SentryEvent, projectID uuid.UUID, groupingMode string, queries *db.Queries) (groupingHint, bool) {
	hint, ok := event.URLGroupingHint()
	if !ok || groupingMode != "by_url" || queries == nil {
		return hint, ok
	}

	method, currentPath := event.requestMethodAndPath()
	if currentPath == "" {
		return hint, ok
	}
	rule, matched, err := routegroup.FindCanonicalRoute(context.Background(), queries, projectID, method, currentPath)
	if err != nil || !matched || rule.CanonicalPath == "" || rule.CanonicalPath == currentPath {
		return hint, ok
	}
	canonicalPath := rule.CanonicalPath

	culprit := canonicalPath
	fingerprintKey := "info:url|" + canonicalPath
	if method != "" {
		culprit = method + " " + canonicalPath
		fingerprintKey = "info:url|" + method + "|" + canonicalPath
	}

	slog.Debug("route grouping matched",
		"project_id", projectID,
		"method", method,
		"raw_url", currentPath,
		"normalized_url", canonicalPath,
		"rule_id", rule.ID,
		"rule_source", rule.Source,
		"confidence", rule.Confidence,
		"culprit", culprit,
	)

	return groupingHint{
		FingerprintKey: fingerprintKey,
		Title:          event.groupingTitleFromKey(culprit),
		Culprit:        culprit,
	}, true
}

func resolveIssueGrouping(projectID uuid.UUID, event *SentryEvent, settings issueSettings, queries *db.Queries) (string, string, string) {
	fingerprint := event.ComputeFingerprint()
	title := event.IssueTitle()
	culprit := event.Culprit()
	groupingMode := groupingModeForLevel(normalizeIssueLevel(event.Level, settings.WarningAsError), settings)

	if groupingMode == "by_url" && !event.HasExceptionStacktrace() {
		if hint, ok := canonicalURLGroupingHint(event, projectID, groupingMode, queries); ok {
			return hashFingerprintKey(hint.FingerprintKey), title, hint.Culprit
		}
	}

	if !isInformationalLevel(event.Level) {
		return fingerprint, title, culprit
	}

	switch groupingMode {
	case "by_url":
		if hint, ok := canonicalURLGroupingHint(event, projectID, groupingMode, queries); ok {
			return hashFingerprintKey(hint.FingerprintKey), hint.Title, hint.Culprit
		}
	case "by_file":
		if hint, ok := event.FileGroupingHint(); ok {
			return hashFingerprintKey(hint.FingerprintKey), hint.Title, hint.Culprit
		}
	}

	return fingerprint, title, culprit
}

func (h *Handler) processEvent(r *http.Request, project db.Project, event *SentryEvent) {
	ctx := r.Context()
	projectID := project.ID
	settings := loadIssueSettings(ctx, h.queries, project)
	fingerprint, issueTitle, culprit := resolveIssueGrouping(projectID, event, settings, h.queries)
	now := time.Now()
	issueLevel := normalizeIssueLevel(event.Level, settings.WarningAsError)

	// Check if this fingerprint is an alias for a merged issue
	if alias, err := h.queries.GetIssueAlias(ctx, db.GetIssueAliasParams{
		ProjectID:   projectID,
		Fingerprint: fingerprint,
	}); err == nil {
		// Redirect to the primary issue's fingerprint
		primaryIssue, err := h.queries.GetIssue(ctx, alias.PrimaryIssueID)
		if err == nil {
			fingerprint = primaryIssue.Fingerprint
		}
	}

	if isInformationalLevel(event.Level) && settings.MaxInfoIssues > 0 {
		_, err := h.queries.GetIssueByFingerprint(ctx, db.GetIssueByFingerprintParams{
			ProjectID:   projectID,
			Fingerprint: fingerprint,
		})
		if err == sql.ErrNoRows {
			reachedLimit, limitErr := h.queries.HasReachedInfoIssueLimit(ctx, db.HasReachedInfoIssueLimitParams{
				ProjectID:     projectID,
				MaxInfoIssues: settings.MaxInfoIssues,
			})
			if limitErr != nil {
				slog.Error("failed to check informational issue limit", "error", limitErr, "project_id", projectID)
				return
			}
			if reachedLimit {
				slog.Warn("dropping new informational issue because max_info_issues limit was reached", "project_id", projectID, "fingerprint", fingerprint, "limit", project.MaxInfoIssues)
				return
			}
		} else if err != nil {
			slog.Error("failed to check existing issue by fingerprint", "error", err, "project_id", projectID)
			return
		}
	}

	// Upsert issue (create or update event count)
	issue, err := h.queries.UpsertIssue(ctx, db.UpsertIssueParams{
		ProjectID:    projectID,
		Title:        issueTitle,
		Fingerprint:  fingerprint,
		Level:        issueLevel,
		Platform:     event.Platform,
		FirstSeen:    now,
		Culprit:      culprit,
		FirstRelease: event.Release,
	})
	if err != nil {
		slog.Error("failed to upsert issue", "error", err)
		return
	}

	isNew := issue.EventCount == 1
	reopened := false

	if isNew {
		activity.Record(ctx, h.queries, issue.ID, nil, nil, "first_seen", "", "open", nil)
	}

	// Check if we should reopen a resolved issue
	if !isNew && issue.Status == "resolved" {
		shouldReopen := false

		if issue.CooldownUntil.Valid && now.After(issue.CooldownUntil.Time) {
			shouldReopen = true
		}

		if issue.ResolvedInRelease.Valid && event.Release != "" && event.Release != issue.ResolvedInRelease.String {
			shouldReopen = true
		}

		if !issue.CooldownUntil.Valid && !issue.ResolvedInRelease.Valid {
			shouldReopen = true
		}

		if shouldReopen {
			issue, err = h.queries.UpdateIssueStatus(ctx, db.UpdateIssueStatusParams{
				ID:     issue.ID,
				Status: "reopened",
			})
			if err != nil {
				slog.Error("failed to reopen issue", "error", err)
			} else {
				reopened = true
				activity.Record(ctx, h.queries, issue.ID, nil, nil, "auto_reopened", "resolved", "reopened", nil)
			}
		}
	}

	// Check if a snoozed issue should wake up (by event threshold)
	if !isNew && issue.Status == "snoozed" && issue.SnoozeEventThreshold.Valid {
		eventsSinceSnooze := issue.EventCount - issue.SnoozeEventsAtStart
		if eventsSinceSnooze >= issue.SnoozeEventThreshold.Int32 {
			issue, err = h.queries.UpdateIssueStatus(ctx, db.UpdateIssueStatusParams{
				ID:     issue.ID,
				Status: "reopened",
			})
			if err != nil {
				slog.Error("failed to unsnooze issue", "error", err)
			} else {
				reopened = true
				activity.Record(ctx, h.queries, issue.ID, nil, nil, "auto_unsnoozed", "snoozed", "reopened", nil)
			}
		}
	}

	// Check per-issue event cap (0 = unlimited)
	maxEvents := project.MaxEventsPerIssue
	if maxEvents > 0 && issue.EventCount > maxEvents {
		return
	}

	// Store the event first, so velocity queries include it
	rawData, _ := json.Marshal(event.Raw)

	eventID := event.EventID
	if eventID == "" {
		eventID = uuid.New().String()
	}

	userIdentifier := extractUserIdentifier(event.User)

	_, err = h.queries.CreateEvent(ctx, db.CreateEventParams{
		IssueID:        issue.ID,
		ProjectID:      projectID,
		EventID:        eventID,
		Timestamp:      now,
		Platform:       event.Platform,
		Level:          event.Level,
		Message:        event.Title(),
		Release:        event.Release,
		Environment:    event.Environment,
		ServerName:     event.ServerName,
		Data:           rawData,
		UserIdentifier: userIdentifier,
	})
	if err != nil {
		slog.Error("failed to create event", "error", err)
		return
	}

	// Alert after event is persisted (so velocity queries include this event)
	if h.alertFn != nil {
		if isNew {
			h.alertFn(projectID, issue, true)
		} else if reopened {
			h.alertFn(projectID, issue, false)
		}
	}

	// Always run post-event hooks (priority recalc, auto-tags, etc.)
	if h.postEventFn != nil {
		h.postEventFn(projectID, issue, rawData)
	}
}

func extractProjectID(r *http.Request) string {
	if id := chi.URLParam(r, "project_id"); id != "" {
		return id
	}
	return r.PathValue("project_id")
}

var errUnauthorized = &httpError{status: http.StatusUnauthorized, msg: "unauthorized"}

type httpError struct {
	status int
	msg    string
}

func (e *httpError) Error() string { return e.msg }

func normalizeIssueLevel(level string, warningAsError bool) string {
	if warningAsError && level == "warning" {
		return "error"
	}
	return level
}

func extractUserIdentifier(user map[string]any) string {
	if user == nil {
		return ""
	}
	for _, key := range []string{"id", "email", "username", "ip_address"} {
		if v, ok := user[key]; ok {
			if s, ok := v.(string); ok && s != "" {
				return s
			}
		}
	}
	return ""
}

// SecurityReport handles CSP and other security report endpoints
func (h *Handler) SecurityReport(w http.ResponseWriter, r *http.Request) {
	// Read and discard - we accept but don't process security reports
	io.ReadAll(io.LimitReader(r.Body, 1024*1024))
	r.Body.Close()
	w.WriteHeader(http.StatusOK)
}
