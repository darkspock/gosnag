package tags

import (
	"context"
	"log/slog"
	"regexp"
	"strings"

	"github.com/darkspock/gosnag/internal/database/db"
	"github.com/google/uuid"
)

// AutoTag evaluates tag rules for an issue and applies matching tags.
// Should be called asynchronously after event ingestion.
func AutoTag(ctx context.Context, queries *db.Queries, projectID uuid.UUID, issue db.Issue) {
	rules, err := queries.ListEnabledTagRules(ctx, projectID)
	if err != nil || len(rules) == 0 {
		return
	}

	for _, rule := range rules {
		if matchesPattern(rule.Pattern, issue.Title) {
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

func matchesPattern(pattern, title string) bool {
	re, err := regexp.Compile(pattern)
	if err != nil {
		// Fall back to case-insensitive contains
		return strings.Contains(strings.ToLower(title), strings.ToLower(pattern))
	}
	return re.MatchString(title)
}
