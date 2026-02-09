package main

import (
	"bytes"
	"context"
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

	router.GET("/split", splitHandler)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	router.Run(":" + port)
}

func splitHandler(c *gin.Context) {
	bucket := c.Query("bucket")
	key := c.Query("key")

	if bucket == "" || key == "" {
		c.JSON(400, gin.H{"error": "bucket and key required"})
		return
	}

	// Download file from S3
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

	// Split into 3 chunks
	chunkSize := len(content) / 3
	chunks := []string{
		content[:chunkSize],
		content[chunkSize : 2*chunkSize],
		content[2*chunkSize:],
	}

	// Upload chunks to S3
	chunkURLs := []string{}
	for i, chunk := range chunks {
		chunkKey := fmt.Sprintf("chunk-%d.txt", i)
		_, err := s3Client.PutObject(context.TODO(), &s3.PutObjectInput{
			Bucket: aws.String(bucket),
			Key:    aws.String(chunkKey),
			Body:   bytes.NewReader([]byte(chunk)),
		})
		if err != nil {
			c.JSON(500, gin.H{"error": fmt.Sprintf("failed to upload chunk: %v", err)})
			return
		}
		chunkURLs = append(chunkURLs, fmt.Sprintf("s3://%s/%s", bucket, chunkKey))
	}

	c.JSON(200, gin.H{"chunks": chunkURLs})
}
