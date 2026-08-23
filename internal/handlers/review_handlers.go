package handlers

import (
	"log"
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"medagent/internal/auth"
	"medagent/internal/middleware"
)

type ReviewHandler struct {
	DB *pgxpool.Pool
}

type submitReviewRequest struct {
	Decision             string `json:"decision"` // "approved" | "rejected" | "edited_approved"
	EditedJustification  string `json:"edited_justification,omitempty"`
}

// SubmitReview lets a reviewer/admin approve, reject, or edit-and-approve
// a case that's currently flagged needs_review. Only reachable by users
// with role reviewer/admin, enforced by RequireRole middleware upstream.
func (h *ReviewHandler) SubmitReview(w http.ResponseWriter, r *http.Request) {
	claims, ok := r.Context().Value(middleware.ClaimsKey).(*auth.Claims)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	caseIDStr := chi.URLParam(r, "id")
	caseID, err := uuid.Parse(caseIDStr)
	if err != nil {
		http.Error(w, "invalid case id", http.StatusBadRequest)
		return
	}

	var req submitReviewRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.Decision != "approved" && req.Decision != "rejected" && req.Decision != "edited_approved" {
		http.Error(w, "decision must be approved, rejected, or edited_approved", http.StatusBadRequest)
		return
	}

	ctx := r.Context()

	// Confirm the case belongs to this reviewer's org and is actually
	// awaiting review — prevents reviewing someone else's org's case,
	// and prevents double-reviewing an already-decided case.
	var status string
	err = h.DB.QueryRow(ctx,
		`SELECT status FROM cases WHERE id = $1 AND org_id = $2`,
		caseID, claims.OrgID,
	).Scan(&status)
	if err != nil {
		http.Error(w, "case not found", http.StatusNotFound)
		return
	}
	if status != "needs_review" {
		http.Error(w, "case is not awaiting review", http.StatusConflict)
		return
	}

	// Pull the most recent drafting step's output as the "original draft"
	// snapshot — this is what the reviewer saw and is now deciding on.
	var originalDraft []byte
	err = h.DB.QueryRow(ctx,
		`SELECT ast.output FROM agent_steps ast
		 JOIN agent_runs ar ON ast.agent_run_id = ar.id
		 WHERE ar.case_id = $1 AND ast.step_name = 'drafting'
		 ORDER BY ast.created_at DESC LIMIT 1`,
		caseID,
	).Scan(&originalDraft)
	if err != nil {
		http.Error(w, "could not find original draft for this case", http.StatusInternalServerError)
		return
	}

	_, err = h.DB.Exec(ctx,
		`INSERT INTO reviews (case_id, reviewer_id, original_draft, final_decision, edited_justification)
		 VALUES ($1, $2, $3, $4, $5)`,
		caseID, claims.UserID, originalDraft, req.Decision, req.EditedJustification,
	)
	if err != nil {
		http.Error(w, "failed to save review", http.StatusInternalServerError)
		return
	}

	finalCaseStatus := "rejected"
	if req.Decision == "approved" || req.Decision == "edited_approved" {
		finalCaseStatus = "approved"
	}

	_, err = h.DB.Exec(ctx,
		`UPDATE cases SET status = $1, updated_at = now() WHERE id = $2`,
		finalCaseStatus, caseID,
	)
	if err != nil {
		log.Printf("update case status failed: %v", err)
		http.Error(w, "failed to update case status", http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(map[string]string{
		"case_id": caseID.String(),
		"status":  finalCaseStatus,
	})
}//0728ef51-0413-4cb1-a851-365e60904e37