package priority

import (
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

type RuleRequest struct {
	Name       string          `json:"name"`
	RuleType   string          `json:"rule_type"`
	Pattern    string          `json:"pattern"`
	Operator   string          `json:"operator"`
	Threshold  int32           `json:"threshold"`
	Points     int32           `json:"points"`
	Enabled    bool            `json:"enabled"`
	Conditions json.RawMessage `json:"conditions,omitempty"`
}

func (h *Handler) ListRules(w http.ResponseWriter, r *http.Request) {
	projectID, err := uuid.Parse(chi.URLParam(r, "project_id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid project id")
		return
	}

	rules, err := h.queries.ListPriorityRules(r.Context(), projectID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list rules")
		return
	}
	writeJSON(w, http.StatusOK, rules)
}

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
	if req.RuleType == "" {
		writeError(w, http.StatusBadRequest, "rule_type is required")
		return
	}

	rule, err := h.queries.CreatePriorityRule(r.Context(), db.CreatePriorityRuleParams{
		ProjectID:  projectID,
		Name:       req.Name,
		RuleType:   req.RuleType,
		Pattern:    req.Pattern,
		Operator:   req.Operator,
		Threshold:  req.Threshold,
		Points:     req.Points,
		Enabled:    req.Enabled,
		Conditions: toNullJSON(req.Conditions),
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create rule")
		return
	}
	writeJSON(w, http.StatusCreated, rule)
}

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

	rule, err := h.queries.UpdatePriorityRule(r.Context(), db.UpdatePriorityRuleParams{
		ID:         ruleID,
		ProjectID:  projectID,
		Name:       req.Name,
		RuleType:   req.RuleType,
		Pattern:    req.Pattern,
		Operator:   req.Operator,
		Threshold:  req.Threshold,
		Points:     req.Points,
		Enabled:    req.Enabled,
		Conditions: toNullJSON(req.Conditions),
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update rule")
		return
	}
	writeJSON(w, http.StatusOK, rule)
}

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

	err = h.queries.DeletePriorityRule(r.Context(), db.DeletePriorityRuleParams{
		ID:        ruleID,
		ProjectID: projectID,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete rule")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// RecalcAll recalculates priority for all issues in a project.
// Clears AI evaluation history so AI rules re-run with current prompts.
func (h *Handler) RecalcAll(w http.ResponseWriter, r *http.Request) {
	projectID, err := uuid.Parse(chi.URLParam(r, "project_id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid project id")
		return
	}

	// Clear AI evaluations so AI rules re-run
	if err := h.queries.DeleteAIPriorityEvaluationsByProject(r.Context(), projectID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to clear AI evaluations")
		return
	}

	count, err := EvaluateAll(r.Context(), h.queries, h.aiService, projectID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to recalculate")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"recalculated": count})
}

// SuggestRules handles POST /projects/{project_id}/priority-rules/suggest
// Conversational AI assistant for creating priority rules (uses thinking model).
func (h *Handler) SuggestRules(w http.ResponseWriter, r *http.Request) {
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

	// Build context: existing rules
	rules, _ := h.queries.ListPriorityRules(r.Context(), projectID)
	var rulesCtx strings.Builder
	if len(rules) > 0 {
		rulesCtx.WriteString("## Existing Priority Rules\n")
		for _, rule := range rules {
			status := "enabled"
			if !rule.Enabled {
				status = "disabled"
			}
			rulesCtx.WriteString(fmt.Sprintf("- %s (type: %s, points: %d, %s)\n", rule.Name, rule.RuleType, rule.Points, status))
		}
	} else {
		rulesCtx.WriteString("## Existing Priority Rules\nNone configured yet.\n")
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

	systemPrompt := fmt.Sprintf(`You are an AI assistant that helps create priority rules for an error tracking system.
The system scores issues 0-100 (base score 50). Rules add or subtract points when they match.

Available rule types:
- velocity_1h: matches when events in last hour >= threshold (operators: gte, lte, eq, gt, lt)
- velocity_24h: matches when events in last 24h >= threshold
- total_events: matches when total event count meets threshold
- user_count: matches when unique affected users meets threshold
- title_contains: matches when issue title/event data matches regex pattern
- title_not_contains: matches when title/event data does NOT match pattern
- level_is: matches exact error level (error, warning, info, fatal)
- platform_is: matches exact platform name
- ai_prompt: AI evaluates the issue with a custom prompt at a specific event count threshold

When suggesting rules, respond with valid JSON in this exact format:
{
  "message": "Your conversational response explaining the suggestions",
  "suggestions": [
    {
      "name": "Rule name",
      "rule_type": "one of the types above",
      "pattern": "regex pattern or AI prompt text",
      "operator": "gte|lte|eq|gt|lt (for numeric rules)",
      "threshold": 0,
      "points": 10,
      "explanation": "Why this rule is useful"
    }
  ]
}

For ai_prompt rules: pattern contains the AI evaluation prompt, threshold is the event count trigger, points is the max absolute score (AI returns between -points and +points).
For title_contains/title_not_contains: pattern is a regex.
For level_is/platform_is: pattern is the exact match value.
For velocity/total_events/user_count: threshold is the numeric value, operator defaults to "gte".

Keep suggestions practical and specific to the project's error patterns.

%s%s`, rulesCtx.String(), issuesCtx.String())

	resp, err := h.aiService.ThinkingChat(r.Context(), projectID, "rule_suggest", ai.ChatRequest{
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

	// Strip markdown code fences if the model wrapped the JSON
	content := strings.TrimSpace(resp.Content)
	if strings.HasPrefix(content, "```") {
		// Remove opening fence (```json or ```)
		if idx := strings.Index(content, "\n"); idx != -1 {
			content = content[idx+1:]
		}
		// Remove closing fence
		if idx := strings.LastIndex(content, "```"); idx != -1 {
			content = content[:idx]
		}
		content = strings.TrimSpace(content)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(content))
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
