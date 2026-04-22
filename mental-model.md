# MUXI Server — Mental Model

> **Last updated:** 2026-04-22
> **Repo:** `/Users/ran/Projects/muxi/code/server`
> **Language:** Go 1.26 | **License:** Elastic License 2.0

---

## 1. High-Level Architecture

MUXI Server is a **single-binary orchestration platform** for deploying and managing AI agent formations. It combines:

- **HTTP reverse proxy** — routes `/api/{id}/*` and `/draft/{id}/*` to formation ports
- **Process manager** — spawns, monitors, auto-restarts formation processes
- **Port allocator** — pool of 8000–9000, auto-assigned per formation
- **HMAC authentication** — for `/rpc/*` management API
- **Runtime resolver** — downloads SIF images, manages Singularity/Docker execution
- **Skills RCE** — sidecar code-execution service for formations

```
┌──────────────────────────────────────────────────────────────┐
│ MUXI Server (Port 7890)                                      │
│                                                              │
│  Public: /health, /ping, /docs                               │
│  Management API: /rpc/* (HMAC-authenticated)                 │
│  Proxy: /api/{id}/*, /draft/{id}/*, /mcp/{id}/*             │
└────────────────────────┬─────────────────────────────────────┘
                         │ spawns & proxies
           ┌─────────────┼────────────────┐
           ▼             ▼                ▼
      Formation 1   Formation 2     Skills RCE
      :8001          :8002           :7891
```

### Key Design Decisions
- All formations bind to **127.0.0.1** (localhost-only); traffic flows through the proxy
- On **macOS/Windows**, formations bind to **0.0.0.0** (Docker network namespaces require it)
- **Versioning**: `formations/{id}/current/` and `formations/{id}/previous/` with `version.json`
- **Zero-downtime update**: blue-green deployment — staging on new port, health check, atomic port switch
- **Draft/dev mode**: separate registry (`draftFormations`), same ID can have live + draft simultaneously

---

## 2. Package Map

### `cmd/server/` — Entry Point
| File | Purpose |
|------|---------|
| `main.go` | CLI dispatch (`init`, `start`, `version`, `config show`, `upgrade`, `help`), startup orchestration |
| `commands.go` | `cmdInit()` interactive setup, `cmdUpgrade()`, credential generation, service setup (systemd/launchd), CLI profile management |
| `.version` | Embedded ScalVer version string |

**Startup sequence** (`cmdStart`):
1. Parse log level (flag > env > default)
2. Load config → ensure server_id
3. Init telemetry
4. Create ProcessManager, Registry, Persistence (load + auto-save)
5. Create AuthMiddleware, API Server
6. Start Skills RCE (if configured) → wait for healthy
7. **RestoreFormations** — re-spawn previously running formations
8. Start HTTP server, telemetry sender, SDK version refresh
9. Wait for SIGTERM/SIGINT → graceful shutdown (stop processes, flush telemetry, save registry)

### `pkg/api/` — HTTP API & Handlers
| File | Purpose |
|------|---------|
| `server.go` | Route registration, middleware chain (logging → CORS → auth → audit), HTTP server lifecycle |
| `deploy.go` | `POST /rpc/formations` — new formation deploy (bundle upload → extract → validate → runtime resolve → spawn → health check) |
| `update.go` | `PUT /rpc/formations/{id}` — zero-downtime blue-green update |
| `restore.go` | Server startup: re-spawn all registered non-stopped formations |
| `dev.go` | `POST /rpc/dev/run` and `/dev/stop` — draft mode for `muxi up` / Console |
| `rollback.go` | `POST /rpc/formations/{id}/rollback` — swap current↔previous |
| `start.go` | `POST /rpc/formations/{id}/start` — start a stopped formation |
| `restart.go` | `POST /rpc/formations/{id}/restart` — restart running formation |
| `stop.go` | `POST /rpc/formations/{id}/stop` — graceful stop |
| `delete.go` | `DELETE /rpc/formations/{id}` — stop + unregister + cleanup |
| `get.go` / `list.go` | Read endpoints |
| `logs.go` | `GET /rpc/formations/{id}/logs` — tail stdout/stderr logs |
| `download.go` | `GET /rpc/formations/{id}/download` — download formation bundle |
| `draft.go` | `POST /rpc/formations/{id}/draft/files` — upload draft files |
| `cancel_update.go` | Cancel in-progress update |
| `progress.go` | SSE streaming for deploy/update progress |
| `audit.go` | Audit logging middleware for `/rpc/*` |
| `util.go` | `getBindHost()` — platform-aware bind host selection |
| `errors.go` | JSON error response helpers |

