package project

import (
	"context"
	"database/sql"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	_ "github.com/go-sql-driver/mysql"
)

var (
	basicAuthDSNPattern = regexp.MustCompile(`://[^/\s:@]+:[^/\s@]+@`)
	passwordPairPattern = regexp.MustCompile(`(?i)\b(password|pwd)\s*=\s*[^ \t\n\r;]+`)
	mysqlCredentialPattern = regexp.MustCompile(`^([^:@/\s]+):([^@/\s]+)@`)
)

func normalizeAnalysisDriver(driver string) string {
	switch strings.ToLower(strings.TrimSpace(driver)) {
	case "postgres", "postgresql":
		return "postgres"
	case "mysql", "mariadb":
		return "mysql"
	default:
		return ""
	}
}

func sanitizeDBAnalysisError(err error) string {
	if err == nil {
		return "connection test failed"
	}
	msg := strings.TrimSpace(err.Error())
	if msg == "" {
		return "connection test failed"
	}
	msg = basicAuthDSNPattern.ReplaceAllString(msg, "://[redacted]@")
	msg = passwordPairPattern.ReplaceAllString(msg, "$1=[redacted]")
	return msg
}

func maskedAnalysisDSN(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}

	if strings.Contains(raw, "://") {
		raw = basicAuthDSNPattern.ReplaceAllString(raw, "://[redacted]@")
		raw = passwordPairPattern.ReplaceAllString(raw, "$1=[redacted]")
		return raw
	}

	if mysqlCredentialPattern.MatchString(raw) {
		return mysqlCredentialPattern.ReplaceAllString(raw, "$1:[redacted]@")
	}

	return passwordPairPattern.ReplaceAllString(raw, "$1=[redacted]")
}

func testAnalysisConnection(ctx context.Context, driver, dsn string) error {
	db, err := sql.Open(driver, dsn)
	if err != nil {
		return err
	}
	defer db.Close()

	db.SetConnMaxLifetime(30 * time.Second)
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(0)

	return db.PingContext(ctx)
}

// TestDBAnalysisConnection tests the configured database analysis connection for a project.
func (h *Handler) TestDBAnalysisConnection(w http.ResponseWriter, r *http.Request) {
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

	if !project.AnalysisDbEnabled {
		writeError(w, http.StatusBadRequest, "database analysis is disabled for this project")
		return
	}

	driver := normalizeAnalysisDriver(project.AnalysisDbDriver)
	if driver == "" {
		writeError(w, http.StatusBadRequest, "unsupported analysis DB driver")
		return
	}

	if strings.TrimSpace(project.AnalysisDbDsn) == "" {
		writeError(w, http.StatusBadRequest, "database analysis DSN is not configured")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	if err := testAnalysisConnection(ctx, driver, project.AnalysisDbDsn); err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": sanitizeDBAnalysisError(err)})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}
