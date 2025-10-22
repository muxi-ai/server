# MUXI Runtime SIF - File Naming Convention

**Status:** Approved  
**Date:** 2025-01-20

---

## 📦 SIF File Naming

### Standard Format

```
muxi-runtime-{version}-{os}-{arch}.sif
```

### Components

| Component | Description | Examples |
|-----------|-------------|----------|
| `muxi-runtime` | Fixed prefix | `muxi-runtime` |
| `{version}` | Semantic version (no 'v' prefix) | `1.0.0`, `1.2.3`, `2.0.0` |
| `{os}` | Operating system | `linux`, `darwin` |
| `{arch}` | CPU architecture | `amd64`, `arm64`, `arm` |
| `.sif` | File extension | `.sif` |

---

## 🌐 CDN Directory Structure

```
cdn.muxi.org/runtime/
├── 1.0.0/
│   ├── muxi-runtime-1.0.0-linux-amd64.sif
│   ├── muxi-runtime-1.0.0-linux-arm64.sif
│   ├── muxi-runtime-1.0.0-darwin-arm64.sif
│   └── checksums.txt
├── 1.2.3/
│   ├── muxi-runtime-1.2.3-linux-amd64.sif
│   ├── muxi-runtime-1.2.3-linux-arm64.sif
│   ├── muxi-runtime-1.2.3-darwin-arm64.sif
│   └── checksums.txt
├── 2.0.0/
│   └── ...
└── latest.txt                     # Contains latest version number
```

---

## 📋 Examples

### Production Files

```
muxi-runtime-1.0.0-linux-amd64.sif      # Linux x86_64
muxi-runtime-1.0.0-linux-arm64.sif      # Linux ARM64 (AWS Graviton, RPi 4+)
muxi-runtime-1.0.0-darwin-arm64.sif     # macOS Apple Silicon (M1/M2/M3)
muxi-runtime-2.0.0-linux-amd64.sif      # Version 2.0.0 for Linux x86_64
```

### Checksums File (`checksums.txt`)

```
# SHA256 checksums for MUXI Runtime v1.0.0
abc123...  muxi-runtime-1.0.0-linux-amd64.sif
def456...  muxi-runtime-1.0.0-linux-arm64.sif
789xyz...  muxi-runtime-1.0.0-darwin-arm64.sif
```

### Latest Version File (`latest.txt`)

```
2.0.0
```

---

## 🔗 Download URL Construction

### CDN URLs

```
Base: https://cdn.muxi.org/runtime/{version}/

Examples:
https://cdn.muxi.org/runtime/1.0.0/muxi-runtime-1.0.0-linux-amd64.sif
https://cdn.muxi.org/runtime/1.2.3/muxi-runtime-1.2.3-darwin-arm64.sif
https://cdn.muxi.org/runtime/1.0.0/checksums.txt
https://cdn.muxi.org/runtime/latest.txt
```

### GitHub Release URLs

```
Base: https://github.com/muxi-ai/runtime/releases/download/v{version}/

Examples:
https://github.com/muxi-ai/runtime/releases/download/v1.0.0/muxi-runtime-1.0.0-linux-amd64.sif
https://github.com/muxi-ai/runtime/releases/download/v1.2.3/muxi-runtime-1.2.3-darwin-arm64.sif
https://github.com/muxi-ai/runtime/releases/download/v1.0.0/checksums.txt
```

**Note:** GitHub uses `v{version}` in the tag (v1.0.0), but filename uses `{version}` (1.0.0)

---

## 🖥️ Platform Detection (Server-Side)

### Go Platform Detection

```go
package runtime

import (
    "fmt"
    "runtime"
)

// GetPlatform returns the current platform string for SIF downloads
// Returns: "linux-amd64", "darwin-arm64", etc.
func GetPlatform() string {
    return fmt.Sprintf("%s-%s", runtime.GOOS, runtime.GOARCH)
}

// GetFilename constructs the SIF filename for a given version
func GetFilename(version string) string {
    platform := GetPlatform()
    return fmt.Sprintf("muxi-runtime-%s-%s.sif", version, platform)
}

// GetDownloadURL constructs the CDN download URL
func GetDownloadURL(version, baseURL string) string {
    filename := GetFilename(version)
    return fmt.Sprintf("%s/%s/%s", baseURL, version, filename)
}

// GetGitHubURL constructs the GitHub release download URL
func GetGitHubURL(version, baseURL string) string {
    filename := GetFilename(version)
    return fmt.Sprintf("%s/v%s/%s", baseURL, version, filename)
}
```

