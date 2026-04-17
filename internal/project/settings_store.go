package project

import (
	"context"
	"database/sql"
	"encoding/json"

	"github.com/darkspock/gosnag/internal/database/db"
	"github.com/google/uuid"
)

type ProjectSettings struct {
	WarningAsError       bool
	MaxEventsPerIssue    int32
	IssueDisplayMode     string
	ErrorGroupingMode    string
	WarningGroupingMode  string
	InfoGroupingMode     string
	MaxInfoIssues        int32
	JiraBaseURL          string
	JiraEmail            string
	JiraAPIToken         string
	JiraProjectKey       string
	JiraIssueType        string
	GithubToken          string
	GithubOwner          string
	GithubRepo           string
	GithubLabels         string
	RepoProvider         string
	RepoOwner            string
	RepoName             string
	RepoDefaultBranch    string
	RepoToken            string
	RepoPathStrip        string
	AIEnabled            bool
	AIModel              string
	AIMergeSuggestions   bool
	AIAutoMerge          bool
	AIAnomalyDetection   bool
	AITicketDescription  bool
	AIRootCause          bool
	AITriage             bool
	StacktraceRules      json.RawMessage
	AnalysisDBEnabled    bool
	AnalysisDBDriver     string
	AnalysisDBDSN        string
	AnalysisDBName       string
	AnalysisDBSchema     string
	AnalysisDBNotes      string
	Framework            string
	RouteGroupingEnabled bool
}

func legacySettingsFromProject(p db.Project) ProjectSettings {
	return ProjectSettings{
		WarningAsError:       p.WarningAsError,
		MaxEventsPerIssue:    p.MaxEventsPerIssue,
		IssueDisplayMode:     p.IssueDisplayMode,
		ErrorGroupingMode:    p.InfoGroupingMode,
		WarningGroupingMode:  p.InfoGroupingMode,
		InfoGroupingMode:     p.InfoGroupingMode,
		MaxInfoIssues:        p.MaxInfoIssues,
		JiraBaseURL:          p.JiraBaseUrl,
		JiraEmail:            p.JiraEmail,
		JiraAPIToken:         p.JiraApiToken,
		JiraProjectKey:       p.JiraProjectKey,
		JiraIssueType:        p.JiraIssueType,
		GithubToken:          p.GithubToken,
		GithubOwner:          p.GithubOwner,
		GithubRepo:           p.GithubRepo,
		GithubLabels:         p.GithubLabels,
		RepoProvider:         p.RepoProvider,
		RepoOwner:            p.RepoOwner,
		RepoName:             p.RepoName,
		RepoDefaultBranch:    p.RepoDefaultBranch,
		RepoToken:            p.RepoToken,
		RepoPathStrip:        p.RepoPathStrip,
		AIEnabled:            p.AiEnabled,
		AIModel:              p.AiModel,
		AIMergeSuggestions:   p.AiMergeSuggestions,
		AIAutoMerge:          p.AiAutoMerge,
		AIAnomalyDetection:   p.AiAnomalyDetection,
		AITicketDescription:  p.AiTicketDescription,
		AIRootCause:          p.AiRootCause,
		AITriage:             p.AiTriage,
		StacktraceRules:      p.StacktraceRules,
		AnalysisDBEnabled:    p.AnalysisDbEnabled,
		AnalysisDBDriver:     p.AnalysisDbDriver,
		AnalysisDBDSN:        p.AnalysisDbDsn,
		AnalysisDBName:       p.AnalysisDbName,
		AnalysisDBSchema:     p.AnalysisDbSchema,
		AnalysisDBNotes:      p.AnalysisDbNotes,
		Framework:            "generic",
		RouteGroupingEnabled: false,
	}
}

