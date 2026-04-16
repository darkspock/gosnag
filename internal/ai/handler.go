package ai

import (
	"database/sql"
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/darkspock/gosnag/internal/config"
	"github.com/darkspock/gosnag/internal/database/db"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// Handler serves AI-related HTTP endpoints.
type Handler struct {
	queries *db.Queries
	service *Service
	cfg     *config.Config
}

// NewHandler creates an AI handler.
func NewHandler(queries *db.Queries, service *Service, cfg *config.Config) *Handler {
	return &Handler{queries: queries, service: service, cfg: cfg}
}

// GenerateTicketDescription handles POST /projects/{project_id}/tickets/{ticket_id}/generate-description
func (h *Handler) GenerateTicketDescription(w http.ResponseWriter, r *http.Request) {
	projectID, err := uuid.Parse(chi.URLParam(r, "project_id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid project_id")
		return
	}
	ticketID, err := uuid.Parse(chi.URLParam(r, "ticket_id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid ticket_id")
		return
	}

	if h.service == nil {
		writeError(w, http.StatusServiceUnavailable, "AI provider not configured")
		return
	}

	// Verify project has AI enabled
	project, err := h.queries.GetProject(r.Context(), projectID)
	if err != nil {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}
	if !project.AiEnabled || !project.AiTicketDescription {
		writeError(w, http.StatusForbidden, "AI ticket description is disabled for this project")
		return
	}

	// Get the ticket
	ticket, err := h.queries.GetTicket(r.Context(), ticketID)
	if err != nil {
		writeError(w, http.StatusNotFound, "ticket not found")
		return
	}
	if ticket.ProjectID != projectID {
		writeError(w, http.StatusNotFound, "ticket not found")
		return
	}

	var description string
	if ticket.IssueID.Valid {
		description, err = GenerateDescription(r.Context(), h.service, h.queries, projectID, ticket.IssueID.UUID)
	} else {
		description, err = GenerateDescriptionForManualTicket(r.Context(), h.service, projectID, ticket.Title, ticket.Description)
	}
	if err != nil {
		slog.Error("ai: generate description failed", "error", err, "ticket", ticketID)
		if err == ErrBudgetExceeded {
			writeError(w, http.StatusTooManyRequests, "Daily AI token budget exceeded")
			return
		}
		if err == ErrRateLimited {
			writeError(w, http.StatusTooManyRequests, "AI rate limit exceeded, try again in a minute")
			return
		}
		writeError(w, http.StatusServiceUnavailable, userVisibleAIError(err))
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"description": description})
}

// GetMergeSuggestion handles GET /projects/{project_id}/issues/{issue_id}/merge-suggestion
func (h *Handler) GetMergeSuggestion(w http.ResponseWriter, r *http.Request) {
	issueID, err := uuid.Parse(chi.URLParam(r, "issue_id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid issue_id")
		return
	}

	suggestion, err := h.queries.GetPendingMergeSuggestion(r.Context(), issueID)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"suggestion": nil})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"suggestion": map[string]any{
			"id":                 suggestion.ID,
			"issue_id":           suggestion.IssueID,
			"target_issue_id":    suggestion.TargetIssueID,
			"target_issue_title": suggestion.TargetIssueTitle,
			"confidence":         suggestion.Confidence,
			"reason":             suggestion.Reason,
			"status":             suggestion.Status,
			"created_at":         suggestion.CreatedAt,
		},
	})
}

// AcceptMergeSuggestion handles POST /projects/{project_id}/issues/{issue_id}/merge-suggestion/accept
func (h *Handler) AcceptMergeSuggestion(w http.ResponseWriter, r *http.Request) {
	issueID, err := uuid.Parse(chi.URLParam(r, "issue_id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid issue_id")
		return
	}
	projectID, err := uuid.Parse(chi.URLParam(r, "project_id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid project_id")
		return
	}

	suggestion, err := h.queries.GetPendingMergeSuggestion(r.Context(), issueID)
	if err != nil {
		writeError(w, http.StatusNotFound, "no pending suggestion")
		return
	}

	if suggestion.ProjectID != projectID {
		writeError(w, http.StatusNotFound, "no pending suggestion")
		return
	}

	if err := h.queries.AcceptMergeSuggestion(r.Context(), suggestion.ID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to accept suggestion")
		return
	}

	// The actual merge is handled by the caller (frontend calls the existing merge endpoint)
	writeJSON(w, http.StatusOK, map[string]string{"status": "accepted"})
}

// DismissMergeSuggestion handles POST /projects/{project_id}/issues/{issue_id}/merge-suggestion/dismiss
func (h *Handler) DismissMergeSuggestion(w http.ResponseWriter, r *http.Request) {
	issueID, err := uuid.Parse(chi.URLParam(r, "issue_id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid issue_id")
		return
	}

	suggestion, err := h.queries.GetPendingMergeSuggestion(r.Context(), issueID)
	if err != nil {
		writeError(w, http.StatusNotFound, "no pending suggestion")
		return
	}

	if err := h.queries.DismissMergeSuggestion(r.Context(), suggestion.ID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to dismiss suggestion")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "dismissed"})
}

// GetAIStatus handles GET /projects/{project_id}/ai/status — returns AI config visibility info
func (h *Handler) GetAIStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"provider_configured": h.service != nil,
		"provider":            h.service.ProviderName(),
	})
}

