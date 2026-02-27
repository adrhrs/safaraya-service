package main

import (
	"encoding/json"
	"log"
	"net/http"
)

func pingHandler(w http.ResponseWriter, r *http.Request) {
	log.Printf("ping request: method=%s remote=%s", r.Method, r.RemoteAddr)
	w.Header().Set("Content-Type", "application/json")

	resp := map[string]string{"message": "pong v2"}
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		http.Error(w, `{"error":"internal_error"}`, http.StatusInternalServerError)
	}
}

func notFoundHandler(w http.ResponseWriter, r *http.Request) {
	log.Printf("not found: path=%s method=%s remote=%s", r.URL.Path, r.Method, r.RemoteAddr)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotFound)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": "not_found"})
}
