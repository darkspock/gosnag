package priority

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/darkspock/gosnag/internal/ai"
	"github.com/darkspock/gosnag/internal/conditions"
	"github.com/darkspock/gosnag/internal/database/db"
	"github.com/google/uuid"
)

// velocityCache caches event counts per issue to avoid repeated queries during bursts.
type velocityCache struct {
	mu    sync.RWMutex
	items map[string]cachedCount
}

type cachedCount struct {
	count     int32
	expiresAt time.Time
}

var cache = &velocityCache{items: make(map[string]cachedCount)}

func (c *velocityCache) get(key string) (int32, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	item, ok := c.items[key]
	if !ok || time.Now().After(item.expiresAt) {
		return 0, false
	}
	return item.count, true
}

func (c *velocityCache) set(key string, count int32) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items[key] = cachedCount{count: count, expiresAt: time.Now().Add(60 * time.Second)}
	// Lazy cleanup: remove expired entries when cache grows
	if len(c.items) > 1000 {
		now := time.Now()
		for k, v := range c.items {
			if now.After(v.expiresAt) {
				delete(c.items, k)
			}
		}
	}
}

// OnPriorityChange is called when an issue's priority score changes.
type OnPriorityChange func(projectID uuid.UUID, issue db.Issue, oldPriority, newPriority int32)

// Evaluate calculates the priority score for an issue based on project rules.
// Should be called asynchronously after event ingestion.
// eventData is the raw JSON of the latest event (for full-text search in rules).
// aiService may be nil if AI is not configured.
// onChange is called when priority actually changes (may be nil).
func Evaluate(ctx context.Context, queries *db.Queries, aiService *ai.Service, projectID uuid.UUID, issue db.Issue, eventData json.RawMessage, onChange OnPriorityChange) {
	rules, err := queries.ListEnabledPriorityRules(ctx, projectID)
	if err != nil || len(rules) == 0 {
		return
	}

	// Lazy-load expensive data only if needed
	var velocity1h, velocity24h *int32
	var userCount *int32

	// Flatten event data to string for full-text pattern matching
	eventText := string(eventData)

	// Build searchable text: title + full event data
	searchText := issue.Title + "\n" + eventText

	// Build shared eval context for the conditions engine
	loader := &priorityLoader{queries: queries, ctx: ctx, cache: cache}
	evalCtx := conditions.NewEvalContext(conditions.IssueData{
		ID:         issue.ID,
		Title:      issue.Title,
		Level:      issue.Level,
		Platform:   issue.Platform,
		EventCount: issue.EventCount,
	}, eventText, loader)

	score := int32(50) // base score

	for _, rule := range rules {
		matched := false

		// New engine: if conditions JSONB is set, use it
		if rule.Conditions.Valid {
			var group conditions.Group
			if err := json.Unmarshal(rule.Conditions.RawMessage, &group); err == nil {
				matched = conditions.Evaluate(group, evalCtx)
			}
		} else {
			// Legacy path: flat columns
			switch rule.RuleType {
			case "velocity_1h":
				if velocity1h == nil {
					v := getVelocity1h(ctx, queries, issue.ID)
					velocity1h = &v
				}
				matched = compareInt(*velocity1h, rule.Operator, rule.Threshold)

			case "velocity_24h":
				if velocity24h == nil {
					v := getVelocity24h(ctx, queries, issue.ID)
					velocity24h = &v
				}
				matched = compareInt(*velocity24h, rule.Operator, rule.Threshold)

			case "total_events":
				matched = compareInt(issue.EventCount, rule.Operator, rule.Threshold)

			case "user_count":
				if userCount == nil {
					uc := getUserCount(ctx, queries, issue.ID)
					userCount = &uc
				}
				matched = compareInt(*userCount, rule.Operator, rule.Threshold)

			case "title_contains":
				if rule.Pattern != "" {
					matched = matchesPattern(rule.Pattern, searchText)
				}

			case "title_not_contains":
				if rule.Pattern != "" {
					matched = !matchesPattern(rule.Pattern, searchText)
				}

			case "level_is":
				matched = strings.EqualFold(issue.Level, rule.Pattern)

			case "platform_is":
				matched = strings.EqualFold(issue.Platform, rule.Pattern)

			case "ai_prompt":
				pts := evaluateAIRule(ctx, queries, aiService, rule, issue, eventData)
				score += pts
				continue // skip the matched check below, points already applied
			}
		}

		if matched {
			score += rule.Points
		}
	}

	// Clamp 0–100
	if score < 0 {
		score = 0
	}
	if score > 100 {
		score = 100
	}

	// Only update if changed
	if score != issue.Priority {
		oldPriority := issue.Priority
		if err := queries.UpdateIssuePriority(ctx, db.UpdateIssuePriorityParams{
			ID:       issue.ID,
			Priority: score,
		}); err != nil {
			slog.Error("failed to update issue priority", "error", err, "issue_id", issue.ID)
		} else if onChange != nil {
			issue.Priority = score
			onChange(projectID, issue, oldPriority, score)
		}
	}
}

