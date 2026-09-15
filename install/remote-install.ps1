# -----------------------------------------------------------
# JosSecurity Remote Installer, Updater, and Uninstaller (Windows PowerShell)
# Usos:
#   Instalación: iwr -useb <URL_DE_ESTE_SCRIPT> | iex
#   Ejecución manual: .\remote-install.ps1
# -----------------------------------------------------------

$ErrorActionPreference = "Stop"
$Host.UI.RawUI.ForegroundColor = "White"

# --- CONFIGURACIÓN ---
$RepoOwner = "joss-language"
$RepoName = "Joss-Programming-Language"

# Rutas
$InstallDir = "C:\Program Files\JosSecurity"
$LegacySdkInstallDir = "$InstallDir\sdk"
$LogFile = "$env:TEMP\jossecurity-action.log"
$TempDir = "$env:TEMP\jossecurity-temp-action"
$RepoUrl = "https://api.github.com/repos/$RepoOwner/$RepoName/releases/latest"
# --------------------

# --- FUNCIONES DE LOGGING/UTILIDADES ---
function Write-Log {
    param([string]$Message, [string]$Level = "INFO")
    $timestamp = Get-Date -Format "yyyy-MM-dd HH:mm:ss"
    $logMessage = "[$timestamp] [$Level] $Message"
    Add-Content -Path $LogFile -Value $logMessage
    
    switch ($Level) {
        "ERROR" { Write-Host $Message -ForegroundColor Red }
        "SUCCESS" { Write-Host $Message -ForegroundColor Green }
        "WARNING" { Write-Host $Message -ForegroundColor Yellow }
        default { Write-Host $Message }
    }
}

function Test-Administrator {
    $currentUser = [Security.Principal.WindowsIdentity]::GetCurrent()
    $principal = New-Object Security.Principal.WindowsPrincipal($currentUser)
    return $principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
}

function Get-VSCodeCommand {
    foreach ($name in @("code.cmd", "code")) {
        $command = Get-Command $name -ErrorAction SilentlyContinue | Select-Object -First 1
        if ($command) { return $command.Source }
    }

    $candidates = @(
        (Join-Path $env:LOCALAPPDATA "Programs\Microsoft VS Code\bin\code.cmd"),
        (Join-Path $env:ProgramFiles "Microsoft VS Code\bin\code.cmd")
    )
    foreach ($candidate in $candidates) {
        if (Test-Path -LiteralPath $candidate -PathType Leaf) { return $candidate }
    }
    return $null
}

function Test-VSCode {
    return $null -ne (Get-VSCodeCommand)
}

function Invoke-VSCode {
    param([Parameter(Mandatory = $true)][string[]]$Arguments)

    $command = Get-VSCodeCommand
    if (-not $command) {
        return [pscustomobject]@{ ExitCode = 127; Output = @("VS Code CLI was not found") }
    }

    # VS Code/Node can emit deprecation warnings on stderr with exit code 0.
    $previousErrorAction = $ErrorActionPreference
    $nativePreferenceExists = $null -ne (Get-Variable PSNativeCommandUseErrorActionPreference -ErrorAction SilentlyContinue)
    if ($nativePreferenceExists) { $previousNativePreference = $PSNativeCommandUseErrorActionPreference }
    try {
        $ErrorActionPreference = "Continue"
        if ($nativePreferenceExists) { $PSNativeCommandUseErrorActionPreference = $false }
        $output = @(& $command @Arguments 2>&1 | ForEach-Object { $_.ToString() })
        $exitCode = $LASTEXITCODE
    } finally {
        $ErrorActionPreference = $previousErrorAction
        if ($nativePreferenceExists) { $PSNativeCommandUseErrorActionPreference = $previousNativePreference }
    }

    return [pscustomobject]@{ ExitCode = $exitCode; Output = $output }
}

function Add-ToPath {
    param([string]$PathToAdd)
    try {
        $currentPath = [Environment]::GetEnvironmentVariable("Path", "Machine")
        if ($currentPath -notlike "*$PathToAdd*") {
            Write-Log "Adding $PathToAdd to system PATH..." "INFO"
            if (Test-Administrator) {
                [Environment]::SetEnvironmentVariable("Path", "$currentPath;$PathToAdd", "Machine")
                Write-Log "[OK] PATH updated successfully (Requires restart)" "SUCCESS"
                return $true
            } else {
                Write-Log "[X] Administrator permissions required to modify PATH" "ERROR"
                return $false
            }
        } else {
            Write-Log "[OK] Already in PATH" "SUCCESS"
            return $true
        }
    } catch {
        Write-Log "[X] Error updating PATH: $($_.Exception.Message)" "ERROR"
        return $false
    }
}
# --------------------

