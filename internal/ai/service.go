package ai

import (
	"context"
	"database/sql"
	"log/slog"
	"time"

	"github.com/darkspock/gosnag/internal/config"
	"github.com/darkspock/gosnag/internal/database/db"
	"github.com/google/uuid"
)

// Service wraps a Provider with token tracking, rate limiting, and caching.
type Service struct {
	provider Provider
	queries  *db.Queries
	cfg      *config.Config
}

// NewService creates an AI service. Returns nil if AI is not configured.
func NewService(queries *db.Queries, cfg *config.Config) *Service {
	provider := NewProvider(cfg)
	if provider == nil {
		return nil
	}
	return &Service{
		provider: provider,
		queries:  queries,
		cfg:      cfg,
	}
}

// Chat executes an AI call with budget, rate limit, and cache checks.
func (s *Service) Chat(ctx context.Context, projectID uuid.UUID, feature string, req ChatRequest) (*ChatResponse, error) {
	// Check daily token budget
	dailyUsage, err := s.queries.GetDailyTokenUsage(ctx, projectID)
	if err != nil && err != sql.ErrNoRows {
		slog.Error("ai: failed to check daily usage", "error", err)
	}
	if dailyUsage >= int64(s.cfg.AIMaxTokensPerDay) {
		return nil, ErrBudgetExceeded
	}

	// Check rate limit (calls in last 60s)
	callCount, err := s.queries.GetCallsInLastMinute(ctx, projectID)
	if err != nil && err != sql.ErrNoRows {
		slog.Error("ai: failed to check rate limit", "error", err)
	}
	if callCount >= int64(s.cfg.AIMaxCallsPerMinute) {
		return nil, ErrRateLimited
	}

	// Check cache
	promptHash := req.PromptHash()
	cached, err := s.queries.GetCachedResponse(ctx, db.GetCachedResponseParams{
		ProjectID:  projectID,
		PromptHash: sql.NullString{String: promptHash, Valid: true},
	})
	if err == nil && cached.Valid && cached.String != "" {
		// Log cache hit (0 tokens, counts against rate limit)
		s.logCall(ctx, projectID, feature, 0, 0, 0, promptHash, "")
		return &ChatResponse{Content: cached.String}, nil
	}

	// Call the provider
	start := time.Now()
	resp, err := s.provider.Chat(ctx, req)
	latency := time.Since(start).Milliseconds()

	if err != nil {
		// Log failure (counts against rate limit, not against budget)
		s.logCall(ctx, projectID, feature, 0, 0, int(latency), "", "")
		return nil, err
	}

	// Log success with cache
	s.logCall(ctx, projectID, feature, resp.InputTokens, resp.OutputTokens, int(latency), promptHash, resp.Content)
	return resp, nil
}

// ProviderName returns the name of the configured provider.
func (s *Service) ProviderName() string {
	if s == nil || s.provider == nil {
		return ""
	}
	return s.provider.Name()
}

func (s *Service) logCall(ctx context.Context, projectID uuid.UUID, feature string, inputTokens, outputTokens, latencyMs int, promptHash, cachedResponse string) {
	model := ""
	if s.provider != nil {
		model = s.cfg.AIModel
		if model == "" {
			model = s.provider.Name() + "-default"
		}
	}
	err := s.queries.LogAICall(ctx, db.LogAICallParams{
		ProjectID:      projectID,
		Feature:        feature,
		Model:          model,
		InputTokens:    int32(inputTokens),
		OutputTokens:   int32(outputTokens),
		LatencyMs:      int32(latencyMs),
		PromptHash:     sql.NullString{String: promptHash, Valid: promptHash != ""},
		CachedResponse: sql.NullString{String: cachedResponse, Valid: cachedResponse != ""},
	})
	if err != nil {
		slog.Error("ai: failed to log call", "error", err, "feature", feature)
	}
}

// CacheCleanup runs periodically to clear expired cached responses.
func CacheCleanup(ctx context.Context, queries *db.Queries, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := queries.ClearExpiredAICache(ctx); err != nil {
				slog.Error("ai: cache cleanup failed", "error", err)
			}
		}
	}
}
