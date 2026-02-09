import matplotlib.pyplot as plt
import numpy as np

# Data
file_sizes = ['1x\n(160KB)', '10x\n(1.6MB)', '50x\n(8MB)']
sequential_times = [0.017, 0.076, 0.402]
distributed_times = [1.528, 2.089, 2.673]

# Graph 1: Time Comparison
fig, (ax1, ax2) = plt.subplots(1, 2, figsize=(14, 5))

x = np.arange(len(file_sizes))
width = 0.35

bars1 = ax1.bar(x - width/2, sequential_times, width, label='Sequential', color='#2ecc71')
bars2 = ax1.bar(x + width/2, distributed_times, width, label='Distributed (3 mappers)', color='#3498db')

ax1.set_xlabel('File Size', fontsize=12)
ax1.set_ylabel('Time (seconds)', fontsize=12)
ax1.set_title('Sequential vs Distributed MapReduce Performance', fontsize=14, fontweight='bold')
ax1.set_xticks(x)
ax1.set_xticklabels(file_sizes)
ax1.legend()
ax1.grid(axis='y', alpha=0.3)

# Add value labels on bars
for bars in [bars1, bars2]:
    for bar in bars:
        height = bar.get_height()
        ax1.text(bar.get_x() + bar.get_width()/2., height,
                f'{height:.3f}s',
                ha='center', va='bottom', fontsize=9)

# Graph 2: Overhead Analysis
overhead = [d - s for d, s in zip(distributed_times, sequential_times)]
computation = sequential_times

ax2.bar(x, computation, width*2, label='Actual Computation', color='#2ecc71')
ax2.bar(x, overhead, width*2, bottom=computation, label='Network/S3 Overhead', color='#e74c3c')

ax2.set_xlabel('File Size', fontsize=12)
ax2.set_ylabel('Time (seconds)', fontsize=12)
ax2.set_title('Distributed Processing: Computation vs Overhead', fontsize=14, fontweight='bold')
ax2.set_xticks(x)
ax2.set_xticklabels(file_sizes)
ax2.legend()
ax2.grid(axis='y', alpha=0.3)

plt.tight_layout()
plt.savefig('mapreduce_performance.png', dpi=300, bbox_inches='tight')
print("✅ Saved: mapreduce_performance.png")

# Graph 3: Breakdown of Distributed Time
fig, ax = plt.subplots(figsize=(10, 6))

stages = ['Split', 'Map', 'Reduce']
times = [0.450, 0.763, 0.314]
colors = ['#3498db', '#e67e22', '#9b59b6']

bars = ax.bar(stages, times, color=colors, edgecolor='black', linewidth=1.5)
ax.set_ylabel('Time (seconds)', fontsize=12)
ax.set_title('Distributed MapReduce: Time Breakdown by Stage\n(Hamlet 1x, 3 mappers)', fontsize=14, fontweight='bold')
ax.grid(axis='y', alpha=0.3)

# Add value labels
for bar in bars:
    height = bar.get_height()
    ax.text(bar.get_x() + bar.get_width()/2., height,
            f'{height:.3f}s\n({height/sum(times)*100:.1f}%)',
            ha='center', va='bottom', fontsize=11, fontweight='bold')

plt.tight_layout()
plt.savefig('mapreduce_stages.png', dpi=300, bbox_inches='tight')
print("✅ Saved: mapreduce_stages.png")

# Summary Table
print("\n" + "="*70)
print("PERFORMANCE SUMMARY")
print("="*70)
print(f"{'File Size':<15} {'Sequential':<15} {'Distributed':<15} {'Overhead':<15}")
print("-"*70)
for i, size in enumerate(['160KB', '1.6MB', '8MB']):
    seq = sequential_times[i]
    dist = distributed_times[i]
    ovr = overhead[i]
    print(f"{size:<15} {seq:.3f}s{'':<9} {dist:.3f}s{'':<9} {ovr:.3f}s ({ovr/dist*100:.1f}%)")

print("\n" + "="*70)
print("KEY INSIGHTS:")
print("="*70)
print("1. Network/S3 overhead dominates for small files (98-99% overhead)")
print("2. Distributed processing has ~1.5s baseline overhead")
print("3. For 160KB file: Sequential is 90x faster!")
print("4. For 8MB file: Sequential is still 6.6x faster")
print("5. MapReduce would win with MUCH larger files (100MB+) or complex computations")
print("="*70)