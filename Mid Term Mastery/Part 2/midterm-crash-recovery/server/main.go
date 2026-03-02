package main

import (
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// album represents data about a record album.
type album struct {
	ID     string  `json:"id"`
	Title  string  `json:"title"`
	Artist string  `json:"artist"`
	Price  float64 `json:"price"`
}

// albums slice to seed record album data.
var albums = []album{
	{ID: "1", Title: "Blue Train", Artist: "John Coltrane", Price: 56.99},
	{ID: "2", Title: "Jeru", Artist: "Gerry Mulligan", Price: 17.99},
	{ID: "3", Title: "Sarah Vaughan and Clifford Brown", Artist: "Sarah Vaughan", Price: 39.99},
}

// ---------- CRASH SIMULATION ----------
// Simulates a service that degrades under load and eventually crashes.
var (
	requestCount int
	mu           sync.Mutex
	crashed      bool
)

const crashThreshold = 50 // crash after 50 requests

// crashMiddleware simulates a service that degrades and eventually crashes
func crashMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		mu.Lock()
		requestCount++
		count := requestCount
		mu.Unlock()

		// Stage 1: Normal (0-30 requests)
		if count <= 30 {
			c.Next()
			return
		}

		// Stage 2: Degraded (30-50 requests) — slow responses
		if count <= crashThreshold {
			delay := time.Duration(rand.Intn(3000)+1000) * time.Millisecond
			fmt.Printf("[DEGRADED] Request #%d — adding %v delay\n", count, delay)
			time.Sleep(delay)
			c.Next()
			return
		}

		// Stage 3: Crashed — return 503 Service Unavailable
		mu.Lock()
		crashed = true
		mu.Unlock()
		fmt.Printf("[CRASHED] Request #%d — service unavailable!\n", count)
		c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{
			"error": "Service has crashed due to resource exhaustion",
		})
	}
}

// healthCheck endpoint for monitoring
func healthCheck(c *gin.Context) {
	mu.Lock()
	defer mu.Unlock()
	if crashed {
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": "unhealthy", "requests": requestCount})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "healthy", "requests": requestCount})
}

// ---------- ALBUM HANDLERS ----------

func getAlbums(c *gin.Context) {
	c.IndentedJSON(http.StatusOK, albums)
}

func postAlbums(c *gin.Context) {
	var newAlbum album
	if err := c.BindJSON(&newAlbum); err != nil {
		return
	}
	albums = append(albums, newAlbum)
	c.IndentedJSON(http.StatusCreated, newAlbum)
}

func getAlbumByID(c *gin.Context) {
	id := c.Param("id")
	for _, a := range albums {
		if a.ID == id {
			c.IndentedJSON(http.StatusOK, a)
			return
		}
	}
	c.IndentedJSON(http.StatusNotFound, gin.H{"message": "album not found"})
}

func main() {
	router := gin.Default()

	// Apply crash middleware to all routes
	router.Use(crashMiddleware())

	// Health check (used by circuit breaker)
	router.GET("/health", healthCheck)

	// Album routes
	router.GET("/albums", getAlbums)
	router.GET("/albums/:id", getAlbumByID)
	router.POST("/albums", postAlbums)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	fmt.Println("=== Albums Service (Crash Demo) ===")
	fmt.Printf("Server starting on port %s\n", port)
	fmt.Printf("Will degrade after 30 requests, crash after %d requests\n", crashThreshold)
	router.Run(":" + port)
}
