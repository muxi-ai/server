# MUXI Server - Service/Daemon Design

**Status:** Design Document  
**Date:** 2025-10-23

---

## Overview

Transform MUXI Server into a proper system service with nginx-like daemon management commands.

---

## 1. Version System (ScalVer + Build Info)

### ScalVer Format

**Format:** `MAJOR.YYYYMMDD.PATCH`

**Example:** `0.20251023.0`
- `0` - Alpha/experimental (MAJOR)
- `20251023` - October 23, 2025 (DATE)
- `0` - First release today (PATCH)

**Version File:** `.version`
```
0.20251023.0
```

### Build-Time Information

Version command shows 3 pieces of info:

```bash
$ muxi-server version
MUXI Server 0.20251023.0
Git Commit: a14a587
Build Time: 2025-10-23T18:47:32Z
```

**How it works:**

1. **Version** - Read from `.version` file
2. **Git Commit** - Injected at build time via `-ldflags`
3. **Build Time** - Injected at build time via `-ldflags`

**Build Command:**
```bash
go build -ldflags "\
  -X 'main.Version=$(cat ../.version)' \
  -X 'main.GitCommit=$(git rev-parse --short HEAD)' \
  -X 'main.BuildTime=$(date -u +%Y-%m-%dT%H:%M:%SZ)'" \
  -o muxi-server ./cmd/server
```

**In code:**
```go
// src/cmd/server/commands.go
var (
    Version   = "dev"      // Default, overridden by build
    GitCommit = "unknown"  // Overridden by build
    BuildTime = "unknown"  // Overridden by build
)

func cmdVersion() error {
    fmt.Printf("MUXI Server %s\n", Version)
    fmt.Printf("Git Commit: %s\n", GitCommit)
    fmt.Printf("Build Time: %s\n", BuildTime)
    return nil
}
```

**Release Workflow** (GitHub Actions):
```yaml
- name: Get version
  id: version
  run: echo "VERSION=$(cat .version)" >> $GITHUB_OUTPUT

- name: Build with version info
  run: |
    VERSION="${{ steps.version.outputs.VERSION }}"
    COMMIT="${{ github.sha }}"
    BUILD_TIME=$(date -u +%Y-%m-%dT%H:%M:%SZ)
    
    go build -ldflags "\
      -X 'main.Version=$VERSION' \
      -X 'main.GitCommit=${COMMIT:0:7}' \
      -X 'main.BuildTime=$BUILD_TIME'" \
      -o muxi-server ./cmd/server
```

---

## 2. Service Installation Strategy

### Who Installs the Service?

**Option A: Install Script** (Recommended)
```bash
curl -sSL https://muxi.org/install | sudo bash
```
- Installs binary
- Creates directories
- **Offers to install service:** "Install as system service? [Y/n]"
- Creates systemd/launchd config
- Enables and starts service

**Option B: Init Command**
```bash
sudo muxi-server init
```
- Generates config
- Generates credentials
- **Offers to install service:** "Install as system service? [Y/n]"

**Option C: Dedicated Install-Service Command**
```bash
sudo muxi-server install-service
```
- Separate command for service installation
- Can be run anytime after init

**Recommendation:** **Option A** - Install script offers service setup during install

---

## 3. nginx-like Command Structure

### Current Commands
```bash
muxi-server init          # Initialize config
muxi-server start         # Run in foreground (current behavior)
muxi-server version       # Show version
muxi-server config show   # Show config
muxi-server help          # Help
```

### Proposed New Commands

```bash
# Daemon Management (requires service installation)
muxi-server start         # Start daemon
muxi-server stop          # Stop daemon
muxi-server restart       # Restart daemon
muxi-server reload        # Reload config without restart
muxi-server status        # Show daemon status

# Config Management
muxi-server configtest    # Validate config (like nginx -t)
muxi-server config show   # Show current config

# Service Installation
muxi-server install-service   # Install systemd/launchd service
muxi-server uninstall-service # Remove service

# Other
muxi-server init          # Initialize config
muxi-server version       # Version info
muxi-server help          # Help
```

### Command Behavior

#### Development Mode (No Service)
```bash
# Run in foreground
muxi-server serve         # New: explicit foreground mode
muxi-server start         # Checks for service first, falls back to serve

# Config validation
muxi-server configtest    # Always works (no service needed)
```

