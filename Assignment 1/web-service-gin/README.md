# Assignment 1b - AWS EC2 Deployment & Performance Testing

**Student:** Eroniction Presley  
**Course:** CS6650 - Building Scalable Distributed Systems  
**Date:** January 19, 2026  
**Institution:** Northeastern University

## Project Overview

This assignment demonstrates deploying a Go-Gin web service to AWS EC2 and conducting performance analysis through load testing. The service provides a RESTful API for managing a music album collection.

## Repository Structure
```
.
├── Assignment 1/
│   └── web-service-gin/
│       ├── main.go                    # Go-Gin web service
│       ├── go.mod                     # Go module dependencies
│       ├── go.sum                     # Go dependency checksums
│       ├── load_test.py               # Python load testing script
│       ├── load_test_results.png      # Performance test visualization
│       └── screenshots/               # Assignment screenshots
│           ├── ec2-instance.png
│           ├── load-test-stats.png
│           ├── performance-graphs.png
│           └── file-upload.png
└── README.md
```

## Deployment Information

- **Cloud Provider:** AWS EC2
- **Region:** us-east-1 (N. Virginia)
- **Instance Type:** t3.micro (2 vCPUs, 1 GB RAM)
- **Operating System:** Amazon Linux 2023
- **Public IP:** 107.22.147.217
- **Endpoint:** `http://107.22.147.217:8080/albums`

## API Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/albums` | Retrieve all albums |
| GET | `/albums/:id` | Retrieve album by ID |
| POST | `/albums` | Add a new album |

## Local Development

### Prerequisites
- Go 1.21+
- Git

### Running Locally
```bash
# Clone the repository
git clone https://github.com/Eroniction14/Scalable-Distributed-Systems.git
cd Scalable-Distributed-Systems/Assignment\ 1/web-service-gin

# Run the server
go run main.go

# Test locally
curl http://localhost:8080/albums
```

## AWS Deployment Process

### 1. Cross-Compilation
```bash
# Compile for Linux from Windows
GOOS=linux GOARCH=amd64 go build -o go-server main.go
```

### 2. Set Key Permissions
```bash
chmod 400 cs6650-key.pem
```

### 3. Upload to EC2
```bash
scp -i cs6650-key.pem ./go-server ec2-user@107.22.147.217:/home/ec2-user/app/
```

### 4. Run on EC2
```bash
ssh -i cs6650-key.pem ec2-user@107.22.147.217
cd app
chmod +x go-server
./go-server
```

### 5. Security Group Configuration
- Port 22 (SSH): My IP only
- Port 8080 (HTTP): 0.0.0.0/0 (open for testing)

## Performance Testing

### Test Setup
```bash
# Install dependencies
pip install requests matplotlib numpy

# Run load test
python load_test.py
```

### Test Configuration
- **Duration:** 30 seconds
- **Request Type:** Sequential GET requests
- **Target:** `http://107.22.147.217:8080/albums`

---

## Performance Analysis Results

### Key Metrics
- **Total Requests:** 432 (~14.4 requests/second)
- **Average Response Time:** 69.04ms
- **Median Response Time:** 65.99ms
- **Min Response Time:** 48.96ms
- **Max Response Time:** 126.59ms
- **Standard Deviation:** 11.38ms
- **90th Percentile:** 83.71ms
- **95th Percentile:** 90.56ms
- **99th Percentile:** 106.02ms

---

## Detailed Observations

### 1. Distribution Shape - Clear Long Tail Pattern

The histogram clearly demonstrates a **long-tail distribution** typical of web service response times. The majority of requests (approximately 70%) completed between 55-75ms, forming a tight cluster around the median of 65.99ms. However, approximately **5.1% of requests exceeded the 95th percentile threshold of 90.56ms**, with outliers reaching up to 126.59ms. This represents a **2.6x difference between minimum and maximum** response times, indicating moderate variability in service performance.

### 2. Consistency & Temporal Patterns

The scatter plot reveals **relatively consistent performance throughout the 30-second test window** with no obvious degradation over time. Response times remained clustered between 55-85ms for most requests, with occasional spikes to 100-140ms distributed evenly across the timeline. This suggests:
- No significant warm-up period was needed
- No memory leaks or resource exhaustion during the test
- Outliers appear randomly rather than in clusters, indicating transient network/system events

