# Install Script - Production Checklist

**Status:** ✅ Script Created, 🔜 Needs Testing & Deployment

---

## Prerequisites (Before Script Works)

### 1. GitHub Releases with Binaries

**Required binaries:**
- `muxi-server-linux-amd64`
- `muxi-server-linux-arm64`
- `muxi-server-darwin-amd64`
- `muxi-server-darwin-arm64`

**How to create:**

```bash
# Build for all platforms
cd src/

# Linux AMD64
GOOS=linux GOARCH=amd64 go build -o ../dist/muxi-server-linux-amd64 ./cmd/server

# Linux ARM64
GOOS=linux GOARCH=arm64 go build -o ../dist/muxi-server-linux-arm64 ./cmd/server

# macOS AMD64 (Intel)
GOOS=darwin GOARCH=amd64 go build -o ../dist/muxi-server-darwin-amd64 ./cmd/server

# macOS ARM64 (Apple Silicon)
GOOS=darwin GOARCH=arm64 go build -o ../dist/muxi-server-darwin-arm64 ./cmd/server

# Create release
cd ..
gh release create v1.0.0 \
  dist/muxi-server-linux-amd64 \
  dist/muxi-server-linux-arm64 \
  dist/muxi-server-darwin-amd64 \
  dist/muxi-server-darwin-arm64 \
  --title "MUXI Server v1.0.0" \
  --notes "Initial release"
```

### 2. Domain Setup

