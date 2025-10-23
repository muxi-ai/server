# MUXI Server - Product Requirements Document

**Version:** 1.0  
**Status:** Planning  
**Target Release:** Q4 2025  

---

## Executive Summary

MUXI Server is a production-grade orchestration platform for deploying and managing MUXI formations at scale. Built on pm2-go, it combines robust process management with intelligent HTTP routing, enabling organizations to run multiple AI formations as a unified service.

**Key Value Props:**
- 🚀 **One-Command Deploy**: `muxi formation deploy` → production-ready API
- 🎯 **Zero Configuration**: Formations run as-is, server handles orchestration
- 📦 **Single Binary**: No dependencies, just install and run
- 🔄 **Auto-Recovery**: Crashed formations restart automatically
- 🌐 **Smart Routing**: HTTP proxy with formation-level isolation
- 📊 **Built-in Telemetry**: Usage tracking without touching formation code

**Install Experience:**

```bash
curl -sSL https://muxi.org/install | bash
# Installs: MUXI Server + Muxi Runtime + uv + Python 3.13+
# You're done!

muxi-server serve              # Start server
muxi formation deploy app.yaml # Deploy formation
curl https://server.com/app/chat  # It just works
```

---

## Problem Statement

### Current State: Manual Deployment Headaches

**For Solo Developers:**
- Formations run as standalone processes (manual pm2/systemd management)
- No unified API endpoint (each formation on different port)
- Crash recovery requires manual intervention
- Zero visibility into multi-formation deployments
- Tedious to manage multiple formations

**For Small Teams:**
- Each formation is a separate deployment
- No central management interface
- Manual proxy configuration (nginx/caddy setup required)
- Difficult to scale formations horizontally
- Logs scattered across processes

**For Organizations:**
- Complex deployment pipelines required
- Manual orchestration (k8s overkill for simple use cases)
- No standardized formation lifecycle
- Expensive infrastructure (cloud services, load balancers)
- Operational overhead (DevOps team needed)

### The Gap

**Docker/K8s**: Too complex for most users, requires containerization expertise  
**PM2**: Great for Node.js, doesn't understand formations  
**Systemd**: Low-level, no HTTP routing, manual setup  
**Cloud Platforms**: Expensive, vendor lock-in, overkill for simple deployments

**Users need:** Simple, batteries-included orchestration that "just works"

---

## Solution Overview

### MUXI Server: The Missing Orchestration Layer

Think **"Docker + PM2 + Nginx"** in a single binary, purpose-built for MUXI formations.

```
┌──────────────────────────────────────────────────────┐
│ One Install Command                                  │
│  curl https://muxi.org/install | bash                │
│                                                      │
│  Installs:                                           │
│    • MUXI Server (single binary)                     │
│    • uv (Python package manager)                     │
│    • Python 3.13+ (if needed)                        │
└──────────────────────────────────────────────────────┘

┌──────────────────────────────────────────────────────┐
│ Deploy Formation                                     │
│  muxi formation deploy formation.yaml                │
│                                                      │
│  Server:                                             │
│    1. Injects metadata (_server_id, _deployment_mode)│
│    2. Allocates port (8001)                          │
│    3. Spawns process (python run.py formation.yaml)  │
│    4. Monitors health                                │
└──────────────────────────────────────────────────────┘

┌──────────────────────────────────────────────────────┐
│ Users Hit Formation                                  │
│  POST https://server.com/my-formation/chat           │
│                                                      │
│  Server proxies → localhost:8001/chat                │
│  Formation processes → returns response              │
└──────────────────────────────────────────────────────┘
```

### Key Design Principles

1. **Formations are Unchanged**: Deploy existing YAMLs without modification
2. **Server is Dumb**: No formation logic, just orchestration + proxy
3. **Single Binary**: One executable, no dependencies
4. **Language Agnostic**: Server in Go, formations in Python (decoupled)
5. **Production Ready**: Auto-restart, log rotation, health checks built-in

---

## Architecture

### High-Level Overview

