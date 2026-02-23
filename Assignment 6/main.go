package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
)

type Product struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Category    string `json:"category"`
	Description string `json:"description"`
	Brand       string `json:"brand"`
}

type SearchResponse struct {
	Products   []Product `json:"products"`
	TotalFound int       `json:"total_found"`
	SearchTime string    `json:"search_time"`
}

var productStore sync.Map

var brands = []string{"Alpha", "Beta", "Gamma", "Delta", "Epsilon", "Zeta", "Theta", "Iota", "Kappa", "Lambda"}
var categories = []string{"Electronics", "Books", "Home", "Sports", "Clothing", "Toys", "Food", "Health", "Auto", "Garden"}

func generateProducts() {
	for i := 1; i <= 100000; i++ {
		brand := brands[i%len(brands)]
		category := categories[i%len(categories)]
		p := Product{
			ID:          i,
			Name:        fmt.Sprintf("Product %s %d", brand, i),
			Category:    category,
			Description: fmt.Sprintf("A great %s product by %s", category, brand),
			Brand:       brand,
		}
		productStore.Store(i, p)
	}
	log.Println("Generated 100,000 products")
}

func searchHandler(w http.ResponseWriter, r *http.Request) {
	query := strings.ToLower(r.URL.Query().Get("q"))
	if query == "" {
		http.Error(w, "missing query parameter 'q'", http.StatusBadRequest)
		return
	}

	start := time.Now()

	var results []Product
	totalFound := 0
	checked := 0

	productStore.Range(func(key, value interface{}) bool {
		if checked >= 100 {
			return false // stop after checking 100 products
		}
		checked++

		p := value.(Product)
		if strings.Contains(strings.ToLower(p.Name), query) ||
			strings.Contains(strings.ToLower(p.Category), query) {
			totalFound++
			if len(results) < 20 {
				results = append(results, p)
			}
		}
		return true
	})

	resp := SearchResponse{
		Products:   results,
		TotalFound: totalFound,
		SearchTime: time.Since(start).String(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

func main() {
	generateProducts()

	http.HandleFunc("/products/search", searchHandler)
	http.HandleFunc("/health", healthHandler)

	log.Println("Server starting on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
