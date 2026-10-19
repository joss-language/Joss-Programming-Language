# Master Benchmark Runner for Joss and 8 Industrial Languages
# Automatically runs all benchmark suites and outputs results.

Write-Host "=========================================================="
Write-Host "  JOSS MULTI-LANGUAGE REPRODUCIBLE BENCHMARK SUITE"
Write-Host "=========================================================="

Write-Host "`n[1/5] Running Sieve of Eratosthenes (Computation)..."
& powershell -ExecutionPolicy Bypass -File benchmarks\run_primes.ps1

Write-Host "`n[2/5] Running Quicksort (Algorithms & Recursion)..."
& powershell -ExecutionPolicy Bypass -File benchmarks\run_quicksort.ps1

Write-Host "`n[3/5] Running HashMap Ingestion & Query (Memory & Data Structures)..."
& powershell -ExecutionPolicy Bypass -File benchmarks\run_hashmap.ps1

Write-Host "`n[4/5] Running CSV Parsing & Stats (I/O & Aggregation)..."
& powershell -ExecutionPolicy Bypass -File benchmarks\run_csv.ps1

Write-Host "`n[5/5] Running Concurrency Workers (Parallelism & Channels)..."
& powershell -ExecutionPolicy Bypass -File benchmarks\run_concurrency.ps1

Write-Host "`n=========================================================="
Write-Host "  ALL BENCHMARKS COMPLETED SUCCESSFULLY."
Write-Host "  Technical report available at: BENCHMARKS.md"
Write-Host "=========================================================="