```
┌────────────────────────────────────────────────┐
│ Internet                                       │
│  ↓                                             │
│ HTTPS (Caddy/Nginx - optional)                 │
│  ↓                                             │
│ ╔════════════════════════════════════════════╗ │
│ ║ MUXI Server (Port 3000)                    ║ │
│ ║                                            ║ │
│ ║  ┌──────────────────────────────────────┐  ║ │
│ ║  │ Management API (for CLI)             │  ║ │
│ ║  │  POST /formations/deploy             │  ║ │
│ ║  │  GET  /formations                    │  ║ │
│ ║  │  DELETE /formations/{id}             │  ║ │
│ ║  └──────────────────────────────────────┘  ║ │
│ ║                                            ║ │
│ ║  ┌──────────────────────────────────────┐  ║ │
│ ║  │ HTTP Proxy (for users)               │  ║ │
│ ║  │  /{formation_id}/* → localhost:PORT  │  ║ │
│ ║  └──────────────────────────────────────┘  ║ │
│ ║                                            ║ │
│ ║  ┌──────────────────────────────────────┐  ║ │
│ ║  │ Process Manager (from pm2-go)        │  ║ │
│ ║  │  • Start/stop formations             │  ║ │
│ ║  │  • Monitor health                    │  ║ │
│ ║  │  • Auto-restart on crash             │  ║ │
│ ║  │  • Log rotation                      │  ║ │
│ ║  └──────────────────────────────────────┘  ║ │
│ ║                                            ║ │
│ ║  ┌──────────────────────────────────────┐  ║ │
│ ║  │ Formation Registry                   │  ║ │
│ ║  │  formation-abc → Port 8001           │  ║ │
│ ║  │  formation-def → Port 8002           │  ║ │
│ ║  └──────────────────────────────────────┘  ║ │
│ ╚════════════════════════════════════════════╝ │
│              ↓                    ↓            │
│  ┌──────────────────┐  ┌──────────────────┐    │
│  │ Formation Runtime│  │ Formation Runtime│    │
│  │ (Port 8001)      │  │ (Port 8002)      │    │
│  │                  │  │                  │    │
│  │ FastAPI Server   │  │ FastAPI Server   │    │
│  │ /chat            │  │ /chat            │    │
│  │ /workflow        │  │ /status          │    │
│  │ /health          │  │ /health          │    │
│  │                  │  │                  │    │
│  │ _server_id: abc  │  │ _server_id: abc  │    │
│  │ Sends telemetry  │  │ Sends telemetry  │    │
│  └──────────────────┘  └──────────────────┘    │
└────────────────────────────────────────────────┘
```

### Component Breakdown

**1. MUXI Server (Go Binary)**
- **Process Manager**: Fork of pm2-go for process lifecycle
- **HTTP Proxy**: Reverse proxy routing to formations
- **Formation Registry**: Tracks formations → ports → processes
- **Management API**: REST endpoints for CLI
- **Port Allocator**: Assigns unique ports to formations

**2. MUXI CLI (Python)**
- **Deploy Command**: `muxi formation deploy` sends YAML to server
- **Status Command**: `muxi formation status` queries server
- **Logs Command**: `muxi formation logs` streams from server

**3. Formation Runtime (Python)**
- **FastAPI Server**: HTTP endpoints for chat/workflow/etc
- **Formation Instance**: Loads YAML (with injected metadata)
- **Telemetry Client**: Reports usage to telemetry endpoint

---

## Core Features

### 1. Formation Deployment

**User Experience:**
```bash
# Deploy formation
$ muxi formation deploy formation.yaml

Deploying formation...
✓ Formation validated
✓ Metadata injected
✓ Port allocated: 8001
✓ Process started: formation-a3d7
✓ Health check passed

Formation URL: https://server.com/formation-a3d7
Status: https://server.com/formation-a3d7/health

# Alternative: Deploy with custom ID
$ muxi formation deploy formation.yaml --id my-api

Formation URL: https://server.com/my-api
```

**What Happens:**
1. CLI reads `formation.yaml`
2. Sends to server: `POST /formations/deploy`
3. Server validates YAML schema
4. Server injects metadata:
   ```yaml
   _server_id: "server-abc-123"
   _deployment_mode: "server"
   ```
