# MUXI Server

Production-grade orchestration platform for deploying and managing MUXI formations at scale.

**Think:** Docker + PM2 + Nginx in a single Go binary, purpose-built for MUXI formations.

## Features

- 🚀 **One-Command Deploy** - Bundle upload with automatic metadata injection
- 🔐 **HMAC Authentication** - AWS-style key/secret authentication
- 🎯 **HTTP Proxy** - Automatic routing to formations (`/v1/{formation_id}/*`)
- 📊 **Server Telemetry** - Automatic `_server_id` and `_deployment_mode` injection
- 🔄 **Auto-Restart** - Crashed formations automatically restart
- 📝 **Complete API** - Full CRUD operations on formations
- 🎨 **Simple CLI** - `muxi-server init`, `version`, `config show`

## Repository Structure

```
.
├── AGENTS.md            # AI agent development guide
├── PRD.md               # Product Requirements Document
├── LICENSE              # MIT License
├── README.md            # This file
│
├── pm2-go/              # Reference implementation (DO NOT MODIFY)
│   └── (pm2-go codebase for reference)
│
└── src/                 # MUXI Server implementation
    ├── cmd/
    │   └── server/      # Main entry point
    ├── pkg/
    │   ├── process/     # Process lifecycle management
    │   ├── registry/    # Formation tracking & port allocation
    │   ├── api/         # HTTP API endpoints
    │   ├── proxy/       # HTTP reverse proxy (future)
    │   └── config/      # Configuration management
    ├── test/
    │   ├── dummy_app.py # Test FastAPI server
    │   └── fixtures/    # Test data
    ├── go.mod
    └── go.sum
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
curl -X POST http://localhost:3000/formations/deploy \
  -H "Authorization: MUXI-HMAC key=$KEY, timestamp=$TIMESTAMP, signature=$SIGNATURE" \
  -H "Content-Type: application/gzip" \
  --data-binary "@formation.tar.gz"

# Access via proxy
curl http://localhost:3000/v1/my-formation/health
```

### Test

```bash
cd src

# Run all tests
go test ./...

# With coverage
go test ./... -cover

# Verbose
go test ./... -v
```

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

- ✅ **Phase 1 Complete** - Core server functionality
  - Process management with auto-restart
  - Formation registry with port allocation
  - HTTP API (8 endpoints)
  - HTTP proxy routing
  - HMAC authentication
  - Server CLI commands
  - Formation bundle upload
  - Server ID generation & metadata injection
  
- 🔜 **Phase 2** - Client CLI tool (separate project)
- 🔜 **Phase 3** - Singularity/Apptainer SIF runtime

## Documentation

### For Developers
- **AGENTS.md** - Development guide for AI agents
- **PRD.md** - Product Requirements Document
- **AUTH.md** - Authentication design
- **CLI-PROTOCOL.md** - Client-Server protocol specification
- **STATUS.md** - Current implementation status

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
