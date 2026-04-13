package conditions

import (
	"github.com/google/uuid"
)

// DataLoader provides lazy-loaded expensive data for condition evaluation.
type DataLoader interface {
	GetVelocity1h(issueID uuid.UUID) (int32, error)
	GetVelocity24h(issueID uuid.UUID) (int32, error)
	GetUserCount(issueID uuid.UUID) (int32, error)
}

// IssueData holds the issue fields available for condition evaluation.
type IssueData struct {
	ID          uuid.UUID
	Title       string
	Level       string
	Platform    string
	EventCount  int32
	Environment string
	Release     string
	Priority    int32
}

// EvalContext provides data for condition evaluation with lazy loading.
type EvalContext struct {
	Issue     IssueData
	EventData string // raw JSON of the latest event

	loader      DataLoader
	velocity1h  *int32
	velocity24h *int32
	userCount   *int32
}

// NewEvalContext creates a context for condition evaluation.
func NewEvalContext(issue IssueData, eventData string, loader DataLoader) *EvalContext {
	return &EvalContext{
		Issue:     issue,
		EventData: eventData,
		loader:    loader,
	}
}

// Velocity1h returns the 1-hour velocity, loading it on first access.
func (c *EvalContext) Velocity1h() int32 {
	if c.velocity1h != nil {
		return *c.velocity1h
	}
	v := int32(0)
	if c.loader != nil {
		if val, err := c.loader.GetVelocity1h(c.Issue.ID); err == nil {
			v = val
		}
	}
	c.velocity1h = &v
	return v
}

// Velocity24h returns the 24-hour velocity, loading it on first access.
func (c *EvalContext) Velocity24h() int32 {
	if c.velocity24h != nil {
		return *c.velocity24h
	}
	v := int32(0)
	if c.loader != nil {
		if val, err := c.loader.GetVelocity24h(c.Issue.ID); err == nil {
			v = val
		}
	}
	c.velocity24h = &v
	return v
}

// UserCount returns the distinct user count, loading it on first access.
func (c *EvalContext) UserCount() int32 {
	if c.userCount != nil {
		return *c.userCount
	}
	v := int32(0)
	if c.loader != nil {
		if val, err := c.loader.GetUserCount(c.Issue.ID); err == nil {
			v = val
		}
	}
	c.userCount = &v
	return v
}