**Route structure:**
- Public (no auth): `/health`, `/ping`, `/docs`
- Management `/rpc/*` (HMAC): formations CRUD, server status/logs, dev run/stop
- Proxy (no auth): `/api/{id}/*`, `/draft/{id}/*`, `/mcp/{id}/*`

### `pkg/process/` — Process Lifecycle
| File | Purpose |
|------|---------|
| `process.go` | `Process` struct — ID, PID, status, command, runtime type, SIF path. Thread-safe status/restart methods |
| `manager.go` | `Manager` — Start/Stop/ForceKill/Restart/StopAll. Crash handler with auto-restart logic |
| `monitor.go` | `Monitor` — goroutine per process, polls every 5s, detects crashes via `IsProcessRunning(PID)`, initial health check with 150 retries × 2s |
| `spawn_common.go` | **Core spawning logic** — validates config, builds command, handles native/Singularity/Docker modes, host tool binding, log file management |
| `spawn_unix.go` | Unix process group setup (`Setpgid`) |
| `spawn_windows.go` | Windows job object setup |
| `health.go` | `HealthChecker` — configurable timeout/interval, crash detection during health check |

**Process statuses:** `stopped` → `starting` → `running` → `stopping` → `stopped` | `crashed` → `restarting`

**Auto-restart flow:**
1. Monitor detects PID not running + no stop signal → `StatusCrashed`
2. Manager.handleCrash: check `ShouldRestart()` (auto_restart && count < max && !stop_signal)
3. Increment restart count, sleep `RestartDelay`, re-spawn with original config
4. New monitor created for new process

### `pkg/runtime/` — SIF Runtime Management
| File | Purpose |
|------|---------|
| `resolver.go` | Version constraint resolution: `"latest"` → pass-through, `"1.2.3"` → exact pass-through, `"1.2"` → latest `1.2.x` from local registry, `"1"` → latest `1.x.x` |
| `download.go` | `Downloader` — fetch latest version via GitHub redirect (no API, no rate limit), download SIF with progress, ensure runtime-runner Docker image |
| `registry.go` | `Registry` — tracks installed SIF files, formation→runtime mapping, JSON persistence |
| `validator.go` | SIF file validation |

### `pkg/config/` — Configuration
| File | Purpose |
|------|---------|
| `config.go` | Config struct (YAML), platform-aware path detection, env var overrides |

**Path resolution priority:** `MUXI_*_DIR` env var > platform detection (Linux `/etc/muxi`, `/var/lib/muxi`; Windows `%APPDATA%`) > user home (`~/.muxi/server`)

**Key config sections:**
- `ServerConfig`: port (7890), host (0.0.0.0)
- `AuthConfig`: enabled, key, secret, timestamp_tolerance (300s)
- `FormationsConfig`: port range, bind host, auto-restart, max restarts, health check settings, deployment (blue-green) config, log rotation
- `RuntimeConfig`: SIF base URL, runtime-runner Docker image
- `RCEConfig`: port (7891), auth token
- `LoggingConfig`: level, audit log path

