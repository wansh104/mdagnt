package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pgvector/pgvector-go"
	"medagent/internal/retrieval"
)

// Usage: go run ./cmd/ingest_policy <payer_name> <title> <path_to_txt_file>
func main() {
	if len(os.Args) < 4 {
		log.Fatal("usage: go run ./cmd/ingest_policy <payer_name> <title> <path_to_txt_file>")
	}
	payerName, title, path := os.Args[1], os.Args[2], os.Args[3]

	raw, err := os.ReadFile(path)
	if err != nil {
		log.Fatalf("failed to read file: %v", err)
	}

	ctx := context.Background()
	fmt.Printf("DEBUG connecting with: %q\n", os.Getenv("DATABASE_URL"))
	db, err := pgxpool.New(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		log.Fatalf("failed to connect to db: %v", err)
	}
	defer db.Close()

	var policyID string
	err = db.QueryRow(ctx,
		`INSERT INTO policies (payer_name, title, raw_text) VALUES ($1, $2, $3) RETURNING id`,
		payerName, title, string(raw),
	).Scan(&policyID)
	if err != nil {
		log.Fatalf("failed to insert policy: %v", err)
	}
	fmt.Printf("Created policy %s\n", policyID)

	chunks := retrieval.ChunkPolicyText(string(raw), 1000)
	fmt.Printf("Split into %d chunks\n", len(chunks))

	for i, chunk := range chunks {
		embedding, err := retrieval.EmbedText(chunk)
		if err != nil {
			log.Fatalf("failed to embed chunk %d: %v", i, err)
		}

		_, err = db.Exec(ctx,
			`INSERT INTO policy_clauses (policy_id, clause_text, embedding) VALUES ($1, $2, $3)`,
			policyID, chunk, pgvector.NewVector(embedding),
		)
		if err != nil {
			log.Fatalf("failed to insert clause %d: %v", i, err)
		}
		fmt.Printf("Embedded chunk %d/%d\n", i+1, len(chunks))
	}

	fmt.Println("Done. Policy ID:", policyID)
}