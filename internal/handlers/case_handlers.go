package handlers

import (
	"context"
	"encoding/json"
	"log"
	"net/http"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"medagent/internal/auth"
	"medagent/internal/middleware"
	"medagent/internal/orchestrator"
)

type CaseHandler struct {
	DB *pgxpool.Pool
}

type createCaseRequest struct {
	PatientID          string `json:"patient_id"`
	TreatmentRequested string `json:"treatment_requested"`
	ClinicalNote       string `json:"clinical_note"`
	PolicyID           string `json:"policy_id"`
}

func (h *CaseHandler) ListCases(w http.ResponseWriter, r *http.Request) {
	claims, ok := r.Context().Value(middleware.ClaimsKey).(*auth.Claims)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	rows, err := h.DB.Query(context.Background(),
		`SELECT id::text, treatment_requested, status, created_at::text
 		FROM cases WHERE org_id = $1 ORDER BY created_at DESC`,
		claims.OrgID,
	)
	if err != nil {
		http.Error(w, "failed to fetch cases", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	type caseRow struct {
		ID                 string `json:"id"`
		TreatmentRequested string `json:"treatment_requested"`
		Status             string `json:"status"`
		CreatedAt          string `json:"created_at"`
	}
	var cases []caseRow
	for rows.Next() {
		var c caseRow
		if err := rows.Scan(&c.ID, &c.TreatmentRequested, &c.Status, &c.CreatedAt); err != nil {
			http.Error(w, "failed to read case row", http.StatusInternalServerError)
			return
		}
		cases = append(cases, c)
	}

	json.NewEncoder(w).Encode(cases)
}

// CreateCase inserts the case, then synchronously runs the agent pipeline.
func (h *CaseHandler) CreateCase(w http.ResponseWriter, r *http.Request) {
	claims, ok := r.Context().Value(middleware.ClaimsKey).(*auth.Claims)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var req createCaseRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	patientID, err := uuid.Parse(req.PatientID)
	if err != nil {
		http.Error(w, "invalid patient_id", http.StatusBadRequest)
		return
	}

	var caseID uuid.UUID
	err = h.DB.QueryRow(r.Context(),
		`INSERT INTO cases (org_id, patient_id, created_by, treatment_requested, clinical_note, policy_id, status)
		 VALUES ($1, $2, $3, $4, $5, $6, 'processing') RETURNING id`,
		claims.OrgID, patientID, claims.UserID, req.TreatmentRequested, req.ClinicalNote, req.PolicyID,
	).Scan(&caseID)
	if err != nil {
		log.Printf("create case insert failed: %v", err)
		http.Error(w, "failed to create case", http.StatusInternalServerError)
		return
	}

	result, err := orchestrator.RunPipeline(r.Context(), h.DB, caseID.String(), req.ClinicalNote, req.PolicyID)
	if err != nil {
		log.Printf("pipeline failed for case %s: %v", caseID, err)
		http.Error(w, "case created but pipeline failed, check server logs", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"case_id":    caseID.String(),
		"status":     result.FinalStatus,
		"confidence": result.Confidence,
		"decision":   result.Decision,
	})
}