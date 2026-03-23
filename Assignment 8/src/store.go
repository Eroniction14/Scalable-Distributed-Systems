package main

// CartStore defines the interface for cart persistence.
// Both MySQL and DynamoDB implement this interface,
// allowing the service to swap backends via DB_TYPE env var.
type CartStore interface {
	// CreateCart creates a new cart and returns it with a generated ID
	CreateCart(customerID string) (*Cart, error)

	// GetCart retrieves a cart by ID, including all its items
	GetCart(id string) (*Cart, error)

	// AddItem adds or updates an item in an existing cart.
	// If a product already exists in the cart, its quantity is updated.
	AddItem(cartID string, item CartItem) (*Cart, error)

	// Close cleans up any resources (connection pools, etc.)
	Close() error
}