# --- FUNCIONES DE ACCIÓN ---

# 1. Instalación
function Install-JosSecurity {
    Write-Log "[1/2] Installing Joss runtime..." "INFO"
    
    # 1. Check Administrator Privileges (Enforced)
    if (-not (Test-Administrator)) {
        Write-Log "[X] Error: Administrator privileges are required to install to Program Files." "ERROR"
        Write-Log "    Please restart PowerShell as Administrator and run the command again." "WARNING"
        return $false
    }

    try {
        # 2. Stop running instances
        $running = Get-Process "joss" -ErrorAction SilentlyContinue
        if ($running) {
             Write-Log "Stopping running joss.exe processes..." "INFO"
             $running | Stop-Process -Force
             Start-Sleep -Seconds 2
        }

        # 3. Prepare Directory
        if (-not (Test-Path $InstallDir)) {
            New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
        }
        if (Test-Path -LiteralPath $LegacySdkInstallDir) {
            Remove-Item -LiteralPath $LegacySdkInstallDir -Recurse -Force
            Write-Log "[OK] Legacy SDK removed" "SUCCESS"
        }
        
        # El binario de Windows se llama siempre joss.exe en el ZIP.
        $binaryPath = Get-ChildItem -Path $TempDir -Filter "joss.exe" -Recurse -ErrorAction SilentlyContinue | Select-Object -First 1
        
        if ($null -eq $binaryPath) {
            Write-Log "[X] joss.exe not found in temp directory" "ERROR"
            return $false
        }
        
        # 4. Remove old file explicitly (helps with some lock cases/replacements)
        if (Test-Path "$InstallDir\joss.exe") {
            Remove-Item "$InstallDir\joss.exe" -Force -ErrorAction SilentlyContinue
        }

        Copy-Item -Path $binaryPath.FullName -Destination "$InstallDir\joss.exe" -Force
        
        if (Test-Path "$InstallDir\joss.exe") {
            Write-Log "[OK] Binary installed" "SUCCESS"
            if (-not (Add-ToPath -PathToAdd $InstallDir)) { return $false }
        } else {
            Write-Log "[X] Failed to copy binary" "ERROR"
            return $false
        }
        
        Write-Log "[OK] JosSecurity installed" "SUCCESS"
        return $true
    } catch {
        Write-Log "[X] Installation failed: $($_.Exception.Message)" "ERROR"
        return $false
    }
}

function Install-Extension {
    Write-Log "[2/2] Installing VS Code extension..." "INFO"
    
    if (-not (Test-VSCode)) {
        Write-Log "[X] VS Code not detected. Skipping extension install." "WARNING"
        return $true
    }

    try {
        # Buscar CUALQUIER archivo VSIX en el temp
        $vsixFile = Get-ChildItem -Path $TempDir -Filter "*.vsix" -Recurse -ErrorAction SilentlyContinue | Select-Object -First 1
        
        if ($null -eq $vsixFile) {
            Write-Log "[X] VSIX file not found in temp directory" "ERROR"
            return $false
        }
        
        Write-Log "Found VSIX: $($vsixFile.Name)" "INFO"
        
        # Usar --force para asegurar la instalación/actualización
        $installResult = Invoke-VSCode -Arguments @("--install-extension", $vsixFile.FullName, "--force")
        if ($installResult.ExitCode -ne 0) {
            $detail = ($installResult.Output -join " ").Trim()
            Write-Log "[X] Extension installation failed (exit $($installResult.ExitCode)): $detail" "ERROR"
            return $false
        }
        
        # Verificación simple (asume que la extensión se llama 'joss-language')
        $listResult = Invoke-VSCode -Arguments @("--list-extensions")
        $extensionIds = @($listResult.Output | ForEach-Object { $_.Trim().ToLowerInvariant() })
        if ($listResult.ExitCode -eq 0 -and $extensionIds -contains "jossecurity.joss-language") {
            Write-Log "[OK] Extension installed successfully" "SUCCESS"
            return $true
        } else {
            Write-Log "[X] Extension installation could not be verified" "WARNING"
            return $false
        }
    } catch {
        Write-Log "[X] Extension installation failed: $($_.Exception.Message)" "ERROR"
        return $false
    }
}

