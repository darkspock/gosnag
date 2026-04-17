package project

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/darkspock/gosnag/internal/database/db"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type Handler struct {
	queries *db.Queries
	cache   *StatsCache
}

func NewHandler(queries *db.Queries, cache *StatsCache) *Handler {
	return &Handler{queries: queries, cache: cache}
}

type CreateProjectRequest struct {
	Name                   string           `json:"name"`
	Slug                   string           `json:"slug"`
	DefaultCooldownMinutes *int32           `json:"default_cooldown_minutes,omitempty"`
	WarningAsError         *bool            `json:"warning_as_error,omitempty"`
	MaxEventsPerIssue      *int32           `json:"max_events_per_issue,omitempty"`
	MaxInfoIssues          *int32           `json:"max_info_issues,omitempty"`
	Icon                   *string          `json:"icon,omitempty"`
	Color                  *string          `json:"color,omitempty"`
	IssueDisplayMode       string           `json:"issue_display_mode"`
	ErrorGroupingMode      string           `json:"error_grouping_mode"`
	WarningGroupingMode    string           `json:"warning_grouping_mode"`
	InfoGroupingMode       string           `json:"info_grouping_mode"`
	JiraBaseURL            string           `json:"jira_base_url"`
	JiraEmail              string           `json:"jira_email"`
	JiraAPIToken           string           `json:"jira_api_token"`
	JiraProjectKey         string           `json:"jira_project_key"`
	JiraIssueType          string           `json:"jira_issue_type"`
	GithubToken            string           `json:"github_token"`
	GithubOwner            string           `json:"github_owner"`
	GithubRepo             string           `json:"github_repo"`
	GithubLabels           string           `json:"github_labels"`
	WorkflowMode           string           `json:"workflow_mode"`
	RepoProvider           string           `json:"repo_provider"`
	RepoOwner              string           `json:"repo_owner"`
	RepoName               string           `json:"repo_name"`
	RepoDefaultBranch      string           `json:"repo_default_branch"`
	RepoToken              string           `json:"repo_token"`
	RepoPathStrip          string           `json:"repo_path_strip"`
	GroupID                *string          `json:"group_id,omitempty"`
	AIEnabled              *bool            `json:"ai_enabled,omitempty"`
	AIModel                string           `json:"ai_model"`
	AIMergeSuggestions     *bool            `json:"ai_merge_suggestions,omitempty"`
	AIAutoMerge            *bool            `json:"ai_auto_merge,omitempty"`
	AIAnomalyDetection     *bool            `json:"ai_anomaly_detection,omitempty"`
	AITicketDescription    *bool            `json:"ai_ticket_description,omitempty"`
	AIRootCause            *bool            `json:"ai_root_cause,omitempty"`
	AITriage               *bool            `json:"ai_triage,omitempty"`
	StacktraceRules        *StacktraceRules `json:"stacktrace_rules,omitempty"`
	AnalysisDBEnabled      *bool            `json:"analysis_db_enabled,omitempty"`
	AnalysisDBDriver       *string          `json:"analysis_db_driver,omitempty"`
	AnalysisDBDSN          *string          `json:"analysis_db_dsn,omitempty"`
	AnalysisDBName         *string          `json:"analysis_db_name,omitempty"`
	AnalysisDBSchema       *string          `json:"analysis_db_schema,omitempty"`
	AnalysisDBNotes        *string          `json:"analysis_db_notes,omitempty"`
	Framework              *string          `json:"framework,omitempty"`
	RouteGroupingEnabled   *bool            `json:"route_grouping_enabled,omitempty"`
}

type StacktraceRules struct {
	Preset            string   `json:"preset"`
	AppPatterns       []string `json:"app_patterns"`
	FrameworkPatterns []string `json:"framework_patterns"`
	ExternalPatterns  []string `json:"external_patterns"`
}

