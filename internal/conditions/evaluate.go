package conditions

import (
	"regexp"
	"strings"
)

// Evaluate checks whether a condition group matches the given context.
func Evaluate(group Group, ctx *EvalContext) bool {
	if len(group.Conditions) == 0 {
		return true
	}

	if group.Operator == "or" {
		for _, node := range group.Conditions {
			if evaluateNode(node, ctx) {
				return true
			}
		}
		return false
	}

	// Default: AND
	for _, node := range group.Conditions {
		if !evaluateNode(node, ctx) {
			return false
		}
	}
	return true
}

func evaluateNode(node Node, ctx *EvalContext) bool {
	if node.IsGroup() {
		return Evaluate(node.AsGroup(), ctx)
	}
	return evaluateLeaf(node, ctx)
}

func evaluateLeaf(node Node, ctx *EvalContext) bool {
	switch node.Type {
	case "level":
		return compareString(ctx.Issue.Level, node.Op, node)
	case "platform":
		return compareString(ctx.Issue.Platform, node.Op, node)
	case "environment":
		return compareString(ctx.Issue.Environment, node.Op, node)
	case "release":
		return compareString(ctx.Issue.Release, node.Op, node)
	case "title":
		return compareText(ctx.Issue.Title, node.Op, node.StringValue())
	case "event_data":
		searchText := ctx.Issue.Title + "\n" + ctx.EventData
		return compareText(searchText, node.Op, node.StringValue())
	case "total_events":
		return compareInt(ctx.Issue.EventCount, node.Op, node.IntValue())
	case "velocity_1h":
		return compareInt(ctx.Velocity1h(), node.Op, node.IntValue())
	case "velocity_24h":
		return compareInt(ctx.Velocity24h(), node.Op, node.IntValue())
	case "user_count":
		return compareInt(ctx.UserCount(), node.Op, node.IntValue())
	case "priority":
		return compareInt(ctx.Issue.Priority, node.Op, node.IntValue())
	default:
		return false
	}
}

// compareString handles eq, neq, in, not_in for string fields.
func compareString(value, op string, node Node) bool {
	switch op {
	case "eq", "":
		return strings.EqualFold(value, node.StringValue())
	case "neq":
		return !strings.EqualFold(value, node.StringValue())
	case "in":
		for _, s := range node.StringSliceValue() {
			if strings.EqualFold(value, s) {
				return true
			}
		}
		return false
	case "not_in":
		for _, s := range node.StringSliceValue() {
			if strings.EqualFold(value, s) {
				return false
			}
		}
		return true
	case "contains":
		return strings.Contains(strings.ToLower(value), strings.ToLower(node.StringValue()))
	case "not_contains":
		return !strings.Contains(strings.ToLower(value), strings.ToLower(node.StringValue()))
	case "matches":
		return MatchesPattern(node.StringValue(), value)
	default:
		return strings.EqualFold(value, node.StringValue())
	}
}

// compareText handles contains, not_contains, matches for text fields.
func compareText(text, op, pattern string) bool {
	switch op {
	case "contains", "":
		return MatchesPattern(pattern, text)
	case "not_contains":
		return !MatchesPattern(pattern, text)
	case "matches":
		re, err := regexp.Compile(pattern)
		if err != nil {
			return false
		}
		return re.MatchString(text)
	case "eq":
		return strings.EqualFold(text, pattern)
	case "neq":
		return !strings.EqualFold(text, pattern)
	default:
		return MatchesPattern(pattern, text)
	}
}

// compareInt handles numeric comparison operators.
func compareInt(value int32, op string, threshold int32) bool {
	switch op {
	case "gte", ">=", "":
		return value >= threshold
	case "lte", "<=":
		return value <= threshold
	case "eq", "==":
		return value == threshold
	case "neq", "!=":
		return value != threshold
	case "gt", ">":
		return value > threshold
	case "lt", "<":
		return value < threshold
	default:
		return value >= threshold
	}
}

// MatchesPattern tries regex first, falls back to case-insensitive substring.
// Exported so other packages can use the shared implementation.
func MatchesPattern(pattern, text string) bool {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return strings.Contains(strings.ToLower(text), strings.ToLower(pattern))
	}
	return re.MatchString(text)
}
