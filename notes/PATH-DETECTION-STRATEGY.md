# MUXI Server Path Detection Strategy

**Version:** 1.0  
**Last Updated:** 2025-01-23  
**Status:** Design Document - To Be Implemented

---

## Overview

MUXI Server uses **intelligent path detection** based on installation method and platform. This allows it to work seamlessly as:
- **Production service** on Linux (system-wide, like Nginx)
- **User application** on macOS/Windows (no sudo required)
- **Development tool** (manual binary)

---

## Core Principle

> **The installer decides the scope, the binary detects and adapts.**

- Install with `sudo` → System paths (`/etc`, `/var/lib`, `/var/log`)
- Install without `sudo` → User paths (`~/.muxi/server`)
- Platform matters → Linux allows system install, macOS/Windows are user-only

---

## Path Detection Logic

### Configuration Directory

**Purpose:** Where `config.yaml` lives

```go
func GetConfigDir() (string, error) {
    // 1. Environment override (highest priority)
    if dir := os.Getenv("MUXI_CONFIG_DIR"); dir != "" {
        return dir, nil
    }
    
    // 2. Platform + binary location detection
    exe, err := os.Executable()
    if err == nil {
        // Linux + installed in /usr → system paths
        if runtime.GOOS == "linux" && strings.HasPrefix(exe, "/usr/") {
            return "/etc/muxi/server", nil
        }
    }
    
    // 3. User paths (macOS, Windows, or non-system Linux)
    home, err := os.UserHomeDir()
    if err != nil {
        return "", err
    }
    return filepath.Join(home, ".muxi", "server"), nil
}
```

**Decision Tree:**
```
MUXI_CONFIG_DIR set?
├─ YES → Use that path
└─ NO → Is OS Linux AND binary in /usr/*?
       ├─ YES → /etc/muxi/server
       └─ NO  → ~/.muxi/server
```

### Data Directory

**Purpose:** Where formations, registry.json, runtime files live

```go
func GetDataDir() (string, error) {
    if dir := os.Getenv("MUXI_DATA_DIR"); dir != "" {
        return dir, nil
    }
    
    exe, _ := os.Executable()
    if runtime.GOOS == "linux" && strings.HasPrefix(exe, "/usr/") {
        return "/var/lib/muxi", nil
    }
    
    home, err := os.UserHomeDir()
    if err != nil {
        return "", err
    }
    return filepath.Join(home, ".muxi", "server"), nil
}
```

**Decision Tree:** Same as config, but returns `/var/lib/muxi` or `~/.muxi/server`

### Log Directory

**Purpose:** Where audit logs and formation logs live

```go
func GetLogDir() (string, error) {
    if dir := os.Getenv("MUXI_LOG_DIR"); dir != "" {
        return dir, nil
    }
    
    exe, _ := os.Executable()
    if runtime.GOOS == "linux" && strings.HasPrefix(exe, "/usr/") {
        return "/var/log/muxi", nil
    }
    
    home, err := os.UserHomeDir()
    if err != nil {
        return "", err
    }
    return filepath.Join(home, ".muxi", "server", "logs"), nil
}
```

**Decision Tree:** Same logic, returns `/var/log/muxi` or `~/.muxi/server/logs`

---

## Installation Methods

### 1. One-Command Install Script

**URL:** `https://install.muxi.org` (or `get.muxi.org`)

#### System Install (with sudo)

```bash
curl -sSL https://install.muxi.org | sudo bash
```