5. Server saves: `formations/formation-a3d7.yaml`
6. Server allocates port: `8001`
7. Server spawns: `python run.py formations/formation-a3d7.yaml --port 8001`
8. Server registers: `formation-a3d7 → 8001`
9. Server health checks: `GET localhost:8001/health`
10. CLI returns formation URL

---

### 2. HTTP Proxy Routing

**User Experience:**
```bash
# Chat with formation
curl -X POST https://server.com/my-api/chat \
  -H "Content-Type: application/json" \
  -d '{"message": "Hello!", "user_id": "user-123"}'

# Response (proxied from localhost:8001/chat)
{"response": "Hello! How can I help you?"}
```

**Routing Logic:**
```
Request:  POST /my-api/chat
          ↓
Server:   Lookup "my-api" → Port 8001
          ↓
Server:   Rewrite path: /my-api/chat → /chat
          ↓
Server:   Proxy: POST localhost:8001/chat
          ↓
Formation: Process request
          ↓
Server:   Forward response to client
```

**Path Rewriting:**
- `/formation-abc/chat` → `localhost:8001/chat`
- `/formation-abc/workflow` → `localhost:8001/workflow`
- `/formation-abc/custom/endpoint` → `localhost:8001/custom/endpoint`

---

### 3. Process Management

**Built on pm2-go:**
- **Auto-restart on crash**: Formation crashes? Restarted in seconds
- **Log rotation**: Automatic log management (10MB max, 10 files)
- **Graceful shutdown**: SIGTERM → cleanup → exit
- **Process persistence**: Survives server restarts

**User Commands:**
```bash
# List all formations
muxi formation list
# ID            Status    Uptime    Memory    CPU
# my-api        running   2d 3h     256MB     12%
# chatbot       running   5h 12m    128MB     8%
# support       stopped   -         -         -

# Stop formation
muxi formation stop my-api

# Restart formation
muxi formation restart my-api

# Delete formation (stop + remove)
muxi formation delete my-api

# View logs
muxi formation logs my-api
muxi formation logs my-api --follow  # Tail logs
```

---

### 4. Formation Registry

**In-Memory + Persistent:**
```go
type FormationRegistry struct {
    formations map[string]*FormationInfo
}

type FormationInfo struct {
    ID          string    // "my-api"
    Port        int       // 8001
    ProcessID   int       // pm2-go process ID
    ConfigPath  string    // "formations/my-api.yaml"
    Status      string    // "running", "stopped", "crashed"
    StartedAt   time.Time
    RestartCount int
}
```

**Persistence:**
- Saved to: `~/.muxi-server/registry.json`
- Loaded on server start
- Updated on formation changes

---

### 5. Port Allocation

**Port Pool Management:**
```go
type PortPool struct {
    start     int   // 8000
    end       int   // 9000
    available []int // [8000, 8001, 8002, ...]
    used      map[int]string // {8001: "my-api"}
}

// Allocate port for new formation
port := portPool.Allocate("my-api")
// Returns: 8001

// Release port when formation deleted
portPool.Release(8001)
```

**Configuration:**
```yaml
# ~/.muxi-server/config.yaml
server:
  port: 3000  # Server HTTP port
  
formations:
  port_range_start: 8000
  port_range_end: 9000
  max_formations: 100
```

---

### 6. Health Checks

**Automatic Health Monitoring:**
```go
// On formation start
go func() {
    time.Sleep(2 * time.Second)
    
    resp, err := http.Get(fmt.Sprintf("http://localhost:%d/health", port))
    if err != nil || resp.StatusCode != 200 {
        logger.Error("Formation failed health check")
        // Mark as unhealthy, retry
    }
}()

// Periodic health checks
ticker := time.NewTicker(30 * time.Second)
for range ticker.C {
    checkFormationHealth(formationID)
}
```

**Health Endpoint (Formation Runtime):**
```python
# run.py (formation runtime)
@app.get("/health")
async def health():
    return {
        "status": "ok",
        "formation": formation.name,
        "agents": len(formation.agents),
        "uptime": time.time() - start_time
    }
```

