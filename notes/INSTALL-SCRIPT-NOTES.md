# Install Script Requirements - Runtime Dependencies

**Status:** TODO - Needs implementation  
**Priority:** High (required before public release)

---

## Overview

The MUXI Server install script needs to detect platform and install appropriate container runtime:

- **Linux:** Singularity/Apptainer
- **macOS:** Docker Desktop
- **Windows:** Docker Desktop

---

## Install Script Flow

```bash
#!/bin/bash
# install.sh - MUXI Server installation script

# 1. Detect platform
OS="$(uname -s)"
ARCH="$(uname -m)"

# 2. Install MUXI Server binary
curl -L https://github.com/muxi-ai/server/releases/latest/download/muxi-server-${OS}-${ARCH} \
  -o /usr/local/bin/muxi-server
chmod +x /usr/local/bin/muxi-server

# 3. Install runtime dependencies (NEW!)
case "$OS" in
  Linux)
    install_singularity
    ;;
  Darwin)
    install_docker_desktop_macos
    ;;
  *)
    echo "Unsupported OS: $OS"
    exit 1
    ;;
esac

# 4. Initialize server
muxi-server init
```

---

## Linux - Singularity Installation

```bash
install_singularity() {
  echo "📦 Installing Singularity..."
  
  # Check if already installed
  if command -v singularity &> /dev/null; then
    echo "✓ Singularity already installed: $(singularity --version)"
    return 0
  fi
  
  # Detect Linux distribution
  if [ -f /etc/os-release ]; then
    . /etc/os-release
    DISTRO=$ID
  fi
  
  case "$DISTRO" in
    ubuntu|debian)
      echo "Installing Singularity on Ubuntu/Debian..."
      sudo apt-get update
      sudo apt-get install -y singularity-container
      ;;
    
    centos|rhel|fedora)
      echo "Installing Singularity on RHEL/CentOS/Fedora..."
      sudo yum install -y epel-release
      sudo yum install -y singularity
      ;;
    
    *)
      echo "⚠️  Automatic installation not supported for: $DISTRO"
      echo ""
      echo "Please install Singularity manually:"
      echo "  https://sylabs.io/guides/latest/admin-guide/installation.html"
      echo ""
      echo "After installation, run: muxi-server init"
      exit 1
      ;;
  esac
  
  # Verify installation
  if command -v singularity &> /dev/null; then
    echo "✓ Singularity installed: $(singularity --version)"
  else
    echo "❌ Singularity installation failed"
    exit 1
  fi
}
```

---

## macOS - Docker Desktop Installation

```bash
install_docker_desktop_macos() {
  echo "📦 Checking Docker Desktop..."
  
  # Check if Docker is already installed
  if command -v docker &> /dev/null; then
    # Check if Docker daemon is running
    if docker info &> /dev/null; then
      echo "✓ Docker Desktop already installed and running"
      return 0
    else
      echo "⚠️  Docker is installed but not running"
      echo "   Please start Docker Desktop from Applications"
      return 0
    fi
  fi
  
  # Check if Homebrew is available
  if command -v brew &> /dev/null; then
    echo "Installing Docker Desktop via Homebrew..."
    brew install --cask docker
    
    echo ""
    echo "✓ Docker Desktop installed"
    echo ""
    echo "⚠️  IMPORTANT: Please start Docker Desktop from Applications"
    echo "   Then run: muxi-server init"
    echo ""
  else
    echo "⚠️  Homebrew not found"
    echo ""
    echo "Please install Docker Desktop manually:"
    echo "  https://www.docker.com/products/docker-desktop"
    echo ""
    echo "Or install Homebrew first:"
    echo "  /bin/bash -c \"\$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)\""
    echo ""
    echo "After Docker Desktop is running, run: muxi-server init"
  fi
}
```

---

## Windows - Docker Desktop Installation

