package main

import (
	"bufio"
	"fmt"
	"os"
	"time"
)

func main() {
	iterations := 100000
	line := []byte("Hello, this is a test line of text!\n")

	// Unbuffered writes
	f1, _ := os.Create("unbuffered.txt")
	start1 := time.Now()
	for i := 0; i < iterations; i++ {
		f1.Write(line)
	}
	f1.Close()
	unbufferedTime := time.Since(start1)

	// Buffered writes
	f2, _ := os.Create("buffered.txt")
	w := bufio.NewWriter(f2)
	start2 := time.Now()
	for i := 0; i < iterations; i++ {
		w.Write(line)
	}
	w.Flush()
	f2.Close()
	bufferedTime := time.Since(start2)

	fmt.Println("Unbuffered time:", unbufferedTime)
	fmt.Println("Buffered time:  ", bufferedTime)
	fmt.Printf("Buffered is %.1fx faster\n", float64(unbufferedTime)/float64(bufferedTime))
}