### `pkg/registry/` — Formation Registry
| File | Purpose |
|------|---------|
| `registry.go` | Thread-safe registry with `formations` (live, persisted) and `draftFormations` (draft, not persisted). Port pool management, staging port for blue-green deploys |
| `formation.go` | `Formation` struct — ID, port, status, staging port, deploying flag, health, timestamps |
| `persistence.go` | Auto-save with 2s debounce, JSON file at `registry.json` |
| `port_pool.go` | Port allocation from configurable range (default 8000–9000) |
| `validation.go` | Formation ID validation (alphanumeric + hyphens) |

### `pkg/proxy/` — HTTP Reverse Proxy
| File | Purpose |
|------|---------|
| `proxy.go` | `Handler` — routes requests to formation ports. SSE streaming support (chunk-flush). X-Forwarded-* headers. Server-owned headers (X-Muxi-Server). Draft proxy uses `GetDraft()`. MCP proxy preserves `/mcp` prefix. |

### `pkg/auth/` — HMAC Authentication
| File | Purpose |
|------|---------|
| `middleware.go` | Validates `Authorization: MUXI-HMAC-SHA256 Key=..., Timestamp=..., Signature=...` header. Key validation, timestamp tolerance, constant-time signature comparison |

### `pkg/formation/` — Formation Bundle Handling
| File | Purpose |
|------|---------|
| `formation.go` | Parse `formation.afs` / `.yaml` / `.yml`. Secrets validation (`${{ secrets.XXX }}`). Default command: `python app.py`. Env vars: PORT, HOST, FORMATION_ID, MUXI_SERVER_URL |
| `extract.go` | Tar.gz bundle extraction |
| `version.go` | `VersionHistory` — current/previous version tracking with bundle hash |
| `metadata.go` | Inject server_id metadata, generate server IDs |

### `pkg/rce/` — Skills RCE Service
| File | Purpose |
|------|---------|
| `rce.go` | Manages Skills RCE sidecar (code execution for formations). Linux: SIF via Apptainer. macOS/Windows: Docker container. Health check, env var injection (`MUXI_RCE_URL`, `MUXI_RCE_TOKEN`). `EnsureDocker()` pulls the image via `dockerutil.RenderPullProgress` for consistent UX with runtime-runner. |

### `pkg/hfcache/` — Embedding Model Pre-Download
| File | Purpose |
|------|---------|
| `hfcache.go` | Pre-downloads the default lean embedding model (`nomic-ai/nomic-embed-text-v1.5`, ~524 MiB) into `<cacheDir>/<org>--<model>/`. Pure HTTP (no `huggingface_hub` library) so it works identically on Linux/macOS/Windows. Exports `EnsureLeanModel`, `EnsureModel`, `IsModelCached`. Returns `(alreadyCached bool, err error)` so callers can skip any "downloading…" UX when the fast-path applies. |

**Fast-path invariant:** `IsModelCached` checks every expected file in `leanModelFiles` exists with non-zero size. If all present, `EnsureModel` returns `(true, nil)` WITHOUT any HTTP call — critical for re-init / upgrade flows that would otherwise re-fetch 524 MiB on every run.

**File writes:** `downloadFileIfMissing` writes to `<file>.tmp` then atomic-renames to `<file>`. Prevents partial-file poisoning if the process is killed mid-download; a subsequent init sees the `.tmp` orphan, ignores it, and re-fetches cleanly.

**Cache layout** (chosen to be minimal, not full HF hub format):
```
<cacheDir>/
  nomic-ai--nomic-embed-text-v1.5/
    config.json
    tokenizer.json
    onnx/model.onnx         (~270 MiB)
    onnx/model_quantized.onnx
    ... (10 files total)
```

The runtime SIF is expected to bind-mount `<cacheDir>` at `/opt/hf-cache` and set `HF_HOME=/opt/hf-cache` so HuggingFace's own cache resolver finds the files.