# 2. Desinstalación
function Uninstall-JosSecurity {
    Write-Log "Uninstalling JosSecurity..." "INFO"
    try {
        if (Test-Path "$InstallDir\joss.exe") {
            Remove-Item "$InstallDir\joss.exe" -Force
            Write-Log "[OK] Binary removed" "SUCCESS"
        }

        # Clean installations made by releases that still shipped the legacy SDK.
        if (Test-Path -LiteralPath $LegacySdkInstallDir) {
            Remove-Item -LiteralPath $LegacySdkInstallDir -Recurse -Force
            Write-Log "[OK] Legacy SDK removed" "SUCCESS"
        }
        
        # Remover directorio solo si está vacío
        if ((Test-Path -LiteralPath $InstallDir) -and (Get-ChildItem -LiteralPath $InstallDir -Force | Measure-Object).Count -eq 0) {
            Remove-Item $InstallDir -Recurse -Force
            Write-Log "[OK] Installation directory removed" "SUCCESS"
        }
        
        # Remover de PATH (requiere Admin)
        $currentPath = [Environment]::GetEnvironmentVariable("Path", "Machine")
        if ($currentPath -like "*$InstallDir*") {
            if (Test-Administrator) {
                 $newPath = $currentPath -replace [regex]::Escape(";$InstallDir"), ""
                 $newPath = $newPath -replace [regex]::Escape("$InstallDir;"), ""
                 [Environment]::SetEnvironmentVariable("Path", $newPath, "Machine")
                 Write-Log "[OK] Removed from PATH" "SUCCESS"
            } else {
                Write-Log "[WARNING] Could not remove from PATH (Admin required)" "WARNING"
            }
        }
        Write-Log "[OK] JosSecurity uninstalled" "SUCCESS"
        return $true
    } catch {
        Write-Log "[X] Error: $($_.Exception.Message)" "ERROR"
        return $false
    }
}

function Uninstall-Extension {
    Write-Log "Uninstalling VS Code extension..." "INFO"
    try {
        if (Test-VSCode) {
            $listResult = Invoke-VSCode -Arguments @("--list-extensions")
            $extensionIds = @($listResult.Output | ForEach-Object { $_.Trim().ToLowerInvariant() })
            if ($listResult.ExitCode -eq 0 -and $extensionIds -contains "jossecurity.joss-language") {
                $uninstallResult = Invoke-VSCode -Arguments @("--uninstall-extension", "jossecurity.joss-language")
                if ($uninstallResult.ExitCode -ne 0) {
                    $detail = ($uninstallResult.Output -join " ").Trim()
                    Write-Log "[X] Extension uninstall failed (exit $($uninstallResult.ExitCode)): $detail" "ERROR"
                    return $false
                }
                Write-Log "[OK] Extension uninstalled" "SUCCESS"
                return $true
            }
        }
        Write-Log "[OK] Extension not found or already removed" "INFO"
        return $true
    } catch {
        Write-Log "[X] Error: $($_.Exception.Message)" "ERROR"
        return $false
    }
}

# 3. Actualización
function Test-Update {
    Write-Log "Checking for updates..." "INFO"
    
    $LocalVersion = "0.0.0"
    $InstalledBinary = "$InstallDir\joss.exe"
    
    if (Test-Path $InstalledBinary) {
        try {
            $versionOutput = & $InstalledBinary version 2>&1
            if ($versionOutput -match "v?(\d+(?:\.\d+){2,3})") {
                $LocalVersion = $Matches[1]
            }
        } catch {
            $LocalVersion = "0.0.0"
        }
    } else {
        $cmdJoss = Get-Command joss -ErrorAction SilentlyContinue
        if ($cmdJoss) {
            try {
                $versionOutput = & joss version 2>&1
                if ($versionOutput -match "v?(\d+(?:\.\d+){2,3})") {
                    $LocalVersion = $Matches[1]
                }
            } catch {}
        }
    }
    
    Write-Log "Current version: $LocalVersion" "INFO"
    
    try {
        $release = Get-ReleaseInfo
        $latestVersion = $release.tag_name -replace '^v', ''
        
        Write-Log "Latest version: $latestVersion" "INFO"
        
        if ([version]$latestVersion -gt [version]$LocalVersion) {
            Write-Log "[!] Update available: $latestVersion" "WARNING"
            return @{ Available = $true; Version = $latestVersion; Release = $release }
        } else {
            Write-Log "[OK] You have the latest version" "SUCCESS"
            return @{ Available = $false; Version = $latestVersion; Release = $release }
        }
    } catch {
        Write-Log "[X] Error checking for updates: $($_.Exception.Message)" "ERROR"
        return @{ Available = $false; Error = $_.Exception.Message }
    }
}


