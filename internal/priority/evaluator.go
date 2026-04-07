package priority

import (
	"context"
	"encoding/json"
	"log/slog"
	"regexp"
	"strings"
	"sync"
	"time"

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

// Evaluate calculates the priority score for an issue based on project rules.
// Should be called asynchronously after event ingestion.
// eventData is the raw JSON of the latest event (for full-text search in rules).
func Evaluate(ctx context.Context, queries *db.Queries, projectID uuid.UUID, issue db.Issue, eventData json.RawMessage) {
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
		if err := queries.UpdateIssuePriority(ctx, db.UpdateIssuePriorityParams{
			ID:       issue.ID,
			Priority: score,
		}); err != nil {
			slog.Error("failed to update issue priority", "error", err, "issue_id", issue.ID)
		}
	}
}

// EvaluateAll recalculates priority for all issues in a project.
// Loads rules once and reuses them across all issues to avoid N+1 queries.
func EvaluateAll(ctx context.Context, queries *db.Queries, projectID uuid.UUID) (int, error) {
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
		evaluateWithRules(ctx, queries, rules, issue, eventData)
		count++
	}
	return count, nil
}

// evaluateWithRules scores an issue using pre-loaded rules (avoids reloading per issue).
func evaluateWithRules(ctx context.Context, queries *db.Queries, rules []db.PriorityRule, issue db.Issue, eventData json.RawMessage) {
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
		if err := queries.UpdateIssuePriority(ctx, db.UpdateIssuePriorityParams{
			ID:       issue.ID,
			Priority: score,
		}); err != nil {
			slog.Error("failed to update issue priority", "error", err, "issue_id", issue.ID)
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
