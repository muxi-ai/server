# Windows Development Guide

**MUXI Server on Windows** - Complete guide for running MUXI Server on Windows for development.

> **Note:** Windows support is optimized for development environments. For production deployments, we recommend Linux or Docker.

---

## Quick Start

### Installation

**One-command install:**
```powershell
irm https://install.muxi.ai/windows.ps1 | iex
```

**Or manual download:**
```powershell
# Download latest release
$version = "v0.20251024.0"  # Or check releases page
$arch = "amd64"  # Or "arm64" for ARM64 Windows

Invoke-WebRequest `
  -Uri "https://github.com/muxi-ai/server/releases/download/$version/muxi-server-windows-$arch.exe" `
  -OutFile "muxi-server.exe"

# Move to installation directory
New-Item -ItemType Directory -Force -Path "$env:LOCALAPPDATA\muxi\bin"
Move-Item muxi-server.exe "$env:LOCALAPPDATA\muxi\bin\muxi-server.exe"

# Add to PATH (current session)
$env:Path += ";$env:LOCALAPPDATA\muxi\bin"
```

### Initialize

```powershell
muxi-server init
```

Follow the prompts to configure:
- **Server Name:** My Dev Server
- **Port:** 7890 (default)
- **Admin Email:** dev@example.com

### Start Server

```powershell
muxi-server serve
```

The server will start on http://localhost:7890

**Test it:**
```powershell
# Health check
Invoke-WebRequest http://localhost:7890/health

# Server info
muxi-server config show
```

---

## Architecture Detection

The install script automatically detects your Windows architecture:

- **Intel/AMD 64-bit:** `windows-amd64`
- **ARM64 (Surface Pro X, etc.):** `windows-arm64`

Check yours:
```powershell
$env:PROCESSOR_ARCHITECTURE  # Shows: AMD64 or ARM64
```

---

## Directory Structure

MUXI Server uses Windows-appropriate paths:

### User Install (Default)
```
%APPDATA%\muxi\server\          # C:\Users\<YourName>\AppData\Roaming\muxi\server
├── config.yaml                 # Server configuration
├── .muxi_key                   # Authentication keys
├── registry.json               # Formation registry
├── formations\                 # Formation deployments
│   └── my-formation\
│       ├── current\            # Active version
│       ├── previous\           # Backup version
│       └── version.json        # Version metadata
├── logs\                       # Log files
│   ├── formation-*.log
│   └── audit.log
└── pids\                       # PID files
```

### System Install (Admin)
```
C:\ProgramData\muxi\
├── server\
│   └── config.yaml
├── data\
│   ├── registry.json
│   └── formations\
└── logs\
```

### Custom Paths
```powershell
# Override with environment variables
$env:MUXI_CONFIG_DIR = "C:\custom\config"
$env:MUXI_DATA_DIR = "C:\custom\data"
$env:MUXI_LOG_DIR = "C:\custom\logs"

muxi-server serve
```

---

## Running the Server

### Foreground (Development)

**Simple:**
```powershell
muxi-server serve
```

**With logging:**
```powershell
muxi-server serve 2>&1 | Tee-Object -FilePath muxi-server.log
```

### Background (Hidden Window)

**Start in background:**
```powershell
Start-Process muxi-server -ArgumentList "serve" -WindowStyle Hidden
```

**Stop background process:**
```powershell
Stop-Process -Name "muxi-server" -Force
```

**Check if running:**
```powershell
Get-Process muxi-server -ErrorAction SilentlyContinue
```

### Windows Terminal Profile (Recommended)

Add to your Windows Terminal `settings.json`:

```json
{
  "profiles": {
    "list": [
      {
        "name": "MUXI Server",
        "commandline": "powershell.exe -NoExit -Command \"muxi-server serve\"",
        "icon": "🚀",
        "colorScheme": "One Half Dark",
        "startingDirectory": "%APPDATA%\\muxi\\server"
      }
    ]
  }
}
```

Now you can start MUXI Server from a dedicated terminal tab!

---

## Docker Desktop (Required for SIF Runtime)

MUXI Server uses Docker to run Singularity containers on Windows (for SIF runtime).

### Install Docker Desktop

1. Download from: https://www.docker.com/products/docker-desktop
2. Install and restart Windows
3. Verify installation:

```powershell
docker --version
docker ps
```

### Configure Docker

