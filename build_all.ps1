[CmdletBinding()]
param(
    [switch]$SkipVSCode
)

$ErrorActionPreference = 'Stop'
$root = $PSScriptRoot
$dist = Join-Path $root 'dist'
$work = Join-Path $root '.joss-release-work'

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
    $global:LASTEXITCODE = 0
    try {
        & $Command
    } catch {
        $failedWithException = $true
    }

    $exitCode = if ($null -ne $LASTEXITCODE) { $LASTEXITCODE } else { 0 }
    if ($failedWithException -or $exitCode -ne 0) {
        Write-Host "❌ Etapa fallida: $Label" -ForegroundColor Red
        Write-Host "Comando fallido: $resolvedCommand" -ForegroundColor Red
        Write-Host "Codigo de salida: $exitCode" -ForegroundColor Red
        throw "$Label termino con codigo $exitCode"
    }
}

function Copy-Required {
    param([string]$Source, [string]$Destination)
    if (-not (Test-Path -LiteralPath $Source -PathType Leaf)) {
        throw "No existe el archivo requerido: $Source"
    }
    Copy-Item -LiteralPath $Source -Destination $Destination -Force
}

function Compress-Directory {
    param([string]$Source, [string]$Destination)
    if (Test-Path -LiteralPath $Destination) {
        Remove-Item -LiteralPath $Destination -Force
    }
    $items = @(Get-ChildItem -LiteralPath $Source -Force)
    if ($items.Count -eq 0) { throw "No hay archivos para crear $Destination" }
    Compress-Archive -Path (Join-Path $Source '*') -DestinationPath $Destination -CompressionLevel Optimal
    Write-Host "Creado: $Destination" -ForegroundColor Green
}

function New-StagingDirectory {
    param([string]$Name)
    $path = Join-Path $work $Name
    New-Item -ItemType Directory -Force -Path $path | Out-Null
    return $path
}

function Remove-WorkDirectory {
    if (-not (Test-Path -LiteralPath $work)) { return }
    $resolvedRoot = (Resolve-Path -LiteralPath $root).Path
    $resolvedWork = (Resolve-Path -LiteralPath $work).Path
    if (-not $resolvedWork.StartsWith($resolvedRoot + [IO.Path]::DirectorySeparatorChar) -or
        (Split-Path $resolvedWork -Leaf) -ne '.joss-release-work') {
        throw "Ruta de limpieza insegura: $resolvedWork"
    }
    Remove-Item -LiteralPath $resolvedWork -Recurse -Force
}

