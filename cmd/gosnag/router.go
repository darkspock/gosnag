package main

import (
	"context"
	"database/sql"
	"io/fs"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/darkspock/gosnag/internal/alert"
	"github.com/darkspock/gosnag/internal/auth"
	"github.com/darkspock/gosnag/internal/config"
	"github.com/darkspock/gosnag/internal/database/db"
	"github.com/darkspock/gosnag/internal/ingest"
	"github.com/darkspock/gosnag/internal/issue"
	"github.com/darkspock/gosnag/internal/jira"
	"github.com/darkspock/gosnag/internal/project"
	"github.com/darkspock/gosnag/internal/user"
	"github.com/darkspock/gosnag/web"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
)

func corsMiddleware(cfg *config.Config) func(http.Handler) http.Handler {
	allowedOrigins := buildAllowedOrigins(cfg.BaseURL, cfg.CORSAllowedOrigins)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch {
			case isIngestPath(r.URL.Path):
				applyIngestCORS(w)
				if r.Method == http.MethodOptions {
					w.WriteHeader(http.StatusNoContent)
					return
				}
			case isManagedAPIPath(r.URL.Path):
				origin := r.Header.Get("Origin")
				if origin != "" {
					if !isAllowedOrigin(origin, requestOrigin(r), allowedOrigins) {
						http.Error(w, `{"error":"origin not allowed"}`, http.StatusForbidden)
						return
					}
					applyManagedCORS(w, origin)
				}
				if r.Method == http.MethodOptions {
					w.WriteHeader(http.StatusNoContent)
					return
				}
			}

			next.ServeHTTP(w, r)
		})
	}
}

func setupRouter(database *sql.DB, cfg *config.Config) http.Handler {
	queries := db.New(database)

	projectHandler := project.NewHandler(queries)
	issueHandler := issue.NewHandler(queries)
	userHandler := user.NewHandler(queries)
	alertHandler := alert.NewHandler(queries)
	jiraHandler := jira.NewHandler(queries, cfg)
	oauthHandler := auth.NewOAuthHandler(queries, cfg)

	alertService := alert.NewService(queries, cfg)
	ingestHandler := ingest.NewHandler(queries, func(projectID uuid.UUID, iss db.Issue, isNew bool) {
		alertService.Notify(projectID, iss, isNew)
		go jira.CheckAndCreateTicket(context.Background(), queries, cfg.BaseURL, projectID, iss)
	})

	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Heartbeat("/health"))
	r.Use(corsMiddleware(cfg))

	// Sentry SDK ingest endpoints (authenticated by DSN key, no session needed)
	ingestRL := ingest.NewRateLimiter(cfg.IngestRateLimitPerMin, 1*time.Minute)
	r.With(ingestRL).Post("/api/{project_id}/store/", ingestHandler.Store)
	r.With(ingestRL).Post("/api/{project_id}/envelope/", ingestHandler.Envelope)

	// Auth endpoints (no session needed)
	r.Route("/api/v1/auth", func(r chi.Router) {
		r.Get("/config", oauthHandler.AuthConfig)
		r.Post("/google/token", oauthHandler.TokenLogin)
		r.Post("/logout", oauthHandler.Logout)
	})

	// Management API (session or API token)
	r.Route("/api/v1", func(r chi.Router) {
		r.Use(auth.MiddlewareWithToken(queries, cfg.BaseURL))

		r.Get("/me", oauthHandler.Me)

		// Projects
		r.Route("/projects", func(r chi.Router) {
			r.Get("/", projectHandler.List)
			r.With(auth.RequireAdmin).Post("/", projectHandler.Create)
			r.Route("/{project_id}", func(r chi.Router) {
				r.Get("/", projectHandler.Get)
				r.With(auth.RequireAdmin).Put("/", projectHandler.Update)
				r.With(auth.RequireAdmin).Delete("/", projectHandler.Delete)

				// API Tokens per project
				r.Route("/tokens", func(r chi.Router) {
					r.Get("/", projectHandler.ListTokens)
					r.With(auth.RequireAdmin).Post("/", projectHandler.CreateToken)
					r.With(auth.RequireAdmin).Delete("/{tokenId}", projectHandler.DeleteToken)
				})

				// Jira integration
				r.Post("/jira/test", jiraHandler.TestConnection)
				r.Route("/jira/rules", func(r chi.Router) {
					r.Get("/", jiraHandler.ListRules)
					r.With(auth.RequireAdmin).Post("/", jiraHandler.CreateRule)
					r.With(auth.RequireAdmin).Put("/{rule_id}", jiraHandler.UpdateRule)
					r.With(auth.RequireAdmin).Delete("/{rule_id}", jiraHandler.DeleteRule)
				})

				// Alerts per project
				r.Route("/alerts", func(r chi.Router) {
					r.Get("/", alertHandler.List)
					r.With(auth.RequireAdmin).Post("/", alertHandler.Create)
					r.With(auth.RequireAdmin).Put("/{alert_id}", alertHandler.Update)
					r.With(auth.RequireAdmin).Delete("/{alert_id}", alertHandler.Delete)
				})
			})
		})

		// Issues (readable by API tokens, writable needs readwrite permission)
		r.Route("/projects/{project_id}/issues", func(r chi.Router) {
			r.Get("/", issueHandler.List)
			r.With(auth.RequireWritePermission).Delete("/", issueHandler.BulkDelete)
			r.Get("/counts", issueHandler.Counts)
			r.Route("/{issue_id}", func(r chi.Router) {
				r.Get("/", issueHandler.Get)
				r.With(auth.RequireWritePermission).Put("/", issueHandler.UpdateStatus)
				r.With(auth.RequireWritePermission).Put("/assign", issueHandler.Assign)
				r.Get("/events", issueHandler.ListEvents)
				r.With(auth.RequireWritePermission).Post("/jira", jiraHandler.CreateTicket)
			})
		})

		// Users (admin only for write, session only)
		r.Route("/users", func(r chi.Router) {
			r.Get("/", userHandler.List)
			r.With(auth.RequireAdmin).Post("/invite", userHandler.Invite)
			r.With(auth.RequireAdmin).Put("/{user_id}", userHandler.UpdateRole)
			r.With(auth.RequireAdmin).Put("/{user_id}/status", userHandler.UpdateStatus)
		})
	})

	// Serve embedded frontend (SPA fallback)
	distFS, _ := fs.Sub(web.Assets, "dist")
	fileServer := http.FileServer(http.FS(distFS))
	r.NotFound(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path == "" {
			path = "index.html"
		}
		if _, err := fs.Stat(distFS, path); err == nil {
			fileServer.ServeHTTP(w, r)
			return
		}
		// SPA fallback: serve index.html for client-side routes
		r.URL.Path = "/"
		fileServer.ServeHTTP(w, r)
	})

	return r
}

