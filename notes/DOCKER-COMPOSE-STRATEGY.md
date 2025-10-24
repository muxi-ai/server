# Docker Compose Strategy for MUXI Server

**Question:** Are we going to distribute the server with a docker-compose file? How's that going to work?

**Answer:** MUXI Server is designed to run **natively** (as a Go binary), not in Docker. Here's why and what the alternatives are:

---

## Architecture Decision

### **MUXI Server = Native Binary** ✅

```
Host OS (Linux/macOS/Windows)
  ↓
  MUXI Server (Go binary) ← Runs directly on host
  ↓
  Spawns & manages formations:
    - Native Singularity (Linux)
    - Docker wrapper (macOS/Windows)
```

**Why not in Docker?**
1. **Docker-in-Docker complexity** - Running Docker inside Docker is complex
2. **Socket mounting security** - Mounting `/var/run/docker.sock` is a security risk
3. **Process management** - Server needs direct access to host processes
4. **Simplicity** - Native binary is simpler to install and run
5. **Performance** - No container overhead for the orchestrator itself

---

## Deployment Models

### Model 1: Native Server (Recommended) ✅

```bash
# Install server binary
curl -sSL https://get.muxi.ai | bash

# Run as service
systemctl start muxi-server  # Linux
launchctl start muxi-server  # macOS

# Or run directly
muxi-server serve
```

**How formations are deployed:**
- Server spawns formations via Singularity (Linux) or Docker (macOS/Windows)
- Each formation is a separate process/container
- Server manages lifecycle, health checks, restarts

**Docker Compose role:** NONE (server runs natively)

---

### Model 2: Everything in Docker Compose (NOT Recommended) ❌

This would require:

```yaml
# docker-compose.yml (PROBLEMATIC!)
version: '3.8'

services:
  muxi-server:
    image: muxi-server:latest
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock  # ← Security risk!
    privileged: true  # ← Bad practice
    ports:
      - "7890:7890"
```

**Problems:**
- Docker-in-Docker (DinD) complexity
- Security issues (privileged container + socket mount)
- Nested container networking complexity
- Formation containers as siblings (harder to manage)
- Breaks on non-Docker systems (Singularity on Linux)

**Verdict:** ❌ Don't do this

---

### Model 3: Hybrid (Possible but Complex) ⚠️

Server runs natively, formations coordinated via Docker Compose:

```yaml
# formations-compose.yml
version: '3.8'

services:
  formation-1:
    image: muxi-formation-runtime:0.1.0
    command: python -m muxi_runtime formation-1
    ports:
      - "8001:8001"
    environment:
      FORMATION_ID: formation-1
      MUXI_SERVER_URL: http://host.docker.internal:7890

  formation-2:
    image: muxi-formation-runtime:0.1.0
    command: python -m muxi_runtime formation-2
    ports:
      - "8002:8002"
    environment:
      FORMATION_ID: formation-2
      MUXI_SERVER_URL: http://host.docker.internal:7890
```

**How it works:**
- Server runs natively on host
- Server generates docker-compose.yml per deployment
- Uses `docker-compose up` instead of `docker run`
- Formations run as Docker services

**Trade-offs:**
- ✅ Better for multi-container formations
- ✅ Can use Docker Compose features (networks, volumes, depends_on)
- ❌ More complex than simple `docker run`
- ❌ Requires docker-compose CLI installed
- ❌ Doesn't work with Singularity (Linux)

**Verdict:** ⚠️ Possible future enhancement, but adds complexity

---

## Recommended Approach

### **Native Server + Simple Container Execution** ✅

```
MUXI Server (native binary)
  ↓
  Uses platform-appropriate runtime:
    - Linux: singularity exec runtime.sif
    - macOS/Windows: docker run runtime-runner
```

**Benefits:**
- ✅ Simple installation (single binary)
- ✅ Works on all platforms (Linux, macOS, Windows)
- ✅ No Docker-in-Docker complexity
- ✅ Direct process management
- ✅ Optimal performance

**Distribution:**
- Binary: GitHub releases (muxi-server-linux-amd64, muxi-server-darwin-arm64, etc.)
- Install script: `curl -sSL https://get.muxi.ai | bash`
- Homebrew: `brew install muxi/tap/muxi-server`
- APT/YUM repos: `apt install muxi-server`

**No Docker Compose needed for the server itself!**

---

## When Docker Compose COULD Be Useful

### Use Case 1: Development Environment

```yaml
# dev-compose.yml (for developing the server itself)
version: '3.8'

services:
  # Test databases
  postgres:
    image: postgres:15
    environment:
      POSTGRES_DB: muxi_test
      POSTGRES_PASSWORD: test

  # Redis for caching (future)
  redis:
    image: redis:7

  # Not the server itself - run that natively!
  # docker-compose up -d postgres redis
  # go run ./src/cmd/server serve
```

