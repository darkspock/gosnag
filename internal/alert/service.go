package alert

import (
	"context"
	"encoding/json"
	"log/slog"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/darkspock/gosnag/internal/config"
	"github.com/darkspock/gosnag/internal/database/db"
	"github.com/google/uuid"
)

type Service struct {
	queries  *db.Queries
	cfg      *config.Config
	debounce map[string]time.Time // issue_id -> last alert time
	mu       sync.Mutex
}

func NewService(queries *db.Queries, cfg *config.Config) *Service {
	return &Service{
		queries:  queries,
		cfg:      cfg,
		debounce: make(map[string]time.Time),
	}
}

// matchesAlert checks if an issue matches the alert's level and title filters.
func matchesAlert(ac db.AlertConfig, issue db.Issue) bool {
	// Level filter: comma-separated list of levels, empty = all
	if ac.LevelFilter != "" {
		levels := strings.Split(ac.LevelFilter, ",")
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

	// Title pattern: regex match, empty = all
	if ac.TitlePattern != "" {
		re, err := regexp.Compile("(?i)" + ac.TitlePattern)
		if err != nil {
			// Invalid regex — treat as literal substring match
			if !strings.Contains(strings.ToLower(issue.Title), strings.ToLower(ac.TitlePattern)) {
				return false
			}
		} else if !re.MatchString(issue.Title) {
			return false
		}
	}

	return true
}

// Notify sends alerts for a new or reopened issue.
func (s *Service) Notify(projectID uuid.UUID, issue db.Issue, isNew bool) {
	// Debounce: don't alert more than once per 5 minutes per issue
	s.mu.Lock()
	key := issue.ID.String()
	if last, ok := s.debounce[key]; ok && time.Since(last) < 5*time.Minute {
		s.mu.Unlock()
		return
	}
	s.debounce[key] = time.Now()
	s.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	configs, err := s.queries.GetEnabledAlerts(ctx, projectID)
	if err != nil {
		slog.Error("failed to get alert configs", "error", err, "project_id", projectID)
		return
	}

	project, err := s.queries.GetProject(ctx, projectID)
	if err != nil {
		slog.Error("failed to get project for alert", "error", err, "project_id", projectID)
		return
	}

	action := "New issue"
	if !isNew {
		action = "Reopened issue"
	}

	for _, ac := range configs {
		if !matchesAlert(ac, issue) {
			continue
		}

		switch ac.AlertType {
		case "email":
			var emailCfg EmailConfig
			if err := json.Unmarshal(ac.Config, &emailCfg); err != nil {
				slog.Error("invalid email alert config", "error", err)
				continue
			}
			go s.sendEmail(emailCfg, project, issue, action)

		case "slack":
			var slackCfg SlackConfig
			if err := json.Unmarshal(ac.Config, &slackCfg); err != nil {
				slog.Error("invalid slack alert config", "error", err)
				continue
			}
			go s.sendSlack(slackCfg, project, issue, action)
		}
	}
}

// CleanupDebounce removes old debounce entries. Call periodically.
func (s *Service) CleanupDebounce() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for k, v := range s.debounce {
		if time.Since(v) > 30*time.Minute {
			delete(s.debounce, k)
		}
	}
}
