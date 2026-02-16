# CS6650 Assignment 5 — Product API with Terraform Deployment

## Overview
A simple Product API built in Go, containerized with Docker, and deployed to AWS ECS/Fargate using Terraform. The API implements the Product endpoints from the provided OpenAPI specification, supporting product creation and retrieval with in-memory storage.

## Project Structure
```
CS6650_2b_demo/
├── src/
│   ├── main.go          # Go Product API server
│   ├── go.mod           # Go module file
│   └── Dockerfile       # Docker build configuration
├── terraform/
│   ├── main.tf          # Terraform resource wiring
│   ├── provider.tf      # AWS & Docker provider config
│   ├── variables.tf     # Configurable variables
│   ├── outputs.tf       # Output values (cluster/service names)
│   └── modules/
│       ├── ecr/         # ECR repository module
│       ├── ecs/         # ECS cluster, task, service module
│       ├── logging/     # CloudWatch log group module
│       └── network/     # VPC, subnets, security group module
├── locustfile.py        # Load testing script
└── README.md
```

## API Endpoints

### GET /products/{productId}
Retrieve a product by its unique ID.

**Responses:**
- `200 OK` — Product found
- `400 Bad Request` — Invalid product ID
- `404 Not Found` — Product does not exist

### POST /products/{productId}/details
Add or update product details.

**Request Body (JSON):**
```json
{
  "product_id": 1,
  "sku": "ABC-123",
  "manufacturer": "Acme Corp",
  "category_id": 5,
  "weight": 1250,
  "some_other_id": 789
}
```

**Responses:**
- `204 No Content` — Product created/updated successfully
- `400 Bad Request` — Invalid input data
- `404 Not Found` — Product not found

### GET /health
Health check endpoint. Returns `200 OK`.

## API Response Examples

```bash
# 204 — Create product
curl -i -X POST http://<SERVER_IP>:8080/products/1/details \
  -H "Content-Type: application/json" \
  -d '{"product_id":1,"sku":"ABC-123","manufacturer":"Acme Corp","category_id":5,"weight":1250,"some_other_id":789}'

# 200 — Get product
curl -i http://<SERVER_IP>:8080/products/1

# 404 — Product not found
curl -i http://<SERVER_IP>:8080/products/999

# 400 — Invalid input (missing required fields)
curl -i -X POST http://<SERVER_IP>:8080/products/2/details \
  -H "Content-Type: application/json" \
  -d '{"sku":"","manufacturer":"","category_id":0,"weight":-1,"some_other_id":0}'

# 400 — Invalid product ID
curl -i http://<SERVER_IP>:8080/products/abc
```

## Deployment Instructions

### Prerequisites
- AWS CLI installed and configured with valid credentials
- Terraform installed
- Docker Desktop installed and running
- Go 1.21+ (for local development)

### Step 1: Configure AWS Credentials
```bash
aws configure
# Enter: Access Key ID, Secret Access Key
# Region: us-west-2
# Output: json

aws configure set aws_session_token <YOUR_SESSION_TOKEN>
aws sts get-caller-identity  # Verify
```

### Step 2: Deploy with Terraform
```bash
cd terraform
terraform init
terraform plan     # Preview changes
terraform apply    # Deploy (type 'yes' to confirm)
```

### Step 3: Get the Server Public IP
```bash
# Get task ARN
aws ecs list-tasks --cluster CS6650L2-cluster --service-name CS6650L2 --region us-west-2

# Get network interface ID (replace TASK_ID)
aws ecs describe-tasks --cluster CS6650L2-cluster --tasks <TASK_ID> --region us-west-2 \
  --query "tasks[0].attachments[0].details" --output table

# Get public IP (replace ENI_ID)
aws ec2 describe-network-interfaces --network-interface-ids <ENI_ID> --region us-west-2 \
  --query "NetworkInterfaces[0].Association.PublicIp" --output text
```

### Step 4: Test the API
```bash
curl -i http://<PUBLIC_IP>:8080/health
```

### Tear Down
```bash
cd terraform
terraform destroy  # Type 'yes' to confirm
```

## Load Testing

Load tests were conducted using Locust against the deployed AWS ECS instance.

### Test Configuration
- **GET:POST ratio** — 3:1 (simulating real-world read-heavy e-commerce traffic)
- **Wait time** — 1-3 seconds between requests
- **Tests run** — HttpUser and FastHttpUser at 50 and 200 concurrent users

### Results Summary

| Test | RPS | p50 (ms) | p95 (ms) | Failures |
|------|-----|----------|----------|----------|
| HttpUser — 50 users | ~25 | ~90 | ~110 | 0% |
| HttpUser — 200 users | ~95 | ~90 | ~120 | 0% |
| FastHttpUser — 50 users | ~25 | ~90 | ~110 | 0% |
| FastHttpUser — 200 users | ~95 | ~90 | ~120 | 0% |

### Analysis

**HttpUser vs FastHttpUser:** Both produced nearly identical results. This is because the bottleneck is the network round-trip latency to AWS (~90ms), not the client-side HTTP library. FastHttpUser uses `geventhttpclient` which is faster than Python's `requests` library used by HttpUser, but this advantage only matters when server response times are <5ms (e.g., localhost) and the client needs to generate thousands of RPS.

**Scaling behavior:** The server scaled linearly from ~25 RPS (50 users) to ~95 RPS (200 users) with stable response times and 0% failures, demonstrating Go's efficient concurrency handling via goroutines.

**Data structure tradeoff:** A hashmap (Go's `sync.Map` / `map` with `sync.RWMutex`) was chosen for O(1) lookups and insertions. In a real e-commerce system, GET requests dominate (customers browsing products), making fast reads the priority. The `RWMutex` allows concurrent reads while serializing writes.

## Design Questions

### Scalable Backend Design for the Full API
To handle the full e-commerce API (Products, Cart, Warehouse, Payments), a microservices architecture would be appropriate — each service independently deployed, scaled, and managed with its own data store. An API gateway would route requests, and services would communicate via message queues (e.g., SQS) for operations like checkout that span multiple services.

### Terraform: Declarative vs Imperative
Terraform is declarative — you define the *desired end state* of your infrastructure, and Terraform figures out the steps to get there. An imperative approach (like a shell script) requires you to specify *each step* in order. Declarative is beneficial because Terraform can compute the minimal set of changes needed, handles dependencies automatically, and makes infrastructure reproducible and version-controlled.