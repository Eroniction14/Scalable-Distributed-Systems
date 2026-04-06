package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

// ─── Versioned KV Entry ───────────────────────────────────────────────────────

type Entry struct {
	Value   string `json:"value"`
	Version int    `json:"version"`
}

// ─── In-Memory Store ──────────────────────────────────────────────────────────

type KVStore struct {
	mu   sync.RWMutex
	data map[string]Entry
}

func NewKVStore() *KVStore {
	return &KVStore{data: make(map[string]Entry)}
}

func (s *KVStore) Get(key string) (Entry, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	e, ok := s.data[key]
	return e, ok
}

func (s *KVStore) Set(key, value string, version int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if existing, ok := s.data[key]; !ok || version > existing.Version {
		s.data[key] = Entry{Value: value, Version: version}
	}
}

func (s *KVStore) SetLocal(key, value string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	v := 1
	if existing, ok := s.data[key]; ok {
		v = existing.Version + 1
	}
	s.data[key] = Entry{Value: value, Version: v}
	return v
}

// ─── Node Configuration ──────────────────────────────────────────────────────

type Config struct {
	NodeAddr string   // this node's address
	Peers    []string // all OTHER nodes in the cluster
	N        int      // total nodes (5)
}

func loadConfig() Config {
	nodeAddr := envOrDefault("NODE_ADDR", "localhost:8080")
	peersRaw := envOrDefault("PEERS", "") // comma-separated list of other nodes
	n := envOrDefaultInt("N", 5)

	var peers []string
	if peersRaw != "" {
		peers = strings.Split(peersRaw, ",")
	}

	return Config{NodeAddr: nodeAddr, Peers: peers, N: n}
}

var (
	store  *KVStore
	config Config
)

// ─── HTTP Handlers ────────────────────────────────────────────────────────────

// POST /set  — any node can receive this; it becomes the Write Coordinator
// W=N: coordinator must write to ALL nodes before responding
func handleSet(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "use POST", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Key   string `json:"key"`
		Value string `json:"value"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Key == "" {
		http.Error(w, "bad request: key is required", http.StatusBadRequest)
		return
	}

	// 1. Write locally first — this node is the Write Coordinator
	version := store.SetLocal(req.Key, req.Value)
	log.Printf("[COORDINATOR %s] SET key=%s value=%s version=%d, replicating to %d peers",
		config.NodeAddr, req.Key, req.Value, version, len(config.Peers))

	// 2. Replicate to ALL peers (W=N)
	ackCh := make(chan bool, len(config.Peers))
	for _, p := range config.Peers {
		go func(addr string) {
			ok := replicateToPeer(addr, req.Key, req.Value, version)
			ackCh <- ok
		}(p)
	}

	// 3. Wait for ALL peers to ACK
	acks := 0
	failures := 0
	for i := 0; i < len(config.Peers); i++ {
		if <-ackCh {
			acks++
		} else {
			failures++
		}
	}

	needed := config.N - 1 // all other nodes
	if acks < needed {
		log.Printf("[COORDINATOR %s] FAILED: only %d/%d acks for key=%s",
			config.NodeAddr, acks, needed, req.Key)
		http.Error(w, "failed to reach all nodes", http.StatusServiceUnavailable)
		return
	}

	log.Printf("[COORDINATOR %s] SUCCESS: key=%s version=%d all %d peers acked",
		config.NodeAddr, req.Key, version, acks)

	resp, _ := json.Marshal(map[string]interface{}{
		"status": "created", "key": req.Key, "version": version,
	})
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	w.Write(resp)
}

// GET /get?key=...  — R=1, just return local value
func handleGet(w http.ResponseWriter, r *http.Request) {
	key := r.URL.Query().Get("key")
	if key == "" {
		http.Error(w, "key is required", http.StatusBadRequest)
		return
	}

	e, ok := store.Get(key)
	if !ok {
		http.Error(w, "key not found", http.StatusNotFound)
		return
	}

	resp, _ := json.Marshal(map[string]interface{}{
		"value": e.Value, "version": e.Version,
	})
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(resp)
}

// POST /replicate  — internal: called by the Write Coordinator to push updates
func handleReplicate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Key     string `json:"key"`
		Value   string `json:"value"`
		Version int    `json:"version"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	// Simulate storage delay: sleep 100ms before applying
	time.Sleep(100 * time.Millisecond)

	store.Set(req.Key, req.Value, req.Version)
	log.Printf("[NODE %s] replicated key=%s version=%d", config.NodeAddr, req.Key, req.Version)

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"ok"}`))
}

// ─── Inter-Node Communication ─────────────────────────────────────────────────

func replicateToPeer(addr, key, value string, version int) bool {
	body, _ := json.Marshal(map[string]interface{}{
		"key": key, "value": value, "version": version,
	})

	// Coordinator sleeps 200ms after sending to each peer (spec)
	defer func() { time.Sleep(200 * time.Millisecond) }()

	resp, err := http.Post(
		fmt.Sprintf("http://%s/replicate", addr),
		"application/json",
		bytes.NewReader(body),
	)
	if err != nil {
		log.Printf("[NODE %s] replication to %s failed: %v", config.NodeAddr, addr, err)
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envOrDefaultInt(key string, def int) int {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	n := def
	fmt.Sscanf(v, "%d", &n)
	return n
}

// ─── Main ─────────────────────────────────────────────────────────────────────

func main() {
	store = NewKVStore()
	config = loadConfig()

	log.Printf("Starting LEADERLESS KV node | addr=%s N=%d peers=%v",
		config.NodeAddr, config.N, config.Peers)

	// Public API
	http.HandleFunc("/set", handleSet)
	http.HandleFunc("/get", handleGet)

	// Internal API
	http.HandleFunc("/replicate", handleReplicate)

	port := "8080"
	if p := os.Getenv("PORT"); p != "" {
		port = p
	}
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
