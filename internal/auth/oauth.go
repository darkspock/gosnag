package auth

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/darkspock/gosnag/internal/config"
	"github.com/darkspock/gosnag/internal/database/db"
)

type OAuthHandler struct {
	queries    *db.Queries
	clientID   string
	baseURL    string
	sessionTTL time.Duration
	httpClient *http.Client
}

func NewOAuthHandler(queries *db.Queries, cfg *config.Config) *OAuthHandler {
	return &OAuthHandler{
		queries:    queries,
		clientID:   cfg.GoogleClientID,
		baseURL:    cfg.BaseURL,
		sessionTTL: 7 * 24 * time.Hour, // 7 days
		httpClient: &http.Client{Timeout: 5 * time.Second},
	}
}

type GoogleTokenInfo struct {
	Sub           string `json:"sub"`
	Email         string `json:"email"`
	EmailVerified string `json:"email_verified"`
	Name          string `json:"name"`
	Picture       string `json:"picture"`
	Aud           string `json:"aud"`
}

// AuthConfig returns the Google client ID for the frontend GIS integration.
func (h *OAuthHandler) AuthConfig(w http.ResponseWriter, r *http.Request) {
	if h.clientID == "" {
		http.Error(w, `{"error":"google auth not configured"}`, http.StatusServiceUnavailable)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"google_client_id": h.clientID,
	})
}

// TokenLogin verifies a Google ID token from the frontend and creates a session.
func (h *OAuthHandler) TokenLogin(w http.ResponseWriter, r *http.Request) {
	if h.clientID == "" {
		http.Error(w, `{"error":"google auth not configured"}`, http.StatusServiceUnavailable)
		return
	}

	var body struct {
		Credential string `json:"credential"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Credential == "" {
		http.Error(w, `{"error":"missing credential"}`, http.StatusBadRequest)
		return
	}

	// Verify token with Google
	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, "https://oauth2.googleapis.com/tokeninfo?id_token="+body.Credential, nil)
	if err != nil {
		slog.Error("failed to create google token verification request", "error", err)
		http.Error(w, `{"error":"token verification failed"}`, http.StatusInternalServerError)
		return
	}

	resp, err := h.httpClient.Do(req)
	if err != nil {
		slog.Error("failed to verify google token", "error", err)
		http.Error(w, `{"error":"token verification failed"}`, http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		http.Error(w, `{"error":"invalid token"}`, http.StatusUnauthorized)
		return
	}

	var tokenInfo GoogleTokenInfo
	if err := json.NewDecoder(resp.Body).Decode(&tokenInfo); err != nil {
		http.Error(w, `{"error":"failed to parse token"}`, http.StatusInternalServerError)
		return
	}

	// Verify audience matches our client ID
	if tokenInfo.Aud != h.clientID {
		slog.Warn("token audience mismatch", "expected", h.clientID, "got", tokenInfo.Aud)
		http.Error(w, `{"error":"invalid token audience"}`, http.StatusUnauthorized)
		return
	}

	if tokenInfo.Email == "" || tokenInfo.EmailVerified != "true" {
		slog.Warn("token rejected: unverified email", "email", tokenInfo.Email)
		http.Error(w, `{"error":"unverified email"}`, http.StatusUnauthorized)
		return
	}

	// Check if this is the first user (auto-create admin)
	userCount, _ := h.queries.CountUsers(r.Context())

	var user db.User

	if userCount == 0 {
		// First user: auto-create as admin
		user, err = h.queries.UpsertUserByGoogle(r.Context(), db.UpsertUserByGoogleParams{
			Email:     tokenInfo.Email,
			Name:      tokenInfo.Name,
			GoogleID:  toNullString(tokenInfo.Sub),
			AvatarUrl: tokenInfo.Picture,
		})
		if err != nil {
			slog.Error("failed to create first user", "error", err)
			http.Error(w, `{"error":"failed to create user"}`, http.StatusInternalServerError)
			return
		}
		user, _ = h.queries.UpdateUserRole(r.Context(), db.UpdateUserRoleParams{ID: user.ID, Role: "admin"})
		user, _ = h.queries.UpdateUserStatus(r.Context(), db.UpdateUserStatusParams{ID: user.ID, Status: "active"})
		slog.Info("first user created as admin", "email", user.Email)
	} else {
		// Invite-only: user must already exist
		existing, err := h.queries.GetUserByEmail(r.Context(), tokenInfo.Email)
		if err != nil {
			slog.Warn("login rejected: user not invited", "email", tokenInfo.Email)
			http.Error(w, `{"error":"not invited"}`, http.StatusForbidden)
			return
		}

		if existing.Status == "disabled" {
			slog.Warn("login rejected: user disabled", "email", tokenInfo.Email)
			http.Error(w, `{"error":"account disabled"}`, http.StatusForbidden)
			return
		}

		if existing.Status == "invited" {
			// First login: activate and fill in Google profile data
			user, err = h.queries.ActivateUser(r.Context(), db.ActivateUserParams{
				ID:        existing.ID,
				Name:      tokenInfo.Name,
				GoogleID:  toNullString(tokenInfo.Sub),
				AvatarUrl: tokenInfo.Picture,
			})
			if err != nil {
				slog.Error("failed to activate user", "error", err)
				http.Error(w, `{"error":"failed to activate user"}`, http.StatusInternalServerError)
				return
			}
			slog.Info("user activated on first login", "email", user.Email)
		} else {
			// Existing active user: update profile info
			user, err = h.queries.UpsertUserByGoogle(r.Context(), db.UpsertUserByGoogleParams{
				Email:     tokenInfo.Email,
				Name:      tokenInfo.Name,
				GoogleID:  toNullString(tokenInfo.Sub),
				AvatarUrl: tokenInfo.Picture,
			})
			if err != nil {
				slog.Error("failed to update user", "error", err)
				http.Error(w, `{"error":"failed to update user"}`, http.StatusInternalServerError)
				return
			}
		}
	}

	// Create session
	sessionToken := generateToken(32)
	_, err = h.queries.CreateSession(r.Context(), db.CreateSessionParams{
		Token:     sessionToken,
		UserID:    user.ID,
		ExpiresAt: time.Now().Add(h.sessionTTL),
	})
	if err != nil {
		slog.Error("failed to create session", "error", err)
		http.Error(w, `{"error":"failed to create session"}`, http.StatusInternalServerError)
		return
	}

	// Set session cookie
	http.SetCookie(w, sessionCookie(r, h.baseURL, sessionToken, int(h.sessionTTL.Seconds())))

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(user)
}

// Logout destroys the session.
func (h *OAuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("session")
	if err == nil {
		h.queries.DeleteSession(r.Context(), cookie.Value)
	}

	clearSessionCookie(w, r, h.baseURL)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, `{"ok":true}`)
}

// Me returns the current authenticated user.
func (h *OAuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, `{"error":"not authenticated"}`, http.StatusUnauthorized)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(user)
}
