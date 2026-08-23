package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"medagent/internal/retrieval"
)

type PipelineResult struct {
	AgentRunID  string
	Decision    *DraftDecision
	Confidence  ConfidenceResult
	FinalStatus string
}

// RunPipeline executes extraction -> policy_matching -> drafting -> confidence_scoring
// for one case. Extraction and drafting/confidence run in the Python agent
// service (via HTTP); policy matching runs in Go (direct pgvector access).
// Every step writes an agent_steps row so the full reasoning trail is
// reconstructable later.
func RunPipeline(ctx context.Context, db *pgxpool.Pool, caseID, clinicalNote, policyID string) (*PipelineResult, error) {
	var runID string
	err := db.QueryRow(ctx,
		`INSERT INTO agent_runs (case_id) VALUES ($1) RETURNING id`, caseID,
	).Scan(&runID)
	if err != nil {
		return nil, fmt.Errorf("failed to create agent_run: %w", err)
	}

	// --- Step 1: Extraction (Python) ---
	t0 := time.Now()
	var facts ExtractedFacts
	err = callAgentService("/extract", map[string]string{"clinical_note": clinicalNote}, &facts)
	if err != nil {
		return nil, fmt.Errorf("extraction failed: %w", err)
	}
	if err := logStep(ctx, db, runID, "extraction", 1,
		map[string]string{"clinical_note": clinicalNote}, facts, time.Since(t0)); err != nil {
		return nil, err
	}

	// --- Step 2: Policy matching (Go, pgvector) ---
	t0 = time.Now()
	queryText := fmt.Sprintf("Treatment: %s. Diagnosis: %s. Symptoms: %v. Duration: %s.",
		facts.TreatmentRequested, facts.Diagnosis, facts.Symptoms, facts.DurationOfSymptoms)
	matched, err := retrieval.MatchPolicy(ctx, db, policyID, queryText, 5)
	if err != nil {
		return nil, fmt.Errorf("policy matching failed: %w", err)
	}
	if err := logStep(ctx, db, runID, "policy_matching", 2, facts, matched, time.Since(t0)); err != nil {
		return nil, err
	}

	// --- Step 3: Drafting (Python) ---
	t0 = time.Now()
	clauseInputs := make([]ClauseInput, len(matched))
	for i, m := range matched {
		clauseInputs[i] = ClauseInput{ID: m.ID, ClauseText: m.ClauseText}
	}
	var decision DraftDecision
	err = callAgentService("/draft", map[string]interface{}{
		"facts":   facts,
		"clauses": clauseInputs,
	}, &decision)
	if err != nil {
		return nil, fmt.Errorf("drafting failed: %w", err)
	}
	if err := logStep(ctx, db, runID, "drafting", 3, clauseInputs, decision, time.Since(t0)); err != nil {
		return nil, err
	}

	// --- Step 4: Confidence scoring (Python, rule-based) ---
	t0 = time.Now()
	clausesWithSim := make([]ClauseWithSimilarity, len(matched))
	for i, m := range matched {
		clausesWithSim[i] = ClauseWithSimilarity{ID: m.ID, ClauseText: m.ClauseText, Similarity: m.Similarity}
	}
	var confidence ConfidenceResult
	err = callAgentService("/score-confidence", map[string]interface{}{
		"clauses":  clausesWithSim,
		"decision": decision,
	}, &confidence)
	if err != nil {
		return nil, fmt.Errorf("confidence scoring failed: %w", err)
	}
	if err := logStep(ctx, db, runID, "confidence_scoring", 4, decision, confidence, time.Since(t0)); err != nil {
		return nil, err
	}

	finalStatus := "auto_approved"
	if confidence.NeedsHumanReview {
		finalStatus = "needs_review"
	}

	_, err = db.Exec(ctx,
		`UPDATE agent_runs SET finished_at = now(), final_confidence = $1, final_status = $2 WHERE id = $3`,
		confidence.Score, finalStatus, runID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to finalize agent_run: %w", err)
	}

	_, err = db.Exec(ctx, `UPDATE cases SET status = $1, updated_at = now() WHERE id = $2`, finalStatus, caseID)
	if err != nil {
		return nil, fmt.Errorf("failed to update case status: %w", err)
	}

	return &PipelineResult{
		AgentRunID:  runID,
		Decision:    &decision,
		Confidence:  confidence,
		FinalStatus: finalStatus,
	}, nil
}

func logStep(ctx context.Context, db *pgxpool.Pool, runID, stepName string, order int, input, output interface{}, latency time.Duration) error {
	inputJSON, err := json.Marshal(input)
	if err != nil {
		return err
	}
	outputJSON, err := json.Marshal(output)
	if err != nil {
		return err
	}

	_, err = db.Exec(ctx,
		`INSERT INTO agent_steps (agent_run_id, step_name, step_order, input_summary, output, model_used, latency_ms)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		runID, stepName, order, inputJSON, outputJSON, "gemini-3.6-flash", latency.Milliseconds(),
	)
	return err
}