package alert

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/darkspock/gosnag/internal/ai"
	"github.com/darkspock/gosnag/internal/database/db"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/sqlc-dev/pqtype"
)

type Handler struct {
	queries   *db.Queries
	aiService *ai.Service
}

func NewHandler(queries *db.Queries, aiService *ai.Service) *Handler {
	return &Handler{queries: queries, aiService: aiService}
}

type CreateAlertRequest struct {
	AlertType      string          `json:"alert_type"`
	Config         json.RawMessage `json:"config"`
	Enabled        bool            `json:"enabled"`
	LevelFilter    string          `json:"level_filter"`
	TitlePattern   string          `json:"title_pattern"`
	MinEvents      int32           `json:"min_events"`
	MinVelocity1h  int32           `json:"min_velocity_1h"`
	ExcludePattern string          `json:"exclude_pattern"`
	Conditions     json.RawMessage `json:"conditions,omitempty"`
}

type UpdateAlertRequest struct {
	Config         json.RawMessage `json:"config"`
	Enabled        bool            `json:"enabled"`
	LevelFilter    string          `json:"level_filter"`
	TitlePattern   string          `json:"title_pattern"`
	MinEvents      int32           `json:"min_events"`
	MinVelocity1h  int32           `json:"min_velocity_1h"`
	ExcludePattern string          `json:"exclude_pattern"`
	Conditions     json.RawMessage `json:"conditions,omitempty"`
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	projectID, err := uuid.Parse(chi.URLParam(r, "project_id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid project id")
		return
	}

	configs, err := h.queries.ListAlertConfigs(r.Context(), projectID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list alert configs")
		return
	}

	writeJSON(w, http.StatusOK, toSafeAlerts(configs))
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	projectID, err := uuid.Parse(chi.URLParam(r, "project_id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid project id")
		return
	}

	var req CreateAlertRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	switch req.AlertType {
	case "email", "slack":
	default:
		writeError(w, http.StatusBadRequest, "alert_type must be 'email' or 'slack'")
		return
	}

	if err := validateAlertConfig(req.AlertType, req.Config); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	config, err := h.queries.CreateAlertConfig(r.Context(), db.CreateAlertConfigParams{
		ProjectID:      projectID,
		AlertType:      req.AlertType,
		Config:         req.Config,
		Enabled:        req.Enabled,
		LevelFilter:    req.LevelFilter,
		TitlePattern:   req.TitlePattern,
		MinEvents:      req.MinEvents,
		MinVelocity1h:  req.MinVelocity1h,
		ExcludePattern: req.ExcludePattern,
		Conditions:     toNullJSON(req.Conditions),
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create alert config")
		return
	}

	writeJSON(w, http.StatusCreated, toSafeAlert(config))
}

func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	projectID, err := uuid.Parse(chi.URLParam(r, "project_id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid project id")
		return
	}
	alertID, err := uuid.Parse(chi.URLParam(r, "alert_id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid alert id")
		return
	}

	var req UpdateAlertRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Look up existing to get alert_type for config validation
	existing, err := h.queries.ListAlertConfigs(r.Context(), projectID)
	if err == nil {
		for _, a := range existing {
			if a.ID == alertID {
				if err := validateAlertConfig(a.AlertType, req.Config); err != nil {
					writeError(w, http.StatusBadRequest, err.Error())
					return
				}
				break
			}
		}
	}

	// For Slack alerts, preserve existing webhook_url if not provided in the request
	// (the API redacts webhook_url on read, so clients may send updates without it)
	finalConfig := req.Config
	for _, a := range existing {
		if a.ID == alertID && a.AlertType == "slack" {
			finalConfig = preserveSlackWebhook(a.Config, req.Config)
			break
		}
	}

	config, err := h.queries.UpdateAlertConfig(r.Context(), db.UpdateAlertConfigParams{
		ID:             alertID,
		ProjectID:      projectID,
		Config:         finalConfig,
		Enabled:        req.Enabled,
		LevelFilter:    req.LevelFilter,
		TitlePattern:   req.TitlePattern,
		MinEvents:      req.MinEvents,
		MinVelocity1h:  req.MinVelocity1h,
		ExcludePattern: req.ExcludePattern,
		Conditions:     toNullJSON(req.Conditions),
	})
	if err != nil {
		if err == sql.ErrNoRows {
			writeError(w, http.StatusNotFound, "alert config not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to update alert config")
		return
	}

	writeJSON(w, http.StatusOK, toSafeAlert(config))
}

func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	projectID, err := uuid.Parse(chi.URLParam(r, "project_id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid project id")
		return
	}
	alertID, err := uuid.Parse(chi.URLParam(r, "alert_id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid alert id")
		return
	}

	if err := h.queries.DeleteAlertConfig(r.Context(), db.DeleteAlertConfigParams{ID: alertID, ProjectID: projectID}); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete alert config")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// SuggestAlerts handles POST /projects/{project_id}/alerts/suggest
// Conversational AI assistant for creating alert configurations (uses thinking model).
func (h *Handler) SuggestAlerts(w http.ResponseWriter, r *http.Request) {
	projectID, err := uuid.Parse(chi.URLParam(r, "project_id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid project id")
		return
	}
	if h.aiService == nil {
		writeError(w, http.StatusServiceUnavailable, "AI not configured")
		return
	}

	var req struct {
		IncludeIssues bool         `json:"include_issues"`
		Messages      []ai.Message `json:"messages"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if len(req.Messages) == 0 {
		writeError(w, http.StatusBadRequest, "messages required")
		return
	}

	// Build context: existing alerts
	alerts, _ := h.queries.ListAlertConfigs(r.Context(), projectID)
	var alertsCtx strings.Builder
	if len(alerts) > 0 {
		alertsCtx.WriteString("## Existing Alerts\n")
		for _, a := range alerts {
			status := "enabled"
			if !a.Enabled {
				status = "disabled"
			}
			alertsCtx.WriteString(fmt.Sprintf("- %s alert (%s)", a.AlertType, status))
			if a.LevelFilter != "" {
				alertsCtx.WriteString(fmt.Sprintf(", level: %s", a.LevelFilter))
			}
			if a.TitlePattern != "" {
				alertsCtx.WriteString(fmt.Sprintf(", pattern: %s", a.TitlePattern))
			}
			alertsCtx.WriteString("\n")
		}
	} else {
		alertsCtx.WriteString("## Existing Alerts\nNone configured yet.\n")
	}

	// Optionally include recent issues
	var issuesCtx strings.Builder
	if req.IncludeIssues {
		issues, err := h.queries.ListRecentIssuesForContext(r.Context(), projectID)
		if err == nil && len(issues) > 0 {
			issuesCtx.WriteString("\n## Recent Issues (last 20)\n")
			for _, iss := range issues {
				issuesCtx.WriteString(fmt.Sprintf("- [%s] %s (platform: %s, events: %d, culprit: %s)\n",
					iss.Level, iss.Title, iss.Platform, iss.EventCount, iss.Culprit))
			}
		}
	}

	systemPrompt := fmt.Sprintf(`You are an AI assistant that helps configure alert rules for an error tracking system.
Alerts notify teams via email or Slack when issues match certain conditions.

Each alert has:
- alert_type: "email" or "slack"
- conditions: a JSON condition tree that filters which issues trigger the alert

The conditions system supports these operators:
- level_is: matches error level (value: "error", "warning", "fatal", "info")
- platform_is: matches platform name
- title_contains: regex match on issue title and event data
- title_not_contains: regex negative match
- total_events: numeric comparison on total event count (operators: gte, lte, eq, gt, lt)
- velocity_1h: events in the last hour
- velocity_24h: events in the last 24 hours
- user_count: unique affected users
- priority: numeric comparison on the issue's priority score (0-100, base 50). Alerts with priority conditions fire when the priority changes after rule evaluation.

Conditions can be grouped with "and" / "or" operators.

When suggesting alerts, respond with valid JSON in this exact format:
{
  "message": "Your conversational response explaining the suggestions",
  "suggestions": [
    {
      "name": "Descriptive name for this alert",
      "alert_type": "email or slack",
      "conditions": {
        "operator": "and",
        "conditions": [
          { "type": "level_is", "value": "fatal" },
          { "type": "total_events", "operator": "gte", "value": "5" }
        ]
      },
      "explanation": "Why this alert is useful"
    }
  ]
}

Keep suggestions practical. Suggest email alerts by default unless the user mentions Slack.

%s%s`, alertsCtx.String(), issuesCtx.String())

	resp, err := h.aiService.ThinkingChat(r.Context(), projectID, "alert_suggest", ai.ChatRequest{
		SystemPrompt: systemPrompt,
		Messages:     req.Messages,
		MaxTokens:    2048,
		Temperature:  0.3,
		JSON:         true,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("AI call failed: %v", err))
		return
	}

	content := stripCodeFences(resp.Content)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(content))
}

func validateAlertConfig(alertType string, raw json.RawMessage) error {
	switch alertType {
	case "email":
		var cfg struct {
			Recipients []string `json:"recipients"`
		}
		if err := json.Unmarshal(raw, &cfg); err != nil {
			return fmt.Errorf("invalid email config: %w", err)
		}
		if len(cfg.Recipients) == 0 || cfg.Recipients[0] == "" {
			return fmt.Errorf("at least one recipient email is required")
		}
	case "slack":
		var cfg struct {
			WebhookURL string `json:"webhook_url"`
		}
		if err := json.Unmarshal(raw, &cfg); err != nil {
			return fmt.Errorf("invalid slack config: %w", err)
		}
		if cfg.WebhookURL == "" {
			return fmt.Errorf("webhook URL is required")
		}
	}
	return nil
}

func toSafeAlert(a db.AlertConfig) map[string]any {
	m := map[string]any{
		"id":              a.ID,
		"project_id":      a.ProjectID,
		"alert_type":      a.AlertType,
		"config":          redactAlertConfig(a.AlertType, a.Config),
		"enabled":         a.Enabled,
		"level_filter":    a.LevelFilter,
		"title_pattern":   a.TitlePattern,
		"min_events":      a.MinEvents,
		"min_velocity_1h": a.MinVelocity1h,
		"exclude_pattern": a.ExcludePattern,
		"created_at":      a.CreatedAt,
		"updated_at":      a.UpdatedAt,
	}
	if a.Conditions.Valid {
		m["conditions"] = a.Conditions.RawMessage
	} else {
		m["conditions"] = nil
	}
	return m
}

// redactAlertConfig strips secrets from alert config for API responses.
// Slack webhook URLs are replaced with a masked indicator.
func redactAlertConfig(alertType string, raw json.RawMessage) json.RawMessage {
	switch alertType {
	case "slack":
		var cfg map[string]any
		if err := json.Unmarshal(raw, &cfg); err != nil {
			return raw
		}
		if url, ok := cfg["webhook_url"].(string); ok && url != "" {
			cfg["webhook_url_set"] = true
			delete(cfg, "webhook_url")
		}
		out, _ := json.Marshal(cfg)
		return out
	default:
		return raw
	}
}

func toSafeAlerts(configs []db.AlertConfig) []map[string]any {
	result := make([]map[string]any, len(configs))
	for i, c := range configs {
		result[i] = toSafeAlert(c)
	}
	return result
}

// preserveSlackWebhook keeps the existing webhook_url when the incoming config
// omits it (because the API redacts it on read).
func preserveSlackWebhook(existingRaw, incomingRaw json.RawMessage) json.RawMessage {
	var incoming map[string]any
	if err := json.Unmarshal(incomingRaw, &incoming); err != nil {
		return incomingRaw
	}

	// If the incoming request has a non-empty webhook_url, use it as-is
	if url, ok := incoming["webhook_url"].(string); ok && url != "" {
		return incomingRaw
	}

	// Otherwise, preserve the existing one
	var existing map[string]any
	if err := json.Unmarshal(existingRaw, &existing); err != nil {
		return incomingRaw
	}
	if url, ok := existing["webhook_url"].(string); ok && url != "" {
		incoming["webhook_url"] = url
		out, _ := json.Marshal(incoming)
		return out
	}

	return incomingRaw
}

func toNullJSON(raw json.RawMessage) pqtype.NullRawMessage {
	if len(raw) == 0 || string(raw) == "null" {
		return pqtype.NullRawMessage{}
	}
	return pqtype.NullRawMessage{RawMessage: raw, Valid: true}
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func stripCodeFences(s string) string {
	s = strings.TrimSpace(s)
	fence := "```"
	if strings.HasPrefix(s, fence) {
		if idx := strings.Index(s, "\n"); idx != -1 {
			s = s[idx+1:]
		}
		if idx := strings.LastIndex(s, fence); idx != -1 {
			s = s[:idx]
		}
		s = strings.TrimSpace(s)
	}
	return s
}