#### Production Mode (With Service)
```bash
# Service control (uses systemctl/launchctl)
muxi-server start         # systemctl start muxi-server
muxi-server stop          # systemctl stop muxi-server
muxi-server restart       # systemctl restart muxi-server
muxi-server reload        # systemctl reload muxi-server
muxi-server status        # systemctl status muxi-server

# Config validation
muxi-server configtest    # Validates, suggests restart if needed
```

---

## 4. Service Files

### systemd (Linux)

**File:** `/etc/systemd/system/muxi-server.service`

```ini
[Unit]
Description=MUXI Server - Formation Orchestration Platform
Documentation=https://docs.muxi.org
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=root
Group=root
ExecStart=/usr/local/bin/muxi-server serve
ExecReload=/bin/kill -HUP $MAINPID
Restart=on-failure
RestartSec=5s

# Security
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/etc/muxi/server /var/lib/muxi /var/log/muxi

# Limits
LimitNOFILE=65536

[Install]
WantedBy=multi-user.target
```

**Created by:**
```bash
sudo muxi-server install-service
```

**Commands:**
```bash
# Created during install-service:
sudo systemctl daemon-reload
sudo systemctl enable muxi-server
sudo systemctl start muxi-server

# User commands (via muxi-server):
muxi-server start    → sudo systemctl start muxi-server
muxi-server stop     → sudo systemctl stop muxi-server
muxi-server restart  → sudo systemctl restart muxi-server
muxi-server status   → sudo systemctl status muxi-server
```

### launchd (macOS)

**File:** `~/Library/LaunchAgents/ai.muxi.server.plist`

```xml
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>ai.muxi.server</string>
    
    <key>ProgramArguments</key>
    <array>
        <string>/usr/local/bin/muxi-server</string>
        <string>serve</string>
    </array>
    
    <key>RunAtLoad</key>
    <true/>
    
    <key>KeepAlive</key>
    <true/>
    
    <key>StandardOutPath</key>
    <string>/usr/local/var/log/muxi-server.log</string>
    
    <key>StandardErrorPath</key>
    <string>/usr/local/var/log/muxi-server-error.log</string>
    
    <key>EnvironmentVariables</key>
    <dict>
        <key>PATH</key>
        <string>/usr/local/bin:/usr/bin:/bin:/usr/sbin:/sbin</string>
    </dict>
</dict>
</plist>
```

**Created by:**
```bash
muxi-server install-service  # No sudo on macOS (user service)
```

**Commands:**
```bash
muxi-server start    → launchctl load ~/Library/LaunchAgents/ai.muxi.server.plist
muxi-server stop     → launchctl unload ~/Library/LaunchAgents/ai.muxi.server.plist
muxi-server restart  → unload + load
muxi-server status   → launchctl list | grep ai.muxi.server
```

---

## 5. Implementation Plan

### Phase 1: Version System
- [x] Create `.version` file
- [ ] Update build to inject git commit and build time
- [ ] Update version command to show all 3 pieces
- [ ] Test local builds
- [ ] Update GitHub Actions workflow

### Phase 2: New Commands
- [ ] Add `serve` command (foreground mode)
- [ ] Add `configtest` command
- [ ] Update `start` to detect service
- [ ] Add `stop`, `restart`, `reload`, `status` commands
- [ ] Add service detection logic

### Phase 3: Service Installation
- [ ] Add `install-service` command
- [ ] Create systemd template
- [ ] Create launchd template
- [ ] Add `uninstall-service` command
- [ ] Test on Linux and macOS

### Phase 4: Install Script Integration
- [ ] Update install.sh to offer service installation
- [ ] Test end-to-end flow
- [ ] Update documentation

---

## 6. Command Flow Examples

### Fresh Install (System)

```bash
# Install
curl -sSL https://muxi.org/install | sudo bash

→ MUXI Server installed successfully!
  Install as system service? [Y/n]: y

→ Installing service...
✓ Service installed: /etc/systemd/system/muxi-server.service
✓ Service enabled (starts on boot)

# Initialize
sudo muxi-server init

→ Server initialized at /etc/muxi/server/config.yaml
  Start server now? [Y/n]: y

→ Starting server...
✓ Server started successfully
  Status: muxi-server status

# Check status
muxi-server status

● muxi-server.service - MUXI Server
   Loaded: loaded (/etc/systemd/system/muxi-server.service)
   Active: active (running) since Wed 2025-10-23 18:47:32 UTC
   Main PID: 1234
   ...
```

### Development Mode (No Service)

