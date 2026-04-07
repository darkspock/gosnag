package n1

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/darkspock/gosnag/internal/database/db"
	"github.com/google/uuid"
)

const (
	// Minimum distinct events before flagging (avoid flukes)
	defaultMinEvents = 3
	// Minimum avg queries per event to flag as N+1
	defaultMinAvgPerEvent = 5
)

// Detector runs periodically to find N+1 query patterns and create issues.
type Detector struct {
	queries *db.Queries
	baseURL string
}

func NewDetector(queries *db.Queries, baseURL string) *Detector {
	return &Detector{queries: queries, baseURL: baseURL}
}

// Run starts the detection loop. Blocks until ctx is cancelled.
func (d *Detector) Run(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			d.detectAll(ctx)
		}
	}
}

func (d *Detector) detectAll(ctx context.Context) {
	projects, err := d.queries.ListProjects(ctx)
	if err != nil {
		slog.Error("n1: failed to list projects", "error", err)
		return
	}

	for _, p := range projects {
		if ctx.Err() != nil {
			return
		}
		d.detectForProject(ctx, p.ID)
	}
}

func (d *Detector) detectForProject(ctx context.Context, projectID uuid.UUID) {
	candidates, err := d.queries.ListN1Candidates(ctx, db.ListN1CandidatesParams{
		ProjectID:      projectID,
		DistinctEvents: defaultMinEvents,
		EventCount:     defaultMinAvgPerEvent,
	})
	if err != nil {
		slog.Error("n1: failed to list candidates", "error", err, "project_id", projectID)
		return
	}

	for _, c := range candidates {
		avgPerEvent := float64(c.EventCount) / float64(max(c.DistinctEvents, 1))
		if avgPerEvent < float64(defaultMinAvgPerEvent) {
			continue
		}

		fingerprint := n1Fingerprint(projectID, c.Transaction, c.QueryHash)
		title := fmt.Sprintf("[N+1] %s", truncate(c.NormalizedQuery, 120))
		if c.Transaction != "" {
			title += fmt.Sprintf(" — avg %.0f/req on %s", avgPerEvent, c.Transaction)
		}

		// Check if issue already exists — only create synthetic event for new detections
		existing, err := d.queries.GetIssueByFingerprint(ctx, db.GetIssueByFingerprintParams{
			ProjectID:   projectID,
			Fingerprint: fingerprint,
		})
		isNew := err != nil || existing.Status != "open"

		// Upsert as an issue (creates or updates title/last_seen)
		issue, err := d.queries.UpsertIssue(ctx, db.UpsertIssueParams{
			ProjectID:   projectID,
			Title:       title,
			Fingerprint: fingerprint,
			Level:       "warning",
			Platform:    "sql",
			FirstSeen:   c.FirstSeen,
		})
		if err != nil {
			slog.Error("n1: failed to upsert issue", "error", err, "fingerprint", fingerprint)
			continue
		}

		// Auto-tag the issue (idempotent)
		d.ensureTag(ctx, issue.ID, "n1", "detected")
		if c.TableName != "" {
			d.ensureTag(ctx, issue.ID, "table", c.TableName)
		}
		if c.Transaction != "" {
			d.ensureTag(ctx, issue.ID, "transaction", c.Transaction)
		}

		// Only create synthetic event for new detections or re-detections
		if isNew {
			avgMs := 0.0
			if c.EventCount > 0 {
				avgMs = c.TotalExecMs / float64(c.EventCount)
			}
			eventData := map[string]any{
				"n1_detection": map[string]any{
					"query":           c.NormalizedQuery,
					"table":           c.TableName,
					"transaction":     c.Transaction,
					"total_seen":      c.EventCount,
					"distinct_events": c.DistinctEvents,
					"avg_per_event":   avgPerEvent,
					"avg_exec_ms":     avgMs,
					"total_exec_ms":   c.TotalExecMs,
					"first_seen":      c.FirstSeen,
					"last_seen":       c.LastSeen,
				},
			}
			rawData, _ := json.Marshal(eventData)

			d.queries.CreateEvent(ctx, db.CreateEventParams{
				IssueID:        issue.ID,
				ProjectID:      projectID,
				EventID:        uuid.New().String(),
				Timestamp:      time.Now(),
				Platform:       "sql",
				Level:          "warning",
				Message:        title,
				Data:           rawData,
				UserIdentifier: "",
			})
		}
	}

	// Auto-resolve N+1 issues not seen in 24h
	d.autoResolve(ctx, projectID)
}

func (d *Detector) autoResolve(ctx context.Context, projectID uuid.UUID) {
	issues, err := d.queries.ListOpenN1Issues(ctx, projectID)
	if err != nil {
		return
	}

	cutoff := time.Now().Add(-24 * time.Hour)
	for _, iss := range issues {
		if iss.LastSeen.Before(cutoff) {
			d.queries.UpdateIssueStatus(ctx, db.UpdateIssueStatusParams{
				ID:         iss.ID,
				Status:     "resolved",
				ResolvedAt: sql.NullTime{Time: time.Now(), Valid: true},
			})
		}
	}
}

func (d *Detector) ensureTag(ctx context.Context, issueID uuid.UUID, key, value string) {
	d.queries.AddIssueTag(ctx, db.AddIssueTagParams{
		IssueID: issueID,
		Key:     key,
		Value:   value,
	})
}

func n1Fingerprint(projectID uuid.UUID, transaction, queryHash string) string {
	h := sha256.New()
	h.Write([]byte("n1:"))
	h.Write([]byte(projectID.String()))
	h.Write([]byte(transaction))
	h.Write([]byte(queryHash))
	return "n1:" + fmt.Sprintf("%x", h.Sum(nil))[:29]
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

func max(a, b int32) int32 {
	if a > b {
		return a
	}
	return b
}
