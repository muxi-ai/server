# MUXI Server - Windows Installation Script
# Usage: irm https://muxi.org/install/windows.ps1 | iex
# Or:    Invoke-RestMethod -Uri https://muxi.org/install/windows.ps1 | Invoke-Expression

param(
    [string]$Version = "latest",
    [switch]$AddToPath = $false,
    [switch]$Help
)

$ErrorActionPreference = "Stop"

# Display help
if ($Help) {
    Write-Host @"
MUXI Server - Windows Installation Script

USAGE:
    irm https://muxi.org/install/windows.ps1 | iex
    
    Or with options:
    irm https://muxi.org/install/windows.ps1 | iex -Version v0.20251024.0 -AddToPath

OPTIONS:
    -Version <version>   Install specific version (default: latest)
    -AddToPath           Add MUXI to system PATH (requires admin)
    -Help                Show this help message

EXAMPLES:
    # Install latest version
    irm https://muxi.org/install/windows.ps1 | iex
    
    # Install specific version
    irm https://muxi.org/install/windows.ps1 | iex -Version v0.20251024.0
    
    # Install and add to PATH
    irm https://muxi.org/install/windows.ps1 | iex -AddToPath

AFTER INSTALLATION:
    muxi-server init       Initialize configuration
    muxi-server serve      Start the server
    muxi-server version    Show version info

DOCUMENTATION:
    https://github.com/muxi-ai/server/blob/main/docs/windows-dev.md
"@
    exit 0
}

# ASCII Art Banner
Write-Host @"

    ███╗   ███╗██╗   ██╗██╗  ██╗██╗
    ████╗ ████║██║   ██║╚██╗██╔╝██║
    ██╔████╔██║██║   ██║ ╚███╔╝ ██║
    ██║╚██╔╝██║██║   ██║ ██╔██╗ ██║
    ██║ ╚═╝ ██║╚██████╔╝██╔╝ ██╗██║
    ╚═╝     ╚═╝ ╚═════╝ ╚═╝  ╚═╝╚═╝
    
    MUXI Server - Windows Installer
    https://github.com/muxi-ai/server

"@ -ForegroundColor Cyan

# Detect architecture
$arch = if ([Environment]::Is64BitOperatingSystem) {
    if ($env:PROCESSOR_ARCHITECTURE -eq "ARM64") {
        "arm64"
    } else {
        "amd64"
    }
} else {
    Write-Host "❌ Error: 32-bit Windows is not supported" -ForegroundColor Red
    exit 1
}

Write-Host "🔍 Detected: Windows $arch" -ForegroundColor Green
Write-Host ""

# Determine version to install
$githubRepo = "muxi-ai/server"
$installVersion = $Version

if ($Version -eq "latest") {
    Write-Host "📡 Fetching latest release info..."
    try {
        $latestRelease = Invoke-RestMethod -Uri "https://api.github.com/repos/$githubRepo/releases/latest"
        $installVersion = $latestRelease.tag_name
        Write-Host "✓ Latest version: $installVersion" -ForegroundColor Green
    } catch {
        Write-Host "⚠️  Warning: Could not fetch latest release, using fallback" -ForegroundColor Yellow
        $installVersion = "v0.20251024.0"
    }
} else {
    Write-Host "📦 Installing version: $installVersion"
}

Write-Host ""

# Set installation directory
$installDir = "$env:LOCALAPPDATA\muxi\bin"
$configDir = "$env:APPDATA\muxi\server"
$binaryName = "muxi-server.exe"
$downloadName = "muxi-server-windows-$arch.exe"

# Create directories
Write-Host "📁 Creating directories..."
New-Item -ItemType Directory -Force -Path $installDir | Out-Null
New-Item -ItemType Directory -Force -Path $configDir | Out-Null
Write-Host "   → $installDir" -ForegroundColor Gray
Write-Host "   → $configDir" -ForegroundColor Gray
Write-Host ""

# Download binary
$downloadUrl = "https://github.com/$githubRepo/releases/download/$installVersion/$downloadName"
$binaryPath = Join-Path $installDir $binaryName

Write-Host "⬇️  Downloading MUXI Server..."
Write-Host "   → $downloadUrl" -ForegroundColor Gray

try {
    # Use BITS transfer for better progress and reliability
    Import-Module BitsTransfer
    Start-BitsTransfer -Source $downloadUrl -Destination $binaryPath -Description "Downloading MUXI Server"
    Write-Host "✓ Download complete" -ForegroundColor Green
} catch {
    Write-Host "⚠️  BITS transfer failed, trying webclient..." -ForegroundColor Yellow
    try {
        $webClient = New-Object System.Net.WebClient
        $webClient.DownloadFile($downloadUrl, $binaryPath)
        Write-Host "✓ Download complete" -ForegroundColor Green
    } catch {
        Write-Host "❌ Error: Failed to download binary" -ForegroundColor Red
        Write-Host "   URL: $downloadUrl" -ForegroundColor Red
        Write-Host "   Error: $_" -ForegroundColor Red
        exit 1
    }
}

Write-Host ""

# Verify binary
if (-not (Test-Path $binaryPath)) {
    Write-Host "❌ Error: Binary not found after download" -ForegroundColor Red
    exit 1
}

