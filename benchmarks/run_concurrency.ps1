# Benchmark runner for Concurrency Workers (8 workers, 800 tasks, 400,000 inner computations)
# 5 iterations per runtime

$iterations = 5

$benchmarks = @(
    @{ Name = "C (gcc -O3 Win32 Threads)"; Cmd = "benchmarks\concurrency_workers\c_concurrency.exe" },
    @{ Name = "C++ (g++ -O3 std::thread)"; Cmd = "benchmarks\concurrency_workers\cpp_concurrency.exe" },
    @{ Name = "Rust (rustc -O mpsc/thread)"; Cmd = "benchmarks\concurrency_workers\rust_concurrency.exe" },
    @{ Name = "Go (go build Goroutines)"; Cmd = "benchmarks\concurrency_workers\go_concurrency.exe" },
    @{ Name = "Java 21 (JVM ExecutorService)"; Cmd = "java -cp benchmarks\concurrency_workers Main" },
    @{ Name = "Kotlin 2.4 (JVM ExecutorService)"; Cmd = "java -jar benchmarks\concurrency_workers\kt_concurrency.jar" },
    @{ Name = "Python 3.14 (ThreadPoolExecutor)"; Cmd = "python benchmarks\concurrency_workers\main.py" },
    @{ Name = "PHP 8.5 (Sequential CLI)"; Cmd = "php benchmarks\concurrency_workers\main.php" },
    @{ Name = "Joss (Interpreter Async/Await)"; Cmd = "build\joss.exe run benchmarks\concurrency_workers\main.joss" }
)

$results = @()

foreach ($bench in $benchmarks) {
    Write-Host "Running $($bench.Name)..."
    $times = @()
    for ($i = 1; $i -le $iterations; $i++) {
        $output = Invoke-Expression $bench.Cmd
        $match = $output | Select-String -Pattern "in ([\d\.]+) ms"
        if ($match) {
            $ms = [double]$match.Matches[0].Groups[1].Value
            $times += $ms
        }
    }
    if ($times.Count -gt 0) {
        $avg = ($times | Measure-Object -Average).Average
        $min = ($times | Measure-Object -Minimum).Minimum
        $max = ($times | Measure-Object -Maximum).Maximum
        $results += [PSCustomObject]@{
            Language = $bench.Name
            AvgMs = [Math]::Round($avg, 2)
            MinMs = [Math]::Round($min, 2)
            MaxMs = [Math]::Round($max, 2)
        }
    }
}

Write-Host "`n=== RESULTADOS CONCURRENCIA (8 WORKERS, 800 TAREAS) ==="
$results | Format-Table -AutoSize
$results | Export-Clixml -Path "benchmarks\concurrency_workers\results.xml"
