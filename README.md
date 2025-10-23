# MUXI Server

Production-grade orchestration platform for deploying and managing MUXI formations at scale.

**Think:** Docker + PM2 + Nginx in a single Go binary, purpose-built for MUXI formations.

## Versioning

**Current Version:** `0.20251023.0`

MUXI Server uses **[ScalVer (Scalable Calendar Versioning)](https://scalver.org)** - a calendar-aware versioning scheme that's fully compatible with SemVer.

**Format:** `MAJOR.YYYYMMDD.PATCH`
- `0` - Alpha/experimental (current)
- `20251023` - Release date (October 23, 2025)
- `0` - First release of the day

**Learn more:** [docs/VERSIONING.md](docs/VERSIONING.md)

## Features

- 🚀 **One-Command Deploy** - Bundle upload with automatic metadata injection
- 🔐 **HMAC Authentication** - AWS-style key/secret authentication
- 🎯 **HTTP Proxy** - Automatic routing to formations (`/api/{formation_id}/*`)
- 🔄 **Formation Versioning** - Update formations with rollback support
- 📝 **Audit Logging** - Track all API requests to formations
- 📊 **Server Telemetry** - Automatic `_server_id` and `_deployment_mode` injection
- 🔄 **Auto-Restart** - Crashed formations automatically restart
- 📝 **Complete API** - 14 endpoints for formations, versioning, and server management
- 🎨 **Simple CLI** - `muxi-server init`, `version`, `config show`

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
└── wip/                 # Work in progress docs & notes
    ├── COVERAGE-ACHIEVEMENT.md  # Coverage analysis
    ├── PHASE-1-FINAL-SUMMARY.md # Phase 1 recap
    └── ...

Test Coverage: 88.3% average (11,500+ lines across 30 test files)
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
- `auth`: 97.3% ⭐
- `registry`: 91.7% ⭐
- `process`: 90.3% ⭐
- `formation`: 89.2% ✅
- `config`: 88.9% ✅
- `proxy`: 88.5% ✅
- `api`: 72.5% ✅

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

- ✅ **Phase 1 Complete** - Production-ready server
  - Process management with auto-restart
  - Formation registry with port allocation
  - RESTful API (14 endpoints)
  - Formation versioning & rollback
  - HTTP proxy routing
  - HMAC authentication
  - Audit logging
  - Server CLI commands
  - Formation bundle upload
  - Server ID generation & metadata injection
  - Localhost-only formation binding (security)
  
- 🔜 **Phase 2** - Client CLI tool (separate project)
- 🔜 **Phase 3** - Singularity/Apptainer SIF runtime

## Documentation

### For Contributors
- **[AGENTS.md](AGENTS.md)** - Development guide for AI agents
- **[docs/VERSIONING.md](docs/VERSIONING.md)** - Versioning & release process
- **CHANGELOG.md** - Version history

### For Developers
- **PRD.md** - Product Requirements Document
- **AUTH.md** - Authentication design
- **CLI-PROTOCOL.md** - Client-Server protocol specification

### Implementation Summaries
- **MANAGEMENT-API-COMPLETE.md** - Management API endpoints (5 endpoints)
- **SERVER-CLI-COMPLETE.md** - Server CLI commands
- **BUNDLE-UPLOAD-COMPLETE.md** - Formation bundle upload & server ID

### For Users
- **docs/** - User documentation (8 files)
  - Getting started, installation, configuration
  - Authentication, formations, API reference
  - Troubleshooting

### Testing
- **TESTING.md** - Testing guide
- **src/test/test_*.sh** - Test scripts for API, proxy, auth

## License

See LICENSE file in repository root.
