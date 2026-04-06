package main

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

// ─── Configuration ────────────────────────────────────────────────────────────

type LoadTestConfig struct {
	Mode         string   // "leader" or "leaderless"
	WriteAddr    string   // leader address (leader mode) or ignored (leaderless)
	ReadAddrs    []string // follower addresses (leader) or all nodes (leaderless)
	AllAddrs     []string // all node addresses (leaderless writes go to random node)
	WritePct     float64  // write percentage (0.0 - 1.0)
	NumRequests  int      // total requests to send
	Concurrency  int      // number of concurrent workers
	NumKeys      int      // small key space to force read-write overlap
	OutputPrefix string   // prefix for output CSV files
}

// ─── Version Tracker ──────────────────────────────────────────────────────────
// Tracks the latest known version for each key to detect stale reads

type VersionTracker struct {
	mu       sync.RWMutex
	versions map[string]int
	// Track write timestamps per key for read-write interval measurement
	writeTimes map[string]time.Time
}

func NewVersionTracker() *VersionTracker {
	return &VersionTracker{
		versions:   make(map[string]int),
		writeTimes: make(map[string]time.Time),
	}
}

func (vt *VersionTracker) RecordWrite(key string, version int) {
	vt.mu.Lock()
	defer vt.mu.Unlock()
	if version > vt.versions[key] {
		vt.versions[key] = version
	}
	vt.writeTimes[key] = time.Now()
}

func (vt *VersionTracker) CheckRead(key string, version int) (stale bool, lastWriteTime time.Time) {
	vt.mu.RLock()
	defer vt.mu.RUnlock()
	expected := vt.versions[key]
	stale = version < expected && expected > 0
	lastWriteTime = vt.writeTimes[key]
	return
}

// ─── Result Types ─────────────────────────────────────────────────────────────

type RequestResult struct {
	Type       string // "read" or "write"
	Key        string
	Latency    time.Duration
	StatusCode int
	Stale      bool // for reads: was this a stale read?
	Version    int
	RWInterval time.Duration // time since last write to this key (reads only)
}

// ─── HTTP Client ──────────────────────────────────────────────────────────────

var client = &http.Client{Timeout: 10 * time.Second}

type KVResponse struct {
	Value   string `json:"value"`
	Version int    `json:"version"`
	Status  string `json:"status"`
	Key     string `json:"key"`
}

func doWrite(addr, key, value string) (KVResponse, time.Duration, int) {
	body, _ := json.Marshal(map[string]string{"key": key, "value": value})
	start := time.Now()
	resp, err := client.Post(
		fmt.Sprintf("http://%s/set", addr), "application/json", bytes.NewReader(body),
	)
	lat := time.Since(start)
	if err != nil {
		return KVResponse{}, lat, 0
	}
	defer resp.Body.Close()
	var result KVResponse
	json.NewDecoder(resp.Body).Decode(&result)
	return result, lat, resp.StatusCode
}

func doRead(addr, key string) (KVResponse, time.Duration, int) {
	start := time.Now()
	resp, err := client.Get(fmt.Sprintf("http://%s/get?key=%s", addr, key))
	lat := time.Since(start)
	if err != nil {
		return KVResponse{}, lat, 0
	}
	defer resp.Body.Close()
	var result KVResponse
	if resp.StatusCode == http.StatusOK {
		json.NewDecoder(resp.Body).Decode(&result)
	}
	return result, lat, resp.StatusCode
}

// ─── Load Generator ───────────────────────────────────────────────────────────
// Uses a small key space (NumKeys) to guarantee reads and writes frequently
// hit the same key. Keys are chosen uniformly at random from key-0..key-(N-1).
// With a small N (e.g. 20), concurrent workers will naturally produce
// temporally clustered read-write pairs on the same key.

