# Benchmark runner for Quicksort (N = 50,000)
# 5 iterations per runtime

$iterations = 5

$benchmarks = @(
    @{ Name = "C (gcc -O3)"; Cmd = "benchmarks\algo_quicksort\c_quicksort.exe" },
    @{ Name = "C++ (g++ -O3)"; Cmd = "benchmarks\algo_quicksort\cpp_quicksort.exe" },
    @{ Name = "Rust (rustc -O)"; Cmd = "benchmarks\algo_quicksort\rust_quicksort.exe" },
    @{ Name = "Go (go build)"; Cmd = "benchmarks\algo_quicksort\go_quicksort.exe" },
    @{ Name = "Joss (Native AOT)"; Cmd = "build\qs_pure.exe" },
    @{ Name = "Java 25 (JVM)"; Cmd = "java -cp benchmarks\algo_quicksort Main" },
    @{ Name = "Kotlin 2.4 (JVM)"; Cmd = "java -jar benchmarks\algo_quicksort\kt_quicksort.jar" },
    @{ Name = "PHP 8.5"; Cmd = "php benchmarks\algo_quicksort\main.php" },
    @{ Name = "Python 3.14"; Cmd = "python benchmarks\algo_quicksort\main.py" },
    @{ Name = "Joss (Interpreter)"; Cmd = "build\joss.exe run benchmarks\algo_quicksort\main.joss" }
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

Write-Host "`n=== RESULTADOS QUICKSORT (N = 50,000) ==="
$results | Format-Table -AutoSize
$results | Export-Clixml -Path "benchmarks\algo_quicksort\results.xml"
