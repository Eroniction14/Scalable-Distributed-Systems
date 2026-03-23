package main

import "time"

// Cart represents a shopping cart
type Cart struct {
	ID         string     `json:"id"`
	CustomerID string     `json:"customer_id"`
	Status     string     `json:"status"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
	Items      []CartItem `json:"items"`
}

// CartItem represents an item in a shopping cart
type CartItem struct {
	ProductID string  `json:"product_id"`
	Name      string  `json:"name"`
	Quantity  int     `json:"quantity"`
	Price     float64 `json:"price"`
}

// CreateCartRequest is the payload for POST /shopping-carts
type CreateCartRequest struct {
	CustomerID string `json:"customer_id"`
}

// AddItemRequest is the payload for POST /shopping-carts/{id}/items
type AddItemRequest struct {
	ProductID string  `json:"product_id"`
	Name      string  `json:"name"`
	Quantity  int     `json:"quantity"`
	Price     float64 `json:"price"`
}

// ErrorResponse for consistent error formatting
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message,omitempty"`
}
