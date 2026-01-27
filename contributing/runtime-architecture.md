# Runtime Architecture

**How MUXI Server Executes Formations**

**🎉 Status:** Multi-Architecture Support COMPLETE (2025-12-04)

---

## Current Status

### What's Working Now

**End-to-end runtime orchestration with full multi-architecture support:**

✅ **Formation Deployment**
- Server accepts gzip bundle uploads
- Extracts and validates formation.yaml
- Reads runtime version constraint (e.g., "0.2025.0")

✅ **Runtime Resolution**
- Resolves version to SIF file path
- Architecture detection (arm64, amd64)
- SIF path: `~/.muxi/server/runtimes/muxi-runtime-0.2025.0-linux-{arch}.sif`

✅ **Container Spawning**
- **macOS/Windows:** Uses `runtime-runner` (Docker + Singularity) to execute SIF
- **Linux:** Native Singularity execution
- Platform-aware host binding (0.0.0.0 for Docker, 127.0.0.1 for Singularity)

✅ **Multi-Architecture Support**
- runtime-runner available for both amd64 and arm64
- SIF files built for both architectures
- Native performance on all platforms

**Supported Platforms:**
| Platform | Architecture | Runtime Method |
|----------|-------------|----------------|
| Linux | amd64 | Native Singularity |
| Linux | arm64 | Native Singularity |
| macOS | Intel (amd64) | runtime-runner + SIF |
| macOS | Apple Silicon (arm64) | runtime-runner + SIF |
| Windows | amd64 | runtime-runner + SIF |
| Windows | arm64 | runtime-runner + SIF |

---

## Overview

MUXI Server uses **Singularity SIF files** as the universal runtime format for executing formations. SIF (Singularity Image Format) provides:

- **Single-file distribution** - One portable file per runtime version
- **Strong isolation** - Container-level security and resource limits
- **Reproducibility** - Same behavior across all environments
- **Performance** - Minimal overhead on Linux production servers

---

## The Challenge: Linux-Only Containers

### What is Singularity?

Singularity is a **Linux container runtime** designed for scientific computing and HPC environments. Unlike Docker, it:

- Runs containers without a daemon
- Integrates with HPC schedulers
- Provides better security for multi-tenant systems
- Stores containers as single SIF files

**The catch:** Singularity **only runs on Linux** because it uses Linux-specific kernel features:

- **User namespaces** - Process isolation
- **Control groups (cgroups)** - Resource limiting
- **OverlayFS** - Filesystem layering
- **Seccomp** - System call filtering

These kernel features are not available on macOS or Windows.

---

## The Solution: Platform-Specific Execution

MUXI Server automatically detects the host platform and uses the appropriate execution method:

```
┌──────────────────────────────────────────────────────────┐
│ Platform Detection                                       │
├──────────────────────────────────────────────────────────┤
│                                                          │
│ Linux (Production)                                       │
│   ✓ Singularity available natively                      │
│   ✓ Direct SIF execution                                │
│   ✓ Zero overhead, maximum performance                  │
│   → singularity exec runtime.sif python app.py          │
│                                                          │
│ macOS / Windows (Development)                           │
│   ✗ Singularity not available                           │
│   ✓ Docker provides Linux kernel                        │
│   ✓ Runtime-runner image contains Singularity           │
│   → docker run runtime-runner exec runtime.sif ...      │
│                                                          │
└──────────────────────────────────────────────────────────┘
```

### Linux: Native Execution

On Linux, formations execute directly through Singularity:

```bash
# Server runs this command:
singularity exec \
  --env PORT=8001 \
  --env FORMATION_ID=my-formation \
  --bind /tmp \
  /path/to/muxi-runtime-0.1.0.sif \
  python app.py

# Performance:
# - Startup: ~50ms
# - Memory overhead: 0MB
# - CPU overhead: 0%
```

**This is production-optimized:** No Docker daemon, no container layers, just pure Linux container execution.

### macOS / Windows: Docker Wrapper

On macOS and Windows, the Linux kernel isn't available. To run Singularity:

1. **Docker Desktop** provides a Linux VM
2. **Runtime-runner image** contains Singularity + Linux
3. **SIF file is mounted** into the container
4. **Singularity runs inside** the Linux environment

```bash
# Server runs this command:
docker run \
  --rm \
  --privileged \
  -v /path/to/runtime.sif:/sif/runtime.sif \
  -v /formation:/formation \
  -p 8001:8001 \
  -e PORT=8001 \
  ghcr.io/muxi-ai/runtime-runner:latest \
  exec /sif/runtime.sif python app.py

# Performance:
# - Startup: ~200-500ms (acceptable for dev)
# - Memory overhead: ~100MB (Docker layer)
# - CPU overhead: Minimal (emulation if ARM Mac)
```

