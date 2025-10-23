# MUXI Server Install Script

**Script:** `install.sh`  
**Hosted at:** `https://install.muxi.ai` (or `https://get.muxi.ai`)  
**Last Updated:** 2025-01-23

---

## Overview

The MUXI Server install script provides a one-command installation experience similar to other modern CLI tools. It automatically detects your platform, downloads the appropriate binary, and configures your system.

**Key Features:**
- ✅ Automatic platform detection (Linux/macOS, amd64/arm64)
- ✅ Smart install scope (system vs user based on `sudo`)
- ✅ PATH management (auto-adds to shell config)
- ✅ Interactive initialization option
- ✅ Colorful, informative output
- ✅ Comprehensive error handling

---

## Usage

### System Install (Linux with sudo)

```bash
curl -sSL https://install.muxi.ai | sudo bash
```

**What it does:**
- Installs binary to `/usr/local/bin/muxi-server`
- Creates system directories:
  - `/etc/muxi/server` (config)
  - `/var/lib/muxi` (data)
  - `/var/log/muxi` (logs)
- Sets appropriate permissions
- Ready for system service use

**After install:**
```bash
sudo muxi-server init    # Initialize with system paths
sudo muxi-server start   # Start server
```

### User Install (No sudo)

```bash
curl -sSL https://install.muxi.ai | bash
```

**What it does:**
- Installs binary to `~/.local/bin/muxi-server`
- Creates `~/.muxi/server/` directory
- Adds `~/.local/bin` to PATH (if needed)
- Updates shell config file (`.bashrc`, `.zshrc`, etc.)

**After install:**
```bash
source ~/.bashrc         # Reload shell (or open new terminal)
muxi-server init        # Initialize with user paths
muxi-server start       # Start server
```

### macOS Install

```bash
curl -sSL https://install.muxi.ai | bash
```

**Note:** Even with `sudo`, macOS uses user-level paths (`~/.muxi/server`) because macOS apps typically don't use `/etc` and `/var` for third-party software.

---

## Install Options

### Specify Version

```bash
# Install specific version
export MUXI_VERSION=v1.0.0
curl -sSL https://install.muxi.ai | bash

# Or inline
curl -sSL https://install.muxi.ai | MUXI_VERSION=v1.0.0 bash
```

### Non-Interactive

```bash
# Skip the "Run init now?" prompt
curl -sSL https://install.muxi.ai | bash -s -- --non-interactive
```

---

## What Gets Installed

### System Install (Linux + sudo)

```
/usr/local/bin/
  └── muxi-server          ← Binary (executable)

/etc/muxi/server/
  └── (empty)              ← Config directory (created by init)

/var/lib/muxi/
  └── (empty)              ← Data directory (formations, registry)

/var/log/muxi/
  └── (empty)              ← Logs directory
```

### User Install (No sudo)

```
~/.local/bin/
  └── muxi-server          ← Binary (executable)

~/.muxi/server/
  └── (empty)              ← All paths (config, data, logs)

~/.bashrc (or ~/.zshrc)
  └── export PATH="$HOME/.local/bin:$PATH"  ← Added automatically
```

---

## Platform Support

| Platform | Architecture | Status | Notes |
|----------|-------------|--------|-------|
| Linux | amd64 (x86_64) | ✅ Supported | Primary platform |
| Linux | arm64 (aarch64) | ✅ Supported | ARM servers, Raspberry Pi 4+ |
| macOS | amd64 (Intel) | ✅ Supported | Intel Macs |
| macOS | arm64 (Apple Silicon) | ✅ Supported | M1/M2/M3 Macs |
| Windows | amd64 | 🔜 Planned | WSL2 works today |

---

## Detection Logic

### Platform Detection

```bash
# Detect OS
uname -s  → Linux, Darwin (macOS), etc.

# Detect Architecture
uname -m  → x86_64, aarch64, arm64, etc.

# Map to binary names
muxi-server-linux-amd64
muxi-server-linux-arm64
muxi-server-darwin-amd64
muxi-server-darwin-arm64
```

### Install Type Detection

```bash
# Check if root
if [ "$EUID" = 0 ]; then
    # Running with sudo
    
    if [ "$OS" = "linux" ]; then
        # System install (Linux only)
        INSTALL_DIR="/usr/local/bin"
        CONFIG_DIR="/etc/muxi/server"
        DATA_DIR="/var/lib/muxi"
        LOG_DIR="/var/log/muxi"
    else
        # macOS with sudo still uses user paths
        INSTALL_TYPE="user"
    fi
else
    # User install
    INSTALL_DIR="$HOME/.local/bin"
    # Everything in ~/.muxi/server
fi
```

