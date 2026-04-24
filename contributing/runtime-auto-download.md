# Runtime Auto-Download

## Overview

When deploying a formation, the MUXI Server needs access to:
1. **SIF file** - The Singularity container image containing the MUXI runtime
2. **runtime-runner** - Docker image that wraps Singularity (for macOS/Windows)

This document describes the auto-download flow for when these are missing.

## Flow

```
┌─────────────────────────────────────────────────────────────────┐
│ POST /rpc/formations (deploy)                                   │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│ 1. Parse formation.yaml                                         │
│    - Extract muxi_runtime version (e.g., ">=0.2025.0")          │
│    - Resolve to specific version (e.g., "0.2025.0")             │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│ 2. Check SIF file exists                                        │
│    Path: ~/.muxi/server/runtimes/muxi-runtime-{VERSION}-{ARCH}.sif │
│    Example: muxi-runtime-0.2025.0-linux-arm64.sif               │
└─────────────────────────────────────────────────────────────────┘
                              │
              ┌───────────────┴───────────────┐
              │ Exists?                       │
              ▼                               ▼
         ┌────────┐                    ┌─────────────┐
         │  Yes   │                    │     No      │
         └────────┘                    └─────────────┘
              │                               │
              │                               ▼
              │         ┌─────────────────────────────────────────┐
              │         │ 3. Download SIF from GitHub Releases    │
              │         │    URL: https://github.com/muxi-ai/     │
              │         │         runtime/releases/download/      │
              │         │         v{VERSION}/muxi-runtime-        │
              │         │         {VERSION}-linux-{ARCH}.sif      │
              │         └─────────────────────────────────────────┘
              │                               │
              └───────────────┬───────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│ 4. Check runtime-runner Docker image (macOS/Windows only)       │
│    Image: ghcr.io/muxi-ai/runtime-runner:latest                 │
└─────────────────────────────────────────────────────────────────┘
                              │
              ┌───────────────┴───────────────┐
              │ Exists?                       │
              ▼                               ▼
         ┌────────┐                    ┌─────────────┐
         │  Yes   │                    │     No      │
         └────────┘                    └─────────────┘
              │                               │
              │                               ▼
              │         ┌─────────────────────────────────────────┐
              │         │ 5. Docker pull runtime-runner           │
              │         │    docker pull ghcr.io/muxi-ai/         │
              │         │    runtime-runner:latest                │
              │         └─────────────────────────────────────────┘
              │                               │
              └───────────────┬───────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│ 6. Spawn formation with resolved paths                          │
└─────────────────────────────────────────────────────────────────┘
```

## URLs

### SIF Files (GitHub Releases)

**Production:**
```
https://pkg.muxi.org/runtime/v{VERSION}/muxi-runtime-{VERSION}-linux-{ARCH}.sif
```

**Examples:**
- `https://pkg.muxi.org/runtime/v0.2025.0/muxi-runtime-0.2025.0-linux-amd64.sif`
- `https://pkg.muxi.org/runtime/v0.2025.0/muxi-runtime-0.2025.0-linux-arm64.sif`

**Local Development (configurable):**
```
http://localhost:8080/muxi-runtime-{VERSION}-linux-{ARCH}.sif
```

### Runtime Runner (Docker)

**Production:**
```
ghcr.io/muxi-ai/runtime-runner:latest
ghcr.io/muxi-ai/runtime-runner:{VERSION}
```

## Architecture Detection

| Platform | GOARCH | SIF Arch |
|----------|--------|----------|
| Linux x86_64 | amd64 | linux-amd64 |
| Linux ARM64 | arm64 | linux-arm64 |
| macOS Intel | amd64 | linux-amd64 |
| macOS Apple Silicon | arm64 | linux-arm64 |
| Windows x64 | amd64 | linux-amd64 |
| Windows ARM64 | arm64 | linux-arm64 |

Note: SIF files are always Linux containers. On macOS/Windows, they run inside Docker.

## Configuration

```yaml
# ~/.muxi/server/config.yaml
runtime:
  # Override SIF download URL (for development/testing)
  sif_base_url: "http://localhost:8080"

  # Override runtime-runner image (for development/testing)
  runtime_runner_image: "ghcr.io/muxi-ai/runtime-runner:latest"

  # Auto-download behavior
  auto_download: true  # Set to false to require manual installation
```

## Error Handling

1. **Network failure during download:**
   - Retry 3 times with exponential backoff
   - Return clear error message with manual download instructions

2. **Invalid/corrupted SIF:**
   - Verify SHA256 checksum after download
   - Delete corrupted file and retry

3. **Docker pull failure:**
   - Check Docker daemon is running
   - Return clear error with `docker pull` command for manual retry

## Storage

```
~/.muxi/server/
├── runtimes/
│   ├── registry.json                              # Tracks downloaded versions
│   ├── muxi-runtime-0.2025.0-linux-amd64.sif     # Downloaded SIF
│   └── muxi-runtime-0.2025.0-linux-arm64.sif     # Downloaded SIF
└── ...
```

## Implementation Notes

### Phase 1: Basic Download
- [x] Detect missing SIF
- [ ] Download from configurable URL
- [ ] Save to runtimes directory
- [ ] Update registry.json

### Phase 2: Docker Pull
- [ ] Check if runtime-runner image exists
- [ ] Pull if missing
- [ ] Handle authentication for ghcr.io (if private)

### Phase 3: Verification
- [ ] SHA256 checksum verification
- [ ] SIF signature verification (future)

## Testing

**Local SIF server:**
```bash
# Serve SIF files from local directory
cd /path/to/sif/files
python3 -m http.server 8080
```

**Config for local testing:**
```yaml
runtime:
  sif_base_url: "http://localhost:8080"
```