func (s *ProjectSettings) normalizeDefaults() {
	if s.MaxEventsPerIssue == 0 {
		s.MaxEventsPerIssue = 1000
	}
	if s.IssueDisplayMode == "" {
		s.IssueDisplayMode = "classic"
	}
	if s.ErrorGroupingMode == "" {
		s.ErrorGroupingMode = "normal"
	}
	if s.WarningGroupingMode == "" {
		s.WarningGroupingMode = "normal"
	}
	if s.InfoGroupingMode == "" {
		s.InfoGroupingMode = "normal"
	}
	if s.GithubLabels == "" {
		s.GithubLabels = "bug"
	}
	if s.RepoDefaultBranch == "" {
		s.RepoDefaultBranch = "main"
	}
	if s.JiraIssueType == "" {
		s.JiraIssueType = "Bug"
	}
	if len(s.StacktraceRules) == 0 {
		s.StacktraceRules = marshalStacktraceRules(defaultStacktraceRules())
	}
	if s.Framework == "" {
		s.Framework = "generic"
	}
}

func loadProjectSettings(ctx context.Context, queries *db.Queries, p db.Project) (ProjectSettings, error) {
	out := legacySettingsFromProject(p)
	rawdb := queries.RawDB()

	if err := rawdb.QueryRowContext(ctx, `
		SELECT warning_as_error, max_events_per_issue, issue_display_mode, error_grouping_mode, warning_grouping_mode, info_grouping_mode, max_info_issues
		FROM project_issue_settings
		WHERE project_id = $1
	`, p.ID).Scan(
		&out.WarningAsError,
		&out.MaxEventsPerIssue,
		&out.IssueDisplayMode,
		&out.ErrorGroupingMode,
		&out.WarningGroupingMode,
		&out.InfoGroupingMode,
		&out.MaxInfoIssues,
	); err != nil && err != sql.ErrNoRows {
		return out, err
	}

	if err := rawdb.QueryRowContext(ctx, `
		SELECT base_url, email, api_token, project_key, issue_type
		FROM project_jira_settings
		WHERE project_id = $1
	`, p.ID).Scan(
		&out.JiraBaseURL,
		&out.JiraEmail,
		&out.JiraAPIToken,
		&out.JiraProjectKey,
		&out.JiraIssueType,
	); err != nil && err != sql.ErrNoRows {
		return out, err
	}

	if err := rawdb.QueryRowContext(ctx, `
		SELECT token, owner, repo, labels
		FROM project_github_settings
		WHERE project_id = $1
	`, p.ID).Scan(
		&out.GithubToken,
		&out.GithubOwner,
		&out.GithubRepo,
		&out.GithubLabels,
	); err != nil && err != sql.ErrNoRows {
		return out, err
	}

	if err := rawdb.QueryRowContext(ctx, `
		SELECT provider, owner, name, default_branch, token, path_strip
		FROM project_repo_settings
		WHERE project_id = $1
	`, p.ID).Scan(
		&out.RepoProvider,
		&out.RepoOwner,
		&out.RepoName,
		&out.RepoDefaultBranch,
		&out.RepoToken,
		&out.RepoPathStrip,
	); err != nil && err != sql.ErrNoRows {
		return out, err
	}

	if err := rawdb.QueryRowContext(ctx, `
		SELECT enabled, model, merge_suggestions, auto_merge, anomaly_detection, ticket_description, root_cause, triage
		FROM project_ai_settings
		WHERE project_id = $1
	`, p.ID).Scan(
		&out.AIEnabled,
		&out.AIModel,
		&out.AIMergeSuggestions,
		&out.AIAutoMerge,
		&out.AIAnomalyDetection,
		&out.AITicketDescription,
		&out.AIRootCause,
		&out.AITriage,
	); err != nil && err != sql.ErrNoRows {
		return out, err
	}

	if err := rawdb.QueryRowContext(ctx, `
		SELECT rules
		FROM project_stacktrace_settings
		WHERE project_id = $1
	`, p.ID).Scan(&out.StacktraceRules); err != nil && err != sql.ErrNoRows {
		return out, err
	}

	if err := rawdb.QueryRowContext(ctx, `
		SELECT enabled, driver, dsn, name, schema, notes
		FROM project_db_analysis_settings
		WHERE project_id = $1
	`, p.ID).Scan(
		&out.AnalysisDBEnabled,
		&out.AnalysisDBDriver,
		&out.AnalysisDBDSN,
		&out.AnalysisDBName,
		&out.AnalysisDBSchema,
		&out.AnalysisDBNotes,
	); err != nil && err != sql.ErrNoRows {
		return out, err
	}

	if err := rawdb.QueryRowContext(ctx, `
		SELECT framework, enabled
		FROM project_route_settings
		WHERE project_id = $1
	`, p.ID).Scan(&out.Framework, &out.RouteGroupingEnabled); err != nil && err != sql.ErrNoRows {
		return out, err
	}

	out.normalizeDefaults()
	return out, nil
}

