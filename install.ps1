# sushiro-overdose one-click installer for Windows
# Usage: irm https://raw.githubusercontent.com/Ryujoxys/sushiro-overdose/master/install.ps1 | iex

$ErrorActionPreference = "Stop"

$Repo = "Ryujoxys/sushiro-overdose"
$Binary = "sushiro-overdose"

Write-Host "=== sushiro-overdose installer ===" -ForegroundColor Green

# Detect architecture
$Arch = "amd64"
if ([Environment]::Is64BitOperatingSystem -and -not [Environment]::Is64BitProcess) {
    $Arch = "amd64"
}

Write-Host "Detected: windows/$Arch"

# Find latest release
$Latest = (Invoke-RestMethod -Uri "https://api.github.com/repos/$Repo/releases/latest").tag_name
if (-not $Latest) {
    Write-Host "Could not determine latest version" -ForegroundColor Red
    exit 1
}

Write-Host "Latest version: $Latest"

# Build download URL
$Filename = "${Binary}_$($Latest -replace '^v')_windows_${Arch}.zip"
$Url = "https://github.com/$Repo/releases/download/$Latest/$Filename"

Write-Host "Downloading $Url..."
$TempDir = [System.IO.Path]::GetTempPath()
$ZipPath = Join-Path $TempDir $Filename

Invoke-WebRequest -Uri $Url -OutFile $ZipPath

# Extract
Write-Host "Extracting..."
$ExtractDir = Join-Path $TempDir "sushiro-install"
if (Test-Path $ExtractDir) { Remove-Item $ExtractDir -Recurse -Force }
Expand-Archive -Path $ZipPath -DestinationPath $ExtractDir

# Install
$InstallDir = Join-Path $env:APPDATA "sushiro"
New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null

$ExePath = Get-ChildItem -Path $ExtractDir -Filter "*.exe" -Recurse | Select-Object -First 1
if ($ExePath) {
    Copy-Item $ExePath.FullName (Join-Path $InstallDir "$Binary.exe") -Force

    # Add to PATH if not already there
    $UserPath = [Environment]::GetEnvironmentVariable("Path", "User")
    if ($UserPath -notlike "*$InstallDir*") {
        [Environment]::SetEnvironmentVariable("Path", "$UserPath;$InstallDir", "User")
        Write-Host "Added $InstallDir to user PATH" -ForegroundColor Yellow
    }
}

# Cleanup
Remove-Item $ZipPath -Force
Remove-Item $ExtractDir -Recurse -Force

Write-Host ""
Write-Host "sushiro-overdose $Latest installed to $InstallDir\$Binary.exe" -ForegroundColor Green
Write-Host "  Restart terminal, then run: $Binary" -ForegroundColor Cyan