// EvaluateAll recalculates priority for all issues in a project.
// Loads rules once and reuses them across all issues to avoid N+1 queries.
func EvaluateAll(ctx context.Context, queries *db.Queries, aiService *ai.Service, projectID uuid.UUID, onChange OnPriorityChange) (int, error) {
	rules, err := queries.ListEnabledPriorityRules(ctx, projectID)
	if err != nil || len(rules) == 0 {
		return 0, err
	}

	issueIDs, err := queries.ListIssueIDsByProject(ctx, projectID)
	if err != nil {
		return 0, err
	}

	count := 0
	for _, id := range issueIDs {
		issue, err := queries.GetIssue(ctx, id)
		if err != nil {
			continue
		}
		var eventData json.RawMessage
		events, err := queries.ListEventsByIssue(ctx, db.ListEventsByIssueParams{IssueID: id, Limit: 1, Offset: 0})
		if err == nil && len(events) > 0 {
			eventData = events[0].Data
		}
		evaluateWithRules(ctx, queries, aiService, rules, issue, eventData, onChange)
		count++
	}
	return count, nil
}

// evaluateWithRules scores an issue using pre-loaded rules (avoids reloading per issue).
func evaluateWithRules(ctx context.Context, queries *db.Queries, aiService *ai.Service, rules []db.PriorityRule, issue db.Issue, eventData json.RawMessage, onChange OnPriorityChange) {
	eventText := string(eventData)
	searchText := issue.Title + "\n" + eventText

	loader := &priorityLoader{queries: queries, ctx: ctx, cache: cache}
	evalCtx := conditions.NewEvalContext(conditions.IssueData{
		ID:         issue.ID,
		Title:      issue.Title,
		Level:      issue.Level,
		Platform:   issue.Platform,
		EventCount: issue.EventCount,
	}, eventText, loader)

	var velocity1h, velocity24h, userCount *int32
	score := int32(50)

	for _, rule := range rules {
		matched := false

		if rule.Conditions.Valid {
			var group conditions.Group
			if err := json.Unmarshal(rule.Conditions.RawMessage, &group); err == nil {
				matched = conditions.Evaluate(group, evalCtx)
			}
		} else {
			switch rule.RuleType {
			case "velocity_1h":
				if velocity1h == nil {
					v := getVelocity1h(ctx, queries, issue.ID)
					velocity1h = &v
				}
				matched = compareInt(*velocity1h, rule.Operator, rule.Threshold)
			case "velocity_24h":
				if velocity24h == nil {
					v := getVelocity24h(ctx, queries, issue.ID)
					velocity24h = &v
				}
				matched = compareInt(*velocity24h, rule.Operator, rule.Threshold)
			case "total_events":
				matched = compareInt(issue.EventCount, rule.Operator, rule.Threshold)
			case "user_count":
				if userCount == nil {
					uc := getUserCount(ctx, queries, issue.ID)
					userCount = &uc
				}
				matched = compareInt(*userCount, rule.Operator, rule.Threshold)
			case "title_contains":
				if rule.Pattern != "" {
					matched = matchesPattern(rule.Pattern, searchText)
				}
			case "title_not_contains":
				if rule.Pattern != "" {
					matched = !matchesPattern(rule.Pattern, searchText)
				}
			case "level_is":
				matched = strings.EqualFold(issue.Level, rule.Pattern)
			case "platform_is":
				matched = strings.EqualFold(issue.Platform, rule.Pattern)

			case "ai_prompt":
				pts := evaluateAIRule(ctx, queries, aiService, rule, issue, eventData)
				score += pts
				continue
			}
		}

		if matched {
			score += rule.Points
		}
	}

	if score < 0 {
		score = 0
	}
	if score > 100 {
		score = 100
	}

	if score != issue.Priority {
		oldPriority := issue.Priority
		if err := queries.UpdateIssuePriority(ctx, db.UpdateIssuePriorityParams{
			ID:       issue.ID,
			Priority: score,
		}); err != nil {
			slog.Error("failed to update issue priority", "error", err, "issue_id", issue.ID)
		} else if onChange != nil {
			issue.Priority = score
			onChange(issue.ProjectID, issue, oldPriority, score)
		}
	}
}

