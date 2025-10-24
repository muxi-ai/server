# MUXI Server Deployment Strategy - Complete Picture

**Date:** 2025-10-23  
**Status:** Strategy Defined  
**Next:** Implementation

---

## Executive Summary

**What we built:** Cross-platform SIF runtime execution  
**How we distribute:** Native Go binary + Docker image for dev platforms  
**Why it works:** Simple, performant, works everywhere

---

## The Three Questions Answered

### 1. ✅ Install Script Needs Runtime Dependencies

**Created:** `docs/INSTALL-SCRIPT-NOTES.md`

**Strategy:**
```bash
# install.sh or muxi-server init
↓
Detect platform
↓
Linux:    Install Singularity (apt/yum)
macOS:    Install Docker Desktop (Homebrew)
Windows:  Install Docker Desktop (Chocolatey)
↓
Verify installation
↓
Server ready!
```

**Implementation:** Update `muxi-server init` to check/install runtimes

---

### 2. ✅ Docker Registry Changed to GHCR

**Updated files:**
- `build-runtime-runner.sh` → `ghcr.io/muxi-ai/runtime-runner`
- `spawn.go` → Uses GHCR image
- `validator.go` → Uses GHCR image
- `Dockerfile.runtime-runner` → Added source label

**Created:** `test/dummy-sif/PUBLISH-TO-GHCR.md`

**Why GHCR:**
- Same pattern as faissx ✅
- Free for public images ✅
- Integrated with GitHub ✅
- No Docker Hub rate limits ✅

**Publishing:**
```bash
# Manual
docker buildx build --platform linux/amd64 \
  -t ghcr.io/muxi-ai/runtime-runner:latest \
  --push .

# Automated (GitHub Actions)
# Builds on every release/push to main
```

---

### 3. ✅ No Docker Compose for Server

**Created:** `docs/DOCKER-COMPOSE-STRATEGY.md`

**Decision:** MUXI Server runs **natively**, not in Docker

**Why:**
- ✅ Simpler installation (single binary)
- ✅ No Docker-in-Docker complexity
- ✅ Works with Singularity (Linux)
- ✅ Direct process management
- ✅ Optimal performance

**Distribution:**
```
Binary releases:
  - muxi-server-linux-amd64
  - muxi-server-linux-arm64
  - muxi-server-darwin-amd64
  - muxi-server-darwin-arm64
  - muxi-server-windows-amd64.exe

Install methods:
  - curl | bash install script
  - Homebrew tap
  - APT/YUM repositories
  - Direct download from GitHub
```

**Run as service:**
- Linux: systemd
- macOS: launchd
- Windows: Windows Service

**Docker Compose:** Only for dev dependencies (postgres, redis), not the server itself

---

## Complete Architecture

```
┌─────────────────────────────────────────────────────────────┐
│ MUXI Server (Native Go Binary)                             │
│                                                             │
│ Install:                                                    │
│   curl -sSL https://get.muxi.org | bash                    │
│                                                             │
│ Runtime dependencies:                                       │
│   Linux:    Singularity (installed by init script)        │
│   macOS:    Docker Desktop (installed by init script)      │
│   Windows:  Docker Desktop (installed by init script)      │
│                                                             │
│ Distribution:                                               │
│   - GitHub releases (binaries)                             │
│   - Homebrew tap                                           │
│   - APT/YUM repos                                          │
│                                                             │
│ Run as:                                                     │
│   - systemd service (Linux)                                │
│   - launchd service (macOS)                                │
│   - Windows Service (Windows)                              │
└─────────────────────────────────────────────────────────────┘
                              ↓
                    Spawns formations via
                              ↓
┌──────────────────┬──────────────────────┬─────────────────┐
│ Linux            │ macOS                │ Windows         │
├──────────────────┼──────────────────────┼─────────────────┤
│ Native           │ Docker wrapper       │ Docker wrapper  │
│ Singularity      │                      │                 │
│                  │                      │                 │
│ singularity      │ docker run           │ docker run      │
│ exec             │  ghcr.io/muxi-ai/    │  ghcr.io/muxi-  │
│ runtime.sif      │  runtime-runner      │  ai/runtime-    │
│                  │  exec runtime.sif    │  runner         │
│                  │                      │  exec .sif      │
└──────────────────┴──────────────────────┴─────────────────┘
```

---

## Distribution Components

### 1. MUXI Server Binary
```
Source: Go code compiled to native binary
Targets:
  - linux/amd64
  - linux/arm64
  - darwin/amd64
  - darwin/arm64
  - windows/amd64

Distribution:
  - GitHub Releases
  - Homebrew: brew install muxi-ai/tap/muxi-server
  - APT: apt install muxi-server
  - YUM: yum install muxi-server
  - Scoop: scoop install muxi-server (Windows)
```

### 2. Runtime Runner Image
```
Source: Dockerfile.runtime-runner
Target: linux/amd64 (emulated on ARM)
Image: ghcr.io/muxi-ai/runtime-runner:latest

Purpose: Provides Singularity for macOS/Windows
Auto-pulled: Yes (by validator.go)

Distribution:
  - GitHub Container Registry (public)
  - Tagged: latest, 1.0.0, 1.0, 1
```

### 3. Runtime SIF Files
```
Source: Built from muxi-runtime source
Target: linux/amd64 (universal)
Files: muxi-runtime-{version}.sif

Purpose: Formation execution environment
Distribution:
  - GitHub Releases
  - CDN (future)
  - Registry tracks installed versions
```

---

## Installation Flow

