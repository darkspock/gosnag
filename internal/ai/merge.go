package ai

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/darkspock/gosnag/internal/database/db"
	"github.com/google/uuid"
	"github.com/sqlc-dev/pqtype"
)

// MergeChecker runs periodically to find and suggest duplicate issues.
type MergeChecker struct {
	queries  *db.Queries
	service  *Service
	database *sql.DB
	lastRun  time.Time
}

// NewMergeChecker creates a new merge checker.
func NewMergeChecker(queries *db.Queries, service *Service, database *sql.DB) *MergeChecker {
	return &MergeChecker{
		queries:  queries,
		service:  service,
		database: database,
		lastRun:  time.Now(),
	}
}

// Run starts the merge checker background loop.
func (mc *MergeChecker) Run(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			mc.check(ctx)
		}
	}
}

func (mc *MergeChecker) check(ctx context.Context) {
	projects, err := mc.queries.ListAIEnabledProjects(ctx)
	if err != nil {
		slog.Error("ai merge: failed to list projects", "error", err)
		return
	}

	since := mc.lastRun
	mc.lastRun = time.Now()

	for _, project := range projects {
		if !project.AiEnabled || !project.AiMergeSuggestions {
			continue
		}

		newIssues, err := mc.queries.ListNewIssuesSince(ctx, db.ListNewIssuesSinceParams{
			ProjectID: project.ID,
			FirstSeen: since,
		})
		if err != nil {
			slog.Error("ai merge: failed to list new issues", "error", err, "project", project.ID)
			continue
		}

		if len(newIssues) == 0 {
			continue
		}

		// Get issues that already have pending suggestions (exclude them)
		pendingIssueIDs, err := mc.queries.ListIssuesWithPendingSuggestions(ctx, project.ID)
		if err != nil {
			slog.Error("ai merge: failed to list pending suggestions", "error", err)
			continue
		}
		pendingSet := make(map[uuid.UUID]bool)
		for _, id := range pendingIssueIDs {
			pendingSet[id] = true
		}

		evaluated := 0
		for _, newIssue := range newIssues {
			if pendingSet[newIssue.ID] {
				continue
			}
			// Limit to 3 issues per project per cycle to avoid exhausting the rate limit
			if evaluated >= 3 {
				break
			}
			evaluated++

			mc.evaluateIssue(ctx, project, newIssue, pendingSet)
		}
	}
}

func (mc *MergeChecker) evaluateIssue(ctx context.Context, project db.Project, newIssue db.Issue, pendingSet map[uuid.UUID]bool) {
	// Fetch candidates (recent open issues, excluding the new one and issues with pending suggestions)
	candidates, err := mc.queries.ListRecentOpenIssues(ctx, db.ListRecentOpenIssuesParams{
		ProjectID: project.ID,
		ID:        newIssue.ID,
	})
	if err != nil || len(candidates) == 0 {
		return
	}

	// Filter out candidates with pending suggestions
	var filtered []db.Issue
	for _, c := range candidates {
		if !pendingSet[c.ID] {
			filtered = append(filtered, c)
		}
	}
	if len(filtered) == 0 {
		return
	}

	// Get latest event for new issue (for stack trace)
	newEvent, err := mc.queries.GetLatestEventByIssue(ctx, newIssue.ID)
	if err != nil {
		return
	}

	// Get latest events for candidates
	var candidatesWithEvents []candidateWithEvent
	for _, c := range filtered {
		event, err := mc.queries.GetLatestEventByIssue(ctx, c.ID)
		if err != nil {
			continue
		}
		candidatesWithEvents = append(candidatesWithEvents, candidateWithEvent{issue: c, event: event})
	}
	if len(candidatesWithEvents) == 0 {
		return
	}

	// Build prompt
	prompt := buildMergePrompt(newIssue, newEvent, candidatesWithEvents)

	resp, err := mc.service.Chat(ctx, project.ID, "auto_merge", ChatRequest{
		SystemPrompt: mergeSystemPrompt,
		Messages: []Message{
			{Role: "user", Content: prompt},
		},
		MaxTokens:   500,
		Temperature: 0.1,
		JSON:        true,
	})
	if err != nil {
		slog.Warn("ai merge: chat failed", "error", err, "issue", newIssue.ID)
		return
	}

	// Parse response
	var result mergeResponse
	if err := json.Unmarshal([]byte(resp.Content), &result); err != nil {
		slog.Warn("ai merge: failed to parse response", "error", err, "content", resp.Content)
		return
	}

	if result.MergeWith == "" || result.Confidence < 0.8 {
		return
	}

	targetID, err := uuid.Parse(result.MergeWith)
	if err != nil {
		slog.Warn("ai merge: invalid target ID", "merge_with", result.MergeWith)
		return
	}

	// Verify target exists and belongs to the same project
	target, err := mc.queries.GetIssue(ctx, targetID)
	if err != nil || target.ProjectID != project.ID {
		return
	}

	if project.AiAutoMerge {
		// Auto-merge: execute merge directly in a transaction
		if err := mc.executeMerge(ctx, project.ID, newIssue, targetID, result.Reason); err != nil {
			slog.Error("ai merge: auto-merge failed", "error", err, "source", newIssue.ID, "target", targetID)
		} else {
			slog.Info("ai merge: auto-merged", "source", newIssue.ID, "target", targetID, "confidence", result.Confidence)
		}
		return
	}

	// Create suggestion for manual review
	_, err = mc.queries.CreateMergeSuggestion(ctx, db.CreateMergeSuggestionParams{
		IssueID:       newIssue.ID,
		TargetIssueID: targetID,
		ProjectID:     project.ID,
		Confidence:    float32(result.Confidence),
		Reason:        result.Reason,
	})
	if err != nil {
		slog.Error("ai merge: failed to create suggestion", "error", err)
	}
}

