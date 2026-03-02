package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// CircuitBreaker implements the circuit breaker pattern
type CircuitBreaker struct {
	mu               sync.Mutex
	state            string // "CLOSED", "OPEN", "HALF_OPEN"
	failureCount     int
	successCount     int
	failureThreshold int
	recoveryTimeout  time.Duration
	lastFailureTime  time.Time
}

func NewCircuitBreaker(failureThreshold int, recoveryTimeout time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		state:            "CLOSED",
		failureThreshold: failureThreshold,
		recoveryTimeout:  recoveryTimeout,
	}
}

func (cb *CircuitBreaker) CanRequest() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case "CLOSED":
		return true
	case "OPEN":
		// Check if recovery timeout has passed
		if time.Since(cb.lastFailureTime) > cb.recoveryTimeout {
			cb.state = "HALF_OPEN"
			fmt.Println("[CIRCUIT BREAKER] State: OPEN -> HALF_OPEN (testing recovery)")
			return true
		}
		return false
	case "HALF_OPEN":
		return true
	}
	return false
}

func (cb *CircuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if cb.state == "HALF_OPEN" {
		cb.successCount++
		if cb.successCount >= 2 {
			cb.state = "CLOSED"
			cb.failureCount = 0
			cb.successCount = 0
			fmt.Println("[CIRCUIT BREAKER] State: HALF_OPEN -> CLOSED (service recovered!)")
		}
	} else {
		cb.failureCount = 0
	}
}

func (cb *CircuitBreaker) RecordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.failureCount++
	cb.lastFailureTime = time.Now()
	cb.successCount = 0

	if cb.failureCount >= cb.failureThreshold {
		cb.state = "OPEN"
		fmt.Printf("[CIRCUIT BREAKER] State: -> OPEN (failures: %d)\n", cb.failureCount)
	}
}

func (cb *CircuitBreaker) GetState() string {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	return cb.state
}

func main() {
	backendURL := "http://localhost:8080"
	cb := NewCircuitBreaker(3, 5*time.Second) // open after 3 failures, retry after 5s

	router := gin.Default()

	// Proxy all requests through circuit breaker
	router.Any("/*path", func(c *gin.Context) {
		path := c.Param("path")

		// Check circuit breaker
		if !cb.CanRequest() {
			fmt.Printf("[PROXY] Circuit OPEN — returning fallback for %s\n", path)
			c.JSON(http.StatusOK, gin.H{
				"source":  "fallback_cache",
				"message": "Service temporarily unavailable, returning cached data",
				"data": []gin.H{
					{"id": "1", "title": "Blue Train", "artist": "John Coltrane", "price": 56.99},
					{"id": "2", "title": "Jeru", "artist": "Gerry Mulligan", "price": 17.99},
					{"id": "3", "title": "Sarah Vaughan and Clifford Brown", "artist": "Sarah Vaughan", "price": 39.99},
				},
				"circuit_state": cb.GetState(),
			})
			return
		}

		// Forward request to backend
		targetURL := backendURL + path
		resp, err := http.Get(targetURL)

		if err != nil || resp.StatusCode >= 500 {
			cb.RecordFailure()
			if err != nil {
				fmt.Printf("[PROXY] Backend error: %s\n", err)
			} else {
				resp.Body.Close()
				fmt.Printf("[PROXY] Backend returned %d\n", resp.StatusCode)
			}
			// Return fallback on failure
			c.JSON(http.StatusOK, gin.H{
				"source":        "fallback_cache",
				"message":       "Backend failed, returning cached data",
				"circuit_state": cb.GetState(),
			})
			return
		}

		// Success — forward response
		cb.RecordSuccess()
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		c.Data(resp.StatusCode, resp.Header.Get("Content-Type"), body)
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "9090"
	}

	fmt.Println("=== Circuit Breaker Proxy ===")
	fmt.Printf("Proxy on :%s -> Backend on %s\n", port, backendURL)
	fmt.Printf("Failure threshold: %d, Recovery timeout: %s\n", cb.failureThreshold, cb.recoveryTimeout)
	router.Run(":" + port)
}