---

## Installation

### One-Command Install

```bash
curl https://muxi.org/install | bash
```

**What It Does:**
1. **Detect OS**: macOS, Linux (Ubuntu/Debian/RHEL/Arch)
2. **Install Dependencies**:
   - **uv** (Python package manager)
   - **Python 3.13+** (if not present or too old)
3. **Download MUXI Server**:
   - Binary for OS/arch: `muxi-server-{os}-{arch}`
   - Install to: `/usr/local/bin/muxi-server`
4. **Download MUXI CLI**:
   - Install via uv: `uv tool install muxi-cli`
5. **Initialize**:
   - Create config: `~/.muxi-server/config.yaml`
   - Create dirs: `~/.muxi-server/formations/`
6. **Start Server**:
   - Optional: Install as systemd service
   - Or run manually: `muxi-server serve`

**Install Script:**
```bash
#!/bin/bash
set -e

echo "🚀 Installing MUXI Server..."

# Detect OS
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)
case $ARCH in
    x86_64) ARCH="amd64" ;;
    aarch64|arm64) ARCH="arm64" ;;
esac

# Install uv
echo "📦 Installing uv..."
curl -LsSf https://astral.sh/uv/install.sh | sh

# Install Python 3.13+ (if needed)
if ! command -v python3 &> /dev/null || \
   [ $(python3 -c 'import sys; print(sys.version_info >= (3,13))') != "True" ]; then
    echo "🐍 Installing Python 3.13..."
    uv python install 3.13
fi

# Download MUXI Server binary
echo "📥 Downloading MUXI Server..."
VERSION="latest"
URL="https://github.com/muxi-ai/server/releases/download/${VERSION}/muxi-server-${OS}-${ARCH}"
curl -L $URL -o /tmp/muxi-server
chmod +x /tmp/muxi-server
sudo mv /tmp/muxi-server /usr/local/bin/muxi-server

# Install MUXI CLI
echo "💻 Installing MUXI CLI..."
uv tool install muxi-cli

# Initialize server
echo "⚙️  Initializing..."
muxi-server init

# Ask about systemd service
if command -v systemctl &> /dev/null; then
    read -p "Install as systemd service? [Y/n] " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        muxi-server install-service
        sudo systemctl enable muxi-server
        sudo systemctl start muxi-server
        echo "✅ MUXI Server running as systemd service"
    fi
fi

echo ""
echo "✅ Installation complete!"
echo ""
echo "Start server:  muxi-server serve"
echo "Deploy formation:  muxi formation deploy formation.yaml"
echo "Documentation: https://muxi.org/docs/server"
```

---

## Technical Design

### Technology Stack

**Server (Go):**
- **Base**: pm2-go (forked)
- **HTTP Router**: gorilla/mux
- **HTTP Proxy**: net/http/httputil (ReverseProxy)
- **YAML**: gopkg.in/yaml.v3
- **Process Management**: os/exec, syscall

**CLI (Python):**
- **Framework**: typer (CLI framework)
- **HTTP Client**: httpx
- **YAML**: PyYAML

**Formation Runtime (Python):**
- **Web Framework**: FastAPI
- **Server**: uvicorn
- **Formation**: muxi (existing runtime)

---

### Server Configuration

```yaml
# ~/.muxi-server/config.yaml

server:
  # HTTP server settings
  port: 3000
  host: "0.0.0.0"
  
  # Process management
  formations_dir: "~/.muxi-server/formations"
  logs_dir: "~/.muxi-server/logs"
  
  # Port allocation for formations
  port_range_start: 8000
  port_range_end: 9000
  max_formations: 100
  
  # Health checks
  health_check_interval: 30  # seconds
  health_check_timeout: 5    # seconds
  startup_health_check_delay: 2  # seconds
  
  # Process settings
  auto_restart: true
  max_restart_count: 10
  restart_delay: 1  # seconds
  
  # Log rotation
  log_rotation_enabled: true
  log_max_size: "10M"
  log_max_files: 10
  
  # Telemetry
  telemetry_enabled: true
  server_id: ""  # Auto-generated on first run

# Optional: TLS
tls:
  enabled: false
  cert_file: ""
  key_file: ""
```

