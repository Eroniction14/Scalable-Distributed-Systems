import time
from collections import Counter

def sequential_wordcount(filename):
    start = time.time()
    
    with open(filename, 'r', encoding='utf-8') as f:
        content = f.read()
    
    words = content.lower().split()
    word_counts = Counter()
    for word in words:
        cleaned = word.strip('.,!?;:"\'()[]{}')
        if cleaned:
            word_counts[cleaned] += 1
    
    elapsed = time.time() - start
    return len(word_counts), elapsed

print("Sequential Processing Times:")
print("="*60)

files = [
    ("hamlet.txt", "1x (160KB)"),
    ("hamlet_10x.txt", "10x (1.6MB)"),
    ("hamlet_50x.txt", "50x (8MB)")
]

for filename, label in files:
    unique, time_taken = sequential_wordcount(filename)
    print(f"{label:20} {time_taken:.3f}s  ({unique} unique words)")