package main

import (
	"encoding/json"
	"log"
	"net/http"
)

func main() {
	http.HandleFunc("/ping", pingHandler)

	log.Println("HTTP server listening on :8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}

func pingHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	resp := map[string]string{"message": "pong"}
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		// Best-effort error response if encoding fails
		http.Error(w, `{"error":"internal_error"}`, http.StatusInternalServerError)
	}
}
