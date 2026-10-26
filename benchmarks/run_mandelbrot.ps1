# Benchmark 8: Mandelbrot Set (Scientific Floating-Point Vector Math)
# Evaluates 9 languages on N=500x500 complex numbers grid, max 50 iters.

$iterations = 5
$results = @()

function Measure-Exec {
    param([string]$name, [scriptblock]$action)
    Write-Host "Running $name..."
    $times = @()
    # Warm-up
    & $action | Out-Null
    for ($i = 0; $i -lt $iterations; $i++) {
        $sw = [System.Diagnostics.Stopwatch]::StartNew()
        $out = & $action
        $sw.Stop()
        # Parse time if output contains it, otherwise use stopwatch
        if ($out -match 'time=([0-9\.]+)ms') {
            $times += [double]$matches[1]
        } else {
            $times += $sw.Elapsed.TotalMilliseconds
        }
    }
    $avg = ($times | Measure-Object -Average).Average
    $min = ($times | Measure-Object -Minimum).Minimum
    $max = ($times | Measure-Object -Maximum).Maximum
    return [PSCustomObject]@{
        Language = $name
        AvgMs    = [Math]::Round($avg, 2)
        MinMs    = [Math]::Round($min, 2)
        MaxMs    = [Math]::Round($max, 2)
        Raw      = ($times -join ", ")
    }
}

$results += Measure-Exec "C (gcc -O3)" { .\benchmarks\computation_mandelbrot\c_mandelbrot.exe }
$results += Measure-Exec "C++ (g++ -O3)" { .\benchmarks\computation_mandelbrot\cpp_mandelbrot.exe }
$results += Measure-Exec "Rust (rustc -O)" { .\benchmarks\computation_mandelbrot\rust_mandelbrot.exe }
$results += Measure-Exec "Go (go build)" { .\benchmarks\computation_mandelbrot\go_mandelbrot.exe }
$results += Measure-Exec "Java 25 (JVM)" { java -cp benchmarks\computation_mandelbrot Main }
$results += Measure-Exec "Kotlin 2.4 (JVM)" { java -jar benchmarks\computation_mandelbrot\kt_mandelbrot.jar }
$results += Measure-Exec "Joss (JIT In-Memory)" { joss run benchmarks\computation_mandelbrot\main.joss }
$results += Measure-Exec "PHP 8.5" { php benchmarks\computation_mandelbrot\main.php }
$results += Measure-Exec "Python 3.14" { python benchmarks\computation_mandelbrot\main.py }

Write-Host "`n=== RESULTADOS MANDELBROT SET (N = 500x500, MAX_ITER = 50) ==="
$results | Format-Table -AutoSize
