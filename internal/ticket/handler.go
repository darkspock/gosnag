package ticket

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/darkspock/gosnag/internal/activity"
	"github.com/darkspock/gosnag/internal/auth"
	"github.com/darkspock/gosnag/internal/database/db"
	"github.com/darkspock/gosnag/internal/workflow"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// NotifyFunc is called to notify followers of changes.
type NotifyFunc func(issueID, projectID uuid.UUID, issueTitle, action string, excludeUserID *uuid.UUID)

// AlertFunc sends project-level alerts (email/slack) for an issue event.
type AlertFunc func(projectID uuid.UUID, issue db.Issue, isNew bool)

// FileDeleter can delete a stored file by URL.
type FileDeleter interface {
	Delete(ctx context.Context, url string) error
}

type Handler struct {
	queries     *db.Queries
	notifyFn    NotifyFunc
	alertFn     AlertFunc
	fileDeleter FileDeleter
}

func NewHandler(queries *db.Queries, notifyFn NotifyFunc, alertFn AlertFunc, fileDeleter ...FileDeleter) *Handler {
	h := &Handler{queries: queries, notifyFn: notifyFn, alertFn: alertFn}
	if len(fileDeleter) > 0 {
		h.fileDeleter = fileDeleter[0]
	}
	return h
}

// Create creates a ticket for an issue.
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
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

	user := auth.GetUserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	// Check project has managed workflow
	project, err := h.queries.GetProject(r.Context(), projectID)
	if err != nil {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}
	if project.WorkflowMode != workflow.ModeManaged {
		writeError(w, http.StatusBadRequest, "tickets require managed workflow mode")
		return
	}

	// Check issue exists and belongs to project
	issue, err := h.queries.GetIssue(r.Context(), issueID)
	if err != nil || issue.ProjectID != projectID {
		writeError(w, http.StatusNotFound, "issue not found")
		return
	}

	// Check no active ticket exists
	if existing, err := h.queries.GetTicketByIssue(r.Context(), uuid.NullUUID{UUID: issueID, Valid: true}); err == nil {
		writeJSON(w, http.StatusConflict, ticketJSON(existing))
		return
	}

	var req struct {
		Priority int `json:"priority,omitempty"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	if req.Priority == 0 {
		req.Priority = 50
	}

	ticket, err := h.queries.CreateTicket(r.Context(), db.CreateTicketParams{
		IssueID:    uuid.NullUUID{UUID: issueID, Valid: true},
		ProjectID:  projectID,
		Status:     workflow.StatusAcknowledged,
		AssignedTo: uuid.NullUUID{UUID: user.ID, Valid: true},
		CreatedBy:  user.ID,
		Priority:   int32(req.Priority),
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create ticket")
		return
	}

	activity.Record(r.Context(), h.queries, issueID, &ticket.ID, &user.ID, "ticket_created", "", workflow.StatusAcknowledged, nil)

	if h.notifyFn != nil {
		go h.notifyFn(issueID, projectID, issue.Title, "Ticket created", &user.ID)
	}

	writeJSON(w, http.StatusCreated, ticketJSON(ticket))
}

// CreateManual creates a standalone ticket (not linked to an issue).
func (h *Handler) CreateManual(w http.ResponseWriter, r *http.Request) {
	projectID, err := uuid.Parse(chi.URLParam(r, "project_id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid project id")
		return
	}

	user := auth.GetUserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	project, err := h.queries.GetProject(r.Context(), projectID)
	if err != nil {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}
	if project.WorkflowMode != workflow.ModeManaged {
		writeError(w, http.StatusBadRequest, "tickets require managed workflow mode")
		return
	}

	var req struct {
		Title       string  `json:"title"`
		Description string  `json:"description"`
		Priority    int     `json:"priority,omitempty"`
		AssignedTo  *string `json:"assigned_to,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Title == "" {
		writeError(w, http.StatusBadRequest, "title is required")
		return
	}
	if req.Priority == 0 {
		req.Priority = 50
	}

	assignedTo := uuid.NullUUID{UUID: user.ID, Valid: true}
	if req.AssignedTo != nil && *req.AssignedTo != "" {
		uid, err := uuid.Parse(*req.AssignedTo)
		if err == nil {
			assignedTo = uuid.NullUUID{UUID: uid, Valid: true}
		}
	}

	ticket, err := h.queries.CreateTicket(r.Context(), db.CreateTicketParams{
		IssueID:     uuid.NullUUID{}, // no issue
		ProjectID:   projectID,
		Status:      workflow.StatusAcknowledged,
		AssignedTo:  assignedTo,
		CreatedBy:   user.ID,
		Priority:    int32(req.Priority),
		Title:       req.Title,
		Description: req.Description,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create ticket")
		return
	}

	writeJSON(w, http.StatusCreated, ticketJSON(ticket))
}