func (mc *MergeChecker) executeMerge(ctx context.Context, projectID uuid.UUID, source db.Issue, targetID uuid.UUID, reason string) error {
	tx, err := mc.database.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	txq := db.New(tx)

	// Move events from source to target
	if _, err := txq.MoveEventsToIssue(ctx, db.MoveEventsToIssueParams{
		IssueID:   source.ID,
		IssueID_2: targetID,
	}); err != nil {
		return fmt.Errorf("move events: %w", err)
	}

	// Repoint aliases
	if err := txq.RepointAliases(ctx, db.RepointAliasesParams{
		PrimaryIssueID:   source.ID,
		PrimaryIssueID_2: targetID,
	}); err != nil {
		return fmt.Errorf("repoint aliases: %w", err)
	}

	// Create fingerprint alias
	if err := txq.CreateIssueAlias(ctx, db.CreateIssueAliasParams{
		ProjectID:      projectID,
		Fingerprint:    source.Fingerprint,
		PrimaryIssueID: targetID,
	}); err != nil {
		return fmt.Errorf("create alias: %w", err)
	}

	// Log activity on target issue
	metadata, _ := json.Marshal(map[string]string{
		"source_issue_id":    source.ID.String(),
		"source_issue_title": source.Title,
		"reason":             reason,
	})
	txq.InsertActivity(ctx, db.InsertActivityParams{
		IssueID:  targetID,
		Action:   "ai_auto_merged",
		NewValue: sql.NullString{String: source.Title, Valid: true},
		Metadata: pqtype.NullRawMessage{RawMessage: metadata, Valid: true},
	})

	// Delete the source issue
	if err := txq.DeleteIssue(ctx, source.ID); err != nil {
		return fmt.Errorf("delete source: %w", err)
	}

	// Recalculate target stats
	if _, err := txq.RecalcIssueStats(ctx, targetID); err != nil {
		return fmt.Errorf("recalc stats: %w", err)
	}

	return tx.Commit()
}

const mergeSystemPrompt = `You are analyzing error groups in an error tracking system.
Your task is to determine if a new issue is a duplicate of an existing issue.
Only suggest merging if the root cause is clearly the same (confidence > 0.8).

Respond in JSON:
{
  "merge_with": "issue_id_string_or_empty",
  "confidence": 0.0,
  "reason": "explanation"
}

If no match found, set merge_with to "" and confidence to 0.`

type mergeResponse struct {
	MergeWith  string  `json:"merge_with"`
	Confidence float64 `json:"confidence"`
	Reason     string  `json:"reason"`
}

type candidateWithEvent struct {
	issue db.Issue
	event db.Event
}

func buildMergePrompt(newIssue db.Issue, newEvent db.Event, candidates []candidateWithEvent) string {
	var sb strings.Builder

	sb.WriteString("New issue:\n")
	sb.WriteString(fmt.Sprintf("Title: %s\n", newIssue.Title))
	sb.WriteString(fmt.Sprintf("Level: %s\n", newIssue.Level))
	sb.WriteString(fmt.Sprintf("Platform: %s\n", newIssue.Platform))
	sb.WriteString(fmt.Sprintf("Stack trace (top 5 frames):\n%s\n", extractTopFrames(newEvent.Data, 5)))

	sb.WriteString("\nExisting open issues:\n")
	for i, c := range candidates {
		sb.WriteString(fmt.Sprintf("%d. ID: %s, Title: %s, Stack: %s\n",
			i+1, c.issue.ID.String(), c.issue.Title, extractTopFrames(c.event.Data, 3)))
	}

	return sb.String()
}

func extractTopFrames(data json.RawMessage, n int) string {
	if data == nil {
		return "(no data)"
	}
	var eventData map[string]json.RawMessage
	if err := json.Unmarshal(data, &eventData); err != nil {
		return "(parse error)"
	}
	exception, ok := eventData["exception"]
	if !ok {
		return "(no exception)"
	}
	var exc struct {
		Values []struct {
			Type       string `json:"type"`
			Value      string `json:"value"`
			Stacktrace struct {
				Frames []struct {
					Filename string `json:"filename"`
					Function string `json:"function"`
					Lineno   int    `json:"lineno"`
				} `json:"frames"`
			} `json:"stacktrace"`
		} `json:"values"`
	}
	if err := json.Unmarshal(exception, &exc); err != nil {
		return "(parse error)"
	}
	var sb strings.Builder
	for _, val := range exc.Values {
		sb.WriteString(fmt.Sprintf("%s: %s\n", val.Type, val.Value))
		frames := val.Stacktrace.Frames
		start := 0
		if len(frames) > n {
			start = len(frames) - n
		}
		for i := len(frames) - 1; i >= start; i-- {
			f := frames[i]
			sb.WriteString(fmt.Sprintf("  %s (%s:%d)\n", f.Function, f.Filename, f.Lineno))
		}
	}
	return sb.String()
}
