package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strings"
	"time"

	"github.com/darkspock/gosnag/internal/database/db"
	"github.com/go-chi/chi/v5"
)

const (
	tokenPrefix     = "gsn_"
	tokenContextKey contextKey = "api_token"
)

// GenerateAPIToken creates a new random API token with the gsn_ prefix.
// Returns the plain token (to show the user once) and its SHA-256 hash (to store).
func GenerateAPIToken() (plain string, hash string) {
	b := make([]byte, 32)
	rand.Read(b)
	plain = tokenPrefix + hex.EncodeToString(b)
	hash = HashToken(plain)
	return
}

// HashToken returns the SHA-256 hex digest of a token string.
func HashToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}

// MiddlewareWithToken tries Bearer token auth first, then falls back to session cookie.
func MiddlewareWithToken(queries *db.Queries, baseURL string) func(http.Handler) http.Handler {
	sessionMw := Middleware(queries, baseURL)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			bearer := extractBearer(r)
			if bearer != "" && strings.HasPrefix(bearer, tokenPrefix) {
				hash := HashToken(bearer)
				token, err := queries.GetAPITokenByHash(r.Context(), hash)
				if err != nil {
					http.Error(w, `{"error":"invalid api token"}`, http.StatusUnauthorized)
					return
				}

				if token.ExpiresAt.Valid && time.Now().After(token.ExpiresAt.Time) {
					http.Error(w, `{"error":"api token expired"}`, http.StatusUnauthorized)
					return
				}

				// Personal tokens (scope=global): load the creator user into context
				// so RequireAdmin/RequireWritePermission work via user role
				if token.Scope == "global" {
					if !token.CreatedBy.Valid {
						http.Error(w, `{"error":"personal token has no associated user"}`, http.StatusForbidden)
						return
					}
					user, err := queries.GetUser(r.Context(), token.CreatedBy.UUID)
					if err != nil {
						http.Error(w, `{"error":"token owner not found"}`, http.StatusForbidden)
						return
					}
					if user.Status != "active" {
						http.Error(w, `{"error":"token owner is disabled"}`, http.StatusForbidden)
						return
					}
					go queries.UpdateAPITokenLastUsed(context.Background(), token.ID)
					ctx := r.Context()
					ctx = context.WithValue(ctx, tokenContextKey, &token)
					ctx = context.WithValue(ctx, userContextKey, &user)
					next.ServeHTTP(w, r.WithContext(ctx))
					return
				}

				// Project-scoped tokens: validate against the requested project
				projectID := chi.URLParam(r, "project_id")
				if projectID == "" {
					projectID = extractProjectIDFromPath(r.URL.Path)
				}
				if projectID == "" {
					http.Error(w, `{"error":"project-scoped tokens can only access project endpoints"}`, http.StatusForbidden)
					return
				}
				if !token.ProjectID.Valid || token.ProjectID.UUID.String() != projectID {
					http.Error(w, `{"error":"token not authorized for this project"}`, http.StatusForbidden)
					return
				}

				// Update last_used_at in background
				go queries.UpdateAPITokenLastUsed(context.Background(), token.ID)

				ctx := context.WithValue(r.Context(), tokenContextKey, &token)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}

			// Fall back to session auth
			sessionMw(next).ServeHTTP(w, r)
		})
	}
}

// GetAPITokenFromContext extracts the API token from context (nil if session auth).
func GetAPITokenFromContext(ctx context.Context) *db.ApiToken {
	t, ok := ctx.Value(tokenContextKey).(*db.ApiToken)
	if !ok {
		return nil
	}
	return t
}

// RequireWritePermission checks that API token has readwrite permission.
// Session-authenticated users always pass.
func RequireWritePermission(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := GetAPITokenFromContext(r.Context())
		if token != nil && token.Permission != "readwrite" {
			http.Error(w, `{"error":"token has read-only access"}`, http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// extractProjectIDFromPath extracts the project UUID from paths like /api/v1/projects/{uuid}/...
func extractProjectIDFromPath(path string) string {
	const prefix = "/api/v1/projects/"
	if !strings.HasPrefix(path, prefix) {
		return ""
	}
	rest := path[len(prefix):]
	// UUID is 36 chars
	if len(rest) < 36 {
		return ""
	}
	candidate := rest[:36]
	// Basic UUID validation (8-4-4-4-12 with hyphens)
	if len(candidate) == 36 && candidate[8] == '-' && candidate[13] == '-' && candidate[18] == '-' && candidate[23] == '-' {
		return candidate
	}
	return ""
}

func extractBearer(r *http.Request) string {
	h := r.Header.Get("Authorization")
	if strings.HasPrefix(h, "Bearer ") {
		return h[7:]
	}
	return ""
}
