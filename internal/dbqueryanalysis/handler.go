package dbqueryanalysis

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/darkspock/gosnag/internal/database/db"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
)

type Handler struct {
	queries *db.Queries
}

func NewHandler(queries *db.Queries) *Handler {
	return &Handler{queries: queries}
}

type analyzeRequest struct {
	NormalizedSQL string `json:"normalized_sql,omitempty"`
}

type Breadcrumb struct {
	Type      string         `json:"type,omitempty"`
	Category  string         `json:"category,omitempty"`
	Message   string         `json:"message,omitempty"`
	Data      map[string]any `json:"data,omitempty"`
	Level     string         `json:"level,omitempty"`
	Timestamp any            `json:"timestamp,omitempty"`
}

type AnalysisResponse struct {
	Summary  Summary       `json:"summary"`
	Queries  []QueryResult `json:"queries"`
	Warnings []string      `json:"warnings"`
	EventID  string        `json:"event_id,omitempty"`
}

type Summary struct {
	QueryCount         int     `json:"query_count"`
	UniqueQueryCount   int     `json:"unique_query_count"`
	TotalDurationMs    float64 `json:"total_duration_ms"`
	MissingTimingCount int     `json:"missing_timing_count"`
	TimingAvailability string  `json:"timing_availability"`
	NPlusOneCandidates int     `json:"n_plus_one_candidates"`
	ExplainAttempted   int     `json:"explain_attempted"`
	ExplainCompleted   int     `json:"explain_completed"`
}

type QueryResult struct {
	NormalizedSQL      string   `json:"normalized_sql"`
	SampleSQL          string   `json:"sample_sql"`
	QueryType          string   `json:"query_type"`
	LikelyEntity       string   `json:"likely_entity,omitempty"`
	Count              int      `json:"count"`
	TotalDurationMs    float64  `json:"total_duration_ms"`
	AvgDurationMs      float64  `json:"avg_duration_ms"`
	MaxDurationMs      float64  `json:"max_duration_ms"`
	MissingTimingCount int      `json:"missing_timing_count"`
	SuspectedNPlusOne  bool     `json:"suspected_n_plus_one"`
	ExplainPlan        string   `json:"explain_plan,omitempty"`
	ExplainError       string   `json:"explain_error,omitempty"`
	ExplainWarnings    []string `json:"explain_warnings,omitempty"`
}

type dbConfig struct {
	Enabled bool
	Driver  string
	DSN     string
}

type extractedQuery struct {
	SQL        string
	DurationMs float64
	HasTiming  bool
}

type aggregate struct {
	Normalized         string
	Sample             string
	QueryType          string
	Count              int
	TotalDurationMs    float64
	MaxDurationMs      float64
	MissingTimingCount int
}

func (h *Handler) AnalyzeIssueQueries(w http.ResponseWriter, r *http.Request) {
	_, _, _, _, _, event, err := h.loadAnalysisContext(r.Context(), chi.URLParam(r, "project_id"), chi.URLParam(r, "issue_id"))
	if err != nil {
		writeAnalysisError(w, err)
		return
	}

	items, warnings := extractQueries(event.Breadcrumbs)
	if len(items) == 0 {
		writeJSON(w, http.StatusOK, AnalysisResponse{
			Summary:  Summary{},
			Queries:  []QueryResult{},
			Warnings: []string{"No SQL breadcrumbs found in this event."},
			EventID:  event.EventID,
		})
		return
	}

	response := analyzeQueries(items)
	response.Warnings = append(response.Warnings, warnings...)
	response.EventID = event.EventID

	writeJSON(w, http.StatusOK, response)
}