**Required:**
- `install.muxi.ai` or `get.muxi.ai` pointing to script host
- HTTPS certificate (Let's Encrypt)

**Options:**

**Option A: GitHub Pages (Simplest)**
```bash
# Create gh-pages branch
git checkout --orphan gh-pages
cp install.sh index.html
git add index.html
git commit -m "Add install script"
git push origin gh-pages

# In GitHub repo settings → Pages:
# - Enable Pages from gh-pages branch
# - Set custom domain: install.muxi.ai
# - Add CNAME record: install.muxi.ai → muxi-ai.github.io
```

**Option B: Cloudflare Workers**
```javascript
export default {
  async fetch(request) {
    const script = await fetch(
      'https://raw.githubusercontent.com/muxi-ai/server/main/install.sh'
    );
    return new Response(script.body, {
      headers: {
        'Content-Type': 'text/x-shellscript; charset=utf-8'
      }
    });
  }
}
```

**Option C: Nginx Server**
```nginx
server {
    listen 443 ssl;
    server_name install.muxi.ai;
    
    location / {
        alias /var/www/muxi/install.sh;
        default_type text/x-shellscript;
    }
}
```

---

## Testing Checklist

### Local Testing (Before Publishing)

- [ ] **Syntax Check**
  ```bash
  bash -n install.sh
  ```

- [ ] **ShellCheck (if available)**
  ```bash
  shellcheck install.sh
  ```

- [ ] **Dry Run with Mock Binary**
  ```bash
  # Build local binary
  cd src && go build -o ../muxi-server ./cmd/server
  
  # Test locally (modify script to use local binary)
  ./install.sh
  ```

### Platform Testing (After Publishing)

- [ ] **Ubuntu 22.04 LTS (System Install)**
  ```bash
  # In Docker or VM
  curl -sSL https://install.muxi.ai | sudo bash
  sudo muxi-server init
  sudo muxi-server start
  curl http://localhost:7890/health
  ```

- [ ] **Ubuntu 22.04 LTS (User Install)**
  ```bash
  curl -sSL https://install.muxi.ai | bash
  muxi-server init
  muxi-server start
  curl http://localhost:7890/health
  ```

- [ ] **macOS (Intel)**
  ```bash
  curl -sSL https://install.muxi.ai | bash
  muxi-server init
  muxi-server start
  curl http://localhost:7890/health
  ```

- [ ] **macOS (Apple Silicon)**
  ```bash
  curl -sSL https://install.muxi.ai | bash
  muxi-server init
  muxi-server start
  curl http://localhost:7890/health
  ```

- [ ] **RHEL/CentOS 9**
  ```bash
  curl -sSL https://install.muxi.ai | sudo bash
  ```

- [ ] **Debian 12**
  ```bash
  curl -sSL https://install.muxi.ai | sudo bash
  ```

---

## Known Issues & Limitations

### Current Limitations

1. **No Checksum Verification**
   - Downloads are not verified with checksums
   - Mitigation: HTTPS-only downloads from GitHub

2. **No Binary Signature**
   - Binaries are not cryptographically signed
   - Future: GPG signing

3. **No Uninstall Command**
   - Manual uninstall required
   - Future: `curl ... | bash -s -- --uninstall`

4. **No Update Command**
   - Must reinstall to update
   - Future: `curl ... | bash -s -- --update`

### Platform-Specific Issues

**macOS:**
- Gatekeeper may block unsigned binaries
- Workaround: `xattr -d com.apple.quarantine muxi-server`
- Future: Code signing with Apple Developer cert

**Linux:**
- Requires `curl` (not always installed)
- Could add fallback to `wget`

---

## Immediate Next Steps

### 1. Create GitHub Release Workflow

Create `.github/workflows/release.yml`:

```yaml
name: Release

on:
  push:
    tags:
      - 'v*'

jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      
      - name: Set up Go
        uses: actions/setup-go@v4
        with:
          go-version: '1.21'
      
      - name: Build binaries
        run: |
          cd src
          GOOS=linux GOARCH=amd64 go build -o ../dist/muxi-server-linux-amd64 ./cmd/server
          GOOS=linux GOARCH=arm64 go build -o ../dist/muxi-server-linux-arm64 ./cmd/server
          GOOS=darwin GOARCH=amd64 go build -o ../dist/muxi-server-darwin-amd64 ./cmd/server
          GOOS=darwin GOARCH=arm64 go build -o ../dist/muxi-server-darwin-arm64 ./cmd/server
      
      - name: Create Release
        uses: softprops/action-gh-release@v1
        with:
          files: |
            dist/muxi-server-linux-amd64
            dist/muxi-server-linux-arm64
            dist/muxi-server-darwin-amd64
            dist/muxi-server-darwin-arm64
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```

### 2. Test Locally

```bash
# Build a test binary
cd src
go build -o ../muxi-server ./cmd/server
cd ..

# Modify install.sh temporarily to use local binary
# (comment out curl download, use local file)

# Test user install
./install.sh

# Verify
~/.local/bin/muxi-server version
```

### 3. Create First Release

```bash
# Tag the current version
git tag v1.0.0
git push origin v1.0.0

# GitHub Actions will build and create release automatically
```

### 4. Set Up Domain

```bash
# Option 1: GitHub Pages
# - Enable in repo settings
# - Add CNAME: install.muxi.ai

# Option 2: Cloudflare
# - Create Worker with script proxy
# - Add route: install.muxi.ai/*
```

### 5. Update Documentation

```bash
# Update docs/installation.md
# - Add curl | bash as PRIMARY method
# - Update all examples to use install.muxi.ai
# - Add troubleshooting section
```

---

## Success Criteria

### Installation Works When:

- ✅ User runs `curl -sSL https://install.muxi.ai | bash`
- ✅ Binary downloads successfully
- ✅ Binary is executable and works (`muxi-server version`)
- ✅ PATH is updated (user install)
- ✅ Directories are created
- ✅ `muxi-server init` works
- ✅ `muxi-server start` works
- ✅ Server responds on port 7890

### Installation Fails Gracefully When:

- ❌ No internet connection (shows helpful error)
- ❌ Unsupported platform (shows supported platforms)
- ❌ GitHub is down (shows alternative download link)
- ❌ curl not installed (shows how to install)

---

## Timeline

### Week 1: Testing & Release Infrastructure
- [ ] Create GitHub Actions workflow
- [ ] Test script locally on macOS
- [ ] Create first release (v1.0.0 or v0.9.0)
- [ ] Test download from GitHub releases

### Week 2: Domain Setup & Platform Testing
- [ ] Set up install.muxi.ai domain
- [ ] Test on Ubuntu (Docker)
- [ ] Test on macOS (local)
- [ ] Test on RHEL (VM if available)

### Week 3: Documentation & Launch
- [ ] Update all documentation
- [ ] Create announcement
- [ ] Add badges to README
- [ ] Publicize install method

---

## Current Status

**Completed:**
- ✅ Install script created (`install.sh`)
- ✅ Documentation written (`docs/INSTALL-SCRIPT.md`)
- ✅ Syntax validated
- ✅ Path detection implemented in server

**Next:**
- 🔜 Create GitHub Actions release workflow
- 🔜 Test locally with mock binary
- 🔜 Create first GitHub release
- 🔜 Set up install.muxi.ai domain
- 🔜 Full platform testing

---

**Ready for:** Local testing and release workflow creation
