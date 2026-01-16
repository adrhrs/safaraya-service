package main

import (
	"context"
	"log"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
)

const dbURL = "postgresql://app_user:oXQImTmTbltQWP2v83UoFSDWLHfChivG@dpg-d5kt8n4oud1c73e0aoqg-a/safaraya_db"

// const dbURL = "postgresql://app_user:oXQImTmTbltQWP2v83UoFSDWLHfChivG@dpg-d5kt8n4oud1c73e0aoqg-a.oregon-postgres.render.com/safaraya_db"

func main() {
	ctx := context.Background()

	log.Println("connecting to database..")
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		log.Fatalf("failed to init db: %v", err)
	}
	log.Println("database connection pool established")
	defer pool.Close()

	srv := &server{db: pool}

	log.Println("registering handlers")
	mux := http.NewServeMux()
	mux.HandleFunc("/ping", pingHandler)
	mux.HandleFunc("/users", srv.getUsersHandler)
	mux.HandleFunc("/", notFoundHandler)

	log.Println("HTTP server listening on :8080")
	if err := http.ListenAndServe(":8080", mux); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}