func runLoadTest(cfg LoadTestConfig) []RequestResult {
	results := make([]RequestResult, 0, cfg.NumRequests)
	var mu sync.Mutex
	tracker := NewVersionTracker()

	var completed int64
	reqCh := make(chan int, cfg.NumRequests)
	var wg sync.WaitGroup

	// Fill the work channel
	for i := 0; i < cfg.NumRequests; i++ {
		reqCh <- i
	}
	close(reqCh)

	// Spawn workers
	for w := 0; w < cfg.Concurrency; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range reqCh {
				key := fmt.Sprintf("key-%d", rand.Intn(cfg.NumKeys))
				isWrite := rand.Float64() < cfg.WritePct

				var res RequestResult

				if isWrite {
					value := fmt.Sprintf("v-%d-%d", time.Now().UnixNano(), rand.Intn(10000))

					var addr string
					if cfg.Mode == "leader" {
						addr = cfg.WriteAddr
					} else {
						// Leaderless: pick a random node as coordinator
						addr = cfg.AllAddrs[rand.Intn(len(cfg.AllAddrs))]
					}

					kvResp, lat, status := doWrite(addr, key, value)
					res = RequestResult{
						Type: "write", Key: key, Latency: lat,
						StatusCode: status, Version: kvResp.Version,
					}
					if status == http.StatusCreated {
						tracker.RecordWrite(key, kvResp.Version)
					}
				} else {
					// Read from a random read-capable node
					var addr string
					if cfg.Mode == "leader" {
						// Read from any node (leader or follower)
						all := append([]string{cfg.WriteAddr}, cfg.ReadAddrs...)
						addr = all[rand.Intn(len(all))]
					} else {
						addr = cfg.AllAddrs[rand.Intn(len(cfg.AllAddrs))]
					}

					kvResp, lat, status := doRead(addr, key)
					var rwInterval time.Duration
					if status == http.StatusOK {
						isStale, lastWrite := tracker.CheckRead(key, kvResp.Version)
						if !lastWrite.IsZero() {
							rwInterval = time.Since(lastWrite)
						}
						res = RequestResult{
							Type: "read", Key: key, Latency: lat,
							StatusCode: status, Stale: isStale,
							Version: kvResp.Version, RWInterval: rwInterval,
						}
					} else {
						res = RequestResult{
							Type: "read", Key: key, Latency: lat,
							StatusCode: status,
						}
					}
				}

				mu.Lock()
				results = append(results, res)
				mu.Unlock()

				c := atomic.AddInt64(&completed, 1)
				if c%500 == 0 {
					fmt.Printf("  Progress: %d/%d\n", c, cfg.NumRequests)
				}
			}
		}()
	}

	wg.Wait()
	return results
}

// ─── Reporting ────────────────────────────────────────────────────────────────

func writeCSV(filename string, results []RequestResult) {
	f, err := os.Create(filename)
	if err != nil {
		fmt.Printf("Error creating %s: %v\n", filename, err)
		return
	}
	defer f.Close()
	w := csv.NewWriter(f)
	defer w.Flush()

	w.Write([]string{"type", "key", "latency_ms", "status_code", "stale", "version", "rw_interval_ms"})
	for _, r := range results {
		stale := "false"
		if r.Stale {
			stale = "true"
		}
		w.Write([]string{
			r.Type, r.Key,
			fmt.Sprintf("%.2f", float64(r.Latency.Microseconds())/1000.0),
			fmt.Sprintf("%d", r.StatusCode),
			stale,
			fmt.Sprintf("%d", r.Version),
			fmt.Sprintf("%.2f", float64(r.RWInterval.Microseconds())/1000.0),
		})
	}
}