### Platform Mapping

| Go GOOS | Go GOARCH | Platform String | Example Filename |
|---------|-----------|-----------------|------------------|
| `linux` | `amd64` | `linux-amd64` | `muxi-runtime-1.0.0-linux-amd64.sif` |
| `linux` | `arm64` | `linux-arm64` | `muxi-runtime-1.0.0-linux-arm64.sif` |
| `linux` | `arm` | `linux-arm` | `muxi-runtime-1.0.0-linux-arm.sif` |
| `darwin` | `amd64` | `darwin-amd64` | `muxi-runtime-1.0.0-darwin-amd64.sif` |
| `darwin` | `arm64` | `darwin-arm64` | `muxi-runtime-1.0.0-darwin-arm64.sif` |

---

## 📦 Supported Platforms (Initial Release)

### Priority 1 (MVP)
- ✅ `linux-amd64` (Most common: AWS, GCP, standard servers)
- ✅ `linux-arm64` (AWS Graviton, modern ARM servers)

### Priority 2 (Nice to Have)
- ⚠️ `darwin-arm64` (Development: Apple Silicon Macs)
- ⚠️ `darwin-amd64` (Development: Intel Macs)

### Future
- `linux-arm` (Raspberry Pi, IoT devices)
- `windows-amd64` (Windows Server - if Singularity support improves)

**Note:** For MVP, we'll build **linux-amd64** first, then expand to other platforms.

---

## 🛠️ Build Script Output

```bash
# After running: ./build.sh 1.0.0

Output files:
├── muxi-runtime-1.0.0-linux-amd64.sif
├── muxi-runtime-1.0.0-linux-arm64.sif
├── muxi-runtime-1.0.0-darwin-arm64.sif
└── checksums.txt

checksums.txt contains:
abc123...  muxi-runtime-1.0.0-linux-amd64.sif
def456...  muxi-runtime-1.0.0-linux-arm64.sif
789xyz...  muxi-runtime-1.0.0-darwin-arm64.sif
```

---

## 🔄 Release Process

### Step 1: Build
```bash
cd runtime/sif/
./build.sh 1.0.0
```

**Produces:**
- `muxi-runtime-1.0.0-{platform}.sif` (for each platform)
- `checksums.txt`

### Step 2: Release
```bash
./release.sh 1.0.0
```

**Does:**
1. Creates GitHub release `v1.0.0`
2. Uploads all `.sif` files + `checksums.txt`
3. Pushes to CDN at `cdn.muxi.org/runtime/1.0.0/`
4. Updates `latest.txt` → `1.0.0`

### Step 3: Server Download
```bash
muxi-server init
```

**Downloads:**
```
URL: https://cdn.muxi.org/runtime/1.0.0/muxi-runtime-1.0.0-linux-amd64.sif
Fallback: https://github.com/muxi-ai/runtime/releases/download/v1.0.0/muxi-runtime-1.0.0-linux-amd64.sif
```

---

## 🧪 Testing URLs

### Test URL Construction

```go
func TestGetFilename(t *testing.T) {
    tests := []struct {
        version  string
        platform string
        expected string
    }{
        {"1.0.0", "linux-amd64", "muxi-runtime-1.0.0-linux-amd64.sif"},
        {"1.2.3", "darwin-arm64", "muxi-runtime-1.2.3-darwin-arm64.sif"},
        {"2.0.0", "linux-arm64", "muxi-runtime-2.0.0-linux-arm64.sif"},
    }
    
    for _, tt := range tests {
        t.Run(tt.version, func(t *testing.T) {
            // Mock platform detection
            got := fmt.Sprintf("muxi-runtime-%s-%s.sif", tt.version, tt.platform)
            if got != tt.expected {
                t.Errorf("got %s, want %s", got, tt.expected)
            }
        })
    }
}
```

### Test Download URLs

```go
func TestGetDownloadURL(t *testing.T) {
    version := "1.0.0"
    cdnBase := "https://cdn.muxi.org/runtime"
    
    expected := "https://cdn.muxi.org/runtime/1.0.0/muxi-runtime-1.0.0-linux-amd64.sif"
    got := GetDownloadURL(version, cdnBase)
    
    // Assuming GetPlatform() returns "linux-amd64"
    if got != expected {
        t.Errorf("got %s, want %s", got, expected)
    }
}
```

