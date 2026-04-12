package ai

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/darkspock/gosnag/internal/database/db"
	"github.com/google/uuid"
)

// GenerateDescription generates a ticket description from issue context.
func GenerateDescription(ctx context.Context, svc *Service, queries *db.Queries, projectID, issueID uuid.UUID) (string, error) {
	// Gather issue context
	issue, err := queries.GetIssue(ctx, issueID)
	if err != nil {
		return "", fmt.Errorf("get issue: %w", err)
	}

	// Get latest event for stack trace + context
	event, err := queries.GetLatestEventByIssue(ctx, issueID)
	if err != nil && err != sql.ErrNoRows {
		return "", fmt.Errorf("get latest event: %w", err)
	}

	// Get user count
	userCount, _ := queries.GetIssueUserCount(ctx, issueID)

	// Build the prompt
	prompt := buildDescriptionPrompt(issue, event, int(userCount))

	resp, err := svc.Chat(ctx, projectID, "description", ChatRequest{
		SystemPrompt: descriptionSystemPrompt,
		Messages: []Message{
			{Role: "user", Content: prompt},
		},
		MaxTokens:   2000,
		Temperature: 0.3,
	})
	if err != nil {
		return "", err
	}

	return SanitizeHTML(resp.Content), nil
}

const descriptionSystemPrompt = `You are an expert at writing clear, actionable bug ticket descriptions for developers.
Given error details from an error tracking system, generate a well-structured HTML description.

Use these HTML tags only: p, h3, ul, ol, li, strong, em, code, pre, a, br, blockquote, table, thead, tbody, tr, th, td.

Structure the description with these sections:
1. Summary - What is happening (1-2 sentences)
2. Root Cause - Likely cause based on the stack trace
3. Impact - How many users/events, frequency, severity
4. Investigation Steps - Actionable steps to fix

Be concise but thorough. Do NOT include any <script>, <style>, or event handler attributes.`

func buildDescriptionPrompt(issue db.Issue, event db.Event, userCount int) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("Error: %s\n", issue.Title))
	sb.WriteString(fmt.Sprintf("Level: %s\n", issue.Level))
	sb.WriteString(fmt.Sprintf("Platform: %s\n", issue.Platform))
	sb.WriteString(fmt.Sprintf("Culprit: %s\n", issue.Culprit))
	sb.WriteString(fmt.Sprintf("Events: %d across %d users\n", issue.EventCount, userCount))
	sb.WriteString(fmt.Sprintf("First seen: %s\n", issue.FirstSeen.Format("2006-01-02 15:04:05 UTC")))
	sb.WriteString(fmt.Sprintf("Last seen: %s\n", issue.LastSeen.Format("2006-01-02 15:04:05 UTC")))

	if issue.FirstRelease != "" {
		sb.WriteString(fmt.Sprintf("Release: %s\n", issue.FirstRelease))
	}

	// Extract stack trace and context from event data
	if event.Data != nil {
		var eventData map[string]json.RawMessage
		if err := json.Unmarshal(event.Data, &eventData); err == nil {
			if exception, ok := eventData["exception"]; ok {
				sb.WriteString("\nStack trace:\n")
				sb.WriteString(extractStackTrace(exception))
			}
			if breadcrumbs, ok := eventData["breadcrumbs"]; ok {
				sb.WriteString("\nBreadcrumbs (last 10):\n")
				sb.WriteString(extractBreadcrumbs(breadcrumbs))
			}
			if request, ok := eventData["request"]; ok {
				sb.WriteString("\nRequest context:\n")
				sb.WriteString(string(request))
				sb.WriteString("\n")
			}
		}
	}

	return sb.String()
}

func extractStackTrace(data json.RawMessage) string {
	var exception struct {
		Values []struct {
			Type       string `json:"type"`
			Value      string `json:"value"`
			Stacktrace struct {
				Frames []struct {
					Filename string `json:"filename"`
					Function string `json:"function"`
					Lineno   int    `json:"lineno"`
					Context  []any  `json:"context_line"`
				} `json:"frames"`
			} `json:"stacktrace"`
		} `json:"values"`
	}
	if err := json.Unmarshal(data, &exception); err != nil {
		return string(data)
	}

	var sb strings.Builder
	for _, val := range exception.Values {
		sb.WriteString(fmt.Sprintf("%s: %s\n", val.Type, val.Value))
		frames := val.Stacktrace.Frames
		// Show top 10 frames (last 10 since Sentry frames are bottom-up)
		start := 0
		if len(frames) > 10 {
			start = len(frames) - 10
		}
		for i := len(frames) - 1; i >= start; i-- {
			f := frames[i]
			sb.WriteString(fmt.Sprintf("  at %s (%s:%d)\n", f.Function, f.Filename, f.Lineno))
		}
	}
	return sb.String()
}

func extractBreadcrumbs(data json.RawMessage) string {
	var bc struct {
		Values []struct {
			Category  string `json:"category"`
			Message   string `json:"message"`
			Level     string `json:"level"`
			Timestamp any    `json:"timestamp"`
		} `json:"values"`
	}
	if err := json.Unmarshal(data, &bc); err != nil {
		// Try flat array
		var flat []struct {
			Category  string `json:"category"`
			Message   string `json:"message"`
			Level     string `json:"level"`
		}
		if err2 := json.Unmarshal(data, &flat); err2 != nil {
			return ""
		}
		var sb strings.Builder
		start := 0
		if len(flat) > 10 {
			start = len(flat) - 10
		}
		for _, b := range flat[start:] {
			sb.WriteString(fmt.Sprintf("  [%s] %s: %s\n", b.Level, b.Category, b.Message))
		}
		return sb.String()
	}

	var sb strings.Builder
	vals := bc.Values
	start := 0
	if len(vals) > 10 {
		start = len(vals) - 10
	}
	for _, b := range vals[start:] {
		sb.WriteString(fmt.Sprintf("  [%s] %s: %s\n", b.Level, b.Category, b.Message))
	}
	return sb.String()
}

// GenerateDescriptionForManualTicket generates a description for a ticket with no linked issue.
func GenerateDescriptionForManualTicket(ctx context.Context, svc *Service, projectID uuid.UUID, title, existingDescription string) (string, error) {
	prompt := fmt.Sprintf("Ticket title: %s\n", title)
	if existingDescription != "" {
		prompt += fmt.Sprintf("Existing notes: %s\n", existingDescription)
	}
	prompt += "\nGenerate a structured ticket description template with sections for: Summary, Steps to Reproduce, Expected Behavior, Actual Behavior, and Possible Fix."

	resp, err := svc.Chat(ctx, projectID, "description", ChatRequest{
		SystemPrompt: descriptionSystemPrompt,
		Messages: []Message{
			{Role: "user", Content: prompt},
		},
		MaxTokens:   1500,
		Temperature: 0.3,
	})
	if err != nil {
		return "", err
	}

	return SanitizeHTML(resp.Content), nil
}

