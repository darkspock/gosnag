package ingest

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/darkspock/gosnag/internal/database/db"
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
	projectID, err := uuid.Parse(projectIDStr)
	if err != nil {
		return uuid.Nil, db.ProjectKey{}, err
	}

	publicKey := ExtractPublicKey(r)
	if publicKey == "" {
		return uuid.Nil, db.ProjectKey{}, errUnauthorized
	}

	key, err := h.queries.GetProjectKeyByPublic(r.Context(), publicKey)
	if err != nil {
		return uuid.Nil, db.ProjectKey{}, errUnauthorized
	}

	if key.ProjectID != projectID {
		return uuid.Nil, db.ProjectKey{}, errUnauthorized
	}

	return projectID, key, nil
}

func (h *Handler) processEvent(r *http.Request, project db.Project, event *SentryEvent) {
	ctx := r.Context()
	projectID := project.ID
	fingerprint := event.ComputeFingerprint()
	now := time.Now()
	issueLevel := normalizeIssueLevel(event.Level, project.WarningAsError)

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

	// Upsert issue (create or update event count)
	issue, err := h.queries.UpsertIssue(ctx, db.UpsertIssueParams{
		ProjectID:   projectID,
		Title:       event.Title(),
		Fingerprint: fingerprint,
		Level:       issueLevel,
		Platform:    event.Platform,
		FirstSeen:   now,
	})
	if err != nil {
		slog.Error("failed to upsert issue", "error", err)
		return
	}

	isNew := issue.EventCount == 1
	reopened := false

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
