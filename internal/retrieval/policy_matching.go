package retrieval

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pgvector/pgvector-go"
)

type MatchedClause struct {
	ID         string  `json:"id"`
	ClauseText string  `json:"clause_text"`
	Similarity float64 `json:"similarity"`
}

// MatchPolicy embeds the query text and retrieves the top-k most similar
// clauses for a given policy, using pgvector's cosine distance operator.
func MatchPolicy(ctx context.Context, db *pgxpool.Pool, policyID, queryText string, topK int) ([]MatchedClause, error) {
	queryEmbedding, err := EmbedText(queryText)
	if err != nil {
		return nil, fmt.Errorf("failed to embed query: %w", err)
	}

	rows, err := db.Query(ctx,
		`SELECT id, clause_text, 1 - (embedding <=> $1) AS similarity
		 FROM policy_clauses
		 WHERE policy_id = $2
		 ORDER BY embedding <=> $1
		 LIMIT $3`,
		pgvector.NewVector(queryEmbedding), policyID, topK,
	)
	if err != nil {
		return nil, fmt.Errorf("policy clause query failed: %w", err)
	}
	defer rows.Close()

	var matches []MatchedClause
	for rows.Next() {
		var m MatchedClause
		if err := rows.Scan(&m.ID, &m.ClauseText, &m.Similarity); err != nil {
			return nil, err
		}
		matches = append(matches, m)
	}

	return matches, nil
}