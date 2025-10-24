# Multiple Docker Images Strategy

**Question:** Can we have multiple Docker images in the same repo?

**Answer:** YES! We'll have **two Docker images** for different use cases.

---

## The Two Images

### 1. **runtime-runner** (Current)
**Purpose:** Runs Singularity SIF files on macOS/Windows

**Location:** `ghcr.io/muxi-ai/runtime-runner:latest`

**Use Case:**
- Developer on macOS/Windows runs MUXI Server **natively**
- Server spawns formations via this Docker wrapper
- Transparent to the developer

**When:**
```bash
# Server running natively on macOS
muxi-server serve

# Behind the scenes, spawns formations:
docker run ghcr.io/muxi-ai/runtime-runner exec runtime.sif python app.py
```

---

### 2. **muxi-server** (NEW)
**Purpose:** Run the entire MUXI Server in Docker

**Location:** `ghcr.io/muxi-ai/muxi-server:latest`

**Use Case:**
- User doesn't want to install anything (Go binary, Singularity, etc.)
- Quick testing/demo environments
- Users "afraid to install stuff on the machine"
- Docker Compose orchestration

**When:**
```bash
# User runs server in Docker
docker run -p 7890:7890 -v /var/run/docker.sock:/var/run/docker.sock \
  ghcr.io/muxi-ai/muxi-server:latest

# Or via docker-compose
docker-compose up
```

---

## Repository Structure

```
server/
├── Dockerfile                    # NEW: MUXI Server image
├── docker-compose.yml            # NEW: Quick start compose file
│
├── test/dummy-sif/
│   └── Dockerfile.runtime-runner # EXISTING: Runtime wrapper image
│
└── .github/workflows/
    ├── build-muxi-server.yml     # NEW: Build server image
    └── build-runtime-runner.yml  # NEW: Build runtime image
```

---

## GitHub Container Registry

Both images publish to GHCR under the same organization:

```
ghcr.io/muxi-ai/
├── muxi-server:latest           # Server image
│   ├── muxi-server:1.0.0
│   ├── muxi-server:1.0
│   └── muxi-server:1
│
└── runtime-runner:latest        # Runtime wrapper image
    ├── runtime-runner:1.0.0
    ├── runtime-runner:1.0
    └── runtime-runner:1
```

**Package visibility:** Both public (no authentication needed to pull)

---

## Image 1: runtime-runner (Existing)

**Already implemented!**

**Dockerfile:** `test/dummy-sif/Dockerfile.runtime-runner`

```dockerfile
FROM --platform=linux/amd64 ubuntu:22.04

# Install Singularity
RUN apt-get update && \
    apt-get install -y singularity-container

WORKDIR /formation
ENTRYPOINT ["singularity"]
CMD ["--help"]
```

**Usage:**
```bash
# Server spawns this automatically
docker run --rm --privileged \
  -v ./runtime.sif:/sif/runtime.sif \
  -p 8001:8001 \
  ghcr.io/muxi-ai/runtime-runner:latest \
  exec /sif/runtime.sif python app.py
```

**Size:** ~120MB

**Purpose:** Provides Singularity on macOS/Windows

---

## Image 2: muxi-server (NEW)

**Need to create!**

**Dockerfile:** `Dockerfile` (root of repo)

```dockerfile
FROM golang:1.21-alpine AS builder

# Build MUXI Server
WORKDIR /build
COPY src/ ./src/
COPY go.mod go.sum ./

RUN go build -o muxi-server ./src/cmd/server

# Runtime image
FROM alpine:latest

# Install Docker CLI (for spawning formations)
RUN apk add --no-cache docker-cli

# Copy server binary
COPY --from=builder /build/muxi-server /usr/local/bin/

# Create directories
RUN mkdir -p /root/.muxi/server

# Expose port
EXPOSE 7890

# Default command
ENTRYPOINT ["muxi-server"]
CMD ["serve"]
```

**Usage:**

#### Option A: Standalone (Docker-in-Docker)
```bash
docker run -d \
  --name muxi-server \
  -p 7890:7890 \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -v muxi-data:/root/.muxi/server \
  ghcr.io/muxi-ai/muxi-server:latest
```

#### Option B: Docker Compose
```yaml
# docker-compose.yml
version: '3.8'

services:
  muxi-server:
    image: ghcr.io/muxi-ai/muxi-server:latest
    ports:
      - "7890:7890"
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock  # Access host Docker
      - muxi-data:/root/.muxi/server                # Persistent data
    environment:
      MUXI_LOG_LEVEL: info
    restart: unless-stopped

volumes:
  muxi-data:
```

**Size:** ~50MB (Go binary is small!)

