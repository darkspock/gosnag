package issue

import (
	"database/sql"
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/darkspock/gosnag/internal/auth"
	"github.com/darkspock/gosnag/internal/database/db"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type Handler struct {
	queries *db.Queries
	database *sql.DB
}

func NewHandler(queries *db.Queries, database ...*sql.DB) *Handler {
	h := &Handler{queries: queries}
	if len(database) > 0 {
		h.database = database[0]
	}
	return h
}

type UpdateIssueRequest struct {
	Status               string  `json:"status"`
	CooldownMinutes      *int    `json:"cooldown_minutes,omitempty"`
	ResolvedInRelease    *string `json:"resolved_in_release,omitempty"`
	SnoozeMinutes        *int    `json:"snooze_minutes,omitempty"`
	SnoozeEventThreshold *int    `json:"snooze_event_threshold,omitempty"`
}

type AssignIssueRequest struct {
	AssignedTo *string `json:"assigned_to"`
}

type IssueTag struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type IssueWithStats struct {
	db.Issue
	UserCount int32      `json:"user_count"`
	Trend     []int32    `json:"trend"`
	Tags      []IssueTag `json:"tags"`
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	projectID, err := uuid.Parse(chi.URLParam(r, "project_id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid project id")
		return
	}

	q := r.URL.Query()
	status := q.Get("status")
	level := q.Get("level")
	search := q.Get("search")
	tagFilter := q.Get("tag") // format: key:value
	limit, _ := strconv.ParseInt(q.Get("limit"), 10, 32)
	offset, _ := strconv.ParseInt(q.Get("offset"), 10, 32)
	todayOnly := q.Get("today") == "true"
	assignedAny := q.Get("assigned_any") == "true"

	if limit <= 0 || limit > 100 {
		limit = 50
	}

	// Handle "assigned to me" filter
	var assignedToUser uuid.NullUUID
	if q.Get("assigned_to") == "me" {
		if user := auth.GetUserFromContext(r.Context()); user != nil {
			assignedToUser = uuid.NullUUID{UUID: user.ID, Valid: true}
		}
	}

	issues, err := h.queries.ListIssuesByProject(r.Context(), db.ListIssuesByProjectParams{
		ProjectID:      projectID,
		Column2:        status,
		Limit:          int32(limit),
		Offset:         int32(offset),
		Column5:        todayOnly,
		Column6:        assignedAny,
		AssignedToUser: assignedToUser,
		LevelFilter:    level,
		Search:         search,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list issues")
		return
	}

	// When tag filter is present, expand results with tag-matched issues and deduplicate
	var tagAllowed map[uuid.UUID]bool
	if tagFilter != "" {
		parts := strings.SplitN(tagFilter, ":", 2)
		if len(parts) == 2 {
			tagIssueIDs, _ := h.queries.ListIssueIDsByTag(r.Context(), db.ListIssueIDsByTagParams{
				Key:   parts[0],
				Value: parts[1],
			})
			tagAllowed = make(map[uuid.UUID]bool, len(tagIssueIDs))
			for _, id := range tagIssueIDs {
				tagAllowed[id] = true
			}
			// Add tag-matched issues not in current page (they matched by tag, not title)
			existing := make(map[uuid.UUID]bool, len(issues))
			for _, iss := range issues {
				existing[iss.ID] = true
			}
			for _, id := range tagIssueIDs {
				if !existing[id] {
					if iss, err := h.queries.GetIssue(r.Context(), id); err == nil && iss.ProjectID == projectID {
						issues = append(issues, iss)
					}
				}
			}
		}
	}

	count, err := h.queries.CountIssuesByProject(r.Context(), db.CountIssuesByProjectParams{
		ProjectID:      projectID,
		Column2:        status,
		Column3:        todayOnly,
		Column4:        assignedAny,
		AssignedToUser: assignedToUser,
		LevelFilter:    level,
		Search:         search,
	})

	// Adjust count when tag filter adds extra results
	if tagAllowed != nil {
		count = int64(len(issues))
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to count issues")
		return
	}

	// Enrich with user counts and trends
	enriched := make([]IssueWithStats, len(issues))
	for i, iss := range issues {
		enriched[i] = IssueWithStats{Issue: iss}
	}

	if len(issues) > 0 {
		ids := make([]uuid.UUID, len(issues))
		for i, iss := range issues {
			ids[i] = iss.ID
		}

		var userCounts []db.GetUniqueUserCountsByIssuesRow
		var trendRows []db.GetEventTrendByIssuesRow
		var tagRows []db.IssueTag
		var wg sync.WaitGroup
		wg.Add(3)
		go func() {
			defer wg.Done()
			var err error
			userCounts, err = h.queries.GetUniqueUserCountsByIssues(r.Context(), ids)
			if err != nil {
				slog.Error("failed to get user counts", "error", err)
			}
		}()
		go func() {
			defer wg.Done()
			var err error
			trendRows, err = h.queries.GetEventTrendByIssues(r.Context(), ids)
			if err != nil {
				slog.Error("failed to get event trends", "error", err)
			}
		}()
		go func() {
			defer wg.Done()
			var err error
			tagRows, err = h.queries.ListTagsByIssueIDs(r.Context(), ids)
			if err != nil {
				slog.Error("failed to get issue tags", "error", err)
			}
		}()
		wg.Wait()

		// Map user counts
		ucMap := map[uuid.UUID]int32{}
		for _, uc := range userCounts {
			ucMap[uc.IssueID] = uc.UserCount
		}

		// Build trend arrays (24 hourly buckets)
		now := time.Now().UTC().Truncate(time.Hour)
		trendMap := map[uuid.UUID][]int32{}
		for _, tr := range trendRows {
			hoursAgo := int(now.Sub(tr.Bucket.UTC().Truncate(time.Hour)).Hours())
			if hoursAgo < 0 || hoursAgo >= 24 {
				continue
			}
			if trendMap[tr.IssueID] == nil {
				trendMap[tr.IssueID] = make([]int32, 24)
			}
			trendMap[tr.IssueID][23-hoursAgo] = tr.Count
		}

		// Map tags
		tagMap := map[uuid.UUID][]IssueTag{}
		for _, t := range tagRows {
			tagMap[t.IssueID] = append(tagMap[t.IssueID], IssueTag{Key: t.Key, Value: t.Value})
		}

		for i := range enriched {
			enriched[i].UserCount = ucMap[enriched[i].ID]
			if t, ok := trendMap[enriched[i].ID]; ok {
				enriched[i].Trend = t
			} else {
				enriched[i].Trend = make([]int32, 24)
			}
			enriched[i].Tags = tagMap[enriched[i].ID]
			if enriched[i].Tags == nil {
				enriched[i].Tags = []IssueTag{}
			}
		}
	}

	// Convert to JSON-safe format
	safeIssues := make([]map[string]any, len(enriched))
	for i, e := range enriched {
		m := issueJSON(e.Issue)
		m["user_count"] = e.UserCount
		m["trend"] = e.Trend
		m["tags"] = e.Tags
		safeIssues[i] = m
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"issues": safeIssues,
		"total":  count,
		"limit":  limit,
		"offset": offset,
	})
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	issue, ok := h.getIssueScoped(w, r)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, issueJSON(issue))
}