---

### Formation Lifecycle

**Deployment Flow:**
```
1. CLI: muxi formation deploy formation.yaml
   ↓
2. CLI: Read YAML, POST /formations/deploy
   ↓
3. Server: Validate YAML schema
   ↓
4. Server: Generate formation ID (or use provided)
   ↓
5. Server: Inject metadata
   config["_server_id"] = server.ID
   config["_deployment_mode"] = "server"
   ↓
6. Server: Save to formations/formation-{id}.yaml
   ↓
7. Server: Allocate port from pool
   ↓
8. Server: Spawn process
   python run.py formations/formation-{id}.yaml --port {port}
   ↓
9. Server: Register in formation registry
   ↓
10. Server: Wait for health check
    ↓
11. Server: Return formation URL to CLI
    ↓
12. CLI: Display success message
```

**Request Flow:**
```
1. User: POST https://server.com/my-api/chat
   ↓
2. Server: Parse route: formation="my-api", endpoint="/chat"
   ↓
3. Server: Lookup formation in registry → Port 8001
   ↓
4. Server: Rewrite URL: localhost:8001/chat
   ↓
5. Server: Proxy request (preserve headers, body)
   ↓
6. Formation: Process via FastAPI → overlord.chat()
   ↓
7. Formation: Return response
   ↓
8. Server: Forward response to user
```

**Crash Recovery:**
```
1. Formation crashes (exit code != 0)
   ↓
2. pm2-go: Detect crash
   ↓
3. pm2-go: Check restart count < max_restart_count
   ↓
4. pm2-go: Wait restart_delay seconds
   ↓
5. pm2-go: Spawn new process
   ↓
6. Server: Update registry status → "restarting"
   ↓
7. Server: Health check
   ↓
8. Server: Update registry status → "running"
```

---

## API Specification

### Management API (for CLI)

**Base URL**: `http://localhost:3000`

#### Deploy Formation
```http
POST /formations/deploy
Content-Type: multipart/form-data

{
  "yaml": "<formation.yaml contents>",
  "id": "my-api"  // Optional custom ID
}

Response 201:
{
  "formation_id": "my-api",
  "port": 8001,
  "status": "running",
  "url": "http://localhost:3000/my-api",
  "health_url": "http://localhost:3000/my-api/health"
}
```

#### List Formations
```http
GET /formations

Response 200:
{
  "formations": [
    {
      "id": "my-api",
      "status": "running",
      "port": 8001,
      "uptime": "2d 3h 45m",
      "memory_mb": 256,
      "cpu_percent": 12.5,
      "restart_count": 0
    }
  ]
}
```

#### Get Formation
```http
GET /formations/{id}

Response 200:
{
  "id": "my-api",
  "status": "running",
  "port": 8001,
  "config_path": "/home/user/.muxi-server/formations/my-api.yaml",
  "started_at": "2024-01-15T10:00:00Z",
  "uptime_seconds": 186270,
  "restart_count": 0,
  "health": {
    "last_check": "2024-01-15T10:30:00Z",
    "status": "ok"
  }
}
```

#### Stop Formation
```http
POST /formations/{id}/stop

Response 200:
{
  "id": "my-api",
  "status": "stopped"
}
```

#### Restart Formation
```http
POST /formations/{id}/restart

Response 200:
{
  "id": "my-api",
  "status": "restarting"
}
```

#### Delete Formation
```http
DELETE /formations/{id}

Response 200:
{
  "id": "my-api",
  "status": "deleted"
}
```

#### Get Logs
```http
GET /formations/{id}/logs?lines=100&follow=false

Response 200:
{
  "logs": "...",
  "lines": 100
}
```

---

### Proxy API (for users/SDKs)

**Base URL**: `http://localhost:3000`

All requests to `/{formation_id}/*` are proxied to the formation runtime.