// ExplainIssueQuery handles POST /projects/{project_id}/issues/{issue_id}/db-analysis/explain
func (h *Handler) ExplainIssueQuery(w http.ResponseWriter, r *http.Request) {
	_, _, _, _, cfg, event, err := h.loadAnalysisContext(r.Context(), chi.URLParam(r, "project_id"), chi.URLParam(r, "issue_id"))
	if err != nil {
		writeAnalysisError(w, err)
		return
	}

	var req analyzeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if strings.TrimSpace(req.NormalizedSQL) == "" {
		writeError(w, http.StatusBadRequest, "normalized_sql is required")
		return
	}

	items, _ := extractQueries(event.Breadcrumbs)
	response := analyzeQueries(items)

	index := -1
	for i, item := range response.Queries {
		if item.NormalizedSQL == strings.TrimSpace(req.NormalizedSQL) {
			index = i
			break
		}
	}
	if index < 0 {
		writeError(w, http.StatusNotFound, "query group not found in latest event")
		return
	}
	if response.Queries[index].QueryType != "select" {
		writeError(w, http.StatusBadRequest, "EXPLAIN is only supported for SELECT queries")
		return
	}

	explainCtx, cancel := context.WithTimeout(r.Context(), 4*time.Second)
	defer cancel()
	plan, warnings, err := runExplainOnSQL(explainCtx, cfg, response.Queries[index].SampleSQL)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"normalized_sql": req.NormalizedSQL,
			"error":          sanitizeAnalysisError(err),
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"normalized_sql": req.NormalizedSQL,
		"plan":           plan,
		"warnings":       warnings,
	})
}

func (h *Handler) loadDBConfig(ctx context.Context, p db.Project) (dbConfig, error) {
	cfg := dbConfig{
		Enabled: p.AnalysisDbEnabled,
		Driver:  normalizeDriver(p.AnalysisDbDriver),
		DSN:     strings.TrimSpace(p.AnalysisDbDsn),
	}

	err := h.queries.RawDB().QueryRowContext(ctx, `
		SELECT enabled, driver, dsn
		FROM project_db_analysis_settings
		WHERE project_id = $1
	`, p.ID).Scan(&cfg.Enabled, &cfg.Driver, &cfg.DSN)
	if err == sql.ErrNoRows {
		return cfg, nil
	}
	if err != nil {
		return cfg, err
	}
	cfg.Driver = normalizeDriver(cfg.Driver)
	cfg.DSN = strings.TrimSpace(cfg.DSN)
	return cfg, nil
}

type analysisContext struct {
	EventID     string
	Breadcrumbs []Breadcrumb
}

type analysisError struct {
	status int
	msg    string
}

func (e analysisError) Error() string { return e.msg }

func (h *Handler) loadAnalysisContext(ctx context.Context, projectIDRaw, issueIDRaw string) (uuid.UUID, uuid.UUID, db.Project, db.Issue, dbConfig, analysisContext, error) {
	projectID, err := uuid.Parse(projectIDRaw)
	if err != nil {
		return uuid.Nil, uuid.Nil, db.Project{}, db.Issue{}, dbConfig{}, analysisContext{}, analysisError{status: http.StatusBadRequest, msg: "invalid project id"}
	}
	issueID, err := uuid.Parse(issueIDRaw)
	if err != nil {
		return uuid.Nil, uuid.Nil, db.Project{}, db.Issue{}, dbConfig{}, analysisContext{}, analysisError{status: http.StatusBadRequest, msg: "invalid issue id"}
	}

	projectRow, err := h.queries.GetProject(ctx, projectID)
	if err != nil {
		return uuid.Nil, uuid.Nil, db.Project{}, db.Issue{}, dbConfig{}, analysisContext{}, analysisError{status: http.StatusNotFound, msg: "project not found"}
	}
	issueRow, err := h.queries.GetIssue(ctx, issueID)
	if err != nil || issueRow.ProjectID != projectID {
		return uuid.Nil, uuid.Nil, db.Project{}, db.Issue{}, dbConfig{}, analysisContext{}, analysisError{status: http.StatusNotFound, msg: "issue not found"}
	}

	cfg, err := h.loadDBConfig(ctx, projectRow)
	if err != nil {
		return uuid.Nil, uuid.Nil, db.Project{}, db.Issue{}, dbConfig{}, analysisContext{}, analysisError{status: http.StatusInternalServerError, msg: "failed to load database analysis settings"}
	}
	if !cfg.Enabled {
		return uuid.Nil, uuid.Nil, db.Project{}, db.Issue{}, dbConfig{}, analysisContext{}, analysisError{status: http.StatusBadRequest, msg: "database analysis is disabled for this project"}
	}
	if cfg.Driver == "" {
		return uuid.Nil, uuid.Nil, db.Project{}, db.Issue{}, dbConfig{}, analysisContext{}, analysisError{status: http.StatusBadRequest, msg: "unsupported analysis DB driver"}
	}
	if strings.TrimSpace(cfg.DSN) == "" {
		return uuid.Nil, uuid.Nil, db.Project{}, db.Issue{}, dbConfig{}, analysisContext{}, analysisError{status: http.StatusBadRequest, msg: "database analysis DSN is not configured"}
	}

	eventRow, err := h.queries.GetLatestEventByIssue(ctx, issueID)
	if err != nil {
		return uuid.Nil, uuid.Nil, db.Project{}, db.Issue{}, dbConfig{}, analysisContext{}, analysisError{status: http.StatusNotFound, msg: "latest event not found"}
	}
	event, err := parseAnalysisEvent(eventRow.Data)
	if err != nil {
		return uuid.Nil, uuid.Nil, db.Project{}, db.Issue{}, dbConfig{}, analysisContext{}, analysisError{status: http.StatusBadRequest, msg: "latest event does not contain valid breadcrumb data"}
	}
	event.EventID = eventRow.EventID

	return projectID, issueID, projectRow, issueRow, cfg, event, nil
}

