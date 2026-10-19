//go:build ignore

package main

import (
	"fmt"
	"time"
)

func mandelbrot(N, maxIter int) int {
	total := 0
	fn := float64(N)
	for y := 0; y < N; y++ {
		ci := -1.25 + (2.5 * float64(y) / fn)
		for x := 0; x < N; x++ {
			cr := -2.0 + (2.5 * float64(x) / fn)
			zr := 0.0
			zi := 0.0
			it := 0
			for it < maxIter && (zr*zr+zi*zi) <= 4.0 {
				zr2 := zr * zr
				zi2 := zi * zi
				zi = 2.0*zr*zi + ci
				zr = zr2 - zi2 + cr
				it++
			}
			total += it
		}
	}
	return total
}

func main() {
	start := time.Now()
	total := mandelbrot(500, 50)
	elapsed := time.Since(start)
	elapsedMs := float64(elapsed.Nanoseconds()) / 1e6
	fmt.Printf("total=%d time=%.2fms\n", total, elapsedMs)
}
