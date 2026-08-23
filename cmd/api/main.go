package main

import (
	"context"
	"log"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"medagent/internal/handlers"
	"medagent/internal/middleware"
)

func main() {
	dbURL := os.Getenv("DATABASE_URL")
	pool, err := pgxpool.New(context.Background(), dbURL)
	if err != nil {
		log.Fatalf("unable to connect to database: %v", err)
	}
	defer pool.Close()

	authHandler := &handlers.AuthHandler{DB: pool}
	caseHandler := &handlers.CaseHandler{DB: pool}
	reviewHandler := &handlers.ReviewHandler{DB: pool}

	r := chi.NewRouter()

	r.Post("/api/register", authHandler.Register)
	r.Post("/api/login", authHandler.Login)

	r.Group(func(r chi.Router) {
		r.Use(middleware.RequireAuth)
		r.Get("/api/cases", caseHandler.ListCases)
		r.Post("/api/cases", caseHandler.CreateCase)

		r.Group(func(r chi.Router) {
			r.Use(middleware.RequireRole("reviewer", "admin"))
			r.Post("/api/cases/{id}/review", reviewHandler.SubmitReview)
		})
	})

	log.Println("listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", r))
}