func parseAnalysisEvent(raw json.RawMessage) (analysisContext, error) {
	var payload struct {
		Breadcrumbs struct {
			Values []Breadcrumb `json:"values"`
		} `json:"breadcrumbs"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return analysisContext{}, err
	}
	return analysisContext{Breadcrumbs: payload.Breadcrumbs.Values}, nil
}

func extractQueries(breadcrumbs []Breadcrumb) ([]extractedQuery, []string) {
	items := make([]extractedQuery, 0, len(breadcrumbs))
	for _, crumb := range breadcrumbs {
		if !isDBQueryBreadcrumb(crumb) {
			continue
		}
		sqlText := extractSQL(crumb)
		if sqlText == "" {
			continue
		}
		duration, hasTiming := extractDuration(crumb)
		items = append(items, extractedQuery{
			SQL:        sqlText,
			DurationMs: duration,
			HasTiming:  hasTiming,
		})
	}

	if len(items) == 0 {
		return items, nil
	}

	missing := 0
	for _, item := range items {
		if !item.HasTiming {
			missing++
		}
	}

	switch {
	case missing == len(items):
		return items, []string{"None of the SQL breadcrumbs include duration_ms. Improve instrumentation so each query emits timing metadata."}
	case missing > 0:
		return items, []string{"Some SQL breadcrumbs are missing duration_ms, so totals are partial."}
	default:
		return items, nil
	}
}

func analyzeQueries(items []extractedQuery) AnalysisResponse {
	aggregates := map[string]*aggregate{}
	summary := Summary{}

	for _, item := range items {
		normalized := normalizeSQL(item.SQL)
		queryType := detectQueryType(item.SQL)
		group := aggregates[normalized]
		if group == nil {
			group = &aggregate{
				Normalized: normalized,
				Sample:     item.SQL,
				QueryType:  queryType,
			}
			aggregates[normalized] = group
		}

		group.Count++
		summary.QueryCount++

		if item.HasTiming {
			group.TotalDurationMs += item.DurationMs
			if item.DurationMs > group.MaxDurationMs {
				group.MaxDurationMs = item.DurationMs
			}
			summary.TotalDurationMs += item.DurationMs
		} else {
			group.MissingTimingCount++
			summary.MissingTimingCount++
		}
	}

	results := make([]QueryResult, 0, len(aggregates))
	for _, group := range aggregates {
		if isNPlusOneCandidate(*group) {
			summary.NPlusOneCandidates++
		}
		timedCount := group.Count - group.MissingTimingCount
		avg := 0.0
		if timedCount > 0 {
			avg = group.TotalDurationMs / float64(timedCount)
		}
		results = append(results, QueryResult{
			NormalizedSQL:      group.Normalized,
			SampleSQL:          group.Sample,
			QueryType:          group.QueryType,
			LikelyEntity:       extractLikelyEntity(group.Normalized, group.QueryType),
			Count:              group.Count,
			TotalDurationMs:    group.TotalDurationMs,
			AvgDurationMs:      avg,
			MaxDurationMs:      group.MaxDurationMs,
			MissingTimingCount: group.MissingTimingCount,
			SuspectedNPlusOne:  isNPlusOneCandidate(*group),
		})
	}

	sort.Slice(results, func(i, j int) bool {
		if results[i].Count != results[j].Count {
			return results[i].Count > results[j].Count
		}
		if results[i].TotalDurationMs != results[j].TotalDurationMs {
			return results[i].TotalDurationMs > results[j].TotalDurationMs
		}
		return results[i].SampleSQL < results[j].SampleSQL
	})

	summary.UniqueQueryCount = len(results)
	switch {
	case summary.QueryCount == 0:
		summary.TimingAvailability = "none"
	case summary.MissingTimingCount == 0:
		summary.TimingAvailability = "all"
	case summary.MissingTimingCount == summary.QueryCount:
		summary.TimingAvailability = "none"
	default:
		summary.TimingAvailability = "partial"
	}

	if summary.NPlusOneCandidates > 0 {
		resultsWarning := "N+1 signals are heuristic and based on repeated query shapes in the latest event."
		return AnalysisResponse{
			Summary: summary,
			Queries: results,
			Warnings: []string{
				resultsWarning,
			},
		}
	}

	return AnalysisResponse{
		Summary:  summary,
		Queries:  results,
		Warnings: []string{},
	}
}

func runExplainOnSQL(ctx context.Context, cfg dbConfig, sqlText string) (string, []string, error) {
	sqlDB, err := sql.Open(cfg.Driver, cfg.DSN)
	if err != nil {
		return "", nil, err
	}
	defer sqlDB.Close()

	sqlDB.SetConnMaxLifetime(30 * time.Second)
	sqlDB.SetMaxOpenConns(1)
	sqlDB.SetMaxIdleConns(0)

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := sqlDB.PingContext(pingCtx); err != nil {
		return "", nil, err
	}

	plan, err := runExplain(ctx, sqlDB, sqlText)
	if err != nil {
		return "", nil, err
	}
	return plan, explainWarnings(cfg.Driver, plan), nil
}

func runExplain(ctx context.Context, sqlDB *sql.DB, query string) (string, error) {
	query = strings.TrimSpace(query)
	query = strings.TrimSuffix(query, ";")
	if query == "" {
		return "", fmt.Errorf("empty query")
	}
	if strings.Contains(query, ";") {
		return "", fmt.Errorf("multiple statements are not supported")
	}

	rows, err := sqlDB.QueryContext(ctx, "EXPLAIN "+query)
	if err != nil {
		return "", err
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return "", err
	}

	var lines []string
	for rows.Next() {
		values := make([]any, len(cols))
		scanTargets := make([]any, len(cols))
		for i := range values {
			scanTargets[i] = &values[i]
		}
		if err := rows.Scan(scanTargets...); err != nil {
			return "", err
		}

		parts := make([]string, 0, len(cols))
		for i, col := range cols {
			parts = append(parts, col+"="+formatExplainValue(values[i]))
		}
		lines = append(lines, strings.Join(parts, " | "))
	}
	if err := rows.Err(); err != nil {
		return "", err
	}
	return strings.Join(lines, "\n"), nil
}

func formatExplainValue(v any) string {
	switch value := v.(type) {
	case nil:
		return "NULL"
	case []byte:
		return compactWhitespace(string(value))
	case string:
		return compactWhitespace(value)
	default:
		return compactWhitespace(fmt.Sprint(value))
	}
}

func compactWhitespace(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	return whitespaceRegex.ReplaceAllString(value, " ")
}

func isDBQueryBreadcrumb(crumb Breadcrumb) bool {
	category := strings.ToLower(strings.TrimSpace(crumb.Category))
	typ := strings.ToLower(strings.TrimSpace(crumb.Type))
	return strings.Contains(category, "db.query") || category == "query" || strings.Contains(typ, "db.query") || typ == "query"
}

func extractSQL(crumb Breadcrumb) string {
	if msg := strings.TrimSpace(crumb.Message); msg != "" {
		return msg
	}
	if crumb.Data == nil {
		return ""
	}
	for _, key := range []string{"query", "sql", "statement"} {
		if raw, ok := crumb.Data[key]; ok {
			if text := strings.TrimSpace(stringValue(raw)); text != "" {
				return text
			}
		}
	}
	return ""
}

func extractDuration(crumb Breadcrumb) (float64, bool) {
	if crumb.Data == nil {
		return 0, false
	}
	raw, ok := crumb.Data["duration_ms"]
	if !ok {
		return 0, false
	}
	switch value := raw.(type) {
	case float64:
		return value, true
	case float32:
		return float64(value), true
	case int:
		return float64(value), true
	case int32:
		return float64(value), true
	case int64:
		return float64(value), true
	case json.Number:
		v, err := value.Float64()
		return v, err == nil
	case string:
		v, err := json.Number(value).Float64()
		return v, err == nil
	default:
		return 0, false
	}
}

func normalizeSQL(sqlText string) string {
	sqlText = strings.TrimSpace(sqlText)
	if sqlText == "" {
		return ""
	}
	sqlText = strings.TrimSuffix(sqlText, ";")
	sqlText = quotedLiteralRegex.ReplaceAllString(sqlText, "?")
	sqlText = numericLiteralRegex.ReplaceAllString(sqlText, "?")
	sqlText = whitespaceRegex.ReplaceAllString(sqlText, " ")
	return strings.TrimSpace(sqlText)
}

func detectQueryType(sqlText string) string {
	upper := strings.ToUpper(strings.TrimSpace(sqlText))
	switch {
	case strings.HasPrefix(upper, "SELECT"), strings.HasPrefix(upper, "WITH"):
		return "select"
	case strings.HasPrefix(upper, "INSERT"):
		return "insert"
	case strings.HasPrefix(upper, "UPDATE"):
		return "update"
	case strings.HasPrefix(upper, "DELETE"):
		return "delete"
	default:
		return "other"
	}
}

func isNPlusOneCandidate(group aggregate) bool {
	lower := strings.ToLower(group.Normalized)
	if group.QueryType != "select" || group.Count < 4 {
		return false
	}
	return strings.Contains(lower, " where ") || strings.Contains(lower, " join ")
}

func extractLikelyEntity(normalizedSQL, queryType string) string {
	lower := strings.ToLower(strings.TrimSpace(normalizedSQL))
	fields := strings.Fields(lower)
	if len(fields) == 0 {
		return ""
	}

	findAfter := func(keyword string) string {
		for i := 0; i < len(fields)-1; i++ {
			if fields[i] == keyword {
				return strings.Trim(fields[i+1], "`\"")
			}
		}
		return ""
	}

	switch queryType {
	case "select", "delete":
		if entity := findAfter("from"); entity != "" {
			return entity
		}
	case "insert":
		if entity := findAfter("into"); entity != "" {
			return entity
		}
	case "update":
		return strings.Trim(fields[1], "`\"")
	}
	return ""
}