```bash
# Install without service
curl -sSL https://muxi.org/install | bash

# Initialize
muxi-server init

# Run in foreground
muxi-server serve

→ MUXI Server (v0.20251023.0): Starting...
→ Installation: User-level
→ Configuration loaded (/Users/ran/.muxi/server/config.yaml)
→ MUXI Server listening on 0.0.0.0:7890

# Or start (falls back to serve if no service)
muxi-server start

→ No service installed. Running in foreground...
→ (Use muxi-server install-service to install as daemon)
```

### Config Testing

```bash
# Edit config
sudo vim /etc/muxi/server/config.yaml

# Test config
sudo muxi-server configtest

→ Checking configuration...
✓ Configuration is valid

  Server:     0.0.0.0:7890
  Formations: 8000-9000
  Auth:       Enabled

  To apply changes:
    muxi-server restart

# Restart to apply
sudo muxi-server restart

→ Restarting server...
✓ Server restarted successfully
```

---

## 7. Service Detection Logic

```go
// detectServiceInstalled checks if service is installed
func detectServiceInstalled() bool {
    if runtime.GOOS == "linux" {
        // Check for systemd service
        _, err := os.Stat("/etc/systemd/system/muxi-server.service")
        return err == nil
    } else if runtime.GOOS == "darwin" {
        // Check for launchd plist
        plist := filepath.Join(os.Getenv("HOME"), "Library/LaunchAgents/ai.muxi.server.plist")
        _, err := os.Stat(plist)
        return err == nil
    }
    return false
}

// cmdStart handles 'start' command
func cmdStart() error {
    if detectServiceInstalled() {
        // Service installed - use service manager
        if runtime.GOOS == "linux" {
            return exec.Command("systemctl", "start", "muxi-server").Run()
        } else if runtime.GOOS == "darwin" {
            plist := filepath.Join(os.Getenv("HOME"), "Library/LaunchAgents/ai.muxi.server.plist")
            return exec.Command("launchctl", "load", plist).Run()
        }
    }
    
    // No service - run in foreground
    fmt.Println("No service installed. Running in foreground...")
    fmt.Println("(Use 'muxi-server install-service' to install as daemon)")
    return cmdServe()
}
```

---

## 8. Config Validation

```go
func cmdConfigTest() error {
    fmt.Println("Checking configuration...")
    
    // Load config
    configPath, err := config.GetConfigPath()
    if err != nil {
        return fmt.Errorf("failed to get config path: %w", err)
    }
    
    cfg, err := config.Load(configPath)
    if err != nil {
        fmt.Printf("✗ Configuration error: %v\n", err)
        return err
    }
    
    // Validate
    if err := cfg.Validate(); err != nil {
        fmt.Printf("✗ Configuration invalid: %v\n", err)
        return err
    }
    
    fmt.Println("✓ Configuration is valid")
    fmt.Println()
    fmt.Printf("  Server:     %s:%d\n", cfg.Server.Host, cfg.Server.Port)
    fmt.Printf("  Formations: %d-%d\n", cfg.Formations.PortRangeStart, cfg.Formations.PortRangeEnd)
    fmt.Printf("  Auth:       %s\n", map[bool]string{true: "Enabled", false: "Disabled"}[cfg.Auth.Enabled])
    fmt.Println()
    
    // Check if service is running
    if detectServiceInstalled() {
        fmt.Println("  To apply changes:")
        fmt.Println("    muxi-server restart")
    }
    
    return nil
}
```

---

## 9. Questions & Decisions

### Q1: Should `muxi-server start` require sudo?

**Option A:** Require sudo for service commands
```bash
sudo muxi-server start
```

**Option B:** Auto-detect and sudo internally
```bash
muxi-server start  # Auto runs with sudo if needed
```

**Option C:** Wrapper script
```bash
muxi  # Wrapper that handles sudo
```

**Recommendation:** **Option A** - Be explicit about privilege requirements

### Q2: Should configtest automatically restart?

**No** - nginx style is:
1. Test: `nginx -t`
2. Reload: `nginx -s reload`

Keep them separate for safety.

### Q3: What about Windows?

**Future:** Windows Service using sc.exe or nssm.exe  
**Current:** Run in foreground or use Task Scheduler

---

## 10. Documentation Updates Needed

- [ ] `docs/installation.md` - Service installation
- [ ] `docs/getting-started.md` - Service commands
- [ ] `docs/configuration.md` - configtest usage
- [ ] `README.md` - Quick start with service
- [ ] `AGENTS.md` - Development without service

---

**Status:** Ready for Implementation  
**Next Step:** Implement Phase 1 (Version System with ScalVer)
