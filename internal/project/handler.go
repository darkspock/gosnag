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
	cache   *StatsCache
}

func NewHandler(queries *db.Queries, cache *StatsCache) *Handler {
	return &Handler{queries: queries, cache: cache}
}

type CreateProjectRequest struct {
	Name                   string `json:"name"`
	Slug                   string `json:"slug"`
	DefaultCooldownMinutes *int32 `json:"default_cooldown_minutes,omitempty"`
	WarningAsError         *bool  `json:"warning_as_error,omitempty"`
	MaxEventsPerIssue      *int32 `json:"max_events_per_issue,omitempty"`
	JiraBaseURL            string `json:"jira_base_url"`
	JiraEmail              string `json:"jira_email"`
	JiraAPIToken           string `json:"jira_api_token"`
	JiraProjectKey         string  `json:"jira_project_key"`
	JiraIssueType          string  `json:"jira_issue_type"`
	GroupID                *string `json:"group_id,omitempty"`
}

// SafeProject strips sensitive fields (Jira API token) from the project for API responses.
type SafeProject struct {
	ID                     uuid.UUID `json:"id"`
	Name                   string    `json:"name"`
	Slug                   string    `json:"slug"`
	DefaultCooldownMinutes int32     `json:"default_cooldown_minutes"`
	WarningAsError         bool      `json:"warning_as_error"`
	MaxEventsPerIssue      int32     `json:"max_events_per_issue"`
	JiraBaseURL            string    `json:"jira_base_url"`
	JiraEmail              string    `json:"jira_email"`
	JiraAPITokenSet        bool      `json:"jira_api_token_set"` // true if configured, never expose the value
	JiraProjectKey         string    `json:"jira_project_key"`
	JiraIssueType          string        `json:"jira_issue_type"`
	GroupID                *string       `json:"group_id"`
	CreatedAt              time.Time     `json:"created_at"`
	UpdatedAt              time.Time     `json:"updated_at"`
}

func nullUUIDToStringPtr(u uuid.NullUUID) *string {
	if !u.Valid {
		return nil
	}
	s := u.UUID.String()
	return &s
}

func toSafeProject(p db.Project) SafeProject {
	return SafeProject{
		ID:                     p.ID,
		Name:                   p.Name,
		Slug:                   p.Slug,
		DefaultCooldownMinutes: p.DefaultCooldownMinutes,
		WarningAsError:         p.WarningAsError,
		MaxEventsPerIssue:      p.MaxEventsPerIssue,
		JiraBaseURL:            p.JiraBaseUrl,
		JiraEmail:              p.JiraEmail,
		JiraAPITokenSet:        p.JiraApiToken != "",
		JiraProjectKey:         p.JiraProjectKey,
		JiraIssueType:          p.JiraIssueType,
		GroupID:                nullUUIDToStringPtr(p.GroupID),
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

	warningAsError := existing.WarningAsError
	if req.WarningAsError != nil {
		warningAsError = *req.WarningAsError
	}

	jiraBaseURL := req.JiraBaseURL
	if jiraBaseURL == "" {
		jiraBaseURL = existing.JiraBaseUrl
	}
	jiraEmail := req.JiraEmail
	if jiraEmail == "" {
		jiraEmail = existing.JiraEmail
	}
	jiraProjectKey := req.JiraProjectKey
	if jiraProjectKey == "" {
		jiraProjectKey = existing.JiraProjectKey
	}
	jiraIssueType := req.JiraIssueType
	if jiraIssueType == "" {
		jiraIssueType = existing.JiraIssueType
	}
	if jiraIssueType == "" {
		jiraIssueType = "Bug"
	}
	jiraApiToken := req.JiraAPIToken
	if jiraApiToken == "" {
		jiraApiToken = existing.JiraApiToken
	}

	maxEvents := existing.MaxEventsPerIssue
	if req.MaxEventsPerIssue != nil {
		maxEvents = *req.MaxEventsPerIssue
	}

	project, err := h.queries.UpdateProject(r.Context(), db.UpdateProjectParams{
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
	})
	if err != nil {
		if err == sql.ErrNoRows {
			writeError(w, http.StatusNotFound, "project not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to update project")
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
		_ = h.queries.SetProjectGroup(r.Context(), db.SetProjectGroupParams{
			ID:      id,
			GroupID: groupID,
		})
		project.GroupID = groupID
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
