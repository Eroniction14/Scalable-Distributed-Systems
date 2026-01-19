import requests
import time
import matplotlib.pyplot as plt
import numpy as np
from datetime import datetime

def load_test(url, duration_seconds=30):
    response_times = []
    start_time = time.time()
    end_time = start_time + duration_seconds
    
    print(f"Starting load test for {duration_seconds} seconds...")
    print(f"Target URL: {url}")
    print("-" * 50)
    
    request_count = 0
    while time.time() < end_time:
        try:
            start_request = time.time()
            response = requests.get(url, timeout=10)
            end_request = time.time()
            
            response_time = (end_request - start_request) * 1000  # Convert to milliseconds
            response_times.append(response_time)
            request_count += 1
            
            if response.status_code == 200:
                print(f"Request {request_count}: {response_time:.2f}ms - SUCCESS")
            else:
                print(f"Request {request_count}: {response_time:.2f}ms - FAILED (Status: {response.status_code})")
                
        except requests.exceptions.RequestException as e:
            print(f"Request {request_count + 1}: FAILED - {e}")
            
    print("-" * 50)
    print(f"Load test completed!")
    return response_times

# Replace with your EC2 public IP
EC2_URL = "http://107.22.147.217:8080/albums"

# Run the test
response_times = load_test(EC2_URL)

# Plot the results
plt.figure(figsize=(12, 8))

# Histogram
plt.subplot(2, 1, 1)
plt.hist(response_times, bins=50, alpha=0.7, color='blue', edgecolor='black')
plt.xlabel('Response Time (ms)', fontsize=12)
plt.ylabel('Frequency', fontsize=12)
plt.title('Distribution of Response Times', fontsize=14, fontweight='bold')
plt.grid(axis='y', alpha=0.3)

# Scatter plot over time
plt.subplot(2, 1, 2)
plt.scatter(range(len(response_times)), response_times, alpha=0.6, color='green')
plt.xlabel('Request Number', fontsize=12)
plt.ylabel('Response Time (ms)', fontsize=12)
plt.title('Response Times Over Time', fontsize=14, fontweight='bold')
plt.grid(axis='y', alpha=0.3)

plt.tight_layout()
plt.savefig('load_test_results.png', dpi=300, bbox_inches='tight')
print("\nPlot saved as 'load_test_results.png'")

# Print statistics BEFORE showing the plot
print(f"\n{'='*50}")
print(f"PERFORMANCE STATISTICS")
print(f"{'='*50}")
print(f"Total requests: {len(response_times)}")
print(f"Average response time: {np.mean(response_times):.2f}ms")
print(f"Median response time: {np.median(response_times):.2f}ms")
print(f"Min response time: {min(response_times):.2f}ms")
print(f"Max response time: {max(response_times):.2f}ms")
print(f"Standard deviation: {np.std(response_times):.2f}ms")
print(f"\nPercentiles:")
print(f"  50th percentile (median): {np.percentile(response_times, 50):.2f}ms")
print(f"  90th percentile: {np.percentile(response_times, 90):.2f}ms")
print(f"  95th percentile: {np.percentile(response_times, 95):.2f}ms")
print(f"  99th percentile: {np.percentile(response_times, 99):.2f}ms")
print(f"{'='*50}")

# Calculate tail latency
tail_threshold = np.percentile(response_times, 95)
tail_requests = [rt for rt in response_times if rt > tail_threshold]
print(f"\nTail Latency Analysis (>95th percentile):")
print(f"  Number of slow requests: {len(tail_requests)} ({len(tail_requests)/len(response_times)*100:.1f}%)")
print(f"  Threshold: {tail_threshold:.2f}ms")

# Show plot at the very end
plt.show()