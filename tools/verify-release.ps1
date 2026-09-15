$ErrorActionPreference = 'Stop'
$root = (Resolve-Path (Join-Path $PSScriptRoot '..')).Path
$work = Join-Path $root '.joss-release-work'
$dist = Join-Path $root 'dist'

function Invoke-Checked {
    param(
        [string]$Label,
        [scriptblock]$Command,
        [string]$CommandText
    )

    $resolvedCommand = if ([string]::IsNullOrWhiteSpace($CommandText)) {
        $Command.ToString().Trim()
    } else {
        $CommandText
    }

    Write-Host "==> $Label" -ForegroundColor Cyan
    Write-Host "Comando: $resolvedCommand" -ForegroundColor DarkCyan

    $capturedOutput = @()
    $failedWithException = $false
    try {
        & $Command 2>&1 | Tee-Object -Variable capturedOutput
    } catch {
        $failedWithException = $true
        $capturedOutput = @($capturedOutput) + @($_)
    }

    $exitCode = if ($null -ne $LASTEXITCODE) { $LASTEXITCODE } else { 0 }
    if ($failedWithException -or $exitCode -ne 0) {
        Write-Host "❌ Etapa fallida: $Label" -ForegroundColor Red
        Write-Host "Comando fallido: $resolvedCommand" -ForegroundColor Red
        Write-Host "Codigo de salida: $exitCode" -ForegroundColor Red
        if ($capturedOutput.Count -gt 0) {
            Write-Host "Salida capturada (stdout/stderr):" -ForegroundColor Yellow
            $capturedOutput | ForEach-Object { Write-Host $_ }
        }
        throw "$Label termino con codigo $exitCode"
    }
}

function Find-Executable {
    param([string[]]$Names, [string[]]$Fallbacks = @())
    foreach ($name in $Names) {
        $command = Get-Command $name -ErrorAction SilentlyContinue
        if ($command) { return $command.Source }
    }
    $paths = @(
        [Environment]::GetEnvironmentVariable('Path', 'Process'),
        [Environment]::GetEnvironmentVariable('Path', 'User'),
        [Environment]::GetEnvironmentVariable('Path', 'Machine')
    ) -join [IO.Path]::PathSeparator
    foreach ($directory in ($paths -split [IO.Path]::PathSeparator | Where-Object { $_ })) {
        foreach ($name in $Names) {
            $candidate = Join-Path $directory $name
            if (Test-Path -LiteralPath $candidate -PathType Leaf) { return $candidate }
        }
    }
    foreach ($candidate in $Fallbacks) {
        if (Test-Path -LiteralPath $candidate -PathType Leaf) { return $candidate }
    }
    return $null
}

function Remove-ReleaseWork {
    if (-not (Test-Path -LiteralPath $work)) { return }
    $resolved = (Resolve-Path -LiteralPath $work).Path
    if (-not $resolved.StartsWith($root + [IO.Path]::DirectorySeparatorChar) -or
        (Split-Path $resolved -Leaf) -ne '.joss-release-work') {
        throw "Ruta de limpieza insegura: $resolved"
    }
    Remove-Item -LiteralPath $resolved -Recurse -Force
}

function Remove-DistOutput {
    if (-not (Test-Path -LiteralPath $dist)) { return }
    $resolved = (Resolve-Path -LiteralPath $dist).Path
    if (-not $resolved.StartsWith($root + [IO.Path]::DirectorySeparatorChar) -or
        (Split-Path $resolved -Leaf) -ne 'dist') {
        throw "Ruta de salida insegura: $resolved"
    }
    Remove-Item -LiteralPath $resolved -Recurse -Force
}

Push-Location $root
try {
    Remove-ReleaseWork
    Remove-DistOutput
    New-Item -ItemType Directory -Force -Path $work, $dist | Out-Null

    $runnerPath = Join-Path $root 'cmd/joss/runner_windows.exe'
    $oldGOOS, $oldGOARCH, $oldCGO = $env:GOOS, $env:GOARCH, $env:CGO_ENABLED
    try {
        $env:GOOS, $env:GOARCH, $env:CGO_ENABLED = 'windows', 'amd64', '0'
        Invoke-Checked 'Runner Windows embebido' {
            go build -trimpath -o $runnerPath ./cmd/runner
        }
    } finally {
        $env:GOOS, $env:GOARCH, $env:CGO_ENABLED = $oldGOOS, $oldGOARCH, $oldCGO
    }

    Invoke-Checked 'Tests completos de Joss' { go test ./... }

    $jossName = if ($IsWindows -or $env:OS -eq 'Windows_NT') { 'joss.exe' } else { 'joss' }
    $jossBinary = Join-Path $dist $jossName
    Invoke-Checked 'Compilacion del CLI Joss' { go build -trimpath -o $jossBinary ./cmd/joss }
    & $jossBinary version
    if ($LASTEXITCODE -ne 0) { throw 'El binario Joss compilado no inicia' }

    $releaseTargets = @(
        @('windows', 'amd64'), @('windows', 'arm64'),
        @('linux', 'amd64'), @('linux', 'arm64'),
        @('darwin', 'amd64'), @('darwin', 'arm64'),
        @('android', 'arm64')
    )
    foreach ($target in $releaseTargets) {
        $goos, $goarch = $target
        $targetDir = Join-Path $dist "$goos-$goarch"
        New-Item -ItemType Directory -Force -Path $targetDir | Out-Null
        $targetName = if ($goos -eq 'windows') { 'joss.exe' } else { 'joss' }
        $oldGOOS, $oldGOARCH, $oldCGO = $env:GOOS, $env:GOARCH, $env:CGO_ENABLED
        try {
            $env:GOOS, $env:GOARCH, $env:CGO_ENABLED = $goos, $goarch, '0'
            Invoke-Checked "Joss $goos-$goarch" {
                go build -trimpath -o (Join-Path $targetDir $targetName) ./cmd/joss
            }
        } finally {
            $env:GOOS, $env:GOARCH, $env:CGO_ENABLED = $oldGOOS, $oldGOARCH, $oldCGO
        }
    }

    Write-Host "Release verificada. Binario: $jossBinary" -ForegroundColor Green
} finally {
    Pop-Location
    Remove-ReleaseWork
}
