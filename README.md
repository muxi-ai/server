# MUXI Server

[![Release](https://img.shields.io/github/v/release/muxi-ai/server?label=version)](https://github.com/muxi-ai/server/releases)
[![CI](https://img.shields.io/github/actions/workflow/status/muxi-ai/server/ci.yml?branch=develop&label=CI)](https://github.com/muxi-ai/server/actions)
[![Docker](https://img.shields.io/badge/docker-ghcr.io-blue)](https://github.com/muxi-ai/server/pkgs/container/server)
[![Coverage](https://img.shields.io/badge/coverage-91.2%25-brightgreen)](https://github.com/muxi-ai/server/actions)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

Production-grade orchestration platform for deploying and managing MUXI formations at scale.

**Think:** Docker + PM2 + Nginx in a single Go binary, purpose-built for MUXI formations.

## Versioning

**Current Version:** `v0.20251023.1` ([Release Notes](https://github.com/muxi-ai/server/releases/latest))

MUXI Server uses **[ScalVer (Scalable Calendar Versioning)](https://scalver.org)** - a calendar-aware versioning scheme that's fully compatible with SemVer.

**Format:** `MAJOR.YYYYMMDD.PATCH`
- `0` - Alpha/experimental (current)
- `20251023` - Release date (October 23, 2025)
- `1` - Second release of the day

**Learn more:** [docs/VERSIONING.md](docs/VERSIONING.md)

## Quick Install

```bash
# One-command install (Linux/macOS)
curl -sSL https://install.muxi.ai | sudo bash

# Or download binary directly
wget https://github.com/muxi-ai/server/releases/latest/download/muxi-server-linux-amd64
chmod +x muxi-server-linux-amd64
sudo mv muxi-server-linux-amd64 /usr/local/bin/muxi-server

# Or use Docker
docker pull ghcr.io/muxi-ai/server:latest
docker run -p 7890:7890 ghcr.io/muxi-ai/server:latest
```

**Windows (PowerShell):**
```powershell
# One-command install
irm https://install.muxi.ai/windows.ps1 | iex
```

**Supported Platforms:**
- Linux (amd64, arm64)
- macOS (amd64, arm64 - Apple Silicon)
- Windows (amd64, arm64) ✨ New!
- Docker (multi-arch: linux/amd64, linux/arm64)

## Features

- 🚀 **One-Command Deploy** - Bundle upload with automatic metadata injection
- 🔐 **HMAC Authentication** - AWS-style key/secret authentication  
- 🎯 **HTTP Proxy** - Automatic routing to formations (`/api/{formation_id}/*`)
- 🔄 **Formation Versioning** - Update formations with rollback support
- 📝 **Audit Logging** - Track all API requests to formations
- 📊 **Server Telemetry** - Automatic `_server_id` and `_deployment_mode` injection
- 🔄 **Auto-Restart** - Crashed formations automatically restart
- 📝 **Complete API** - 14 RESTful endpoints for formations, versioning, and server management
- 🎨 **Simple CLI** - `muxi-server init`, `version`, `config show`
- 🐳 **Docker Support** - Multi-arch images on GitHub Container Registry
- 🔧 **Zero Dependencies** - Single binary, no runtime requirements

## Repository Structure

```
.
├── AGENTS.md            # AI agent development guide
├── PRD.md               # Product Requirements Document
├── LICENSE              # MIT License
├── README.md            # This file
├── docs/                # User documentation (8 guides)
│   ├── api-reference.md
│   ├── authentication.md
│   ├── getting-started.md
│   └── ...
├── src/                 # MUXI Server implementation
│   ├── cmd/
│   │   └── server/      # Main entry point & CLI
│   ├── pkg/             # Core packages (88.9% test coverage!)
│   │   ├── api/         # HTTP API endpoints (77.2% coverage)
│   │   │   ├── *.go     # API handlers & middleware
│   │   │   └── *_test.go # 6 test files, ~2,500 lines
│   │   ├── auth/        # HMAC authentication (97.3% coverage) ⭐
│   │   │   ├── *.go     # Auth logic & middleware
│   │   │   └── *_test.go # 2 test files, ~900 lines
│   │   ├── config/      # Configuration (88.9% coverage)
│   │   │   ├── *.go     # Config management
│   │   │   └── *_test.go # 1 test file, ~650 lines
│   │   ├── formation/   # Formation handling (88.6% coverage)
│   │   │   ├── *.go     # Bundle extraction & parsing
│   │   │   └── *_test.go # 3 test files, ~1,000 lines
│   │   ├── process/     # Process management (90.3% coverage) ⭐
│   │   │   ├── *.go     # Spawn, monitor, restart
│   │   │   └── *_test.go # 7 test files, ~3,000 lines
│   │   ├── proxy/       # HTTP proxy (88.5% coverage)
│   │   │   ├── *.go     # Request routing
│   │   │   └── *_test.go # 1 test file, ~800 lines
│   │   └── registry/    # Formation registry (91.3% coverage) ⭐
│   │       ├── *.go     # Registry & persistence
│   │       └── *_test.go # 3 test files, ~1,100 lines
│   ├── go.mod
│   └── go.sum
├── test/                # Integration test scripts
│   ├── dummy_app.py     # Test FastAPI server
│   ├── formations/      # Test bundles
│   ├── *.sh             # Integration test scripts
│   └── fixtures/        # Test data
└── notes/               # Implementation notes & design docs
    ├── PRD.md           # Product Requirements Document
    ├── AUTH.md          # Authentication design
    ├── VERSIONING.md    # Versioning strategy
    └── ...

Test Coverage: 91.2% average (11,500+ lines across 30 test files)
- Unit tests: 200+
- Integration tests: 20+
- Security tests: 15+
- Zero flaky tests ✅
```

## Quick Start

### Build

```bash
cd src
go build -o ../muxi-server ./cmd/server
```

### Initialize

```bash
# Generate credentials and config
./muxi-server init

# View configuration
./muxi-server config show
```

### Run

```bash
# Start server (default command)
./muxi-server

# Or explicitly:
./muxi-server start
```

### Deploy a Formation

```bash
# Create a formation bundle (tarball with formation.yaml)
cd my-formation
tar -czf formation.tar.gz .

# Deploy it
curl -X POST http://localhost:7890/rpc/formations/deploy \
  -H "Authorization: MUXI-HMAC key=$KEY, timestamp=$TIMESTAMP, signature=$SIGNATURE" \
  -H "Content-Type: application/gzip" \
  --data-binary "@formation.tar.gz"

# Access via proxy
curl http://localhost:7890/api/my-formation/health
```

### Test

**Coverage: 88.3% average** (11,500+ lines of test code across 30 test files)

```bash
cd src

# Run all tests
go test ./...

# With coverage report
go test ./... -cover

# Coverage by package
go test ./... -cover | grep coverage

# Generate HTML coverage report
go test ./... -coverprofile=coverage.out
go tool cover -html=coverage.out

# Verbose output
go test ./... -v
```

**Coverage Breakdown:**
- `registry`: 91.2% ⭐
- `process`: 90.3% ⭐
- `formation`: 88.6% ✅
- `config`: 88.9% ✅
- `proxy`: 88.5% ✅
- `auth`: 100.0% ⭐
- `api`: 77.2% ✅

**All tests pass with race detector enabled** (`-race` flag)

### Development

```bash
cd src

# Format code
go fmt ./...

# Vet code
go vet ./...

# Run directly
go run ./cmd/server
```

## Dependencies

- **gorilla/mux** - HTTP routing
- **zerolog** - Structured logging
- **yaml.v3** - YAML parsing

## Testing the Dummy App

```bash
cd src

# Install FastAPI (if not already installed)
pip install fastapi uvicorn

# Run dummy app
python test/dummy_app.py --port 8001

# Test health endpoint
curl http://localhost:8001/health

# Test chat endpoint
curl -X POST http://localhost:8001/chat \
  -H "Content-Type: application/json" \
  -d '{"message": "Hello!", "user_id": "test"}'
```

## Current Status

**✅ Production Ready** - Version v0.20251023.1

### Completed Features
- ✅ **Core Server** - Process management, registry, port allocation
- ✅ **RESTful API** - 14 endpoints (formations, versioning, server management)
- ✅ **Formation Versioning** - Update & rollback support
- ✅ **HTTP Proxy** - Automatic routing with telemetry injection
- ✅ **HMAC Authentication** - AWS-style key/secret auth
- ✅ **Audit Logging** - JSON-lines format for all API requests
- ✅ **CLI Commands** - init, version, config show
- ✅ **Bundle Upload** - Tarball deployment with metadata injection
- ✅ **Security** - Localhost-only formation binding
- ✅ **Auto-Restart** - Crashed formations auto-recover
- ✅ **Test Coverage** - 91.2% with race detector enabled
- ✅ **CI/CD Pipeline** - develop → rc → main with auto-versioning
- ✅ **Multi-Platform** - Linux, macOS, Windows (amd64/arm64)
- ✅ **Docker Images** - Multi-arch on GHCR

### Roadmap
- 🔜 **Phase 2** - Client CLI tool (separate repository)
- 🔜 **Phase 3** - Singularity/Apptainer SIF runtime
- ✅ **Windows Support (Phase 1)** - Binary compilation & dev experience complete!

## Documentation

### User Documentation
- **[docs/getting-started.md](docs/getting-started.md)** - Quick start guide
- **[docs/installation.md](docs/installation.md)** - Installation & setup
- **[docs/api-reference.md](docs/api-reference.md)** - Complete API reference
- **[docs/authentication.md](docs/authentication.md)** - HMAC authentication guide
- **[docs/formations.md](docs/formations.md)** - Formation management
- **[docs/configuration.md](docs/configuration.md)** - Server configuration
- **[docs/docker-quick-start.md](docs/docker-quick-start.md)** - Docker deployment
- **[docs/windows-dev.md](docs/windows-dev.md)** - Windows development guide ✨ New!
- **[docs/troubleshooting.md](docs/troubleshooting.md)** - Common issues & solutions

### Developer Documentation
- **[AGENTS.md](AGENTS.md)** - AI agent development guide
- **[docs/VERSIONING.md](docs/VERSIONING.md)** - ScalVer versioning & release process
- **[CHANGELOG.md](CHANGELOG.md)** - Version history
- **[notes/PRD.md](notes/PRD.md)** - Product Requirements Document
- **[notes/AUTH.md](notes/AUTH.md)** - Authentication design
- **[notes/SERVICE-DAEMON-DESIGN.md](notes/SERVICE-DAEMON-DESIGN.md)** - Service management design

### CI/CD & Release
- **[.github/workflows/](/.github/workflows/)** - GitHub Actions workflows
  - `ci.yml` - Continuous Integration (develop branch)
  - `rc.yml` - Release Candidate builds (rc branch)
  - `release.yml` - Production releases (main branch)
  - `docker-build-publish.yml` - Docker image builds
- **[notes/BRANCH-SETUP-GUIDE.md](notes/BRANCH-SETUP-GUIDE.md)** - Branch workflow setup

## License

See LICENSE file in repository root.
