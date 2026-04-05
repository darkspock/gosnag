package jira

import (
	"context"
	"database/sql"
	"log/slog"

	"github.com/darkspock/gosnag/internal/database/db"
	"github.com/google/uuid"
)

// CheckAndCreateTicket evaluates Jira rules for an issue and creates a ticket if matched.
// Should be called after event ingestion.
func CheckAndCreateTicket(ctx context.Context, queries *db.Queries, baseURL string, projectID uuid.UUID, issue db.Issue) {
	// Skip if already has a Jira ticket
	if issue.JiraTicketKey.Valid {
		return
	}

	project, err := queries.GetProject(ctx, projectID)
	if err != nil {
		return
	}

	cfg := ConfigFromProject(project)
	if !cfg.IsConfigured() {
		return
	}

	rules, err := queries.ListEnabledJiraRules(ctx, projectID)
	if err != nil || len(rules) == 0 {
		return
	}

	// Get user count for the issue (approximate from events)
	userCount := int32(0)
	if uc, err := queries.GetIssueUserCount(ctx, issue.ID); err == nil {
		userCount = int32(uc)
	}

	for _, rule := range rules {
		if MatchesRule(rule, issue, userCount) {
			// Re-check jira_ticket_key right before creating (race condition guard)
			fresh, err := queries.GetIssue(ctx, issue.ID)
			if err != nil || fresh.JiraTicketKey.Valid {
				return
			}

			summary := "[GoSnag] " + truncate(issue.Title, 200)
			description := BuildDescription(issue, baseURL, projectID.String(), "")

			result, err := CreateIssue(cfg, summary, description)
			if err != nil {
				slog.Error("failed to auto-create Jira ticket", "error", err, "issue_id", issue.ID, "rule", rule.Name)
				return
			}

			res, err := queries.UpdateIssueJiraTicket(ctx, db.UpdateIssueJiraTicketParams{
				ID:            issue.ID,
				JiraTicketKey: sql.NullString{String: result.Key, Valid: true},
				JiraTicketUrl: sql.NullString{String: result.URL, Valid: true},
			})
			if err != nil {
				slog.Error("failed to save Jira ticket reference", "error", err, "key", result.Key, "issue_id", issue.ID)
				return
			}
			if rows, _ := res.RowsAffected(); rows == 0 {
				slog.Warn("Jira ticket created but issue was linked concurrently", "key", result.Key, "issue_id", issue.ID)
				return
			}

			slog.Info("auto-created Jira ticket", "key", result.Key, "issue_id", issue.ID, "rule", rule.Name)
			return // Only create one ticket per issue
		}
	}
}
