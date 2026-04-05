package jira

import (
	"regexp"
	"strings"

	"github.com/darkspock/gosnag/internal/database/db"
)

// MatchesRule checks whether an issue satisfies all conditions of a Jira rule.
func MatchesRule(rule db.JiraRule, issue db.Issue, userCount int32) bool {
	// Level filter (comma-separated list)
	if rule.LevelFilter != "" {
		levels := strings.Split(rule.LevelFilter, ",")
		matched := false
		for _, l := range levels {
			if strings.TrimSpace(l) == issue.Level {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}

	// Minimum events
	if rule.MinEvents > 0 && issue.EventCount < rule.MinEvents {
		return false
	}

	// Minimum users
	if rule.MinUsers > 0 && userCount < rule.MinUsers {
		return false
	}

	// Title pattern (plain text = contains, regex if starts/ends with special chars)
	if rule.TitlePattern != "" {
		re, err := regexp.Compile(rule.TitlePattern)
		if err != nil {
			// Fall back to contains match
			if !strings.Contains(strings.ToLower(issue.Title), strings.ToLower(rule.TitlePattern)) {
				return false
			}
		} else if !re.MatchString(issue.Title) {
			return false
		}
	}

	return true
}