// SafeProject strips sensitive fields (Jira API token) from the project for API responses.
type SafeProject struct {
	ID                     uuid.UUID       `json:"id"`
	NumericID              int32           `json:"numeric_id"`
	Name                   string          `json:"name"`
	Slug                   string          `json:"slug"`
	DefaultCooldownMinutes int32           `json:"default_cooldown_minutes"`
	WarningAsError         bool            `json:"warning_as_error"`
	MaxEventsPerIssue      int32           `json:"max_events_per_issue"`
	MaxInfoIssues          int32           `json:"max_info_issues"`
	Icon                   string          `json:"icon"`
	Color                  string          `json:"color"`
	Position               int32           `json:"position"`
	ErrorGroupingMode      string          `json:"error_grouping_mode"`
	WarningGroupingMode    string          `json:"warning_grouping_mode"`
	InfoGroupingMode       string          `json:"info_grouping_mode"`
	JiraBaseURL            string          `json:"jira_base_url"`
	JiraEmail              string          `json:"jira_email"`
	JiraAPITokenSet        bool            `json:"jira_api_token_set"`
	JiraProjectKey         string          `json:"jira_project_key"`
	JiraIssueType          string          `json:"jira_issue_type"`
	GithubTokenSet         bool            `json:"github_token_set"`
	GithubOwner            string          `json:"github_owner"`
	GithubRepo             string          `json:"github_repo"`
	GithubLabels           string          `json:"github_labels"`
	WorkflowMode           string          `json:"workflow_mode"`
	RepoProvider           string          `json:"repo_provider"`
	RepoOwner              string          `json:"repo_owner"`
	RepoName               string          `json:"repo_name"`
	RepoDefaultBranch      string          `json:"repo_default_branch"`
	RepoTokenSet           bool            `json:"repo_token_set"`
	RepoPathStrip          string          `json:"repo_path_strip"`
	IssueDisplayMode       string          `json:"issue_display_mode"`
	GroupID                *string         `json:"group_id"`
	GroupName              string          `json:"group_name,omitempty"`
	AIEnabled              bool            `json:"ai_enabled"`
	AIModel                string          `json:"ai_model"`
	AIMergeSuggestions     bool            `json:"ai_merge_suggestions"`
	AIAutoMerge            bool            `json:"ai_auto_merge"`
	AIAnomalyDetection     bool            `json:"ai_anomaly_detection"`
	AITicketDescription    bool            `json:"ai_ticket_description"`
	AIRootCause            bool            `json:"ai_root_cause"`
	AITriage               bool            `json:"ai_triage"`
	StacktraceRules        StacktraceRules `json:"stacktrace_rules"`
	AnalysisDBEnabled      bool            `json:"analysis_db_enabled"`
	AnalysisDBConfigured   bool            `json:"analysis_db_configured"`
	AnalysisDBDriver       string          `json:"analysis_db_driver"`
	AnalysisDBDSNDisplay   string          `json:"analysis_db_dsn_display"`
	AnalysisDBName         string          `json:"analysis_db_name"`
	AnalysisDBSchema       string          `json:"analysis_db_schema"`
	AnalysisDBNotes        string          `json:"analysis_db_notes"`
	Framework              string          `json:"framework"`
	RouteGroupingEnabled   bool            `json:"route_grouping_enabled"`
	CreatedAt              time.Time       `json:"created_at"`
	UpdatedAt              time.Time       `json:"updated_at"`
}

func defaultStacktraceRules() StacktraceRules {
	return StacktraceRules{
		Preset:            "generic",
		AppPatterns:       []string{},
		FrameworkPatterns: []string{},
		ExternalPatterns:  []string{},
	}
}

func normalizeIssueGroupingMode(mode string) string {
	switch strings.TrimSpace(mode) {
	case "by_url", "by_file":
		return strings.TrimSpace(mode)
	default:
		return "normal"
	}
}

func normalizePatternList(patterns []string) []string {
	out := make([]string, 0, len(patterns))
	for _, pattern := range patterns {
		pattern = strings.TrimSpace(pattern)
		if pattern == "" {
			continue
		}
		out = append(out, pattern)
	}
	return out
}

func normalizeStacktraceRules(r StacktraceRules) StacktraceRules {
	out := defaultStacktraceRules()
	if strings.TrimSpace(r.Preset) != "" {
		out.Preset = strings.TrimSpace(r.Preset)
	}
	out.AppPatterns = normalizePatternList(r.AppPatterns)
	out.FrameworkPatterns = normalizePatternList(r.FrameworkPatterns)
	out.ExternalPatterns = normalizePatternList(r.ExternalPatterns)
	return out
}

