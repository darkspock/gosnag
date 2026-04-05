package jira

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/darkspock/gosnag/internal/config"
	"github.com/darkspock/gosnag/internal/database/db"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type Handler struct {
	queries *db.Queries
	cfg     *config.Config
}

func NewHandler(queries *db.Queries, cfg *config.Config) *Handler {
	return &Handler{queries: queries, cfg: cfg}
}

// TestConnection tests the Jira connection for a project.
func (h *Handler) TestConnection(w http.ResponseWriter, r *http.Request) {
	projectID, err := uuid.Parse(chi.URLParam(r, "project_id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid project id")
		return
	}

	project, err := h.queries.GetProject(r.Context(), projectID)
	if err != nil {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}

	cfg := ConfigFromProject(project)
	if !cfg.IsConfigured() {
		writeError(w, http.StatusBadRequest, "Jira is not configured for this project")
		return
	}

	if err := TestConnection(cfg); err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// CreateTicket manually creates a Jira ticket from an issue.
func (h *Handler) CreateTicket(w http.ResponseWriter, r *http.Request) {
	projectID, err := uuid.Parse(chi.URLParam(r, "project_id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid project id")
		return
	}

	issueID, err := uuid.Parse(chi.URLParam(r, "issue_id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid issue id")
		return
	}

	project, err := h.queries.GetProject(r.Context(), projectID)
	if err != nil {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}

	cfg := ConfigFromProject(project)
	if !cfg.IsConfigured() {
		writeError(w, http.StatusBadRequest, "Jira is not configured for this project")
		return
	}

	issue, err := h.queries.GetIssue(r.Context(), issueID)
	if err != nil {
		writeError(w, http.StatusNotFound, "issue not found")
		return
	}

	if issue.ProjectID != projectID {
		writeError(w, http.StatusBadRequest, "issue does not belong to this project")
		return
	}

	if issue.JiraTicketKey.Valid {
		writeError(w, http.StatusConflict, "issue already has a Jira ticket: "+issue.JiraTicketKey.String)
		return
	}

	// Get latest stacktrace from most recent event
	stacktrace := h.getLatestStacktrace(r, projectID, issueID)

	baseURL := h.cfg.BaseURL
	summary := "[GoSnag] " + truncate(issue.Title, 200)
	description := BuildDescription(issue, baseURL, projectID.String(), stacktrace)

	result, err := CreateIssue(cfg, summary, description)
	if err != nil {
		writeError(w, http.StatusBadGateway, "failed to create Jira ticket: "+err.Error())
		return
	}

	// Store Jira reference on the issue
	_ = h.queries.UpdateIssueJiraTicket(r.Context(), db.UpdateIssueJiraTicketParams{
		ID:            issueID,
		JiraTicketKey: sql.NullString{String: result.Key, Valid: true},
		JiraTicketUrl: sql.NullString{String: result.URL, Valid: true},
	})

	writeJSON(w, http.StatusCreated, map[string]string{
		"key": result.Key,
		"url": result.URL,
	})
}

// ListRules lists all Jira rules for a project.
func (h *Handler) ListRules(w http.ResponseWriter, r *http.Request) {
	projectID, err := uuid.Parse(chi.URLParam(r, "project_id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid project id")
		return
	}

	rules, err := h.queries.ListJiraRules(r.Context(), projectID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list rules")
		return
	}
	writeJSON(w, http.StatusOK, rules)
}

type RuleRequest struct {
	Name         string `json:"name"`
	Enabled      bool   `json:"enabled"`
	LevelFilter  string `json:"level_filter"`
	MinEvents    int32  `json:"min_events"`
	MinUsers     int32  `json:"min_users"`
	TitlePattern string `json:"title_pattern"`
}

// CreateRule creates a new Jira auto-creation rule.
func (h *Handler) CreateRule(w http.ResponseWriter, r *http.Request) {
	projectID, err := uuid.Parse(chi.URLParam(r, "project_id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid project id")
		return
	}

	var req RuleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}

	rule, err := h.queries.CreateJiraRule(r.Context(), db.CreateJiraRuleParams{
		ProjectID:    projectID,
		Name:         req.Name,
		Enabled:      req.Enabled,
		LevelFilter:  req.LevelFilter,
		MinEvents:    req.MinEvents,
		MinUsers:     req.MinUsers,
		TitlePattern: req.TitlePattern,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create rule")
		return
	}

	writeJSON(w, http.StatusCreated, rule)
}

// UpdateRule updates a Jira rule.
func (h *Handler) UpdateRule(w http.ResponseWriter, r *http.Request) {
	projectID, err := uuid.Parse(chi.URLParam(r, "project_id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid project id")
		return
	}

	ruleID, err := uuid.Parse(chi.URLParam(r, "rule_id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid rule id")
		return
	}

	var req RuleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	rule, err := h.queries.UpdateJiraRule(r.Context(), db.UpdateJiraRuleParams{
		ID:           ruleID,
		ProjectID:    projectID,
		Name:         req.Name,
		Enabled:      req.Enabled,
		LevelFilter:  req.LevelFilter,
		MinEvents:    req.MinEvents,
		MinUsers:     req.MinUsers,
		TitlePattern: req.TitlePattern,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update rule")
		return
	}

	writeJSON(w, http.StatusOK, rule)
}

// DeleteRule deletes a Jira rule.
func (h *Handler) DeleteRule(w http.ResponseWriter, r *http.Request) {
	projectID, err := uuid.Parse(chi.URLParam(r, "project_id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid project id")
		return
	}

	ruleID, err := uuid.Parse(chi.URLParam(r, "rule_id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid rule id")
		return
	}

	err = h.queries.DeleteJiraRule(r.Context(), db.DeleteJiraRuleParams{
		ID:        ruleID,
		ProjectID: projectID,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete rule")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (h *Handler) getLatestStacktrace(r *http.Request, projectID, issueID uuid.UUID) string {
	events, err := h.queries.ListEventsByIssue(r.Context(), db.ListEventsByIssueParams{
		IssueID: issueID,
		Limit:   1,
		Offset:  0,
	})
	if err != nil || len(events) == 0 {
		return ""
	}

	// Try to extract stacktrace from event data
	var data map[string]any
	if err := json.Unmarshal(events[0].Data, &data); err != nil {
		return ""
	}

	// Check exception.values[0].stacktrace
	if exc, ok := data["exception"].(map[string]any); ok {
		if values, ok := exc["values"].([]any); ok && len(values) > 0 {
			if v, ok := values[0].(map[string]any); ok {
				if st, ok := v["stacktrace"].(map[string]any); ok {
					if frames, ok := st["frames"].([]any); ok {
						return formatFrames(frames)
					}
				}
			}
		}
	}

	return ""
}

func formatFrames(frames []any) string {
	start := 0
	if len(frames) > 10 {
		start = len(frames) - 10
	}

	var result string
	for i := start; i < len(frames); i++ {
		f, ok := frames[i].(map[string]any)
		if !ok {
			continue
		}
		file, _ := f["filename"].(string)
		lineno, _ := f["lineno"].(float64)
		fn, _ := f["function"].(string)
		if file != "" {
			result += fmt.Sprintf("  %s:%.0f in %s\n", file, lineno, fn)
		}
	}
	return result
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