### `pkg/dockerutil/` — Shared Docker CLI Output Rendering
| File | Purpose |
|------|---------|
| `progress.go` | `RenderPullProgress(io.Reader, io.Writer)` — collapses Docker's verbose non-TTY pull output (5 events × N layers) into a single in-place progress line with an animated braille spinner. Used by both `cmd/server/commands.go::pullRuntimeRunner` and `pkg/rce/rce.go::EnsureDocker`. Exports `SpinnerFrames` and `SpinnerTick` for callers that paint their own progress lines (e.g. `downloadReporter`). |

**Why a shared package** — previously `renderPullProgress` lived in `cmd/server/commands.go` only, and `pkg/rce/EnsureDocker` shipped raw Docker output. After extraction, both Docker pulls in `init` render identically; any future tweak (ETA, throughput, color) lands in both places from a single edit.

**Design of the renderer:**
- Producer goroutine drains the scanner into a buffered channel
- Consumer `select`s between new events and ticker fires, repainting on either
- Ticker independent of events so the spinner keeps animating during silent layer downloads (the exact moment users wonder "is this hung?")
- All writes to `out` happen from the consumer goroutine — race-free by construction

### `pkg/telemetry/` — Anonymous Usage Telemetry
| File | Purpose |
|------|---------|
| `telemetry.go` | Global collector/sender pattern. Tracks: server starts, deploys, updates, rollbacks, crashes, auto-restarts, API calls, request latency |

### `pkg/updates/` — SDK Version Notifications
| File | Purpose |
|------|---------|
| `sdk_versions.go` | Background refresh of latest SDK versions. Adds `X-Muxi-SDK-Latest` header to proxy responses when `X-Muxi-SDK` is present |

---

## 3. Core Data Flows

### Deploy Flow (New Formation)
```
POST /rpc/formations (gzip bundle)
  │
  ├─ Validate X-Formation-ID header (early conflict detection)
  ├─ Save bundle to temp file
  ├─ Extract tar.gz → temp dir
  ├─ Find formation.afs/yaml/yml → ParseFormation
  ├─ Validate: ID match, version match, secrets
  ├─ Inject server metadata
  ├─ Allocate port from pool
  ├─ Move extracted dir → formations/{id}/current/
  ├─ Create version.json (v1)
  │
  ├─ If muxi_runtime specified:
  │   ├─ Resolve version constraint (resolver)
  │   ├─ EnsureSIF (download if missing)
  │   ├─ EnsureRuntimeRunner (macOS/Windows: pull Docker image)
  │   └─ Set spawn config: singularity + SIF path
  │
  ├─ Spawn process (Manager.Start)
  │   ├─ Native: exec.Command(python, app.py)
  │   ├─ Linux SIF: apptainer exec --bind ... SIF python -m muxi.runtime...
  │   └─ macOS SIF: docker run --privileged -v SIF ... runtime-runner singularity exec ...
  │
  ├─ Register in registry
  ├─ Health check loop (configurable timeout, default 5min)
  │   └─ GET http://localhost:{port}/v1/health
  ├─ Update status → "running"
  └─ Return formation details (or SSE stream)
```

### Update Flow (Zero-Downtime Blue-Green)
```
PUT /rpc/formations/{id} (gzip bundle)
  │
  ├─ Check formation exists, not already deploying
  ├─ SetDeploying(true) — prevents concurrent updates
  ├─ Extract bundle → temp dir
  ├─ Check bundle hash ≠ current (no-op if identical)
  ├─ Allocate STAGING port
  ├─ Move extracted → formations/{id}/staging/
  ├─ Preserve memory.db from current if not in upload
  ├─ Validate, inject metadata
  │
  ├─ Spawn staging on new port
  ├─ Health check staging
  │
  ├─ On success:
  │   ├─ SwitchToStagingPort (atomic port swap in registry)
  │   ├─ Stop old process (ForceKill if graceful fails)
  │   ├─ Move: staging → current, current → previous
  │   ├─ Update version.json
  │   └─ Return success
  │
  └─ On failure:
      ├─ Kill staging process
      ├─ Remove staging dir
      └─ Old version continues running (zero downtime maintained)
```