```powershell
# install.ps1 - Windows installation script

function Install-DockerDesktop {
  Write-Host "📦 Checking Docker Desktop..." -ForegroundColor Cyan
  
  # Check if Docker is installed
  if (Get-Command docker -ErrorAction SilentlyContinue) {
    # Check if Docker daemon is running
    try {
      docker info | Out-Null
      Write-Host "✓ Docker Desktop already installed and running" -ForegroundColor Green
      return
    } catch {
      Write-Host "⚠️  Docker is installed but not running" -ForegroundColor Yellow
      Write-Host "   Please start Docker Desktop from Start Menu" -ForegroundColor Yellow
      return
    }
  }
  
  # Check if Chocolatey is available
  if (Get-Command choco -ErrorAction SilentlyContinue) {
    Write-Host "Installing Docker Desktop via Chocolatey..." -ForegroundColor Cyan
    choco install docker-desktop -y
    
    Write-Host ""
    Write-Host "✓ Docker Desktop installed" -ForegroundColor Green
    Write-Host ""
    Write-Host "⚠️  IMPORTANT: Please start Docker Desktop from Start Menu" -ForegroundColor Yellow
    Write-Host "   Then run: muxi-server init" -ForegroundColor Yellow
  } else {
    Write-Host "⚠️  Chocolatey not found" -ForegroundColor Yellow
    Write-Host ""
    Write-Host "Please install Docker Desktop manually:" -ForegroundColor White
    Write-Host "  https://www.docker.com/products/docker-desktop" -ForegroundColor Cyan
    Write-Host ""
    Write-Host "Or install Chocolatey first:" -ForegroundColor White
    Write-Host "  Set-ExecutionPolicy Bypass -Scope Process -Force;" -ForegroundColor Cyan
    Write-Host "  [System.Net.ServicePointManager]::SecurityProtocol = [System.Net.ServicePointManager]::SecurityProtocol -bor 3072;" -ForegroundColor Cyan
    Write-Host "  iex ((New-Object System.Net.WebClient).DownloadString('https://community.chocolatey.org/install.ps1'))" -ForegroundColor Cyan
  }
}
```

---

## Alternative: `muxi-server init` Handles Installation

Instead of install script, we could make `muxi-server init` check and guide users:

```bash
$ muxi-server init

MUXI Server Initialization
==========================

✓ Server binary installed
✓ Configuration directory created: ~/.muxi-server/

Checking runtime dependencies...
❌ Singularity not found

To use MUXI Server, you need Singularity:

  Ubuntu/Debian:
    sudo apt-get update
    sudo apt-get install -y singularity-container

  RHEL/CentOS/Fedora:
    sudo yum install -y epel-release
    sudo yum install -y singularity

  Other Linux distributions:
    https://sylabs.io/guides/latest/admin-guide/installation.html

After installing Singularity, run: muxi-server init --verify
```

---

## Recommended Approach

**Two-phase installation:**

### Phase 1: Install MUXI Server Binary
```bash
curl -sSL https://get.muxi.ai | bash
# Downloads and installs muxi-server binary only
```

### Phase 2: Initialize (checks/installs dependencies)
```bash
muxi-server init
# Checks for Singularity/Docker
# Offers to install if missing
# Guides user through manual installation if automatic fails
```

**Benefits:**
- Server binary installs fast (no waiting for Docker/Singularity)
- User can review what will be installed
- Graceful handling of permission issues (sudo required)
- Clear next steps if automatic installation fails

---

## Implementation Checklist

- [ ] Update `muxi-server init` command to check runtime availability
- [ ] Add `--install-runtime` flag to automatically install dependencies
- [ ] Add platform detection logic
- [ ] Add Singularity installation for common Linux distros
- [ ] Add Docker Desktop installation guidance for macOS/Windows
- [ ] Add `muxi-server runtime validate` command to verify setup
- [ ] Update installation documentation
- [ ] Add troubleshooting guide for runtime issues

---

## Files to Create/Modify

1. **src/cmd/server/init.go** - Add runtime validation
2. **src/pkg/runtime/validator.go** - Already created! ✅
3. **install.sh** - Update with runtime installation
4. **install.ps1** - Create Windows installation script
5. **docs/installation.md** - Update with runtime requirements

---

## Example User Flow

```bash
# User runs install script
$ curl -sSL https://get.muxi.ai | bash
Downloading MUXI Server...
✓ Installed to /usr/local/bin/muxi-server

# User initializes server
$ muxi-server init
Initializing MUXI Server...
✓ Configuration created: ~/.muxi-server/config.yaml
✓ Credentials generated: ~/.muxi-server/credentials.json

Checking runtime dependencies...
❌ Singularity not found

Would you like to install Singularity now? [Y/n] y

Installing Singularity...
[sudo] password for user: 
✓ Singularity installed: singularity-ce version 3.11.4

✓ MUXI Server is ready!

Start the server:
  muxi-server serve

Deploy a formation:
  muxi-server formation deploy my-formation.tar.gz
```

---

## Testing Requirements

- [ ] Test on Ubuntu 22.04 LTS
- [ ] Test on Debian 11/12
- [ ] Test on RHEL/CentOS 8+
- [ ] Test on macOS (Intel and Apple Silicon)
- [ ] Test on Windows 10/11
- [ ] Test with Singularity already installed
- [ ] Test with Docker already installed
- [ ] Test with neither installed
- [ ] Test permission denied scenarios
- [ ] Test offline scenarios (provide offline instructions)

---

**Next Action:** Implement runtime validation in `muxi-server init` command.
