#!/bin/bash
# run_load_tests.sh
# Runs load tests for all 4 database configs × 4 read-write ratios = 16 runs
#
# Prerequisites:
#   1. Build the load tester: cd loadtest && go build -o loadtest .
#   2. Start the appropriate cluster before each section

set -e

REQUESTS=2000
CONCURRENCY=20
KEYS=20
OUT_DIR="results"
mkdir -p "$OUT_DIR"

WRITE_PCTS=(0.01 0.10 0.50 0.90)

echo "=============================================="
echo "  DISTRIBUTED KV STORE LOAD TEST SUITE"
echo "=============================================="

# ── Helper ──
run_tests() {
    local MODE=$1
    local QUORUM=$2

    for WP in "${WRITE_PCTS[@]}"; do
        WP_LABEL=$(printf "%.0f" "$(echo "$WP * 100" | bc)")
        echo ""
        echo "► Running: mode=$MODE quorum=$QUORUM write_pct=${WP_LABEL}%"
        ./loadtest/loadtest \
            -mode "$MODE" \
            -quorum "$QUORUM" \
            -write-pct "$WP" \
            -requests "$REQUESTS" \
            -concurrency "$CONCURRENCY" \
            -keys "$KEYS" \
            -out "$OUT_DIR"
        echo "  ✓ Done"
    done
}

# ══════════════════════════════════════════════════
#  LEADER-FOLLOWER TESTS
# ══════════════════════════════════════════════════
echo ""
echo "=== LEADER-FOLLOWER: W=5, R=1 ==="
echo "Make sure cluster is running: W=5 R=1 docker compose -f docker-compose-leader.yml up --build"
read -p "Press Enter when ready..."
run_tests "leader" "W5R1"

echo ""
echo "=== LEADER-FOLLOWER: W=1, R=5 ==="
echo "Restart cluster: W=1 R=5 docker compose -f docker-compose-leader.yml up --build"
read -p "Press Enter when ready..."
run_tests "leader" "W1R5"

echo ""
echo "=== LEADER-FOLLOWER: W=3, R=3 ==="
echo "Restart cluster: W=3 R=3 docker compose -f docker-compose-leader.yml up --build"
read -p "Press Enter when ready..."
run_tests "leader" "W3R3"

# ══════════════════════════════════════════════════
#  LEADERLESS TESTS
# ══════════════════════════════════════════════════
echo ""
echo "=== LEADERLESS: W=N, R=1 ==="
echo "Start leaderless cluster: docker compose -f docker-compose-leaderless.yml up --build"
read -p "Press Enter when ready..."
run_tests "leaderless" "WNR1"

# ══════════════════════════════════════════════════
echo ""
echo "=============================================="
echo "  ALL TESTS COMPLETE"
echo "  Results in: $OUT_DIR/"
echo "=============================================="
ls -la "$OUT_DIR"/*.csv