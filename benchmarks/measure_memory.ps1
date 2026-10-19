$benchmarks = @(
    @{ Name = "C (gcc -O3)"; File = "benchmarks\memory_hashmap\c_hashmap.exe"; Args = "" },
    @{ Name = "C++ (g++ -O3)"; File = "benchmarks\memory_hashmap\cpp_hashmap.exe"; Args = "" },
    @{ Name = "Rust (rustc -O)"; File = "benchmarks\memory_hashmap\rust_hashmap.exe"; Args = "" },
    @{ Name = "Go (go build)"; File = "benchmarks\memory_hashmap\go_hashmap.exe"; Args = "" },
    @{ Name = "Java 21 (JVM)"; File = "java"; Args = "-cp benchmarks\memory_hashmap Main" },
    @{ Name = "Kotlin 2.4 (JVM)"; File = "java"; Args = "-jar benchmarks\memory_hashmap\kt_hashmap.jar" },
    @{ Name = "Python 3.14"; File = "python"; Args = "benchmarks\memory_hashmap\main.py" },
    @{ Name = "PHP 8.4"; File = "php"; Args = "benchmarks\memory_hashmap\main.php" },
    @{ Name = "Joss (Interpreter)"; File = "build\joss.exe"; Args = "run benchmarks\memory_hashmap\main.joss" }
)

$memResults = @()

foreach ($bench in $benchmarks) {
    $pinfo = New-Object System.Diagnostics.ProcessStartInfo
    $pinfo.FileName = $bench.File
    $pinfo.Arguments = $bench.Args
    $pinfo.RedirectStandardOutput = $true
    $pinfo.RedirectStandardError = $true
    $pinfo.UseShellExecute = $false
    $p = [System.Diagnostics.Process]::Start($pinfo)
    $maxWorkingSet = 0
    while (!$p.HasExited) {
        $p.Refresh()
        try {
            $ws = $p.WorkingSet64
            if ($ws -gt $maxWorkingSet) {
                $maxWorkingSet = $ws
            }
        } catch {}
        Start-Sleep -Milliseconds 5
    }
    $memMB = [Math]::Round($maxWorkingSet / 1MB, 2)
    $memResults += [PSCustomObject]@{
        Language = $bench.Name
        PeakMemoryMB = $memMB
    }
}

Write-Host "`n=== MEMORIA PICO (PEAK WORKING SET - 50,000 HASHMAP ITEMS) ==="
$memResults | Format-Table -AutoSize
