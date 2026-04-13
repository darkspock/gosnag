package tags

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"regexp"
	"strings"

	"github.com/darkspock/gosnag/internal/ai"
	"github.com/darkspock/gosnag/internal/conditions"
	"github.com/darkspock/gosnag/internal/database/db"
	"github.com/google/uuid"
)

// AutoTag evaluates tag rules for an issue and applies matching tags.
// Searches the issue title and error-relevant event fields (exception, request, breadcrumbs, transaction).
// Excludes noise fields like "modules" (installed packages) to prevent false positives.
// Should be called asynchronously after event ingestion.
func AutoTag(ctx context.Context, queries *db.Queries, aiService *ai.Service, projectID uuid.UUID, issue db.Issue, eventData json.RawMessage) {
	rules, err := queries.ListEnabledTagRules(ctx, projectID)
	if err != nil || len(rules) == 0 {
		return
	}

	searchText := buildSearchText(issue.Title, eventData)

	// Shared eval context for conditions engine (no loader needed — tags don't use velocity/users)
	evalCtx := conditions.NewEvalContext(conditions.IssueData{
		ID:         issue.ID,
		Title:      issue.Title,
		Level:      issue.Level,
		Platform:   issue.Platform,
		EventCount: issue.EventCount,
	}, string(eventData), nil)

	for _, rule := range rules {
		if rule.RuleType == "ai_prompt" {
			evaluateAITagRule(ctx, queries, aiService, rule, issue, eventData)
			continue
		}

		matched := false
		if rule.Conditions.Valid {
			var group conditions.Group
			if err := json.Unmarshal(rule.Conditions.RawMessage, &group); err == nil {
				matched = conditions.Evaluate(group, evalCtx)
			}
		} else {
			matched = matchesPattern(rule.Pattern, searchText)
		}
		if matched {
			err := queries.AddIssueTag(ctx, db.AddIssueTagParams{
				IssueID: issue.ID,
				Key:     rule.TagKey,
				Value:   rule.TagValue,
			})
			if err != nil {
				slog.Error("failed to auto-tag issue", "error", err, "issue_id", issue.ID, "tag", rule.TagKey+":"+rule.TagValue)
			}
		}
	}
}