**This is dev-optimized:** Transparent to developers, same SIF files as production, just works.

---

## Formation Spawning Model

### One Container Per Formation

When you deploy a formation, MUXI Server spawns a **dedicated container** for it:

```
MUXI Server (Native Process)
  │
  ├── Formation 1 → docker run --name formation-1 -p 8001:8001 ...
  │                  (Runs independently)
  │
  ├── Formation 2 → docker run --name formation-2 -p 8002:8002 ...
  │                  (Runs independently)
  │
  └── Formation 3 → docker run --name formation-3 -p 8003:8003 ...
                     (Runs independently)
```

**Each formation container:**
- Has its own process space (isolated from other formations)
- Binds to a unique port (8001, 8002, 8003, etc.)
- Runs the same SIF file with different environment variables
- Exits when the formation stops
- Is automatically removed (`--rm` flag)

**Why not one container for all formations?**
- ✅ **Isolation** - One formation crash doesn't affect others
- ✅ **Independence** - Deploy/update formations individually
- ✅ **Resource limits** - Set CPU/memory per formation
- ✅ **Debugging** - Clear logs per formation
- ✅ **Scaling** - Easy to distribute formations across servers

---

## Container Lifecycle

### Deployment

```bash
$ muxi formation deploy my-formation.tar.gz

# Server:
# 1. Extracts formation bundle
# 2. Parses formation.yaml (finds runtime: "0.1.0")
# 3. Resolves runtime version (0.1.0 → exact SIF file)
# 4. Allocates port (e.g., 8001)
# 5. Spawns container:

docker run \
  --rm \
  --name formation-my-formation \
  -p 8001:8001 \
  -v /path/to/runtime.sif:/sif/runtime.sif \
  -v /formation:/formation \
  -e PORT=8001 \
  -e FORMATION_ID=my-formation \
  ghcr.io/muxi-ai/runtime-runner:latest \
  exec /sif/runtime.sif python app.py

# Formation starts in ~200-500ms
# Server registers it and begins health checks
```

### Runtime

```bash
$ curl http://localhost:7890/api/my-formation/health
{"status": "healthy"}

# Behind the scenes:
# - Request hits MUXI Server (port 7890)
# - Server proxies to formation (port 8001)
# - Formation container handles request
# - Response flows back through server
```

### Shutdown

```bash
$ muxi formation stop my-formation

# Server:
# 1. Sends SIGTERM to container
docker stop formation-my-formation

# 2. Container stops gracefully
# 3. Container removed automatically (--rm flag)
# 4. Port 8001 released for reuse
```

---

## Why This Architecture?

### Universal SIF Format

**One file format, works everywhere:**

```
Build SIF once:
  → muxi-runtime-0.1.0.sif (55MB)

Deploy anywhere:
  ✓ Linux production server (native Singularity)
  ✓ macOS developer laptop (Docker wrapper)
  ✓ Windows developer machine (Docker wrapper)
  ✓ HPC cluster (native Singularity)
  ✓ Cloud VM (native Singularity)
```

**True dev/prod parity:** What you test locally is exactly what runs in production.

### Platform Optimization

**Production (Linux):**
- Native Singularity execution
- Zero Docker overhead
- Optimal startup time (~50ms)
- Minimal resource usage
- HPC-grade isolation

**Development (macOS/Windows):**
- Transparent Docker wrapper
- Same SIF files as production
- Acceptable performance (~200-500ms startup)
- Familiar Docker Desktop
- No Linux VM setup needed

### Simplicity

**For users:**
```bash
muxi formation deploy my-formation.tar.gz
# Just works! No platform-specific commands needed.
```

**For server:**
```go
// Automatic platform detection
if runtime.GOOS == "linux" {
    // Use native Singularity
    singularity exec runtime.sif python app.py
} else {
    // Use Docker wrapper
    docker run runtime-runner exec runtime.sif python app.py
}
```

---

## Runtime Image: `runtime-runner`

### What is it?

The `runtime-runner` is a Docker image that provides Singularity on non-Linux platforms:

```dockerfile
FROM ubuntu:22.04

# Install Singularity
RUN apt-get update && \
    apt-get install -y singularity-container

# Configure for SIF execution
WORKDIR /formation
ENTRYPOINT ["singularity"]
```

**Purpose:** Bring Linux kernel features (via Docker) + Singularity to macOS/Windows

**Image location:** `ghcr.io/muxi-ai/runtime-runner:latest`

**Size:** ~120MB (cached after first pull)

### How It Works

