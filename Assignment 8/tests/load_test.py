"""
Performance test for shopping cart API.
Runs 150 operations: 50 create, 50 add items, 50 get cart.
Outputs results to mysql_test_results.json or dynamodb_test_results.json.

Usage:
    python load_test.py <base_url> <output_file>
    python load_test.py http://1.2.3.4:8080 mysql_test_results.json
    python load_test.py http://1.2.3.4:8080 dynamodb_test_results.json
"""

import sys
import json
import time
import requests
from datetime import datetime, timezone

def run_test(base_url: str, output_file: str):
    results = []
    cart_ids = []

    print(f"Testing {base_url} -> {output_file}")
    print("=" * 50)

    # Phase 1: Create 50 carts
    print("\n[Phase 1] Creating 50 carts...")
    for i in range(50):
        start = time.time()
        try:
            resp = requests.post(
                f"{base_url}/shopping-carts",
                json={"customer_id": f"customer-{i}"},
                timeout=10,
            )
            elapsed = (time.time() - start) * 1000  # ms
            success = resp.status_code == 201
            if success:
                cart_ids.append(resp.json()["id"])
            results.append({
                "operation": "create_cart",
                "response_time": round(elapsed, 2),
                "success": success,
                "status_code": resp.status_code,
                "timestamp": datetime.now(timezone.utc).isoformat(),
            })
        except Exception as e:
            elapsed = (time.time() - start) * 1000
            results.append({
                "operation": "create_cart",
                "response_time": round(elapsed, 2),
                "success": False,
                "status_code": 0,
                "timestamp": datetime.now(timezone.utc).isoformat(),
                "error": str(e),
            })

    print(f"  Created {len(cart_ids)} carts")

    # Phase 2: Add items to 50 carts
    print("\n[Phase 2] Adding items to 50 carts...")
    for i in range(50):
        cart_id = cart_ids[i % len(cart_ids)]
        start = time.time()
        try:
            resp = requests.post(
                f"{base_url}/shopping-carts/{cart_id}/items",
                json={
                    "product_id": f"product-{i}",
                    "name": f"Test Product {i}",
                    "quantity": (i % 5) + 1,
                    "price": round(9.99 + i * 0.5, 2),
                },
                timeout=10,
            )
            elapsed = (time.time() - start) * 1000
            results.append({
                "operation": "add_items",
                "response_time": round(elapsed, 2),
                "success": resp.status_code == 201,
                "status_code": resp.status_code,
                "timestamp": datetime.now(timezone.utc).isoformat(),
            })
        except Exception as e:
            elapsed = (time.time() - start) * 1000
            results.append({
                "operation": "add_items",
                "response_time": round(elapsed, 2),
                "success": False,
                "status_code": 0,
                "timestamp": datetime.now(timezone.utc).isoformat(),
                "error": str(e),
            })

    # Phase 3: Get 50 carts
    print("\n[Phase 3] Retrieving 50 carts...")
    for i in range(50):
        cart_id = cart_ids[i % len(cart_ids)]
        start = time.time()
        try:
            resp = requests.get(
                f"{base_url}/shopping-carts/{cart_id}",
                timeout=10,
            )
            elapsed = (time.time() - start) * 1000
            results.append({
                "operation": "get_cart",
                "response_time": round(elapsed, 2),
                "success": resp.status_code == 200,
                "status_code": resp.status_code,
                "timestamp": datetime.now(timezone.utc).isoformat(),
            })
        except Exception as e:
            elapsed = (time.time() - start) * 1000
            results.append({
                "operation": "get_cart",
                "response_time": round(elapsed, 2),
                "success": False,
                "status_code": 0,
                "timestamp": datetime.now(timezone.utc).isoformat(),
                "error": str(e),
            })

    # Save results
    with open(output_file, "w") as f:
        json.dump(results, f, indent=2)

    # Print summary
    print("\n" + "=" * 50)
    print("SUMMARY")
    print("=" * 50)
    for op in ["create_cart", "add_items", "get_cart"]:
        op_results = [r for r in results if r["operation"] == op]
        times = [r["response_time"] for r in op_results if r["success"]]
        successes = sum(1 for r in op_results if r["success"])
        if times:
            avg = sum(times) / len(times)
            times_sorted = sorted(times)
            p50 = times_sorted[len(times_sorted) // 2]
            p95 = times_sorted[int(len(times_sorted) * 0.95)]
            print(f"  {op:15s} | success: {successes}/50 | avg: {avg:.1f}ms | p50: {p50:.1f}ms | p95: {p95:.1f}ms")
        else:
            print(f"  {op:15s} | success: {successes}/50 | NO SUCCESSFUL RESPONSES")

    total_success = sum(1 for r in results if r["success"])
    print(f"\n  Total: {total_success}/150 operations successful")
    print(f"  Results saved to: {output_file}")

if __name__ == "__main__":
    if len(sys.argv) != 3:
        print("Usage: python load_test.py <base_url> <output_file>")
        print("  e.g.: python load_test.py http://1.2.3.4:8080 mysql_test_results.json")
        sys.exit(1)

    run_test(sys.argv[1], sys.argv[2])