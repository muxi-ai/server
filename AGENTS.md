# AGENTS.md - AI Agent Development Guide

**Project:** MUXI Server  
**Version:** 1.0.0-dev  
**Status:** Phase 1 Complete ✅ - Ready for Phase 2 (Client CLI)  
**Last Updated:** 2025-10-17

---

## Project Overview

MUXI Server is a production-grade orchestration platform for deploying and managing MUXI formations at scale. It combines robust process management with intelligent HTTP routing, enabling organizations to run multiple AI formations as a unified service.

**Think:** Docker + PM2 + Nginx in a single Go binary, purpose-built for MUXI formations.

**Key Features:**
- 🚀 One-command deploy: formations become production APIs instantly
- 🎯 Zero configuration: formations run as-is
- 📦 Single binary: no dependencies except runtime (Singularity/Docker)
- 🔄 Auto-recovery: crashed formations restart automatically
- 🌐 Smart routing: HTTP proxy with formation-level isolation
- 📊 Built-in telemetry: usage tracking without touching formation code

---

## Architecture Overview

```
┌─────────────────────────────────────────────┐
│ MUXI Server (Go Binary)                     │
│  - HTTP API (port 3000)                     │
│  - HTTP Proxy (/{formation_id}/*)           │
│  - Formation Registry (in-memory + persist) │
│  - Process Manager (spawning & monitoring)  │
│  - Port Allocator (8000-9000 pool)          │
└─────────────────────────────────────────────┘
              ↓
    Spawns formation runtimes
              ↓
┌──────────────┐ ┌──────────────┐ ┌──────────────┐
│ Formation 1  │ │ Formation 2  │ │ Formation 3  │
│ Port: 8001   │ │ Port: 8002   │ │ Port: 8003   │
│ FastAPI      │ │ FastAPI      │ │ FastAPI      │
│ /chat        │ │ /chat        │ │ /workflow    │
│ /health      │ │ /health      │ │ /health      │
└──────────────┘ └──────────────┘ └──────────────┘
```

---

## Key Architectural Decisions

### 1. Formation Runtime (Issue #1, comments)
- **Phase 1 (Current):** Spawn Python processes directly (`python app.py`)
- **Phase 2+ (Future):** Use Singularity/Apptainer SIF images (self-contained containers)
- **Why:** Clean server (no Python pollution), container isolation, single file distribution

### 2. Server Process Management (Issue #1, comments)
- **Production:** systemd (Linux) / launchd (macOS) service
- **Development:** Manual execution (`muxi-server serve`)
- Install script offers service setup by default