func matchesPattern(pattern, text string) bool {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return strings.Contains(strings.ToLower(text), strings.ToLower(pattern))
	}
	return re.MatchString(text)
}

func getVelocity1h(ctx context.Context, queries *db.Queries, issueID uuid.UUID) int32 {
	key := "v1h:" + issueID.String()
	if v, ok := cache.get(key); ok {
		return v
	}
	count, err := queries.GetIssueVelocity1h(ctx, issueID)
	if err != nil {
		return 0
	}
	cache.set(key, count)
	return count
}

func getVelocity24h(ctx context.Context, queries *db.Queries, issueID uuid.UUID) int32 {
	key := "v24h:" + issueID.String()
	if v, ok := cache.get(key); ok {
		return v
	}
	count, err := queries.GetIssueVelocity24h(ctx, issueID)
	if err != nil {
		return 0
	}
	cache.set(key, count)
	return count
}

func getUserCount(ctx context.Context, queries *db.Queries, issueID uuid.UUID) int32 {
	count, err := queries.GetIssueUserCount(ctx, issueID)
	if err != nil {
		return 0
	}
	return int32(count)
}

func compareInt(value int32, operator string, threshold int32) bool {
	switch operator {
	case "gte", ">=", "":
		return value >= threshold
	case "lte", "<=":
		return value <= threshold
	case "eq", "==":
		return value == threshold
	case "gt", ">":
		return value > threshold
	case "lt", "<":
		return value < threshold
	default:
		return value >= threshold
	}
}

// priorityLoader implements conditions.DataLoader using cached velocity queries.
type priorityLoader struct {
	queries *db.Queries
	ctx     context.Context
	cache   *velocityCache
}

func (l *priorityLoader) GetVelocity1h(issueID uuid.UUID) (int32, error) {
	return getVelocity1h(l.ctx, l.queries, issueID), nil
}

func (l *priorityLoader) GetVelocity24h(issueID uuid.UUID) (int32, error) {
	return getVelocity24h(l.ctx, l.queries, issueID), nil
}

func (l *priorityLoader) GetUserCount(issueID uuid.UUID) (int32, error) {
	return getUserCount(l.ctx, l.queries, issueID), nil
}

