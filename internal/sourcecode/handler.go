package sourcecode

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

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

// TestConnection tests the source code repository connection.
func (h *Handler) TestConnection(w http.ResponseWriter, r *http.Request) {
	projectID, err := uuid.Parse(chi.URLParam(r, "project_id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid project id")
		return
	}

	project, err := h.queries.GetProject(r.Context(), projectID)
	if err != nil {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}

	cfg := ConfigFromProject(project)
	if !cfg.IsConfigured() {
		writeError(w, http.StatusBadRequest, "repository not configured")
		return
	}

	provider := NewProvider(cfg)
	if provider == nil {
		writeError(w, http.StatusBadRequest, "unsupported provider: "+cfg.Provider)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	if err := provider.TestConnection(ctx); err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// SuspectCommits returns recent commits that touched files in the stack trace.
func (h *Handler) SuspectCommits(w http.ResponseWriter, r *http.Request) {
	projectID, err := uuid.Parse(chi.URLParam(r, "project_id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid project id")
		return
	}
	issueID, err := uuid.Parse(chi.URLParam(r, "issue_id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid issue id")
		return
	}

	project, err := h.queries.GetProject(r.Context(), projectID)
	if err != nil {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}

	cfg := ConfigFromProject(project)
	if !cfg.IsConfigured() {
		writeJSON(w, http.StatusOK, map[string]any{"commits": []any{}})
		return
	}

	provider := NewProvider(cfg)
	if provider == nil {
		writeJSON(w, http.StatusOK, map[string]any{"commits": []any{}})
		return
	}

	// Get the latest event for this issue to extract stack trace files
	events, err := h.queries.ListEventsByIssue(r.Context(), db.ListEventsByIssueParams{
		IssueID: issueID,
		Limit:   1,
		Offset:  0,
	})
	if err != nil || len(events) == 0 {
		writeJSON(w, http.StatusOK, map[string]any{"commits": []any{}})
		return
	}

	files := extractFilesFromEvent(events[0].Data, cfg)
	if len(files) == 0 {
		writeJSON(w, http.StatusOK, map[string]any{"commits": []any{}})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	since := time.Now().Add(-7 * 24 * time.Hour)
	commits, err := provider.GetCommitsForFiles(ctx, files, since)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"commits": []any{}, "error": err.Error()})
		return
	}

	// Sort by number of matching files (desc), then by time (desc)
	// Simple scoring: more matching files = more suspect
	for i := range commits {
		commits[i].Files = uniqueStrings(commits[i].Files)
	}

	// Limit to top 5
	if len(commits) > 5 {
		commits = commits[:5]
	}

	writeJSON(w, http.StatusOK, map[string]any{"commits": commits})
}

func extractFilesFromEvent(data json.RawMessage, cfg Config) []string {
	var eventData map[string]any
	if err := json.Unmarshal(data, &eventData); err != nil {
		return nil
	}

	var files []string
	seen := map[string]bool{}

	exc, _ := eventData["exception"].(map[string]any)
	values, _ := exc["values"].([]any)
	for _, v := range values {
		val, _ := v.(map[string]any)
		st, _ := val["stacktrace"].(map[string]any)
		frames, _ := st["frames"].([]any)
		for _, f := range frames {
			frame, _ := f.(map[string]any)
			filename, _ := frame["filename"].(string)
			if filename == "" || IsLibraryPath(filename) {
				continue
			}
			clean := cfg.StripPath(filename)
			if !seen[clean] {
				seen[clean] = true
				files = append(files, filename) // pass original for stripping in provider
			}
		}
	}

	return files
}

func uniqueStrings(ss []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range ss {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
