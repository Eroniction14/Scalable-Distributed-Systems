package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
)

// Product represents the product schema from api.yaml
type Product struct {
	ProductID    int    `json:"product_id"`
	SKU          string `json:"sku"`
	Manufacturer string `json:"manufacturer"`
	CategoryID   int    `json:"category_id"`
	Weight       int    `json:"weight"`
	SomeOtherID  int    `json:"some_other_id"`
}

// ErrorResponse represents the error schema from api.yaml
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
	Details string `json:"details,omitempty"`
}

// In-memory store
var (
	products = make(map[int]Product)
	mu       sync.RWMutex
)

func main() {
	http.HandleFunc("/products/", productsHandler)
	http.HandleFunc("/health", healthHandler)

	port := ":8080"
	log.Printf("Product API server starting on port %s", port)
	log.Fatal(http.ListenAndServe(port, nil))
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status": "healthy"}`))
}

func productsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Parse the path: /products/{productId} or /products/{productId}/details
	path := strings.TrimPrefix(r.URL.Path, "/products/")
	parts := strings.Split(path, "/")

	if len(parts) == 0 || parts[0] == "" {
		writeError(w, http.StatusBadRequest, "INVALID_INPUT", "Product ID is required", "")
		return
	}

	// Parse product ID
	productID, err := strconv.Atoi(parts[0])
	if err != nil || productID < 1 {
		writeError(w, http.StatusBadRequest, "INVALID_INPUT", "Invalid product ID", "Product ID must be a positive integer")
		return
	}

	// Route: GET /products/{productId}
	if len(parts) == 1 && r.Method == http.MethodGet {
		getProduct(w, productID)
		return
	}

	// Route: POST /products/{productId}/details
	if len(parts) == 2 && parts[1] == "details" && r.Method == http.MethodPost {
		addProductDetails(w, r, productID)
		return
	}

	writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid endpoint or method", "")
}

// GET /products/{productId}
func getProduct(w http.ResponseWriter, productID int) {
	mu.RLock()
	product, exists := products[productID]
	mu.RUnlock()

	if !exists {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "Product not found", fmt.Sprintf("No product found with ID %d", productID))
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(product)
}

// POST /products/{productId}/details
func addProductDetails(w http.ResponseWriter, r *http.Request, productID int) {
	var product Product
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&product); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_INPUT", "Invalid JSON body", err.Error())
		return
	}

	// Validate required fields
	if err := validateProduct(product, productID); err != "" {
		writeError(w, http.StatusBadRequest, "INVALID_INPUT", "Validation failed", err)
		return
	}

	// Ensure the product_id in the body matches the URL path
	product.ProductID = productID

	mu.Lock()
	products[productID] = product
	mu.Unlock()

	w.WriteHeader(http.StatusNoContent)
}

// validateProduct checks all required fields per the OpenAPI spec
func validateProduct(p Product, pathID int) string {
	var errors []string

	if p.ProductID != 0 && p.ProductID != pathID {
		errors = append(errors, "product_id in body must match URL path")
	}
	if p.SKU == "" {
		errors = append(errors, "sku is required")
	} else if len(p.SKU) > 100 {
		errors = append(errors, "sku must be at most 100 characters")
	}
	if p.Manufacturer == "" {
		errors = append(errors, "manufacturer is required")
	} else if len(p.Manufacturer) > 200 {
		errors = append(errors, "manufacturer must be at most 200 characters")
	}
	if p.CategoryID < 1 {
		errors = append(errors, "category_id must be a positive integer")
	}
	if p.Weight < 0 {
		errors = append(errors, "weight must be non-negative")
	}
	if p.SomeOtherID < 1 {
		errors = append(errors, "some_other_id must be a positive integer")
	}

	if len(errors) > 0 {
		return strings.Join(errors, "; ")
	}
	return ""
}

func writeError(w http.ResponseWriter, status int, errCode, message, details string) {
	w.WriteHeader(status)
	resp := ErrorResponse{
		Error:   errCode,
		Message: message,
		Details: details,
	}
	json.NewEncoder(w).Encode(resp)
}