// Get returns a ticket by ID.
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	projectID, err := uuid.Parse(chi.URLParam(r, "project_id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid project id")
		return
	}
	ticketID, err := uuid.Parse(chi.URLParam(r, "ticket_id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid ticket id")
		return
	}

	ticket, err := h.queries.GetTicket(r.Context(), ticketID)
	if err != nil {
		writeError(w, http.StatusNotFound, "ticket not found")
		return
	}
	if ticket.ProjectID != projectID {
		writeError(w, http.StatusNotFound, "ticket not found")
		return
	}

	writeJSON(w, http.StatusOK, ticketJSON(ticket))
}

// GetByIssue returns the active ticket for an issue.
func (h *Handler) GetByIssue(w http.ResponseWriter, r *http.Request) {
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

	ticket, err := h.queries.GetTicketByIssueIncludingDone(r.Context(), uuid.NullUUID{UUID: issueID, Valid: true})
	if err != nil {
		if err == sql.ErrNoRows {
			writeJSON(w, http.StatusOK, map[string]any{"ticket": nil})
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to get ticket")
		return
	}

	// Verify ticket belongs to this project
	if ticket.ProjectID != projectID {
		writeJSON(w, http.StatusOK, map[string]any{"ticket": nil})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"ticket": ticketJSON(ticket)})
}

