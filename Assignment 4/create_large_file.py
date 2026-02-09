# Create larger test files
with open('hamlet.txt', 'r', encoding='utf-8') as f:
    hamlet = f.read()

# 10x Hamlet (~1.6MB)
with open('hamlet_10x.txt', 'w', encoding='utf-8') as f:
    f.write(hamlet * 10)

# 50x Hamlet (~8MB)
with open('hamlet_50x.txt', 'w', encoding='utf-8') as f:
    f.write(hamlet * 50)

print("Created:")
print("  hamlet_10x.txt (~1.6MB)")
print("  hamlet_50x.txt (~8MB)")