function Run-Update {
    Write-Log "Running update: Download and reinstalling."
    if (Invoke-InstallComponents) {
        Write-Log "Update completed successfully." "SUCCESS"
        return $true
    }
    Write-Log "Update failed." "ERROR"
    return $false
}

function Invoke-InstallComponents {
    $binaryInstalled = Install-JosSecurity
    $extensionInstalled = Install-Extension
    return $binaryInstalled -and $extensionInstalled
}

# --- FLUJO DE TRABAJO PRINCIPAL ---

# --- FUNCIONES DE CHEQUEO PREVIO ---

function Ensure-VSCode {
    if (Test-VSCode) { return $true }
    
    Write-Host "[?] Visual Studio Code not found." -ForegroundColor Yellow
    $ans = Read-Host "Do you want to install VS Code? (y/n)"
    if ($ans -eq 'y') {
        try {
            Write-Log "Installing VS Code via Winget..." "INFO"
            winget install -e --id Microsoft.VisualStudioCode --accept-package-agreements --accept-source-agreements
            
            # Refresh PATH environment variable logic is messy in current session.
            Write-Log "[OK] VS Code installed. Please Restart PowerShell after this script." "SUCCESS"
            return $true
        } catch {
            Write-Log "[X] Auto-install failed: $($_.Exception.Message)" "ERROR"
            Write-Host "Please install manually at https://code.visualstudio.com/" -ForegroundColor Yellow
        }
    }
    return $false
}

# --- FLUJO DE TRABAJO PRINCIPAL ---

# 1. Descarga y Extracción
function Download-File {
    param($Url, $Dest)
    try {
        Invoke-WebRequest -Uri $Url -OutFile $Dest -UseBasicParsing
        return $true
    } catch {
        Write-Log "[X] Download failed ($Url): $($_.Exception.Message)" "ERROR"
        return $false
    }
}

function Get-ReleaseInfo {
    param([string]$Version)

    if ([string]::IsNullOrWhiteSpace($Version)) {
        return Invoke-RestMethod -Uri $RepoUrl -UseBasicParsing
    }

    $normalized = $Version.Trim() -replace '^[vV]', ''
    if ($normalized -notmatch '^\d+(?:\.\d+){2,3}(?:-[0-9A-Za-z][0-9A-Za-z.-]*)?$') {
        throw "Invalid version '$Version'. Use a value such as 3.6.7.2."
    }

    $tags = @($Version.Trim(), "v$normalized", "V$normalized") | Select-Object -Unique
    foreach ($tag in $tags) {
        try {
            $encodedTag = [Uri]::EscapeDataString($tag)
            return Invoke-RestMethod -Uri "https://api.github.com/repos/$RepoOwner/$RepoName/releases/tags/$encodedTag" -UseBasicParsing
        } catch {
            # Try the next conventional tag spelling.
        }
    }
    throw "Release $normalized was not found in $RepoOwner/$RepoName."
}

function Get-ReleaseAssetUrl {
    param([Parameter(Mandatory = $true)]$Release, [Parameter(Mandatory = $true)][string]$Name)

    $asset = @($Release.assets | Where-Object { $_.name -eq $Name } | Select-Object -First 1)
    if ($asset.Count -eq 0) {
        throw "Release $($Release.tag_name) does not contain required asset $Name."
    }
    return $asset[0].browser_download_url
}

