# Installation

Complete installation guide for MUXI Server on different platforms.

---

## Quick Install

### Unix/Linux/macOS

**One-Command Install (Recommended):**

**⚠️ System-Level Installation** - Requires `sudo` privileges:

```bash
curl -sSL https://get.muxi.ai | sudo bash
```

**Why `sudo`?**  
MUXI Server is a **system service** that:
- Installs to `/usr/local/bin/` (system-wide binary)
- Manages background processes (formations)
- Requires elevated permissions on production servers
- Sets up systemd/launchd services

**Installs:**
- ✅ MUXI Server binary → `/usr/local/bin/muxi-server`
- ✅ Configuration directory → `~/.muxi/server/`
- ✅ System service setup (optional)
- ✅ Runtime directories

**Supported Platforms:**
- macOS (Intel & Apple Silicon)
- Linux (Ubuntu, Debian, RHEL, CentOS, Fedora, Arch)

### Windows

**One-Command Install (PowerShell):**

```powershell
irm https://install.muxi.ai/windows.ps1 | iex
```

**User-Level Installation** - No admin required (default):

**Installs:**
- ✅ MUXI Server binary → `%LOCALAPPDATA%\muxi\bin\muxi-server.exe`
- ✅ Configuration directory → `%APPDATA%\muxi\server\`
- ✅ Optional PATH configuration
- ✅ Runtime directories

**Supported Platforms:**
- Windows 10/11 (x64)
- Windows 11 ARM64 (Surface Pro X, etc.)

> **Note:** Windows support is optimized for development environments. For production deployments, we recommend Linux or Docker.

**See:** [Windows Development Guide](windows-dev.md) for complete setup and usage

---

## Manual Installation

### Download Binary

**macOS (Apple Silicon):**

```bash
curl -L https://github.com/muxi-ai/server/releases/latest/download/muxi-server-darwin-arm64 \
  -o muxi-server
chmod +x muxi-server
sudo mv muxi-server /usr/local/bin/
```

**macOS (Intel):**

```bash
curl -L https://github.com/muxi-ai/server/releases/latest/download/muxi-server-darwin-amd64 \
  -o muxi-server
chmod +x muxi-server
sudo mv muxi-server /usr/local/bin/
```

**Linux (x86_64):**

```bash
curl -L https://github.com/muxi-ai/server/releases/latest/download/muxi-server-linux-amd64 \
  -o muxi-server
chmod +x muxi-server
sudo mv muxi-server /usr/local/bin/
```

**Linux (ARM64):**

```bash
curl -L https://github.com/muxi-ai/server/releases/latest/download/muxi-server-linux-arm64 \
  -o muxi-server
chmod +x muxi-server
sudo mv muxi-server /usr/local/bin/
```

**Windows (x64):**

```powershell
# Download binary
$version = "v0.20251024.0"  # Check releases page for latest
Invoke-WebRequest `
  -Uri "https://github.com/muxi-ai/server/releases/latest/download/muxi-server-windows-amd64.exe" `
  -OutFile "muxi-server.exe"

# Install to user directory
New-Item -ItemType Directory -Force -Path "$env:LOCALAPPDATA\muxi\bin"
Move-Item muxi-server.exe "$env:LOCALAPPDATA\muxi\bin\muxi-server.exe"

# Add to PATH (current session)
$env:Path += ";$env:LOCALAPPDATA\muxi\bin"
```

**Windows (ARM64):**

```powershell
# Download binary for ARM64 (Surface Pro X, etc.)
Invoke-WebRequest `
  -Uri "https://github.com/muxi-ai/server/releases/latest/download/muxi-server-windows-arm64.exe" `
  -OutFile "muxi-server.exe"

# Install to user directory
New-Item -ItemType Directory -Force -Path "$env:LOCALAPPDATA\muxi\bin"
Move-Item muxi-server.exe "$env:LOCALAPPDATA\muxi\bin\muxi-server.exe"

# Add to PATH (current session)
$env:Path += ";$env:LOCALAPPDATA\muxi\bin"
```

### Initialize Configuration

**Unix/Linux/macOS:**

After installing the binary:

```bash
# Create configuration directory
mkdir -p ~/.muxi/server