### 3. Percentile Analysis - Moderate Variability

The gap between the **median (65.99ms) and 95th percentile (90.56ms) is 24.57ms**, representing a **37% increase**. While this shows some variability, it's relatively modest for a cloud-based service. The 99th percentile at 106.02ms indicates that even the slowest 1% of requests complete in reasonable time. This performance profile would be acceptable for many production workloads, though the tail latency could impact user experience for latency-sensitive applications.

### 4. Infrastructure Impact - t3.micro Limitations

Running on a basic **t3.micro instance with 1GB RAM and burstable CPU credits** contributes to response time variability:
- **CPU Credit System:** T3 instances use credit-based CPU; sustained load can deplete credits
- **Shared Hardware:** Small instances run on shared physical hardware with potential noisy neighbor effects
- **Network Performance:** T3.micro has "Up to 5 Gigabit" bandwidth, which is shared and variable
- **Single Instance:** No load balancing means all variance impacts user-facing latency

The observed variability (standard deviation of 11.38ms) is likely partially attributable to these infrastructure constraints.

### 5. Scaling Implications - Sequential vs. Concurrent Load

This test used **sequential requests** (one at a time), achieving ~14.4 requests/second. With 100 concurrent users, several challenges would emerge:
- **Connection Overhead:** Managing 100 concurrent TCP connections increases memory/CPU usage
- **Go Runtime:** Goroutines handle concurrency well, but context switching overhead increases
- **CPU Exhaustion:** The t3.micro's limited CPU would become a bottleneck
- **Response Time Degradation:** Average latency would likely increase 3-10x
- **Request Queuing:** Without rate limiting, queuing could cause timeout cascades

**Estimated capacity:** A single t3.micro could likely handle 20-30 concurrent users before significant degradation, requiring horizontal scaling beyond that.

### 6. Network vs. Processing Time Analysis

The observed latency of 65-70ms (median/average) can be decomposed into:

**Network Latency Components:**
- Round-trip time (RTT) to us-east-1: Estimated 10-30ms (major component)
- DNS resolution: Negligible (cached after first request)
- TCP handshake: ~1 RTT

**Server Processing Time:**
- Go-Gin framework overhead: <1ms
- JSON serialization: <1ms
- Business logic: <1ms

**Investigation Methods:**
1. **SSH and test locally:** `curl localhost:8080/albums` would show pure processing time (~5ms)
2. **Server-side logging:** Add timestamps to measure request handling duration
3. **Network diagnostics:** `ping 107.22.147.217` to measure RTT
4. **Multi-region comparison:** Deploy to different AWS regions and compare latency

**Conclusion:** Given that simple JSON responses take <5ms in Go, **network latency accounts for approximately 85-95%** of total response time (55-65ms out of 70ms). Tail latency spikes (100-140ms) are likely network-related or caused by occasional CPU throttling.

---

## Key Takeaways

1. ✅ **Acceptable baseline performance** for a simple API on minimal infrastructure
2. ⚠️ **Tail latency exists** but is manageable at 5.1% of requests
3. 🌐 **Network latency dominates** total response time
4. 📈 **Horizontal scaling required** for production workloads beyond 20-30 concurrent users
5. 💰 **Infrastructure matters** - larger instance types would reduce variability

---

## Technologies Used

- **Backend:** Go 1.21, Gin Web Framework
- **Cloud:** AWS EC2, Amazon Linux 2023
- **Testing:** Python, requests, matplotlib, numpy
- **Version Control:** Git, GitHub

---

## Screenshots

See `screenshots/` folder for:
1. AWS EC2 Console (running instance)
2. Load test terminal output
3. Performance graphs (histogram + scatter)
4. File upload and compilation process

---

## Author

**Eroniction Presley**  
Northeastern University - Khoury College of Computer Sciences  
Master's in Computer Science  
CS6650 - Building Scalable Distributed Systems

---

## Acknowledgments

- Professor Mark Coady
- CS6650 Teaching Staff
- AWS Academy Learner Lab