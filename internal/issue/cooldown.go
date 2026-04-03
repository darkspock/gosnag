package issue

import (
	"context"
	"database/sql"
	"log/slog"
	"time"

	"github.com/darkspock/gosnag/internal/database/db"
)

// CooldownChecker periodically confirms resolved issues whose cooldown has expired
// without any new events (they stay resolved). Issues that received events during
// cooldown and are still marked "resolved" will have already been reopened by the
// ingest handler, so this goroutine only needs to clean up the cooldown_until field.
func CooldownChecker(ctx context.Context, queries *db.Queries, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	slog.Info("cooldown checker started", "interval", interval)

	for {
		select {
		case <-ctx.Done():
			slog.Info("cooldown checker stopped")
			return
		case <-ticker.C:
			confirmExpiredCooldowns(ctx, queries)
			reopenExpiredSnoozes(ctx, queries)
		}
	}
}

func reopenExpiredSnoozes(ctx context.Context, queries *db.Queries) {
	issues, err := queries.GetExpiredSnoozeIssues(ctx)
	if err != nil {
		slog.Error("failed to get expired snooze issues", "error", err)
		return
	}

	for _, iss := range issues {
		_, err := queries.UpdateIssueStatus(ctx, db.UpdateIssueStatusParams{
			ID:     iss.ID,
			Status: "reopened",
		})
		if err != nil {
			slog.Error("failed to unsnooze issue", "error", err, "issue_id", iss.ID)
		} else {
			slog.Info("issue unsnoozed (time expired)", "issue_id", iss.ID)
		}
	}
}

func confirmExpiredCooldowns(ctx context.Context, queries *db.Queries) {
	issues, err := queries.GetExpiredCooldownIssues(ctx)
	if err != nil {
		slog.Error("failed to get expired cooldown issues", "error", err)
		return
	}

	for _, iss := range issues {
		// Clear cooldown - issue stays resolved (confirmed)
		_, err := queries.UpdateIssueStatus(ctx, db.UpdateIssueStatusParams{
			ID:                iss.ID,
			Status:            "resolved",
			ResolvedAt:        iss.ResolvedAt,
			CooldownUntil:     sql.NullTime{Valid: false},
			ResolvedInRelease: iss.ResolvedInRelease,
		})
		if err != nil {
			slog.Error("failed to confirm resolved issue", "error", err, "issue_id", iss.ID)
		} else {
			slog.Info("confirmed resolved issue (cooldown expired)", "issue_id", iss.ID)
		}
	}
}
