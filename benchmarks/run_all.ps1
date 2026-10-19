# Master Benchmark Runner for Joss and 8 Industrial Languages
# Automatically runs all benchmark suites and outputs results.

Write-Host "=========================================================="
Write-Host "  JOSS MULTI-LANGUAGE REPRODUCIBLE BENCHMARK SUITE"
Write-Host "=========================================================="

Write-Host "`n[1/7] Running Sieve of Eratosthenes (Computation)..."
& powershell -ExecutionPolicy Bypass -File benchmarks\run_primes.ps1

Write-Host "`n[2/7] Running Quicksort (Algorithms & Recursion)..."
& powershell -ExecutionPolicy Bypass -File benchmarks\run_quicksort.ps1

Write-Host "`n[3/7] Running HashMap Ingestion & Query (Memory & Data Structures)..."
& powershell -ExecutionPolicy Bypass -File benchmarks\run_hashmap.ps1

Write-Host "`n[4/7] Running CSV Parsing & Stats (I/O & Aggregation)..."
& powershell -ExecutionPolicy Bypass -File benchmarks\run_csv.ps1

Write-Host "`n[5/7] Running Concurrency Workers (Parallelism & Channels)..."
& powershell -ExecutionPolicy Bypass -File benchmarks\run_concurrency.ps1

Write-Host "`n[6/7] Running Binary Trees (Memory Allocation, Pointers & GC)..."
& powershell -ExecutionPolicy Bypass -File benchmarks\run_binarytrees.ps1

Write-Host "`n[7/7] Running Mandelbrot Set (Scientific Floating-Point Vector Math)..."
& powershell -ExecutionPolicy Bypass -File benchmarks\run_mandelbrot.ps1

Write-Host "`n=========================================================="
Write-Host "  ALL 7 BENCHMARKS COMPLETED SUCCESSFULLY."
Write-Host "  Technical report available at: branchmark.md"
Write-Host "=========================================================="