func (h *Handler) UpdateStatus(w http.ResponseWriter, r *http.Request) {
	currentIssue, ok := h.getIssueScoped(w, r)
	if !ok {
		return
	}
	issueID := currentIssue.ID

	var req UpdateIssueRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	switch req.Status {
	case "open", "resolved", "reopened", "ignored", "snoozed":
	default:
		writeError(w, http.StatusBadRequest, "invalid status")
		return
	}

	params := db.UpdateIssueStatusParams{
		ID:     issueID,
		Status: req.Status,
	}

	if req.Status == "resolved" {
		now := time.Now()
		params.ResolvedAt = sql.NullTime{Time: now, Valid: true}

		cooldownMinutes := req.CooldownMinutes
		if cooldownMinutes == nil {
			project, err := h.queries.GetProject(r.Context(), currentIssue.ProjectID)
			if err != nil {
				writeError(w, http.StatusInternalServerError, "failed to get project")
				return
			}
			cooldownMinutes = resolveCooldownMinutes(project.DefaultCooldownMinutes, nil)
		}

		if cooldownMinutes != nil && *cooldownMinutes > 0 {
			cooldownEnd := now.Add(time.Duration(*cooldownMinutes) * time.Minute)
			params.CooldownUntil = sql.NullTime{Time: cooldownEnd, Valid: true}
		}

		if req.ResolvedInRelease != nil {
			params.ResolvedInRelease = sql.NullString{String: *req.ResolvedInRelease, Valid: true}
		}
	}

	if req.Status == "snoozed" {
		if req.SnoozeMinutes != nil && *req.SnoozeMinutes > 0 {
			snoozeEnd := time.Now().Add(time.Duration(*req.SnoozeMinutes) * time.Minute)
			params.SnoozeUntil = sql.NullTime{Time: snoozeEnd, Valid: true}
		}

		if req.SnoozeEventThreshold != nil && *req.SnoozeEventThreshold > 0 {
			params.SnoozeEventThreshold = sql.NullInt32{Int32: int32(*req.SnoozeEventThreshold), Valid: true}
			params.SnoozeEventsAtStart = currentIssue.EventCount
		}
	}

	if req.Status == "open" || req.Status == "reopened" {
		params.ResolvedAt = sql.NullTime{Valid: false}
		params.CooldownUntil = sql.NullTime{Valid: false}
		params.ResolvedInRelease = sql.NullString{Valid: false}
		params.SnoozeUntil = sql.NullTime{Valid: false}
		params.SnoozeEventThreshold = sql.NullInt32{Valid: false}
		params.SnoozeEventsAtStart = 0
	}

	issue, err := h.queries.UpdateIssueStatus(r.Context(), params)
	if err != nil {
		if err == sql.ErrNoRows {
			writeError(w, http.StatusNotFound, "issue not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to update issue")
		return
	}

	writeJSON(w, http.StatusOK, issueJSON(issue))
}

func (h *Handler) Assign(w http.ResponseWriter, r *http.Request) {
	scoped, ok := h.getIssueScoped(w, r)
	if !ok {
		return
	}
	issueID := scoped.ID

	var req AssignIssueRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	var assignedTo uuid.NullUUID
	if req.AssignedTo != nil && *req.AssignedTo != "" {
		uid, err := uuid.Parse(*req.AssignedTo)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid user id")
			return
		}
		assignedTo = uuid.NullUUID{UUID: uid, Valid: true}
	}

	issue, err := h.queries.AssignIssue(r.Context(), db.AssignIssueParams{
		ID:         issueID,
		AssignedTo: assignedTo,
	})
	if err != nil {
		if err == sql.ErrNoRows {
			writeError(w, http.StatusNotFound, "issue not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to assign issue")
		return
	}

	writeJSON(w, http.StatusOK, issueJSON(issue))
}

func (h *Handler) BulkDelete(w http.ResponseWriter, r *http.Request) {
	projectID, err := uuid.Parse(chi.URLParam(r, "project_id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid project id")
		return
	}

	var req struct {
		IDs []string `json:"ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if len(req.IDs) == 0 {
		writeError(w, http.StatusBadRequest, "no ids provided")
		return
	}

	ids := make([]uuid.UUID, 0, len(req.IDs))
	for _, id := range req.IDs {
		uid, err := uuid.Parse(id)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid issue id: "+id)
			return
		}
		ids = append(ids, uid)
	}

	result, err := h.queries.DeleteIssues(r.Context(), db.DeleteIssuesParams{
		Ids:       ids,
		ProjectID: projectID,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete issues")
		return
	}

	deleted, _ := result.RowsAffected()
	writeJSON(w, http.StatusOK, map[string]any{"deleted": deleted})
}

func (h *Handler) Counts(w http.ResponseWriter, r *http.Request) {
	projectID, err := uuid.Parse(chi.URLParam(r, "project_id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid project id")
		return
	}

	ctx := r.Context()
	levelFilter := r.URL.Query().Get("level")

	statusCounts, err := h.queries.GetIssueCountsByStatus(ctx, db.GetIssueCountsByStatusParams{
		ProjectID: projectID,
		Column2:   levelFilter,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get counts")
		return
	}

	today, _ := h.queries.CountIssuesToday(ctx, db.CountIssuesTodayParams{
		ProjectID: projectID,
		Column2:   levelFilter,
	})
	assignedAny, _ := h.queries.CountIssuesAssigned(ctx, db.CountIssuesAssignedParams{
		ProjectID: projectID,
		Column2:   levelFilter,
	})

	assignedToMe := int32(0)
	if user := auth.GetUserFromContext(ctx); user != nil {
		assignedToMe, _ = h.queries.CountIssuesAssignedToUser(ctx, db.CountIssuesAssignedToUserParams{
			ProjectID:  projectID,
			AssignedTo: uuid.NullUUID{UUID: user.ID, Valid: true},
			Column3:    levelFilter,
		})
	}

	byStatus := map[string]int32{}
	total := int32(0)
	for _, sc := range statusCounts {
		byStatus[sc.Status] = sc.Count
		total += sc.Count
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"total":          total,
		"by_status":      byStatus,
		"today":          today,
		"assigned_to_me": assignedToMe,
		"assigned_any":   assignedAny,
	})
}

func (h *Handler) ListEvents(w http.ResponseWriter, r *http.Request) {
	scoped, ok := h.getIssueScoped(w, r)
	if !ok {
		return
	}
	issueID := scoped.ID

	limit, _ := strconv.ParseInt(r.URL.Query().Get("limit"), 10, 32)
	offset, _ := strconv.ParseInt(r.URL.Query().Get("offset"), 10, 32)

	if limit <= 0 || limit > 100 {
		limit = 50
	}

	events, err := h.queries.ListEventsByIssue(r.Context(), db.ListEventsByIssueParams{
		IssueID: issueID,
		Limit:   int32(limit),
		Offset:  int32(offset),
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list events")
		return
	}

	count, err := h.queries.CountEventsByIssue(r.Context(), issueID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to count events")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"events": events,
		"total":  count,
		"limit":  limit,
		"offset": offset,
	})
}

// getIssueScoped loads an issue by ID and verifies it belongs to the project in the URL.
func (h *Handler) getIssueScoped(w http.ResponseWriter, r *http.Request) (db.Issue, bool) {
	projectID, err := uuid.Parse(chi.URLParam(r, "project_id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid project id")
		return db.Issue{}, false
	}
	issueID, err := uuid.Parse(chi.URLParam(r, "issue_id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid issue id")
		return db.Issue{}, false
	}
	issue, err := h.queries.GetIssue(r.Context(), issueID)
	if err != nil {
		if err == sql.ErrNoRows {
			writeError(w, http.StatusNotFound, "issue not found")
		} else {
			writeError(w, http.StatusInternalServerError, "failed to get issue")
		}
		return db.Issue{}, false
	}
	if issue.ProjectID != projectID {
		writeError(w, http.StatusNotFound, "issue not found")
		return db.Issue{}, false
	}
	return issue, true
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// issueJSON converts a db.Issue to a JSON-safe map, converting sql.Null* to nil.
func issueJSON(i db.Issue) map[string]any {
	m := map[string]any{
		"id":                    i.ID,
		"project_id":           i.ProjectID,
		"title":                i.Title,
		"fingerprint":          i.Fingerprint,
		"status":               i.Status,
		"level":                i.Level,
		"platform":             i.Platform,
		"first_seen":           i.FirstSeen,
		"last_seen":            i.LastSeen,
		"event_count":          i.EventCount,
		"assigned_to":          nullUUID(i.AssignedTo),
		"resolved_at":          nullTime(i.ResolvedAt),
		"cooldown_until":       nullTime(i.CooldownUntil),
		"resolved_in_release":  nullString(i.ResolvedInRelease),
		"created_at":           i.CreatedAt,
		"updated_at":           i.UpdatedAt,
		"snooze_until":         nullTime(i.SnoozeUntil),
		"snooze_event_threshold": nullInt32(i.SnoozeEventThreshold),
		"snooze_events_at_start": i.SnoozeEventsAtStart,
		"jira_ticket_key":      nullString(i.JiraTicketKey),
		"jira_ticket_url":      nullString(i.JiraTicketUrl),
		"priority":             i.Priority,
	}
	return m
}

func nullString(ns sql.NullString) any {
	if ns.Valid { return ns.String }
	return nil
}
func nullTime(nt sql.NullTime) any {
	if nt.Valid { return nt.Time }
	return nil
}
func nullUUID(nu uuid.NullUUID) any {
	if nu.Valid { return nu.UUID }
	return nil
}
func nullInt32(ni sql.NullInt32) any {
	if ni.Valid { return ni.Int32 }
	return nil
}

func resolveCooldownMinutes(projectDefault int32, requested *int) *int {
	if requested != nil {
		return requested
	}
	if projectDefault <= 0 {
		return nil
	}
	minutes := int(projectDefault)
	return &minutes
}
