package tags

import (
	"context"
	"encoding/json"
	"log/slog"
	"regexp"
	"strings"

	"github.com/darkspock/gosnag/internal/conditions"
	"github.com/darkspock/gosnag/internal/database/db"
	"github.com/google/uuid"
)

// AutoTag evaluates tag rules for an issue and applies matching tags.
// Searches the issue title and error-relevant event fields (exception, request, breadcrumbs, transaction).
// Excludes noise fields like "modules" (installed packages) to prevent false positives.
// Should be called asynchronously after event ingestion.
func AutoTag(ctx context.Context, queries *db.Queries, projectID uuid.UUID, issue db.Issue, eventData json.RawMessage) {
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