```
Host (macOS/Windows)
  ↓
Docker Desktop (provides Linux VM)
  ↓
runtime-runner container (has Singularity)
  ↓
SIF file (mounted from host)
  ↓
Formation executes (inside SIF)
```

**Layers:**
1. **Host OS** - macOS or Windows
2. **Docker Desktop** - Linux VM
3. **runtime-runner** - Ubuntu + Singularity
4. **SIF file** - Formation runtime
5. **Formation code** - Your application

**Data flow:**
```
HTTP Request
  ↓
Host network (localhost:7890)
  ↓
MUXI Server (native)
  ↓
Docker port mapping (-p 8001:8001)
  ↓
runtime-runner container
  ↓
SIF file execution
  ↓
Formation handles request
```

---

## Requirements by Platform

### Linux

**Required:**
- Singularity/Apptainer (`apt install singularity-container`)
- Linux kernel 3.10+ with namespace support

**Optional:**
- Docker (not needed for formations, only if you want Docker-based tools)

**Installation:**
```bash
# Ubuntu/Debian
sudo apt-get update
sudo apt-get install -y singularity-container

# Verify
singularity --version
```

### macOS

**Required:**
- Docker Desktop (`brew install --cask docker`)

**Not needed:**
- Singularity (provided by runtime-runner image)

**Installation:**
```bash
# Install Docker Desktop
brew install --cask docker

# Start Docker Desktop from Applications
# Then verify:
docker info
```

### Windows

**Required:**
- Docker Desktop (download from docker.com)
- WSL 2 (Windows Subsystem for Linux)

**Not needed:**
- Singularity (provided by runtime-runner image)

**Installation:**
```powershell
# Install WSL 2
wsl --install

# Install Docker Desktop
# Download from https://docker.com/products/docker-desktop

# Verify
docker info
```

---

## Performance Characteristics

### Linux (Production)

| Metric | Value | Impact |
|--------|-------|--------|
| Formation startup | ~50ms | Excellent |
| Container overhead | 0MB | None |
| CPU overhead | 0% | None |
| Network latency | Native | Optimal |
| Disk I/O | Direct | Maximum |

**Ideal for:** Production servers, HPC, high-throughput workloads

### macOS (Development)

| Metric | Value | Impact |
|--------|-------|--------|
| Formation startup | ~200-500ms | Good for dev |
| Container overhead | ~100MB | Acceptable |
| CPU overhead | 5-10% | Minimal |
| Network latency | +1-2ms | Negligible |
| Disk I/O | Via VM | Acceptable |

**Ideal for:** Local development, testing, demonstrations

### Windows (Development)

| Metric | Value | Impact |
|--------|-------|--------|
| Formation startup | ~200-500ms | Good for dev |
| Container overhead | ~100MB | Acceptable |
| CPU overhead | 5-10% | Minimal |
| Network latency | +1-2ms | Negligible |
| Disk I/O | Via WSL | Acceptable |

**Ideal for:** Local development, testing

---

## Troubleshooting

### "Singularity not found" (Linux)

**Cause:** Singularity not installed

**Solution:**
```bash
# Ubuntu/Debian
sudo apt-get install singularity-container

# Or initialize server (auto-installs)
muxi-server init
```

### "Docker not found" (macOS/Windows)

**Cause:** Docker Desktop not installed or not running

**Solution:**
```bash
# macOS
brew install --cask docker
# Then start Docker Desktop from Applications

# Windows
# Download and install Docker Desktop from docker.com
# Start from Start Menu
```

### "Cannot connect to Docker daemon"

**Cause:** Docker Desktop not running

**Solution:**
- Start Docker Desktop from Applications (macOS)
- Start from Start Menu (Windows)
- Wait for Docker icon to show "running"

### "Platform does not match" warning

**Output:**
```
WARNING: The requested image's platform (linux/amd64) does not match
the detected host platform (linux/arm64/v8)
```

**Cause:** Running amd64 image on ARM Mac (Apple Silicon)

**Impact:** Cosmetic only - Docker handles emulation automatically

**Solution:** Ignore warning, or build ARM-specific images (future enhancement)

### Slow startup on first run

**Cause:** Pulling `runtime-runner` image for first time

**What happens:**
```bash
Pulling runtime runner image (first time only)...
[===================>] 120MB/120MB
✓ Runtime runner image pulled successfully
```

**Impact:** 1-2 minutes first run, instant thereafter

**Solution:** Wait for pull to complete (only happens once)

---

## Advanced Topics

### Custom Runtime Images

Want to customize the runtime? Build your own:

```dockerfile
FROM ghcr.io/muxi-ai/runtime-runner:latest

# Add custom tools
RUN apt-get update && \
    apt-get install -y your-package

# Or install Python packages in SIF
# (Better: build custom SIF file)
```

