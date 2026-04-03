package alert

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/darkspock/gosnag/internal/database/db"
)

type SlackConfig struct {
	WebhookURL string `json:"webhook_url"`
	Channel    string `json:"channel,omitempty"`
}

type slackMessage struct {
	Text        string            `json:"text"`
	Channel     string            `json:"channel,omitempty"`
	Attachments []slackAttachment `json:"attachments,omitempty"`
}

type slackAttachment struct {
	Color  string `json:"color"`
	Title  string `json:"title"`
	Text   string `json:"text"`
	Footer string `json:"footer"`
	Ts     int64  `json:"ts"`
}

func (s *Service) sendSlack(cfg SlackConfig, project db.Project, issue db.Issue, action string) {
	webhookURL := cfg.WebhookURL
	if webhookURL == "" {
		webhookURL = s.cfg.SlackWebhookURL
	}
	if webhookURL == "" {
		return
	}

	color := "#e74c3c" // red for errors
	switch issue.Level {
	case "warning":
		color = "#f39c12"
	case "info":
		color = "#3498db"
	case "debug":
		color = "#95a5a6"
	}

	issueURL := fmt.Sprintf("%s/projects/%s/issues/%s", s.cfg.BaseURL, project.ID.String(), issue.ID.String())

	msg := slackMessage{
		Text:    fmt.Sprintf("*%s* in *%s*", action, project.Name),
		Channel: cfg.Channel,
		Attachments: []slackAttachment{
			{
				Color:  color,
				Title:  issue.Title,
				Text:   fmt.Sprintf("Level: %s | Events: %d\n<%s|View in GoSnag>", issue.Level, issue.EventCount, issueURL),
				Footer: "GoSnag",
				Ts:     issue.LastSeen.Unix(),
			},
		},
	}

	payload, err := json.Marshal(msg)
	if err != nil {
		slog.Error("failed to marshal slack message", "error", err)
		return
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Post(webhookURL, "application/json", bytes.NewReader(payload))
	if err != nil {
		slog.Error("failed to send slack alert", "error", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		slog.Error("slack webhook returned non-200", "status", resp.StatusCode)
	} else {
		slog.Info("slack alert sent", "issue", issue.Title)
	}
}
