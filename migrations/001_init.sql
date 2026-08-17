-- Enable pgvector for policy clause embeddings
CREATE EXTENSION IF NOT EXISTS vector;
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

CREATE TABLE organizations (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);


CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    org_id UUID NOT NULL REFERENCES organizations(id),
    email TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    role TEXT NOT NULL CHECK (role IN ('provider', 'reviewer', 'admin')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE patients (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    org_id UUID NOT NULL REFERENCES organizations(id),
    display_name TEXT NOT NULL, -- synthetic data only, never real PHI
    date_of_birth DATE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Payer policy documents (e.g. a CMS LCD PDF)
CREATE TABLE policies (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    payer_name TEXT NOT NULL,
    title TEXT NOT NULL,
    source_url TEXT,
    raw_text TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);


-- Chunked, embedded clauses of a policy — this is what gets retrieved
CREATE TABLE policy_clauses (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    policy_id UUID NOT NULL REFERENCES policies(id) ON DELETE CASCADE,
    clause_text TEXT NOT NULL,
    embedding vector(768), -- matches text-embedding-3-small dims
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX ON policy_clauses USING ivfflat (embedding vector_cosine_ops) WITH (lists = 100);

-- A prior-auth request being processed
CREATE TABLE cases (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    org_id UUID NOT NULL REFERENCES organizations(id),
    patient_id UUID NOT NULL REFERENCES patients(id),
    created_by UUID NOT NULL REFERENCES users(id),
    treatment_requested TEXT NOT NULL,
    clinical_note TEXT NOT NULL, -- synthetic
    policy_id UUID REFERENCES policies(id),
    status TEXT NOT NULL DEFAULT 'pending' CHECK (
        status IN ('pending', 'processing', 'auto_approved', 'needs_review', 'approved', 'rejected')
    ),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- One row per agent pipeline execution for a case
CREATE TABLE agent_runs (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    case_id UUID NOT NULL REFERENCES cases(id) ON DELETE CASCADE,
    started_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    finished_at TIMESTAMPTZ,
    final_confidence NUMERIC(4,3),
    final_status TEXT
);

-- One row per agent step within a run — this is the audit trail
CREATE TABLE agent_steps (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    agent_run_id UUID NOT NULL REFERENCES agent_runs(id) ON DELETE CASCADE,
    step_name TEXT NOT NULL CHECK (
        step_name IN ('extraction', 'policy_matching', 'drafting', 'confidence_scoring')
    ),
    step_order INT NOT NULL,
    input_summary JSONB NOT NULL,
    output JSONB NOT NULL,
    model_used TEXT NOT NULL,
    latency_ms INT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Human review / final decision on a case
CREATE TABLE reviews (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    case_id UUID NOT NULL REFERENCES cases(id) ON DELETE CASCADE,
    reviewer_id UUID REFERENCES users(id),
    original_draft JSONB NOT NULL,
    final_decision TEXT NOT NULL CHECK (final_decision IN ('approved', 'rejected', 'edited_approved')),
    edited_justification TEXT,
    reviewed_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_cases_org_status ON cases(org_id, status);
CREATE INDEX idx_agent_steps_run ON agent_steps(agent_run_id, step_order);