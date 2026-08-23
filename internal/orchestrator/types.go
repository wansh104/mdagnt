package orchestrator

// These structs mirror the Pydantic models in agent_service/main.py exactly.
// Field names use json tags to match Python's snake_case output.

type ExtractedFacts struct {
	TreatmentRequested string   `json:"treatment_requested"`
	Diagnosis          string   `json:"diagnosis"`
	PriorTreatments    []string `json:"prior_treatments"`
	Symptoms           []string `json:"symptoms"`
	DurationOfSymptoms string   `json:"duration_of_symptoms"`
}

type ClauseInput struct {
	ID         string `json:"id"`
	ClauseText string `json:"clause_text"`
}

type ClaimCitation struct {
	Claim     string `json:"claim"`
	ClauseID  string `json:"clause_id"`
	Supported bool   `json:"supported"`
}

type DraftDecision struct {
	Recommendation string          `json:"recommendation"`
	Justification  string          `json:"justification"`
	CitedClauseIDs []string        `json:"cited_clause_ids"`
	ClaimCitations []ClaimCitation `json:"claim_citations"`
}

type ClauseWithSimilarity struct {
	ID         string  `json:"id"`
	ClauseText string  `json:"clause_text"`
	Similarity float64 `json:"similarity"`
}

type ConfidenceResult struct {
	Score           float64 `json:"score"`
	NeedsHumanReview bool   `json:"needs_human_review"`
	Reason          string  `json:"reason"`
}