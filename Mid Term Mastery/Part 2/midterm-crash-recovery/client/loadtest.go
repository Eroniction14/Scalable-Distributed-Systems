// package main

// import (
// 	"encoding/json"
// 	"fmt"
// 	"net/http"
// 	"os"
// 	"time"
// )

// type Result struct {
// 	RequestNum   int           `json:"request_num"`
// 	StatusCode   int           `json:"status_code"`
// 	ResponseTime time.Duration `json:"response_time_ms"`
// 	Success      bool          `json:"success"`
// }

// func main() {
// 	serverURL := "http://localhost:8080/albums"
// 	totalRequests := 70
// 	results := []Result{}

// 	fmt.Println("=== Load Test: Albums Service (No Circuit Breaker) ===")
// 	fmt.Printf("Target: %s\n", serverURL)
// 	fmt.Printf("Total Requests: %d\n\n", totalRequests)

// 	successCount := 0
// 	failCount := 0
// 	var totalResponseTime time.Duration

// 	for i := 1; i <= totalRequests; i++ {
// 		start := time.Now()
// 		resp, err := http.Get(serverURL)
// 		elapsed := time.Since(start)

// 		r := Result{RequestNum: i, ResponseTime: elapsed}

// 		if err != nil {
// 			r.StatusCode = 0
// 			r.Success = false
// 			failCount++
// 			fmt.Printf("Request #%d | FAILED  | %v | Error: %s\n", i, elapsed, err)
// 		} else {
// 			r.StatusCode = resp.StatusCode
// 			r.Success = resp.StatusCode == 200
// 			resp.Body.Close()

// 			if r.Success {
// 				successCount++
// 				fmt.Printf("Request #%d | SUCCESS | %v | Status: %d\n", i, elapsed, r.StatusCode)
// 			} else {
// 				failCount++
// 				fmt.Printf("Request #%d | FAILED  | %v | Status: %d\n", i, elapsed, r.StatusCode)
// 			}
// 		}

// 		totalResponseTime += elapsed
// 		results = append(results, r)
// 		time.Sleep(50 * time.Millisecond) // small delay between requests
// 	}

// 	// Print summary
// 	fmt.Println("\n=== RESULTS SUMMARY ===")
// 	fmt.Printf("Total Requests:  %d\n", totalRequests)
// 	fmt.Printf("Successful:      %d (%.1f%%)\n", successCount, float64(successCount)/float64(totalRequests)*100)
// 	fmt.Printf("Failed:          %d (%.1f%%)\n", failCount, float64(failCount)/float64(totalRequests)*100)
// 	fmt.Printf("Avg Response:    %v\n", totalResponseTime/time.Duration(totalRequests))

// 	// Save results to JSON for charting
// 	file, _ := os.Create("results_no_fix.json")
// 	defer file.Close()
// 	json.NewEncoder(file).Encode(results)
// 	fmt.Println("\nResults saved to results_no_fix.json")
// }