func explainWarnings(driver, plan string) []string {
	warnings := []string{}
	lower := strings.ToLower(plan)
	switch driver {
	case "mysql":
		if strings.Contains(plan, "type=ALL") || strings.Contains(lower, " key=null") {
			warnings = append(warnings, "Possible full scan or missing index in MySQL EXPLAIN output.")
		}
		if strings.Contains(lower, "using temporary") || strings.Contains(lower, "using filesort") {
			warnings = append(warnings, "MySQL plan indicates temporary table or filesort.")
		}
	case "postgres":
		if strings.Contains(lower, "seq scan") {
			warnings = append(warnings, "PostgreSQL plan contains Seq Scan.")
		}
		if strings.Contains(lower, "nested loop") {
			warnings = append(warnings, "PostgreSQL plan contains Nested Loop.")
		}
	}
	return warnings
}

func normalizeDriver(driver string) string {
	switch strings.ToLower(strings.TrimSpace(driver)) {
	case "postgres", "postgresql":
		return "postgres"
	case "mysql", "mariadb":
		return "mysql"
	default:
		return ""
	}
}

func sanitizeAnalysisError(err error) string {
	if err == nil {
		return "database analysis failed"
	}
	msg := strings.TrimSpace(err.Error())
	if msg == "" {
		return "database analysis failed"
	}
	msg = basicAuthDSNPattern.ReplaceAllString(msg, "://[redacted]@")
	msg = passwordPairPattern.ReplaceAllString(msg, "$1=[redacted]")
	return msg
}

func stringValue(v any) string {
	switch value := v.(type) {
	case string:
		return value
	case []byte:
		return string(value)
	default:
		b, _ := json.Marshal(value)
		return string(b)
	}
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func writeAnalysisError(w http.ResponseWriter, err error) {
	if analysisErr, ok := err.(analysisError); ok {
		writeError(w, analysisErr.status, analysisErr.msg)
		return
	}
	writeError(w, http.StatusInternalServerError, "database analysis failed")
}
