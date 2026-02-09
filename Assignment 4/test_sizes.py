import requests
import time

BUCKET = "cs6650-mapreduce-66076"
SPLITTER_IP = "35.90.160.114"
MAPPER_IPS = ["54.184.175.227", "16.144.225.40", "54.212.142.164"]
REDUCER_IP = "54.202.52.92"

def test_size(input_file, size_name):
    print(f"\n{'='*60}")
    print(f"Testing: {size_name}")
    print(f"{'='*60}")
    
    total_start = time.time()
    
    # Split
    response = requests.get(f"http://{SPLITTER_IP}:8080/split?bucket={BUCKET}&key={input_file}")
    
    # Map
    mapper_results = []
    for i, mapper_ip in enumerate(MAPPER_IPS):
        chunk_key = f"chunk-{i}.txt"
        output_key = f"mapper-{i}-{size_name}.json"
        requests.get(
            f"http://{mapper_ip}:8080/map?bucket={BUCKET}&key={chunk_key}&output={output_key}"
        )
        mapper_results.append(output_key)
    
    # Reduce
    mapper_params = "&".join([f"mapper={r}" for r in mapper_results])
    response = requests.get(
        f"http://{REDUCER_IP}:8080/reduce?bucket={BUCKET}&{mapper_params}&output=final-{size_name}.json"
    )
    
    total_time = time.time() - total_start
    result = response.json()
    
    print(f"Time: {total_time:.3f}s")
    print(f"Unique words: {result['total_words']}")
    
    return total_time

# Test different sizes
times = {}
times['1x (160KB)'] = 1.528  # We already know this
times['10x (1.6MB)'] = test_size("input_10x.txt", "10x")
times['50x (8MB)'] = test_size("input_50x.txt", "50x")

print(f"\n{'='*60}")
print("SUMMARY:")
print(f"{'='*60}")
for size, t in times.items():
    print(f"{size:20} {t:.3f}s")