package issue

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/darkspock/gosnag/internal/database/db"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type MergeRequest struct {
	PrimaryID string   `json:"primary_id"`
	IssueIDs  []string `json:"issue_ids"` // secondary issues to merge into primary
}

func (h *Handler) Merge(w http.ResponseWriter, r *http.Request) {
	projectID, err := uuid.Parse(chi.URLParam(r, "project_id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid project id")
		return
	}

	var req MergeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	primaryID, err := uuid.Parse(req.PrimaryID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid primary_id")
		return
	}

	if len(req.IssueIDs) == 0 {
		writeError(w, http.StatusBadRequest, "issue_ids is required")
		return
	}

	secondaryIDs := make([]uuid.UUID, len(req.IssueIDs))
	for i, s := range req.IssueIDs {
		id, err := uuid.Parse(s)
		if err != nil {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid issue id: %s", s))
			return
		}
		if id == primaryID {
			writeError(w, http.StatusBadRequest, "primary_id must not be in issue_ids")
			return
		}
		secondaryIDs[i] = id
	}

	if h.database == nil {
		writeError(w, http.StatusInternalServerError, "merge not available")
		return
	}

	// Verify primary issue exists and belongs to project
	primary, err := h.queries.GetIssue(r.Context(), primaryID)
	if err != nil {
		writeError(w, http.StatusNotFound, "primary issue not found")
		return
	}
	if primary.ProjectID != projectID {
		writeError(w, http.StatusBadRequest, "primary issue does not belong to this project")
		return
	}

	// Run merge in a transaction
	tx, err := h.database.BeginTx(r.Context(), nil)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to start transaction")
		return
	}
	defer tx.Rollback()

	txq := db.New(tx)

	for _, secID := range secondaryIDs {
		secondary, err := txq.GetIssue(r.Context(), secID)
		if err != nil {
			writeError(w, http.StatusNotFound, fmt.Sprintf("issue %s not found", secID))
			return
		}
		if secondary.ProjectID != projectID {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("issue %s does not belong to this project", secID))
			return
		}

		// Move all events from secondary to primary
		_, err = txq.MoveEventsToIssue(r.Context(), db.MoveEventsToIssueParams{
			IssueID:   secID,
			IssueID_2: primaryID,
		})
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to move events")
			return
		}

		// Repoint any existing aliases that pointed to this secondary
		err = txq.RepointAliases(r.Context(), db.RepointAliasesParams{
			PrimaryIssueID:   secID,
			PrimaryIssueID_2: primaryID,
		})
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to repoint aliases")
			return
		}

		// Register fingerprint alias so future events go to primary
		err = txq.CreateIssueAlias(r.Context(), db.CreateIssueAliasParams{
			ProjectID:      projectID,
			Fingerprint:    secondary.Fingerprint,
			PrimaryIssueID: primaryID,
		})
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to create alias")
			return
		}

		// Delete the secondary issue
		err = txq.DeleteIssue(r.Context(), secID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to delete merged issue")
			return
		}
	}

	// Recalculate primary issue stats
	updated, err := txq.RecalcIssueStats(r.Context(), primaryID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to recalculate stats")
		return
	}

	if err := tx.Commit(); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to commit merge")
		return
	}

	writeJSON(w, http.StatusOK, issueJSON(updated))
}
