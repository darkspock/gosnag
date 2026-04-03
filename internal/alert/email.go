package alert

import (
	"fmt"
	"log/slog"
	"net/smtp"
	"strings"

	"github.com/darkspock/gosnag/internal/database/db"
)

type EmailConfig struct {
	Recipients []string `json:"recipients"`
}

func (s *Service) sendEmail(cfg EmailConfig, project db.Project, issue db.Issue, action string) {
	if s.cfg.SMTPHost == "" || len(cfg.Recipients) == 0 {
		return
	}

	subject := fmt.Sprintf("[GoSnag] %s in %s: %s", action, project.Name, issue.Title)
	body := fmt.Sprintf(
		"Project: %s\nIssue: %s\nLevel: %s\nStatus: %s\nEvents: %d\nFirst seen: %s\nLast seen: %s\n\nView: %s/projects/%s/issues/%s",
		project.Name,
		issue.Title,
		issue.Level,
		issue.Status,
		issue.EventCount,
		issue.FirstSeen.Format("2006-01-02 15:04:05 UTC"),
		issue.LastSeen.Format("2006-01-02 15:04:05 UTC"),
		s.cfg.BaseURL,
		project.ID.String(),
		issue.ID.String(),
	)

	msg := fmt.Sprintf(
		"From: %s\r\nTo: %s\r\nSubject: %s\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n%s",
		s.cfg.SMTPFrom,
		strings.Join(cfg.Recipients, ", "),
		subject,
		body,
	)

	addr := fmt.Sprintf("%s:%d", s.cfg.SMTPHost, s.cfg.SMTPPort)

	var auth smtp.Auth
	if s.cfg.SMTPUser != "" {
		auth = smtp.PlainAuth("", s.cfg.SMTPUser, s.cfg.SMTPPassword, s.cfg.SMTPHost)
	}

	if err := smtp.SendMail(addr, auth, s.cfg.SMTPFrom, cfg.Recipients, []byte(msg)); err != nil {
		slog.Error("failed to send email alert", "error", err, "recipients", cfg.Recipients)
	} else {
		slog.Info("email alert sent", "recipients", cfg.Recipients, "issue", issue.Title)
	}
}
