package main

import (
	"log"
	"net/http"
	"time"
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

// server holds shared dependencies for handlers.
type server struct {
	productOffer *ProductOfferClient
}

func main() {
	baseURL := "https://open-api.affiliate.shopee.co.id/graphql"
	appID := "11177960001"
	secret := "ZPBCHYLXKWPT74E3JU2AMHPWSR6OZ7HW"

	productOfferClient := NewProductOfferClient(ProductOfferClientConfig{
		BaseURL: baseURL,
		AppID:   appID,
		Secret:  secret,
		Timeout: 30 * time.Second,
	})
	srv := &server{productOffer: productOfferClient}

	log.Println("registering handlers")
	mux := http.NewServeMux()
	mux.HandleFunc("/ping", pingHandler)
	mux.HandleFunc("/resolve-shopee-url", resolveShopeeURLHandler)
	mux.HandleFunc("/get-shopee-item-details/", srv.getShopeeItemDetailsHandler)
	mux.HandleFunc("/", notFoundHandler)

	log.Println("HTTP now server listening on :8080")
	if err := http.ListenAndServe(":8080", corsAll(mux)); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}
