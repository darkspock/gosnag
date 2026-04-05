package project

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"
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

type CreateProjectRequest struct {
	Name                   string `json:"name"`
	Slug                   string `json:"slug"`
	DefaultCooldownMinutes *int32 `json:"default_cooldown_minutes,omitempty"`
	WarningAsError         *bool  `json:"warning_as_error,omitempty"`
	JiraBaseURL            string `json:"jira_base_url"`
	JiraEmail              string `json:"jira_email"`
	JiraAPIToken           string `json:"jira_api_token"`
	JiraProjectKey         string `json:"jira_project_key"`
	JiraIssueType          string `json:"jira_issue_type"`
}

// SafeProject strips sensitive fields (Jira API token) from the project for API responses.
type SafeProject struct {
	ID                     uuid.UUID `json:"id"`
	Name                   string    `json:"name"`
	Slug                   string    `json:"slug"`
	DefaultCooldownMinutes int32     `json:"default_cooldown_minutes"`
	WarningAsError         bool      `json:"warning_as_error"`
	JiraBaseURL            string    `json:"jira_base_url"`
	JiraEmail              string    `json:"jira_email"`
	JiraAPITokenSet        bool      `json:"jira_api_token_set"` // true if configured, never expose the value
	JiraProjectKey         string    `json:"jira_project_key"`
	JiraIssueType          string    `json:"jira_issue_type"`
	CreatedAt              time.Time `json:"created_at"`
	UpdatedAt              time.Time `json:"updated_at"`
}

func toSafeProject(p db.Project) SafeProject {
	return SafeProject{
		ID:                     p.ID,
		Name:                   p.Name,
		Slug:                   p.Slug,
		DefaultCooldownMinutes: p.DefaultCooldownMinutes,
		WarningAsError:         p.WarningAsError,
		JiraBaseURL:            p.JiraBaseUrl,
		JiraEmail:              p.JiraEmail,
		JiraAPITokenSet:        p.JiraApiToken != "",
		JiraProjectKey:         p.JiraProjectKey,
		JiraIssueType:          p.JiraIssueType,
		CreatedAt:              p.CreatedAt,
		UpdatedAt:              p.UpdatedAt,
	}
}

type ProjectResponse struct {
	SafeProject
	DSN string `json:"dsn,omitempty"`
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

	dsn := buildDSN(r, key.PublicKey, project.ID)
	writeJSON(w, http.StatusCreated, ProjectResponse{SafeProject: toSafeProject(project), DSN: dsn})
}

type ProjectListItem struct {
	SafeProject
	TotalIssues    int32   `json:"total_issues"`
	OpenIssues     int32   `json:"open_issues"`
	LatestEvent    string  `json:"latest_event,omitempty"`
	Trend          []int32 `json:"trend"`
	LatestRelease  string  `json:"latest_release,omitempty"`
	ErrorsThisWeek int32  `json:"errors_this_week"`
	ErrorsLastWeek int32  `json:"errors_last_week"`
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	projects, err := h.queries.ListProjects(ctx)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list projects")
		return
	}

	stats, _ := h.queries.GetProjectStats(ctx)
	statsMap := make(map[uuid.UUID]db.GetProjectStatsRow)
	for _, s := range stats {
		statsMap[s.ProjectID] = s
	}

	trendRows, _ := h.queries.GetProjectEventTrend(ctx)
	releaseRows, _ := h.queries.GetProjectLatestRelease(ctx)
	weeklyRows, _ := h.queries.GetProjectWeeklyErrors(ctx)

	// Build trend map (14 daily buckets per project)
	now := time.Now().UTC().Truncate(24 * time.Hour)
	trendMap := make(map[uuid.UUID][]int32)
	for _, tr := range trendRows {
		daysAgo := int(now.Sub(tr.Bucket.UTC().Truncate(24*time.Hour)).Hours() / 24)
		if daysAgo < 0 || daysAgo >= 14 {
			continue
		}
		if trendMap[tr.ProjectID] == nil {
			trendMap[tr.ProjectID] = make([]int32, 14)
		}
		trendMap[tr.ProjectID][13-daysAgo] = tr.Count
	}

	releaseMap := make(map[uuid.UUID]string)
	for _, r := range releaseRows {
		releaseMap[r.ProjectID] = r.Release
	}

	weeklyMap := make(map[uuid.UUID]db.GetProjectWeeklyErrorsRow)
	for _, w := range weeklyRows {
		weeklyMap[w.ProjectID] = w
	}

	result := make([]ProjectListItem, len(projects))
	for i, p := range projects {
		item := ProjectListItem{SafeProject: toSafeProject(p), Trend: make([]int32, 14)}
		if s, ok := statsMap[p.ID]; ok {
			item.TotalIssues = s.TotalIssues
			item.OpenIssues = s.OpenIssues
			if t, ok := s.LatestEvent.(time.Time); ok {
				item.LatestEvent = t.Format(time.RFC3339)
			}
		}
		if t, ok := trendMap[p.ID]; ok {
			item.Trend = t
		}
		item.LatestRelease = releaseMap[p.ID]
		if w, ok := weeklyMap[p.ID]; ok {
			item.ErrorsThisWeek = w.ThisWeek
			item.ErrorsLastWeek = w.LastWeek
		}
		result[i] = item
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
	if len(keys) > 0 {
		dsn = buildDSN(r, keys[0].PublicKey, project.ID)
	}

	writeJSON(w, http.StatusOK, ProjectResponse{SafeProject: toSafeProject(project), DSN: dsn})
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

	cooldown := int32(30)
	if req.DefaultCooldownMinutes != nil {
		cooldown = *req.DefaultCooldownMinutes
	}

	warningAsError := false
	if req.WarningAsError != nil {
		warningAsError = *req.WarningAsError
	}

	jiraIssueType := req.JiraIssueType
	if jiraIssueType == "" {
		jiraIssueType = "Bug"
	}

	// Preserve existing Jira API token if not provided
	jiraApiToken := req.JiraAPIToken
	if jiraApiToken == "" {
		existing, err := h.queries.GetProject(r.Context(), id)
		if err == nil {
			jiraApiToken = existing.JiraApiToken
		}
	}

	project, err := h.queries.UpdateProject(r.Context(), db.UpdateProjectParams{
		ID:                     id,
		Name:                   req.Name,
		Slug:                   req.Slug,
		DefaultCooldownMinutes: cooldown,
		WarningAsError:         warningAsError,
		JiraBaseUrl:            req.JiraBaseURL,
		JiraEmail:              req.JiraEmail,
		JiraApiToken:           jiraApiToken,
		JiraProjectKey:         req.JiraProjectKey,
		JiraIssueType:          jiraIssueType,
	})
	if err != nil {
		if err == sql.ErrNoRows {
			writeError(w, http.StatusNotFound, "project not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to update project")
		return
	}

	writeJSON(w, http.StatusOK, toSafeProject(project))
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

	w.WriteHeader(http.StatusNoContent)
}

func generateKeyPair() (string, string) {
	pub := make([]byte, 16)
	sec := make([]byte, 32)
	rand.Read(pub)
	rand.Read(sec)
	return hex.EncodeToString(pub), hex.EncodeToString(sec)
}

func buildDSN(r *http.Request, publicKey string, projectID uuid.UUID) string {
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
