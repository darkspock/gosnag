package auth

import (
	"context"
	"database/sql"
	"net/http"

	"github.com/darkspock/gosnag/internal/database/db"
)

type contextKey string

const userContextKey contextKey = "user"

// Middleware validates the session cookie and injects the user into context.
func Middleware(queries *db.Queries, baseURL string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie("session")
			if err != nil {
				http.Error(w, `{"error":"not authenticated"}`, http.StatusUnauthorized)
				return
			}

			session, err := queries.GetSession(r.Context(), cookie.Value)
			if err != nil {
				if err == sql.ErrNoRows {
					clearSessionCookie(w, r, baseURL)
					http.Error(w, `{"error":"session expired"}`, http.StatusUnauthorized)
					return
				}
				http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
				return
			}

			user, err := queries.GetUser(r.Context(), session.UserID)
			if err != nil {
				clearSessionCookie(w, r, baseURL)
				http.Error(w, `{"error":"user not found"}`, http.StatusUnauthorized)
				return
			}

			if user.Status != "active" {
				_ = queries.DeleteSession(r.Context(), cookie.Value)
				clearSessionCookie(w, r, baseURL)
				http.Error(w, `{"error":"account not active"}`, http.StatusUnauthorized)
				return
			}

			ctx := context.WithValue(r.Context(), userContextKey, &user)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireAdmin checks that the authenticated user has admin role.
func RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := GetUserFromContext(r.Context())
		if user == nil || user.Role != "admin" {
			http.Error(w, `{"error":"admin access required"}`, http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// GetUserFromContext extracts the user from the request context.
func GetUserFromContext(ctx context.Context) *db.User {
	user, ok := ctx.Value(userContextKey).(*db.User)
	if !ok {
		return nil
	}
	return user
}
