package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
)

func main() {
	dbType := os.Getenv("DB_TYPE") // "mysql" or "dynamodb"
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	var store CartStore
	var err error

	switch dbType {
	case "mysql":
		// DSN format: user:password@tcp(host:3306)/dbname
		dsn := os.Getenv("MYSQL_DSN")
		if dsn == "" {
			log.Fatal("MYSQL_DSN environment variable is required when DB_TYPE=mysql")
		}
		store, err = NewMySQLStore(dsn)
	case "dynamodb":
		region := os.Getenv("AWS_REGION")
		if region == "" {
			region = "us-east-1"
		}
		tableName := os.Getenv("DYNAMO_TABLE")
		if tableName == "" {
			tableName = "shopping-carts"
		}
		store, err = NewDynamoStore(region, tableName)
	default:
		log.Fatalf("DB_TYPE must be 'mysql' or 'dynamodb', got: '%s'", dbType)
	}

	if err != nil {
		log.Fatalf("Failed to initialize %s store: %v", dbType, err)
	}
	defer store.Close()

	log.Printf("Starting server on :%s with %s backend", port, dbType)

	// Router using standard library
	mux := http.NewServeMux()

	mux.HandleFunc("/health", handleHealth)

	// Custom routing for /shopping-carts paths
	mux.HandleFunc("/shopping-carts", func(w http.ResponseWriter, r *http.Request) {
		// POST /shopping-carts — create cart
		if r.URL.Path == "/shopping-carts" && r.Method == http.MethodPost {
			handleCreateCart(store)(w, r)
			return
		}
		writeError(w, http.StatusNotFound, "not found")
	})

	mux.HandleFunc("/shopping-carts/", func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		parts := strings.Split(strings.TrimPrefix(path, "/"), "/")

		switch {
		// GET /shopping-carts/{id}
		case len(parts) == 2 && r.Method == http.MethodGet:
			handleGetCart(store)(w, r)

		// POST /shopping-carts/{id}/items
		case len(parts) == 3 && parts[2] == "items" && r.Method == http.MethodPost:
			handleAddItem(store)(w, r)

		default:
			writeError(w, http.StatusNotFound, "not found")
		}
	})

	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%s", port), mux))
}
