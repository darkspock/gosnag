package jira

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/darkspock/gosnag/internal/database/db"
)

// Config holds Jira connection settings from a project.
type Config struct {
	BaseURL    string
	Email      string
	APIToken   string
	ProjectKey string
	IssueType  string
}

// ConfigFromProject extracts Jira config from a project.
func ConfigFromProject(p db.Project) Config {
	return Config{
		BaseURL:    strings.TrimRight(p.JiraBaseUrl, "/"),
		Email:      p.JiraEmail,
		APIToken:   p.JiraApiToken,
		ProjectKey: p.JiraProjectKey,
		IssueType:  p.JiraIssueType,
	}
}

// IsConfigured returns true if the project has Jira configured.
func (c Config) IsConfigured() bool {
	return c.BaseURL != "" && c.Email != "" && c.APIToken != "" && c.ProjectKey != ""
}

// CreateIssueResult holds the result of creating a Jira issue.
type CreateIssueResult struct {
	Key  string `json:"key"`
	ID   string `json:"id"`
	Self string `json:"self"`
	URL  string `json:"url"`
}

// CreateIssue creates a new issue in Jira.
func CreateIssue(cfg Config, summary, description string) (*CreateIssueResult, error) {
	payload := map[string]any{
		"fields": map[string]any{
			"project": map[string]string{
				"key": cfg.ProjectKey,
			},
			"summary":   summary,
			"issuetype": map[string]string{"name": cfg.IssueType},
			"description": map[string]any{
				"type":    "doc",
				"version": 1,
				"content": []any{
					map[string]any{
						"type": "paragraph",
						"content": []any{
							map[string]any{
								"type": "text",
								"text": description,
							},
						},
					},
				},
			},
		},
	}

	body, _ := json.Marshal(payload)
	req, err := http.NewRequest("POST", cfg.BaseURL+"/rest/api/3/issue", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.SetBasicAuth(cfg.Email, cfg.APIToken)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to call Jira API: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("jira API error %d: %s", resp.StatusCode, string(respBody))
	}

	var result CreateIssueResult
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to parse Jira response: %w", err)
	}

	result.URL = cfg.BaseURL + "/browse/" + result.Key
	return &result, nil
}

// TestConnection verifies that the Jira config is valid by fetching the project.
func TestConnection(cfg Config) error {
	req, err := http.NewRequest("GET", cfg.BaseURL+"/rest/api/3/project/"+cfg.ProjectKey, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.SetBasicAuth(cfg.Email, cfg.APIToken)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("connection failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 401 {
		return fmt.Errorf("authentication failed: check email and API token")
	}
	if resp.StatusCode == 404 {
		return fmt.Errorf("project '%s' not found", cfg.ProjectKey)
	}
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// BuildDescription creates a formatted description for a Jira ticket from a GoSnag issue.
func BuildDescription(issue db.Issue, baseURL, projectID string, latestStacktrace string) string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("Level: %s\n", issue.Level))
	b.WriteString(fmt.Sprintf("Platform: %s\n", issue.Platform))
	b.WriteString(fmt.Sprintf("Events: %d\n", issue.EventCount))
	b.WriteString(fmt.Sprintf("First seen: %s\n", issue.FirstSeen.Format(time.RFC3339)))
	b.WriteString(fmt.Sprintf("Last seen: %s\n", issue.LastSeen.Format(time.RFC3339)))
	b.WriteString(fmt.Sprintf("\nGoSnag: %s/projects/%s/issues/%s\n", baseURL, projectID, issue.ID))

	if latestStacktrace != "" {
		b.WriteString(fmt.Sprintf("\n--- Stacktrace ---\n%s\n", latestStacktrace))
	}

	return b.String()
}
