package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/gin-gonic/gin"
)

var s3Client *s3.Client

func init() {
	cfg, err := config.LoadDefaultConfig(context.TODO(), config.WithRegion("us-west-2"))
	if err != nil {
		log.Fatal(err)
	}
	s3Client = s3.NewFromConfig(cfg)
}

func main() {
	router := gin.Default()

	router.GET("/map", mapHandler)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	router.Run(":" + port)
}

func mapHandler(c *gin.Context) {
	bucket := c.Query("bucket")
	key := c.Query("key")
	outputKey := c.Query("output")

	if bucket == "" || key == "" || outputKey == "" {
		c.JSON(400, gin.H{"error": "bucket, key, and output required"})
		return
	}

	// Download chunk from S3
	result, err := s3Client.GetObject(context.TODO(), &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		c.JSON(500, gin.H{"error": fmt.Sprintf("failed to download: %v", err)})
		return
	}
	defer result.Body.Close()

	// Read content
	buf := new(bytes.Buffer)
	buf.ReadFrom(result.Body)
	content := buf.String()

	// Count words
	wordCounts := make(map[string]int)
	words := strings.Fields(content)
	for _, word := range words {
		// Clean word
		word = strings.ToLower(strings.Trim(word, ".,!?;:\"'()[]{}"))
		if word != "" {
			wordCounts[word]++
		}
	}

	// Convert to JSON
	resultJSON, err := json.Marshal(wordCounts)
	if err != nil {
		c.JSON(500, gin.H{"error": fmt.Sprintf("failed to marshal: %v", err)})
		return
	}

	// Upload result to S3
	_, err = s3Client.PutObject(context.TODO(), &s3.PutObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(outputKey),
		Body:   bytes.NewReader(resultJSON),
	})
	if err != nil {
		c.JSON(500, gin.H{"error": fmt.Sprintf("failed to upload: %v", err)})
		return
	}

	c.JSON(200, gin.H{"result": fmt.Sprintf("s3://%s/%s", bucket, outputKey)})
}