### Multiple Formations, One SIF

All formations using the same runtime version share the SIF file:

```
Runtime SIF: muxi-runtime-0.1.0.sif
  ↓
  Used by:
    - formation-1 (port 8001, different env vars)
    - formation-2 (port 8002, different env vars)
    - formation-3 (port 8003, different env vars)
```

This is efficient - SIF file loaded once, shared read-only.

### Privileged Mode (`--privileged`)

The Docker wrapper requires privileged mode for user namespaces:

```bash
docker run --privileged ...
```

**Why:** Singularity creates user namespaces for isolation, which requires elevated privileges inside Docker.

**Security:** Only affects dev machines (macOS/Windows). Linux production uses native Singularity without Docker.

---

## Future Enhancements

### Multi-Architecture SIF Support

Currently: One SIF file (linux/amd64) for all

**Future:**
- `muxi-runtime-0.1.0-amd64.sif` - Intel/AMD
- `muxi-runtime-0.1.0-arm64.sif` - ARM (better for Apple Silicon)

Server auto-selects based on platform.

### Docker Compose for Complex Formations

Currently: One container per formation

**Future:** Support formations with multiple services:
```yaml
# formation.afs (or .yaml)
runtime: "0.1.0"
services:
  app:
    command: python app.py
  worker:
    command: python worker.py
  db:
    image: postgres:15
```

Server generates docker-compose.yml and manages lifecycle.

### Alternative Runtimes

Currently: Singularity + Docker wrapper

**Future:**
- Podman (daemonless alternative)
- Native Python + venv (fallback)
- WebAssembly (when Python support matures)

---

## Building Runtime Components

### Building SIF Files

SIF files are built from the `muxi-runtime` Docker image:

```bash
# In the runtime repository
cd ../runtime

# Build Docker image for specific architecture
./build-runtime.sh --platform linux/arm64   # For ARM (Apple Silicon, ARM servers)
./build-runtime.sh --platform linux/amd64   # For Intel/AMD

# Convert to SIF
./build-sif.sh --arch arm64   # Creates muxi-runtime-X.X.X-linux-arm64.sif
./build-sif.sh --arch amd64   # Creates muxi-runtime-X.X.X-linux-amd64.sif

# Install to server runtimes directory
cp muxi-runtime-*.sif ~/.muxi/server/runtimes/
```

**SIF Naming Convention:**
```
muxi-runtime-{version}-linux-{arch}.sif

Examples:
- muxi-runtime-0.2025.0-linux-arm64.sif
- muxi-runtime-0.2025.0-linux-amd64.sif
```

### Building runtime-runner

The runtime-runner Docker image provides Singularity for macOS/Windows:

```bash
# In the runtime-runner repository
cd ../runtime-runner

# Build for local architecture
./build.sh

# Build and push multi-arch to registry
./build.sh v1.0.0 --push
```

**Image location:** `ghcr.io/muxi-ai/runtime-runner:latest`

### Manual Testing

Test the full runtime-runner + SIF flow:

```bash
# Test runtime-runner + SIF execution
docker run --rm --privileged \
  -v ~/.muxi/server/runtimes/muxi-runtime-0.2025.0-linux-arm64.sif:/sif/runtime.sif:ro \
  -v /path/to/formation:/formation:ro \
  -v /etc/localtime:/etc/localtime:ro \
  -p 8001:8001 \
  ghcr.io/muxi-ai/runtime-runner:latest \
  exec --bind /formation:/formation --bind /tmp \
  /sif/runtime.sif \
  python -m muxi.utils.run_formation /formation/formation.afs --port 8001 --host 0.0.0.0
```

---

## Summary

**MUXI Server uses native Linux containers (Singularity SIF files) for formation execution.**

**Why Singularity?**
- Single-file format (easy distribution)
- Production-optimized (HPC-grade performance)
- Strong isolation (security + resource limits)

**Why runtime-runner?**
- Singularity only works on Linux
- macOS/Windows lack Linux kernel features
- Docker Desktop provides Linux VM
- runtime-runner brings Singularity to non-Linux
- Multi-arch support (amd64 + arm64)

**How it works:**
- Linux: Native Singularity (fast, zero overhead)
- macOS/Windows: runtime-runner (Docker + Singularity + SIF)
- One container per formation (isolated, independent)
- Same SIF file everywhere (true dev/prod parity)

**The result:** Universal runtime that's production-optimized for Linux and dev-friendly for all platforms! 🚀

---

**Related Documentation:**
- [Installation Guide](installation.md) - Platform-specific setup
- [Formations Guide](formations.md) - Creating and deploying formations
- [Troubleshooting](troubleshooting.md) - Common issues and solutions
