# Changelog

All notable changes to MUXI Server will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### API Architecture Refactor (2025-10-19)

This is a **major breaking change** that introduces a production-ready RESTful API architecture.

#### Added

**Formation Versioning:**
- Formation update system with automatic backup to `previous/` directory
- `PUT /rpc/formations/{id}` - Update formation with new bundle
- `POST /rpc/formations/{id}/rollback` - Rollback to previous version
- `version.json` tracking with SHA256 bundle hashing
- `KeepBackups` configuration option (default: 1)

**Server Management:**
- `GET /rpc/server/status` - Server statistics, uptime, formation counts, port pool status
- `GET /rpc/server/logs` - Retrieve audit logs with configurable line limit
- `GET /ping` - Simple ping endpoint for health monitoring

**Security & Audit:**
- Audit logging middleware for all `/rpc/*` requests
- JSON-formatted audit logs with method, path, status, duration
- Formation localhost binding (bind to `127.0.0.1` only)
- `HOST` environment variable injected into formations
- Reserved formation IDs: `health`, `ping`, `rpc`, `server`, `admin`, `metrics`, `api`

**Configuration:**
- `BindHost: "127.0.0.1"` - Localhost-only formation binding
- `LoggingConfig` struct with `AuditLog` path
- Default audit log: `logs/audit.log`

#### Changed

**BREAKING: API Routes Restructured**
- **Port:** `3000` → `7890` (official "MUXI Port")
- **Management API:** `/formations/*` → `/rpc/formations/*`
- **Proxy:** `/v1/{formation_id}/*` → `/api/{formation_id}/*`

**Old Routes (v1):**
```
GET    /health
POST   /formations/deploy
GET    /formations
GET    /formations/{id}
POST   /formations/{id}/stop
POST   /formations/{id}/restart
DELETE /formations/{id}
GET    /formations/{id}/logs
GET    /v1/{id}/*              (proxy)
```

**New Routes (v2):**
```
# Public endpoints (no auth)
GET    /health
GET    /ping

# Management API (HMAC auth required)
POST   /rpc/formations/deploy
GET    /rpc/formations
GET    /rpc/formations/{id}
PUT    /rpc/formations/{id}
POST   /rpc/formations/{id}/stop
POST   /rpc/formations/{id}/restart
POST   /rpc/formations/{id}/rollback
DELETE /rpc/formations/{id}
GET    /rpc/formations/{id}/logs
GET    /rpc/server/status
GET    /rpc/server/logs

# Formation proxy (no auth)
GET    /api/{id}/*
POST   /api/{id}/*
PUT    /api/{id}/*
DELETE /api/{id}/*
PATCH  /api/{id}/*
```

**Environment Variables:**
- Formations now receive `HOST=127.0.0.1` environment variable
- Added `_bind_host` and `_port` metadata variables

**Configuration:**
- Default port changed from `3000` to `7890`
- Default server bind remains `0.0.0.0` (accessible externally)
- Formations bind to `127.0.0.1` only (localhost-only, security improvement)

#### Fixed
- Formation ID validation prevents reserved words
- Improved error messages for invalid formation IDs
- Better handling of formation updates and rollbacks

#### Security
- **IMPORTANT:** Formations now bind to `127.0.0.1` only
  - Not directly accessible from external networks
  - Must be accessed via MUXI proxy (`/api/{id}/*`)
  - Prevents direct formation exposure
- Audit logging tracks all management API requests
- Reserved formation IDs prevent namespace conflicts

#### Migration Guide

See [MIGRATION.md](./MIGRATION.md) for detailed upgrade instructions.

**Quick Migration:**
1. Update client code to use port `7890` instead of `3000`
2. Update management API routes: `/formations` → `/rpc/formations`
3. Update proxy routes: `/v1/{id}` → `/api/{id}`
4. Verify formations bind to `127.0.0.1` (check `HOST` env var)
5. Update HMAC signatures to use new routes

#### Testing
- Added 42 new tests across 5 test files
- Test coverage increased from 60.6% to 88.3%
- All integration tests updated for new routes

---

## [1.0.0] - 2025-10-17

### Phase 1 Complete - Production Ready

Initial production release with comprehensive features.

#### Added

**Core Features:**
- Process management with auto-restart
- Formation registry with port allocation (8000-9000)
- HTTP API with 8 endpoints
- HTTP proxy routing
- HMAC authentication (AWS-style)
- Formation bundle upload (gzip tarball)
- Server ID generation
- Metadata injection into formations

**Configuration:**
- YAML-based configuration (`~/.muxi/server/config.yaml`)
- Environment-based overrides
- Hot-reload support

**Documentation:**
- 8 comprehensive user guides
- API reference documentation
- Authentication guide
- Troubleshooting guide

**Testing:**
- 88.9% average test coverage
- 200+ unit tests
- 20+ integration tests
- Security tests for authentication

#### Technical Details

**Dependencies:**
- `gorilla/mux` - HTTP routing
- `zerolog` - Structured logging
- `yaml.v3` - YAML parsing

**Architecture:**
- Single Go binary (~10MB)
- In-memory formation registry with persistence
- Port pool allocation system
- Process spawning and monitoring (adapted from pm2-go)

---

## Development Roadmap

### Completed
- ✅ Phase 1: Core server functionality
- ✅ API Architecture Refactor: RESTful routes + versioning

### Planned
- 🔜 Phase 2: Client CLI tool (`muxi` command)
- 🔜 Phase 3: Singularity/Apptainer SIF runtime
- 🔜 Phase 4: Installation scripts & package managers
- 🔜 Phase 5: Multi-server orchestration

---

## Support

For issues, questions, or contributions, see:
- **Documentation:** [docs/README.md](./docs/README.md)
- **API Reference:** [docs/api-reference.md](./docs/api-reference.md)
- **Troubleshooting:** [docs/troubleshooting.md](./docs/troubleshooting.md)

---

**Note:** This project is under active development. Breaking changes may occur before v1.0.0 stable release.