# Generate authentication credentials
muxi-server init
```

This creates:
- `~/.muxi/server/config.yaml` - Server configuration
- `~/.muxi/server/credentials.yaml` - Authentication keys

---

## Package Managers (Coming Soon)

**🚧 Planned - Not Yet Available**

Native package manager support is in development. These will be available in a future release:

### Homebrew (macOS/Linux) - PLANNED

```bash
# Add MUXI tap (NOT YET AVAILABLE)
brew tap muxi-ai/tap

# Install
brew install muxi-server

# Upgrade
brew upgrade muxi-server
```

**Status:** 🟡 In development  
**Timeline:** Q1 2025  
**Track:** [Issue #TODO](https://github.com/muxi-ai/server/issues/TODO)

### APT (Ubuntu/Debian) - PLANNED

```bash
# Add repository (NOT YET AVAILABLE)
curl -fsSL https://packages.muxi.ai/apt/gpg.key | sudo gpg --dearmor -o /usr/share/keyrings/muxi.gpg
echo "deb [signed-by=/usr/share/keyrings/muxi.gpg] https://packages.muxi.ai/apt stable main" | \
  sudo tee /etc/apt/sources.list.d/muxi.list

# Install
sudo apt update
sudo apt install muxi-server
```

**Status:** 🟡 Planned  
**Timeline:** Q1 2025

### YUM/DNF (RHEL/CentOS/Fedora) - PLANNED

```bash
# Add repository (NOT YET AVAILABLE)
sudo tee /etc/yum.repos.d/muxi.repo <<EOF
[muxi]
name=MUXI Repository
baseurl=https://packages.muxi.ai/yum/stable
enabled=1
gpgcheck=1
gpgkey=https://packages.muxi.ai/yum/gpg.key
EOF

# Install (RHEL/CentOS)
sudo yum install muxi-server

# Install (Fedora)
sudo dnf install muxi-server
```

**Status:** 🟡 Planned  
**Timeline:** Q1 2025

### AUR (Arch Linux) - PLANNED

```bash
# Using yay (NOT YET AVAILABLE)
yay -S muxi-server

# Or using paru
paru -S muxi-server
```

**Status:** 🟡 Planned  
**Timeline:** Q1 2025

---

**For now, use:**
1. ✅ **One-command install:** `curl -sSL https://get.muxi.ai | sudo bash`
2. ✅ **Manual binary download:** See "Manual Installation" below
3. ✅ **Build from source:** See "Build from Source" below
4. ✅ **Docker:** See "Docker" section below

---

## Build from Source

### Prerequisites

