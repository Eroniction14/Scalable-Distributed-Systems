package main

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"github.com/google/uuid"
)

// DynamoStore implements CartStore using Amazon DynamoDB.
//
// Table design rationale (single-table):
//   - Partition key: cart ID (UUID) — ensures even distribution across partitions.
//   - No sort key needed — each cart is a single item with embedded items array.
//   - Single-table design: cart metadata + items stored in one DynamoDB item.
//     This avoids the need for Query operations or secondary indexes,
//     and gives us single-digit-ms GetItem performance.
//   - Items are embedded as a JSON list attribute rather than separate DynamoDB items.
//     Trade-off: can't query individual items, but for shopping carts you always
//     load the full cart anyway. Max 50 items per cart fits well within
//     DynamoDB's 400KB item size limit.
//   - GSI on customer_id supports "get all carts by customer" access pattern.
//
// Consistency:
//   - Writes use PutItem (eventually consistent by default).
//   - Reads use GetItem with ConsistentRead=true for strong consistency,
//     ensuring read-after-write correctness for cart operations.
type DynamoStore struct {
	client    *dynamodb.DynamoDB
	tableName string
}

// dynamoCart is the DynamoDB item representation
type dynamoCart struct {
	ID         string `dynamodbav:"id"`
	CustomerID string `dynamodbav:"customer_id"`
	Status     string `dynamodbav:"status"`
	CreatedAt  string `dynamodbav:"created_at"`
	UpdatedAt  string `dynamodbav:"updated_at"`
	Items      string `dynamodbav:"items"` // JSON-encoded []CartItem
}

func NewDynamoStore(region, tableName string) (*DynamoStore, error) {
	sess, err := session.NewSession(&aws.Config{
		Region: aws.String(region),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create AWS session: %w", err)
	}

	client := dynamodb.New(sess)
	store := &DynamoStore{client: client, tableName: tableName}

	// Verify table exists by describing it
	_, err = client.DescribeTable(&dynamodb.DescribeTableInput{
		TableName: aws.String(tableName),
	})
	if err != nil {
		return nil, fmt.Errorf("DynamoDB table %s not accessible: %w", tableName, err)
	}

	log.Printf("DynamoDB store initialized (table: %s, region: %s)", tableName, region)
	return store, nil
}

func (s *DynamoStore) CreateCart(customerID string) (*Cart, error) {
	id := uuid.New().String()
	now := time.Now().UTC()

	itemsJSON, _ := json.Marshal([]CartItem{})

	dc := dynamoCart{
		ID:         id,
		CustomerID: customerID,
		Status:     "active",
		CreatedAt:  now.Format(time.RFC3339),
		UpdatedAt:  now.Format(time.RFC3339),
		Items:      string(itemsJSON),
	}

	av, err := dynamodbattribute.MarshalMap(dc)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal cart: %w", err)
	}

	_, err = s.client.PutItem(&dynamodb.PutItemInput{
		TableName: aws.String(s.tableName),
		Item:      av,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to put cart: %w", err)
	}

	return &Cart{
		ID:         id,
		CustomerID: customerID,
		Status:     "active",
		CreatedAt:  now,
		UpdatedAt:  now,
		Items:      []CartItem{},
	}, nil
}

func (s *DynamoStore) GetCart(id string) (*Cart, error) {
	result, err := s.client.GetItem(&dynamodb.GetItemInput{
		TableName:      aws.String(s.tableName),
		ConsistentRead: aws.Bool(true),
		Key: map[string]*dynamodb.AttributeValue{
			"id": {S: aws.String(id)},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get cart: %w", err)
	}
	if result.Item == nil {
		return nil, fmt.Errorf("cart not found")
	}

	var dc dynamoCart
	if err := dynamodbattribute.UnmarshalMap(result.Item, &dc); err != nil {
		return nil, fmt.Errorf("failed to unmarshal cart: %w", err)
	}

	return s.dynamoCartToCart(&dc)
}

func (s *DynamoStore) AddItem(cartID string, item CartItem) (*Cart, error) {
	// Get existing cart
	cart, err := s.GetCart(cartID)
	if err != nil {
		return nil, err
	}

	// Upsert logic: update quantity if product exists, otherwise append
	found := false
	for i, existing := range cart.Items {
		if existing.ProductID == item.ProductID {
			cart.Items[i].Quantity = item.Quantity
			cart.Items[i].Name = item.Name
			cart.Items[i].Price = item.Price
			found = true
			break
		}
	}
	if !found {
		cart.Items = append(cart.Items, item)
	}

	now := time.Now().UTC()
	itemsJSON, _ := json.Marshal(cart.Items)

	dc := dynamoCart{
		ID:         cart.ID,
		CustomerID: cart.CustomerID,
		Status:     cart.Status,
		CreatedAt:  cart.CreatedAt.Format(time.RFC3339),
		UpdatedAt:  now.Format(time.RFC3339),
		Items:      string(itemsJSON),
	}

	av, err := dynamodbattribute.MarshalMap(dc)
	if err != nil {
		return nil, err
	}

	_, err = s.client.PutItem(&dynamodb.PutItemInput{
		TableName: aws.String(s.tableName),
		Item:      av,
	})
	if err != nil {
		return nil, err
	}

	cart.UpdatedAt = now
	return cart, nil
}

func (s *DynamoStore) Close() error {
	// DynamoDB SDK doesn't need explicit close
	return nil
}

// dynamoCartToCart converts the DynamoDB representation to the API model
func (s *DynamoStore) dynamoCartToCart(dc *dynamoCart) (*Cart, error) {
	createdAt, _ := time.Parse(time.RFC3339, dc.CreatedAt)
	updatedAt, _ := time.Parse(time.RFC3339, dc.UpdatedAt)

	var items []CartItem
	if err := json.Unmarshal([]byte(dc.Items), &items); err != nil {
		items = []CartItem{}
	}

	return &Cart{
		ID:         dc.ID,
		CustomerID: dc.CustomerID,
		Status:     dc.Status,
		CreatedAt:  createdAt,
		UpdatedAt:  updatedAt,
		Items:      items,
	}, nil
}
