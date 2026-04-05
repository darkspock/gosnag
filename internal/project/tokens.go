package project

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"time"

	"github.com/darkspock/gosnag/internal/auth"
	"github.com/darkspock/gosnag/internal/database/db"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type CreateTokenRequest struct {
	Name       string `json:"name"`
	Permission string `json:"permission"` // "read" or "readwrite"
	ExpiresIn  *int   `json:"expires_in"` // days, optional
}

type TokenResponse struct {
	ID         uuid.UUID  `json:"id"`
	ProjectID  uuid.UUID  `json:"project_id"`
	Name       string     `json:"name"`
	Permission string     `json:"permission"`
	Token      string     `json:"token,omitempty"` // only on create
	LastUsedAt *time.Time `json:"last_used_at"`
	ExpiresAt  *time.Time `json:"expires_at"`
	CreatedAt  time.Time  `json:"created_at"`
}

func (h *Handler) ListTokens(w http.ResponseWriter, r *http.Request) {
	projectID, err := uuid.Parse(chi.URLParam(r, "projectId"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid project id")
		return
	}

	tokens, err := h.queries.ListAPITokensByProject(r.Context(), projectID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list tokens")
		return
	}

	resp := make([]TokenResponse, len(tokens))
	for i, t := range tokens {
		resp[i] = tokenToResponse(t)
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) CreateToken(w http.ResponseWriter, r *http.Request) {
	projectID, err := uuid.Parse(chi.URLParam(r, "projectId"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid project id")
		return
	}

	var req CreateTokenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}

	if req.Permission == "" {
		req.Permission = "read"
	}
	if req.Permission != "read" && req.Permission != "readwrite" {
		writeError(w, http.StatusBadRequest, "permission must be 'read' or 'readwrite'")
		return
	}

	plain, hash := auth.GenerateAPIToken()

	params := db.CreateAPITokenParams{
		ProjectID:  projectID,
		TokenHash:  hash,
		Name:       req.Name,
		Permission: req.Permission,
	}

	if req.ExpiresIn != nil && *req.ExpiresIn > 0 {
		exp := time.Now().AddDate(0, 0, *req.ExpiresIn)
		params.ExpiresAt = sql.NullTime{Time: exp, Valid: true}
	}

	user := auth.GetUserFromContext(r.Context())
	if user != nil {
		params.CreatedBy = uuid.NullUUID{UUID: user.ID, Valid: true}
	}

	token, err := h.queries.CreateAPIToken(r.Context(), params)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create token")
		return
	}

	resp := tokenToResponse(token)
	resp.Token = plain // show plain token only on creation
	writeJSON(w, http.StatusCreated, resp)
}

func (h *Handler) DeleteToken(w http.ResponseWriter, r *http.Request) {
	projectID, err := uuid.Parse(chi.URLParam(r, "projectId"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid project id")
		return
	}

	tokenID, err := uuid.Parse(chi.URLParam(r, "tokenId"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid token id")
		return
	}

	err = h.queries.DeleteAPIToken(r.Context(), db.DeleteAPITokenParams{
		ID:        tokenID,
		ProjectID: projectID,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete token")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func tokenToResponse(t db.ApiToken) TokenResponse {
	resp := TokenResponse{
		ID:         t.ID,
		ProjectID:  t.ProjectID,
		Name:       t.Name,
		Permission: t.Permission,
		CreatedAt:  t.CreatedAt,
	}
	if t.LastUsedAt.Valid {
		resp.LastUsedAt = &t.LastUsedAt.Time
	}
	if t.ExpiresAt.Valid {
		resp.ExpiresAt = &t.ExpiresAt.Time
	}
	return resp
}