function Download-And-Extract {
    param($Release = $null)

    $WindowsZip = "jossecurity-windows.zip"
    $ExtensionZip = "jossecurity-vscode.zip"

    try {
        if ($null -eq $Release) { $Release = Get-ReleaseInfo }
        $WindowsUrl = Get-ReleaseAssetUrl -Release $Release -Name $WindowsZip
        $ExtensionUrl = Get-ReleaseAssetUrl -Release $Release -Name $ExtensionZip
        Write-Log "Selected release: $($Release.tag_name)" "INFO"
    } catch {
        Write-Log "[X] Could not resolve release assets: $($_.Exception.Message)" "ERROR"
        return $false
    }

    Write-Log "[INIT] Preparing temp directory..."
    if (Test-Path $TempDir) { Remove-Item $TempDir -Recurse -Force }
    New-Item -ItemType Directory -Path $TempDir -Force | Out-Null

    # 1. Download Windows Binaries
    Write-Host "[INIT] Downloading Binaries ($WindowsZip)..." -ForegroundColor Cyan
    if (-not (Download-File -Url $WindowsUrl -Dest "$TempDir\$WindowsZip")) { return $false }

    Write-Log "[INIT] Extracting Binaries..."
    Expand-Archive -Path "$TempDir\$WindowsZip" -DestinationPath "$TempDir\runtime" -Force
    Remove-Item "$TempDir\$WindowsZip" -Force

    # 2. Download Extension
    Write-Host "[INIT] Downloading Extension ($ExtensionZip)..." -ForegroundColor Cyan
    if (-not (Download-File -Url $ExtensionUrl -Dest "$TempDir\$ExtensionZip")) { return $false }
    
    Write-Log "[INIT] Extracting Extension..."
    Expand-Archive -Path "$TempDir\$ExtensionZip" -DestinationPath "$TempDir\extension" -Force
    Remove-Item "$TempDir\$ExtensionZip" -Force

    # 3. Check/Install VS Code (Optional but part of flow)
    Ensure-VSCode | Out-Null
    
    return $true
}

# Ejecutar lógica principal
function Show-MainMenu {
    Write-Host "=======================================" -ForegroundColor Blue
    Write-Host "   JosSecurity Action Menu" -ForegroundColor Blue
    Write-Host "=======================================" -ForegroundColor Blue
    Write-Host ""
    Write-Host "Select an action:" -ForegroundColor Cyan
    Write-Host ""
    Write-Host "  [1] Install (Joss Runtime + VS Code Extension)" -ForegroundColor White
    Write-Host "  [2] Update (Only When a New Version Exists)" -ForegroundColor White
    Write-Host "  [3] Reinstall (Latest or Specific Version)" -ForegroundColor White
    Write-Host "  [4] Uninstall (Remove Runtime + Extension)" -ForegroundColor White
    Write-Host "  [0] Exit" -ForegroundColor White
    Write-Host ""

    $option = Read-Host "Option"

    switch ($option) {
        "1" {
            if (Download-And-Extract) { $null = Invoke-InstallComponents }
        }
        "2" {
            $updateInfo = Test-Update
            if ($updateInfo.Available) {
                Write-Host "Updating to v$($updateInfo.Version)" -ForegroundColor Green
                if (Download-And-Extract -Release $updateInfo.Release) { $null = Run-Update }
            }
        }
        "3" {
            $requestedVersion = Read-Host "Version to reinstall (leave blank for latest)"
            try {
                $release = Get-ReleaseInfo -Version $requestedVersion
                Write-Host "Reinstalling $($release.tag_name)..." -ForegroundColor Green
                if (Download-And-Extract -Release $release) { $null = Run-Update }
            } catch {
                Write-Log "[X] Reinstallation failed: $($_.Exception.Message)" "ERROR"
            }
        }
        "4" {
            $null = Uninstall-JosSecurity
            $null = Uninstall-Extension
        }
        "0" { Write-Log "Operation cancelled."; return }
        default { Write-Log "Invalid option" "ERROR" }
    }

    Write-Host ""
    Write-Log "Cleaning up temp directory..."
    Remove-Item $TempDir -Recurse -Force -ErrorAction SilentlyContinue
    Write-Log "Operation finished. Log: $LogFile" "SUCCESS"
}

if ($env:JOSS_INSTALLER_SKIP_MENU -ne "1") {
    if (-not (Test-Administrator)) {
        Write-Host "WARNING: Not running as Administrator. PATH changes or installation to 'Program Files' may fail." -ForegroundColor Yellow
        Write-Host "Run as Administrator for full functionality." -ForegroundColor Yellow
    }
    Show-MainMenu
}
