package main

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/google/uuid"
)

// MySQLStore implements CartStore using Amazon RDS MySQL.
//
// Schema design rationale:
//   - Two tables (carts + cart_items) follow normalized relational design.
//   - carts table: stores cart metadata (customer, status, timestamps).
//   - cart_items table: stores line items with a foreign key to carts.
//   - Composite unique index on (cart_id, product_id) prevents duplicate products
//     and enables upsert behavior via INSERT ... ON DUPLICATE KEY UPDATE.
//   - Index on customer_id supports "get all carts by customer" queries.
//   - Foreign key with CASCADE delete ensures no orphaned items.
//
// Connection pooling:
//   - MaxOpenConns=25 limits total connections (db.t3.micro supports ~66 max).
//   - MaxIdleConns=10 keeps warm connections for low-latency reuse.
//   - ConnMaxLifetime=5m prevents stale connections.
type MySQLStore struct {
	db *sql.DB
}

func NewMySQLStore(dsn string) (*MySQLStore, error) {
	db, err := sql.Open("mysql", dsn+"?parseTime=true")
	if err != nil {
		return nil, fmt.Errorf("failed to open MySQL: %w", err)
	}

	// Connection pool configuration
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(10)
	db.SetConnMaxLifetime(5 * time.Minute)

	// Verify connectivity
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping MySQL: %w", err)
	}

	store := &MySQLStore{db: db}
	if err := store.initSchema(); err != nil {
		return nil, fmt.Errorf("failed to init schema: %w", err)
	}

	log.Println("MySQL store initialized successfully")
	return store, nil
}

func (s *MySQLStore) initSchema() error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS carts (
			id VARCHAR(36) PRIMARY KEY,
			customer_id VARCHAR(255) NOT NULL,
			status VARCHAR(50) NOT NULL DEFAULT 'active',
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			INDEX idx_customer_id (customer_id)
		) ENGINE=InnoDB`,

		`CREATE TABLE IF NOT EXISTS cart_items (
			id INT AUTO_INCREMENT PRIMARY KEY,
			cart_id VARCHAR(36) NOT NULL,
			product_id VARCHAR(255) NOT NULL,
			name VARCHAR(255) NOT NULL DEFAULT '',
			quantity INT NOT NULL DEFAULT 1,
			price DECIMAL(10,2) NOT NULL DEFAULT 0.00,
			FOREIGN KEY (cart_id) REFERENCES carts(id) ON DELETE CASCADE,
			UNIQUE INDEX idx_cart_product (cart_id, product_id)
		) ENGINE=InnoDB`,
	}

	for _, q := range queries {
		if _, err := s.db.Exec(q); err != nil {
			return err
		}
	}
	return nil
}

func (s *MySQLStore) CreateCart(customerID string) (*Cart, error) {
	id := uuid.New().String()
	now := time.Now().UTC()

	_, err := s.db.Exec(
		"INSERT INTO carts (id, customer_id, status, created_at, updated_at) VALUES (?, ?, 'active', ?, ?)",
		id, customerID, now, now,
	)
	if err != nil {
		return nil, err
	}

	return &Cart{
		ID:         id,
		CustomerID: customerID,
		Status:     "active",
		CreatedAt:  now,
		UpdatedAt:  now,
		Items:      []CartItem{},
	}, nil
}

func (s *MySQLStore) GetCart(id string) (*Cart, error) {
	// Get cart metadata
	cart := &Cart{}
	err := s.db.QueryRow(
		"SELECT id, customer_id, status, created_at, updated_at FROM carts WHERE id = ?", id,
	).Scan(&cart.ID, &cart.CustomerID, &cart.Status, &cart.CreatedAt, &cart.UpdatedAt)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("cart not found")
		}
		return nil, err
	}

	// Get items via JOIN-equivalent (separate query for clarity, same performance with index)
	rows, err := s.db.Query(
		"SELECT product_id, name, quantity, price FROM cart_items WHERE cart_id = ?", id,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	cart.Items = []CartItem{}
	for rows.Next() {
		var item CartItem
		if err := rows.Scan(&item.ProductID, &item.Name, &item.Quantity, &item.Price); err != nil {
			return nil, err
		}
		cart.Items = append(cart.Items, item)
	}

	return cart, rows.Err()
}

func (s *MySQLStore) AddItem(cartID string, item CartItem) (*Cart, error) {
	// Verify cart exists
	var exists bool
	err := s.db.QueryRow("SELECT EXISTS(SELECT 1 FROM carts WHERE id = ?)", cartID).Scan(&exists)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, fmt.Errorf("cart not found")
	}

	// Upsert item: insert or update quantity if product already in cart.
	// The UNIQUE INDEX on (cart_id, product_id) makes this safe for concurrent access.
	_, err = s.db.Exec(
		`INSERT INTO cart_items (cart_id, product_id, name, quantity, price)
		 VALUES (?, ?, ?, ?, ?)
		 ON DUPLICATE KEY UPDATE quantity = VALUES(quantity), name = VALUES(name), price = VALUES(price)`,
		cartID, item.ProductID, item.Name, item.Quantity, item.Price,
	)
	if err != nil {
		return nil, err
	}

	// Update cart timestamp
	s.db.Exec("UPDATE carts SET updated_at = ? WHERE id = ?", time.Now().UTC(), cartID)

	return s.GetCart(cartID)
}

func (s *MySQLStore) Close() error {
	return s.db.Close()
}