---

## 🔐 Checksum Verification

### Process

1. **Download SIF file**
   ```
   muxi-runtime-1.0.0-linux-amd64.sif
   ```

2. **Download checksums**
   ```
   checksums.txt
   ```

3. **Parse checksums.txt**
   ```
   abc123...  muxi-runtime-1.0.0-linux-amd64.sif
   ```

4. **Compute SHA256 of downloaded file**
   ```go
   hash := sha256.Sum256(fileBytes)
   computed := hex.EncodeToString(hash[:])
   ```

5. **Compare**
   ```go
   if computed != expected {
       return fmt.Errorf("checksum mismatch")
   }
   ```

---

## 📏 Filename Length Limits

### Analysis

```
muxi-runtime-1.0.0-linux-amd64.sif     = 38 characters
muxi-runtime-10.20.30-linux-amd64.sif  = 41 characters (max realistic)
```

**File System Limits:**
- Linux (ext4): 255 characters ✅
- macOS (APFS): 255 characters ✅
- Windows (NTFS): 255 characters ✅

**Conclusion:** No issues with filename length.

---

## 🚨 Edge Cases

### Unsupported Platform

If server runs on unsupported platform (e.g., `windows-amd64` before SIF support):

```go
platform := GetPlatform() // "windows-amd64"
url := GetDownloadURL("1.0.0", "https://cdn.muxi.org/runtime")
// https://cdn.muxi.org/runtime/1.0.0/muxi-runtime-1.0.0-windows-amd64.sif

// Download attempt → 404 Not Found
// Error: Runtime not available for platform windows-amd64
```

**Solution:** Check platform compatibility before download.

```go
var supportedPlatforms = map[string]bool{
    "linux-amd64":  true,
    "linux-arm64":  true,
    "darwin-arm64": true,
}

func IsPlatformSupported(platform string) bool {
    return supportedPlatforms[platform]
}
```

### Missing Version

```go
// User requests version that doesn't exist
url := GetDownloadURL("99.99.99", "https://cdn.muxi.org/runtime")
// https://cdn.muxi.org/runtime/99.99.99/muxi-runtime-99.99.99-linux-amd64.sif

// Download attempt → 404 Not Found
// Error: Runtime version 99.99.99 not found
```

**Solution:** Validate version exists before download (query GitHub API or check `latest.txt`).

---

## ✅ Validation Rules

### Version Format
- ✅ Semantic versioning: `X.Y.Z`
- ✅ Examples: `1.0.0`, `1.2.3`, `10.20.30`
- ❌ Invalid: `v1.0.0`, `1.0`, `latest`

### Platform Format
- ✅ Format: `{os}-{arch}`
- ✅ Lowercase only
- ✅ Examples: `linux-amd64`, `darwin-arm64`
- ❌ Invalid: `Linux-AMD64`, `linux_amd64`, `linux`

### Filename Validation (Regex)

```regex
^muxi-runtime-\d+\.\d+\.\d+-[a-z]+-[a-z0-9]+\.sif$
```

**Examples:**
- ✅ `muxi-runtime-1.0.0-linux-amd64.sif`
- ✅ `muxi-runtime-10.20.30-darwin-arm64.sif`
- ❌ `muxi-runtime-v1.0.0-linux-amd64.sif` (no 'v' prefix)
- ❌ `muxi-runtime-1.0-linux-amd64.sif` (incomplete version)
- ❌ `runtime-1.0.0-linux-amd64.sif` (missing prefix)

---

## 🎯 Summary

### Naming Convention

```
muxi-runtime-{version}-{os}-{arch}.sif
```

### URL Patterns

**CDN:**
```
https://cdn.muxi.org/runtime/{version}/muxi-runtime-{version}-{platform}.sif
```

**GitHub:**
```
https://github.com/muxi-ai/runtime/releases/download/v{version}/muxi-runtime-{version}-{platform}.sif
```

### Platform Detection

```go
platform := fmt.Sprintf("%s-%s", runtime.GOOS, runtime.GOARCH)
// "linux-amd64"
```

### Supported Platforms (MVP)

- `linux-amd64` (Priority 1)
- `linux-arm64` (Priority 1)
- `darwin-arm64` (Priority 2)

---

**This naming convention is approved and will be used across:**
- Build scripts (`build.sh`)
- Release scripts (`release.sh`)
- Server download logic (`pkg/runtime/download.go`)
- CDN structure
- GitHub releases
