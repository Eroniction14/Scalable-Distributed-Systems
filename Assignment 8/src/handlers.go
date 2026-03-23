package main

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
)

// extractCartID pulls the cart ID from URLs like /shopping-carts/{id} or /shopping-carts/{id}/items
func extractCartID(path string) string {
	// path: /shopping-carts/{id} or /shopping-carts/{id}/items
	parts := strings.Split(strings.TrimPrefix(path, "/"), "/")
	if len(parts) >= 2 {
		return parts[1]
	}
	return ""
}

func handleCreateCart(store CartStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}

		var req CreateCartRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		if req.CustomerID == "" {
			writeError(w, http.StatusBadRequest, "customer_id is required")
			return
		}

		cart, err := store.CreateCart(req.CustomerID)
		if err != nil {
			log.Printf("ERROR creating cart: %v", err)
			writeError(w, http.StatusInternalServerError, "failed to create cart")
			return
		}

		writeJSON(w, http.StatusCreated, cart)
	}
}

func handleGetCart(store CartStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}

		id := extractCartID(r.URL.Path)
		if id == "" {
			writeError(w, http.StatusBadRequest, "cart ID is required")
			return
		}

		cart, err := store.GetCart(id)
		if err != nil {
			if err.Error() == "cart not found" {
				writeError(w, http.StatusNotFound, "cart not found")
				return
			}
			log.Printf("ERROR getting cart %s: %v", id, err)
			writeError(w, http.StatusInternalServerError, "failed to retrieve cart")
			return
		}

		writeJSON(w, http.StatusOK, cart)
	}
}

func handleAddItem(store CartStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}

		cartID := extractCartID(r.URL.Path)
		if cartID == "" {
			writeError(w, http.StatusBadRequest, "cart ID is required")
			return
		}

		var req AddItemRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		if req.ProductID == "" || req.Quantity <= 0 {
			writeError(w, http.StatusBadRequest, "product_id and positive quantity are required")
			return
		}

		item := CartItem{
			ProductID: req.ProductID,
			Name:      req.Name,
			Quantity:  req.Quantity,
			Price:     req.Price,
		}

		cart, err := store.AddItem(cartID, item)
		if err != nil {
			if err.Error() == "cart not found" {
				writeError(w, http.StatusNotFound, "cart not found")
				return
			}
			log.Printf("ERROR adding item to cart %s: %v", cartID, err)
			writeError(w, http.StatusInternalServerError, "failed to add item")
			return
		}

		writeJSON(w, http.StatusCreated, cart)
	}
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "healthy"})
}

// --- helpers ---

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, ErrorResponse{Error: http.StatusText(status), Message: msg})
}
