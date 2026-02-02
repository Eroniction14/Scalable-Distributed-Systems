package main

import (
	"fmt"
	"runtime"
	"time"
)

func main() {
	const iterations = 1000000

	// Test 1: Single OS thread
	runtime.GOMAXPROCS(1)
	time1 := pingPong(iterations)
	fmt.Printf("GOMAXPROCS=1: %v (%.0f ns/switch)\n", time1, float64(time1.Nanoseconds())/(2*iterations))

	// Test 2: Multiple OS threads
	runtime.GOMAXPROCS(runtime.NumCPU())
	time2 := pingPong(iterations)
	fmt.Printf("GOMAXPROCS=%d: %v (%.0f ns/switch)\n", runtime.NumCPU(), time2, float64(time2.Nanoseconds())/(2*iterations))
}

func pingPong(iterations int) time.Duration {
	ch1 := make(chan struct{})
	ch2 := make(chan struct{})
	done := make(chan struct{})

	// Goroutine 1: ping
	go func() {
		for i := 0; i < iterations; i++ {
			ch1 <- struct{}{}
			<-ch2
		}
		done <- struct{}{}
	}()

	// Goroutine 2: pong
	go func() {
		for i := 0; i < iterations; i++ {
			<-ch1
			ch2 <- struct{}{}
		}
	}()

	start := time.Now()
	<-done
	return time.Since(start)
}
