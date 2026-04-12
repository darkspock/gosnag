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

// BitbucketProvider implements Provider for Bitbucket Cloud repositories.
type BitbucketProvider struct {
	cfg Config
}

func (b *BitbucketProvider) FileURL(path string, line int, commitOrBranch string) string {
	ref := commitOrBranch
	if ref == "" {
		ref = b.cfg.DefaultBranch
	}
	cleanPath := b.cfg.StripPath(path)
	u := fmt.Sprintf("https://bitbucket.org/%s/%s/src/%s/%s", b.cfg.Owner, b.cfg.Name, ref, cleanPath)
	if line > 0 {
		u += fmt.Sprintf("#lines-%d", line)
	}
	return u
}

func (b *BitbucketProvider) GetCommitsForFiles(ctx context.Context, files []string, since time.Time) ([]Commit, error) {
	seen := map[string]bool{}
	var commits []Commit

	for _, file := range files {
		cleanPath := b.cfg.StripPath(file)
		params := url.Values{}
		params.Set("path", cleanPath)
		params.Set("pagelen", "5")

		body, err := b.apiGet(ctx, fmt.Sprintf("/2.0/repositories/%s/%s/commits?%s", b.cfg.Owner, b.cfg.Name, params.Encode()))
		if err != nil {
			continue
		}

		var result struct {
			Values []struct {
				Hash    string `json:"hash"`
				Message string `json:"message"`
				Date    string `json:"date"`
				Author  struct {
					Raw string `json:"raw"`
				} `json:"author"`
				Links struct {
					HTML struct {
						Href string `json:"href"`
					} `json:"html"`
				} `json:"links"`
			} `json:"values"`
		}
		if err := json.Unmarshal(body, &result); err != nil {
			continue
		}

		for _, bc := range result.Values {
			t, _ := time.Parse(time.RFC3339, bc.Date)
			if !since.IsZero() && t.Before(since) {
				continue
			}
			if seen[bc.Hash] {
				for i := range commits {
					if commits[i].SHA == bc.Hash {
						commits[i].Files = append(commits[i].Files, cleanPath)
						break
					}
				}
				continue
			}
			seen[bc.Hash] = true
			author, email := parseAuthorRaw(bc.Author.Raw)
			commits = append(commits, Commit{
				SHA:       bc.Hash,
				Message:   firstLineBB(bc.Message),
				Author:    author,
				Email:     email,
				Timestamp: t,
				URL:       bc.Links.HTML.Href,
				Files:     []string{cleanPath},
			})
		}
	}

	return commits, nil
}

func (b *BitbucketProvider) ResolveRef(ctx context.Context, ref string) (string, error) {
	// Try as tag
	body, err := b.apiGet(ctx, fmt.Sprintf("/2.0/repositories/%s/%s/refs/tags/%s", b.cfg.Owner, b.cfg.Name, ref))
	if err == nil {
		var result struct {
			Target struct {
				Hash string `json:"hash"`
			} `json:"target"`
		}
		if json.Unmarshal(body, &result) == nil && result.Target.Hash != "" {
			return result.Target.Hash, nil
		}
	}

	// Try as branch
	body, err = b.apiGet(ctx, fmt.Sprintf("/2.0/repositories/%s/%s/refs/branches/%s", b.cfg.Owner, b.cfg.Name, ref))
	if err == nil {
		var result struct {
			Target struct {
				Hash string `json:"hash"`
			} `json:"target"`
		}
		if json.Unmarshal(body, &result) == nil && result.Target.Hash != "" {
			return result.Target.Hash, nil
		}
	}

	return "", fmt.Errorf("could not resolve ref '%s'", ref)
}

func (b *BitbucketProvider) TestConnection(ctx context.Context) error {
	_, err := b.apiGet(ctx, fmt.Sprintf("/2.0/repositories/%s/%s", b.cfg.Owner, b.cfg.Name))
	return err
}

func (b *BitbucketProvider) apiGet(ctx context.Context, path string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", "https://api.bitbucket.org"+path, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+b.cfg.Token)
	req.Header.Set("Accept", "application/json")

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
		return nil, fmt.Errorf("repository not found")
	}
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("API error %d", resp.StatusCode)
	}
	return body, nil
}

// parseAuthorRaw parses "Name <email>" format from Bitbucket.
func parseAuthorRaw(raw string) (string, string) {
	if idx := strings.Index(raw, " <"); idx >= 0 {
		name := raw[:idx]
		email := strings.TrimSuffix(strings.TrimPrefix(raw[idx+2:], ""), ">")
		return name, email
	}
	return raw, ""
}

func firstLineBB(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}