### Linux Server
```bash
# 1. Install MUXI Server
curl -sSL https://get.muxi.org | bash
# Downloads binary to /usr/local/bin/muxi-server

# 2. Initialize (installs Singularity)
muxi-server init
# Detects Linux
# Runs: apt install singularity-container (or yum)
# Creates config: ~/.muxi-server/config.yaml

# 3. Setup service (optional)
muxi-server install-service
# Creates: /etc/systemd/system/muxi-server.service
# Enables: systemctl enable muxi-server

# 4. Start server
systemctl start muxi-server
# or: muxi-server serve

# 5. Deploy formation
muxi formation deploy my-formation.tar.gz
# Server uses native Singularity ✅
```

### macOS Developer
```bash
# 1. Install MUXI Server
brew install muxi-ai/tap/muxi-server
# or: curl -sSL https://get.muxi.org | bash

# 2. Initialize (checks Docker)
muxi-server init
# Detects macOS
# Checks if Docker installed
# If not: Guides installation (brew install --cask docker)
# If yes but not running: Prompts to start Docker Desktop

# 3. Start server
muxi-server serve
# Runs in foreground for dev

# 4. Deploy formation
muxi formation deploy my-formation.tar.gz
# Server pulls ghcr.io/muxi-ai/runtime-runner (first time)
# Uses Docker wrapper ✅
```

### Windows Developer
```powershell
# 1. Install MUXI Server
scoop install muxi-server
# or: choco install muxi-server
# or: Download from GitHub releases

# 2. Initialize (checks Docker)
muxi-server init
# Detects Windows
# Checks if Docker installed
# Guides installation if needed

# 3. Start server
muxi-server serve

# 4. Deploy formation
muxi formation deploy my-formation.tar.gz
# Server uses Docker wrapper ✅
```

---

## Files to Create (TODO)

### Installation
- [ ] `install.sh` - Universal install script (Linux/macOS)
- [ ] `install.ps1` - Windows install script
- [ ] Update `init.go` - Runtime validation & installation
- [ ] `systemd/muxi-server.service` - Linux service template
- [ ] `launchd/ai.muxi.server.plist` - macOS service template

### Documentation
- [x] `INSTALL-SCRIPT-NOTES.md` - Installation requirements ✅
- [x] `PUBLISH-TO-GHCR.md` - Docker registry guide ✅
- [x] `DOCKER-COMPOSE-STRATEGY.md` - Why no compose ✅
- [ ] Update `docs/installation.md` - User-facing install docs
- [ ] Create `docs/deployment.md` - Production deployment guide

### CI/CD
- [ ] `.github/workflows/build-runtime-runner.yml` - Auto-build Docker image
- [ ] `.github/workflows/release.yml` - Build & publish binaries
- [ ] Update release process documentation

---

## Next Actions (Priority Order)

### High Priority (Before Public Release)

1. **Implement Runtime Installation** (1 week)
   - Update `muxi-server init` command
   - Add platform detection
   - Add Singularity installation (Linux)
   - Add Docker check (macOS/Windows)
   - Test on all platforms

2. **Publish Runtime Runner** (1 day)
   - Setup GitHub Actions workflow
   - Build and push to GHCR
   - Make image public
   - Test auto-pull functionality

3. **Update Documentation** (2 days)
   - Installation guide with runtime requirements
   - Deployment guide for production
   - Troubleshooting section
   - Platform-specific notes

### Medium Priority (Post-Launch)

4. **Package Managers** (1-2 weeks)
   - Create Homebrew tap
   - Setup APT repository
   - Setup YUM repository
   - Windows Scoop/Chocolatey packages

5. **Service Templates** (3 days)
   - systemd service file
   - launchd plist
   - Windows service wrapper
   - Auto-install option in init

### Low Priority (Future)

6. **Enhanced Installation** (1 week)
   - GUI installer (macOS/Windows)
   - Unattended installation mode
   - Installation validation tests
   - Rollback support

---

## Testing Checklist

### Installation Testing
- [ ] Linux (Ubuntu 22.04)
- [ ] Linux (Debian 11)
- [ ] Linux (RHEL/CentOS 8)
- [ ] macOS (Intel)
- [ ] macOS (Apple Silicon)
- [ ] Windows 10
- [ ] Windows 11

### Runtime Testing
- [ ] Singularity on Linux (native)
- [ ] Docker on macOS (wrapper)
- [ ] Docker on Windows (wrapper)
- [ ] Formation deployment end-to-end
- [ ] Multiple formations simultaneously
- [ ] Formation restart after crash
- [ ] Health check functionality

### Service Testing
- [ ] systemd start/stop/restart
- [ ] systemd auto-start on boot
- [ ] launchd start/stop
- [ ] launchd auto-start on login
- [ ] Service logs working
- [ ] Service status reporting

---

## Success Metrics

✅ **Completed:**
- Cross-platform runtime execution
- Docker wrapper for dev platforms
- Platform detection in spawn.go
- GHCR migration
- Architecture decisions documented

🚧 **In Progress:**
- Installation script updates
- Runtime validation
- Documentation updates

⏳ **Planned:**
- Package manager distribution
- Service templates
- CI/CD automation

---

## Summary

**What we have:**
- ✅ Universal SIF format
- ✅ Platform-specific execution
- ✅ Docker wrapper for dev
- ✅ Native Singularity for prod
- ✅ Clear distribution strategy

**What we need:**
- 🚧 Install script with runtime deps
- 🚧 GHCR publishing setup
- 🚧 Updated documentation

**Deployment model:**
- Native binary (not in Docker) ✅
- systemd/launchd services ✅
- Platform-appropriate runtimes ✅
- Simple, maintainable, performant ✅

**Ready for:** Implementation of installation improvements! 🚀

---

**Bottom line:** We have a complete, coherent strategy that:
1. Works on all platforms
2. Performs optimally in production
3. Supports developers effectively
4. Distributes simply
5. Scales well

No Docker Compose needed. Just a great native orchestrator! ✨
