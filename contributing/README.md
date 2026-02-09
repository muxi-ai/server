# Contributing to MUXI Server

Thank you for your interest in contributing to MUXI Server.


## What is MUXI Server?

MUXI Server is a single-binary orchestration platform that makes deploying and managing MUXI formations simple and production-ready. Think of it as **"Docker + PM2 + Nginx"** specifically built for MUXI formations.

### Key Features

- 🚀 **One-Command Deploy** - `muxi formation deploy` → production API
- 📦 **Single Binary** - No dependencies, just install and run
- ⚡ **Zero-Downtime Deployments** ✨ NEW - Blue-green deployment with automatic health checks
- 🔄 **Auto-Recovery** - Formations restart automatically on crash
- 🔐 **Secure** - HMAC-based authentication (AWS-style)
- 🌐 **HTTP Proxy** - Smart routing with formation-level isolation
- 📊 **Process Management** - Monitor, restart, and manage formations

### Architecture

```
┌───────────────────────────────────────┐
│ MUXI Server (Port 7890)               │
│                                       │
│ ┌───────────────────────────────────┐ │
│ │ Management API (Protected)        │ │
│ │  POST /rpc/rpc/formations/deploy  │ │
│ │  GET  /rpc/formations             │ │
│ │  DELETE /rpc/rpc/formations/{id}  │ │
│ └───────────────────────────────────┘ │
│                                       │
│ ┌───────────────────────────────────┐ │
│ │ Proxy API (Transparent)           │ │
│ │  /{formation_id}/*                │ │
│ └───────────────────────────────────┘ │
└───────────────────────────────────────┘
               │
               ↓
┌───────────────────────────────────────┐
│ Formations (Ports 8000-9000)          │
│   • formation-1 (Port 8001)           │
│   • formation-2 (Port 8002)           │
│   • formation-3 (Port 8003)           │
└───────────────────────────────────────┘
```

## Installation

### macOS (Homebrew)

```bash
brew install muxi-ai/tap/muxi
```

### macOS / Linux

```bash
curl -fsSL https://muxi.org/install | bash
```

### Linux (Production)

```bash
curl -fsSL https://muxi.org/install | sudo bash
```

### Windows

```powershell
irm https://muxi.org/install | iex
```

See the [install repo](https://github.com/muxi-ai/install) for full options (`--cli-only`, `--non-interactive`, `--dry-run`).

## Development Setup

```bash
# Clone the repo
git clone https://github.com/muxi-ai/server.git
cd server/src

# Install dependencies
go mod download

# Build
go build ./cmd/server

# Run tests
go test ./... -v -race

# Format & vet
go fmt ./...
go vet ./...
```

## Project Structure

```
src/
├── cmd/server/         # Entry point (main.go)
└── pkg/
    ├── api/            # HTTP API endpoints & middleware
    ├── auth/           # HMAC authentication
    ├── config/         # Configuration management
    ├── formation/      # Formation bundle handling
    ├── process/        # Process lifecycle & auto-restart
    ├── proxy/          # HTTP reverse proxy
    ├── registry/       # Formation registry & port allocation
    ├── runtime/        # Singularity/Docker runtime
    └── telemetry/      # Anonymous usage telemetry
```

## Contributing Docs

| Document | Description |
|----------|-------------|
| [auth.md](auth.md) | HMAC authentication design |
| [cli-protocol.md](cli-protocol.md) | CLI-Server communication protocol |
| [draft-api.md](draft-api.md) | Draft File API for Console integration |
| [how-formations-run.md](how-formations-run.md) | Formation runtime execution guide |
| [runtime-architecture.md](runtime-architecture.md) | SIF/Docker runtime architecture |
| [runtime-auto-download.md](runtime-auto-download.md) | Auto-download of runtime components |
| [sdk-update-notifications.md](sdk-update-notifications.md) | SDK version update notifications |
| [windows-dev.md](windows-dev.md) | Windows development guide |

## Conventions

- **Go style:** `gofmt`, `golint`, standard Go conventions
- **Logging:** [zerolog](https://github.com/rs/zerolog) (structured, zero-alloc)
- **Error handling:** Always wrap with context (`fmt.Errorf("...: %w", err)`)
- **Tests:** Table-driven, `*_test.go` alongside implementation, target >80% coverage

## Branch Workflow

- `develop` - Active development
- `rc` - Release candidate (cross-platform build & test)
- `main` - Production releases (auto-tagged via CI)

## License

[Elastic License 2.0](../LICENSE-ELv2)