### 3. Base Code (PRD)
- Adapted from **pm2-go** (https://github.com/hatchet/pm2-go) for process management
- Stripped: gRPC, Cobra CLI, cron features, JSON config
- Kept: Process spawning, monitoring, log rotation, PID management
- Added: HTTP API, formation registry, port allocation, HTTP proxy
- **Note:** pm2-go code is available in git history and at the original repo if needed for reference

### 4. Language & Dependencies
- **Server:** Go (single binary, no runtime dependencies)
- **Formations:** Python (FastAPI servers)
- **Minimal deps:** gorilla/mux (routing), zerolog (logging), yaml.v3 (parsing)

---

## Directory Structure

```
/
├── AGENTS.md              # This file - AI agent guide
├── AUTH.md                # Authentication design document
├── PRD.md                 # Product Requirements Document
├── README.md              # Project overview
├── LICENSE                # MIT License
├── .gitignore
├── docs/                  # User documentation
│   ├── README.md
│   ├── getting-started.md
│   ├── installation.md
│   ├── configuration.md
│   ├── authentication.md
│   ├── formations.md
│   ├── api-reference.md
│   └── troubleshooting.md
│
└── src/                   # MUXI Server implementation
    ├── cmd/
    │   └── server/
    │       └── main.go            # Entry point: muxi-server serve
    │
    ├── pkg/
    │   ├── process/               # Process lifecycle management
    │   │   ├── process.go         # Process types & structs
    │   │   ├── spawn.go           # Process spawning (adapted from pm2-go)
    │   │   ├── monitor.go         # Health checks & auto-restart
    │   │   └── manager.go         # Orchestration layer
    │   │
    │   ├── registry/              # Formation tracking
    │   │   ├── formation.go       # Formation info struct
    │   │   ├── registry.go        # In-memory registry (thread-safe)
    │   │   ├── ports.go           # Port pool allocation
    │   │   └── persistence.go     # Save/load to JSON
    │   │
    │   ├── api/                   # HTTP API endpoints
    │   │   ├── server.go          # HTTP server setup
    │   │   ├── deploy.go          # POST /formations/deploy
    │   │   ├── list.go            # GET /formations
    │   │   └── errors.go          # Error handling
    │   │
    │   ├── proxy/                 # HTTP reverse proxy (future)
    │   │   └── proxy.go           # /{formation_id}/* routing
    │   │
    │   └── config/                # Configuration management
    │       └── config.go          # Load ~/.muxi-server/config.yaml
    │
    ├── test/                      # Test fixtures
    │   ├── dummy_app.py           # Simple FastAPI app for testing
    │   └── fixtures/              # Test YAML files
    │
    ├── go.mod                     # Go module definition
    └── go.sum                     # Dependency checksums
```

---

## Development Phases

### ✅ Phase 1: Baseline Server (COMPLETE!)
**Goal:** Production-ready server with full API and bundle deployment

- [x] Issue #3: Project setup & structure
- [x] Issue #4: Process management core
- [x] Issue #5: Formation registry & port allocation
- [x] Issue #6: HTTP API (8 endpoints - full CRUD)
- [x] HTTP proxy routing (`/v1/{formation_id}/*`)
- [x] HMAC authentication
- [x] Server CLI commands (`init`, `version`, `config show`)
- [x] Formation bundle upload (gzip tarball support)
- [x] Server ID generation & metadata injection
- [x] Comprehensive documentation (8 user docs + 3 implementation summaries)

**Deliverables:** Production-ready server with ~5,000+ lines of code

### 🔜 Phase 2: Client CLI Tool (Separate Project)
Build standalone `muxi` CLI tool for formation management:
- Profile management (`~/.muxi/profiles.yaml`)
- HMAC request signing (reusable auth library)
- Formation deployment (`muxi formation deploy`)
- Formation management (list, get, stop, restart, delete, logs)
- Log streaming with `--follow` flag
- Configuration commands

**Timeline:** 1-2 weeks  
**Repository:** Separate from server (independent evolution)

### 🔜 Phase 3: Singularity/Apptainer Runtime
- Replace direct Python spawning with SIF execution
- Build runtime Docker image → convert to SIF
- Update spawn logic to execute `.sif` files
- Clean server (no Python dependencies)

### 🔜 Phase 4: Installation & Distribution
- Install script (`curl | bash`)
- Homebrew tap, APT/YUM repos
- Docker images
- systemd/launchd service generation

### 🔜 Phase 5: Multi-Server Orchestration
- Server registration API
- Formation telemetry aggregation
- Multi-server deployment
- Server health dashboard

---

## Coding Conventions

### Go Style
- Follow standard Go conventions (gofmt, golint)
- Use **zerolog** for structured logging
- Error handling: always check errors, wrap with context
- Package naming: lowercase, single word
- Interfaces over concrete types where appropriate

### File Organization
- One package per directory
- `*_test.go` files alongside implementation
- Internal packages in `pkg/` (not `internal/` - we want flexibility)

### Logging
```go
import "github.com/rs/zerolog/log"

// Use structured logging
log.Info().
    Str("formation_id", formationID).
    Int("port", port).
    Msg("Formation started")

log.Error().
    Err(err).
    Str("formation_id", formationID).
    Msg("Failed to spawn formation")
```

### Error Handling
```go
// Wrap errors with context
if err != nil {
    return fmt.Errorf("failed to spawn process: %w", err)
}

// Use custom error types for API
type FormationNotFoundError struct {
    ID string
}
```

### Configuration
```yaml
# ~/.muxi-server/config.yaml
server:
  port: 3000
  host: "0.0.0.0"

formations:
  runtime_type: "singularity"  # singularity|docker|native
  port_range_start: 8000
  port_range_end: 9000
  logs_dir: "~/.muxi-server/logs"
  
  auto_restart: true
  max_restart_count: 10
  restart_delay: 1
```

---

## Key Dependencies

| Package | Purpose | Why This One |
|---------|---------|--------------|
| `github.com/gorilla/mux` | HTTP routing | Industry standard, clean API, powerful |
| `gopkg.in/yaml.v3` | YAML parsing | Latest version, good API |
| `github.com/rs/zerolog` | Structured logging | Zero allocation, fast, JSON output |

### Dependencies to AVOID
- ❌ `github.com/spf13/cobra` - We're not building a CLI (server only)
- ❌ `google.golang.org/grpc` - Too heavy, HTTP is simpler
- ❌ `github.com/aptible/supercronic` - No cron features needed

---

## Testing Strategy

### Unit Tests
- All packages must have `*_test.go` files
- Target: >80% coverage
- Use table-driven tests for multiple cases
```go
func TestPortAllocate(t *testing.T) {
    tests := []struct {
        name    string
        start   int
        end     int
        want    int
    }{
        {"first port", 8000, 8010, 8000},
        {"second port", 8000, 8010, 8001},
    }
    // ...
}
```

### Integration Tests
- `test/dummy_app.py` - Simple FastAPI server for end-to-end tests
- Test full flow: deploy → spawn → health check → list → stop

### Manual Testing
```bash
# Start server
go run ./src/cmd/server serve

# Deploy formation (in another terminal)
curl -X POST http://localhost:3000/formations/deploy \
  -H "Content-Type: application/json" \
  -d '{"command": "python test/dummy_app.py", "id": "test-1"}'

# List formations
curl http://localhost:3000/formations

# Check formation health
curl http://localhost:8001/health
```

---

## Code Adapted from pm2-go

We've already adapted the necessary code from **pm2-go** (https://github.com/hatchet/pm2-go).

### What We Adapted

1. **Process Spawning** (now in `pkg/process/spawn.go`)
   - `exec.Command` setup
   - Stdout/stderr redirection to log files
   - PID tracking and management

2. **Process Monitoring** (now in `pkg/process/monitor.go`)
   - Check if process running by PID
   - Auto-restart logic
   - Restart count limiting

3. **Log Management** (integrated into process management)
   - Log file creation
   - Log rotation (size-based)
   - Keep N old log files

### What We Removed

1. **gRPC Communication** - Using HTTP/REST instead
2. **CLI Commands** - Server only (CLI will be separate tool)
3. **Protobuf Definitions** - Not needed for HTTP API
4. **Cron Scheduling** - Out of scope for MUXI Server
5. **JSON Process Config** - We use YAML for formations

### Reference

If you need to reference the original pm2-go code:
- **GitHub:** https://github.com/hatchet/pm2-go
- **Our adapted code:** See `src/pkg/process/` directory
- **Git history:** Earlier commits contain the full pm2-go codebase

---

## Common Tasks

### Adding a New API Endpoint
1. Create handler in `pkg/api/{name}.go`
2. Register route in `pkg/api/server.go`
3. Add tests in `pkg/api/{name}_test.go`
4. Update this doc

### Adding a Configuration Option
1. Update struct in `pkg/config/config.go`
2. Update default config template
3. Document in PRD.md

### Debugging
```bash
# Enable debug logging
export MUXI_LOG_LEVEL=debug
go run ./src/cmd/server serve

# Check process status
ps aux | grep muxi

# Check logs
tail -f ~/.muxi-server/logs/formation-{id}.log

# Check registry
cat ~/.muxi-server/registry.json | jq
```

---

## Important Files

| File | Purpose | When to Modify |
|------|---------|----------------|
| `PRD.md` | Product requirements, full spec | Clarifying features, architecture changes |
| `AGENTS.md` | This file - AI agent guide | New conventions, structure changes |
| `src/cmd/server/main.go` | Entry point | Adding commands, startup logic |
| `src/pkg/process/manager.go` | Core process orchestration | Changing process lifecycle |
| `src/pkg/registry/registry.go` | Formation tracking | Adding formation metadata |
| `src/pkg/api/server.go` | HTTP server setup | Adding routes, middleware |
| `test/dummy_app.py` | Test fixture | Adding test endpoints |

---

## Git Workflow

### Branch Naming
- `feature/phase-1.1-setup` - New features
- `fix/port-allocation-bug` - Bug fixes
- `refactor/process-spawn` - Refactoring
- `docs/update-readme` - Documentation

### Commit Messages
Follow the repository's existing style (check `git log --oneline`):
```
Add process spawning logic

- Implement process spawning based on pm2-go patterns
- Support command-line arguments
- Redirect stdout/stderr to log files

Co-authored-by: factory-droid[bot] <138933559+factory-droid[bot]@users.noreply.github.com>
```

### Before Committing
```bash
# Format code
go fmt ./...

# Run tests
go test ./... -v

# Check for issues
go vet ./...

# Ensure builds
go build ./src/cmd/server
```

---

## Key GitHub Issues

- **#1** - Master epic (PRD: MUXI Server)
- **#2** - Distribution & Installation Strategy
- **#3** - Phase 1.1: Project Setup & Structure ⬅️ **START HERE**
- **#4** - Phase 1.2: Process Management Core
- **#5** - Phase 1.3: Formation Registry & Port Allocation
- **#6** - Phase 1.4: HTTP API Server (2 Endpoints)

---

## Reference Links

- **PRD:** See `PRD.md` in repository root
- **AUTH.md:** Complete authentication design document
- **docs/:** User-facing documentation for deployers
- **pm2-go (original):** https://github.com/hatchet/pm2-go (process management reference)
- **Issue #1 Comments:** Architectural decisions documented there
  - Process management approach
  - Runtime strategy (Singularity/Apptainer)
  - Dependency audit
  - Phase 1 implementation plan

---

## Quick Start for AI Agents

### Working on Phase 1
1. Read this file (AGENTS.md)
2. Review `PRD.md` for context
3. Check current phase in issue tracker
4. Follow issue checklist (e.g., #3 for setup)
5. Write tests alongside implementation
6. Update this file if adding new patterns

### Making Changes
1. Understand the phase goal (check issue description)
2. Review existing code in `src/pkg/` for patterns
3. Follow coding conventions above
4. Write/update tests
5. Run `go fmt`, `go vet`, `go test`
6. Commit with descriptive message

### Getting Context
- **Architecture decisions:** Issue #1 comments
- **Current phase:** Check open issues #3-6
- **Design patterns:** Look at existing `pkg/` code
- **Testing patterns:** Look at `*_test.go` files

---

## Notes for AI Agents

### When Adding New Code
- Always add logging (use zerolog)
- Always add error handling with context
- Always write tests (aim for >80% coverage)
- Always update this doc if adding new patterns

### When Adding New Features
- Look at existing patterns in `src/pkg/` directories
- Follow the same structure and conventions
- Add comprehensive error handling
- Write tests alongside implementation
- Update documentation (AGENTS.md, docs/)

### When Unsure
- Check PRD.md for requirements
- Check AUTH.md for authentication details
- Check issue #1 comments for architectural decisions
- Review existing code for established patterns
- Ask for clarification before making assumptions

---

## Success Criteria

### Phase 1 Complete ✅ ALL ACHIEVED!
- [x] `go build ./src/cmd/server` compiles
- [x] `muxi-server init` generates credentials
- [x] `muxi-server start` starts HTTP server
- [x] `POST /formations/deploy` accepts tarball bundles
- [x] Full CRUD API (8 endpoints implemented)
- [x] HTTP proxy routing works
- [x] HMAC authentication functional
- [x] Formation bundle upload with metadata injection
- [x] Server ID generation (hostname + SHA256 hash)
- [x] Killed formation auto-restarts
- [x] Tests pass: `go test ./... -v`
- [x] Documentation complete (8 user docs + 3 implementation summaries)

---

**Last Updated:** 2025-10-17  
**Current Phase:** Phase 1 Complete ✅  
**Next Milestone:** Phase 2 - Build `muxi` CLI tool
