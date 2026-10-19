// Benchmark A: Prime Sieve (Go implementation)
package main

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

func countPrimes(limit int) int {
	if limit < 2 {
		return 0
	}
	sieve := make([]bool, limit+1)
	for i := 2; i <= limit; i++ {
		sieve[i] = true
	}
	for p := 2; p*p <= limit; p++ {
		if sieve[p] {
			for i := p * p; i <= limit; i += p {
				sieve[i] = false
			}
		}
	}
	count := 0
	for p := 2; p <= limit; p++ {
		if sieve[p] {
			count++
		}
	}
	return count
}

func main() {
	limit := 1000000
	if len(os.Args) > 1 {
		if n, err := strconv.Atoi(os.Args[1]); err == nil {
			limit = n
		}
	}

	start := time.Now()
	count := countPrimes(limit)
	elapsed := time.Since(start)
	ms := float64(elapsed.Nanoseconds()) / 1e6

	fmt.Printf("LIMIT=%d COUNT=%d TIME_MS=%.3f\n", limit, count, ms)
}