func validatePatternList(label string, patterns []string) error {
	for _, pattern := range patterns {
		if _, err := regexp.Compile(pattern); err != nil {
			return fmt.Errorf("invalid %s regex %q: %w", label, pattern, err)
		}
	}
	return nil
}

func validateStacktraceRules(r StacktraceRules) error {
	if err := validatePatternList("app_patterns", r.AppPatterns); err != nil {
		return err
	}
	if err := validatePatternList("framework_patterns", r.FrameworkPatterns); err != nil {
		return err
	}
	if err := validatePatternList("external_patterns", r.ExternalPatterns); err != nil {
		return err
	}
	return nil
}

func parseStacktraceRules(raw json.RawMessage) StacktraceRules {
	if len(raw) == 0 || string(raw) == "null" {
		return defaultStacktraceRules()
	}
	var rules StacktraceRules
	if err := json.Unmarshal(raw, &rules); err != nil {
		return defaultStacktraceRules()
	}
	return normalizeStacktraceRules(rules)
}

func marshalStacktraceRules(r StacktraceRules) json.RawMessage {
	normalized := normalizeStacktraceRules(r)
	raw, err := json.Marshal(normalized)
	if err != nil {
		raw, _ = json.Marshal(defaultStacktraceRules())
	}
	return raw
}

func nullUUIDToStringPtr(u uuid.NullUUID) *string {
	if !u.Valid {
		return nil
	}
	s := u.UUID.String()
	return &s
}

func toSafeProject(p db.Project, settings ProjectSettings) SafeProject {
	return SafeProject{
		ID:                     p.ID,
		NumericID:              p.NumericID,
		Name:                   p.Name,
		Slug:                   p.Slug,
		DefaultCooldownMinutes: p.DefaultCooldownMinutes,
		WarningAsError:         settings.WarningAsError,
		MaxEventsPerIssue:      settings.MaxEventsPerIssue,
		MaxInfoIssues:          settings.MaxInfoIssues,
		Icon:                   p.Icon,
		Color:                  p.Color,
		Position:               p.Position,
		ErrorGroupingMode:      normalizeIssueGroupingMode(settings.ErrorGroupingMode),
		WarningGroupingMode:    normalizeIssueGroupingMode(settings.WarningGroupingMode),
		InfoGroupingMode:       normalizeIssueGroupingMode(settings.InfoGroupingMode),
		JiraBaseURL:            settings.JiraBaseURL,
		JiraEmail:              settings.JiraEmail,
		JiraAPITokenSet:        settings.JiraAPIToken != "",
		JiraProjectKey:         settings.JiraProjectKey,
		JiraIssueType:          settings.JiraIssueType,
		GithubTokenSet:         settings.GithubToken != "",
		GithubOwner:            settings.GithubOwner,
		GithubRepo:             settings.GithubRepo,
		GithubLabels:           settings.GithubLabels,
		WorkflowMode:           p.WorkflowMode,
		RepoProvider:           settings.RepoProvider,
		RepoOwner:              settings.RepoOwner,
		RepoName:               settings.RepoName,
		RepoDefaultBranch:      settings.RepoDefaultBranch,
		RepoTokenSet:           settings.RepoToken != "",
		RepoPathStrip:          settings.RepoPathStrip,
		IssueDisplayMode:       settings.IssueDisplayMode,
		GroupID:                nullUUIDToStringPtr(p.GroupID),
		AIEnabled:              settings.AIEnabled,
		AIModel:                settings.AIModel,
		AIMergeSuggestions:     settings.AIMergeSuggestions,
		AIAutoMerge:            settings.AIAutoMerge,
		AIAnomalyDetection:     settings.AIAnomalyDetection,
		AITicketDescription:    settings.AITicketDescription,
		AIRootCause:            settings.AIRootCause,
		AITriage:               settings.AITriage,
		StacktraceRules:        parseStacktraceRules(settings.StacktraceRules),
		AnalysisDBEnabled:      settings.AnalysisDBEnabled,
		AnalysisDBConfigured:   strings.TrimSpace(settings.AnalysisDBDSN) != "",
		AnalysisDBDriver:       settings.AnalysisDBDriver,
		AnalysisDBDSNDisplay:   maskedAnalysisDSN(settings.AnalysisDBDSN),
		AnalysisDBName:         settings.AnalysisDBName,
		AnalysisDBSchema:       settings.AnalysisDBSchema,
		AnalysisDBNotes:        settings.AnalysisDBNotes,
		Framework:              settings.Framework,
		RouteGroupingEnabled:   settings.RouteGroupingEnabled,
		CreatedAt:              p.CreatedAt,
		UpdatedAt:              p.UpdatedAt,
	}
}

