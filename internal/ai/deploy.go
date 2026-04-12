package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/darkspock/gosnag/internal/database/db"
	"github.com/google/uuid"
)

// DeployAnalyzer checks recent deploys and runs anomaly detection.
type DeployAnalyzer struct {
	queries     *db.Queries
	service     *Service
	alertFunc   func(projectID uuid.UUID, severity, summary, details string)
	analyzed    map[uuid.UUID]bool // track already-analyzed deploy IDs
}

// NewDeployAnalyzer creates a new deploy analyzer.
func NewDeployAnalyzer(queries *db.Queries, service *Service, alertFunc func(uuid.UUID, string, string, string)) *DeployAnalyzer {
	return &DeployAnalyzer{
		queries:   queries,
		service:   service,
		alertFunc: alertFunc,
		analyzed:  make(map[uuid.UUID]bool),
	}
}

// Run starts the deploy analyzer background loop.
func (da *DeployAnalyzer) Run(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			da.check(ctx)
		}
	}
}

func (da *DeployAnalyzer) check(ctx context.Context) {
	projects, err := da.queries.ListAIEnabledProjects(ctx)
	if err != nil {
		slog.Error("ai deploy: failed to list projects", "error", err)
		return
	}

	for _, project := range projects {
		if !project.AiEnabled || !project.AiAnomalyDetection {
			continue
		}

		deploy, err := da.queries.GetLatestDeploy(ctx, project.ID)
		if err != nil {
			continue // no deploys
		}

		// Skip already analyzed
		if da.analyzed[deploy.ID] {
			continue
		}

		// Check if an analysis already exists
		if _, err := da.queries.GetDeployAnalysis(ctx, deploy.ID); err == nil {
			da.analyzed[deploy.ID] = true
			continue
		}

		// Wait 15 minutes after deploy
		if time.Since(deploy.DeployedAt) < 15*time.Minute {
			continue
		}

		da.analyzed[deploy.ID] = true
		da.analyzeDeploy(ctx, project, deploy)
	}

	// Clean up old entries to prevent memory growth
	if len(da.analyzed) > 1000 {
		da.analyzed = make(map[uuid.UUID]bool)
	}
}

func (da *DeployAnalyzer) analyzeDeploy(ctx context.Context, project db.Project, deploy db.Deploy) {
	slog.Info("ai deploy: analyzing", "project", project.ID, "deploy", deploy.ID, "version", deploy.ReleaseVersion)

	preStart := deploy.DeployedAt.Add(-15 * time.Minute)
	preEnd := deploy.DeployedAt
	postStart := deploy.DeployedAt
	postEnd := deploy.DeployedAt.Add(15 * time.Minute)

	// Count events pre and post
	preCount, _ := da.queries.CountEventsInWindow(ctx, db.CountEventsInWindowParams{
		ProjectID: project.ID,
		Timestamp: preStart,
		Timestamp_2: preEnd,
	})
	postCount, _ := da.queries.CountEventsInWindow(ctx, db.CountEventsInWindowParams{
		ProjectID: project.ID,
		Timestamp: postStart,
		Timestamp_2: postEnd,
	})

	// New issues since deploy
	newIssues, _ := da.queries.ListIssuesCreatedSince(ctx, db.ListIssuesCreatedSinceParams{
		ProjectID: project.ID,
		FirstSeen: deploy.DeployedAt,
	})

	// Reopened issues since deploy
	reopenedIssues, _ := da.queries.ListIssuesReopenedSince(ctx, db.ListIssuesReopenedSinceParams{
		ProjectID: project.ID,
		LastSeen: deploy.DeployedAt,
	})

	// Find spiked issues: compare per-issue event counts pre vs post
	preIssueCounts, _ := da.queries.CountEventsPerIssueInWindow(ctx, db.CountEventsPerIssueInWindowParams{
		ProjectID: project.ID,
		Timestamp: preStart,
		Timestamp_2: preEnd,
	})
	postIssueCounts, _ := da.queries.CountEventsPerIssueInWindow(ctx, db.CountEventsPerIssueInWindowParams{
		ProjectID: project.ID,
		Timestamp: postStart,
		Timestamp_2: postEnd,
	})

	preMap := make(map[uuid.UUID]int)
	for _, c := range preIssueCounts {
		preMap[c.IssueID] = int(c.EventCount)
	}

	var spiked []deploySpikedIssue
	for _, c := range postIssueCounts {
		pre := preMap[c.IssueID]
		post := int(c.EventCount)
		if pre > 0 && post >= pre*3 {
			spiked = append(spiked, deploySpikedIssue{c.IssueID, pre, post})
		}
	}

	// If no anomalies, store analysis with severity=none and skip AI
	if len(newIssues) == 0 && len(spiked) == 0 && len(reopenedIssues) == 0 {
		da.queries.CreateDeployAnalysis(ctx, db.CreateDeployAnalysisParams{
			DeployID:            deploy.ID,
			ProjectID:           project.ID,
			Severity:            "none",
			Summary:             "No anomalies detected after deploy",
			RecommendedAction:   "monitor",
		})
		slog.Info("ai deploy: no anomalies", "project", project.ID, "deploy", deploy.ID)
		return
	}

	// Build AI prompt
	prompt := da.buildDeployPrompt(deploy, preCount, postCount, newIssues, spiked, reopenedIssues)

	resp, err := da.service.Chat(ctx, project.ID, "anomaly", ChatRequest{
		SystemPrompt: deploySystemPrompt,
		Messages:     []Message{{Role: "user", Content: prompt}},
		MaxTokens:    1000,
		Temperature:  0.2,
		JSON:         true,
	})
	if err != nil {
		slog.Warn("ai deploy: chat failed", "error", err, "deploy", deploy.ID)
		return
	}

	var result deployAnalysisResponse
	if err := json.Unmarshal([]byte(resp.Content), &result); err != nil {
		slog.Warn("ai deploy: failed to parse response", "error", err, "content", resp.Content)
		return
	}

	analysis, err := da.queries.CreateDeployAnalysis(ctx, db.CreateDeployAnalysisParams{
		DeployID:            deploy.ID,
		ProjectID:           project.ID,
		Severity:            result.Severity,
		Summary:             result.Summary,
		Details:             result.Details,
		LikelyDeployCaused:  result.LikelyCausedByDeploy,
		RecommendedAction:   result.RecommendedAction,
		NewIssuesCount:      int32(len(newIssues)),
		SpikedIssuesCount:   int32(len(spiked)),
		ReopenedIssuesCount: int32(len(reopenedIssues)),
	})
	if err != nil {
		slog.Error("ai deploy: failed to store analysis", "error", err)
		return
	}

	slog.Info("ai deploy: analysis stored", "deploy", deploy.ID, "severity", analysis.Severity)

	// Send alerts for critical/warning
	if da.alertFunc != nil && (result.Severity == "critical" || result.Severity == "warning") {
		da.alertFunc(project.ID, result.Severity, result.Summary, result.Details)
	}
}

