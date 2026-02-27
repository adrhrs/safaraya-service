package main

import (
	"log"
	"net/http"
)

func main() {
	log.Println("registering handlers")
	mux := http.NewServeMux()
	mux.HandleFunc("/ping", pingHandler)
	mux.HandleFunc("/", notFoundHandler)

	log.Println("HTTP server listening on :8080")
	if err := http.ListenAndServe(":8080", mux); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}
