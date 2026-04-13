package handlers

import (
	"net/http"

	"album-store/models"
	"album-store/store"

	"github.com/gin-gonic/gin"
)

// CreateOrUpdateAlbum handles PUT /albums/:album_id
// Idempotent — same album_id overwrites, never duplicates.
func CreateOrUpdateAlbum(c *gin.Context) {
	albumID := c.Param("album_id")

	var album models.Album
	if err := c.ShouldBindJSON(&album); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	// Ensure the path param takes priority over the body
	album.AlbumID = albumID

	if err := store.PutAlbum(c.Request.Context(), album); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"album_id":    album.AlbumID,
		"title":       album.Title,
		"description": album.Description,
		"owner":       album.Owner,
	})
}

// GetAlbum handles GET /albums/:album_id
func GetAlbum(c *gin.Context) {
	albumID := c.Param("album_id")

	album, err := store.GetAlbum(c.Request.Context(), albumID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if album == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"album_id":    album.AlbumID,
		"title":       album.Title,
		"description": album.Description,
		"owner":       album.Owner,
	})
}

// ListAlbums handles GET /albums
func ListAlbums(c *gin.Context) {
	albums, err := store.ListAlbums(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, albums)
}