**Enable WSL 2 backend** (Recommended):
- Settings → General → Use WSL 2 based engine

**Resources:**
- Settings → Resources → Advanced
- Allocate sufficient RAM (4GB+ recommended)

### Test Docker

```powershell
docker run hello-world
```

---

## Firewall Configuration

MUXI Server uses port **7890** by default.

### Allow in Windows Firewall

**PowerShell (Admin):**
```powershell
New-NetFirewallRule `
  -DisplayName "MUXI Server" `
  -Direction Inbound `
  -Action Allow `
  -Protocol TCP `
  -LocalPort 7890
```

**Or use GUI:**
1. Windows Defender Firewall → Advanced Settings
2. Inbound Rules → New Rule
3. Port → TCP → 7890
4. Allow the connection

### Check Current Firewall Rules

```powershell
Get-NetFirewallRule -DisplayName "MUXI Server"
```

---

## Deploying Formations

### Native Python Formations

```powershell
# Create a simple formation
mkdir my-formation
cd my-formation

# app.py
@"
from fastapi import FastAPI
import os

app = FastAPI()

@app.get("/")
def read_root():
    return {"message": "Hello from Windows!"}

@app.get("/health")
def health():
    return {"status": "healthy"}
"@ | Out-File -Encoding UTF8 app.py

# Deploy
Compress-Archive -Path . -DestinationPath formation.zip
Invoke-WebRequest `
  -Uri "http://localhost:7890/rpc/formations/deploy" `
  -Method POST `
  -ContentType "application/gzip" `
  -InFile formation.zip
```

### SIF Runtime (Docker-based)

```powershell
# Deploy SIF formation
Invoke-WebRequest `
  -Uri "http://localhost:7890/rpc/formations/deploy" `
  -Method POST `
  -ContentType "application/json" `
  -Body '{"id":"my-sif","runtime_type":"singularity","sif_path":"C:\\formations\\my.sif"}'
```

> **Note:** On Windows, SIF files run inside Docker containers using the runtime-runner image.

---

## VS Code Integration

### Tasks Configuration

Create `.vscode/tasks.json`:

```json
{
  "version": "2.0.0",
  "tasks": [
    {
      "label": "Start MUXI Server",
      "type": "shell",
      "command": "muxi-server serve",
      "problemMatcher": [],
      "presentation": {
        "reveal": "always",
        "panel": "dedicated"
      },
      "isBackground": true
    },
    {
      "label": "Stop MUXI Server",
      "type": "shell",
      "command": "Stop-Process -Name 'muxi-server' -Force",
      "problemMatcher": []
    },
    {
      "label": "View MUXI Logs",
      "type": "shell",
      "command": "Get-Content $env:APPDATA\\muxi\\server\\logs\\*.log -Tail 50 -Wait",
      "problemMatcher": []
    }
  ]
}
```

### Launch Configuration

Create `.vscode/launch.json`:

```json
{
  "version": "0.2.0",
  "configurations": [
    {
      "name": "MUXI Server",
      "type": "go",
      "request": "launch",
      "mode": "debug",
      "program": "${workspaceFolder}/src/cmd/server",
      "args": ["serve"],
      "cwd": "${workspaceFolder}"
    }
  ]
}
```

---

## Troubleshooting

### Server Won't Start

**Check port availability:**
```powershell
Get-NetTCPConnection -LocalPort 7890 -ErrorAction SilentlyContinue
```

**Kill process on port 7890:**
```powershell
$port = Get-NetTCPConnection -LocalPort 7890 -ErrorAction SilentlyContinue
if ($port) {
    Stop-Process -Id $port.OwningProcess -Force
}
```

### Docker Not Found

**Verify Docker installation:**
```powershell
docker --version

# If not found, add to PATH:
$env:Path += ";C:\Program Files\Docker\Docker\resources\bin"
```

### Formation Logs Not Showing

**View logs:**
```powershell
# Formation output
Get-Content $env:APPDATA\muxi\server\logs\formation-<id>-out.log -Tail 50 -Wait

# Formation errors
Get-Content $env:APPDATA\muxi\server\logs\formation-<id>-err.log -Tail 50 -Wait