type ProjectResponse struct {
	SafeProject
	DSN       string `json:"dsn,omitempty"`
	LegacyDSN string `json:"legacy_dsn,omitempty"`
}

type ProjectKeyResponse struct {
	db.ProjectKey
	DSN string `json:"dsn"`
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	var req CreateProjectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}

	if req.Slug == "" {
		req.Slug = slugify(req.Name)
	}

	cooldown := int32(30)
	if req.DefaultCooldownMinutes != nil {
		cooldown = *req.DefaultCooldownMinutes
	}

	project, err := h.queries.CreateProject(r.Context(), db.CreateProjectParams{
		Name:                   req.Name,
		Slug:                   req.Slug,
		DefaultCooldownMinutes: cooldown,
	})
	if err != nil {
		if strings.Contains(err.Error(), "duplicate key") {
			writeError(w, http.StatusConflict, "project slug already exists")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to create project")
		return
	}

	// Auto-create a default API key
	pubKey, secKey := generateKeyPair()
	key, err := h.queries.CreateProjectKey(r.Context(), db.CreateProjectKeyParams{
		ProjectID: project.ID,
		PublicKey: pubKey,
		SecretKey: secKey,
		Label:     "Default",
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create project key")
		return
	}

	h.cache.InvalidateSync(r.Context())
	dsn := buildDSN(r, key.PublicKey, project.NumericID)
	legacyDSN := buildLegacyDSN(r, key.PublicKey, project.ID)
	settings, err := loadProjectSettings(r.Context(), h.queries, project)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load project settings")
		return
	}
	writeJSON(w, http.StatusCreated, ProjectResponse{SafeProject: toSafeProject(project, settings), DSN: dsn, LegacyDSN: legacyDSN})
}

type ProjectListItem struct {
	SafeProject
	TotalIssues    int32   `json:"total_issues"`
	OpenIssues     int32   `json:"open_issues"`
	LatestEvent    string  `json:"latest_event,omitempty"`
	Trend          []int32 `json:"trend"`
	LatestRelease  string  `json:"latest_release,omitempty"`
	ErrorsThisWeek int32   `json:"errors_this_week"`
	ErrorsLastWeek int32   `json:"errors_last_week"`
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	result, err := h.cache.Get(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list projects")
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "project_id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid project id")
		return
	}

	project, err := h.queries.GetProject(r.Context(), id)
	if err != nil {
		if err == sql.ErrNoRows {
			writeError(w, http.StatusNotFound, "project not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to get project")
		return
	}

	keys, err := h.queries.ListProjectKeys(r.Context(), project.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get project keys")
		return
	}

	dsn := ""
	legacyDSN := ""
	if len(keys) > 0 {
		dsn = buildDSN(r, keys[0].PublicKey, project.NumericID)
		legacyDSN = buildLegacyDSN(r, keys[0].PublicKey, project.ID)
	}

	settings, err := loadProjectSettings(r.Context(), h.queries, project)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load project settings")
		return
	}
	sp := toSafeProject(project, settings)
	if project.GroupID.Valid {
		if groups, err := h.queries.ListProjectGroups(r.Context()); err == nil {
			for _, g := range groups {
				if g.ID == project.GroupID.UUID {
					sp.GroupName = g.Name
					break
				}
			}
		}
	}
	writeJSON(w, http.StatusOK, ProjectResponse{SafeProject: sp, DSN: dsn, LegacyDSN: legacyDSN})
}

