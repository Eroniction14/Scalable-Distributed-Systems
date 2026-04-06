package tests

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"testing"
	"time"
)

// ─── Helpers ──────────────────────────────────────────────────────────────────

type KVResponse struct {
	Value   string `json:"value"`
	Version int    `json:"version"`
	Status  string `json:"status"`
	Key     string `json:"key"`
}

func setKey(t *testing.T, addr, key, value string) KVResponse {
	t.Helper()
	body, _ := json.Marshal(map[string]string{"key": key, "value": value})
	resp, err := http.Post(
		fmt.Sprintf("http://%s/set", addr), "application/json", bytes.NewReader(body),
	)
	if err != nil {
		t.Fatalf("SET failed to %s: %v", addr, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("SET returned %d: %s", resp.StatusCode, string(b))
	}
	var result KVResponse
	json.NewDecoder(resp.Body).Decode(&result)
	return result
}

func getKey(t *testing.T, addr, key string) (KVResponse, int) {
	t.Helper()
	resp, err := http.Get(fmt.Sprintf("http://%s/get?key=%s", addr, key))
	if err != nil {
		t.Fatalf("GET failed from %s: %v", addr, err)
	}
	defer resp.Body.Close()
	var result KVResponse
	if resp.StatusCode == http.StatusOK {
		json.NewDecoder(resp.Body).Decode(&result)
	}
	return result, resp.StatusCode
}

func localRead(t *testing.T, addr, key string) (KVResponse, int) {
	t.Helper()
	resp, err := http.Get(fmt.Sprintf("http://%s/local_read?key=%s", addr, key))
	if err != nil {
		t.Fatalf("LOCAL_READ failed from %s: %v", addr, err)
	}
	defer resp.Body.Close()
	var result KVResponse
	if resp.StatusCode == http.StatusOK {
		json.NewDecoder(resp.Body).Decode(&result)
	}
	return result, resp.StatusCode
}

// ─── Leader-Follower Addresses ────────────────────────────────────────────────
// Assumes docker-compose-leader.yml is running with default port mappings

const (
	leaderAddr    = "localhost:8080"
	follower1Addr = "localhost:8081"
	follower2Addr = "localhost:8082"
	follower3Addr = "localhost:8083"
	follower4Addr = "localhost:8084"
)

var followerAddrs = []string{follower1Addr, follower2Addr, follower3Addr, follower4Addr}

// ─── Leaderless Addresses ─────────────────────────────────────────────────────
// Assumes docker-compose-leaderless.yml is running

var leaderlessNodes = []string{
	"localhost:9001", "localhost:9002", "localhost:9003",
	"localhost:9004", "localhost:9005",
}

// ═══════════════════════════════════════════════════════════════════════════════
//  LEADER-FOLLOWER TESTS
// ═══════════════════════════════════════════════════════════════════════════════

// Test 1: After leader acknowledges write, reading from the Leader returns consistent data
func TestLeader_ReadAfterWrite_Consistent(t *testing.T) {
	key := fmt.Sprintf("test-lw-%d", time.Now().UnixNano())
	value := "hello-leader"

	// Write to leader (blocks until W quorum is met)
	setKey(t, leaderAddr, key, value)

	// Read from leader — should be consistent
	result, status := getKey(t, leaderAddr, key)
	if status != http.StatusOK {
		t.Fatalf("Expected 200, got %d", status)
	}
	if result.Value != value {
		t.Errorf("Leader read inconsistent: got %q, want %q", result.Value, value)
	}
	t.Logf("✓ Leader read after write: key=%s value=%s version=%d", key, result.Value, result.Version)
}

// Test 2: After leader acknowledges write, reading from a Follower (via /get) returns consistent data
func TestLeader_FollowerReadAfterAck_Consistent(t *testing.T) {
	key := fmt.Sprintf("test-fr-%d", time.Now().UnixNano())
	value := "hello-follower"

	// Write to leader — waits for quorum
	setKey(t, leaderAddr, key, value)

	// Small grace period for replication to finish
	time.Sleep(500 * time.Millisecond)

	// Read from a follower — should be consistent after leader ack'd
	for _, fAddr := range followerAddrs {
		result, status := getKey(t, fAddr, key)
		if status != http.StatusOK {
			t.Errorf("Follower %s: expected 200, got %d", fAddr, status)
			continue
		}
		if result.Value != value {
			t.Errorf("Follower %s inconsistent: got %q, want %q", fAddr, result.Value, value)
		} else {
			t.Logf("✓ Follower %s consistent: value=%s version=%d", fAddr, result.Value, result.Version)
		}
	}
}

// Test 3: During replication window, local_read on followers MAY show inconsistency
func TestLeader_LocalRead_InconsistencyWindow(t *testing.T) {
	inconsistencySeen := false
	attempts := 50

	for i := 0; i < attempts; i++ {
		key := fmt.Sprintf("test-inc-%d-%d", time.Now().UnixNano(), i)
		oldValue := "old"
		newValue := "new"

		// Write initial value and wait for propagation
		setKey(t, leaderAddr, key, oldValue)
		time.Sleep(600 * time.Millisecond)

		// Fire off a new write in a goroutine (don't wait for ack)
		go func() {
			body, _ := json.Marshal(map[string]string{"key": key, "value": newValue})
			http.Post(fmt.Sprintf("http://%s/set", leaderAddr), "application/json", bytes.NewReader(body))
		}()

		// Immediately read local_read from followers — might catch stale data
		time.Sleep(10 * time.Millisecond) // tiny delay so leader starts processing
		for _, fAddr := range followerAddrs {
			result, status := localRead(t, fAddr, key)
			if status == http.StatusOK && result.Value == oldValue {
				t.Logf("⚡ INCONSISTENCY on %s: local_read got %q (stale), new value is %q",
					fAddr, result.Value, newValue)
				inconsistencySeen = true
			}
		}

		if inconsistencySeen {
			break
		}
	}

	if inconsistencySeen {
		t.Log("✓ Successfully observed inconsistency window via local_read")
	} else {
		t.Log("⚠ No inconsistency observed in this run (may need higher load or more attempts)")
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
//  LEADERLESS TESTS
// ═══════════════════════════════════════════════════════════════════════════════

// Test 4: Leaderless — expose inconsistency window during replication
func TestLeaderless_InconsistencyWindow(t *testing.T) {
	inconsistencySeen := false
	attempts := 50

	for i := 0; i < attempts; i++ {
		key := fmt.Sprintf("test-ll-%d-%d", time.Now().UnixNano(), i)
		oldValue := "before"
		newValue := "after"

		// Pick a random coordinator
		coordIdx := rand.Intn(len(leaderlessNodes))
		coordAddr := leaderlessNodes[coordIdx]

		// Write initial value and let it propagate
		setKey(t, coordAddr, key, oldValue)
		time.Sleep(600 * time.Millisecond)

		// Pick a DIFFERENT coordinator for the update
		newCoordIdx := (coordIdx + 1) % len(leaderlessNodes)
		newCoordAddr := leaderlessNodes[newCoordIdx]

		// Fire off write without waiting
		go func() {
			body, _ := json.Marshal(map[string]string{"key": key, "value": newValue})
			http.Post(fmt.Sprintf("http://%s/set", newCoordAddr), "application/json", bytes.NewReader(body))
		}()

		// Immediately read from other nodes (R=1 reads local only)
		time.Sleep(10 * time.Millisecond)
		for j, nodeAddr := range leaderlessNodes {
			if j == newCoordIdx {
				continue // skip the coordinator
			}
			result, status := getKey(t, nodeAddr, key)
			if status == http.StatusOK && result.Value == oldValue {
				t.Logf("⚡ INCONSISTENCY on %s: got %q (stale), coordinator %s writing %q",
					nodeAddr, result.Value, newCoordAddr, newValue)
				inconsistencySeen = true
			}
		}

		if inconsistencySeen {
			break
		}
	}

	if inconsistencySeen {
		t.Log("✓ Successfully observed leaderless inconsistency window")
	} else {
		t.Log("⚠ No inconsistency observed (try higher load)")
	}
}

// Test 5: After coordinator acknowledges write, reading from coordinator is consistent
func TestLeaderless_CoordinatorReadAfterAck_Consistent(t *testing.T) {
	coordAddr := leaderlessNodes[0]
	key := fmt.Sprintf("test-ll-cons-%d", time.Now().UnixNano())
	value := "consistent-value"

	// Write to coordinator — blocks until all N nodes ack
	setKey(t, coordAddr, key, value)

	// Read from coordinator — must be consistent (R=1 reads local)
	result, status := getKey(t, coordAddr, key)
	if status != http.StatusOK {
		t.Fatalf("Expected 200, got %d", status)
	}
	if result.Value != value {
		t.Errorf("Coordinator inconsistent: got %q, want %q", result.Value, value)
	}
	t.Logf("✓ Coordinator read consistent: value=%s version=%d", result.Value, result.Version)
}

// Test 6: After coordinator acknowledges write, reading from another node is consistent
func TestLeaderless_OtherNodeReadAfterAck_Consistent(t *testing.T) {
	coordAddr := leaderlessNodes[0]
	otherAddr := leaderlessNodes[3]
	key := fmt.Sprintf("test-ll-other-%d", time.Now().UnixNano())
	value := "propagated-value"

	// Write to coordinator — blocks until W=N
	setKey(t, coordAddr, key, value)

	// Read from a different node — should be consistent since W=N completed
	result, status := getKey(t, otherAddr, key)
	if status != http.StatusOK {
		t.Fatalf("Expected 200 from %s, got %d", otherAddr, status)
	}
	if result.Value != value {
		t.Errorf("Node %s inconsistent: got %q, want %q", otherAddr, result.Value, value)
	}
	t.Logf("✓ Other node %s consistent after ack: value=%s version=%d",
		otherAddr, result.Value, result.Version)
}
