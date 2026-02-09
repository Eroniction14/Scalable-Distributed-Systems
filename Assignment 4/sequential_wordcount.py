import time
import json
from collections import Counter

def sequential_wordcount(filename):
    start = time.time()
    
    # Read file
    with open(filename, 'r', encoding='utf-8') as f:
        content = f.read()
    
    # Clean and count words
    words = content.lower().split()
    word_counts = Counter()
    for word in words:
        cleaned = word.strip('.,!?;:"\'()[]{}')
        if cleaned:
            word_counts[cleaned] += 1
    
    end = time.time()
    elapsed = end - start
    
    return dict(word_counts), elapsed, len(word_counts)

if __name__ == "__main__":
    result, time_taken, unique_words = sequential_wordcount("hamlet.txt")
    
    print(f"Sequential Processing:")
    print(f"Time: {time_taken:.3f} seconds")
    print(f"Unique words: {unique_words}")
    
    # Save result
    with open("sequential-result.json", "w") as f:
        json.dump(result, f)