// evaluateAITagRule handles the ai_prompt rule type for tag classification.
// The rule fires once per issue when event_count >= threshold. Results are stored
// in ai_tag_evaluations as both execution guard and audit log.
func evaluateAITagRule(ctx context.Context, queries *db.Queries, aiService *ai.Service, rule db.TagRule, issue db.Issue, eventData json.RawMessage) {
	if aiService == nil {
		return
	}

	// Only evaluate when issue reaches the event threshold
	if rule.Threshold > 0 && issue.EventCount < rule.Threshold {
		return
	}

	// Check if already evaluated
	eval, err := queries.GetAITagEvaluation(ctx, db.GetAITagEvaluationParams{
		IssueID: issue.ID,
		RuleID:  rule.ID,
	})
	if err == nil {
		if eval.Status == "success" {
			return
		}
		if eval.Retries >= 3 {
			return
		}
	} else if err != sql.ErrNoRows {
		slog.Error("ai tag: failed to check evaluation", "error", err, "issue_id", issue.ID, "rule_id", rule.ID)
		return
	}

	retries := int32(0)
	if err == nil {
		retries = eval.Retries
	}

	// Build AI prompt
	eventSnippet := string(eventData)
	if len(eventSnippet) > 2000 {
		eventSnippet = eventSnippet[:2000] + "..."
	}

	systemPrompt := fmt.Sprintf(`You are an AI assistant that classifies error issues for tagging.
You will receive a classification prompt and issue details. Respond with valid JSON only.

Response format: {"tag_value": "<string>", "reason": "<string>"}
The tag_key for this rule is "%s".
Valid tag_value options: %s
If none of the options fit, return an empty tag_value "" to skip tagging.
Keep the reason concise (1-2 sentences).`, rule.TagKey, rule.TagValue)

	userContent := fmt.Sprintf(`## Classification Criteria
%s

## Issue Details
- Title: %s
- Level: %s
- Platform: %s
- Event Count: %d
- Culprit: %s

## Latest Event Data
%s`, rule.Pattern, issue.Title, issue.Level, issue.Platform, issue.EventCount, issue.Culprit, eventSnippet)

	resp, err := aiService.Chat(ctx, issue.ProjectID, "tag_eval", ai.ChatRequest{
		SystemPrompt: systemPrompt,
		Messages:     []ai.Message{{Role: "user", Content: userContent}},
		MaxTokens:    256,
		Temperature:  0.1,
		JSON:         true,
	})
	if err != nil {
		slog.Error("ai tag: call failed", "error", err, "issue_id", issue.ID, "rule_id", rule.ID)
		queries.UpsertAITagEvaluation(ctx, db.UpsertAITagEvaluationParams{
			IssueID:  issue.ID,
			RuleID:   rule.ID,
			Status:   "error",
			TagValue: "",
			Reason:   err.Error(),
			Retries:  retries + 1,
		})
		return
	}

	// Parse AI response
	var result struct {
		TagValue string `json:"tag_value"`
		Reason   string `json:"reason"`
	}
	cleaned := stripCodeFences(resp.Content)
	if err := json.Unmarshal([]byte(cleaned), &result); err != nil {
		slog.Error("ai tag: failed to parse response", "error", err, "content", resp.Content, "issue_id", issue.ID)
		queries.UpsertAITagEvaluation(ctx, db.UpsertAITagEvaluationParams{
			IssueID:  issue.ID,
			RuleID:   rule.ID,
			Status:   "error",
			TagValue: "",
			Reason:   fmt.Sprintf("parse error: %v", err),
			Retries:  retries + 1,
		})
		return
	}

	// Validate tag_value against the allowlist
	if result.TagValue != "" && rule.TagValue != "" {
		validValues := strings.Split(rule.TagValue, ",")
		valid := false
		for _, v := range validValues {
			if strings.TrimSpace(v) == result.TagValue {
				valid = true
				break
			}
		}
		if !valid {
			slog.Warn("ai tag: value not in allowlist", "value", result.TagValue, "allowed", rule.TagValue, "issue_id", issue.ID)
			result.Reason = fmt.Sprintf("AI returned value %q not in allowlist, skipped", result.TagValue)
			result.TagValue = ""
		}
	}

	// Store success (even if tag_value is empty, marking it as evaluated)
	queries.UpsertAITagEvaluation(ctx, db.UpsertAITagEvaluationParams{
		IssueID:  issue.ID,
		RuleID:   rule.ID,
		Status:   "success",
		TagValue: result.TagValue,
		Reason:   result.Reason,
		Retries:  retries,
	})

	// Apply the tag if non-empty
	if result.TagValue != "" {
		if err := queries.AddIssueTag(ctx, db.AddIssueTagParams{
			IssueID: issue.ID,
			Key:     rule.TagKey,
			Value:   result.TagValue,
		}); err != nil {
			slog.Error("failed to auto-tag issue from AI", "error", err, "issue_id", issue.ID, "tag", rule.TagKey+":"+result.TagValue)
		}
	}
}

// buildSearchText extracts only error-relevant fields from event data for pattern matching.
// This prevents false positives from noise fields like "modules" (composer/npm packages).
func buildSearchText(issueTitle string, eventData json.RawMessage) string {
	var buf strings.Builder
	buf.WriteString(issueTitle)
	buf.WriteByte('\n')

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(eventData, &raw); err != nil {
		// Fallback: if we can't parse, use the full data
		buf.Write(eventData)
		return buf.String()
	}

	// Only include fields relevant for error classification
	relevantKeys := []string{
		"exception",    // exception type, value, stacktrace
		"message",      // log message
		"logentry",     // structured log entry
		"transaction",  // transaction/endpoint name
		"request",      // HTTP request URL, method
		"breadcrumbs",  // breadcrumb trail
		"tags",         // SDK-provided tags
		"extra",        // extra context from SDK
		"fingerprint",  // custom fingerprint
	}

	for _, key := range relevantKeys {
		if val, ok := raw[key]; ok {
			buf.WriteByte('\n')
			buf.Write(val)
		}
	}

	return buf.String()
}

func matchesPattern(pattern, text string) bool {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return strings.Contains(strings.ToLower(text), strings.ToLower(pattern))
	}
	return re.MatchString(text)
}
