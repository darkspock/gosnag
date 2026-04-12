package ai

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/darkspock/gosnag/internal/database/db"
	"github.com/google/uuid"
)

// AnalyzeRootCause generates a root cause analysis for an issue.
func AnalyzeRootCause(ctx context.Context, svc *Service, queries *db.Queries, projectID, issueID uuid.UUID) (*db.AiAnalysis, error) {
	issue, err := queries.GetIssue(ctx, issueID)
	if err != nil {
		return nil, fmt.Errorf("get issue: %w", err)
	}

	// Get latest event for stack trace
	event, err := queries.GetLatestEventByIssue(ctx, issueID)
	if err != nil && err != sql.ErrNoRows {
		return nil, fmt.Errorf("get latest event: %w", err)
	}

	// Get user count
	userCount, _ := queries.GetIssueUserCount(ctx, issueID)

	// Get similar issues (top by events, same project)
	similarIssues, _ := queries.ListTopIssuesByEvents(ctx, db.ListTopIssuesByEventsParams{
		ProjectID: projectID,
		Limit:     5,
	})

	// Get recent deploys
	recentDeploys, _ := queries.ListRecentDeploys(ctx, db.ListRecentDeploysParams{
		ProjectID: projectID,
		Limit:     3,
	})

	// Build prompt
	prompt := buildRCAPrompt(issue, event, int(userCount), similarIssues, recentDeploys)

	resp, err := svc.Chat(ctx, projectID, "rca", ChatRequest{
		SystemPrompt: rcaSystemPrompt,
		Messages:     []Message{{Role: "user", Content: prompt}},
		MaxTokens:    2000,
		Temperature:  0.2,
		JSON:         true,
	})
	if err != nil {
		return nil, err
	}

	// Parse structured response
	var result rcaResponse
	if err := json.Unmarshal([]byte(resp.Content), &result); err != nil {
		return nil, fmt.Errorf("parse rca response: %w", err)
	}

	// Store evidence as JSON
	evidenceJSON, _ := json.Marshal(result.Evidence)

	model := svc.cfg.AIModel
	if model == "" {
		model = svc.ProviderName() + "-default"
	}

	analysis, err := queries.CreateAIAnalysis(ctx, db.CreateAIAnalysisParams{
		IssueID:      issueID,
		ProjectID:    projectID,
		Summary:      result.Summary,
		Evidence:     string(evidenceJSON),
		SuggestedFix: result.SuggestedFix,
		Model:        model,
	})
	if err != nil {
		return nil, fmt.Errorf("store analysis: %w", err)
	}

	return &analysis, nil
}

const rcaSystemPrompt = `You are an expert at analyzing software errors and determining root causes.
Given error details from an error tracking system, determine the root cause.

Respond in JSON:
{
  "summary": "1-2 sentence conclusion about the root cause",
  "evidence": ["evidence item 1", "evidence item 2", ...],
  "suggested_fix": "Actionable steps in Markdown format"
}

Be specific and actionable. Reference file names, function names, and line numbers when available.
The suggested_fix should be in Markdown with concrete steps a developer can follow.`

type rcaResponse struct {
	Summary      string   `json:"summary"`
	Evidence     []string `json:"evidence"`
	SuggestedFix string   `json:"suggested_fix"`
}

func buildRCAPrompt(issue db.Issue, event db.Event, userCount int, similar []db.Issue, deploys []db.Deploy) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("Error: %s\n", issue.Title))
	sb.WriteString(fmt.Sprintf("Level: %s\n", issue.Level))
	sb.WriteString(fmt.Sprintf("Platform: %s\n", issue.Platform))
	sb.WriteString(fmt.Sprintf("Culprit: %s\n", issue.Culprit))
	sb.WriteString(fmt.Sprintf("Events: %d across %d users\n", issue.EventCount, userCount))
	sb.WriteString(fmt.Sprintf("First seen: %s\n", issue.FirstSeen.Format("2006-01-02 15:04:05 UTC")))
	sb.WriteString(fmt.Sprintf("Last seen: %s\n", issue.LastSeen.Format("2006-01-02 15:04:05 UTC")))

	if issue.FirstRelease != "" {
		sb.WriteString(fmt.Sprintf("Release: %s\n", issue.FirstRelease))
	}

	// Full stack trace from latest event
	if event.Data != nil {
		var eventData map[string]json.RawMessage
		if err := json.Unmarshal(event.Data, &eventData); err == nil {
			if exception, ok := eventData["exception"]; ok {
				sb.WriteString("\nFull stack trace:\n")
				sb.WriteString(extractStackTrace(exception))
			}
			if breadcrumbs, ok := eventData["breadcrumbs"]; ok {
				sb.WriteString("\nBreadcrumbs (last 10):\n")
				sb.WriteString(extractBreadcrumbs(breadcrumbs))
			}
			if tags, ok := eventData["tags"]; ok {
				sb.WriteString("\nTags:\n")
				sb.WriteString(string(tags))
				sb.WriteString("\n")
			}
		}
	}

	// Similar issues
	if len(similar) > 0 {
		sb.WriteString("\nTop issues in project (by event count):\n")
		for i, s := range similar {
			if s.ID == issue.ID {
				continue
			}
			sb.WriteString(fmt.Sprintf("  %d. [%s] %s (%d events, %s)\n",
				i+1, s.Level, s.Title, s.EventCount, s.Status))
		}
	}

	// Recent deploys
	if len(deploys) > 0 {
		sb.WriteString("\nRecent deploys:\n")
		for _, d := range deploys {
			commit := ""
			if d.CommitSha.Valid {
				commit = " commit=" + d.CommitSha.String[:minInt(8, len(d.CommitSha.String))]
			}
			sb.WriteString(fmt.Sprintf("  - %s (%s)%s at %s\n",
				d.ReleaseVersion, d.Environment, commit, d.DeployedAt.Format(time.RFC3339)))
		}
	}

	return sb.String()
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