func printSummary(results []RequestResult, label string) {
	var readLats, writeLats []float64
	var staleCount, readCount, writeCount int
	var rwIntervals []float64

	for _, r := range results {
		ms := float64(r.Latency.Microseconds()) / 1000.0
		if r.Type == "write" {
			writeLats = append(writeLats, ms)
			writeCount++
		} else {
			readLats = append(readLats, ms)
			readCount++
			if r.Stale {
				staleCount++
			}
			if r.RWInterval > 0 {
				rwIntervals = append(rwIntervals, float64(r.RWInterval.Microseconds())/1000.0)
			}
		}
	}

	fmt.Printf("\n══════════════════════════════════════════════\n")
	fmt.Printf("  %s\n", label)
	fmt.Printf("══════════════════════════════════════════════\n")
	fmt.Printf("  Total requests:  %d\n", len(results))
	fmt.Printf("  Writes:          %d\n", writeCount)
	fmt.Printf("  Reads:           %d\n", readCount)
	fmt.Printf("  Stale reads:     %d / %d (%.2f%%)\n",
		staleCount, readCount, pct(staleCount, readCount))

	if len(readLats) > 0 {
		sort.Float64s(readLats)
		fmt.Printf("\n  Read Latency (ms):\n")
		fmt.Printf("    p50=%.1f  p90=%.1f  p95=%.1f  p99=%.1f  max=%.1f\n",
			percentile(readLats, 50), percentile(readLats, 90),
			percentile(readLats, 95), percentile(readLats, 99),
			readLats[len(readLats)-1])
	}
	if len(writeLats) > 0 {
		sort.Float64s(writeLats)
		fmt.Printf("\n  Write Latency (ms):\n")
		fmt.Printf("    p50=%.1f  p90=%.1f  p95=%.1f  p99=%.1f  max=%.1f\n",
			percentile(writeLats, 50), percentile(writeLats, 90),
			percentile(writeLats, 95), percentile(writeLats, 99),
			writeLats[len(writeLats)-1])
	}
	if len(rwIntervals) > 0 {
		sort.Float64s(rwIntervals)
		fmt.Printf("\n  Read-Write Interval (ms):\n")
		fmt.Printf("    p50=%.1f  p90=%.1f  p95=%.1f  p99=%.1f  max=%.1f\n",
			percentile(rwIntervals, 50), percentile(rwIntervals, 90),
			percentile(rwIntervals, 95), percentile(rwIntervals, 99),
			rwIntervals[len(rwIntervals)-1])
	}
	fmt.Println()
}

func percentile(sorted []float64, p float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	idx := int(float64(len(sorted)-1) * p / 100.0)
	return sorted[idx]
}

func pct(a, b int) float64 {
	if b == 0 {
		return 0
	}
	return float64(a) / float64(b) * 100.0
}

// ─── Main ─────────────────────────────────────────────────────────────────────

func main() {
	mode := flag.String("mode", "leader", "Database mode: 'leader' or 'leaderless'")
	writePct := flag.Float64("write-pct", 0.10, "Write percentage (0.01, 0.10, 0.50, 0.90)")
	numReqs := flag.Int("requests", 2000, "Total number of requests")
	concurrency := flag.Int("concurrency", 20, "Number of concurrent workers")
	numKeys := flag.Int("keys", 20, "Number of unique keys (smaller = more temporal locality)")
	quorum := flag.String("quorum", "W5R1", "Quorum label for output naming (W5R1, W1R5, W3R3)")
	outDir := flag.String("out", "results", "Output directory for CSVs")
	flag.Parse()

	os.MkdirAll(*outDir, 0755)

	// ── Build config based on mode ──
	var cfg LoadTestConfig
	cfg.Mode = *mode
	cfg.WritePct = *writePct
	cfg.NumRequests = *numReqs
	cfg.Concurrency = *concurrency
	cfg.NumKeys = *numKeys

	if *mode == "leader" {
		cfg.WriteAddr = "localhost:8080"
		cfg.ReadAddrs = []string{
			"localhost:8081", "localhost:8082",
			"localhost:8083", "localhost:8084",
		}
	} else {
		cfg.AllAddrs = []string{
			"localhost:9001", "localhost:9002", "localhost:9003",
			"localhost:9004", "localhost:9005",
		}
	}

	wpctLabel := fmt.Sprintf("%.0f", *writePct*100)
	label := fmt.Sprintf("%s_%s_W%s", *mode, *quorum, wpctLabel)

	fmt.Printf("Starting load test: %s\n", label)
	fmt.Printf("  Mode=%s  Quorum=%s  WritePct=%.0f%%  Requests=%d  Concurrency=%d  Keys=%d\n",
		*mode, *quorum, *writePct*100, *numReqs, *concurrency, *numKeys)

	results := runLoadTest(cfg)
	printSummary(results, label)

	csvFile := fmt.Sprintf("%s/%s.csv", *outDir, label)
	writeCSV(csvFile, results)
	fmt.Printf("Results written to %s\n", csvFile)
}

// Suppress unused import
var _ = io.Discard