### Restore Flow (Server Restart)
```
cmdStart → RestoreFormations
  │
  ├─ Load registry.json (persisted formations)
  ├─ For each formation where status ≠ "stopped":
  │   ├─ Find formations/{id}/current/
  │   ├─ Parse formation.afs
  │   ├─ Compute env vars (same port as before)
  │   ├─ If muxi_runtime: EnsureSIF via downloader
  │   ├─ Spawn process
  │   └─ Preserve restart count from before server restart
  └─ Skip stopped formations
```

### Draft/Dev Flow (`muxi up`)
```
POST /rpc/dev/run {"path": "/abs/path"}
  │
  ├─ Parse formation.afs from path
  ├─ RegisterDraft (separate map, NOT persisted)
  ├─ Allocate port from shared pool
  ├─ Spawn with ID "{formation_id}-draft"
  ├─ Auto-restart: false (dev mode)
  ├─ Health check
  └─ Return port + status
  
Proxy: /draft/{formation_id}/* → GetDraft() → port
```

---

## 4. Platform Differences

| Aspect | Linux | macOS/Windows |
|--------|-------|---------------|
| Runtime | Apptainer/Singularity (native) | Docker + runtime-runner image |
| SIF execution | `apptainer exec --bind ... SIF cmd` | `docker run --privileged -v SIF runtime-runner singularity exec ...` |
| Bind host | `127.0.0.1` (config) | `0.0.0.0` (Docker network) |
| Host tools | Bind-mount real binaries from host | Pre-staged in runtime-runner at `/opt/muxi-tools` |
| RCE | SIF via Apptainer | Docker container |
| Cleanup | Kill process group | `docker rm -f` container |
| Install paths | System: `/etc/muxi/server`, `/var/lib/muxi` | User: `~/.muxi/server` |

---

## 5. Host Tools Binding (SIF)

When running formations inside SIF containers, host tools are made available at `/opt/muxi-tools/bin/`.

**Bound tools:** `node`, `npm`, `npx`, `bun`, `uv`, `uvx`, `git`, `curl`, `wget`, `jq`, `tar`, `gzip`, `unzip`, `ssh`, `sqlite3`, `python3`, `ffmpeg`, `ffprobe`, `tesseract`, `pdftotext`, `pdfinfo`, `pandoc`, `dot`, `make`, `gcc`, `g++`, `cc`

**Tool lookup:** Uses `bash -lc "which {tool}"` (login shell for full PATH including `~/.local/bin`), falls back to `exec.LookPath`.

**npm/npx wrapper scripts:** These tools are symlinks that use relative `require('../lib/cli.js')`. Bind-mounting the resolved real path breaks this. Solution: create wrapper scripts at `/tmp/muxi-tool-wrappers/{tool}` that invoke `exec /opt/muxi-tools/bin/node /opt/muxi-tools/lib/node_modules/npm/bin/{script} "$@"`.

**Environment inside SIF:**
```
PATH=/opt/muxi-tools/bin:$PATH
FONTCONFIG_PATH=/opt/muxi-tools/share/fonts
SSL_CERT_FILE=/opt/muxi-tools/share/certs/ca-certificates.crt
NODE_PATH=/opt/muxi-tools/lib/node_modules
```

**⚠️ No LD_LIBRARY_PATH** — intentionally omitted because runner's shared libraries would override SIF's own versions and break Python/SSL.

### Docker (macOS/Windows) Differences
On Docker (runtime-runner), tools are pre-staged inside the image. The entire `/opt/muxi-tools` is bind-mounted from Docker into the SIF:
```
--bind /opt/muxi-tools:/opt/muxi-tools
```

---

## 6. Runtime Version Resolution

**Resolver chain:**
1. `"latest"` or `""` → always returns `"latest"` string (forces downloader to resolve from GitHub)
2. `"1.2.3"` (exact 3-part) → pass through directly to downloader (check disk, download if missing)
3. `"1.2"` (2-part) → find latest `1.2.x` from local runtime registry
4. `"1"` (1-part) → find latest `1.x.x` from local runtime registry

