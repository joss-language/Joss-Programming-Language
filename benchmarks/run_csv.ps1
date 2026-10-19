# Benchmark runner for CSV I/O & Stats (50,000 records)
# 5 iterations per runtime

$iterations = 5

$benchmarks = @(
    @{ Name = "C (gcc -O3)"; Cmd = "benchmarks\io_csv_stats\c_csv.exe" },
    @{ Name = "C++ (g++ -O3)"; Cmd = "benchmarks\io_csv_stats\cpp_csv.exe" },
    @{ Name = "Rust (rustc -O)"; Cmd = "benchmarks\io_csv_stats\rust_csv.exe" },
    @{ Name = "Go (go build)"; Cmd = "benchmarks\io_csv_stats\go_csv.exe" },
    @{ Name = "Java 21 (JVM)"; Cmd = "java -cp benchmarks\io_csv_stats Main" },
    @{ Name = "Kotlin 2.4 (JVM)"; Cmd = "java -jar benchmarks\io_csv_stats\kt_csv.jar" },
    @{ Name = "Python 3.14"; Cmd = "python benchmarks\io_csv_stats\main.py" },
    @{ Name = "PHP 8.4"; Cmd = "php benchmarks\io_csv_stats\main.php" },
    @{ Name = "Joss (Interpreter)"; Cmd = "build\joss.exe run benchmarks\io_csv_stats\main.joss" }
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

Write-Host "`n=== RESULTADOS CSV I/O & STATS (50,000 FILAS) ==="
$results | Format-Table -AutoSize
$results | Export-Clixml -Path "benchmarks\io_csv_stats\results.xml"