func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "project_id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid project id")
		return
	}

	var req CreateProjectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Load existing project to preserve unset fields
	existing, err := h.queries.GetProject(r.Context(), id)
	if err != nil {
		if err == sql.ErrNoRows {
			writeError(w, http.StatusNotFound, "project not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to get project")
		return
	}

	existingSettings, err := loadProjectSettings(r.Context(), h.queries, existing)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get project settings")
		return
	}

	// Preserve existing values when not provided
	name := req.Name
	if name == "" {
		name = existing.Name
	}
	slug := req.Slug
	if slug == "" {
		slug = existing.Slug
	}

	cooldown := existing.DefaultCooldownMinutes
	if req.DefaultCooldownMinutes != nil {
		cooldown = *req.DefaultCooldownMinutes
	}

	warningAsError := existingSettings.WarningAsError
	if req.WarningAsError != nil {
		warningAsError = *req.WarningAsError
	}

	jiraBaseURL := req.JiraBaseURL
	if jiraBaseURL == "" {
		jiraBaseURL = existingSettings.JiraBaseURL
	}
	jiraEmail := req.JiraEmail
	if jiraEmail == "" {
		jiraEmail = existingSettings.JiraEmail
	}
	jiraProjectKey := req.JiraProjectKey
	if jiraProjectKey == "" {
		jiraProjectKey = existingSettings.JiraProjectKey
	}
	jiraIssueType := req.JiraIssueType
	if jiraIssueType == "" {
		jiraIssueType = existingSettings.JiraIssueType
	}
	if jiraIssueType == "" {
		jiraIssueType = "Bug"
	}
	jiraApiToken := req.JiraAPIToken
	if jiraApiToken == "" {
		jiraApiToken = existingSettings.JiraAPIToken
	}

	githubOwner := req.GithubOwner
	if githubOwner == "" {
		githubOwner = existingSettings.GithubOwner
	}
	githubRepo := req.GithubRepo
	if githubRepo == "" {
		githubRepo = existingSettings.GithubRepo
	}
	githubLabels := req.GithubLabels
	if githubLabels == "" {
		githubLabels = existingSettings.GithubLabels
	}
	if githubLabels == "" {
		githubLabels = "bug"
	}
	githubToken := req.GithubToken
	if githubToken == "" {
		githubToken = existingSettings.GithubToken
	}

	workflowMode := req.WorkflowMode
	if workflowMode == "" {
		workflowMode = existing.WorkflowMode
	}
	if workflowMode != "simple" && workflowMode != "managed" {
		workflowMode = "simple"
	}

	repoProvider := req.RepoProvider
	if repoProvider == "" {
		repoProvider = existingSettings.RepoProvider
	}
	repoOwner := req.RepoOwner
	if repoOwner == "" {
		repoOwner = existingSettings.RepoOwner
	}
	repoName := req.RepoName
	if repoName == "" {
		repoName = existingSettings.RepoName
	}
	repoDefaultBranch := req.RepoDefaultBranch
	if repoDefaultBranch == "" {
		repoDefaultBranch = existingSettings.RepoDefaultBranch
	}
	if repoDefaultBranch == "" {
		repoDefaultBranch = "main"
	}
	repoToken := req.RepoToken
	if repoToken == "" {
		repoToken = existingSettings.RepoToken
	}
	repoPathStrip := req.RepoPathStrip
	if repoPathStrip == "" {
		repoPathStrip = existingSettings.RepoPathStrip
	}

	maxEvents := existingSettings.MaxEventsPerIssue
	if req.MaxEventsPerIssue != nil {
		maxEvents = *req.MaxEventsPerIssue
	}

	maxInfoIssues := existingSettings.MaxInfoIssues
	if req.MaxInfoIssues != nil {
		if *req.MaxInfoIssues < 0 {
			writeError(w, http.StatusBadRequest, "max_info_issues must be >= 0")
			return
		}
		maxInfoIssues = *req.MaxInfoIssues
	}

	icon := existing.Icon
	if req.Icon != nil {
		icon = *req.Icon
	}
	color := existing.Color
	if req.Color != nil {
		color = *req.Color
	}

	issueDisplayMode := existingSettings.IssueDisplayMode
	if req.IssueDisplayMode != "" {
		issueDisplayMode = req.IssueDisplayMode
	}

	errorGroupingMode := normalizeIssueGroupingMode(existingSettings.ErrorGroupingMode)
	if req.ErrorGroupingMode != "" {
		errorGroupingMode = normalizeIssueGroupingMode(req.ErrorGroupingMode)
	}

	warningGroupingMode := normalizeIssueGroupingMode(existingSettings.WarningGroupingMode)
	if req.WarningGroupingMode != "" {
		warningGroupingMode = normalizeIssueGroupingMode(req.WarningGroupingMode)
	}

	infoGroupingMode := normalizeIssueGroupingMode(existingSettings.InfoGroupingMode)
	if req.InfoGroupingMode != "" {
		infoGroupingMode = normalizeIssueGroupingMode(req.InfoGroupingMode)
	}

	aiEnabled := existingSettings.AIEnabled
	if req.AIEnabled != nil {
		aiEnabled = *req.AIEnabled
	}
	aiModel := req.AIModel
	if aiModel == "" {
		aiModel = existingSettings.AIModel
	}
	aiMergeSuggestions := existingSettings.AIMergeSuggestions
	if req.AIMergeSuggestions != nil {
		aiMergeSuggestions = *req.AIMergeSuggestions
	}
	aiAutoMerge := existingSettings.AIAutoMerge
	if req.AIAutoMerge != nil {
		aiAutoMerge = *req.AIAutoMerge
	}
	aiAnomalyDetection := existingSettings.AIAnomalyDetection
	if req.AIAnomalyDetection != nil {
		aiAnomalyDetection = *req.AIAnomalyDetection
	}
	aiTicketDescription := existingSettings.AITicketDescription
	if req.AITicketDescription != nil {
		aiTicketDescription = *req.AITicketDescription
	}
	aiRootCause := existingSettings.AIRootCause
	if req.AIRootCause != nil {
		aiRootCause = *req.AIRootCause
	}
	aiTriage := existingSettings.AITriage
	if req.AITriage != nil {
		aiTriage = *req.AITriage
	}

	stacktraceRules := parseStacktraceRules(existingSettings.StacktraceRules)
	if req.StacktraceRules != nil {
		stacktraceRules = normalizeStacktraceRules(*req.StacktraceRules)
		if err := validateStacktraceRules(stacktraceRules); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
	}

	analysisDBEnabled := existingSettings.AnalysisDBEnabled
	if req.AnalysisDBEnabled != nil {
		analysisDBEnabled = *req.AnalysisDBEnabled
	}
	analysisDBDriver := existingSettings.AnalysisDBDriver
	if req.AnalysisDBDriver != nil {
		analysisDBDriver = strings.TrimSpace(*req.AnalysisDBDriver)
	}
	analysisDBDSN := existingSettings.AnalysisDBDSN
	if req.AnalysisDBDSN != nil {
		analysisDBDSN = strings.TrimSpace(*req.AnalysisDBDSN)
	}
	analysisDBName := existingSettings.AnalysisDBName
	if req.AnalysisDBName != nil {
		analysisDBName = strings.TrimSpace(*req.AnalysisDBName)
	}
	analysisDBSchema := existingSettings.AnalysisDBSchema
	if req.AnalysisDBSchema != nil {
		analysisDBSchema = strings.TrimSpace(*req.AnalysisDBSchema)
	}
	analysisDBNotes := existingSettings.AnalysisDBNotes
	if req.AnalysisDBNotes != nil {
		analysisDBNotes = strings.TrimSpace(*req.AnalysisDBNotes)
	}

	framework := existingSettings.Framework
	if req.Framework != nil {
		framework = strings.TrimSpace(*req.Framework)
	}
	if framework == "" {
		framework = "generic"
	}

	routeGroupingEnabled := existingSettings.RouteGroupingEnabled
	if req.RouteGroupingEnabled != nil {
		routeGroupingEnabled = *req.RouteGroupingEnabled
	}

	rawdb, ok := h.queries.RawDB().(*sql.DB)
	if !ok {
		writeError(w, http.StatusInternalServerError, "project store is not transaction-capable")
		return
	}
	tx, err := rawdb.BeginTx(r.Context(), nil)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to start project update")
		return
	}
	defer tx.Rollback()

	txQueries := h.queries.WithTx(tx)

	project, err := txQueries.UpdateProject(r.Context(), db.UpdateProjectParams{
		ID:                     id,
		Name:                   name,
		Slug:                   slug,
		DefaultCooldownMinutes: cooldown,
		WarningAsError:         warningAsError,
		JiraBaseUrl:            jiraBaseURL,
		JiraEmail:              jiraEmail,
		JiraApiToken:           jiraApiToken,
		JiraProjectKey:         jiraProjectKey,
		JiraIssueType:          jiraIssueType,
		MaxEventsPerIssue:      maxEvents,
		MaxInfoIssues:          maxInfoIssues,
		Icon:                   icon,
		Color:                  color,
		IssueDisplayMode:       issueDisplayMode,
		InfoGroupingMode:       infoGroupingMode,
		GithubToken:            githubToken,
		GithubOwner:            githubOwner,
		GithubRepo:             githubRepo,
		GithubLabels:           githubLabels,
		WorkflowMode:           workflowMode,
		RepoProvider:           repoProvider,
		RepoOwner:              repoOwner,
		RepoName:               repoName,
		RepoDefaultBranch:      repoDefaultBranch,
		RepoToken:              repoToken,
		RepoPathStrip:          repoPathStrip,
		AiEnabled:              aiEnabled,
		AiModel:                aiModel,
		AiMergeSuggestions:     aiMergeSuggestions,
		AiAutoMerge:            aiAutoMerge,
		AiAnomalyDetection:     aiAnomalyDetection,
		AiTicketDescription:    aiTicketDescription,
		AiRootCause:            aiRootCause,
		AiTriage:               aiTriage,
		StacktraceRules:        marshalStacktraceRules(stacktraceRules),
		AnalysisDbEnabled:      analysisDBEnabled,
		AnalysisDbDriver:       analysisDBDriver,
		AnalysisDbDsn:          analysisDBDSN,
		AnalysisDbName:         analysisDBName,
		AnalysisDbSchema:       analysisDBSchema,
		AnalysisDbNotes:        analysisDBNotes,
	})
	if err != nil {
		if err == sql.ErrNoRows {
			writeError(w, http.StatusNotFound, "project not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to update project")
		return
	}

	if err := saveProjectSettings(r.Context(), txQueries, id, ProjectSettings{
		WarningAsError:       warningAsError,
		MaxEventsPerIssue:    maxEvents,
		IssueDisplayMode:     issueDisplayMode,
		ErrorGroupingMode:    errorGroupingMode,
		WarningGroupingMode:  warningGroupingMode,
		InfoGroupingMode:     infoGroupingMode,
		MaxInfoIssues:        maxInfoIssues,
		JiraBaseURL:          jiraBaseURL,
		JiraEmail:            jiraEmail,
		JiraAPIToken:         jiraApiToken,
		JiraProjectKey:       jiraProjectKey,
		JiraIssueType:        jiraIssueType,
		GithubToken:          githubToken,
		GithubOwner:          githubOwner,
		GithubRepo:           githubRepo,
		GithubLabels:         githubLabels,
		RepoProvider:         repoProvider,
		RepoOwner:            repoOwner,
		RepoName:             repoName,
		RepoDefaultBranch:    repoDefaultBranch,
		RepoToken:            repoToken,
		RepoPathStrip:        repoPathStrip,
		AIEnabled:            aiEnabled,
		AIModel:              aiModel,
		AIMergeSuggestions:   aiMergeSuggestions,
		AIAutoMerge:          aiAutoMerge,
		AIAnomalyDetection:   aiAnomalyDetection,
		AITicketDescription:  aiTicketDescription,
		AIRootCause:          aiRootCause,
		AITriage:             aiTriage,
		StacktraceRules:      marshalStacktraceRules(stacktraceRules),
		AnalysisDBEnabled:    analysisDBEnabled,
		AnalysisDBDriver:     analysisDBDriver,
		AnalysisDBDSN:        analysisDBDSN,
		AnalysisDBName:       analysisDBName,
		AnalysisDBSchema:     analysisDBSchema,
		AnalysisDBNotes:      analysisDBNotes,
		Framework:            framework,
		RouteGroupingEnabled: routeGroupingEnabled,
	}); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update project settings")
		return
	}

	// Update group assignment if provided
	if req.GroupID != nil {
		var groupID uuid.NullUUID
		if *req.GroupID != "" {
			parsed, err := uuid.Parse(*req.GroupID)
			if err != nil {
				writeError(w, http.StatusBadRequest, "invalid group_id")
				return
			}
			groupID = uuid.NullUUID{UUID: parsed, Valid: true}
		}
		if err := txQueries.SetProjectGroup(r.Context(), db.SetProjectGroupParams{
			ID:      id,
			GroupID: groupID,
		}); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to update project group")
			return
		}
		project.GroupID = groupID
	}

	if err := tx.Commit(); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to commit project update")
		return
	}

	h.cache.InvalidateSync(r.Context())
	writeJSON(w, http.StatusOK, toSafeProject(project, ProjectSettings{
		WarningAsError:       warningAsError,
		MaxEventsPerIssue:    maxEvents,
		IssueDisplayMode:     issueDisplayMode,
		ErrorGroupingMode:    errorGroupingMode,
		WarningGroupingMode:  warningGroupingMode,
		InfoGroupingMode:     infoGroupingMode,
		MaxInfoIssues:        maxInfoIssues,
		JiraBaseURL:          jiraBaseURL,
		JiraEmail:            jiraEmail,
		JiraAPIToken:         jiraApiToken,
		JiraProjectKey:       jiraProjectKey,
		JiraIssueType:        jiraIssueType,
		GithubToken:          githubToken,
		GithubOwner:          githubOwner,
		GithubRepo:           githubRepo,
		GithubLabels:         githubLabels,
		RepoProvider:         repoProvider,
		RepoOwner:            repoOwner,
		RepoName:             repoName,
		RepoDefaultBranch:    repoDefaultBranch,
		RepoToken:            repoToken,
		RepoPathStrip:        repoPathStrip,
		AIEnabled:            aiEnabled,
		AIModel:              aiModel,
		AIMergeSuggestions:   aiMergeSuggestions,
		AIAutoMerge:          aiAutoMerge,
		AIAnomalyDetection:   aiAnomalyDetection,
		AITicketDescription:  aiTicketDescription,
		AIRootCause:          aiRootCause,
		AITriage:             aiTriage,
		StacktraceRules:      marshalStacktraceRules(stacktraceRules),
		AnalysisDBEnabled:    analysisDBEnabled,
		AnalysisDBDriver:     analysisDBDriver,
		AnalysisDBDSN:        analysisDBDSN,
		AnalysisDBName:       analysisDBName,
		AnalysisDBSchema:     analysisDBSchema,
		AnalysisDBNotes:      analysisDBNotes,
		Framework:            framework,
		RouteGroupingEnabled: routeGroupingEnabled,
	}))
}