**Use:** Only for dev dependencies, not the server

---

### Use Case 2: Multi-Container Formations (Future)

If a formation needs multiple services:

```yaml
# my-formation/docker-compose.yml
version: '3.8'

services:
  app:
    image: muxi-formation-runtime:0.1.0
    command: python app.py
    depends_on:
      - worker
      - db

  worker:
    image: muxi-formation-runtime:0.1.0
    command: python worker.py

  db:
    image: postgres:15
```

Server could support deploying these, but it adds complexity.

---

### Use Case 3: Full Stack Demo

```yaml
# demo-compose.yml (for demos/workshops)
version: '3.8'

services:
  # Note: This is NOT how production works!
  # This is just for quick demos/testing

  muxi-server:
    image: muxi-server:dev
    command: serve
    ports:
      - "7890:7890"
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock
    privileged: true
    environment:
      MUXI_ENV: demo
```

**Use:** Demo/workshop only, not production

---

## What About Kubernetes?

For cloud/production at scale:

```yaml
# muxi-server-deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: muxi-server
spec:
  replicas: 3
  template:
    spec:
      containers:
      - name: muxi-server
        image: muxi-server:latest
        ports:
        - containerPort: 7890
---
# formation-deployment.yaml (per formation)
apiVersion: apps/v1
kind: Deployment
metadata:
  name: formation-my-agent
spec:
  template:
    spec:
      containers:
      - name: runtime
        image: muxi-formation-runtime:0.1.0
        command: ["python", "app.py"]
```

**This is Phase 5+ territory** - multi-server orchestration

---

## Decision Matrix

| Deployment Type | Server Location | Formation Execution | Complexity | Recommended |
|----------------|-----------------|-------------------|------------|-------------|
| **Native** | Host (binary) | Singularity/Docker | Low | ✅ **YES** |
| Docker Compose | Docker container | Docker (DinD) | High | ❌ NO |
| Kubernetes | K8s pod | K8s pods | Very High | ⚠️ Future |

---

## FAQ

### Q: Can I run the server in Docker?
**A:** Technically yes, but not recommended. You'd need:
- Privileged mode
- Docker socket mounted
- Complex networking
- Breaks Singularity support

Better: Run server natively, let it manage containers

### Q: Can formations use Docker Compose?
**A:** Possible future enhancement, but current design is:
- One SIF file per formation
- Executed directly (singularity exec or docker run)
- Simpler, more reliable

### Q: What about systemd/launchd?
**A:** YES! That's the right tool for running the server:

**Linux (systemd):**
```ini
# /etc/systemd/system/muxi-server.service
[Unit]
Description=MUXI Server
After=network.target

[Service]
Type=simple
User=muxi
ExecStart=/usr/local/bin/muxi-server serve
Restart=always

[Install]
WantedBy=multi-user.target
```

**macOS (launchd):**
```xml
<!-- ~/Library/LaunchAgents/ai.muxi.server.plist -->
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" ...>
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>ai.muxi.server</string>
    <key>ProgramArguments</key>
    <array>
        <string>/usr/local/bin/muxi-server</string>
        <string>serve</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
</dict>
</plist>
```

---

## Conclusion

**Distribution Strategy:**

1. **MUXI Server:** Native binary (Go)
   - Install via script, Homebrew, APT, YUM
   - Run as systemd/launchd service
   - No Docker Compose

2. **Runtime Image:** Docker image (for macOS/Windows)
   - Publish to GHCR: `ghcr.io/muxi-ai/runtime-runner`
   - Auto-pulled when needed
   - Only used for dev platforms

3. **Formations:** SIF files
   - Distributed via GitHub releases, CDN
   - Registered in server runtime registry
   - Executed via Singularity or Docker wrapper

**Docker Compose role:** ❌ Not used for server distribution

**The MUXI way:**
```bash
# Install (one time)
curl -sSL https://get.muxi.ai | bash

# Run
muxi-server serve  # Or as systemd service

# Deploy formations
muxi formation deploy my-formation.tar.gz

# Just works! ✨
```

---

## Files to Create

- [ ] `install.sh` - Native binary installation
- [ ] `systemd/muxi-server.service` - Linux service
- [ ] `launchd/ai.muxi.server.plist` - macOS service
- [ ] Installation docs - Updated with service setup
- [ ] Maybe: `docker-compose.demo.yml` - For demos/workshops only

**NOT creating:** `docker-compose.yml` for production server deployment

---

**Summary:** MUXI Server is a native orchestrator, not a containerized app. This is the right design! ✅