// GetTokenUsage handles GET /projects/{project_id}/ai/usage
func (h *Handler) GetTokenUsage(w http.ResponseWriter, r *http.Request) {
	projectID, err := uuid.Parse(chi.URLParam(r, "project_id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid project_id")
		return
	}

	today, err := h.queries.GetProjectAIUsageToday(r.Context(), projectID)
	if err != nil && err != sql.ErrNoRows {
		writeError(w, http.StatusInternalServerError, "failed to get usage")
		return
	}
	week, err := h.queries.GetProjectAIUsageWeek(r.Context(), projectID)
	if err != nil && err != sql.ErrNoRows {
		writeError(w, http.StatusInternalServerError, "failed to get usage")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"today_tokens": today.TotalTokens,
		"today_calls":  today.TotalCalls,
		"week_tokens":  week.TotalTokens,
		"week_calls":   week.TotalCalls,
		"daily_budget": h.cfg.AIMaxTokensPerDay,
	})
}

// AnalyzeIssue handles POST /projects/{project_id}/issues/{issue_id}/analyze
func (h *Handler) AnalyzeIssue(w http.ResponseWriter, r *http.Request) {
	projectID, err := uuid.Parse(chi.URLParam(r, "project_id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid project_id")
		return
	}
	issueID, err := uuid.Parse(chi.URLParam(r, "issue_id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid issue_id")
		return
	}
	if h.service == nil {
		writeError(w, http.StatusServiceUnavailable, "AI provider not configured")
		return
	}

	project, err := h.queries.GetProject(r.Context(), projectID)
	if err != nil {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}
	if !project.AiEnabled || !project.AiRootCause {
		writeError(w, http.StatusForbidden, "AI root cause analysis is disabled for this project")
		return
	}

	analysis, err := AnalyzeRootCause(r.Context(), h.service, h.queries, projectID, issueID)
	if err != nil {
		slog.Error("ai: analyze failed", "error", err, "issue", issueID)
		if err == ErrBudgetExceeded {
			writeError(w, http.StatusTooManyRequests, "Daily AI token budget exceeded")
			return
		}
		if err == ErrRateLimited {
			writeError(w, http.StatusTooManyRequests, "AI rate limit exceeded, try again in a minute")
			return
		}
		writeError(w, http.StatusServiceUnavailable, userVisibleAIError(err))
		return
	}

	writeJSON(w, http.StatusOK, analysisToJSON(analysis))
}

// GetAnalysis handles GET /projects/{project_id}/issues/{issue_id}/analysis
func (h *Handler) GetAnalysis(w http.ResponseWriter, r *http.Request) {
	issueID, err := uuid.Parse(chi.URLParam(r, "issue_id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid issue_id")
		return
	}

	analysis, err := h.queries.GetLatestAIAnalysis(r.Context(), issueID)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"analysis": nil})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"analysis": analysisToJSON(&analysis)})
}

// ListAnalyses handles GET /projects/{project_id}/issues/{issue_id}/analyses
func (h *Handler) ListAnalyses(w http.ResponseWriter, r *http.Request) {
	issueID, err := uuid.Parse(chi.URLParam(r, "issue_id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid issue_id")
		return
	}

	analyses, err := h.queries.ListAIAnalyses(r.Context(), issueID)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"analyses": []any{}})
		return
	}

	result := make([]map[string]any, len(analyses))
	for i, a := range analyses {
		result[i] = analysisToJSON(&a)
	}
	writeJSON(w, http.StatusOK, map[string]any{"analyses": result})
}

// GetDeployAnalysis handles GET /projects/{project_id}/deploys/{deploy_id}/analysis
func (h *Handler) GetDeployAnalysis(w http.ResponseWriter, r *http.Request) {
	deployID, err := uuid.Parse(chi.URLParam(r, "deploy_id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid deploy_id")
		return
	}

	analysis, err := h.queries.GetDeployAnalysis(r.Context(), deployID)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"analysis": nil})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"analysis": deployAnalysisToJSON(analysis),
	})
}

// GetLatestDeployHealth handles GET /projects/{project_id}/deploy-health
func (h *Handler) GetLatestDeployHealth(w http.ResponseWriter, r *http.Request) {
	projectID, err := uuid.Parse(chi.URLParam(r, "project_id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid project_id")
		return
	}

	analysis, err := h.queries.GetLatestDeployAnalysisByProject(r.Context(), projectID)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"analysis": nil})
		return
	}

	// Only return if critical or warning
	if analysis.Severity != "critical" && analysis.Severity != "warning" {
		writeJSON(w, http.StatusOK, map[string]any{"analysis": nil})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"analysis": deployAnalysisToJSON(analysis),
	})
}

func analysisToJSON(a *db.AiAnalysis) map[string]any {
	var evidence []string
	json.Unmarshal([]byte(a.Evidence), &evidence)
	return map[string]any{
		"id":            a.ID,
		"issue_id":      a.IssueID,
		"project_id":    a.ProjectID,
		"summary":       a.Summary,
		"evidence":      evidence,
		"suggested_fix": a.SuggestedFix,
		"model":         a.Model,
		"version":       a.Version,
		"created_at":    a.CreatedAt,
	}
}

func deployAnalysisToJSON(a db.DeployAnalysis) map[string]any {
	return map[string]any{
		"id":                    a.ID,
		"deploy_id":             a.DeployID,
		"project_id":            a.ProjectID,
		"severity":              a.Severity,
		"summary":               a.Summary,
		"details":               a.Details,
		"likely_deploy_caused":  a.LikelyDeployCaused,
		"recommended_action":    a.RecommendedAction,
		"new_issues_count":      a.NewIssuesCount,
		"spiked_issues_count":   a.SpikedIssuesCount,
		"reopened_issues_count": a.ReopenedIssuesCount,
		"created_at":            a.CreatedAt,
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
