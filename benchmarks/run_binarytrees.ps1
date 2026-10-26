# Benchmark runner for Binary Trees (Max Depth = 14)
# 5 iterations per runtime

$iterations = 5

$benchmarks = @(
    @{ Name = "C (gcc -O3)"; Cmd = "benchmarks\memory_binarytrees\c_binarytrees.exe" },
    @{ Name = "C++ (g++ -O3)"; Cmd = "benchmarks\memory_binarytrees\cpp_binarytrees.exe" },
    @{ Name = "Rust (rustc -O)"; Cmd = "benchmarks\memory_binarytrees\rust_binarytrees.exe" },
    @{ Name = "Go (go build)"; Cmd = "benchmarks\memory_binarytrees\go_binarytrees.exe" },
    @{ Name = "Java 25 (JVM)"; Cmd = "java -cp benchmarks\memory_binarytrees Main" },
    @{ Name = "Kotlin 2.4 (JVM)"; Cmd = "java -jar benchmarks\memory_binarytrees\kt_binarytrees.jar" },
    @{ Name = "Python 3.14"; Cmd = "python benchmarks\memory_binarytrees\main.py" },
    @{ Name = "PHP 8.5"; Cmd = "php benchmarks\memory_binarytrees\main.php" },
    @{ Name = "Joss (Interpreter)"; Cmd = "build\joss.exe run benchmarks\memory_binarytrees\main.joss" }
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
        } else {
            Write-Host "Warning: could not parse output: $output"
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
            Raw = ($times -join ", ")
        }
    }
}

Write-Host "`n=== RESULTADOS BINARY TREES (DEPTH = 14) ==="
$results | Format-Table -AutoSize
$results | Export-Clixml -Path "benchmarks\memory_binarytrees\results.xml"