type deploySpikedIssue struct {
	issueID uuid.UUID
	pre     int
	post    int
}

func (da *DeployAnalyzer) buildDeployPrompt(deploy db.Deploy, preCount, postCount int64, newIssues []db.Issue, spiked []deploySpikedIssue, reopened []db.Issue) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("Deploy: version=%s, environment=%s, time=%s\n",
		deploy.ReleaseVersion, deploy.Environment, deploy.DeployedAt.Format(time.RFC3339)))
	if deploy.CommitSha.Valid {
		sb.WriteString(fmt.Sprintf("Commit: %s\n", deploy.CommitSha.String))
	}

	sb.WriteString(fmt.Sprintf("\nEvents: %d pre-deploy (15min) → %d post-deploy (15min)\n", preCount, postCount))

	if len(newIssues) > 0 {
		sb.WriteString(fmt.Sprintf("\nNew issues since deploy (%d):\n", len(newIssues)))
		for i, iss := range newIssues {
			if i >= 10 { break }
			sb.WriteString(fmt.Sprintf("  - [%s] %s (events: %d)\n", iss.Level, iss.Title, iss.EventCount))
		}
	}

	if len(spiked) > 0 {
		sb.WriteString(fmt.Sprintf("\nSpiked issues (%d, velocity 3x+ increase):\n", len(spiked)))
		for _, s := range spiked {
			sb.WriteString(fmt.Sprintf("  - Issue %s: %d → %d events\n", s.issueID, s.pre, s.post))
		}
	}

	if len(reopened) > 0 {
		sb.WriteString(fmt.Sprintf("\nReopened issues (%d):\n", len(reopened)))
		for i, iss := range reopened {
			if i >= 10 { break }
			sb.WriteString(fmt.Sprintf("  - [%s] %s\n", iss.Level, iss.Title))
		}
	}

	return sb.String()
}

const deploySystemPrompt = `You are analyzing the impact of a software deploy on an error tracking system.
Given pre-deploy and post-deploy error metrics, determine if the deploy caused issues.

Respond in JSON:
{
  "severity": "critical|warning|info|none",
  "summary": "one-line summary",
  "details": "multi-line explanation",
  "likely_caused_by_deploy": true/false,
  "recommended_action": "rollback|investigate|monitor|ignore"
}

Guidelines:
- "critical": new fatal/error issues directly caused by deploy, or 10x+ spike in errors
- "warning": moderate spike (3x-10x), some new issues, or regressions
- "info": minor changes, within normal variance
- "none": no meaningful change`

type deployAnalysisResponse struct {
	Severity             string `json:"severity"`
	Summary              string `json:"summary"`
	Details              string `json:"details"`
	LikelyCausedByDeploy bool   `json:"likely_caused_by_deploy"`
	RecommendedAction    string `json:"recommended_action"`
}