func applyIngestCORS(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Sentry-Auth, Sentry-Trace, Baggage")
	w.Header().Set("Access-Control-Max-Age", "600")
}

func applyManagedCORS(w http.ResponseWriter, origin string) {
	addVary(w.Header(), "Origin")
	w.Header().Set("Access-Control-Allow-Origin", origin)
	w.Header().Set("Access-Control-Allow-Credentials", "true")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
	w.Header().Set("Access-Control-Max-Age", "600")
}

func addVary(header http.Header, value string) {
	existing := header.Values("Vary")
	for _, item := range existing {
		for _, part := range strings.Split(item, ",") {
			if strings.TrimSpace(part) == value {
				return
			}
		}
	}
	header.Add("Vary", value)
}

func isIngestPath(path string) bool {
	return strings.HasPrefix(path, "/api/") && !isManagedAPIPath(path)
}

func isManagedAPIPath(path string) bool {
	return strings.HasPrefix(path, "/api/v1/")
}

func buildAllowedOrigins(baseURL string, configured []string) map[string]struct{} {
	out := make(map[string]struct{})

	for _, origin := range configured {
		if normalized, ok := normalizeOrigin(origin); ok {
			out[normalized] = struct{}{}
		}
	}

	baseOrigin, ok := normalizeOrigin(baseURL)
	if !ok {
		return out
	}

	out[baseOrigin] = struct{}{}

	u, err := url.Parse(baseURL)
	if err != nil {
		return out
	}

	if !isLocalHost(u.Hostname()) {
		return out
	}

	for _, origin := range []string{
		"http://localhost:5173",
		"http://127.0.0.1:5173",
		"http://localhost:4173",
		"http://127.0.0.1:4173",
	} {
		if normalized, ok := normalizeOrigin(origin); ok {
			out[normalized] = struct{}{}
		}
	}

	return out
}

func isAllowedOrigin(origin, requestOrigin string, allowed map[string]struct{}) bool {
	normalizedOrigin, ok := normalizeOrigin(origin)
	if !ok {
		return false
	}

	if requestOrigin != "" && normalizedOrigin == requestOrigin {
		return true
	}

	_, ok = allowed[normalizedOrigin]
	return ok
}

func requestOrigin(r *http.Request) string {
	if r == nil || r.Host == "" {
		return ""
	}

	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	if proto := authForwardedProto(r); proto != "" {
		scheme = proto
	}

	return scheme + "://" + r.Host
}

func authForwardedProto(r *http.Request) string {
	if r == nil {
		return ""
	}
	parts := strings.Split(r.Header.Get("X-Forwarded-Proto"), ",")
	if len(parts) == 0 {
		return ""
	}
	return strings.TrimSpace(parts[0])
}

func normalizeOrigin(raw string) (string, bool) {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return "", false
	}
	return strings.ToLower(u.Scheme) + "://" + strings.ToLower(u.Host), true
}

func isLocalHost(host string) bool {
	switch strings.ToLower(host) {
	case "localhost", "127.0.0.1", "::1":
		return true
	default:
		return false
	}
}