**Purpose:** Full server in Docker (for users who don't want native install)

---

## Docker-in-Docker Considerations

### The Challenge

When MUXI Server runs in Docker and needs to spawn formation containers:

```
Host OS
  ↓
  Docker (muxi-server container)
    ↓
    Needs to spawn: Docker (formation containers)
```

### The Solution: Docker Socket Mounting

Mount the host's Docker socket into the container:

```bash
-v /var/run/docker.sock:/var/run/docker.sock
```

**What this does:**
- Server container gets access to host Docker daemon
- Formations spawn as **sibling containers** (not nested)
- No Docker-in-Docker complexity

**Architecture:**
```
Host OS
├── Docker Daemon
    ├── muxi-server (container 1)
    ├── formation-1 (container 2) ← spawned by muxi-server
    └── formation-2 (container 3) ← spawned by muxi-server
```

Formations are siblings, not children!

### Security Considerations

**⚠️ Warning:** Mounting Docker socket gives container **full control** over host Docker.

**Mitigation:**
- Document this clearly
- Only for dev/test environments (not production)
- For production, use native server install (not Docker)

**When to use Docker server:**
- ✅ Local testing/demos
- ✅ CI/CD environments
- ✅ Kubernetes (with proper RBAC)
- ❌ Production servers (use native binary instead)

---

## User Decision Tree

### "Should I use the Docker image?"

```
Do you want to install MUXI Server?
│
├─ YES, I'll install it natively
│   │
│   ├─ Linux Server (Production)
│   │   Install: apt install muxi-server
│   │   Runtime: Native Singularity
│   │   Performance: ⭐⭐⭐⭐⭐ Optimal
│   │
│   └─ macOS/Windows (Development)
│       Install: brew install muxi-server
│       Runtime: Docker wrapper (automatic)
│       Performance: ⭐⭐⭐⭐ Good
│
└─ NO, I want Docker for everything
    │
    └─ Run server in Docker
        Install: docker run ghcr.io/muxi-ai/muxi-server
        Runtime: Docker-in-Docker (socket mount)
        Performance: ⭐⭐⭐ Acceptable
        Best for: Testing, demos, quick start
```

---

## GitHub Actions Workflows

### Build runtime-runner

```yaml
# .github/workflows/build-runtime-runner.yml
name: Build Runtime Runner

on:
  push:
    paths:
      - 'test/dummy-sif/Dockerfile.runtime-runner'
      - '.github/workflows/build-runtime-runner.yml'
  release:
    types: [created]
  workflow_dispatch:

jobs:
  build:
    runs-on: ubuntu-latest
    permissions:
      contents: read
      packages: write

    steps:
      - uses: actions/checkout@v4
      
      - name: Log in to GHCR
        uses: docker/login-action@v3
        with:
          registry: ghcr.io
          username: ${{ github.actor }}
          password: ${{ secrets.GITHUB_TOKEN }}

      - name: Build and push
        uses: docker/build-push-action@v5
        with:
          context: test/dummy-sif
          file: test/dummy-sif/Dockerfile.runtime-runner
          platforms: linux/amd64
          push: true
          tags: |
            ghcr.io/muxi-ai/runtime-runner:latest
            ghcr.io/muxi-ai/runtime-runner:1.0.0
```

### Build muxi-server

```yaml
# .github/workflows/build-muxi-server.yml
name: Build MUXI Server Image

on:
  push:
    branches: [main]
    paths:
      - 'Dockerfile'
      - 'src/**'
      - '.github/workflows/build-muxi-server.yml'
  release:
    types: [created]
  workflow_dispatch:

jobs:
  build:
    runs-on: ubuntu-latest
    permissions:
      contents: read
      packages: write

    steps:
      - uses: actions/checkout@v4
      
      - name: Log in to GHCR
        uses: docker/login-action@v3
        with:
          registry: ghcr.io
          username: ${{ github.actor }}
          password: ${{ secrets.GITHUB_TOKEN }}

      - name: Build and push
        uses: docker/build-push-action@v5
        with:
          context: .
          file: Dockerfile
          platforms: linux/amd64,linux/arm64
          push: true
          tags: |
            ghcr.io/muxi-ai/muxi-server:latest
            ghcr.io/muxi-ai/muxi-server:${{ github.sha }}
```

---

## Quick Start for Users

### Native Install (Recommended)

```bash
# Linux
curl -sSL https://get.muxi.org | bash

# macOS
brew install muxi-ai/tap/muxi-server

# Start server
muxi-server serve
```

### Docker Install (Alternative)

```bash
# Pull image
docker pull ghcr.io/muxi-ai/muxi-server:latest

# Run server
docker run -d \
  -p 7890:7890 \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -v ~/.muxi/server:/root/.muxi/server \
  ghcr.io/muxi-ai/muxi-server:latest

# Or use docker-compose
curl -O https://raw.githubusercontent.com/muxi-ai/server/main/docker-compose.yml
docker-compose up -d
```

### Kubernetes (Advanced)

```yaml
# deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: muxi-server
spec:
  replicas: 1
  template:
    spec:
      containers:
      - name: muxi-server
        image: ghcr.io/muxi-ai/muxi-server:latest
        ports:
        - containerPort: 7890
        volumeMounts:
        - name: docker-sock
          mountPath: /var/run/docker.sock
      volumes:
      - name: docker-sock
        hostPath:
          path: /var/run/docker.sock
```

---

## Documentation Updates Needed

### README.md

Add installation section:

```markdown
## Installation

### Option 1: Native Install (Recommended)

Best performance, production-ready.

#### Linux
\`\`\`bash
curl -sSL https://get.muxi.org | bash
\`\`\`

#### macOS
\`\`\`bash
brew install muxi-ai/tap/muxi-server
\`\`\`

### Option 2: Docker (Alternative)

Quick start, no installation needed.

\`\`\`bash
docker run -d -p 7890:7890 \
  -v /var/run/docker.sock:/var/run/docker.sock \
  ghcr.io/muxi-ai/muxi-server:latest
\`\`\`

Or use docker-compose:

\`\`\`bash
docker-compose up -d
\`\`\`
```

### docs/installation.md

Add Docker section explaining:
- When to use Docker vs native
- Docker socket security implications
- Kubernetes deployment

---

## Comparison

| Feature | Native Install | Docker Server |
|---------|---------------|---------------|
| **Installation** | Binary + Singularity/Docker | Just Docker |
| **Performance** | ⭐⭐⭐⭐⭐ Optimal | ⭐⭐⭐ Good |
| **Production** | ✅ Recommended | ⚠️ Not ideal |
| **Security** | ✅ Isolated | ⚠️ Socket access |
| **Simplicity** | Install once, works | Pull & run |
| **Updates** | Package manager | `docker pull` |
| **Best for** | Production, Development | Quick demos, Testing |

---

## Implementation Checklist

### Phase 1: Create Server Dockerfile
- [ ] Create `Dockerfile` at repo root
- [ ] Multi-stage build (Go builder + Alpine runtime)
- [ ] Install Docker CLI in runtime image
- [ ] Test local build
- [ ] Test with socket mount

### Phase 2: Docker Compose
- [ ] Create `docker-compose.yml`
- [ ] Add environment variables
- [ ] Add volume mounts
- [ ] Test full stack

### Phase 3: GitHub Actions
- [ ] Create workflow for server image
- [ ] Create workflow for runtime-runner image
- [ ] Test automatic builds
- [ ] Publish to GHCR (make public)

### Phase 4: Documentation
- [ ] Update README with Docker install option
- [ ] Update docs/installation.md
- [ ] Add docker-compose.yml to repo
- [ ] Document Docker socket security implications
- [ ] Add "Quick Start with Docker" guide

---

## FAQ

### Why two images?

**runtime-runner:** Used by native server on macOS/Windows (hidden from users)
**muxi-server:** Full server in Docker (visible alternative for users)

### Can I use just Docker?

Yes! `docker-compose up` and you're running.

But **native install is better** for production (performance, security).

### What about Windows?

Both work:
- Native: Install muxi-server.exe, needs Docker Desktop
- Docker: Install Docker Desktop, run server in container

### Security concerns?

Mounting Docker socket (`/var/run/docker.sock`) gives container full Docker access.

**Fine for:**
- Local dev/testing
- CI/CD
- Kubernetes with RBAC

**Not ideal for:**
- Production servers (use native install)
- Untrusted users

### Can I run without Docker socket?

No - server needs to spawn formation containers.

Alternative: Native install (no Docker socket needed on Linux with Singularity).

---

## Summary

**YES - Multiple Docker images in one repo:**

1. **runtime-runner** - Singularity wrapper (hidden infrastructure)
   - Already built ✅
   - Published to `ghcr.io/muxi-ai/runtime-runner`

2. **muxi-server** - Full server (user-facing option)
   - Need to create
   - Will publish to `ghcr.io/muxi-ai/muxi-server`

**Both live in the same repo, published to GHCR, serve different purposes.**

**Recommendation for users:**
- **Production:** Native install (best performance, security)
- **Quick start/testing:** Docker image (easiest, but socket mount required)

---

**Next:** Create Dockerfile and docker-compose.yml for muxi-server image!
