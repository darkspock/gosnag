package sourcecode

import (
	"context"
	"strings"
	"time"

	"github.com/darkspock/gosnag/internal/database/db"
)

// Provider abstracts source code hosting operations.
type Provider interface {
	// FileURL returns a direct link to a file and line in the repository.
	FileURL(path string, line int, commitOrBranch string) string

	// GetCommitsForFiles returns recent commits that touched any of the given files.
	GetCommitsForFiles(ctx context.Context, files []string, since time.Time) ([]Commit, error)

	// ResolveRef resolves a tag or branch name to a commit SHA.
	ResolveRef(ctx context.Context, ref string) (string, error)

	// TestConnection verifies the token and repo are accessible.
	TestConnection(ctx context.Context) error
}

type Commit struct {
	SHA       string    `json:"sha"`
	Message   string    `json:"message"`
	Author    string    `json:"author"`
	Email     string    `json:"email"`
	Timestamp time.Time `json:"timestamp"`
	URL       string    `json:"url"`
	Files     []string  `json:"files,omitempty"`
}

// Config holds source code provider configuration from a project.
type Config struct {
	Provider      string
	Owner         string
	Name          string
	DefaultBranch string
	Token         string
	PathStrip     string
}

// ConfigFromProject extracts source code config from a project.
func ConfigFromProject(p db.Project) Config {
	return Config{
		Provider:      p.RepoProvider,
		Owner:         p.RepoOwner,
		Name:          p.RepoName,
		DefaultBranch: p.RepoDefaultBranch,
		Token:         p.RepoToken,
		PathStrip:     p.RepoPathStrip,
	}
}

// IsConfigured returns true if the project has a repository configured.
func (c Config) IsConfigured() bool {
	return c.Provider != "" && c.Owner != "" && c.Name != "" && c.Token != ""
}

// NewProvider creates a provider from config.
func NewProvider(cfg Config) Provider {
	switch cfg.Provider {
	case "github":
		return &GitHubProvider{cfg: cfg}
	case "bitbucket":
		return &BitbucketProvider{cfg: cfg}
	default:
		return nil
	}
}

// StripPath removes the configured prefix from a runtime file path.
func (c Config) StripPath(runtimePath string) string {
	if c.PathStrip == "" {
		return runtimePath
	}
	return strings.TrimPrefix(runtimePath, c.PathStrip)
}

// IsLibraryPath returns true if the path looks like a third-party dependency.
func IsLibraryPath(path string) bool {
	patterns := []string{
		"node_modules/", "vendor/", "site-packages/", "lib/python",
		".gem/", "/usr/lib/", "/usr/local/lib/", "dist-packages/",
		"__pycache__/", ".tox/", "venv/", ".venv/",
	}
	lower := strings.ToLower(path)
	for _, p := range patterns {
		if strings.Contains(lower, p) {
			return true
		}
	}
	return false
}
