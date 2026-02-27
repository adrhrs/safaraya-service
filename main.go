package main

import (
	"log"
	"net/http"
)

func corsAll(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func main() {
	log.Println("registering handlers")
	mux := http.NewServeMux()
	mux.HandleFunc("/ping", pingHandler)
	mux.HandleFunc("/resolve-shopee-url", resolveShopeeURLHandler)
	mux.HandleFunc("/", notFoundHandler)

	log.Println("HTTP now server listening on :8080")
	if err := http.ListenAndServe(":8080", corsAll(mux)); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}