### Shell Detection for PATH

```bash
# Detect current shell
SHELL_NAME=$(basename "$SHELL")

# Update appropriate config
bash   → ~/.bashrc (Linux) or ~/.bash_profile (macOS)
zsh    → ~/.zshrc
fish   → ~/.config/fish/config.fish
```

---

## Error Handling

### Network Issues

```bash
✗ Failed to download MUXI Server

Possible reasons:
  • No release available for darwin/arm64
  • Network connectivity issues
  • GitHub API rate limiting

Try downloading manually from:
  https://github.com/muxi-ai/server/releases
```

### Unsupported Platform

```bash
✗ Unsupported architecture: i686
  MUXI Server requires 64-bit architecture (amd64 or arm64)
```

### Missing Dependencies

```bash
✗ curl is required but not installed

Install with:
  sudo apt install curl  (Debian/Ubuntu)
  sudo yum install curl  (RHEL/CentOS)
  brew install curl      (macOS)
```

---

## Output Examples

### Successful System Install

```
███╗   ███╗██╗   ██╗██╗  ██╗██╗
████╗ ████║██║   ██║╚██╗██╔╝██║
██╔████╔██║██║   ██║ ╚███╔╝ ██║
██║╚██╔╝██║██║   ██║ ██╔██╗ ██║
██║ ╚═╝ ██║╚██████╔╝██╔╝ ██╗██║
╚═╝     ╚═╝ ╚═════╝ ╚═╝  ╚═╝╚═╝

MUXI Server Installation

→ Installing MUXI Server (system-wide)
→ Platform: linux/amd64
→ Binary: /usr/local/bin/muxi-server
→ Config: /etc/muxi/server
→ Data:   /var/lib/muxi
→ Logs:   /var/log/muxi

→ Downloading MUXI Server from GitHub...
✓ Downloaded successfully
→ Creating directories...
✓ Created system directories
→ Installing binary to /usr/local/bin/muxi-server...
✓ Binary installed
✓ Verified: MUXI Server 1.0.0

────────────────────────────────────────────────────────────
✓ MUXI Server installed successfully!
────────────────────────────────────────────────────────────

Next steps:

  1. Initialize the server:
     sudo muxi-server init

  2. Start the server:
     sudo muxi-server start

  3. Check server status:
     curl http://localhost:7890/health

Documentation: https://docs.muxi.ai/getting-started
Repository:    https://github.com/muxi-ai/server
```

### Successful User Install

```
[Same header]

→ Installing MUXI Server (user-level)
→ Platform: darwin/arm64
→ Binary: /Users/ran/.local/bin/muxi-server
→ Paths:  ~/.muxi/server/

→ Downloading MUXI Server from GitHub...
✓ Downloaded successfully
→ Creating directories...
✓ Created user directories
→ Installing binary to /Users/ran/.local/bin/muxi-server...
✓ Binary installed
✓ Verified: MUXI Server 1.0.0

! Adding /Users/ran/.local/bin to PATH

✓ Added to /Users/ran/.zshrc

→ Reload your shell:
  source /Users/ran/.zshrc

Or open a new terminal window

────────────────────────────────────────────────────────────
✓ MUXI Server installed successfully!
────────────────────────────────────────────────────────────

Next steps:

  1. Initialize the server:
     muxi-server init

  2. Start the server:
     muxi-server start

  3. Check server status:
     curl http://localhost:7890/health

Documentation: https://docs.muxi.ai/getting-started
Repository:    https://github.com/muxi-ai/server
```

---

## Security Considerations

### Download Verification

Currently, the script downloads directly from GitHub Releases. Future enhancements:

```bash
# TODO: Add checksum verification
CHECKSUM_URL="https://github.com/${REPO}/releases/download/${VERSION}/checksums.txt"
curl -fsSL "$CHECKSUM_URL" | sha256sum -c --ignore-missing
```

### Script Inspection

Users can inspect the script before running:

```bash
# Download and inspect
curl -sSL https://install.muxi.ai > install.sh
less install.sh

# Run manually
bash install.sh
```

### HTTPS Only

All downloads use HTTPS to prevent man-in-the-middle attacks.

---

## Testing

### Local Testing

```bash
# Test the script locally
cd /path/to/muxi/server
./install.sh

# Test system install (requires sudo)
sudo ./install.sh

# Test specific version
MUXI_VERSION=v0.9.0 ./install.sh
```

### Platform Testing Matrix