- Go 1.21+ ([install Go](https://go.dev/doc/install))
- Git

### Build Steps

```bash
# Clone repository
git clone https://github.com/muxi-ai/server.git
cd server

# Build
cd src
go build -o muxi-server ./cmd/server

# Install
sudo mv muxi-server /usr/local/bin/

# Verify
muxi-server version
```

### Development Build

For development with debug symbols:

```bash
cd src
go build -tags debug -o muxi-server ./cmd/server
```

---

## Running as a Service

### systemd (Linux)

**1. Create service file:**

```bash
sudo tee /etc/systemd/system/muxi-server.service <<EOF
[Unit]
Description=MUXI Server - Formation Orchestration Platform
After=network.target

[Service]
Type=simple
User=muxi
Group=muxi
WorkingDirectory=/var/lib/muxi
ExecStart=/usr/local/bin/muxi-server start
Restart=always
RestartSec=10
StandardOutput=journal
StandardError=journal
SyslogIdentifier=muxi-server

# Security
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/var/lib/muxi /var/log/muxi

[Install]
WantedBy=multi-user.target
EOF
```

**2. Create user and directories:**

```bash
# Create system user
sudo useradd -r -s /bin/false -d /var/lib/muxi muxi

# Create directories
sudo mkdir -p /var/lib/muxi/formations
sudo mkdir -p /var/log/muxi
sudo chown -R muxi:muxi /var/lib/muxi /var/log/muxi
```

**3. Start service:**

```bash
# Reload systemd
sudo systemctl daemon-reload

# Enable on boot
sudo systemctl enable muxi-server

# Start now
sudo systemctl start muxi-server

# Check status
sudo systemctl status muxi-server
```

**4. View logs:**

```bash
# Follow logs
sudo journalctl -u muxi-server -f

# View recent logs
sudo journalctl -u muxi-server -n 100
```

### launchd (macOS)

**1. Create plist file:**

```bash
sudo tee ~/Library/LaunchAgents/ai.muxi.server.plist <<EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>ai.muxi.server</string>
    
    <key>ProgramArguments</key>
    <array>
        <string>/usr/local/bin/muxi-server</string>
        <string>start</string>
    </array>
    
    <key>RunAtLoad</key>
    <true/>
    
    <key>KeepAlive</key>
    <dict>
        <key>SuccessfulExit</key>
        <false/>
    </dict>
    
    <key>StandardOutPath</key>
    <string>/usr/local/var/log/muxi-server.log</string>
    
    <key>StandardErrorPath</key>
    <string>/usr/local/var/log/muxi-server-error.log</string>
    
    <key>WorkingDirectory</key>
    <string>/usr/local/var/muxi</string>
</dict>
</plist>
EOF
```

**2. Create directories:**

```bash
mkdir -p /usr/local/var/muxi/formations
mkdir -p /usr/local/var/log
```

**3. Load service:**

```bash
# Load service
launchctl load ~/Library/LaunchAgents/ai.muxi.server.plist

# Check status
launchctl list | grep muxi

# Unload (stop)
launchctl unload ~/Library/LaunchAgents/ai.muxi.server.plist
```

---

## Docker

### Quick Start

```bash
# Run server
docker run -d \
  --name muxi-server \
  -p 7890:7890 \
  -p 8000-8100:8000-8100 \
  -v ~/.muxi:/root/.muxi \
  muxiai/server:latest
```

### Docker Compose

```yaml
version: '3.8'

services:
  muxi-server:
    image: muxiai/server:latest
    container_name: muxi-server
    restart: unless-stopped
    ports:
      - "7890:7890"        # Management API
      - "8000-8100:8000-8100"  # Formation ports
    volumes:
      - ~/.muxi:/root/.muxi
      - muxi-data:/var/lib/muxi
    environment:
      - MUXI_LOG_LEVEL=info
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:7890/health"]
      interval: 30s
      timeout: 10s
      retries: 3

volumes:
  muxi-data:
```

**Start:**

```bash
docker-compose up -d
```

---

## Verification

After installation, verify everything works:

```bash
# Check version
muxi-server version

# Initialize (if not done)
muxi-server init

# Start server
muxi-server start

# Check health (in another terminal)
curl http://localhost:7890/health
```

Expected output:
```json
{
  "status": "healthy",
  "version": "1.0.0",
  "uptime": 123
}
```

---

## Uninstallation

### Remove Binary

```bash
# Stop service (if running)
sudo systemctl stop muxi-server    # Linux
launchctl unload ~/Library/LaunchAgents/ai.muxi.server.plist  # macOS

# Remove binary
sudo rm /usr/local/bin/muxi-server
```

### Remove Data

```bash
# Remove configuration and data
rm -rf ~/.muxi/server

# Remove system service directories (Linux)
sudo rm -rf /var/lib/muxi
sudo rm -rf /var/log/muxi
sudo userdel muxi
```

### Package Manager Uninstall

```bash
# Homebrew
brew uninstall muxi-server
brew untap muxi-ai/tap

# APT
sudo apt remove muxi-server
sudo rm /etc/apt/sources.list.d/muxi.list

# YUM/DNF
sudo yum remove muxi-server
sudo rm /etc/yum.repos.d/muxi.repo
```

---

## Next Steps

- [Configure your server](./configuration.md)
- [Set up authentication](./authentication.md)
- [Deploy your first formation](./formations.md)

---

**Having issues?** See the [Troubleshooting Guide](./troubleshooting.md)