# Server audit log
Get-Content $env:APPDATA\muxi\server\logs\audit.log -Tail 50 -Wait
```

### Permission Denied

**Run as Administrator:**
- Right-click PowerShell → "Run as Administrator"
- Only needed for system-wide installation

**Or use user install:**
```powershell
# User install (no admin needed)
$env:MUXI_CONFIG_DIR = "$env:APPDATA\muxi\server"
muxi-server serve
```

### Antivirus Blocking

Some antivirus software may flag MUXI Server:

1. Add exception for `muxi-server.exe`
2. Add exception for `%LOCALAPPDATA%\muxi` directory
3. Windows Defender exclusions:

```powershell
Add-MpPreference -ExclusionPath "$env:LOCALAPPDATA\muxi"
```

---

## Development Workflow

### Typical Dev Session

```powershell
# Terminal 1 - Start MUXI Server
muxi-server serve

# Terminal 2 - Development
cd my-formation
python app.py  # Test locally first

# Deploy to MUXI
Compress-Archive -Path . -DestinationPath formation.zip -Force
Invoke-WebRequest `
  -Uri "http://localhost:7890/rpc/formations/deploy" `
  -Method POST `
  -ContentType "application/gzip" `
  -InFile formation.zip

# Test
Invoke-WebRequest http://localhost:8001/
```

### Auto-restart on Changes

Use `nodemon` or similar:

```powershell
npm install -g nodemon

nodemon --watch . --ext py,yaml --exec "muxi formation deploy"
```

---

## Performance Tips

### 1. Use SSD for Formations
Store formations on SSD for faster startup:
```powershell
$env:MUXI_DATA_DIR = "C:\formations"  # SSD location
```

### 2. Disable Windows Defender Real-time Scanning
For development directories:
```powershell
Add-MpPreference -ExclusionPath "C:\dev\muxi-formations"
```

### 3. WSL 2 for Docker
Enable WSL 2 backend in Docker Desktop for better performance.

### 4. Allocate More RAM to Docker
Docker Desktop → Settings → Resources → Memory (4GB+)

---

## Comparison: Windows vs Linux

| Feature | Windows | Linux |
|---------|---------|-------|
| **Binary Size** | ~15 MB | ~12 MB |
| **Process Management** | Job Objects | Process Groups |
| **Signals** | Ctrl+Break | SIGTERM |
| **SIF Runtime** | Docker wrapper | Native Singularity |
| **Service** | Manual (dev) | systemd (prod) |
| **Paths** | %APPDATA% | ~/.muxi |
| **Performance** | Good | Excellent |

---

## Uninstallation

### Remove MUXI Server

```powershell
# Stop server
Stop-Process -Name "muxi-server" -Force -ErrorAction SilentlyContinue

# Remove binary
Remove-Item "$env:LOCALAPPDATA\muxi\bin\muxi-server.exe" -Force

# Remove data (optional)
Remove-Item -Recurse -Force "$env:APPDATA\muxi"

# Remove from PATH
$path = [Environment]::GetEnvironmentVariable("Path", "User")
$newPath = ($path.Split(';') | Where-Object { $_ -ne "$env:LOCALAPPDATA\muxi\bin" }) -join ';'
[Environment]::SetEnvironmentVariable("Path", $newPath, "User")
```

---

## FAQ

**Q: Can I use MUXI Server in production on Windows?**  
A: While it works, we recommend Linux or Docker for production deployments. Windows support is optimized for development.

**Q: Do I need Docker Desktop?**  
A: Only if you're using SIF runtime. Native Python formations work without Docker.

**Q: Can I run MUXI Server as a Windows Service?**  
A: Not yet. For now, use background process or Windows Terminal. Service support may come in a future release if there's demand.

**Q: Does it work on Windows 11 ARM (Surface Pro X)?**  
A: Yes! We provide `windows-arm64` binaries.

**Q: Can I use WSL instead?**  
A: Yes! If you prefer Linux environment, run MUXI Server in WSL 2. It will use native Linux binaries.

**Q: How do I update MUXI Server?**  
A: Re-run the install script: `irm https://install.muxi.ai/windows.ps1 | iex`

---

## Additional Resources

- **GitHub:** https://github.com/muxi-ai/server
- **Issues:** https://github.com/muxi-ai/server/issues
- **API Reference:** [api-reference.md](./api-reference.md)
- **Installation:** [installation.md](./installation.md)
- **Troubleshooting:** [troubleshooting.md](./troubleshooting.md)

---

**Need help?** Open an issue on GitHub or check the troubleshooting guide.

**Happy developing on Windows! 🪟🚀**
