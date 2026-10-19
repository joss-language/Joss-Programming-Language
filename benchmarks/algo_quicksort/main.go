//go:build ignore

package main

import (
	"fmt"
	"os"
	"time"
)

const N = 50000

func partition(arr []int32, low, high int) int {
	pivot := arr[high]
	i := low - 1
	for j := low; j < high; j++ {
		if arr[j] <= pivot {
			i++
			arr[i], arr[j] = arr[j], arr[i]
		}
	}
	arr[i+1], arr[high] = arr[high], arr[i+1]
	return i + 1
}

func quicksort(arr []int32, low, high int) {
	if low < high {
		pi := partition(arr, low, high)
		quicksort(arr, low, pi-1)
		quicksort(arr, pi+1, high)
	}
}

func main() {
	arr := make([]int32, N)
	var state uint32 = 123456789
	for i := 0; i < N; i++ {
		state = state*1664525 + 1013904223
		arr[i] = int32(state & 0x7FFFFFFF)
	}

	start := time.Now()
	quicksort(arr, 0, N-1)
	elapsed := time.Since(start)

	for i := 0; i < N-1; i++ {
		if arr[i] > arr[i+1] {
			fmt.Fprintf(os.Stderr, "Error: Array not sorted at %d\n", i)
			os.Exit(1)
		}
	}

	fmt.Printf("Quicksort sorted %d elements in %.4f ms\n", N, float64(elapsed.Nanoseconds())/1e6)
}