**Examples:**
```http
# Chat
POST /my-api/chat
→ localhost:8001/chat

# Workflow
POST /my-api/workflow
→ localhost:8001/workflow

# Custom endpoint
GET /my-api/custom/route
→ localhost:8001/custom/route
```

**Headers Preserved:**
- `Content-Type`
- `Authorization`
- `X-User-ID` (custom headers)
- All user headers forwarded

**Headers Added:**
```
X-Forwarded-For: <client-ip>
X-Forwarded-Proto: http|https
X-Forwarded-Host: server.com
```

---

## Implementation Plan

### Phase 1: Fork & Setup (Week 1)

**Goal**: Working pm2-go fork with MUXI branding

**Tasks:**
- [ ] Fork pm2-go → muxi-server
- [ ] Update imports (github.com/dunstorm/pm2-go → github.com/muxi-ai/server)
- [ ] Add dependencies (gorilla/mux, yaml.v3)
- [ ] Setup CI/CD (GitHub Actions)
- [ ] Setup releases (GoReleaser)
- [ ] Add basic README

---

### Phase 2: Formation Deployment (Week 2)

**Goal**: Deploy formations with YAML injection

**Tasks:**
- [ ] Add `formation/` package
- [ ] Implement YAML injection
- [ ] Implement port allocation
- [ ] Implement formation registry (in-memory + persistent)
- [ ] Add `muxi-server deploy` command
- [ ] Integrate with pm2-go process spawning
- [ ] Add formation validation
- [ ] Unit tests for deployment logic

---

### Phase 3: HTTP Proxy (Week 3)

**Goal**: Route user requests to formations

**Tasks:**
- [ ] Add `proxy/` package
- [ ] Implement reverse proxy with path rewriting
- [ ] Add route parsing (/{formation_id}/{endpoint})
- [ ] Add formation lookup
- [ ] Add error handling (formation not found, unhealthy)
- [ ] Add request/response logging
- [ ] Add proxy middleware (headers, timeouts)
- [ ] Load testing (1000+ req/s)

---

### Phase 4: Management API (Week 4)

**Goal**: REST API for CLI integration

**Tasks:**
- [ ] Add `api/` package
- [ ] Implement management endpoints
- [ ] Add authentication (API keys)
- [ ] Add request validation
- [ ] Add error responses
- [ ] OpenAPI spec generation
- [ ] Integration tests

---

### Phase 5: CLI Integration (Week 5)

**Goal**: CLI commands for formation management

**Tasks:**
- [ ] Create `muxi-cli` package (Python)
- [ ] Implement `muxi formation deploy`
- [ ] Implement `muxi formation list`
- [ ] Implement `muxi formation stop/restart/delete`
- [ ] Implement `muxi formation logs`
- [ ] Add progress indicators
- [ ] Add error messages
- [ ] Add configuration (~/.muxi/config)

---

### Phase 6: Health & Monitoring (Week 6)

**Goal**: Health checks and process monitoring

**Tasks:**
- [ ] Implement health check system
- [ ] Add startup health checks
- [ ] Add periodic health checks
- [ ] Add health status to registry
- [ ] Add `/formations/{id}/health` endpoint
- [ ] Add alerting (optional)
- [ ] Dashboard (optional, future)

---

### Phase 7: Installation & Distribution (Week 7)

**Goal**: One-command install experience

**Tasks:**
- [ ] Write install script (install.sh)
- [ ] Test on macOS, Linux (Ubuntu, Debian, RHEL, Arch)
- [ ] Add systemd service generation
- [ ] Add server initialization (`muxi-server init`)
- [ ] Setup CDN for binary distribution
- [ ] Add auto-update mechanism (optional)
- [ ] Documentation (installation guide)

---

### Phase 8: Polish & Documentation (Week 8)

**Goal**: Production-ready release

**Tasks:**
- [ ] Comprehensive documentation
- [ ] Example formations
- [ ] Deployment guides (AWS, GCP, DigitalOcean)
- [ ] Troubleshooting guide
- [ ] Performance benchmarks
- [ ] Security audit
- [ ] Beta testing with users
- [ ] v1.0.0 release

