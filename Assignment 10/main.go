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
	// Only apply if incoming version is newer
	if existing, ok := s.data[key]; !ok || version > existing.Version {
		s.data[key] = Entry{Value: value, Version: version}
	}
}

// Set locally and return the new version (used by leader/coordinator)
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
	Role      string   // "leader" or "follower"
	NodeAddr  string   // this node's address (e.g. "node1:8080")
	Followers []string // addresses of follower nodes (leader only)
	W         int      // write quorum
	R         int      // read quorum
	N         int      // total nodes (always 5)
}

func loadConfig() Config {
	role := envOrDefault("ROLE", "follower")
	nodeAddr := envOrDefault("NODE_ADDR", "localhost:8080")
	followersRaw := envOrDefault("FOLLOWERS", "") // comma-separated
	w := envOrDefaultInt("W", 5)
	r := envOrDefaultInt("R", 1)
	n := envOrDefaultInt("N", 5)

	var followers []string
	if followersRaw != "" {
		followers = strings.Split(followersRaw, ",")
	}

	return Config{
		Role: role, NodeAddr: nodeAddr,
		Followers: followers, W: w, R: r, N: n,
	}
}

// ─── Global State ─────────────────────────────────────────────────────────────

var (
	store  *KVStore
	config Config
)

// ─── HTTP Handlers ────────────────────────────────────────────────────────────

// POST /set  — body: {"key": "...", "value": "..."}
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

	if config.Role == "follower" {
		http.Error(w, "writes must go to leader", http.StatusForbidden)
		return
	}

	// ── Leader write path ──
	// 1. Write locally first
	version := store.SetLocal(req.Key, req.Value)
	acksNeeded := config.W - 1 // leader counts as 1
	log.Printf("[LEADER] SET key=%s value=%s version=%d (W=%d, need %d follower acks)",
		req.Key, req.Value, version, config.W, acksNeeded)

	if acksNeeded > 0 {
		ackCh := make(chan bool, len(config.Followers))

		for _, f := range config.Followers {
			go func(addr string) {
				ok := replicateToFollower(addr, req.Key, req.Value, version)
				ackCh <- ok
				// Sleep 200ms AFTER sending to each follower (as per spec)
				time.Sleep(200 * time.Millisecond)
			}(f)
		}

		// Wait for enough acks
		acks := 0
		failures := 0
		total := len(config.Followers)
		for i := 0; i < total; i++ {
			if <-ackCh {
				acks++
				if acks >= acksNeeded {
					break
				}
			} else {
				failures++
				if failures > (total - acksNeeded) {
					break
				}
			}
		}

		if acks < acksNeeded {
			http.Error(w, "failed to reach write quorum", http.StatusServiceUnavailable)
			return
		}
	}

	resp, _ := json.Marshal(map[string]interface{}{
		"status": "created", "key": req.Key, "version": version,
	})
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	w.Write(resp)
}

// GET /get?key=...
func handleGet(w http.ResponseWriter, r *http.Request) {
	key := r.URL.Query().Get("key")
	if key == "" {
		http.Error(w, "key is required", http.StatusBadRequest)
		return
	}

	if config.Role == "leader" {
		handleLeaderRead(w, key)
	} else {
		handleFollowerRead(w, key)
	}
}

func handleLeaderRead(w http.ResponseWriter, key string) {
	// Leader always reads locally first
	localEntry, localOk := store.Get(key)
	best := localEntry
	found := localOk

	readsNeeded := config.R - 1 // leader counts as 1 read

	if readsNeeded > 0 {
		type result struct {
			entry Entry
			ok    bool
		}
		ch := make(chan result, len(config.Followers))

		for _, f := range config.Followers {
			go func(addr string) {
				e, ok := readFromFollower(addr, key)
				ch <- result{e, ok}
			}(f)
		}

		reads := 0
		for i := 0; i < len(config.Followers); i++ {
			res := <-ch
			if res.ok {
				reads++
				found = true
				if res.entry.Version > best.Version {
					best = res.entry
				}
			}
			if reads >= readsNeeded {
				break
			}
		}
	}

	if !found {
		http.Error(w, "key not found", http.StatusNotFound)
		return
	}

	resp, _ := json.Marshal(map[string]interface{}{
		"value": best.Value, "version": best.Version,
	})
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(resp)
}

func handleFollowerRead(w http.ResponseWriter, key string) {
	// Follower: sleep 50ms before responding (spec requirement)
	time.Sleep(50 * time.Millisecond)

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

// GET /local_read?key=...  — sneaky test endpoint, reads local store only
func handleLocalRead(w http.ResponseWriter, r *http.Request) {
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

// POST /replicate  — internal endpoint called by leader to push updates
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

	// Follower sleeps 100ms before applying the update (spec requirement)
	time.Sleep(100 * time.Millisecond)

	store.Set(req.Key, req.Value, req.Version)
	log.Printf("[FOLLOWER] replicated key=%s version=%d", req.Key, req.Version)

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"ok"}`))
}

// GET /internal_read?key=...  — internal endpoint for leader to read from follower
func handleInternalRead(w http.ResponseWriter, r *http.Request) {
	key := r.URL.Query().Get("key")
	if key == "" {
		http.Error(w, "key is required", http.StatusBadRequest)
		return
	}

	// Follower sleeps 50ms when leader reads from it (spec requirement)
	time.Sleep(50 * time.Millisecond)

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

// ─── Inter-Node Communication ─────────────────────────────────────────────────

func replicateToFollower(addr, key, value string, version int) bool {
	body, _ := json.Marshal(map[string]interface{}{
		"key": key, "value": value, "version": version,
	})
	resp, err := http.Post(
		fmt.Sprintf("http://%s/replicate", addr),
		"application/json",
		bytes.NewReader(body),
	)
	if err != nil {
		log.Printf("[LEADER] replication to %s failed: %v", addr, err)
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

func readFromFollower(addr, key string) (Entry, bool) {
	resp, err := http.Get(fmt.Sprintf("http://%s/internal_read?key=%s", addr, key))
	if err != nil || resp.StatusCode != http.StatusOK {
		return Entry{}, false
	}
	defer resp.Body.Close()

	var result struct {
		Value   string `json:"value"`
		Version int    `json:"version"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return Entry{}, false
	}
	return Entry{Value: result.Value, Version: result.Version}, true
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

	log.Printf("Starting KV node | role=%s addr=%s W=%d R=%d N=%d followers=%v",
		config.Role, config.NodeAddr, config.W, config.R, config.N, config.Followers)

	// Public API
	http.HandleFunc("/set", handleSet)
	http.HandleFunc("/get", handleGet)
	http.HandleFunc("/local_read", handleLocalRead)

	// Internal (inter-node) API
	http.HandleFunc("/replicate", handleReplicate)
	http.HandleFunc("/internal_read", handleInternalRead)

	port := "8080"
	if p := os.Getenv("PORT"); p != "" {
		port = p
	}
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
