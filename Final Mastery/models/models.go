package models

type Album struct {
	AlbumID     string `json:"album_id" dynamodbav:"album_id"`
	Title       string `json:"title" dynamodbav:"title"`
	Description string `json:"description" dynamodbav:"description"`
	Owner       string `json:"owner" dynamodbav:"owner"`
	PhotoSeq    int    `json:"photo_seq,omitempty" dynamodbav:"photo_seq"`
}

type Photo struct {
	PhotoID string `json:"photo_id" dynamodbav:"photo_id"`
	AlbumID string `json:"album_id" dynamodbav:"album_id"`
	Seq     int    `json:"seq" dynamodbav:"seq"`
	Status  string `json:"status" dynamodbav:"status"`
	URL     string `json:"url,omitempty" dynamodbav:"url,omitempty"`
}