// Update updates a ticket (status, assignee, priority, etc.)
func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	projectID, err := uuid.Parse(chi.URLParam(r, "project_id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid project id")
		return
	}
	ticketID, err := uuid.Parse(chi.URLParam(r, "ticket_id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid ticket id")
		return
	}

	current, err := h.queries.GetTicket(r.Context(), ticketID)
	if err != nil {
		writeError(w, http.StatusNotFound, "ticket not found")
		return
	}
	if current.ProjectID != projectID {
		writeError(w, http.StatusNotFound, "ticket not found")
		return
	}

	var req UpdateTicketRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	user := auth.GetUserFromContext(r.Context())
	var userID *uuid.UUID
	if user != nil {
		userID = &user.ID
	}

	// Status transition validation
	status := current.Status
	if req.Status != nil && *req.Status != current.Status {
		if !workflow.IsValidTransition(current.Status, *req.Status) && !req.Force {
			valid := workflow.ValidNextStatuses(current.Status)
			writeJSON(w, http.StatusConflict, map[string]any{
				"error":             "non_standard_transition",
				"message":           "Transition from '" + current.Status + "' to '" + *req.Status + "' is not standard",
				"valid_transitions": valid,
				"can_force":         true,
			})
			return
		}
		status = *req.Status
	}

	assignedTo := current.AssignedTo
	if req.AssignedTo != nil {
		if *req.AssignedTo == "" {
			assignedTo = uuid.NullUUID{}
		} else {
			uid, err := uuid.Parse(*req.AssignedTo)
			if err != nil {
				writeError(w, http.StatusBadRequest, "invalid assignee id")
				return
			}
			assignedTo = uuid.NullUUID{UUID: uid, Valid: true}
		}
	}

	priority := current.Priority
	if req.Priority != nil {
		priority = int32(*req.Priority)
	}

	dueDate := current.DueDate
	if req.DueDate != nil {
		if *req.DueDate == "" {
			dueDate = sql.NullTime{}
		} else {
			t, err := time.Parse(time.RFC3339, *req.DueDate)
			if err != nil {
				writeError(w, http.StatusBadRequest, "invalid due_date format (use RFC3339)")
				return
			}
			dueDate = sql.NullTime{Time: t, Valid: true}
		}
	}

	resolutionType := current.ResolutionType
	resolutionNotes := current.ResolutionNotes
	fixReference := current.FixReference
	if status == workflow.StatusDone {
		if req.ResolutionType != nil {
			resolutionType = sql.NullString{String: *req.ResolutionType, Valid: *req.ResolutionType != ""}
		}
		if req.ResolutionNotes != nil {
			resolutionNotes = sql.NullString{String: *req.ResolutionNotes, Valid: *req.ResolutionNotes != ""}
		}
		if req.FixReference != nil {
			fixReference = sql.NullString{String: *req.FixReference, Valid: *req.FixReference != ""}
		}
	}

	title := current.Title
	if req.Title != nil {
		title = *req.Title
	}

	description := current.Description
	if req.Description != nil {
		description = *req.Description
	}

	escalatedSystem := current.EscalatedSystem
	escalatedKey := current.EscalatedKey
	escalatedUrl := current.EscalatedUrl
	if req.EscalatedSystem != nil {
		escalatedSystem = sql.NullString{String: *req.EscalatedSystem, Valid: *req.EscalatedSystem != ""}
	}
	if req.EscalatedKey != nil {
		escalatedKey = sql.NullString{String: *req.EscalatedKey, Valid: *req.EscalatedKey != ""}
	}
	if req.EscalatedURL != nil {
		escalatedUrl = sql.NullString{String: *req.EscalatedURL, Valid: *req.EscalatedURL != ""}
	}

	ticket, err := h.queries.UpdateTicket(r.Context(), db.UpdateTicketParams{
		ID:              ticketID,
		Status:          status,
		AssignedTo:      assignedTo,
		Priority:        priority,
		DueDate:         dueDate,
		ResolutionType:  resolutionType,
		ResolutionNotes: resolutionNotes,
		FixReference:    fixReference,
		Title:           title,
		Description:     description,
		EscalatedSystem: escalatedSystem,
		EscalatedKey:    escalatedKey,
		EscalatedUrl:    escalatedUrl,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update ticket")
		return
	}

	// Record activities and notify followers
	if status != current.Status {
		if ticket.IssueID.Valid {
			activity.Record(r.Context(), h.queries, ticket.IssueID.UUID, &ticket.ID, userID, "ticket_status_changed", current.Status, status, nil)

			if h.notifyFn != nil {
				issue, _ := h.queries.GetIssue(r.Context(), ticket.IssueID.UUID)
				go h.notifyFn(ticket.IssueID.UUID, ticket.ProjectID, issue.Title, "Status changed to "+status, userID)
			}

			// When ticket is done or wontfix, resolve the linked issue
			if status == workflow.StatusDone || status == workflow.StatusWontfix {
				issueStatus := "resolved"
				if status == workflow.StatusWontfix {
					issueStatus = "ignored"
				}

				updateParams := db.UpdateIssueStatusParams{
					ID:     ticket.IssueID.UUID,
					Status: issueStatus,
				}
				if status == workflow.StatusDone {
					now := time.Now()
					updateParams.ResolvedAt = sql.NullTime{Time: now, Valid: true}
					// Apply project default cooldown
					proj, err := h.queries.GetProject(r.Context(), ticket.ProjectID)
					if err == nil && proj.DefaultCooldownMinutes > 0 {
						cooldownEnd := now.Add(time.Duration(proj.DefaultCooldownMinutes) * time.Minute)
						updateParams.CooldownUntil = sql.NullTime{Time: cooldownEnd, Valid: true}
					}
				}
				h.queries.UpdateIssueStatus(r.Context(), updateParams)
				activity.Record(r.Context(), h.queries, ticket.IssueID.UUID, &ticket.ID, nil, "status_changed", "", issueStatus, map[string]string{"reason": "ticket_" + status})

				// Send project-level alert notifications (email/slack)
				if h.alertFn != nil {
					issue, err := h.queries.GetIssue(r.Context(), ticket.IssueID.UUID)
					if err == nil {
						go h.alertFn(ticket.ProjectID, issue, false)
					}
				}
			}

			// When ticket is reopened (done/wontfix -> acknowledged), reopen the linked issue
			if (current.Status == workflow.StatusDone || current.Status == workflow.StatusWontfix) && status == workflow.StatusAcknowledged {
				issue, err := h.queries.GetIssue(r.Context(), ticket.IssueID.UUID)
				if err == nil && (issue.Status == "resolved" || issue.Status == "ignored") {
					h.queries.UpdateIssueStatus(r.Context(), db.UpdateIssueStatusParams{
						ID:     ticket.IssueID.UUID,
						Status: "reopened",
					})
					activity.Record(r.Context(), h.queries, ticket.IssueID.UUID, &ticket.ID, userID, "status_changed", "resolved", "reopened", map[string]string{"reason": "ticket_reopened"})
				}
			}
		}
	}
	if assignedTo != current.AssignedTo && ticket.IssueID.Valid {
		old := ""
		if current.AssignedTo.Valid {
			old = current.AssignedTo.UUID.String()
		}
		new := ""
		if assignedTo.Valid {
			new = assignedTo.UUID.String()
		}
		activity.Record(r.Context(), h.queries, ticket.IssueID.UUID, &ticket.ID, userID, "ticket_assigned", old, new, nil)
	}
	if priority != current.Priority && ticket.IssueID.Valid {
		activity.Record(r.Context(), h.queries, ticket.IssueID.UUID, &ticket.ID, userID, "ticket_priority_changed", fmt.Sprintf("%d", current.Priority), fmt.Sprintf("%d", priority), nil)
	}

	writeJSON(w, http.StatusOK, ticketJSON(ticket))
}

