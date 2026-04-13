package tags

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

// --- Issue Tags ---

type TagRequest struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// validateIssueScope checks that the issue exists and belongs to the project in the URL.
func (h *Handler) validateIssueScope(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	projectID, err := uuid.Parse(chi.URLParam(r, "project_id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid project id")
		return uuid.Nil, false
	}
	issueID, err := uuid.Parse(chi.URLParam(r, "issue_id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid issue id")
		return uuid.Nil, false
	}
	issue, err := h.queries.GetIssue(r.Context(), issueID)
	if err != nil {
		writeError(w, http.StatusNotFound, "issue not found")
		return uuid.Nil, false
	}
	if issue.ProjectID != projectID {
		writeError(w, http.StatusNotFound, "issue not found")
		return uuid.Nil, false
	}
	return issueID, true
}

func (h *Handler) ListIssueTags(w http.ResponseWriter, r *http.Request) {
	issueID, ok := h.validateIssueScope(w, r)
	if !ok {
		return
	}
	tags, err := h.queries.ListIssueTags(r.Context(), issueID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list tags")
		return
	}
	writeJSON(w, http.StatusOK, tags)
}

func (h *Handler) AddTag(w http.ResponseWriter, r *http.Request) {
	issueID, ok := h.validateIssueScope(w, r)
	if !ok {
		return
	}
	var req TagRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Key == "" || req.Value == "" {
		writeError(w, http.StatusBadRequest, "key and value are required")
		return
	}
	err := h.queries.AddIssueTag(r.Context(), db.AddIssueTagParams{
		IssueID: issueID,
		Key:     req.Key,
		Value:   req.Value,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to add tag")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"status": "added"})
}

func (h *Handler) RemoveTag(w http.ResponseWriter, r *http.Request) {
	issueID, ok := h.validateIssueScope(w, r)
	if !ok {
		return
	}
	var req TagRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Key == "" || req.Value == "" {
		writeError(w, http.StatusBadRequest, "key and value are required")
		return
	}
	err := h.queries.RemoveIssueTag(r.Context(), db.RemoveIssueTagParams{
		IssueID: issueID,
		Key:     req.Key,
		Value:   req.Value,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to remove tag")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "removed"})
}

func (h *Handler) ListDistinctTags(w http.ResponseWriter, r *http.Request) {
	projectID, err := uuid.Parse(chi.URLParam(r, "project_id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid project id")
		return
	}
	tags, err := h.queries.ListDistinctTags(r.Context(), projectID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list tags")
		return
	}
	writeJSON(w, http.StatusOK, tags)
}

// --- Tag Rules ---

type TagRuleRequest struct {
	Name       string          `json:"name"`
	RuleType   string          `json:"rule_type"`
	Pattern    string          `json:"pattern"`
	TagKey     string          `json:"tag_key"`
	TagValue   string          `json:"tag_value"`
	Threshold  int32           `json:"threshold"`
	Enabled    bool            `json:"enabled"`
	Conditions json.RawMessage `json:"conditions,omitempty"`
}

func toNullJSON(raw json.RawMessage) pqtype.NullRawMessage {
	if len(raw) == 0 || string(raw) == "null" {
		return pqtype.NullRawMessage{}
	}
	return pqtype.NullRawMessage{RawMessage: raw, Valid: true}
}

func (h *Handler) ListRules(w http.ResponseWriter, r *http.Request) {
	projectID, err := uuid.Parse(chi.URLParam(r, "project_id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid project id")
		return
	}
	rules, err := h.queries.ListTagRules(r.Context(), projectID)
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
	var req TagRuleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	ruleType := req.RuleType
	if ruleType == "" {
		ruleType = "pattern"
	}
	if req.Name == "" || req.TagKey == "" {
		writeError(w, http.StatusBadRequest, "name and tag_key are required")
		return
	}
	if req.Pattern == "" || req.TagValue == "" {
		writeError(w, http.StatusBadRequest, "pattern and tag_value are required")
		return
	}
	rule, err := h.queries.CreateTagRule(r.Context(), db.CreateTagRuleParams{
		ProjectID:  projectID,
		Name:       req.Name,
		Pattern:    req.Pattern,
		TagKey:     req.TagKey,
		TagValue:   req.TagValue,
		Enabled:    req.Enabled,
		Conditions: toNullJSON(req.Conditions),
		RuleType:   ruleType,
		Threshold:  req.Threshold,
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
	var req TagRuleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	ruleType := req.RuleType
	if ruleType == "" {
		ruleType = "pattern"
	}
	rule, err := h.queries.UpdateTagRule(r.Context(), db.UpdateTagRuleParams{
		ID:         ruleID,
		ProjectID:  projectID,
		Name:       req.Name,
		Pattern:    req.Pattern,
		TagKey:     req.TagKey,
		TagValue:   req.TagValue,
		Enabled:    req.Enabled,
		Conditions: toNullJSON(req.Conditions),
		RuleType:   ruleType,
		Threshold:  req.Threshold,
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
	err = h.queries.DeleteTagRule(r.Context(), db.DeleteTagRuleParams{
		ID:        ruleID,
		ProjectID: projectID,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete rule")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// SuggestTags handles POST /projects/{project_id}/tag-rules/suggest
// Conversational AI assistant for creating tag rules (uses thinking model).
func (h *Handler) SuggestTags(w http.ResponseWriter, r *http.Request) {
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

	// Build context: existing tag rules
	rules, _ := h.queries.ListTagRules(r.Context(), projectID)
	var rulesCtx strings.Builder
	if len(rules) > 0 {
		rulesCtx.WriteString("## Existing Tag Rules\n")
		for _, rule := range rules {
			status := "enabled"
			if !rule.Enabled {
				status = "disabled"
			}
			rulesCtx.WriteString(fmt.Sprintf("- %s: pattern=%q → %s:%s (%s)\n", rule.Name, rule.Pattern, rule.TagKey, rule.TagValue, status))
		}
	} else {
		rulesCtx.WriteString("## Existing Tag Rules\nNone configured yet.\n")
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

	// Build existing tags context
	distinctTags, _ := h.queries.ListDistinctTags(r.Context(), projectID)
	var tagsCtx strings.Builder
	if len(distinctTags) > 0 {
		tagsCtx.WriteString("\n## Existing Tags in Use\n")
		for _, t := range distinctTags {
			tagsCtx.WriteString(fmt.Sprintf("- %s:%s\n", t.Key, t.Value))
		}
	}

	systemPrompt := fmt.Sprintf(`You are an AI assistant that helps create auto-tagging rules for an error tracking system.
Tag rules automatically label issues when their title or event data matches a pattern.

Each tag rule has:
- name: descriptive name for the rule
- pattern: regex pattern to match against issue title and event data
- tag_key: the tag key to apply (e.g. "team", "service", "category")
- tag_value: the tag value to apply (e.g. "payments", "auth", "database")

Tags are key:value pairs. Common tagging strategies:
- By team ownership: team:payments, team:platform, team:frontend
- By service/component: service:api, service:worker, service:web
- By error category: category:database, category:network, category:auth
- By severity context: impact:user-facing, impact:internal
- By environment: env:production, env:staging

When suggesting tag rules, respond with valid JSON in this exact format:
{
  "message": "Your conversational response explaining the suggestions",
  "suggestions": [
    {
      "name": "Rule name",
      "pattern": "regex pattern",
      "tag_key": "key",
      "tag_value": "value",
      "explanation": "Why this rule is useful"
    }
  ]
}

Keep suggestions practical. Use regex patterns that match error titles and stack traces.
Suggest consistent tag keys to enable useful filtering (don't create 10 different keys — reuse a few).

%s%s%s`, rulesCtx.String(), tagsCtx.String(), issuesCtx.String())

	resp, err := h.aiService.ThinkingChat(r.Context(), projectID, "tag_suggest", ai.ChatRequest{
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

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
