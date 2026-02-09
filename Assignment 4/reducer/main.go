package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

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

	router.GET("/reduce", reduceHandler)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	router.Run(":" + port)
}

func reduceHandler(c *gin.Context) {
	bucket := c.Query("bucket")
	mapperKeys := c.QueryArray("mapper")
	outputKey := c.Query("output")

	if bucket == "" || len(mapperKeys) == 0 || outputKey == "" {
		c.JSON(400, gin.H{"error": "bucket, mapper keys, and output required"})
		return
	}

	// Aggregate word counts from all mappers
	finalCounts := make(map[string]int)

	for _, key := range mapperKeys {
		// Download mapper result
		result, err := s3Client.GetObject(context.TODO(), &s3.GetObjectInput{
			Bucket: aws.String(bucket),
			Key:    aws.String(key),
		})
		if err != nil {
			c.JSON(500, gin.H{"error": fmt.Sprintf("failed to download %s: %v", key, err)})
			return
		}

		// Parse JSON
		var counts map[string]int
		if err := json.NewDecoder(result.Body).Decode(&counts); err != nil {
			result.Body.Close()
			c.JSON(500, gin.H{"error": fmt.Sprintf("failed to parse %s: %v", key, err)})
			return
		}
		result.Body.Close()

		// Merge counts
		for word, count := range counts {
			finalCounts[word] += count
		}
	}

	// Convert to JSON
	resultJSON, err := json.Marshal(finalCounts)
	if err != nil {
		c.JSON(500, gin.H{"error": fmt.Sprintf("failed to marshal: %v", err)})
		return
	}

	// Upload final result to S3
	_, err = s3Client.PutObject(context.TODO(), &s3.PutObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(outputKey),
		Body:   bytes.NewReader(resultJSON),
	})
	if err != nil {
		c.JSON(500, gin.H{"error": fmt.Sprintf("failed to upload: %v", err)})
		return
	}

	c.JSON(200, gin.H{
		"result":      fmt.Sprintf("s3://%s/%s", bucket, outputKey),
		"total_words": len(finalCounts),
	})
}
