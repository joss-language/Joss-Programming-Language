package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

func main() {
	start := time.Now()

	file, err := os.Open("benchmarks/io_csv_stats/data.csv")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening file: %v\n", err)
		os.Exit(1)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	// Skip header
	if scanner.Scan() {
		_ = scanner.Text()
	}

	matchingCount := 0
	totalRawSales := 0.0
	totalTaxedValue := 0.0

	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Split(line, ",")
		if len(parts) >= 6 {
			region := parts[1]
			salesAmount, _ := strconv.ParseFloat(parts[3], 64)
			quantity, _ := strconv.Atoi(parts[4])
			taxRate, _ := strconv.ParseFloat(parts[5], 64)

			if region == "North" && quantity >= 5 {
				matchingCount++
				totalRawSales += salesAmount
				totalTaxedValue += salesAmount * float64(quantity) * (1.0 + taxRate)
			}
		}
	}

	elapsed := time.Since(start)
	elapsedMs := float64(elapsed.Nanoseconds()) / 1e6

	fmt.Printf("CSV Stats: matches=%d, raw=%.2f, taxed=%.2f in %.4f ms\n",
		matchingCount, totalRawSales, totalTaxedValue, elapsedMs)
}