$fileSize = (Get-Item $binaryPath).Length / 1MB
Write-Host "✓ Binary verified ($([math]::Round($fileSize, 2)) MB)" -ForegroundColor Green
Write-Host ""

# Test binary
Write-Host "🧪 Testing binary..."
try {
    $versionOutput = & $binaryPath version 2>&1
    Write-Host "✓ Binary is working" -ForegroundColor Green
    Write-Host ""
} catch {
    Write-Host "⚠️  Warning: Could not verify binary (may still work)" -ForegroundColor Yellow
    Write-Host ""
}

# Add to PATH (optional)
if ($AddToPath) {
    Write-Host "🔧 Adding to PATH..."
    
    # Check if running as admin
    $isAdmin = ([Security.Principal.WindowsPrincipal] [Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
    
    if ($isAdmin) {
        # Add to system PATH
        $currentPath = [Environment]::GetEnvironmentVariable("Path", "Machine")
        if ($currentPath -notlike "*$installDir*") {
            [Environment]::SetEnvironmentVariable("Path", "$currentPath;$installDir", "Machine")
            Write-Host "✓ Added to system PATH (all users)" -ForegroundColor Green
            Write-Host "   Restart your terminal for changes to take effect" -ForegroundColor Yellow
        } else {
            Write-Host "✓ Already in system PATH" -ForegroundColor Green
        }
    } else {
        # Add to user PATH
        $currentPath = [Environment]::GetEnvironmentVariable("Path", "User")
        if ($currentPath -notlike "*$installDir*") {
            [Environment]::SetEnvironmentVariable("Path", "$currentPath;$installDir", "User")
            Write-Host "✓ Added to user PATH" -ForegroundColor Green
            Write-Host "   Restart your terminal for changes to take effect" -ForegroundColor Yellow
        } else {
            Write-Host "✓ Already in user PATH" -ForegroundColor Green
        }
    }
    
    # Update current session
    $env:Path = "$env:Path;$installDir"
    Write-Host ""
}

# Installation summary
Write-Host "═══════════════════════════════════════════════" -ForegroundColor Green
Write-Host "✅ MUXI Server installed successfully!" -ForegroundColor Green
Write-Host "═══════════════════════════════════════════════" -ForegroundColor Green
Write-Host ""
Write-Host "📍 Installation Details:" -ForegroundColor Cyan
Write-Host "   Binary:  $binaryPath" -ForegroundColor Gray
Write-Host "   Config:  $configDir" -ForegroundColor Gray
Write-Host "   Version: $installVersion" -ForegroundColor Gray
Write-Host ""

if (-not $AddToPath) {
    Write-Host "💡 Quick Start:" -ForegroundColor Cyan
    Write-Host "   Add to PATH for easier access:" -ForegroundColor Gray
    Write-Host "   `$env:Path += `";$installDir`"" -ForegroundColor Yellow
    Write-Host ""
}

Write-Host "🚀 Next Steps:" -ForegroundColor Cyan
Write-Host ""
Write-Host "   1. Initialize configuration:" -ForegroundColor White
if ($AddToPath -or $env:Path -like "*$installDir*") {
    Write-Host "      muxi-server init" -ForegroundColor Yellow
} else {
    Write-Host "      & `"$binaryPath`" init" -ForegroundColor Yellow
}
Write-Host ""
Write-Host "   2. Start the server:" -ForegroundColor White
if ($AddToPath -or $env:Path -like "*$installDir*") {
    Write-Host "      muxi-server serve" -ForegroundColor Yellow
} else {
    Write-Host "      & `"$binaryPath`" serve" -ForegroundColor Yellow
}
Write-Host ""
Write-Host "   3. Deploy a formation:" -ForegroundColor White
Write-Host "      See: https://github.com/muxi-ai/server/blob/main/docs/windows-dev.md" -ForegroundColor Yellow
Write-Host ""

# Docker Desktop check
Write-Host "📋 Requirements Check:" -ForegroundColor Cyan
Write-Host ""

$dockerInstalled = $false
try {
    $dockerVersion = docker --version 2>$null
    if ($dockerVersion) {
        Write-Host "   ✓ Docker Desktop: $dockerVersion" -ForegroundColor Green
        $dockerInstalled = $true
    }
} catch {
    # Docker not found
}

if (-not $dockerInstalled) {
    Write-Host "   ⚠️  Docker Desktop: Not installed" -ForegroundColor Yellow
    Write-Host "      Required for SIF runtime support" -ForegroundColor Gray
    Write-Host "      Install from: https://www.docker.com/products/docker-desktop" -ForegroundColor Gray
}

Write-Host ""

# Firewall reminder
Write-Host "🔥 Firewall Notice:" -ForegroundColor Cyan
Write-Host "   MUXI Server uses port 7890 by default" -ForegroundColor Gray
Write-Host "   You may need to allow it in Windows Firewall" -ForegroundColor Gray
Write-Host ""

# Docs link
Write-Host "📚 Documentation:" -ForegroundColor Cyan
Write-Host "   https://github.com/muxi-ai/server/blob/main/docs/windows-dev.md" -ForegroundColor Gray
Write-Host ""

Write-Host "═══════════════════════════════════════════════" -ForegroundColor Green
Write-Host "Happy coding! 🎉" -ForegroundColor Cyan
Write-Host "═══════════════════════════════════════════════" -ForegroundColor Green