**Downloader.EnsureSIF:**
1. If version is `"latest"` → `fetchLatestVersion()` via GitHub redirect (HEAD to `/releases/latest/download/version.txt`, parse version from redirect URL)
2. Check if SIF exists on disk at `~/.muxi/server/runtimes/muxi-runtime-{version}-linux-{arch}.sif`
3. If missing → download from `{sif_base_url}/v{version}/{filename}`
4. Returns: (sifPath, resolvedVersion, wasDownloaded, error)

**SIF filename format:** `muxi-runtime-{version}-linux-{arch}.sif` (always `linux-*` even on macOS — SIF is always a Linux container)

---

## 7. Port Allocation

- Pool: configurable range (default 8000–9000)
- Allocation: `PortPool.Allocate(formationID)` → finds first available port, marks as allocated
- Thread-safe via registry mutex
- Shared between live and draft formations (draft uses `{id}-draft` key)
- Staging uses `{id}-staging` key during blue-green deploy
- Port released on: delete, unregister, failed deploy cleanup
- `IsPortAvailable(port)` — actually tries `net.Listen("tcp", ":port")`

---

## 8. Configuration Platform Detection

```
Priority: MUXI_*_DIR env > binary location > user home

Linux + /usr/ binary → System install:
  Config: /etc/muxi/server/config.yaml
  Data:   /var/lib/muxi/
  Logs:   /var/log/muxi/

Windows + Program Files → System install:
  Config: C:\ProgramData\muxi\server\
  Data:   C:\ProgramData\muxi\data\
  
Otherwise → User install:
  Unix/macOS: ~/.muxi/server/
  Windows:    %APPDATA%\muxi\server\
```

`EnsureDirectories()` normalizes relative config paths (logs/, pids/, formations/) to absolute by joining with data dir. Also self-heals the HF cache dir (`<data_dir>/cache` or `MUXI_CACHE_DIR`) so the embedding pre-download has a guaranteed writable location regardless of umask or earlier partial installs.

**Cache directory (`GetCacheDir`):** priority `MUXI_CACHE_DIR` env > `<data_dir>/cache`. On macOS this resolves to `/Users/<user>/.muxi/server/cache`. Holds the embedding model files (see `pkg/hfcache`). Separate from the runtime SIF directory (`<data_dir>/runtimes`) because cache content is expendable — wiping it only costs a re-download, whereas wiping runtimes costs a several-hundred-MB SIF pull.

---

## 9. Dependencies

| Package | Purpose |
|---------|---------|
| `github.com/gorilla/mux` | HTTP routing |
| `github.com/rs/zerolog` | Zero-alloc structured logging |
| `gopkg.in/yaml.v3` | YAML config/formation parsing |
| `golang.org/x/sys` | Platform-specific syscalls (process groups) |

No ORM, no database, no external runtime dependencies. Registry is JSON file. Config is YAML file.

---

## 10. Testing

```bash
cd src
go test ./... -v -race                    # All tests with race detector
go test ./... -coverprofile=coverage.out   # Coverage
go test ./pkg/registry/... -fuzz FuzzValidateFormationID -fuzztime 5s
```

- **CI threshold:** 45% (platform-specific spawn code untestable on single OS)
- **CI runs on:** ubuntu-latest, Go 1.26, with race detector + coverage
- **Test ports:** 19000+ range (avoids conflict with formation port pool)
- **Table-driven tests** throughout
- **Fuzz tests** for registry validation and HMAC computation

---

## 11. Git Workflow

- `develop` → active development
- `rc` → release candidate (cross-platform build & test via `rc.yml`)
- `main` → production releases (auto-tagged via `release.yml`)
- Docker builds via `docker-build-publish.yml`
- SHA-pinned actions throughout

---

## 12. Gotchas and Learnings

