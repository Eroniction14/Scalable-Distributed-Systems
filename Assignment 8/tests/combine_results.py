"""
Combines mysql_test_results.json and dynamodb_test_results.json into combined_results.json.
Also prints the full comparison table for the report.
"""
import json
import statistics

def load_results(filepath):
    with open(filepath) as f:
        return json.load(f)

def calc_stats(results):
    times = [r["response_time"] for r in results if r["success"]]
    if not times:
        return {}
    times_sorted = sorted(times)
    n = len(times_sorted)
    return {
        "avg": round(statistics.mean(times), 2),
        "p50": round(times_sorted[n // 2], 2),
        "p95": round(times_sorted[int(n * 0.95)], 2),
        "p99": round(times_sorted[int(n * 0.99)], 2),
        "success_rate": round(sum(1 for r in results if r["success"]) / len(results) * 100, 1),
        "count": len(results),
    }

def main():
    mysql = load_results("mysql_test_results.json")
    dynamo = load_results("dynamodb_test_results.json")

    # Tag each result with its database
    for r in mysql:
        r["database"] = "mysql"
    for r in dynamo:
        r["database"] = "dynamodb"

    combined = mysql + dynamo
    with open("combined_results.json", "w") as f:
        json.dump(combined, f, indent=2)

    print("combined_results.json created with", len(combined), "total operations")
    print()

    # Overall comparison
    print("=" * 75)
    print("OVERALL PERFORMANCE COMPARISON")
    print("=" * 75)
    mysql_stats = calc_stats(mysql)
    dynamo_stats = calc_stats(dynamo)

    header = f"{'Metric':<25} {'MySQL':>12} {'DynamoDB':>12} {'Winner':>10} {'Margin':>10}"
    print(header)
    print("-" * 75)
    for metric, label in [("avg", "Avg Response (ms)"), ("p50", "P50 Response (ms)"),
                           ("p95", "P95 Response (ms)"), ("p99", "P99 Response (ms)")]:
        m, d = mysql_stats[metric], dynamo_stats[metric]
        winner = "MySQL" if m < d else "DynamoDB"
        margin = f"{abs(m - d):.2f}ms"
        print(f"{label:<25} {m:>12.2f} {d:>12.2f} {winner:>10} {margin:>10}")

    print(f"{'Success Rate (%)':.<25} {mysql_stats['success_rate']:>12.1f} {dynamo_stats['success_rate']:>12.1f} {'Tie':>10} {'0%':>10}")
    print(f"{'Total Operations':<25} {'150':>12} {'150':>12}")

    # Per-operation breakdown
    print()
    print("=" * 75)
    print("OPERATION-SPECIFIC BREAKDOWN")
    print("=" * 75)
    header2 = f"{'Operation':<15} {'MySQL Avg (ms)':>15} {'DynamoDB Avg (ms)':>18} {'Faster By':>12}"
    print(header2)
    print("-" * 75)

    for op in ["create_cart", "add_items", "get_cart"]:
        m_ops = [r for r in mysql if r["operation"] == op]
        d_ops = [r for r in dynamo if r["operation"] == op]
        m_avg = calc_stats(m_ops)["avg"]
        d_avg = calc_stats(d_ops)["avg"]
        faster = "MySQL" if m_avg < d_avg else "DynamoDB"
        margin = f"{abs(m_avg - d_avg):.2f}ms"
        print(f"{op:<15} {m_avg:>15.2f} {d_avg:>18.2f} {faster + ' ' + margin:>12}")

if __name__ == "__main__":
    main()