import os
from pydantic import BaseModel
from fastapi import FastAPI
from langchain_google_genai import ChatGoogleGenerativeAI

app = FastAPI()

llm = ChatGoogleGenerativeAI(model="gemini-3.6-flash")


# ---------- Extraction ----------

class ExtractionRequest(BaseModel):
    clinical_note: str


class ExtractedFacts(BaseModel):
    treatment_requested: str
    diagnosis: str
    prior_treatments: list[str]
    symptoms: list[str]
    duration_of_symptoms: str


EXTRACTION_PROMPT = """You are a clinical data extraction assistant. Given a clinical note,
extract ONLY the facts explicitly stated in the note. Do NOT infer or assume anything not written.
If a field is not mentioned, use an empty string or empty list."""

extraction_llm = llm.with_structured_output(ExtractedFacts)


@app.post("/extract", response_model=ExtractedFacts)
def extract(req: ExtractionRequest):
    result = extraction_llm.invoke(EXTRACTION_PROMPT + "\n\nClinical note:\n" + req.clinical_note)
    return result


# ---------- Drafting ----------

class ClauseInput(BaseModel):
    id: str
    clause_text: str


class DraftRequest(BaseModel):
    facts: ExtractedFacts
    clauses: list[ClauseInput]


class ClaimCitation(BaseModel):
    claim: str
    clause_id: str
    supported: bool


class DraftDecision(BaseModel):
    recommendation: str  # "approve" | "deny" | "insufficient_info"
    justification: str
    cited_clause_ids: list[str]
    claim_citations: list[ClaimCitation]


DRAFTING_PROMPT = """You are a prior authorization drafting assistant. Given structured patient
facts and a set of retrieved policy clauses, determine whether the requested treatment meets the
policy's criteria. For EVERY claim you make about why the patient qualifies (or doesn't), you MUST
cite the specific clause_id that supports it. If a claim has no supporting clause, mark
supported: false and leave clause_id empty rather than inventing a citation."""

drafting_llm = llm.with_structured_output(DraftDecision)


@app.post("/draft", response_model=DraftDecision)
def draft(req: DraftRequest):
    clause_block = "\n\n".join(f"{c.id}: {c.clause_text}" for c in req.clauses)
    prompt = (
        DRAFTING_PROMPT
        + "\n\nPatient facts:\n" + req.facts.model_dump_json()
        + "\n\nRetrieved policy clauses:\n" + clause_block
    )
    result = drafting_llm.invoke(prompt)
    return result


# ---------- Confidence scoring (rule-based, no LLM) ----------

class ClauseWithSimilarity(BaseModel):
    id: str
    clause_text: str
    similarity: float


class ConfidenceRequest(BaseModel):
    clauses: list[ClauseWithSimilarity]
    decision: DraftDecision


class ConfidenceResult(BaseModel):
    score: float
    needs_human_review: bool
    reason: str


MIN_CLAUSE_COUNT = 2
MIN_SIMILARITY = 0.70


@app.post("/score-confidence", response_model=ConfidenceResult)
def score_confidence(req: ConfidenceRequest):
    if len(req.clauses) < MIN_CLAUSE_COUNT:
        return ConfidenceResult(
            score=0.3,
            needs_human_review=True,
            reason="fewer than 2 relevant policy clauses were retrieved",
        )

    if any(c.similarity < MIN_SIMILARITY for c in req.clauses):
        return ConfidenceResult(
            score=0.5,
            needs_human_review=True,
            reason="retrieved clause similarity below threshold",
        )

    if any(not claim.supported for claim in req.decision.claim_citations):
        return ConfidenceResult(
            score=0.4,
            needs_human_review=True,
            reason="one or more claims have no supporting clause",
        )

    if req.decision.recommendation == "insufficient_info":
        return ConfidenceResult(
            score=0.5,
            needs_human_review=True,
            reason="drafting agent reported insufficient information",
        )

    return ConfidenceResult(
        score=0.9,
        needs_human_review=False,
        reason="all claims cited, clause similarity above threshold",
    )