### npm/npx Symlink Issue
npm and npx are typically symlinks to `node_modules/npm/bin/npm-cli.js`. They use `require('../lib/cli.js')` relative to themselves. When bind-mounting the resolved real path into the SIF, the relative require breaks. **Fix:** Create wrapper shell scripts that invoke `node /opt/muxi-tools/lib/node_modules/npm/bin/{script}` directly.

### Binary Path Mismatch on Server
When muxi-server runs under systemd, PATH is minimal. Tool lookup uses `bash -lc "which {tool}"` to get the full user PATH (including `~/.local/bin`). Falls back to `exec.LookPath` if login shell fails.

### LD_LIBRARY_PATH Intentionally Omitted
Host/runner shared libraries (libcrypto, libc) would override SIF's own versions and break Python/SSL imports. Tools in `/opt/muxi-tools/bin` rely on SIF's base system libraries instead.

### Resolver Pass-Through for Exact Versions
Exact 3-part versions (`"1.2.3"`) are passed through directly to the downloader without checking the local registry. This allows deploying with a version not yet downloaded. The downloader will fetch it from GitHub. Only partial versions (`"1.2"`, `"1"`) require local registry lookup.

### `"latest"` Always Hits GitHub
The resolver always returns the string `"latest"` for empty or "latest" constraints, forcing the downloader to resolve the actual version from GitHub. This prevents stale locally-cached versions from being used when "latest" is requested.

### Health Check Timing
- Deploy: handler does its own health check with SSE progress callbacks (`SkipInitialHealthCheck=true` for monitor)
- Monitor: 150 retries × 2s = 5 min max startup time for formations
- Docker + Singularity + Python startup can easily take 90+ seconds
- Health endpoint: `/v1/health` (configurable via `deployment.health_check.endpoint`)

### Blue-Green Port Lifecycle
During update, a staging port is allocated separately. On success, `SwitchToStagingPort()` atomically swaps the port in the registry (old port released, staging port becomes primary). On failure, staging port is released and old formation continues serving.

### Draft Formations Not Persisted
Draft formations (`draftFormations` map) are NOT saved to registry.json. They're lost on server restart. This is intentional — drafts are ephemeral development sessions.

### Container Cleanup on macOS/Windows
Before spawning a Docker-based formation, `CleanupDockerContainer()` removes existing containers by name (`muxi-{id}`) and by port. Handles orphans from crashes and server restarts.

### Docker `--privileged` Required
Docker-based SIF execution requires `--privileged` for Singularity user namespaces inside the container.

### WriteTimeout Disabled for SSE
HTTP server `WriteTimeout` is set to 0 to support SSE streaming for deploy/update progress. Health checks can take 2+ minutes.

### memory.db Preservation
During updates, `memory.db` is copied from current to staging if not included in the uploaded bundle. This preserves persistent memory state across formation versions.

### RCE Sidecar
Skills RCE runs as a separate managed process (or Docker container on macOS). Its URL and auth token are injected into every formation's environment as `MUXI_RCE_URL` and `MUXI_RCE_TOKEN`. Only started if `rce.auth_token` is configured.

### Server ID Generation
Server ID format: `server-{hostname}-{random_hex}`. Generated on first `init`, stored in config. Re-generated if missing on `start` (backward compatibility).

### Init UX — Three Setup Sections
`cmdInit` walks three dependency-setup sections in order, each using the same `* Setting up X... / ✓ X ready` pattern:

1. **Runtime-runner** (macOS/Windows only) — `docker pull ghcr.io/muxi-ai/runtime-runner:latest` via `pullRuntimeRunner()` → `dockerutil.RenderPullProgress` renders `⠋ Layers 5/8 (62%)`
2. **Skills RCE** — Linux: SIF download from GitHub releases. macOS/Windows: `docker pull ghcr.io/muxi-ai/skills-rce:latest` via `rce.EnsureDocker()` → also `dockerutil.RenderPullProgress`
3. **Embeddings** — `hfcache.EnsureLeanModel` into the cache dir, progress painted by `downloadReporter` (`⠙ 524 MiB downloaded`)

