package user

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

type UpdateRoleRequest struct {
	Role string `json:"role"`
}

type InviteRequest struct {
	Email string `json:"email"`
	Role  string `json:"role"`
}

type UpdateStatusRequest struct {
	Status string `json:"status"`
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	users, err := h.queries.ListUsers(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list users")
		return
	}
	writeJSON(w, http.StatusOK, users)
}

func (h *Handler) Invite(w http.ResponseWriter, r *http.Request) {
	var req InviteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Email == "" {
		writeError(w, http.StatusBadRequest, "email is required")
		return
	}

	if req.Role == "" {
		req.Role = "viewer"
	}
	switch req.Role {
	case "admin", "viewer":
	default:
		writeError(w, http.StatusBadRequest, "role must be 'admin' or 'viewer'")
		return
	}

	// Check if user already exists
	if _, err := h.queries.GetUserByEmail(r.Context(), req.Email); err == nil {
		writeError(w, http.StatusConflict, "user already exists")
		return
	}

	user, err := h.queries.CreateInvitedUser(r.Context(), db.CreateInvitedUserParams{
		Email: req.Email,
		Role:  req.Role,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to invite user")
		return
	}

	writeJSON(w, http.StatusCreated, user)
}

func (h *Handler) UpdateRole(w http.ResponseWriter, r *http.Request) {
	userID, err := uuid.Parse(chi.URLParam(r, "user_id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid user id")
		return
	}

	var req UpdateRoleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	switch req.Role {
	case "admin", "viewer":
	default:
		writeError(w, http.StatusBadRequest, "role must be 'admin' or 'viewer'")
		return
	}

	user, err := h.queries.UpdateUserRole(r.Context(), db.UpdateUserRoleParams{
		ID:   userID,
		Role: req.Role,
	})
	if err != nil {
		if err == sql.ErrNoRows {
			writeError(w, http.StatusNotFound, "user not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to update user role")
		return
	}

	writeJSON(w, http.StatusOK, user)
}

func (h *Handler) UpdateStatus(w http.ResponseWriter, r *http.Request) {
	userID, err := uuid.Parse(chi.URLParam(r, "user_id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid user id")
		return
	}

	var req UpdateStatusRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	switch req.Status {
	case "active", "disabled":
	default:
		writeError(w, http.StatusBadRequest, "status must be 'active' or 'disabled'")
		return
	}

	user, err := h.queries.UpdateUserStatus(r.Context(), db.UpdateUserStatusParams{
		ID:     userID,
		Status: req.Status,
	})
	if err != nil {
		if err == sql.ErrNoRows {
			writeError(w, http.StatusNotFound, "user not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to update user status")
		return
	}

	writeJSON(w, http.StatusOK, user)
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