// evaluateAIRule handles the ai_prompt rule type. Returns points to add to score.
// The rule fires once per issue when event_count >= threshold. Results are stored
// in ai_priority_evaluations as both execution guard and audit log.
func evaluateAIRule(ctx context.Context, queries *db.Queries, aiService *ai.Service, rule db.PriorityRule, issue db.Issue, eventData json.RawMessage) int32 {
	if aiService == nil {
		return 0
	}

	// Only evaluate when issue reaches the event threshold
	if issue.EventCount < rule.Threshold {
		return 0
	}

	// Check if already evaluated
	eval, err := queries.GetAIPriorityEvaluation(ctx, db.GetAIPriorityEvaluationParams{
		IssueID: issue.ID,
		RuleID:  rule.ID,
	})
	if err == nil {
		// Already evaluated
		if eval.Status == "success" {
			return eval.Points
		}
		// Error status — retry if under limit
		if eval.Retries >= 3 {
			return 0
		}
		// Fall through to retry
	} else if err != sql.ErrNoRows {
		slog.Error("ai priority: failed to check evaluation", "error", err, "issue_id", issue.ID, "rule_id", rule.ID)
		return 0
	}

	retries := int32(0)
	if err == nil {
		retries = eval.Retries
	}

	// Build the AI prompt
	eventSnippet := string(eventData)
	if len(eventSnippet) > 2000 {
		eventSnippet = eventSnippet[:2000] + "..."
	}

	systemPrompt := fmt.Sprintf(`You are an AI assistant that evaluates error issues for priority scoring.
You will receive an evaluation prompt and issue details. Respond with valid JSON only.

Response format: {"points": <int>, "reason": "<string>"}
The points value must be between %d and %d.
Positive points mean higher priority, negative means lower priority.
Keep the reason concise (1-2 sentences).`, -rule.Points, rule.Points)

	userContent := fmt.Sprintf(`## Evaluation Criteria
%s

## Issue Details
- Title: %s
- Level: %s
- Platform: %s
- Event Count: %d
- Culprit: %s

## Latest Event Data
%s`, rule.Pattern, issue.Title, issue.Level, issue.Platform, issue.EventCount, issue.Culprit, eventSnippet)

	resp, err := aiService.Chat(ctx, issue.ProjectID, "priority_eval", ai.ChatRequest{
		SystemPrompt: systemPrompt,
		Messages:     []ai.Message{{Role: "user", Content: userContent}},
		MaxTokens:    256,
		Temperature:  0.1,
		JSON:         true,
	})
	if err != nil {
		slog.Error("ai priority: call failed", "error", err, "issue_id", issue.ID, "rule_id", rule.ID)
		queries.UpsertAIPriorityEvaluation(ctx, db.UpsertAIPriorityEvaluationParams{
			IssueID: issue.ID,
			RuleID:  rule.ID,
			Status:  "error",
			Points:  0,
			Reason:  err.Error(),
			Retries: retries + 1,
		})
		return 0
	}

	// Parse AI response
	var result struct {
		Points int32  `json:"points"`
		Reason string `json:"reason"`
	}
	if err := json.Unmarshal([]byte(resp.Content), &result); err != nil {
		slog.Error("ai priority: failed to parse response", "error", err, "content", resp.Content, "issue_id", issue.ID)
		queries.UpsertAIPriorityEvaluation(ctx, db.UpsertAIPriorityEvaluationParams{
			IssueID: issue.ID,
			RuleID:  rule.ID,
			Status:  "error",
			Points:  0,
			Reason:  fmt.Sprintf("parse error: %v", err),
			Retries: retries + 1,
		})
		return 0
	}

	// Clamp points to [-rule.Points, +rule.Points]
	if result.Points > rule.Points {
		result.Points = rule.Points
	}
	if result.Points < -rule.Points {
		result.Points = -rule.Points
	}

	// Store success
	queries.UpsertAIPriorityEvaluation(ctx, db.UpsertAIPriorityEvaluationParams{
		IssueID: issue.ID,
		RuleID:  rule.ID,
		Status:  "success",
		Points:  result.Points,
		Reason:  result.Reason,
		Retries: retries,
	})

	return result.Points
}
