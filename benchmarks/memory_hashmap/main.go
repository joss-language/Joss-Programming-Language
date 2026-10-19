//go:build ignore

package main

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

const N = 50000

func main() {
	start := time.Now()

	m := make(map[string]string, N)

	// Insertion
	for i := 0; i < N; i++ {
		istr := strconv.Itoa(i)
		m["user_key_"+istr] = "val_" + istr
	}

	// Lookup
	hits := 0
	for i := 0; i < N; i++ {
		istr := strconv.Itoa(i)
		key := "user_key_" + istr
		val, ok := m[key]
		if ok && val == "val_"+istr {
			hits++
		}
	}

	elapsed := time.Since(start)
	elapsedMs := float64(elapsed.Nanoseconds()) / 1e6

	if hits != N {
		fmt.Fprintf(os.Stderr, "Verification failed: hits = %d expected %d\n", hits, N)
		os.Exit(1)
	}

	fmt.Printf("HashMap inserted and queried %d items (%d hits) in %.4f ms\n", N, hits, elapsedMs)
}