All three sections are best-effort; a failure prints a cross-mark and continues so the user isn't blocked on a transient network hiccup. The formation runtime will fetch any missing artifact on first deploy.

**Progress primitives** (all in `cmd/server/commands.go` except the renderer itself):
- `downloadReporter` — `io.Writer` that accumulates bytes via `atomic.Int64`; a ticker goroutine paints `spinner + MiB` every 100 ms. `finish()` stops the ticker and prints a terminating newline if any progress was painted (no-op on cached fast-path).
- `dockerutil.RenderPullProgress` — see package description above.

Both share `dockerutil.SpinnerFrames` + `dockerutil.SpinnerTick` so a single edit changes the spinner appearance everywhere.

### Docker `--quiet` Kills Progress Visibility
Early versions of `pullRuntimeRunner` and `rce.EnsureDocker` used `docker pull -q`, which silences the entire transfer. On a multi-hundred-megabyte image that made `init` look frozen for minutes with no output — users would kill the process thinking it hung. **Fix:** drop `-q` on both pulls, pipe stdout into `dockerutil.RenderPullProgress` for a clean collapsed line.

### `DOCKER_CLI_HINTS=false` Suppresses "What's Next"
Docker Desktop appends a promotional footer after every successful pull:
```
What's next:
    View a summary of image vulnerabilities and recommendations →
    docker scout quickview ghcr.io/muxi-ai/runtime-runner:latest
```
Pure noise in a bootstrap flow. Set `DOCKER_CLI_HINTS=false` on the `exec.Command` env and it disappears. Applied to both `pullRuntimeRunner` and `rce.EnsureDocker`.

### Docker Non-TTY vs TTY Output Formats
When `docker pull` runs with stdout attached to a TTY, it uses in-place line updates with per-byte progress:
```
abc123: Downloading [===>       ] 12MB/120MB
```
When stdout is a pipe (our case — we capture it for `RenderPullProgress`), Docker switches to line-per-event format without byte counters:
```
abc123: Pulling fs layer
abc123: Verifying Checksum
abc123: Pull complete
```
That's why `RenderPullProgress` parses layer-lifecycle events only (`Pulling fs layer` / `Pull complete`) and doesn't attempt to show per-byte progress — the source data isn't there. Layer-count progress is good enough UX and survives any future Docker output format changes as long as those two strings remain.

### Spinner Ticker Decoupled From Events
`RenderPullProgress` uses `select` over both events AND a 100 ms ticker. If we only repainted on events, the spinner would freeze during a large silent layer download (the exact moment users worry init is hung). The ticker guarantees ≥10 FPS animation regardless of event cadence. Same pattern in `downloadReporter`.

### HF Cache Fast-Path — No HTTP When Cached
`hfcache.EnsureLeanModel` returns `(alreadyCached bool, err error)`. On re-init or upgrade, `IsModelCached` sees all expected files with non-zero size, returns true, and `EnsureModel` returns `(true, nil)` without any HTTP call. The calling UX (`cmdInit`) converges cached and fresh paths on the same `✓ Embeddings ready` message — the user sees a progress line only when a download actually happened.

### Runtime Variants (cpu / gpu / cuda)
`runtime.ValidVariants` = `{"cpu", "gpu", "cuda"}`. Variant names enter the SIF filename as a suffix:
```
muxi-runtime-{version}-{variant}-linux-{arch}.sif
```
CPU is the default when unspecified. GPU/CUDA variants are opt-in and larger (cuDNN libraries bundled). The resolver passes the variant through untouched; the downloader maps it to the filename and checks disk before fetching.

**Where variants flow:** 7 API handlers (deploy, update, restore, dev, start, restart, rollback) parse the formation's `muxi_runtime.variant` field, validate against the allowlist, route to the variant-aware SIF, and set `HFCacheDir` on the spawn config so the embedding cache bind-mount is wired regardless of variant.