// Transitions returns valid next statuses for a ticket.
func (h *Handler) Transitions(w http.ResponseWriter, r *http.Request) {
	projectID, err := uuid.Parse(chi.URLParam(r, "project_id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid project id")
		return
	}
	ticketID, err := uuid.Parse(chi.URLParam(r, "ticket_id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid ticket id")
		return
	}
	ticket, err := h.queries.GetTicket(r.Context(), ticketID)
	if err != nil || ticket.ProjectID != projectID {
		writeError(w, http.StatusNotFound, "ticket not found")
		return
	}
	next := workflow.ValidNextStatuses(ticket.Status)
	if next == nil {
		next = []string{}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"current":     ticket.Status,
		"transitions": next,
	})
}

// List returns tickets for a project (for board view).
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	projectID, err := uuid.Parse(chi.URLParam(r, "project_id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid project id")
		return
	}

	q := r.URL.Query()
	status := q.Get("status")
	limit, _ := strconv.ParseInt(q.Get("limit"), 10, 32)
	offset, _ := strconv.ParseInt(q.Get("offset"), 10, 32)
	if limit <= 0 || limit > 200 {
		limit = 50
	}

	tickets, err := h.queries.ListTicketsByProject(r.Context(), db.ListTicketsByProjectParams{
		ProjectID: projectID,
		Column2:   status,
		Limit:     int32(limit),
		Offset:    int32(offset),
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list tickets")
		return
	}

	count, _ := h.queries.CountTicketsByProject(r.Context(), db.CountTicketsByProjectParams{
		ProjectID: projectID,
		Column2:   status,
	})

	items := make([]map[string]any, len(tickets))
	for i, t := range tickets {
		items[i] = ticketListJSON(t)
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"tickets": items,
		"total":   count,
	})
}

