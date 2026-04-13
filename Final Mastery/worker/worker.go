package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"album-store/store"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
)

var (
	s3Client  *s3.Client
	sqsClient *sqs.Client
	bucket    string
	queueURL  string
)

type PhotoMessage struct {
	PhotoID string `json:"photo_id"`
	AlbumID string `json:"album_id"`
	S3Key   string `json:"s3_key"`
}

func Start(s3Bucket, sqsQueueURL string) {
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		log.Fatalf("worker: unable to load AWS config: %v", err)
	}

	s3Client = s3.NewFromConfig(cfg)
	sqsClient = sqs.NewFromConfig(cfg)
	bucket = s3Bucket
	queueURL = sqsQueueURL

	log.Println("worker: started polling SQS")

	for {
		pollAndProcess()
	}
}

func pollAndProcess() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	out, err := sqsClient.ReceiveMessage(ctx, &sqs.ReceiveMessageInput{
		QueueUrl:            &queueURL,
		MaxNumberOfMessages: 10,
		WaitTimeSeconds:     5,
	})
	if err != nil {
		log.Printf("worker: SQS receive error: %v", err)
		time.Sleep(2 * time.Second)
		return
	}

	for _, msg := range out.Messages {
		var pm PhotoMessage
		if err := json.Unmarshal([]byte(*msg.Body), &pm); err != nil {
			log.Printf("worker: bad message body: %v", err)
			deleteMessage(msg.ReceiptHandle)
			continue
		}

		if err := processPhoto(pm); err != nil {
			log.Printf("worker: failed to process photo %s: %v", pm.PhotoID, err)
			continue
		}

		deleteMessage(msg.ReceiptHandle)
	}
}

func processPhoto(pm PhotoMessage) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Check if photo was deleted while in the queue
	existing, err := store.GetPhoto(ctx, pm.PhotoID)
	if err != nil {
		return fmt.Errorf("check photo exists: %w", err)
	}
	if existing == nil {
		// Photo was deleted — clean up S3 and skip
		s3Client.DeleteObject(ctx, &s3.DeleteObjectInput{
			Bucket: &bucket,
			Key:    &pm.S3Key,
		})
		log.Printf("worker: photo %s was deleted, skipping", pm.PhotoID)
		return nil
	}

	// Photo already in final S3 path — just update status
	url := fmt.Sprintf("https://%s.s3.amazonaws.com/%s", bucket, pm.S3Key)
	if err := store.UpdatePhotoStatus(ctx, pm.PhotoID, "completed", url); err != nil {
		return fmt.Errorf("update status: %w", err)
	}

	log.Printf("worker: completed photo %s (album %s)", pm.PhotoID, pm.AlbumID)
	return nil
}

func deleteMessage(receiptHandle *string) {
	_, err := sqsClient.DeleteMessage(context.TODO(), &sqs.DeleteMessageInput{
		QueueUrl:      &queueURL,
		ReceiptHandle: receiptHandle,
	})
	if err != nil {
		log.Printf("worker: failed to delete SQS message: %v", err)
	}
}
