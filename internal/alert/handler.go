package alert

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/darkspock/gosnag/internal/database/db"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type Handler struct {
	queries *db.Queries
}

func NewHandler(queries *db.Queries) *Handler {
	return &Handler{queries: queries}
}

type CreateAlertRequest struct {
	AlertType      string          `json:"alert_type"`
	Config         json.RawMessage `json:"config"`
	Enabled        bool            `json:"enabled"`
	LevelFilter    string          `json:"level_filter"`
	TitlePattern   string          `json:"title_pattern"`
	MinEvents      int32           `json:"min_events"`
	MinVelocity1h  int32           `json:"min_velocity_1h"`
	ExcludePattern string          `json:"exclude_pattern"`
}

type UpdateAlertRequest struct {
	Config         json.RawMessage `json:"config"`
	Enabled        bool            `json:"enabled"`
	LevelFilter    string          `json:"level_filter"`
	TitlePattern   string          `json:"title_pattern"`
	MinEvents      int32           `json:"min_events"`
	MinVelocity1h  int32           `json:"min_velocity_1h"`
	ExcludePattern string          `json:"exclude_pattern"`
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	projectID, err := uuid.Parse(chi.URLParam(r, "project_id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid project id")
		return
	}

	configs, err := h.queries.ListAlertConfigs(r.Context(), projectID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list alert configs")
		return
	}

	writeJSON(w, http.StatusOK, configs)
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	projectID, err := uuid.Parse(chi.URLParam(r, "project_id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid project id")
		return
	}

	var req CreateAlertRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	switch req.AlertType {
	case "email", "slack":
	default:
		writeError(w, http.StatusBadRequest, "alert_type must be 'email' or 'slack'")
		return
	}

	if err := validateAlertConfig(req.AlertType, req.Config); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	config, err := h.queries.CreateAlertConfig(r.Context(), db.CreateAlertConfigParams{
		ProjectID:      projectID,
		AlertType:      req.AlertType,
		Config:         req.Config,
		Enabled:        req.Enabled,
		LevelFilter:    req.LevelFilter,
		TitlePattern:   req.TitlePattern,
		MinEvents:      req.MinEvents,
		MinVelocity1h:  req.MinVelocity1h,
		ExcludePattern: req.ExcludePattern,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create alert config")
		return
	}

	writeJSON(w, http.StatusCreated, config)
}

func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	projectID, err := uuid.Parse(chi.URLParam(r, "project_id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid project id")
		return
	}
	alertID, err := uuid.Parse(chi.URLParam(r, "alert_id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid alert id")
		return
	}

	var req UpdateAlertRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Look up existing to get alert_type for config validation
	existing, err := h.queries.ListAlertConfigs(r.Context(), projectID)
	if err == nil {
		for _, a := range existing {
			if a.ID == alertID {
				if err := validateAlertConfig(a.AlertType, req.Config); err != nil {
					writeError(w, http.StatusBadRequest, err.Error())
					return
				}
				break
			}
		}
	}

	config, err := h.queries.UpdateAlertConfig(r.Context(), db.UpdateAlertConfigParams{
		ID:             alertID,
		ProjectID:      projectID,
		Config:         req.Config,
		Enabled:        req.Enabled,
		LevelFilter:    req.LevelFilter,
		TitlePattern:   req.TitlePattern,
		MinEvents:      req.MinEvents,
		MinVelocity1h:  req.MinVelocity1h,
		ExcludePattern: req.ExcludePattern,
	})
	if err != nil {
		if err == sql.ErrNoRows {
			writeError(w, http.StatusNotFound, "alert config not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to update alert config")
		return
	}

	writeJSON(w, http.StatusOK, config)
}

func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	projectID, err := uuid.Parse(chi.URLParam(r, "project_id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid project id")
		return
	}
	alertID, err := uuid.Parse(chi.URLParam(r, "alert_id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid alert id")
		return
	}

	if err := h.queries.DeleteAlertConfig(r.Context(), db.DeleteAlertConfigParams{ID: alertID, ProjectID: projectID}); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete alert config")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func validateAlertConfig(alertType string, raw json.RawMessage) error {
	switch alertType {
	case "email":
		var cfg struct {
			Recipients []string `json:"recipients"`
		}
		if err := json.Unmarshal(raw, &cfg); err != nil {
			return fmt.Errorf("invalid email config: %w", err)
		}
		if len(cfg.Recipients) == 0 || cfg.Recipients[0] == "" {
			return fmt.Errorf("at least one recipient email is required")
		}
	case "slack":
		var cfg struct {
			WebhookURL string `json:"webhook_url"`
		}
		if err := json.Unmarshal(raw, &cfg); err != nil {
			return fmt.Errorf("invalid slack config: %w", err)
		}
		if cfg.WebhookURL == "" {
			return fmt.Errorf("webhook URL is required")
		}
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