// Counts returns ticket counts by status for a project.
func (h *Handler) Counts(w http.ResponseWriter, r *http.Request) {
	projectID, err := uuid.Parse(chi.URLParam(r, "project_id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid project id")
		return
	}

	rows, err := h.queries.CountTicketsByStatus(r.Context(), projectID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to count tickets")
		return
	}

	counts := map[string]int32{}
	for _, r := range rows {
		counts[r.Status] = r.Count
	}

	user := auth.GetUserFromContext(r.Context())
	if user != nil {
		assignedPending, err := h.queries.CountPendingTicketsAssignedToUser(r.Context(), db.CountPendingTicketsAssignedToUserParams{
			ProjectID:  projectID,
			AssignedTo: uuid.NullUUID{UUID: user.ID, Valid: true},
		})
		if err == nil {
			counts["assigned_to_me_pending"] = assignedPending
		}
	}

	writeJSON(w, http.StatusOK, counts)
}

type UpdateTicketRequest struct {
	Status          *string `json:"status,omitempty"`
	AssignedTo      *string `json:"assigned_to,omitempty"`
	Priority        *int    `json:"priority,omitempty"`
	DueDate         *string `json:"due_date,omitempty"`
	ResolutionType  *string `json:"resolution_type,omitempty"`
	ResolutionNotes *string `json:"resolution_notes,omitempty"`
	FixReference    *string `json:"fix_reference,omitempty"`
	Title           *string `json:"title,omitempty"`
	Description     *string `json:"description,omitempty"`
	EscalatedSystem *string `json:"escalated_system,omitempty"`
	EscalatedKey    *string `json:"escalated_key,omitempty"`
	EscalatedURL    *string `json:"escalated_url,omitempty"`
	Force           bool    `json:"force,omitempty"`
}

func ticketJSON(t db.Ticket) map[string]any {
	m := map[string]any{
		"id":               t.ID,
		"issue_id":         nullUUID(t.IssueID),
		"project_id":       t.ProjectID,
		"status":           t.Status,
		"assigned_to":      nullUUID(t.AssignedTo),
		"created_by":       t.CreatedBy,
		"priority":         t.Priority,
		"due_date":         nullTime(t.DueDate),
		"resolution_type":  nullString(t.ResolutionType),
		"resolution_notes": nullString(t.ResolutionNotes),
		"fix_reference":    nullString(t.FixReference),
		"title":            t.Title,
		"description":      t.Description,
		"escalated_system": nullString(t.EscalatedSystem),
		"escalated_key":    nullString(t.EscalatedKey),
		"escalated_url":    nullString(t.EscalatedUrl),
		"created_at":       t.CreatedAt,
		"updated_at":       t.UpdatedAt,
	}
	return m
}

func ticketListJSON(t db.ListTicketsByProjectRow) map[string]any {
	m := map[string]any{
		"id":                t.ID,
		"issue_id":          nullUUID(t.IssueID),
		"project_id":        t.ProjectID,
		"status":            t.Status,
		"assigned_to":       nullUUID(t.AssignedTo),
		"created_by":        t.CreatedBy,
		"priority":          t.Priority,
		"due_date":          nullTime(t.DueDate),
		"escalated_key":     nullString(t.EscalatedKey),
		"escalated_url":     nullString(t.EscalatedUrl),
		"created_at":        t.CreatedAt,
		"updated_at":        t.UpdatedAt,
		"issue_title":       t.IssueTitle,
		"issue_level":       t.IssueLevel,
		"issue_event_count": t.IssueEventCount,
		"issue_first_seen":  nullTime(t.IssueFirstSeen),
		"issue_last_seen":   nullTime(t.IssueLastSeen),
		"assignee_name":     nullString(t.AssigneeName),
		"assignee_email":    nullString(t.AssigneeEmail),
		"assignee_avatar":   nullString(t.AssigneeAvatar),
	}
	return m
}

func nullUUID(u uuid.NullUUID) any {
	if u.Valid {
		return u.UUID
	}
	return nil
}

func nullString(s sql.NullString) any {
	if s.Valid {
		return s.String
	}
	return nil
}

func nullTime(t sql.NullTime) any {
	if t.Valid {
		return t.Time
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
