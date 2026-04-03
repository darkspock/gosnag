package alert

import (
	"database/sql"
	"encoding/json"
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
	AlertType    string          `json:"alert_type"`
	Config       json.RawMessage `json:"config"`
	Enabled      bool            `json:"enabled"`
	LevelFilter  string          `json:"level_filter"`
	TitlePattern string          `json:"title_pattern"`
}

type UpdateAlertRequest struct {
	Config       json.RawMessage `json:"config"`
	Enabled      bool            `json:"enabled"`
	LevelFilter  string          `json:"level_filter"`
	TitlePattern string          `json:"title_pattern"`
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

	config, err := h.queries.CreateAlertConfig(r.Context(), db.CreateAlertConfigParams{
		ProjectID:    projectID,
		AlertType:    req.AlertType,
		Config:       req.Config,
		Enabled:      req.Enabled,
		LevelFilter:  req.LevelFilter,
		TitlePattern: req.TitlePattern,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create alert config")
		return
	}

	writeJSON(w, http.StatusCreated, config)
}

func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
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

	config, err := h.queries.UpdateAlertConfig(r.Context(), db.UpdateAlertConfigParams{
		ID:           alertID,
		Config:       req.Config,
		Enabled:      req.Enabled,
		LevelFilter:  req.LevelFilter,
		TitlePattern: req.TitlePattern,
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
	alertID, err := uuid.Parse(chi.URLParam(r, "alert_id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid alert id")
		return
	}

	if err := h.queries.DeleteAlertConfig(r.Context(), alertID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete alert config")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
