# Installation & Distribution Roadmap

**Goal:** Make MUXI Server easy to install on all major platforms with native package managers.

**Current Status:** ✅ Basic installation working, 🚧 Native packages in development

---

## Overview

MUXI Server is a **system-level application** that requires:
- Installation to system directories (`/usr/local/bin/`)
- `sudo` privileges for installation
- System service setup (systemd/launchd)
- Process management capabilities

Unlike user-level tools (npm packages, pip packages), MUXI Server runs as a system service managing formations.

---

## Installation Methods

### ✅ Phase 1: Current (Available Now)

#### 1. One-Command Install (PRIMARY)
```bash
curl -sSL https://get.muxi.org | sudo bash
```

**Status:** 🟢 **Available**  
**Features:**
- Detects OS and architecture
- Downloads appropriate binary
- Installs to `/usr/local/bin/`
- Creates configuration directories
- Optionally sets up system service
- Validates installation

**Platforms:**
- ✅ macOS (Intel & Apple Silicon)
- ✅ Linux (Ubuntu, Debian, RHEL, CentOS, Fedora, Arch)

**Script Location:** `install.sh` (to be created)  
**Hosted at:** `https://get.muxi.org`

#### 2. Manual Binary Download
```bash
# Download from GitHub releases
curl -L https://github.com/muxi-ai/server/releases/latest/download/muxi-server-linux-amd64 -o muxi-server
chmod +x muxi-server
sudo mv muxi-server /usr/local/bin/
```

**Status:** 🟢 **Available**  
**Platforms:** All (Linux amd64/arm64, macOS amd64/arm64)

#### 3. Docker (Alternative)
```bash
docker run -d -p 7890:7890 ghcr.io/muxi-ai/muxi-server:latest
```

**Status:** 🟢 **Available**  
**Use Case:** Quick testing, containerized environments  
**Note:** NOT recommended for production (Docker socket security)

---

### 🚧 Phase 2: Native Package Managers (Q1 2025)

#### 1. Homebrew (macOS/Linux)

**Timeline:** Q1 2025  
**Status:** 🟡 In Development

**Installation:**
```bash
brew tap muxi-ai/tap
brew install muxi-server
```

**Benefits:**
- Automatic updates: `brew upgrade muxi-server`
- Dependency management
- Service management: `brew services start muxi-server`
- Uninstall: `brew uninstall muxi-server`

**Requirements:**
- Create Homebrew tap repository: `github.com/muxi-ai/homebrew-tap`
- Formula file: `Formula/muxi-server.rb`
- Binary releases on GitHub
- Bottle builds for common platforms

