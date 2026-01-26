# Assignment 2 - Infrastructure as Code, Docker, and Distributed Systems

## Overview
This assignment demonstrates:
- Infrastructure automation with Terraform
- Application containerization with Docker
- Distributed systems state management challenges

## Repository Structure
```
Assignment 2/
├── main.tf                 # Terraform infrastructure configuration
├── terraform.tfvars        # Terraform variables (IP and key name)
├── test_script.py          # Python script for Part IV
├── screenshots/            # All submission screenshots
│   ├── part2_terraform_apply.png
│   ├── part2_aws_console.png
│   ├── part2_terraform_destroy.png
│   ├── part3_docker_local.png
│   ├── part3_docker_local_test.png
│   └── part4_two_instances.png
└── README.md              # This file
```

## Part II - Terraform
**Files:** `main.tf`, `terraform.tfvars`

Automated EC2 instance creation using Infrastructure as Code. The configuration creates:
- 2 EC2 t3.micro instances
- Security group allowing SSH (from my IP) and HTTP (port 8080)
- Outputs public DNS and IP addresses

**Key Learning:** Infrastructure reproducibility and version control

## Part III - Docker
**Dockerfile location:** `../Assignment 1/web-service-gin/Dockerfile`
**Docker Hub:** `zlayero/albums-api:latest`

Containerized the Go web service and deployed to EC2. Benefits:
- No need to install Go on EC2
- Consistent environment across machines
- Easy deployment and scaling

**Key Learning:** Application portability through containerization

## Part IV - Distributed Systems Problem
**Script:** `test_script.py`

### What Happened?
The script demonstrates **state inconsistency** in distributed systems:

1. **Initial State:** Both EC2 instances have 3 albums
2. **Action:** POST new album (Betty Carter) to Instance 2 only
3. **Result:** 
   - Instance 2 now has 4 albums ✅
   - Instance 1 still has 3 albums ❌

### Why?
Each EC2 instance runs its own Docker container with **independent in-memory storage**. There is:
- No shared database
- No communication between instances
- No state synchronization

### Real-World Solution
Production distributed systems solve this with:
- **Shared databases** (PostgreSQL, MongoDB)
- **Distributed caches** (Redis, Memcached)
- **Message queues** (Kafka, RabbitMQ)
- **Service meshes** for communication

**Key Learning:** Distributed systems require shared state management to maintain consistency

## Technologies Used
- **Terraform** - Infrastructure as Code
- **Docker** - Containerization
- **AWS EC2** - Cloud compute instances
- **Go + Gin** - Web service framework
- **Python + Requests** - Testing script

## Submission
All files and screenshots submitted to Canvas on January 26, 2026.