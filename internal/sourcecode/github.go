package sourcecode

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// GitHubProvider implements Provider for GitHub repositories.
type GitHubProvider struct {
	cfg Config
}

func (g *GitHubProvider) FileURL(path string, line int, commitOrBranch string) string {
	ref := commitOrBranch
	if ref == "" {
		ref = g.cfg.DefaultBranch
	}
	cleanPath := g.cfg.StripPath(path)
	u := fmt.Sprintf("https://github.com/%s/%s/blob/%s/%s", g.cfg.Owner, g.cfg.Name, ref, cleanPath)
	if line > 0 {
		u += fmt.Sprintf("#L%d", line)
	}
	return u
}

func (g *GitHubProvider) GetCommitsForFiles(ctx context.Context, files []string, since time.Time) ([]Commit, error) {
	seen := map[string]bool{}
	var commits []Commit

	for _, file := range files {
		cleanPath := g.cfg.StripPath(file)
		params := url.Values{}
		params.Set("path", cleanPath)
		params.Set("since", since.Format(time.RFC3339))
		params.Set("per_page", "5")

		body, err := g.apiGet(ctx, fmt.Sprintf("/repos/%s/%s/commits?%s", g.cfg.Owner, g.cfg.Name, params.Encode()))
		if err != nil {
			continue
		}

		var ghCommits []struct {
			SHA    string `json:"sha"`
			HTMLURL string `json:"html_url"`
			Commit struct {
				Message string `json:"message"`
				Author  struct {
					Name  string    `json:"name"`
					Email string    `json:"email"`
					Date  time.Time `json:"date"`
				} `json:"author"`
			} `json:"commit"`
		}
		if err := json.Unmarshal(body, &ghCommits); err != nil {
			continue
		}

		for _, gc := range ghCommits {
			if seen[gc.SHA] {
				// Add this file to existing commit
				for i := range commits {
					if commits[i].SHA == gc.SHA {
						commits[i].Files = append(commits[i].Files, cleanPath)
						break
					}
				}
				continue
			}
			seen[gc.SHA] = true
			commits = append(commits, Commit{
				SHA:       gc.SHA,
				Message:   firstLine(gc.Commit.Message),
				Author:    gc.Commit.Author.Name,
				Email:     gc.Commit.Author.Email,
				Timestamp: gc.Commit.Author.Date,
				URL:       gc.HTMLURL,
				Files:     []string{cleanPath},
			})
		}
	}

	return commits, nil
}

func (g *GitHubProvider) ResolveRef(ctx context.Context, ref string) (string, error) {
	// Try as tag first
	body, err := g.apiGet(ctx, fmt.Sprintf("/repos/%s/%s/git/ref/tags/%s", g.cfg.Owner, g.cfg.Name, ref))
	if err == nil {
		var result struct {
			Object struct {
				SHA  string `json:"sha"`
				Type string `json:"type"`
			} `json:"object"`
		}
		if json.Unmarshal(body, &result) == nil {
			// If annotated tag, resolve the inner object
			if result.Object.Type == "tag" {
				body2, err := g.apiGet(ctx, fmt.Sprintf("/repos/%s/%s/git/tags/%s", g.cfg.Owner, g.cfg.Name, result.Object.SHA))
				if err == nil {
					var tag struct {
						Object struct {
							SHA string `json:"sha"`
						} `json:"object"`
					}
					if json.Unmarshal(body2, &tag) == nil {
						return tag.Object.SHA, nil
					}
				}
			}
			return result.Object.SHA, nil
		}
	}

	// Try as branch
	body, err = g.apiGet(ctx, fmt.Sprintf("/repos/%s/%s/git/ref/heads/%s", g.cfg.Owner, g.cfg.Name, ref))
	if err == nil {
		var result struct {
			Object struct {
				SHA string `json:"sha"`
			} `json:"object"`
		}
		if json.Unmarshal(body, &result) == nil {
			return result.Object.SHA, nil
		}
	}

	return "", fmt.Errorf("could not resolve ref '%s'", ref)
}

func (g *GitHubProvider) TestConnection(ctx context.Context) error {
	_, err := g.apiGet(ctx, fmt.Sprintf("/repos/%s/%s", g.cfg.Owner, g.cfg.Name))
	return err
}

func (g *GitHubProvider) apiGet(ctx context.Context, path string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", "https://api.github.com"+path, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+g.cfg.Token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := (&http.Client{Timeout: 15 * time.Second}).Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode == 401 || resp.StatusCode == 403 {
		return nil, fmt.Errorf("authentication failed")
	}
	if resp.StatusCode == 404 {
		return nil, fmt.Errorf("not found")
	}
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(body[:min(200, len(body))]))
	}
	return body, nil
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
