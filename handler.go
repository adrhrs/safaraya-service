package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
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

func (s *server) getShopeeItemDetailsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusMethodNotAllowed)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "method_not_allowed"})
		return
	}

	prefix := "/get-shopee-item-details/"
	path := strings.TrimPrefix(r.URL.Path, prefix)
	if path == r.URL.Path || path == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "item_id_required"})
		return
	}
	// support only the item_id segment (ignore any trailing path)
	if idx := strings.Index(path, "/"); idx != -1 {
		path = path[:idx]
	}
	itemID, err := strconv.ParseInt(path, 10, 64)
	if err != nil || itemID <= 0 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "invalid_item_id"})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	raw, err := s.productOffer.GetProductOfferByItemID(ctx, itemID, 0, 10, 0)
	if err != nil {
		log.Printf("getShopeeItemDetails GetProductOfferByItemID failed: %v", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "shopee_api_failed"})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if _, err := w.Write(raw); err != nil {
		log.Printf("getShopeeItemDetails write failed: %v", err)
	}
}

func (s *server) findSimilarProductHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusMethodNotAllowed)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "method_not_allowed"})
		return
	}

	keyword := strings.TrimSpace(r.URL.Query().Get("keyword"))
	if keyword == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "keyword_required"})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	raw, err := s.productOffer.GetProductOfferByKeyword(ctx, GetProductOfferByKeywordRequest{
		Keyword: keyword,
		Page:    0,
		Limit:   10,
	})
	if err != nil {
		log.Printf("findSimilarProduct GetProductOfferByKeyword failed: %v", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "shopee_api_failed"})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if _, err := w.Write(raw); err != nil {
		log.Printf("findSimilarProduct write failed: %v", err)
	}
}

func notFoundHandler(w http.ResponseWriter, r *http.Request) {
	log.Printf("not found: path=%s method=%s remote=%s", r.URL.Path, r.Method, r.RemoteAddr)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotFound)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": "not_found"})
}