func loadProjectSettingsMap(ctx context.Context, queries *db.Queries, projects []db.Project) (map[uuid.UUID]ProjectSettings, error) {
	result := make(map[uuid.UUID]ProjectSettings, len(projects))
	for _, p := range projects {
		result[p.ID] = legacySettingsFromProject(p)
	}

	rawdb := queries.RawDB()

	type issueRow struct {
		projectID           uuid.UUID
		warningAsError      bool
		maxEventsPerIssue   int32
		issueDisplayMode    string
		errorGroupingMode   string
		warningGroupingMode string
		infoGroupingMode    string
		maxInfoIssues       int32
	}
	rows, err := rawdb.QueryContext(ctx, `
		SELECT project_id, warning_as_error, max_events_per_issue, issue_display_mode, error_grouping_mode, warning_grouping_mode, info_grouping_mode, max_info_issues
		FROM project_issue_settings
	`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var row issueRow
		if err := rows.Scan(&row.projectID, &row.warningAsError, &row.maxEventsPerIssue, &row.issueDisplayMode, &row.errorGroupingMode, &row.warningGroupingMode, &row.infoGroupingMode, &row.maxInfoIssues); err != nil {
			rows.Close()
			return nil, err
		}
		item := result[row.projectID]
		item.WarningAsError = row.warningAsError
		item.MaxEventsPerIssue = row.maxEventsPerIssue
		item.IssueDisplayMode = row.issueDisplayMode
		item.ErrorGroupingMode = row.errorGroupingMode
		item.WarningGroupingMode = row.warningGroupingMode
		item.InfoGroupingMode = row.infoGroupingMode
		item.MaxInfoIssues = row.maxInfoIssues
		result[row.projectID] = item
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}

	rows, err = rawdb.QueryContext(ctx, `
		SELECT project_id, base_url, email, api_token, project_key, issue_type
		FROM project_jira_settings
	`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var id uuid.UUID
		var baseURL, email, apiToken, projectKey, issueType string
		if err := rows.Scan(&id, &baseURL, &email, &apiToken, &projectKey, &issueType); err != nil {
			rows.Close()
			return nil, err
		}
		item := result[id]
		item.JiraBaseURL = baseURL
		item.JiraEmail = email
		item.JiraAPIToken = apiToken
		item.JiraProjectKey = projectKey
		item.JiraIssueType = issueType
		result[id] = item
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}

	rows, err = rawdb.QueryContext(ctx, `
		SELECT project_id, token, owner, repo, labels
		FROM project_github_settings
	`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var id uuid.UUID
		var token, owner, repo, labels string
		if err := rows.Scan(&id, &token, &owner, &repo, &labels); err != nil {
			rows.Close()
			return nil, err
		}
		item := result[id]
		item.GithubToken = token
		item.GithubOwner = owner
		item.GithubRepo = repo
		item.GithubLabels = labels
		result[id] = item
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}

	rows, err = rawdb.QueryContext(ctx, `
		SELECT project_id, provider, owner, name, default_branch, token, path_strip
		FROM project_repo_settings
	`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var id uuid.UUID
		var provider, owner, name, defaultBranch, token, pathStrip string
		if err := rows.Scan(&id, &provider, &owner, &name, &defaultBranch, &token, &pathStrip); err != nil {
			rows.Close()
			return nil, err
		}
		item := result[id]
		item.RepoProvider = provider
		item.RepoOwner = owner
		item.RepoName = name
		item.RepoDefaultBranch = defaultBranch
		item.RepoToken = token
		item.RepoPathStrip = pathStrip
		result[id] = item
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}

	rows, err = rawdb.QueryContext(ctx, `
		SELECT project_id, enabled, model, merge_suggestions, auto_merge, anomaly_detection, ticket_description, root_cause, triage
		FROM project_ai_settings
	`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var id uuid.UUID
		var enabled, mergeSuggestions, autoMerge, anomalyDetection, ticketDescription, rootCause, triage bool
		var model string
		if err := rows.Scan(&id, &enabled, &model, &mergeSuggestions, &autoMerge, &anomalyDetection, &ticketDescription, &rootCause, &triage); err != nil {
			rows.Close()
			return nil, err
		}
		item := result[id]
		item.AIEnabled = enabled
		item.AIModel = model
		item.AIMergeSuggestions = mergeSuggestions
		item.AIAutoMerge = autoMerge
		item.AIAnomalyDetection = anomalyDetection
		item.AITicketDescription = ticketDescription
		item.AIRootCause = rootCause
		item.AITriage = triage
		result[id] = item
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}

	rows, err = rawdb.QueryContext(ctx, `
		SELECT project_id, rules
		FROM project_stacktrace_settings
	`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var id uuid.UUID
		var rules json.RawMessage
		if err := rows.Scan(&id, &rules); err != nil {
			rows.Close()
			return nil, err
		}
		item := result[id]
		item.StacktraceRules = rules
		result[id] = item
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}

	rows, err = rawdb.QueryContext(ctx, `
		SELECT project_id, enabled, driver, dsn, name, schema, notes
		FROM project_db_analysis_settings
	`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var id uuid.UUID
		var enabled bool
		var driver, dsn, name, schema, notes string
		if err := rows.Scan(&id, &enabled, &driver, &dsn, &name, &schema, &notes); err != nil {
			rows.Close()
			return nil, err
		}
		item := result[id]
		item.AnalysisDBEnabled = enabled
		item.AnalysisDBDriver = driver
		item.AnalysisDBDSN = dsn
		item.AnalysisDBName = name
		item.AnalysisDBSchema = schema
		item.AnalysisDBNotes = notes
		result[id] = item
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}

	rows, err = rawdb.QueryContext(ctx, `
		SELECT project_id, framework, enabled
		FROM project_route_settings
	`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var id uuid.UUID
		var framework string
		var enabled bool
		if err := rows.Scan(&id, &framework, &enabled); err != nil {
			rows.Close()
			return nil, err
		}
		item := result[id]
		item.Framework = framework
		item.RouteGroupingEnabled = enabled
		result[id] = item
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}

	for id, item := range result {
		item.normalizeDefaults()
		result[id] = item
	}

	return result, nil
}