**What it does:**
```bash
#!/bin/bash

# Detect if running as root
if [ "$EUID" = 0 ]; then
  echo "Installing MUXI Server (system-wide)..."
  
  # Install binary
  INSTALL_DIR="/usr/local/bin"
  curl -L -o "$INSTALL_DIR/muxi-server" \
    https://github.com/muxi-ai/server/releases/latest/download/muxi-server-$(uname -s)-$(uname -m)
  chmod +x "$INSTALL_DIR/muxi-server"
  
  # Create system directories (Linux only)
  if [ "$(uname)" = "Linux" ]; then
    mkdir -p /etc/muxi/server
    mkdir -p /var/lib/muxi
    mkdir -p /var/log/muxi
    chmod 755 /etc/muxi/server /var/lib/muxi /var/log/muxi
    
    echo "✓ Installed system-wide"
    echo "  Binary: /usr/local/bin/muxi-server"
    echo "  Config: /etc/muxi/server/"
    echo "  Data:   /var/lib/muxi/"
    echo "  Logs:   /var/log/muxi/"
    echo ""
    echo "Run: sudo muxi-server init"
  fi
fi
```

**Result:**
- Binary: `/usr/local/bin/muxi-server` → detected as system install
- Paths: `/etc/muxi/server`, `/var/lib/muxi`, `/var/log/muxi`

#### User Install (no sudo)

```bash
curl -sSL https://install.muxi.org | bash
```

**What it does:**
```bash
else
  # USER INSTALL (no sudo)
  echo "Installing MUXI Server (user-level)..."
  
  # Install binary
  INSTALL_DIR="$HOME/.local/bin"
  mkdir -p "$INSTALL_DIR"
  
  curl -L -o "$INSTALL_DIR/muxi-server" \
    https://github.com/muxi-ai/server/releases/latest/download/muxi-server-$(uname -s)-$(uname -m)
  chmod +x "$INSTALL_DIR/muxi-server"
  
  # Add to PATH if needed
  if [[ ":$PATH:" != *":$HOME/.local/bin:"* ]]; then
    echo ""
    echo "⚠️  Add to PATH:"
    
    # Detect shell
    if [ -n "$BASH_VERSION" ]; then
      echo "  echo 'export PATH=\"\$HOME/.local/bin:\$PATH\"' >> ~/.bashrc"
      echo "  source ~/.bashrc"
    elif [ -n "$ZSH_VERSION" ]; then
      echo "  echo 'export PATH=\"\$HOME/.local/bin:\$PATH\"' >> ~/.zshrc"
      echo "  source ~/.zshrc"
    else
      echo "  export PATH=\"\$HOME/.local/bin:\$PATH\""
    fi
  fi
  
  echo ""
  echo "✓ Installed user-level"
  echo "  Binary: ~/.local/bin/muxi-server"
  echo "  Config: ~/.muxi/server/"
  echo ""
  echo "Run: muxi-server init"
fi
```

**Result:**
- Binary: `~/.local/bin/muxi-server` → detected as user install
- Paths: `~/.muxi/server/*`
- **PATH updated** (or instructions shown)

---

### 2. Package Managers

#### APT (Debian/Ubuntu)

```bash
sudo apt install muxi-server
```

**Package behavior:**
- Installs to: `/usr/bin/muxi-server`
- Creates directories: `/etc/muxi/server`, `/var/lib/muxi`, `/var/log/muxi`
- Sets permissions: `755` for directories
- Post-install script: Runs `muxi-server init` if `/etc/muxi/server/config.yaml` doesn't exist
- Creates systemd service: `/etc/systemd/system/muxi-server.service`

**Result:**
- Binary: `/usr/bin/muxi-server` → detected as system install
- Paths: System paths

#### YUM/DNF (RedHat/Fedora/CentOS)

```bash
sudo yum install muxi-server
```

**Same behavior as APT** (uses RPM package)

#### Homebrew (macOS)

```bash
brew install muxi-server
```

**Package behavior:**
- Installs to: `/usr/local/bin/muxi-server` (Intel) or `/opt/homebrew/bin/muxi-server` (ARM)
- Does **NOT** create system directories
- Post-install message: "Run `muxi-server init` to configure"
- Creates launchd plist: `~/Library/LaunchAgents/ai.muxi.server.plist` (user service)

