import requests
import time
import json

BUCKET = "cs6650-mapreduce-66076"
SPLITTER_IP = "35.90.160.114"
MAPPER_IPS = ["54.184.175.227", "16.144.225.40", "54.212.142.164"]
REDUCER_IP = "54.202.52.92"

def measure_distributed():
    total_start = time.time()
    
    # Step 1: Split
    print("Step 1: Splitting...")
    split_start = time.time()
    response = requests.get(f"http://{SPLITTER_IP}:8080/split?bucket={BUCKET}&key=input.txt")
    chunks = response.json()['chunks']
    split_time = time.time() - split_start
    print(f"  Split time: {split_time:.3f}s")
    
    # Step 2: Map (parallel)
    print("\nStep 2: Mapping (3 mappers in parallel)...")
    map_start = time.time()
    
    mapper_results = []
    for i, mapper_ip in enumerate(MAPPER_IPS):
        chunk_key = f"chunk-{i}.txt"
        output_key = f"mapper-{i}-result-timed.json"
        response = requests.get(
            f"http://{mapper_ip}:8080/map?bucket={BUCKET}&key={chunk_key}&output={output_key}"
        )
        mapper_results.append(output_key)
        print(f"  Mapper {i+1} completed")
    
    map_time = time.time() - map_start
    print(f"  Map time: {map_time:.3f}s")
    
    # Step 3: Reduce
    print("\nStep 3: Reducing...")
    reduce_start = time.time()
    
    mapper_params = "&".join([f"mapper={r}" for r in mapper_results])
    response = requests.get(
        f"http://{REDUCER_IP}:8080/reduce?bucket={BUCKET}&{mapper_params}&output=final-result-timed.json"
    )
    result = response.json()
    
    reduce_time = time.time() - reduce_start
    print(f"  Reduce time: {reduce_time:.3f}s")
    
    total_time = time.time() - total_start
    
    print(f"\n{'='*50}")
    print(f"DISTRIBUTED MAPREDUCE RESULTS:")
    print(f"{'='*50}")
    print(f"Split time:    {split_time:.3f}s")
    print(f"Map time:      {map_time:.3f}s")
    print(f"Reduce time:   {reduce_time:.3f}s")
    print(f"Total time:    {total_time:.3f}s")
    print(f"Unique words:  {result['total_words']}")
    
    return {
        'split_time': split_time,
        'map_time': map_time,
        'reduce_time': reduce_time,
        'total_time': total_time,
        'unique_words': result['total_words']
    }

if __name__ == "__main__":
    results = measure_distributed()
    
    # Save results
    with open("distributed_timing.json", "w") as f:
        json.dump(results, f, indent=2)