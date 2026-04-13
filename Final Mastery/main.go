package main

import (
	"log"
	"os"

	"album-store/handlers"
	"album-store/store"

	"github.com/gin-gonic/gin"
)

func main() {
	gin.SetMode(gin.ReleaseMode)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	store.Init()
	handlers.InitAWS()

	r := gin.New()
	r.Use(gin.Recovery())

	r.GET("/health", handlers.Health)
	r.PUT("/albums/:album_id", handlers.CreateOrUpdateAlbum)
	r.GET("/albums/:album_id", handlers.GetAlbum)
	r.GET("/albums", handlers.ListAlbums)
	r.POST("/albums/:album_id/photos", handlers.UploadPhoto)
	r.GET("/albums/:album_id/photos/:photo_id", handlers.GetPhoto)
	r.DELETE("/albums/:album_id/photos/:photo_id", handlers.DeletePhoto)

	log.Printf("Starting server on port %s", port)
	if err := r.Run(":" + port); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