**Result:**
- Binary: `/usr/local/bin/muxi-server` on macOS → detected as user install
- Paths: `~/.muxi/server/*` (macOS doesn't use system paths for user apps)

#### Chocolatey (Windows) - Future

```powershell
choco install muxi-server
```

**Package behavior:**
- Installs to: `C:\Program Files\muxi\muxi-server.exe`
- Uses: `%USERPROFILE%\.muxi\server\` (Windows doesn't have `/etc`)

---

## Path Resolution Matrix

| Install Method | OS | Binary Location | Config Dir | Data Dir | Log Dir |
|----------------|-----|-----------------|------------|----------|---------|
| `curl \| sudo bash` | Linux | `/usr/local/bin/muxi-server` | `/etc/muxi/server` | `/var/lib/muxi` | `/var/log/muxi` |
| `curl \| bash` | Linux | `~/.local/bin/muxi-server` | `~/.muxi/server` | `~/.muxi/server` | `~/.muxi/server/logs` |
| `apt/yum install` | Linux | `/usr/bin/muxi-server` | `/etc/muxi/server` | `/var/lib/muxi` | `/var/log/muxi` |
| `curl \| sudo bash` | macOS | `/usr/local/bin/muxi-server` | `~/.muxi/server` | `~/.muxi/server` | `~/.muxi/server/logs` |
| `curl \| bash` | macOS | `~/.local/bin/muxi-server` | `~/.muxi/server` | `~/.muxi/server` | `~/.muxi/server/logs` |
| `brew install` | macOS | `/usr/local/bin/muxi-server` | `~/.muxi/server` | `~/.muxi/server` | `~/.muxi/server/logs` |
| Manual download | Any | Anywhere | `~/.muxi/server` | `~/.muxi/server` | `~/.muxi/server/logs` |

**Key Insight:** On macOS, even `/usr/local/bin` uses user paths (runtime.GOOS != "linux")

---

## Environment Variable Overrides

All path detection can be overridden with environment variables:

```bash
# Override config directory
export MUXI_CONFIG_DIR=/custom/config

# Override data directory
export MUXI_DATA_DIR=/custom/data

# Override log directory
export MUXI_LOG_DIR=/custom/logs

muxi-server start
```

**Use cases:**
- Docker containers (set paths to mounted volumes)
- Custom deployments
- Testing/development
- Multi-instance servers

---

## PATH Management (User Installs)

### Problem

When installing to `~/.local/bin`, the binary won't be in PATH by default on many systems.

### Solution

The install script **detects and fixes** this:

```bash
# Check if ~/.local/bin is in PATH
if [[ ":$PATH:" != *":$HOME/.local/bin:"* ]]; then
  echo ""
  echo "⚠️  Adding to PATH..."
  
  # Detect shell and add to appropriate rc file
  if [ -f "$HOME/.bashrc" ] && [ -z "$ZSH_VERSION" ]; then
    echo 'export PATH="$HOME/.local/bin:$PATH"' >> ~/.bashrc
    echo "✓ Added to ~/.bashrc"
    echo "  Run: source ~/.bashrc"
  elif [ -f "$HOME/.zshrc" ]; then
    echo 'export PATH="$HOME/.local/bin:$PATH"' >> ~/.zshrc
    echo "✓ Added to ~/.zshrc"
    echo "  Run: source ~/.zshrc"
  else
    # Fallback: show manual instructions
    echo "Add this to your shell profile:"
    echo '  export PATH="$HOME/.local/bin:$PATH"'
  fi
fi
```

**Supported shells:**
- bash → Appends to `~/.bashrc`
- zsh → Appends to `~/.zshrc`
- fish → Appends to `~/.config/fish/config.fish`
- Others → Shows manual instructions

---

## Startup Logs

The server should **clearly show** which paths it's using on startup:

### System Install

```
6:11PM INF MUXI Server (v1.0.0): Starting...
6:11PM INF Installation: System (Linux)
6:11PM INF Configuration loaded (/etc/muxi/server/config.yaml)
6:11PM INF Server ID: server-hostname-abc123
6:11PM INF Data directory: /var/lib/muxi
6:11PM INF Logs directory: /var/log/muxi
6:11PM INF MUXI Server listening on 0.0.0.0:7890
```

### User Install

```
6:11PM INF MUXI Server (v1.0.0): Starting...
6:11PM INF Installation: User-level
6:11PM INF Configuration loaded (/Users/ran/.muxi/server/config.yaml)
6:11PM INF Server ID: server-macbook-abc123
6:11PM INF Data directory: /Users/ran/.muxi/server
6:11PM INF Logs directory: /Users/ran/.muxi/server/logs
6:11PM INF MUXI Server listening on 0.0.0.0:7890
```

### Development (Manual Binary)

```
6:11PM INF MUXI Server (v1.0.0-dev): Starting...
6:11PM INF Installation: Development
6:11PM INF Configuration loaded (/Users/ran/.muxi/server/config.yaml)
6:11PM INF Server ID: server-macbook-abc123
6:11PM INF Data directory: /Users/ran/.muxi/server
6:11PM INF Logs directory: /Users/ran/.muxi/server/logs
6:11PM INF MUXI Server listening on 0.0.0.0:7890
```

---

## Migration Strategy

### Existing Installs (Current: `~/.muxi/server`)

**No breaking changes:**
- All existing installs continue using `~/.muxi/server`
- New system installs use `/etc/muxi/server`
- Both work simultaneously

**Migration path (optional):**
```bash
# If user wants to migrate to system install
sudo mkdir -p /etc/muxi/server /var/lib/muxi /var/log/muxi
sudo cp ~/.muxi/server/config.yaml /etc/muxi/server/
sudo cp -r ~/.muxi/server/formations /var/lib/muxi/
sudo cp ~/.muxi/server/registry.json /var/lib/muxi/
sudo chown -R root:root /etc/muxi/server /var/lib/muxi /var/log/muxi

# Move binary to system location
sudo mv ~/.local/bin/muxi-server /usr/local/bin/muxi-server

# Now server detects system install
muxi-server start
```

---

## Implementation Checklist

### Phase 1: Update Code (This PR)

- [ ] Update `src/pkg/config/config.go`:
  - [ ] Replace `GetMuxiDir()` with `GetConfigDir()`, `GetDataDir()`, `GetLogDir()`
  - [ ] Implement platform + binary location detection
  - [ ] Add environment variable support
- [ ] Update all callers:
  - [ ] `main.go` - Use appropriate directory functions
  - [ ] `registry/persistence.go` - Use `GetDataDir()`
  - [ ] `process/manager.go` - Use `GetDataDir()` and `GetLogDir()`
  - [ ] `api/server.go` - Use `GetLogDir()` for audit logs
- [ ] Update startup logs:
  - [ ] Show installation type (System/User/Development)
  - [ ] Show all paths being used
  - [ ] Remove verbose initialization logs
- [ ] Update tests:
  - [ ] Test path detection logic
  - [ ] Test environment variable overrides
  - [ ] Mock binary location for testing

### Phase 2: Install Script (Next PR)

- [ ] Create `install.sh`:
  - [ ] Detect sudo (root vs user install)
  - [ ] Platform detection (Linux/macOS/Windows)
  - [ ] Download appropriate binary
  - [ ] Create directories
  - [ ] PATH management for user installs
  - [ ] Show appropriate next steps
- [ ] Host at `https://install.muxi.org`
- [ ] Test on:
  - [ ] Ubuntu 22.04 (sudo)
  - [ ] Ubuntu 22.04 (user)
  - [ ] macOS (sudo)
  - [ ] macOS (user)
  - [ ] CentOS/RHEL

### Phase 3: Package Managers (Q1 2025)

- [ ] Debian package (`.deb`):
  - [ ] Create package structure
  - [ ] Post-install script (create dirs, run init)
  - [ ] systemd service file
- [ ] RPM package (`.rpm`):
  - [ ] Same as .deb but for YUM
- [ ] Homebrew formula:
  - [ ] Create formula
  - [ ] User-level install
  - [ ] launchd plist

### Phase 4: Documentation Updates

- [ ] Update `docs/installation.md`:
  - [ ] Document sudo vs non-sudo behavior
  - [ ] Document path detection
  - [ ] Document environment variables
- [ ] Update `docs/configuration.md`:
  - [ ] Document path configuration
  - [ ] Show examples for both install types
- [ ] Update `docs/troubleshooting.md`:
  - [ ] Add "Which paths am I using?" section
  - [ ] Add PATH issues troubleshooting
- [ ] Update `README.md`:
  - [ ] Update install instructions
  - [ ] Clarify system vs user install

---

## Security Considerations

### Permissions

**System install:**
```bash
# Config (readable by all, writable by root)
chmod 644 /etc/muxi/server/config.yaml
chown root:root /etc/muxi/server/config.yaml

# Data (writable by muxi service)
chmod 755 /var/lib/muxi
chown muxi:muxi /var/lib/muxi

# Logs (writable by muxi service)
chmod 755 /var/log/muxi
chown muxi:muxi /var/log/muxi
```

**User install:**
```bash
# Everything owned by user
chmod 755 ~/.muxi/server
chmod 644 ~/.muxi/server/config.yaml
```

### Credentials Protection

Both system and user installs:
```bash
# Config contains credentials - restrict read access
chmod 600 config.yaml  # Only owner can read
```

---

## Testing Strategy

### Manual Testing

```bash
# Test 1: System install (Linux)
curl -sSL https://install.muxi.org | sudo bash
sudo muxi-server init
sudo muxi-server start
# Verify: Uses /etc, /var/lib, /var/log

# Test 2: User install (Linux)
curl -sSL https://install.muxi.org | bash
muxi-server init
muxi-server start
# Verify: Uses ~/.muxi/server

# Test 3: Homebrew (macOS)
brew install muxi-server
muxi-server init
muxi-server start
# Verify: Uses ~/.muxi/server

# Test 4: Environment override
export MUXI_CONFIG_DIR=/tmp/muxi-test
muxi-server init
# Verify: Uses /tmp/muxi-test
```

### Automated Testing

```go
func TestPathDetection(t *testing.T) {
    tests := []struct {
        name       string
        os         string
        binaryPath string
        envVar     string
        expected   string
    }{
        {"Linux system", "linux", "/usr/bin/muxi-server", "", "/etc/muxi/server"},
        {"Linux user", "linux", "/home/user/.local/bin/muxi-server", "", "/home/user/.muxi/server"},
        {"macOS system", "darwin", "/usr/local/bin/muxi-server", "", "/Users/user/.muxi/server"},
        {"Override", "linux", "/usr/bin/muxi-server", "/custom", "/custom"},
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // Mock runtime.GOOS, os.Executable(), os.Getenv()
            // Test GetConfigDir()
        })
    }
}
```

---

## Open Questions

1. **Should we support `XDG_CONFIG_HOME`?**
   - Standard: `$XDG_CONFIG_HOME/muxi/server/` or `~/.config/muxi/server/`
   - Decision: **No** - Keep it simple with `~/.muxi/server/`

2. **What about Windows?**
   - Use `%USERPROFILE%\.muxi\server\` (no system install option)
   - Future: Consider `%PROGRAMDATA%\muxi\` for system service

3. **Multi-user servers?**
   - System install serves all users
   - Each user can have their own user-level install if needed
   - Environment variables allow custom paths

4. **Container deployments?**
   - Use environment variables to set paths to mounted volumes
   - Example: `MUXI_DATA_DIR=/data MUXI_LOG_DIR=/logs`

---

## Related Documents

- `INSTALLATION-ROADMAP.md` - Package manager implementation plan
- `docs/installation.md` - User-facing installation guide
- `wip/API-ARCHITECTURE-IMPLEMENTATION-PLAN-v2.md` - Original system paths spec

---

**Status:** Ready for Implementation  
**Next Step:** Implement Phase 1 (Update Code)
