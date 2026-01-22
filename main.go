package main

import (
	"context"
	"log"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
)

// prod param
const dbURL = "postgresql://app_user:oXQImTmTbltQWP2v83UoFSDWLHfChivG@dpg-d5kt8n4oud1c73e0aoqg-a/safaraya_db"
const serviceHost = "https://safaraya-service.onrender.com"

// local param
// const dbURL = "postgresql://app_user:oXQImTmTbltQWP2v83UoFSDWLHfChivG@dpg-d5kt8n4oud1c73e0aoqg-a.oregon-postgres.render.com/safaraya_db"
// const serviceHost = "http://localhost:8080"
const cvDownloadPathTemplate = "/users/%d/cv"

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
	mux.HandleFunc("/users", srv.usersHandler)
	mux.HandleFunc("/users/", srv.userCVHandler)
	mux.HandleFunc("/registrations", srv.registrationsHandler)
	mux.HandleFunc("/registrations/", srv.registrationDetailHandler)
	mux.HandleFunc("/registration-files", srv.registrationFilesHandler)
	mux.HandleFunc("/registration-files/", srv.registrationFileHandler)
	mux.HandleFunc("/", notFoundHandler)

	log.Println("HTTP server listening on :8080")
	if err := http.ListenAndServe(":8080", mux); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}
