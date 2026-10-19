package main

import (
	"fmt"
	"sync"
	"time"
)

const (
	NumWorkers     = 8
	TasksPerWorker = 100
	InnerIters     = 500
)

func worker(startK, endK int, wg *sync.WaitGroup, results []int64, idx int) {
	defer wg.Done()
	var total int64 = 0
	for k := startK; k <= endK; k++ {
		var taskSum int64 = 0
		for j := 1; j <= InnerIters; j++ {
			taskSum += int64((j*17 + k*31) % 1000)
		}
		total += taskSum
	}
	results[idx] = total
}

func main() {
	start := time.Now()

	var wg sync.WaitGroup
	results := make([]int64, NumWorkers)

	for w := 0; w < NumWorkers; w++ {
		wg.Add(1)
		startK := w*TasksPerWorker + 1
		endK := (w + 1) * TasksPerWorker
		go worker(startK, endK, &wg, results, w)
	}

	wg.Wait()

	var grandTotal int64 = 0
	for _, res := range results {
		grandTotal += res
	}

	elapsed := time.Since(start)
	elapsedMs := float64(elapsed.Nanoseconds()) / 1e6

	fmt.Printf("Concurrency 8 workers processed 800 tasks: checksum=%d in %.4f ms\n", grandTotal, elapsedMs)
}
