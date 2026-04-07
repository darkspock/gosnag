package main

import (
	"context"
	"database/sql"
	"encoding/json"
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
	"github.com/darkspock/gosnag/internal/n1"
	"github.com/darkspock/gosnag/internal/priority"
	"github.com/darkspock/gosnag/internal/project"
	"github.com/darkspock/gosnag/internal/tags"
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

	statsCache := project.NewStatsCache(queries, 30*time.Second)
	projectHandler := project.NewHandler(queries, statsCache)
	issueHandler := issue.NewHandler(queries, database)
	userHandler := user.NewHandler(queries)
	alertHandler := alert.NewHandler(queries)
	jiraHandler := jira.NewHandler(queries, cfg)
	priorityHandler := priority.NewHandler(queries)
	tagsHandler := tags.NewHandler(queries)
	oauthHandler := auth.NewOAuthHandler(queries, cfg)

	alertService := alert.NewService(queries, cfg)
	ingestHandler := ingest.NewHandler(queries,
		func(projectID uuid.UUID, iss db.Issue, isNew bool) {
			alertService.Notify(projectID, iss, isNew)
			go jira.CheckAndCreateTicket(context.Background(), queries, cfg.BaseURL, projectID, iss)
		},
		func(projectID uuid.UUID, iss db.Issue, eventData json.RawMessage) {
			statsCache.Invalidate()
			go priority.Evaluate(context.Background(), queries, projectID, iss, eventData)
			go tags.AutoTag(context.Background(), queries, projectID, iss, eventData)
			go n1.ExtractAndStore(context.Background(), queries, projectID, eventData)
		},
	)

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

		// Personal Access Tokens (any authenticated user)
		r.Route("/tokens", func(r chi.Router) {
			r.Get("/", projectHandler.ListGlobalTokens)
			r.With(auth.RequireWritePermission).Post("/", projectHandler.CreateGlobalToken)
			r.With(auth.RequireWritePermission).Delete("/{tokenId}", projectHandler.DeleteGlobalToken)
		})

		// Project Groups
		r.Route("/groups", func(r chi.Router) {
			r.Get("/", projectHandler.ListGroups)
			r.With(auth.RequireAdmin).Post("/", projectHandler.CreateGroup)
			r.With(auth.RequireAdmin).Put("/{group_id}", projectHandler.UpdateGroup)
			r.With(auth.RequireAdmin).Delete("/{group_id}", projectHandler.DeleteGroup)
		})

		// Favorites (write permission required — mutates server state)
		r.Get("/favorites", projectHandler.ListFavorites)
		r.With(auth.RequireWritePermission).Put("/projects/{project_id}/favorite", projectHandler.AddFavorite)
		r.With(auth.RequireWritePermission).Delete("/projects/{project_id}/favorite", projectHandler.RemoveFavorite)

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

				// Jira integration (test uses stored credentials — admin only)
				r.With(auth.RequireAdmin).Post("/jira/test", jiraHandler.TestConnection)
				r.Route("/jira/rules", func(r chi.Router) {
					r.Get("/", jiraHandler.ListRules)
					r.With(auth.RequireAdmin).Post("/", jiraHandler.CreateRule)
					r.With(auth.RequireAdmin).Put("/{rule_id}", jiraHandler.UpdateRule)
					r.With(auth.RequireAdmin).Delete("/{rule_id}", jiraHandler.DeleteRule)
				})

				// Priority rules
				r.Route("/priority-rules", func(r chi.Router) {
					r.Get("/", priorityHandler.ListRules)
					r.With(auth.RequireAdmin).Post("/", priorityHandler.CreateRule)
					r.With(auth.RequireAdmin).Put("/{rule_id}", priorityHandler.UpdateRule)
					r.With(auth.RequireAdmin).Delete("/{rule_id}", priorityHandler.DeleteRule)
					r.With(auth.RequireAdmin).Post("/recalc", priorityHandler.RecalcAll)
				})

				// Tag rules per project
				r.Route("/tag-rules", func(r chi.Router) {
					r.Get("/", tagsHandler.ListRules)
					r.With(auth.RequireAdmin).Post("/", tagsHandler.CreateRule)
					r.With(auth.RequireAdmin).Put("/{rule_id}", tagsHandler.UpdateRule)
					r.With(auth.RequireAdmin).Delete("/{rule_id}", tagsHandler.DeleteRule)
				})
				r.Get("/tags", tagsHandler.ListDistinctTags)

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
			r.With(auth.RequireWritePermission).Post("/merge", issueHandler.Merge)
			r.Get("/counts", issueHandler.Counts)
			r.Route("/{issue_id}", func(r chi.Router) {
				r.Get("/", issueHandler.Get)
				r.With(auth.RequireWritePermission).Put("/", issueHandler.UpdateStatus)
				r.With(auth.RequireWritePermission).Put("/assign", issueHandler.Assign)
				r.Get("/events", issueHandler.ListEvents)
				r.Get("/tags", tagsHandler.ListIssueTags)
				r.With(auth.RequireWritePermission).Post("/tags", tagsHandler.AddTag)
				r.With(auth.RequireWritePermission).Delete("/tags", tagsHandler.RemoveTag)
				r.With(auth.RequireWritePermission).Post("/jira", jiraHandler.CreateTicket)
			})
		})

		// Users (list strips google_id; write operations admin only)
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
			// Hashed assets (js/css) get long cache; index.html gets no-cache
			if strings.HasPrefix(path, "assets/") {
				w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
			} else {
				w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
			}
			fileServer.ServeHTTP(w, r)
			return
		}
		// SPA fallback: serve index.html for client-side routes
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
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
