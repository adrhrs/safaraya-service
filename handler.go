package main

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
)

func getFinalURL(link string) (string, error) {
	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return errors.New("too many redirects")
			}
			return nil
		},
	}

	resp, err := client.Get(link)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	return resp.Request.URL.String(), nil
}

func resolveShopeeURLHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusMethodNotAllowed)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "method_not_allowed"})
		return
	}

	shortURL := r.URL.Query().Get("url")
	if shortURL == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "url_required"})
		return
	}

	fullURL, err := getFinalURL(shortURL)
	if err != nil {
		log.Printf("resolveShopeeURL getFinalURL failed: %v", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "resolve_failed"})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"full_url": fullURL})
}

func pingHandler(w http.ResponseWriter, r *http.Request) {
	log.Printf("ping request: method=%s remote=%s", r.Method, r.RemoteAddr)
	w.Header().Set("Content-Type", "application/json")

	resp := map[string]string{"message": "pong v3"}
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
