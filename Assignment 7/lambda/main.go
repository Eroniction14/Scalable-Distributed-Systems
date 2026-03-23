package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sns"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"
	"github.com/google/uuid"
)

// ─── Models ───────────────────────────────────────────────────────

type Item struct {
	ProductID string  `json:"product_id"`
	Quantity  int     `json:"quantity"`
	Price     float64 `json:"price"`
}

type Order struct {
	OrderID    string    `json:"order_id"`
	CustomerID int       `json:"customer_id"`
	Status     string    `json:"status"`
	Items      []Item    `json:"items"`
	CreatedAt  time.Time `json:"created_at"`
}

type OrderRequest struct {
	CustomerID int    `json:"customer_id"`
	Items      []Item `json:"items"`
}

// ─── Simulated Payment (buffered-channel bottleneck) ──────────────

// paymentMu ensures only ONE payment processes at a time.
// All other requests block on Lock() until the current payment finishes.
var paymentMu sync.Mutex

func verifyPayment(orderID string) {
	paymentMu.Lock()
	defer paymentMu.Unlock()
	log.Printf("[payment] verifying order %s (3s)", orderID)
	time.Sleep(3 * time.Second)
	log.Printf("[payment] order %s verified", orderID)
}

// ─── Receiver Mode ────────────────────────────────────────────────

func runReceiver() {
	region := os.Getenv("AWS_REGION")
	topicArn := os.Getenv("SNS_TOPIC_ARN")

	cfg, err := config.LoadDefaultConfig(context.TODO(), config.WithRegion(region))
	if err != nil {
		log.Fatalf("unable to load AWS config: %v", err)
	}
	snsClient := sns.NewFromConfig(cfg)

	mux := http.NewServeMux()

	// Health check
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "OK")
	})

	// ── Synchronous endpoint ──
	mux.HandleFunc("/orders/sync", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req OrderRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}

		order := Order{
			OrderID:    uuid.New().String(),
			CustomerID: req.CustomerID,
			Status:     "processing",
			Items:      req.Items,
			CreatedAt:  time.Now(),
		}

		// Blocking: customer waits the full 3 seconds
		verifyPayment(order.OrderID)
		order.Status = "completed"

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(order)
	})

	// ── Asynchronous endpoint ──
	mux.HandleFunc("/orders/async", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req OrderRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}

		order := Order{
			OrderID:    uuid.New().String(),
			CustomerID: req.CustomerID,
			Status:     "pending",
			Items:      req.Items,
			CreatedAt:  time.Now(),
		}

		// Publish to SNS — return immediately
		body, _ := json.Marshal(order)
		_, err := snsClient.Publish(context.TODO(), &sns.PublishInput{
			TopicArn: &topicArn,
			Message:  strPtr(string(body)),
		})
		if err != nil {
			log.Printf("[async] SNS publish failed: %v", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted) // 202
		json.NewEncoder(w).Encode(order)
	})

	log.Println("[receiver] listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", mux))
}

// ─── Processor Mode ───────────────────────────────────────────────

func runProcessor() {
	region := os.Getenv("AWS_REGION")
	queueURL := os.Getenv("SQS_QUEUE_URL")
	numWorkers := getEnvInt("NUM_WORKERS", 1)

	cfg, err := config.LoadDefaultConfig(context.TODO(), config.WithRegion(region))
	if err != nil {
		log.Fatalf("unable to load AWS config: %v", err)
	}
	sqsClient := sqs.NewFromConfig(cfg)

	log.Printf("[processor] starting %d workers, polling %s", numWorkers, queueURL)

	// Also start a health endpoint so ECS doesn't kill us
	go func() {
		mux := http.NewServeMux()
		mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, "OK")
		})
		http.ListenAndServe(":8080", mux)
	}()

	// Poll loop
	for {
		out, err := sqsClient.ReceiveMessage(context.TODO(), &sqs.ReceiveMessageInput{
			QueueUrl:            &queueURL,
			MaxNumberOfMessages: 10,
			WaitTimeSeconds:     20, // long polling
		})
		if err != nil {
			log.Printf("[processor] receive error: %v", err)
			time.Sleep(2 * time.Second)
			continue
		}

		if len(out.Messages) == 0 {
			continue
		}

		// Use a semaphore to limit concurrent workers
		sem := make(chan struct{}, numWorkers)
		var wg sync.WaitGroup

		for _, msg := range out.Messages {
			sem <- struct{}{}
			wg.Add(1)
			go func(m types.Message) {
				defer wg.Done()
				defer func() { <-sem }()
				processMessage(sqsClient, queueURL, m)
			}(msg)
		}
		wg.Wait()
	}
}

func processMessage(client *sqs.Client, queueURL string, msg types.Message) {
	// SNS wraps the actual message in an envelope
	var snsEnvelope struct {
		Message string `json:"Message"`
	}
	if err := json.Unmarshal([]byte(*msg.Body), &snsEnvelope); err != nil {
		log.Printf("[processor] bad SNS envelope: %v", err)
		deleteMsg(client, queueURL, msg)
		return
	}

	var order Order
	if err := json.Unmarshal([]byte(snsEnvelope.Message), &order); err != nil {
		log.Printf("[processor] bad order JSON: %v", err)
		deleteMsg(client, queueURL, msg)
		return
	}

	// Process payment (3s blocking)
	verifyPayment(order.OrderID)

	// Delete from queue
	deleteMsg(client, queueURL, msg)
	log.Printf("[processor] completed order %s for customer %d", order.OrderID, order.CustomerID)
}

func deleteMsg(client *sqs.Client, queueURL string, msg types.Message) {
	_, err := client.DeleteMessage(context.TODO(), &sqs.DeleteMessageInput{
		QueueUrl:      &queueURL,
		ReceiptHandle: msg.ReceiptHandle,
	})
	if err != nil {
		log.Printf("[processor] delete error: %v", err)
	}
}

// ─── Main ─────────────────────────────────────────────────────────

func main() {
	mode := os.Getenv("MODE")
	switch mode {
	case "receiver":
		runReceiver()
	case "processor":
		runProcessor()
	default:
		log.Fatalf("MODE env must be 'receiver' or 'processor', got '%s'", mode)
	}
}

// ─── Helpers ──────────────────────────────────────────────────────

func strPtr(s string) *string { return &s }

func getEnvInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}
