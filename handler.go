package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"
)

func (s *server) getUsersHandler(w http.ResponseWriter, r *http.Request) {
	log.Printf("getUsers start: method=%s remote=%s", r.Method, r.RemoteAddr)
	if r.Method != http.MethodGet {
		log.Printf("getUsers invalid method: %s", r.Method)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusMethodNotAllowed)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "method_not_allowed"})
		return
	}

	w.Header().Set("Content-Type", "application/json")

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	log.Println("getUsers querying database")
	users, err := s.fetchUsers(ctx)
	if err != nil {
		log.Printf("getUsers query failed: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "internal_error"})
		return
	}

	log.Printf("getUsers returning %d users", len(users))
	if err := json.NewEncoder(w).Encode(users); err != nil {
		log.Printf("getUsers encode failed: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "internal_error"})
	}
}

func pingHandler(w http.ResponseWriter, r *http.Request) {
	log.Printf("ping request: method=%s remote=%s", r.Method, r.RemoteAddr)
	w.Header().Set("Content-Type", "application/json")

	resp := map[string]string{"message": "pong v2"}
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		// Best-effort error response if encoding fails
		http.Error(w, `{"error":"internal_error"}`, http.StatusInternalServerError)
	}
}

func notFoundHandler(w http.ResponseWriter, r *http.Request) {
	log.Printf("not found: path=%s method=%s remote=%s", r.URL.Path, r.Method, r.RemoteAddr)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotFound)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": "not_found"})
}