func saveProjectSettings(ctx context.Context, queries *db.Queries, projectID uuid.UUID, s ProjectSettings) error {
	s.normalizeDefaults()
	rawdb := queries.RawDB()

	steps := []struct {
		sql  string
		args []any
	}{
		{
			sql: `
				INSERT INTO project_issue_settings (
					project_id, warning_as_error, max_events_per_issue, issue_display_mode, error_grouping_mode, warning_grouping_mode, info_grouping_mode, max_info_issues, updated_at
				) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, now())
				ON CONFLICT (project_id) DO UPDATE SET
					warning_as_error = EXCLUDED.warning_as_error,
					max_events_per_issue = EXCLUDED.max_events_per_issue,
					issue_display_mode = EXCLUDED.issue_display_mode,
					error_grouping_mode = EXCLUDED.error_grouping_mode,
					warning_grouping_mode = EXCLUDED.warning_grouping_mode,
					info_grouping_mode = EXCLUDED.info_grouping_mode,
					max_info_issues = EXCLUDED.max_info_issues,
					updated_at = now()
			`,
			args: []any{projectID, s.WarningAsError, s.MaxEventsPerIssue, s.IssueDisplayMode, s.ErrorGroupingMode, s.WarningGroupingMode, s.InfoGroupingMode, s.MaxInfoIssues},
		},
		{
			sql: `
				INSERT INTO project_jira_settings (
					project_id, base_url, email, api_token, project_key, issue_type, updated_at
				) VALUES ($1, $2, $3, $4, $5, $6, now())
				ON CONFLICT (project_id) DO UPDATE SET
					base_url = EXCLUDED.base_url,
					email = EXCLUDED.email,
					api_token = EXCLUDED.api_token,
					project_key = EXCLUDED.project_key,
					issue_type = EXCLUDED.issue_type,
					updated_at = now()
			`,
			args: []any{projectID, s.JiraBaseURL, s.JiraEmail, s.JiraAPIToken, s.JiraProjectKey, s.JiraIssueType},
		},
		{
			sql: `
				INSERT INTO project_github_settings (
					project_id, token, owner, repo, labels, updated_at
				) VALUES ($1, $2, $3, $4, $5, now())
				ON CONFLICT (project_id) DO UPDATE SET
					token = EXCLUDED.token,
					owner = EXCLUDED.owner,
					repo = EXCLUDED.repo,
					labels = EXCLUDED.labels,
					updated_at = now()
			`,
			args: []any{projectID, s.GithubToken, s.GithubOwner, s.GithubRepo, s.GithubLabels},
		},
		{
			sql: `
				INSERT INTO project_repo_settings (
					project_id, provider, owner, name, default_branch, token, path_strip, updated_at
				) VALUES ($1, $2, $3, $4, $5, $6, $7, now())
				ON CONFLICT (project_id) DO UPDATE SET
					provider = EXCLUDED.provider,
					owner = EXCLUDED.owner,
					name = EXCLUDED.name,
					default_branch = EXCLUDED.default_branch,
					token = EXCLUDED.token,
					path_strip = EXCLUDED.path_strip,
					updated_at = now()
			`,
			args: []any{projectID, s.RepoProvider, s.RepoOwner, s.RepoName, s.RepoDefaultBranch, s.RepoToken, s.RepoPathStrip},
		},
		{
			sql: `
				INSERT INTO project_ai_settings (
					project_id, enabled, model, merge_suggestions, auto_merge, anomaly_detection, ticket_description, root_cause, triage, updated_at
				) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, now())
				ON CONFLICT (project_id) DO UPDATE SET
					enabled = EXCLUDED.enabled,
					model = EXCLUDED.model,
					merge_suggestions = EXCLUDED.merge_suggestions,
					auto_merge = EXCLUDED.auto_merge,
					anomaly_detection = EXCLUDED.anomaly_detection,
					ticket_description = EXCLUDED.ticket_description,
					root_cause = EXCLUDED.root_cause,
					triage = EXCLUDED.triage,
					updated_at = now()
			`,
			args: []any{projectID, s.AIEnabled, s.AIModel, s.AIMergeSuggestions, s.AIAutoMerge, s.AIAnomalyDetection, s.AITicketDescription, s.AIRootCause, s.AITriage},
		},
		{
			sql: `
				INSERT INTO project_stacktrace_settings (
					project_id, rules, updated_at
				) VALUES ($1, $2, now())
				ON CONFLICT (project_id) DO UPDATE SET
					rules = EXCLUDED.rules,
					updated_at = now()
			`,
			args: []any{projectID, s.StacktraceRules},
		},
		{
			sql: `
				INSERT INTO project_db_analysis_settings (
					project_id, enabled, driver, dsn, name, schema, notes, updated_at
				) VALUES ($1, $2, $3, $4, $5, $6, $7, now())
				ON CONFLICT (project_id) DO UPDATE SET
					enabled = EXCLUDED.enabled,
					driver = EXCLUDED.driver,
					dsn = EXCLUDED.dsn,
					name = EXCLUDED.name,
					schema = EXCLUDED.schema,
					notes = EXCLUDED.notes,
					updated_at = now()
			`,
			args: []any{projectID, s.AnalysisDBEnabled, s.AnalysisDBDriver, s.AnalysisDBDSN, s.AnalysisDBName, s.AnalysisDBSchema, s.AnalysisDBNotes},
		},
		{
			sql: `
				INSERT INTO project_route_settings (
					project_id, framework, enabled, updated_at
				) VALUES ($1, $2, $3, now())
				ON CONFLICT (project_id) DO UPDATE SET
					framework = EXCLUDED.framework,
					enabled = EXCLUDED.enabled,
					updated_at = now()
			`,
			args: []any{projectID, s.Framework, s.RouteGroupingEnabled},
		},
	}

	for _, step := range steps {
		if _, err := rawdb.ExecContext(ctx, step.sql, step.args...); err != nil {
			return err
		}
	}

	return nil
}
