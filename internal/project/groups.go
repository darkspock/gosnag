package project

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/darkspock/gosnag/internal/database/db"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func (h *Handler) ListGroups(w http.ResponseWriter, r *http.Request) {
	groups, err := h.queries.ListProjectGroups(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list groups")
		return
	}
	out := make([]SafeGroup, 0, len(groups))
	for _, group := range groups {
		out = append(out, toSafeGroup(group))
	}
	writeJSON(w, http.StatusOK, out)
}

type GroupRequest struct {
	Name                     string  `json:"name"`
	DefaultSlackWebhookURL   *string `json:"default_slack_webhook_url,omitempty"`
	ClearDefaultSlackWebhook bool    `json:"clear_default_slack_webhook_url,omitempty"`
}

type SafeGroup struct {
	ID                        uuid.UUID `json:"id"`
	Name                      string    `json:"name"`
	Position                  int32     `json:"position"`
	CreatedAt                 string    `json:"created_at"`
	DefaultSlackWebhookURLSet bool      `json:"default_slack_webhook_url_set"`
}

func toSafeGroup(g db.ProjectGroup) SafeGroup {
	return SafeGroup{
		ID:                        g.ID,
		Name:                      g.Name,
		Position:                  g.Position,
		CreatedAt:                 g.CreatedAt.Format(time.RFC3339),
		DefaultSlackWebhookURLSet: g.DefaultSlackWebhookUrl != "",
	}
}

func normalizeSlackWebhook(raw *string) string {
	if raw == nil {
		return ""
	}
	return strings.TrimSpace(*raw)
}

func (h *Handler) CreateGroup(w http.ResponseWriter, r *http.Request) {
	var req GroupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}

	group, err := h.queries.CreateProjectGroup(r.Context(), db.CreateProjectGroupParams{
		Name:                   req.Name,
		DefaultSlackWebhookUrl: normalizeSlackWebhook(req.DefaultSlackWebhookURL),
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create group")
		return
	}
	writeJSON(w, http.StatusCreated, toSafeGroup(group))
}

func (h *Handler) UpdateGroup(w http.ResponseWriter, r *http.Request) {
	groupID, err := uuid.Parse(chi.URLParam(r, "group_id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid group id")
		return
	}

	var req GroupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}

	existing, err := h.queries.GetProjectGroup(r.Context(), groupID)
	if err != nil {
		writeError(w, http.StatusNotFound, "group not found")
		return
	}

	webhook := existing.DefaultSlackWebhookUrl
	switch {
	case req.ClearDefaultSlackWebhook:
		webhook = ""
	case req.DefaultSlackWebhookURL != nil:
		webhook = normalizeSlackWebhook(req.DefaultSlackWebhookURL)
	}

	group, err := h.queries.UpdateProjectGroup(r.Context(), db.UpdateProjectGroupParams{
		ID:                     groupID,
		Name:                   req.Name,
		DefaultSlackWebhookUrl: webhook,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update group")
		return
	}
	writeJSON(w, http.StatusOK, toSafeGroup(group))
}

func (h *Handler) DeleteGroup(w http.ResponseWriter, r *http.Request) {
	groupID, err := uuid.Parse(chi.URLParam(r, "group_id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid group id")
		return
	}

	err = h.queries.DeleteProjectGroup(r.Context(), groupID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete group")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}