Push-Location $root
try {
    & (Join-Path $root 'tools/verify-release.ps1')
    if ($LASTEXITCODE -ne 0) { throw 'La verificacion de release fallo' }

    Remove-WorkDirectory
    New-Item -ItemType Directory -Force -Path $work | Out-Null

    # El instalador historico soporta Linux ARMv7 ademas de los targets JP v2.
    $armv7Dir = Join-Path $dist 'linux-armv7'
    New-Item -ItemType Directory -Force -Path $armv7Dir | Out-Null
    $oldGOOS, $oldGOARCH, $oldGOARM, $oldCGO = $env:GOOS, $env:GOARCH, $env:GOARM, $env:CGO_ENABLED
    try {
        $env:GOOS, $env:GOARCH, $env:GOARM, $env:CGO_ENABLED = 'linux', 'arm', '7', '0'
        Invoke-Checked 'Joss linux-armv7' {
            go build -trimpath -o (Join-Path $armv7Dir 'joss') ./cmd/joss
        }
    } finally {
        $env:GOOS, $env:GOARCH, $env:GOARM, $env:CGO_ENABLED = $oldGOOS, $oldGOARCH, $oldGOARM, $oldCGO
    }

    $versionSource = Get-Content (Join-Path $root 'pkg/version/version.go') -Raw
    if ($versionSource -notmatch 'const Version = "([^"]+)"') {
        throw 'No se pudo obtener la version de pkg/version/version.go'
    }
    $releaseVersion = $Matches[1]

    $windows = New-StagingDirectory 'windows'
    Copy-Required (Join-Path $dist 'windows-amd64/joss.exe') (Join-Path $windows 'joss.exe')
    Copy-Required (Join-Path $dist 'windows-arm64/joss.exe') (Join-Path $windows 'joss-windows-arm64.exe')
    Copy-Required (Join-Path $root 'install/remote-install.ps1') (Join-Path $windows 'remote-install.ps1')
    Copy-Required (Join-Path $root 'LICENSE') (Join-Path $windows 'LICENSE')

    $linux = New-StagingDirectory 'linux'
    Copy-Required (Join-Path $dist 'linux-amd64/joss') (Join-Path $linux 'joss-linux-amd64')
    Copy-Required (Join-Path $dist 'linux-arm64/joss') (Join-Path $linux 'joss-linux-arm64')
    Copy-Required (Join-Path $dist 'linux-armv7/joss') (Join-Path $linux 'joss-linux-armv7')
    Copy-Required (Join-Path $root 'install/remote-install.sh') (Join-Path $linux 'remote-install.sh')
    Copy-Required (Join-Path $root 'LICENSE') (Join-Path $linux 'LICENSE')

    $macos = New-StagingDirectory 'macos'
    Copy-Required (Join-Path $dist 'darwin-amd64/joss') (Join-Path $macos 'joss-macos-amd64')
    Copy-Required (Join-Path $dist 'darwin-arm64/joss') (Join-Path $macos 'joss-macos-arm64')
    Copy-Required (Join-Path $root 'install/remote-install.sh') (Join-Path $macos 'remote-install.sh')
    Copy-Required (Join-Path $root 'LICENSE') (Join-Path $macos 'LICENSE')

    $android = New-StagingDirectory 'android'
    Copy-Required (Join-Path $dist 'android-arm64/joss') (Join-Path $android 'joss-android-arm64')
    Copy-Required (Join-Path $dist 'linux-armv7/joss') (Join-Path $android 'joss-android-armv7')
    Copy-Required (Join-Path $root 'LICENSE') (Join-Path $android 'LICENSE')
    $androidReadme = @"
Joss para Android (CLI / Termux)
================================
Instalacion en Termux:
1. Copia el binario correspondiente a tu dispositivo (ej. joss-android-arm64).
2. Dale permisos de ejecucion e instalalo en el PATH de Termux:
   chmod +x joss-android-arm64
   cp joss-android-arm64 `$PREFIX/bin/joss
3. Prueba la instalacion:
   joss version
"@
    [IO.File]::WriteAllText((Join-Path $android 'README-ANDROID.txt'), $androidReadme, [Text.Encoding]::UTF8)

    $mobileSdk = New-StagingDirectory 'mobile-sdk'
    $mobileAndroid = Join-Path $mobileSdk 'android'
    $mobileDesktop = Join-Path $mobileSdk 'desktop'
    $mobileDart = Join-Path $mobileSdk 'dart'
    New-Item -ItemType Directory -Force -Path $mobileAndroid, $mobileDesktop, $mobileDart | Out-Null

    Copy-Required (Join-Path $dist 'android-arm64/joss') (Join-Path $mobileAndroid 'joss-android-arm64')
    Copy-Required (Join-Path $dist 'linux-armv7/joss') (Join-Path $mobileAndroid 'joss-android-armv7')
    Copy-Required (Join-Path $dist 'windows-amd64/joss.exe') (Join-Path $mobileDesktop 'joss-windows-amd64.exe')
    Copy-Required (Join-Path $dist 'linux-amd64/joss') (Join-Path $mobileDesktop 'joss-linux-amd64')
    Copy-Required (Join-Path $dist 'darwin-arm64/joss') (Join-Path $mobileDesktop 'joss-macos-arm64')
    Copy-Required (Join-Path $root 'sdk/dart/lib/joss.dart') (Join-Path $mobileDart 'joss.dart')
    Copy-Required (Join-Path $root 'LICENSE') (Join-Path $mobileSdk 'LICENSE')

    $mobileSdkReadme = @"
Joss Mobile SDK
===============
Version: $releaseVersion

Paquete para integracion en Flutter (Dart) y Android.
Carpetas:
- android/: Binarios arm64 y armv7 para Android.
- desktop/: Binarios para pruebas locales en Windows, Linux y macOS.
- dart/: Conector para apps Dart y Flutter.
"@
    [IO.File]::WriteAllText((Join-Path $mobileSdk 'README.txt'), $mobileSdkReadme, [Text.Encoding]::UTF8)

    Compress-Directory $windows (Join-Path $dist 'jossecurity-windows.zip')
    Compress-Directory $linux (Join-Path $dist 'jossecurity-linux.zip')
    Compress-Directory $macos (Join-Path $dist 'jossecurity-macos.zip')
    Compress-Directory $android (Join-Path $dist 'jossecurity-android.zip')
    Compress-Directory $mobileSdk (Join-Path $dist 'jossecurity-mobile-sdk.zip')

    if (-not $SkipVSCode) {
        $npm = Get-Command npm.cmd -ErrorAction SilentlyContinue
        if (-not $npm) { $npm = Get-Command npm -ErrorAction SilentlyContinue }
        if (-not $npm) { throw 'Falta npm para compilar la extension de VS Code' }
        $extensionPackage = Get-Content (Join-Path $root 'vscode-joss/package.json') -Raw | ConvertFrom-Json
        if ([string]::IsNullOrWhiteSpace($extensionPackage.version)) {
            throw 'vscode-joss/package.json no declara una version valida'
        }
        $vsixPath = Join-Path $work "joss-language-$($extensionPackage.version).vsix"
        Push-Location (Join-Path $root 'vscode-joss')
        try {
            Invoke-Checked 'Dependencias de VS Code' { cmd.exe /c npm ci }
            Invoke-Checked 'Auditoria de dependencias VS Code' { cmd.exe /c npm audit --audit-level=critical }
            Invoke-Checked 'Compilacion IntelliSense VS Code' { cmd.exe /c npm run compile }
            Invoke-Checked 'Extension VS Code' { cmd.exe /c npm run package -- --out $vsixPath }
        } finally {
            Pop-Location
        }
        $vscode = New-StagingDirectory 'vscode'
        Copy-Required $vsixPath (Join-Path $vscode (Split-Path $vsixPath -Leaf))
        Compress-Directory $vscode (Join-Path $dist 'jossecurity-vscode.zip')
    }



    # Solo se publican archivos finales; los binarios intermedios se quitan de dist.
    Get-ChildItem -LiteralPath $dist -Directory | Remove-Item -Recurse -Force
    $hostBinary = Join-Path $dist $(if ($env:OS -eq 'Windows_NT') { 'joss.exe' } else { 'joss' })
    if (Test-Path -LiteralPath $hostBinary) { Remove-Item -LiteralPath $hostBinary -Force }

    $artifactFiles = @(Get-ChildItem -LiteralPath $dist -File | Sort-Object Name)
    $manifestArtifacts = foreach ($file in $artifactFiles) {
        [ordered]@{
            name = $file.Name
            bytes = $file.Length
            sha256 = (Get-FileHash -LiteralPath $file.FullName -Algorithm SHA256).Hash.ToLowerInvariant()
        }
    }
    $manifest = [ordered]@{
        schema = 1
        product = 'Joss'
        version = $releaseVersion
        generated_at = [DateTime]::UtcNow.ToString('o')
        artifacts = @($manifestArtifacts)
    }
    $utf8 = New-Object Text.UTF8Encoding($false)
    [IO.File]::WriteAllText(
        (Join-Path $dist 'release-manifest.json'),
        ($manifest | ConvertTo-Json -Depth 5),
        $utf8
    )

    $checksumLines = foreach ($file in (Get-ChildItem -LiteralPath $dist -File | Sort-Object Name)) {
        $hash = (Get-FileHash -LiteralPath $file.FullName -Algorithm SHA256).Hash.ToLowerInvariant()
        "$hash  $($file.Name)"
    }
    [IO.File]::WriteAllText(
        (Join-Path $dist 'SHA256SUMS.txt'),
        (($checksumLines -join "`n") + "`n"),
        $utf8
    )

    Write-Host "Distribucion Joss v$releaseVersion preparada en $dist" -ForegroundColor Green
    Get-ChildItem -LiteralPath $dist -File | Sort-Object Name | Format-Table Name, Length
} finally {
    Pop-Location
    Remove-WorkDirectory
}
