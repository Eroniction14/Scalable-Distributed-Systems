package main

import (
	"fmt"
	"sync"
	"sync/atomic"
)

func main() {
	// Test 1: Atomic counter (correct)
	var atomicOps atomic.Uint64
	var wg1 sync.WaitGroup

	for i := 0; i < 50; i++ {
		wg1.Add(1)
		go func() {
			defer wg1.Done()
			for j := 0; j < 1000; j++ {
				atomicOps.Add(1)
			}
		}()
	}
	wg1.Wait()
	fmt.Println("Atomic counter:", atomicOps.Load())

	// Test 2: Regular counter (race condition!)
	var regularOps uint64
	var wg2 sync.WaitGroup

	for i := 0; i < 50; i++ {
		wg2.Add(1)
		go func() {
			defer wg2.Done()
			for j := 0; j < 1000; j++ {
				regularOps++ // NOT thread-safe
			}
		}()
	}
	wg2.Wait()
	fmt.Println("Regular counter:", regularOps)
}
