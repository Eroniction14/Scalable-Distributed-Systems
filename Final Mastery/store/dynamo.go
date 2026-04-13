package store

import (
	"context"
	"fmt"
	"os"

	"album-store/models"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/expression"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

var (
	db          *dynamodb.Client
	albumsTable string
	photosTable string
)

// Init loads AWS config and sets up the DynamoDB client.
// Called once from main.go at startup.
func Init() {
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		panic(fmt.Sprintf("unable to load AWS config: %v", err))
	}

	db = dynamodb.NewFromConfig(cfg)

	albumsTable = os.Getenv("ALBUMS_TABLE")
	if albumsTable == "" {
		albumsTable = "Albums"
	}

	photosTable = os.Getenv("PHOTOS_TABLE")
	if photosTable == "" {
		photosTable = "Photos"
	}
}

// ─── Album Operations ─────────────────────────────────────────────────────────

// PutAlbum creates or overwrites an album. DynamoDB PutItem is inherently
// idempotent — same album_id just overwrites, satisfying the spec.
func PutAlbum(ctx context.Context, album models.Album) error {
	item, err := attributevalue.MarshalMap(album)
	if err != nil {
		return fmt.Errorf("marshal album: %w", err)
	}

	_, err = db.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: &albumsTable,
		Item:      item,
	})
	return err
}

// GetAlbum fetches a single album by ID. Returns (nil, nil) if not found.
func GetAlbum(ctx context.Context, albumID string) (*models.Album, error) {
	out, err := db.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: &albumsTable,
		Key: map[string]types.AttributeValue{
			"album_id": &types.AttributeValueMemberS{Value: albumID},
		},
	})
	if err != nil {
		return nil, err
	}
	if out.Item == nil {
		return nil, nil
	}

	var album models.Album
	if err := attributevalue.UnmarshalMap(out.Item, &album); err != nil {
		return nil, err
	}
	return &album, nil
}

// ListAlbums returns every album in the table. Uses a paginator to handle
// DynamoDB's 1MB-per-response limit — keeps fetching until all items are returned.
func ListAlbums(ctx context.Context) ([]models.Album, error) {
	var albums []models.Album

	paginator := dynamodb.NewScanPaginator(db, &dynamodb.ScanInput{
		TableName: &albumsTable,
	})

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}

		var pageAlbums []models.Album
		if err := attributevalue.UnmarshalListOfMaps(page.Items, &pageAlbums); err != nil {
			return nil, err
		}
		albums = append(albums, pageAlbums...)
	}

	return albums, nil
}

// IncrementPhotoSeq atomically increments the photo_seq counter on an album
// and returns the new value. DynamoDB's ADD operation serializes concurrent
// updates — no race conditions, no duplicate seq numbers.
func IncrementPhotoSeq(ctx context.Context, albumID string) (int, error) {
	out, err := db.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: &albumsTable,
		Key: map[string]types.AttributeValue{
			"album_id": &types.AttributeValueMemberS{Value: albumID},
		},
		UpdateExpression: aws.String("ADD photo_seq :inc"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":inc": &types.AttributeValueMemberN{Value: "1"},
		},
		ReturnValues: types.ReturnValueAllNew,
	})
	if err != nil {
		return 0, err
	}

	var album models.Album
	if err := attributevalue.UnmarshalMap(out.Attributes, &album); err != nil {
		return 0, err
	}
	return album.PhotoSeq, nil
}

// ─── Photo Operations ─────────────────────────────────────────────────────────

// PutPhoto creates a photo record (initially with status=processing).
func PutPhoto(ctx context.Context, photo models.Photo) error {
	item, err := attributevalue.MarshalMap(photo)
	if err != nil {
		return fmt.Errorf("marshal photo: %w", err)
	}

	_, err = db.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: &photosTable,
		Item:      item,
	})
	return err
}

// GetPhoto fetches a single photo by ID. Returns (nil, nil) if not found.
func GetPhoto(ctx context.Context, photoID string) (*models.Photo, error) {
	out, err := db.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: &photosTable,
		Key: map[string]types.AttributeValue{
			"photo_id": &types.AttributeValueMemberS{Value: photoID},
		},
	})
	if err != nil {
		return nil, err
	}
	if out.Item == nil {
		return nil, nil
	}

	var photo models.Photo
	if err := attributevalue.UnmarshalMap(out.Item, &photo); err != nil {
		return nil, err
	}
	return &photo, nil
}

// DeletePhoto removes a photo record from DynamoDB.
func DeletePhoto(ctx context.Context, photoID string) error {
	_, err := db.DeleteItem(ctx, &dynamodb.DeleteItemInput{
		TableName: &photosTable,
		Key: map[string]types.AttributeValue{
			"photo_id": &types.AttributeValueMemberS{Value: photoID},
		},
	})
	return err
}

// UpdatePhotoStatus updates a photo's status and optionally sets the URL.
// The worker calls this after processing: status=completed + S3 URL,
// or status=failed with no URL.
func UpdatePhotoStatus(ctx context.Context, photoID, status, url string) error {
	update := expression.Set(expression.Name("status"), expression.Value(status))
	if url != "" {
		update = update.Set(expression.Name("url"), expression.Value(url))
	}

	expr, err := expression.NewBuilder().WithUpdate(update).Build()
	if err != nil {
		return err
	}

	_, err = db.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: &photosTable,
		Key: map[string]types.AttributeValue{
			"photo_id": &types.AttributeValueMemberS{Value: photoID},
		},
		UpdateExpression:          expr.Update(),
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
	})
	return err
}
