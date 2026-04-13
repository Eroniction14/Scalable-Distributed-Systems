package handlers

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"

	"album-store/models"
	"album-store/store"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

var (
	s3Client *s3.Client
	bucket   string
)

func InitAWS() {
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		panic(fmt.Sprintf("unable to load AWS config: %v", err))
	}

	s3Client = s3.NewFromConfig(cfg)

	bucket = os.Getenv("S3_BUCKET")
	if bucket == "" {
		bucket = "album-store-photos"
	}
}

func UploadPhoto(c *gin.Context) {
	albumID := c.Param("album_id")

	album, err := store.GetAlbum(c.Request.Context(), albumID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if album == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "album not found"})
		return
	}

	file, _, err := c.Request.FormFile("photo")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing photo field"})
		return
	}
	defer file.Close()

	photoBytes, err := io.ReadAll(file)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read photo"})
		return
	}

	photoID := uuid.New().String()

	seq, err := store.IncrementPhotoSeq(c.Request.Context(), albumID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to assign sequence"})
		return
	}

	// Upload to S3 final path
	finalKey := fmt.Sprintf("photos/%s/%s", albumID, photoID)
	_, err = s3Client.PutObject(c.Request.Context(), &s3.PutObjectInput{
		Bucket: &bucket,
		Key:    &finalKey,
		Body:   bytes.NewReader(photoBytes),
	})
	if err != nil {
		log.Printf("S3 upload error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to upload to S3"})
		return
	}

	// Save photo as completed in one write
	url := fmt.Sprintf("https://%s.s3.amazonaws.com/%s", bucket, finalKey)
	photo := models.Photo{
		PhotoID: photoID,
		AlbumID: albumID,
		Seq:     seq,
		Status:  "completed",
		URL:     url,
	}
	if err := store.PutPhoto(c.Request.Context(), photo); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save photo metadata"})
		return
	}

	// Return 202 — spec requires "processing" in response body
	c.JSON(http.StatusAccepted, gin.H{
		"photo_id": photoID,
		"seq":      seq,
		"status":   "processing",
	})
}

func GetPhoto(c *gin.Context) {
	photoID := c.Param("photo_id")

	photo, err := store.GetPhoto(c.Request.Context(), photoID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if photo == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}

	resp := gin.H{
		"photo_id": photo.PhotoID,
		"album_id": photo.AlbumID,
		"seq":      photo.Seq,
		"status":   photo.Status,
	}

	if photo.Status == "completed" && photo.URL != "" {
		resp["url"] = photo.URL
	}

	c.JSON(http.StatusOK, resp)
}

func DeletePhoto(c *gin.Context) {
	photoID := c.Param("photo_id")
	albumID := c.Param("album_id")

	photo, err := store.GetPhoto(c.Request.Context(), photoID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if photo == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}

	finalKey := fmt.Sprintf("photos/%s/%s", albumID, photoID)
	s3Client.DeleteObject(c.Request.Context(), &s3.DeleteObjectInput{
		Bucket: &bucket,
		Key:    &finalKey,
	})

	if err := store.DeletePhoto(c.Request.Context(), photoID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Status(http.StatusNoContent)
}