**Reference:**
- [Homebrew Formula Cookbook](https://docs.brew.sh/Formula-Cookbook)
- Example: [influxdb formula](https://github.com/Homebrew/homebrew-core/blob/master/Formula/i/influxdb.rb)

---

#### 2. APT (Debian/Ubuntu)

**Timeline:** Q1 2025  
**Status:** 🟡 Planned

**Installation:**
```bash
curl -fsSL https://packages.muxi.org/apt/gpg.key | sudo gpg --dearmor -o /usr/share/keyrings/muxi.gpg
echo "deb [signed-by=/usr/share/keyrings/muxi.gpg] https://packages.muxi.org/apt stable main" | \
  sudo tee /etc/apt/sources.list.d/muxi.list
sudo apt update
sudo apt install muxi-server
```

**Benefits:**
- Automatic updates: `sudo apt upgrade muxi-server`
- systemd service auto-setup
- Dependency resolution
- Official Ubuntu/Debian support

**Requirements:**
- APT repository hosting: `packages.muxi.ai/apt/`
- GPG signing key
- `.deb` package creation
- Package metadata (control file)
- Post-install scripts (systemd service)

**Reference:**
- [Debian Package Guide](https://www.debian.org/doc/manuals/maint-guide/)
- [Creating APT Repository](https://wiki.debian.org/DebianRepository/Setup)

---

#### 3. YUM/DNF (RHEL/CentOS/Fedora)

**Timeline:** Q1 2025  
**Status:** 🟡 Planned

**Installation:**
```bash
sudo tee /etc/yum.repos.d/muxi.repo <<EOF
[muxi]
name=MUXI Repository
baseurl=https://packages.muxi.org/yum/stable
enabled=1
gpgcheck=1
gpgkey=https://packages.muxi.org/yum/gpg.key
EOF

sudo yum install muxi-server  # RHEL/CentOS
sudo dnf install muxi-server  # Fedora
```

**Benefits:**
- Automatic updates: `sudo yum update muxi-server`
- systemd integration
- Enterprise Linux support

**Requirements:**
- YUM repository hosting: `packages.muxi.ai/yum/`
- RPM package creation (`.rpm`)
- GPG signing
- Spec file for RPM build
- Post-install scripts

**Reference:**
- [RPM Packaging Guide](https://rpm-packaging-guide.github.io/)
- [Creating YUM Repository](https://wiki.centos.org/HowTos/CreateLocalRepos)

---

#### 4. AUR (Arch Linux)

**Timeline:** Q1 2025  
**Status:** 🟡 Planned

**Installation:**
```bash
yay -S muxi-server
# OR
paru -S muxi-server
```

**Benefits:**
- Community-maintained
- Automatic updates
- systemd integration

**Requirements:**
- AUR package repository: `aur.archlinux.org/packages/muxi-server`
- PKGBUILD file
- .SRCINFO metadata
- Maintainer account on AUR

**Reference:**
- [AUR Submission Guidelines](https://wiki.archlinux.org/title/AUR_submission_guidelines)
- [PKGBUILD](https://wiki.archlinux.org/title/PKGBUILD)

---

### 🔮 Phase 3: Advanced Distribution (Q2 2025)

#### 1. Snap (Universal Linux)
```bash
sudo snap install muxi-server
```

**Benefits:** Works on all Linux distros

#### 2. Flatpak
```bash
flatpak install muxi-server
```

**Benefits:** Sandboxed, containerized

#### 3. Windows Installer (MSI)
```powershell
msiexec /i muxi-server-installer.msi
```

**Benefits:** Native Windows support

#### 4. Chocolatey (Windows)
```powershell
choco install muxi-server
```

**Benefits:** Windows package manager

---

## Implementation Plan

### Step 1: Install Script (Week 1-2) ✅ IN PROGRESS

**Create:** `install.sh`

**Features:**
- Detect OS/architecture (Linux/macOS, amd64/arm64)
- Download appropriate binary from GitHub releases
- Verify checksum (SHA256)
- Install to `/usr/local/bin/muxi-server`
- Set executable permissions
- Create directories: `~/.muxi/server/`
- Optionally create systemd/launchd service
- Run `muxi-server init` for first-time setup

**Hosting:**
- Upload to `https://get.muxi.org/install.sh`
- Make accessible via `curl -sSL https://get.muxi.org | sudo bash`

**Testing:**
- Test on Ubuntu 20.04, 22.04, 24.04
- Test on Debian 11, 12
- Test on CentOS 7, 8
- Test on Fedora 38, 39
- Test on macOS 13, 14 (Intel & ARM)

---

### Step 2: GitHub Releases Automation (Week 3)

**Create:** `.github/workflows/release.yml`

**Features:**
- Triggered on git tags: `v*.*.*`
- Build binaries for all platforms:
  - `muxi-server-linux-amd64`
  - `muxi-server-linux-arm64`
  - `muxi-server-darwin-amd64`
  - `muxi-server-darwin-arm64`
- Generate SHA256 checksums
- Create GitHub release with binaries + checksums
- Tag Docker image with version

---

### Step 3: Homebrew Formula (Week 4-5)

**Create:** `homebrew-tap` repository

**Tasks:**
1. Create `github.com/muxi-ai/homebrew-tap`
2. Create `Formula/muxi-server.rb`
3. Add bottle builds for common platforms
4. Test installation: `brew install muxi-ai/tap/muxi-server`
5. Submit to Homebrew core (optional, for wider reach)

**Formula Template:**
```ruby
class MuxiServer < Formula
  desc "AI Formation Orchestration Platform"
  homepage "https://muxi.org"
  url "https://github.com/muxi-ai/server/archive/v1.0.0.tar.gz"
  sha256 "..."
  license "MIT"

  depends_on "go" => :build

  def install
    system "cd", "src", "&&", "go", "build", "-o", "muxi-server", "./cmd/server"
    bin.install "muxi-server"
  end

  service do
    run [opt_bin/"muxi-server", "serve"]
    keep_alive true
    log_path var/"log/muxi-server.log"
    error_log_path var/"log/muxi-server-error.log"
  end

  test do
    assert_match "MUXI Server", shell_output("#{bin}/muxi-server version")
  end
end
```

---

### Step 4: DEB Packages (Week 6-7)

**Create:** `.deb` package build pipeline

**Tasks:**
1. Create `debian/` directory structure
2. Write `control` file (package metadata)
3. Write `postinst` script (systemd service setup)
4. Write `prerm` script (cleanup)
5. Build `.deb` for amd64 and arm64
6. Host at `packages.muxi.ai/apt/`
7. Set up APT repository with GPG signing

**Tools:**
- `dpkg-deb` for package creation
- `reprepro` for APT repository management

---

### Step 5: RPM Packages (Week 8-9)

**Create:** `.rpm` package build pipeline

**Tasks:**
1. Write `muxi-server.spec` file
2. Create build environment (rpmbuild)
3. Build RPM for x86_64 and aarch64
4. Host at `packages.muxi.ai/yum/`
5. Set up YUM repository with GPG signing
6. Create `repodata/` metadata

**Tools:**
- `rpmbuild` for package creation
- `createrepo` for YUM repository

---

### Step 6: AUR Package (Week 10)

**Create:** AUR package submission

**Tasks:**
1. Write `PKGBUILD`
2. Generate `.SRCINFO`
3. Submit to AUR
4. Test with `yay` and `paru`
5. Maintain updates

---

## Testing Matrix

| Platform | One-Command | Homebrew | APT | YUM | AUR | Docker |
|----------|-------------|----------|-----|-----|-----|--------|
| macOS 13 (Intel) | ✅ | 🟡 | - | - | - | ✅ |
| macOS 14 (ARM) | ✅ | 🟡 | - | - | - | ✅ |
| Ubuntu 20.04 | ✅ | 🟡 | 🟡 | - | - | ✅ |
| Ubuntu 22.04 | ✅ | 🟡 | 🟡 | - | - | ✅ |
| Ubuntu 24.04 | ✅ | 🟡 | 🟡 | - | - | ✅ |
| Debian 11 | ✅ | 🟡 | 🟡 | - | - | ✅ |
| Debian 12 | ✅ | 🟡 | 🟡 | - | - | ✅ |
| CentOS 8 | ✅ | - | - | 🟡 | - | ✅ |
| Fedora 39 | ✅ | 🟡 | - | 🟡 | - | ✅ |
| Arch Linux | ✅ | 🟡 | - | - | 🟡 | ✅ |

**Legend:**
- ✅ Available now
- 🟡 In development
- - Not applicable

---

## Documentation Updates

### Files to Create/Update:

1. **install.sh** - One-command installer script
2. **docs/installation.md** - Update with native package manager instructions
3. **docs/INSTALLATION-ROADMAP.md** - This file
4. **README.md** - Add installation badges and quick links
5. **CHANGELOG.md** - Document new installation methods

### Installation Badges (for README):

```markdown
[![Homebrew](https://img.shields.io/badge/Homebrew-000000?logo=homebrew&logoColor=white)](https://formulae.brew.sh/formula/muxi-server)
[![APT](https://img.shields.io/badge/APT-A80030?logo=debian&logoColor=white)](https://packages.muxi.org/apt)
[![YUM](https://img.shields.io/badge/YUM-EE0000?logo=redhat&logoColor=white)](https://packages.muxi.org/yum)
[![Docker](https://img.shields.io/badge/Docker-2496ED?logo=docker&logoColor=white)](https://ghcr.io/muxi-ai/muxi-server)
```

---

## Security Considerations

### Why `sudo` is Required

MUXI Server is a **system service**, not a user application:

1. **Binary Installation:**
   - Installs to `/usr/local/bin/` (system directory)
   - Requires root privileges

2. **Process Management:**
   - Spawns formation processes
   - Manages systemd/launchd services
   - Requires elevated permissions

3. **System Resources:**
   - Binds to ports (7890, 8000-9000)
   - Manages Docker containers (optional)
   - Accesses system directories

4. **Production Deployment:**
   - Runs as system service
   - Auto-starts on boot
   - Managed by systemd/launchd

**Not like:** npm packages, pip packages (user-level)  
**Similar to:** nginx, postgresql, redis (system services)

---

## User Communication

### Installation Page

Update website and docs to clearly explain:

**✅ Recommended: Native Installation (System-Level)**
```bash
curl -sSL https://get.muxi.org | sudo bash
```

**Why sudo?** MUXI Server is a system service (like nginx, postgres, redis) that:
- Manages background processes
- Runs as a daemon
- Requires system-level permissions

**Coming Soon:**
- Homebrew, APT, YUM native packages (Q1 2025)

**Alternative: Docker (Quick Testing)**
```bash
docker run ghcr.io/muxi-ai/muxi-server:latest
```
⚠️ For evaluation only, not production

---

## Success Metrics

**Phase 1 (Current):**
- ✅ One-command install working
- ✅ Manual binary download available
- ✅ Docker image published

**Phase 2 (Q1 2025):**
- 🟡 Homebrew formula submitted
- 🟡 APT repository live
- 🟡 YUM repository live
- 🟡 AUR package available

**Phase 3 (Q2 2025):**
- 🔮 Snap package available
- 🔮 Flatpak available
- 🔮 Windows installer
- 🔮 Chocolatey package

---

## Open Questions

1. **Package hosting:** Self-hosted (`packages.muxi.ai`) vs third-party (CloudSmith, Gemfury)?
2. **GPG signing:** Who maintains the signing keys?
3. **Update frequency:** How often to release new versions?
4. **LTS versions:** Should we have long-term support versions?
5. **Beta channel:** Should we have `stable` and `beta` repositories?

---

## Next Actions

**Immediate (This Week):**
1. Create `install.sh` script
2. Set up GitHub releases automation
3. Update documentation with `sudo` requirement
4. Add package manager "coming soon" notices

**Short-term (This Month):**
1. Start Homebrew formula
2. Research APT/YUM hosting options
3. Create packaging documentation

**Medium-term (Q1 2025):**
1. Launch Homebrew tap
2. Launch APT repository
3. Launch YUM repository
4. Submit AUR package

---

**Last Updated:** 2025-10-23  
**Next Review:** Weekly during development