---

## Success Metrics

### Adoption
- **Target**: 1,000 server installations in 3 months
- **Measurement**: Telemetry (anonymous server IDs)

### Ease of Use
- **Target**: < 5 minutes from install to deployed formation
- **Measurement**: User feedback, time tracking in install script

### Reliability
- **Target**: 99.9% uptime for server process
- **Target**: < 3 seconds formation recovery after crash
- **Measurement**: Health check logs, crash recovery metrics

### Performance
- **Target**: < 5ms proxy overhead (server → formation)
- **Target**: Handle 1,000+ req/s per server instance
- **Measurement**: Load testing, production metrics

### User Satisfaction
- **Target**: 4.5+ star rating on feedback
- **Measurement**: User surveys, GitHub issues/discussions

---

## Future Enhancements (Post-v1.0)

### Multi-Server Clustering
- Load balancing across multiple MUXI Servers
- Shared formation registry (Redis/etcd)
- Formation migration between servers

### Advanced Routing
- Path-based routing beyond formation ID
- Domain-based routing (api.myapp.com → formation-api)
- Rate limiting per formation/user

### Observability Dashboard
- Web UI for formation management
- Real-time metrics (requests/s, latency, errors)
- Log aggregation and search

### Container Support
- Docker integration (run formations in containers)
- Kubernetes deployment option
- Resource limits (CPU/memory per formation)

### Authentication & Authorization
- API key management for formations
- User authentication (OAuth, JWT)
- Role-based access control

---

## Risks & Mitigation

### Risk 1: Process Management Complexity
**Risk**: pm2-go limitations or bugs

**Mitigation**:
- Fork gives us full control to fix issues
- Small codebase (~5k lines) easy to understand
- Active pm2-go community for reference

**Likelihood**: Medium

---

### Risk 2: Port Exhaustion
**Risk**: Run out of available ports (8000-9000 = 1000 max)

**Mitigation**:
- Configurable port range
- Port recycling when formations deleted
- Multi-server deployment for scaling

**Likelihood**: Low

---

### Risk 3: Proxy Performance
**Risk**: HTTP proxy becomes bottleneck

**Mitigation**:
- Go's net/http is highly optimized
- Benchmarking shows <5ms overhead
- Can scale horizontally (multiple servers)

**Likelihood**: Low

---

### Risk 4: Installation Complexity
**Risk**: Install script fails on different OS/envs

**Mitigation**:
- Test on all major platforms (macOS, Ubuntu, Debian, RHEL, Arch)
- Provide manual installation docs
- Clear error messages in install script
- Community feedback during beta

**Likelihood**: Medium

---

## Dependencies

### Build Dependencies
- Go 1.21+
- Python 3.13+
- uv (Python package manager)

### Runtime Dependencies
- None (single binary for server)
- Python 3.13+ (for formations)

### Third-Party Libraries
- **pm2-go**: Process management (forked)
- **gorilla/mux**: HTTP routing
- **gopkg.in/yaml.v3**: YAML parsing
- **typer**: CLI framework (Python)
- **fastapi**: Formation API server (Python)

---

## Open Questions

1. **Multi-tenancy**: Should one server support multiple isolated tenants?
2. **Storage**: Local disk vs cloud storage for formation YAMLs?
3. **Secrets**: Built-in secrets management or rely on formation secrets.env?
4. **Pricing**: Free forever or future paid enterprise features?

---

## Definition of Done

- [ ] One-command install works on macOS, Linux
- [ ] Deploy formation via CLI
- [ ] Proxy requests to formations work
- [ ] Formations auto-restart on crash
- [ ] Health checks work
- [ ] Logs accessible via CLI
- [ ] Stop/restart/delete formations work
- [ ] Documentation complete
- [ ] Beta tested by 10+ users
- [ ] Performance benchmarks meet targets
- [ ] v1.0.0 released on GitHub

---

**Document Version**: 1.0  
**Last Updated**: 2024-01-16  
**Owner**: @ranaroussi 
**Status**: Ready for Implementation
