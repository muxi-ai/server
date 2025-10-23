# Your Questions - Answered

## 1. ScalVer (like onellm)

✅ **Created:** `.version` file with `0.20251023.0`

**Format:** `MAJOR.YYYYMMDD.PATCH`
- `0` - Alpha/experimental
- `20251023` - October 23, 2025
- `0` - First release today

**To bump version:**
```bash
# For a new release today
echo "0.20251023.1" > .version

# For tomorrow
echo "0.20251024.0" > .version

# When going stable
echo "1.20251101.0" > .version
```

**In GitHub Actions:**
- Reads `.version` file
- Builds with that version
- Tags release with `v0.20251023.0`

---

## 2. Git Commit & Build Time in `muxi-server version`

**Purpose:** Show exactly what code is running

```bash
$ muxi-server version
MUXI Server 0.20251023.0
Git Commit: a14a587      ← Which code
Build Time: 2025-10-23T18:47:32Z  ← When built
```

**How it works:**

### During Build
```bash
# Local build
go build -ldflags "\
  -X 'main.Version=$(cat .version)' \
  -X 'main.GitCommit=$(git rev-parse --short HEAD)' \
  -X 'main.BuildTime=$(date -u +%Y-%m-%dT%H:%M:%SZ)'" \
  -o muxi-server ./cmd/server

# GitHub Actions build (automatic)
- Reads .version
- Gets git commit SHA
- Records build timestamp
- Injects all 3 into binary
```

### In Code
```go
// src/cmd/server/commands.go
var (
    Version   = "dev"      // Default, gets overridden at build
    GitCommit = "unknown"  // Gets injected: a14a587
    BuildTime = "unknown"  // Gets injected: 2025-10-23T18:47:32Z
)
```

**Why this matters:**
- **Debug issues:** "Which build are you running?"
- **Rollback:** "Go back to commit a14a587"
- **Audit:** "Built on Oct 23 at 18:47 UTC"

---

## 3. Service Installation - Who Does It?

### Recommended: Install Script

```bash
curl -sSL https://install.muxi.ai | sudo bash

→ Installing MUXI Server...
✓ Binary installed
✓ Directories created

Install as system service? [Y/n]: y

→ Installing service...
✓ Created /etc/systemd/system/muxi-server.service
✓ Service enabled (starts on boot)
```

**Or manually later:**
```bash
sudo muxi-server install-service
```

**Why install script?**
- User expects it (like nginx, postgres install)
- One-time setup
- Can skip if not wanted

**Why not init command?**
- `init` is for config/credentials
- Keep service installation separate
- `init` doesn't require sudo on user installs

---

## 4. nginx-like Commands - YES! 🎯

### Proposed Commands

```bash
# Service Management (nginx-like!)
muxi-server start         # Start daemon
muxi-server stop          # Stop daemon
muxi-server restart       # Restart daemon
muxi-server reload        # Reload config
muxi-server status        # Show status

# Config Management
muxi-server configtest    # Like nginx -t

# Other
muxi-server serve         # Run in foreground (dev mode)
muxi-server init          # Initialize
muxi-server version       # Version info
```

### How They Work

**With Service Installed (Production):**
```bash
muxi-server start
  → systemctl start muxi-server    (Linux)
  → launchctl load ...plist         (macOS)

muxi-server status
  → systemctl status muxi-server
  → Shows: running/stopped, uptime, logs

muxi-server configtest
  → Validates /etc/muxi/server/config.yaml
  → Shows: "Config valid, restart to apply"
```

**Without Service (Development):**
```bash
muxi-server start
  → No service detected
  → Runs in foreground
  → Suggests: "Use install-service to run as daemon"

muxi-server serve
  → Explicit foreground mode
  → Same as current behavior
```

### Example Workflow

```bash
# Install as system service
curl -sSL https://install.muxi.ai | sudo bash
→ Installs binary + service

# Initialize config
sudo muxi-server init
→ Creates /etc/muxi/server/config.yaml

# Start service
sudo muxi-server start
→ ● muxi-server.service - MUXI Server
  Loaded: loaded
  Active: active (running)

# Check status
muxi-server status
→ Shows uptime, port, formations

# Edit config
sudo vim /etc/muxi/server/config.yaml

# Test config (like nginx -t)
sudo muxi-server configtest
→ ✓ Configuration is valid
  To apply: muxi-server restart

# Restart to apply
sudo muxi-server restart
→ ✓ Server restarted
```

---

## Summary

### 1. ScalVer ✅
- `.version` file created
- Format: `0.20251023.0`
- Used in builds and releases

### 2. Version Info ✅
- Shows: version, git commit, build time
- Injected at build via `-ldflags`
- Helps with debugging and rollbacks

### 3. Service Installation ✅
- Install script offers service setup
- Can also run `muxi-server install-service`
- Creates systemd (Linux) or launchd (macOS)

### 4. nginx-like Commands ✅
- `start`, `stop`, `restart`, `reload`, `status`
- `configtest` to validate config
- Smart detection (service vs foreground)

---

## Next Steps

**Want me to implement this?**

1. **Phase 1:** Version system (ScalVer + build info)
2. **Phase 2:** nginx-like commands
3. **Phase 3:** Service installation
4. **Phase 4:** Update install script

**Or adjust anything first?**
