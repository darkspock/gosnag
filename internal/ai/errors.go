package ai

import "errors"

var (
	ErrBudgetExceeded = errors.New("ai: daily token budget exceeded")
	ErrRateLimited    = errors.New("ai: rate limit exceeded")
	ErrNotConfigured  = errors.New("ai: provider not configured")
	ErrDisabled       = errors.New("ai: feature disabled for this project")
)