func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "project_id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid project id")
		return
	}

	if err := h.queries.DeleteProject(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete project")
		return
	}

	h.cache.InvalidateSync(r.Context())
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) Reorder(w http.ResponseWriter, r *http.Request) {
	var req []struct {
		ID       string `json:"id"`
		Position int32  `json:"position"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	for _, item := range req {
		id, err := uuid.Parse(item.ID)
		if err != nil {
			continue
		}
		h.queries.UpdateProjectPosition(r.Context(), db.UpdateProjectPositionParams{
			ID:       id,
			Position: item.Position,
		})
	}

	h.cache.InvalidateSync(r.Context())
	w.WriteHeader(http.StatusNoContent)
}

func generateKeyPair() (string, string) {
	pub := make([]byte, 16)
	sec := make([]byte, 32)
	rand.Read(pub)
	rand.Read(sec)
	return hex.EncodeToString(pub), hex.EncodeToString(sec)
}

func buildDSN(r *http.Request, publicKey string, numericID int32) string {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	if fwd := r.Header.Get("X-Forwarded-Proto"); fwd != "" {
		scheme = fwd
	}
	return scheme + "://" + publicKey + "@" + r.Host + "/" + fmt.Sprintf("%d", numericID)
}

func buildLegacyDSN(r *http.Request, publicKey string, projectID uuid.UUID) string {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	if fwd := r.Header.Get("X-Forwarded-Proto"); fwd != "" {
		scheme = fwd
	}
	return scheme + "://" + publicKey + "@" + r.Host + "/" + projectID.String()
}

func slugify(s string) string {
	s = strings.ToLower(s)
	s = strings.ReplaceAll(s, " ", "-")
	return s
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
