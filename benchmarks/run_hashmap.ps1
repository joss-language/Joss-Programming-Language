# Benchmark runner for HashMap (N = 50,000 items)
# 5 iterations per runtime + peak memory measurement

$iterations = 5

$benchmarks = @(
    @{ Name = "C (gcc -O3)"; Cmd = "benchmarks\memory_hashmap\c_hashmap.exe" },
    @{ Name = "C++ (g++ -O3)"; Cmd = "benchmarks\memory_hashmap\cpp_hashmap.exe" },
    @{ Name = "Rust (rustc -O)"; Cmd = "benchmarks\memory_hashmap\rust_hashmap.exe" },
    @{ Name = "Go (go build)"; Cmd = "benchmarks\memory_hashmap\go_hashmap.exe" },
    @{ Name = "Java 25 (JVM)"; Cmd = "java -cp benchmarks\memory_hashmap Main" },
    @{ Name = "Kotlin 2.4 (JVM)"; Cmd = "java -jar benchmarks\memory_hashmap\kt_hashmap.jar" },
    @{ Name = "Python 3.14"; Cmd = "python benchmarks\memory_hashmap\main.py" },
    @{ Name = "PHP 8.5"; Cmd = "php benchmarks\memory_hashmap\main.php" },
    @{ Name = "Joss (Interpreter)"; Cmd = "build\joss.exe run benchmarks\memory_hashmap\main.joss" }
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

Write-Host "`n=== RESULTADOS HASHMAP (N = 50,000 items) ==="
$results | Format-Table -AutoSize
$results | Export-Clixml -Path "benchmarks\memory_hashmap\results.xml"
