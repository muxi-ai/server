# Changelog

## 0.20260305.0

### MCP Proxy

- **`/mcp/{formation_id}/*`** - New proxy route for MCP protocol requests
- Maps `/mcp/{id}` to the formation's `/mcp` endpoint (preserves `/mcp` prefix)
- Supports SSE transport for MCP client connections (Claude Desktop, Cursor, etc.)

### Host Tools for SIF Containers

- Bind-mount host tools (Node.js, git, curl, ffmpeg, etc.) into SIF at `/opt/muxi-tools`
- Runtime-runner (macOS/Windows): pre-staged tools directory with shared libs via `ldd`
- Native Linux: direct bind-mount of host binaries and library directories
- Sets `PATH`, `NODE_PATH`, `FONTCONFIG_PATH`, `SSL_CERT_FILE` inside SIF

### Fixes

- **Dev logs**: truncate log files on each `muxi up` run (prevents stale error accumulation)
- **Graceful shutdown**: fix panic on Ctrl+C when monitor channel already closed (`sync.Once`)

---

## 0.20260209.0

### Local Development Mode (`muxi up`)

New API for running formations in development mode without full deploy cycle:

- **POST /rpc/dev/run** - Start draft formation from local path or draft directory
- **POST /rpc/dev/stop** - Stop draft formation
- **/draft/{id}/*** - Proxy route for draft formations (separate from live `/api/{id}/*`)

Key features:
- Live and draft formations can run simultaneously with same ID
- Draft formations use separate registry (not persisted, not restored on restart)
- Shares port pool with live formations
- Enables `muxi up` CLI command for fast local iteration

### SDK Update Notifications

Server now notifies SDKs when updates are available:

- Fetches latest release versions from GitHub API on startup (refreshes every 24h)
- Parses `X-Muxi-SDK: {name}/{version}` header from SDK requests
- Responds with `X-Muxi-SDK-Latest: {version}` header
- If no release data available, echoes back SDK's version (no false notifications)

Supported SDKs: go, python, typescript, ruby, php, csharp, swift, kotlin, dart, java, rust, cpp

---

## 0.20260203.1

### Download Endpoint

- **GET /rpc/formations/{id}/download** - Download formation as zip file
- Excludes hidden files (except `.env`) and `memory.db` by default
- Use `?db=true` query param to include persistent memory database

### Draft File API

New API for Console's visual formation editor (Studio):

- **POST /rpc/formations/{id}/draft/files** - Single endpoint with action-based routing
- **init** - Create new draft or clone from live version
- **list** - List files in draft directory
- **read** - Read file content (utf-8 or base64)
- **write** - Write file content (utf-8 or base64)
- **delete** - Delete file or directory
- **deploy** - Deploy draft to live (new or blue-green update)
- **discard** - Remove draft without affecting live

Reuses existing deployment logic - same validation, health checks, and blue-green deployment.

### Memory Persistence

Runtime now creates `memory.db` for persistent agent memory. Server handles this automatically:

- **Update** - Preserves `memory.db` from current version if not in upload
- **Rollback** - Copies `memory.db` from current to previous before swap (roll back code, not data)
- **Download** - Excludes `memory.db` by default for lightweight downloads

### Internal

- Updated GitHub Actions to latest versions (checkout v6, setup-go v6, upload-artifact v6)
- Updated CI workflows to Go 1.24 to match go.mod

---

## 0.20260127.0 - Initial Public Release

The orchestration platform for MUXI formations. Deploy, route, monitor, and auto-restart AI agent formations with a single Go binary.

### Core Platform

- **Single binary orchestration** - Deploy and manage formations without external dependencies
- **14 RESTful API endpoints** - Full CRUD for formation lifecycle management
- **HMAC authentication** - AWS Signature v4 style request signing with replay protection
- **HTTP reverse proxy** - Intelligent routing to formations via `/api/{id}/*`
- **Port pool allocation** - Thread-safe port management (8000-9000 range)
- **Audit logging** - JSON-formatted logs for all management operations
- **Health monitoring** - Continuous health checks with automatic recovery

### Formation Management

- **One-command deploy** - Upload gzip tarball bundles to deploy formations
- **Blue-green deployments** - Zero-downtime updates with staging port validation
- **Formation versioning** - Current/previous directory structure with SHA256 tracking
- **Rollback support** - Instant rollback to previous version with blue-green safety
- **Auto-restart** - Crashed formations restart automatically with configurable limits
- **Secrets validation** - Checks for `secrets.enc` and `.key` when secrets referenced

### Runtime Support

- **Singularity/Apptainer SIF** - Container-based formation execution (Linux native)
- **Docker-wrapped Singularity** - Runtime support for macOS and Windows
- **Auto-download** - SIF images and runtime-runner containers downloaded on demand
- **Runtime version resolution** - Automatic version matching from formation.yaml

### SSE Streaming

- **Deploy progress** - Real-time deployment stages via Server-Sent Events
- **Update progress** - Blue-green update stages with staging port info
- **Rollback progress** - Live rollback status with version tracking
- **Restart progress** - Restart status streaming
- **Log streaming** - `follow=true` parameter for live log tailing

### Server Initialization

- **CLI profile auto-configuration** - Creates `~/.muxi/cli/profiles.yaml` on init
- **System service setup** - Interactive systemd (Linux) and launchd (macOS) installation
- **Gradient banner** - Branded welcome message with version and architecture info

### Telemetry

- **Privacy-first metrics** - No PII, no content, no formation data
- **Hourly reporting** - Batched metrics with 5-second retry backoff
- **Opt-out support** - Disable via `MUXI_TELEMETRY=0` or `~/.muxi/config.yaml`
- **Tracked metrics** - Deployments, updates, rollbacks, crashes, restarts, request latency
- **Machine ID** - Platform-specific deterministic ID generation
- **Country lookup** - Cached geo lookup via ipapi.co

### Proxy Headers

- **X-Muxi-Server** - Server version injected into proxied requests
- **X-Muxi-SDK pass-through** - Client SDK headers forwarded to formations
- **Server-owned protection** - Clients cannot spoof server-injected headers
- **X-Forwarded-\*** - Standard proxy headers (For, Proto, Host)

### Cross-Platform Support

- **Linux** - Native support with systemd service (amd64, arm64)
- **macOS** - Native support with launchd service (Intel, Apple Silicon)
- **Windows** - Development support with Job Objects process management
- **Docker** - Multi-arch images (linux/amd64, linux/arm64)

### Distribution

- **Homebrew** - `brew install muxi-ai/tap/muxi-server`
- **Install script** - `curl -sSL https://muxi.org/install | bash`
- **PowerShell** - `irm https://muxi.org/install/windows.ps1 | iex`
- **Direct download** - GitHub release binaries for all platforms
- **Docker** - `ghcr.io/muxi-ai/muxi-server:latest`

### Configuration

- **YAML-based** - `~/.muxi/server/config.yaml`
- **Port 7890** - Official MUXI Server port
- **Formation isolation** - Formations bind to `127.0.0.1` only
- **Configurable log level** - `--log-level` flag and `MUXI_LOG_LEVEL` env var
- **Auto-restart settings** - Max restarts, restart delay, health check intervals

---

## Release Format

MUXI Server uses **[ScalVer](https://scalver.org)** (Scalable Calendar Versioning):

**Format:** `MAJOR.YYYYMMDD.PATCH`

---

## Support

- **Documentation:** [muxi.org/docs](https://muxi.org/docs)
- **GitHub Issues:** [github.com/muxi-ai/server/issues](https://github.com/muxi-ai/server/issues)





