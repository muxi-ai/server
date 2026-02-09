# AGENTS.md - AI Agent Development Guide

**Project:** MUXI Server
**Language:** Go
**License:** Elastic License 2.0

---

## What This Is

MUXI Server is a single-binary orchestration platform for deploying and managing AI agent formations. It combines process management, HTTP reverse proxy, port allocation, and HMAC authentication into one Go binary.

This repo is part of the [MUXI ecosystem](https://github.com/muxi-ai/muxi/blob/main/ARCHITECTURE.md).

---

## Repository Structure

```
.
├── AGENTS.md                  # This file
├── CHANGELOG.md               # Release changelog
├── README.md                  # Public README
├── LICENSE-ELv2               # Elastic License 2.0
├── SECURITY.md                # Security policy (links to muxi repo)
├── Dockerfile                 # Multi-arch Docker build
├── docker-compose.yml         # Docker Compose setup
├── contributing/              # Contributor documentation
│   ├── README.md              # Contributing guide + install instructions
│   ├── auth.md                # HMAC authentication design
│   ├── cli-protocol.md        # CLI-Server communication protocol
│   ├── how-formations-run.md  # Formation runtime execution
│   ├── runtime-architecture.md # SIF/Docker runtime architecture
│   ├── runtime-auto-download.md # Runtime auto-download
│   └── windows-dev.md         # Windows development guide
├── scripts/test/              # Test scripts
├── .github/
│   ├── workflows/             # CI, RC, Release, Docker (all SHA-pinned)
│   └── dependabot.yml         # Automated dependency updates
└── src/                       # Source code
    ├── go.mod
    ├── cmd/server/
    │   ├── main.go            # Entry point
    │   ├── commands.go        # CLI commands (init, start, version)
    │   └── .version           # ScalVer version file
    └── pkg/
        ├── api/               # HTTP API endpoints & middleware
        ├── auth/              # HMAC authentication
        ├── config/            # Configuration (YAML)
        ├── formation/         # Formation bundle handling
        ├── process/           # Process lifecycle & auto-restart
        ├── proxy/             # HTTP reverse proxy
        ├── registry/          # Formation registry & port allocation
        ├── runtime/           # Singularity/Docker runtime
        └── telemetry/         # Anonymous usage telemetry
```

---

## Architecture

```
┌──────────────────────────────────────────────────────┐
│ MUXI Server (Port 7890)                              │
│                                                      │
│  GET  /health, /ping          Public (no auth)       │
│                                                      │
│  /rpc/formations/*            Management API (HMAC)  │
│    POST   /deploy             Deploy formation       │
│    GET    /                   List formations        │
│    GET    /{id}               Get formation          │
│    PUT    /{id}               Update formation       │
│    POST   /{id}/stop          Stop formation         │
│    POST   /{id}/restart       Restart formation      │
│    POST   /{id}/rollback      Rollback version       │
│    DELETE /{id}               Delete formation       │
│    GET    /{id}/logs          Get logs               │
│  GET  /rpc/server/status      Server statistics      │
│  GET  /rpc/server/logs        Audit logs             │
│                                                      │
│  /rpc/dev/*                   Dev/Draft API (HMAC)   │
│    POST /run                  Start draft formation  │
│    POST /stop                 Stop draft formation   │
│                                                      │
│  ALL  /api/{formation_id}/*   Formation proxy (live) │
│  ALL  /draft/{formation_id}/* Formation proxy (draft)│
└──────────────────────────────────────────────────────┘
                       │
         Spawns & proxies to formations
         (bind to 127.0.0.1 only)
                       │
        ┌──────────────┼──────────────┐
        ▼              ▼              ▼
   Formation 1    Formation 2    Formation 3
   :8001          :8002          :8003
```

**Key design decisions:**
- Formations bind to localhost only (all traffic through proxy)
- Port pool: 8000-9000, auto-allocated
- Versioning: `current/` and `previous/` directories with `version.json`
- Update: upload -> stop -> backup current -> extract new -> start
- Rollback: stop -> swap current/previous -> update metadata -> start
- Draft/dev: separate registry, same ID can have live + draft running simultaneously

---

## Dependencies

| Package | Purpose |
|---------|---------|
| `github.com/gorilla/mux` | HTTP routing |
| `github.com/rs/zerolog` | Structured logging (zero-alloc) |
| `gopkg.in/yaml.v3` | YAML parsing |
| `golang.org/x/sys` | Platform-specific syscalls |

No other dependencies. Keep it minimal.

---

## Coding Conventions

### Go Style
- `gofmt` and `go vet` before committing
- Wrap errors with context: `fmt.Errorf("failed to X: %w", err)`
- One package per directory
- `*_test.go` alongside implementation

### Logging
```go
log.Info().Str("formation_id", id).Int("port", port).Msg("Formation started")
log.Error().Err(err).Str("formation_id", id).Msg("Failed to spawn formation")
```

### Configuration
Config lives at `~/.muxi/server/config.yaml`:
```yaml
server:
  port: 7890
  host: "0.0.0.0"
formations:
  port_range_start: 8000
  port_range_end: 9000
  bind_host: "127.0.0.1"
  auto_restart: true
  max_restart_count: 10
  restart_delay: 1
runtime:
  sif_base_url: "https://github.com/muxi-ai/runtime/releases/download"
  auto_download: true
  runtime_runner_image: "ghcr.io/muxi-ai/runtime-runner:latest"
logging:
  level: "info"
  audit_log: "logs/audit.log"
```

---

## Testing

```bash
cd src

# Run all tests
go test ./... -v -race

# With coverage
go test ./... -coverprofile=coverage.out
go tool cover -func=coverage.out

# Fuzz tests
go test ./pkg/registry/... -fuzz FuzzValidateFormationID -fuzztime 5s
go test ./pkg/auth/... -fuzz FuzzComputeHMAC -fuzztime 5s
```

- Use table-driven tests
- Test ports should use 19000+ range (avoid conflicts with formation port pool)
- CI threshold: 45% (platform-specific code is untestable on single OS)

---

## Common Tasks

### Adding a New API Endpoint
1. Create handler in `pkg/api/{name}.go`
2. Register route in `pkg/api/server.go`
3. Add tests in `pkg/api/{name}_test.go`

### Adding a Configuration Option
1. Update struct in `pkg/config/config.go`
2. Update default config generation in `cmd/server/commands.go`

### Debugging
```bash
MUXI_LOG_LEVEL=debug go run ./cmd/server start
tail -f ~/.muxi/server/logs/audit.log
cat ~/.muxi/server/registry.json | jq
```

---

## Git Workflow

- `develop` - Active development
- `rc` - Release candidate (cross-platform build & test)
- `main` - Production releases (auto-tagged, auto-released)

### Commit Style
```
feat: add formation health monitoring
fix: port allocation race condition
chore: update dependencies
docs: improve auth documentation
```

### Before Committing
```bash
cd src
go fmt ./...
go vet ./...
go test ./... -v
go build ./cmd/server
```

---

## Key Files

| File | Purpose |
|------|---------|
| `src/cmd/server/main.go` | Entry point, startup |
| `src/cmd/server/commands.go` | CLI commands (init, start, version) |
| `src/pkg/api/server.go` | HTTP server, route registration |
| `src/pkg/process/manager.go` | Process lifecycle orchestration |
| `src/pkg/registry/registry.go` | Formation registry (thread-safe) |
| `src/pkg/proxy/proxy.go` | HTTP reverse proxy |
| `src/pkg/auth/middleware.go` | HMAC auth middleware |
| `src/pkg/telemetry/telemetry.go` | Telemetry global instance |
| `test/dummy_app.py` | Test fixture (FastAPI app) |
