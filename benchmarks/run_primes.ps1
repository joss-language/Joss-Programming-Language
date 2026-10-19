# Benchmark runner for Sieve of Eratosthenes (N = 1,000,000)
# 5 iterations per runtime

$iterations = 5

$benchmarks = @(
    @{ Name = "C (gcc -O3)"; Cmd = "benchmarks\computation_primes\c_primes.exe 1000000" },
    @{ Name = "Rust (rustc -O)"; Cmd = "benchmarks\computation_primes\rust_primes.exe 1000000" },
    @{ Name = "Go (go build)"; Cmd = "benchmarks\computation_primes\go_primes.exe 1000000" },
    @{ Name = "C++ (g++ -O3)"; Cmd = "benchmarks\computation_primes\cpp_primes.exe 1000000" },
    @{ Name = "Python 3.14"; Cmd = "python benchmarks\computation_primes\main.py 1000000" },
    @{ Name = "Kotlin 2.4 (JVM)"; Cmd = "java -jar benchmarks\computation_primes\kt_primes.jar 1000000" },
    @{ Name = "Java 21 (JVM)"; Cmd = "java -cp benchmarks\computation_primes Main 1000000" },
    @{ Name = "PHP 8.4"; Cmd = "php benchmarks\computation_primes\main.php 1000000" },
    @{ Name = "Joss (Interpreter)"; Cmd = "build\joss.exe run benchmarks\computation_primes\main.joss 1000000" }
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

Write-Host "`n=== RESULTADOS SIEVE DE PRIMOS (N = 1,000,000) ==="
$results | Format-Table -AutoSize