| Platform | Sudo | Expected Result | Tested |
|----------|------|-----------------|--------|
| Ubuntu 22.04 | Yes | System install to /usr | ✅ |
| Ubuntu 22.04 | No | User install to ~/.local/bin | ✅ |
| macOS (Intel) | Yes | User install (no system paths) | ✅ |
| macOS (Intel) | No | User install to ~/.local/bin | ✅ |
| macOS (ARM) | Yes | User install (no system paths) | ✅ |
| macOS (ARM) | No | User install to ~/.local/bin | ✅ |
| RHEL 9 | Yes | System install to /usr | ⏳ |
| Debian 12 | Yes | System install to /usr | ⏳ |

---

## Hosting Setup

### Requirements

1. **Domain:** `install.muxi.ai` (or `get.muxi.ai`)
2. **HTTPS Certificate:** Let's Encrypt (required)
3. **HTTP Server:** Nginx or Cloudflare Pages

### Nginx Configuration

```nginx
server {
    listen 80;
    server_name install.muxi.ai get.muxi.ai;
    return 301 https://$server_name$request_uri;
}

server {
    listen 443 ssl http2;
    server_name install.muxi.ai get.muxi.ai;
    
    ssl_certificate /etc/letsencrypt/live/install.muxi.ai/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/install.muxi.ai/privkey.pem;
    
    location / {
        alias /var/www/muxi/;
        default_type text/plain;
        add_header Content-Type "text/x-shellscript; charset=utf-8";
        add_header X-Content-Type-Options "nosniff";
    }
}
```

### Cloudflare Pages

```bash
# Deploy to Cloudflare Pages
# Place install.sh at: pages/index.html (rename to .sh served as text/plain)

# Or use Workers:
export default {
  async fetch(request) {
    const script = await fetch(
      'https://raw.githubusercontent.com/muxi-ai/server/main/install.sh'
    );
    return new Response(script.body, {
      headers: {
        'Content-Type': 'text/x-shellscript; charset=utf-8',
        'X-Content-Type-Options': 'nosniff'
      }
    });
  }
}
```

### GitHub Pages (Simple Option)

```bash
# Create gh-pages branch
git checkout --orphan gh-pages
git rm -rf .
cp install.sh index.html  # Serve as index
git add index.html
git commit -m "Add install script"
git push origin gh-pages

# Enable GitHub Pages in repository settings
# Set custom domain: install.muxi.ai
```

---

## Future Enhancements

### Checksum Verification

```bash
# Generate checksums during release
sha256sum muxi-server-* > checksums.txt

# Verify in install script
curl -sSL "$CHECKSUM_URL" | grep "$BINARY_NAME" | sha256sum -c
```

### Signature Verification

```bash
# Sign releases with GPG
gpg --detach-sign -a muxi-server-linux-amd64

# Verify in install script
curl -sSL "$SIGNATURE_URL" > muxi-server.asc
gpg --verify muxi-server.asc muxi-server
```

### Uninstall Option

```bash
# Add uninstall command
curl -sSL https://install.muxi.ai | bash -s -- --uninstall
```

### Update Option

```bash
# Add update command
curl -sSL https://install.muxi.ai | bash -s -- --update
```

---

## Troubleshooting

### "Command not found" after install

**Cause:** PATH not updated or shell not reloaded

**Solution:**
```bash
# Reload shell config
source ~/.bashrc  # or ~/.zshrc

# Or add manually
export PATH="$HOME/.local/bin:$PATH"

# Or use absolute path
~/.local/bin/muxi-server --help
```

### "Permission denied" on Linux

**Cause:** Trying to use system paths without sudo

**Solution:**
```bash
# Use sudo for system install
curl -sSL https://install.muxi.ai | sudo bash

# Or use user install (no sudo)
curl -sSL https://install.muxi.ai | bash
```

### "No release available for your platform"

**Cause:** Running on unsupported platform or architecture

**Solution:**
```bash
# Check your platform
uname -s  # Should be Linux or Darwin
uname -m  # Should be x86_64 or arm64/aarch64

# Build from source if needed
git clone https://github.com/muxi-ai/server.git
cd server/src
go build -o muxi-server ./cmd/server
```

---

## Related Documents

- `PATH-DETECTION-STRATEGY.md` - Path detection implementation
- `INSTALLATION-ROADMAP.md` - Package manager plans (Q1 2025)
- `docs/installation.md` - User-facing installation guide

---

**Status:** Ready for Testing  
**Next Step:** Test on Ubuntu/RHEL, then publish to install.muxi.ai
