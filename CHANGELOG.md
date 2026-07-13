# Changelog

## Unreleased

### Tuning state preserved across deployments

- **`MUXI.md` and `PENDING-MUXI.md` now survive formation updates.** The runtime's self-improvement pass (runtime #283-#285) accumulates learnings in `MUXI.md` at the formation root, with `PENDING-MUXI.md` holding suggestions awaiting review. Both are runtime-owned state, not operator config, so a version deploy must carry them forward the same way `memory.db` is preserved. `updateFromDirectory` (the single choke point for blue-green updates and draft deploys of existing formations; POST deploy rejects existing IDs) now copies both files from `current/` into `staging/` after the bundle move and before validation/spawn, so the staging formation boots with its learnings intact. Unlike `memory.db` (copied only when absent from the upload), the live tuning files always overwrite bundle copies: the server-side state is newer than anything an operator bundled. Copy failures log a warning and never block the deploy; the files are tuning state, not required for operation.
- **`preserveTuningFiles` helper** in `pkg/api/update.go` matches on-disk names exactly via `ReadDir` (the runtime accepts both `MUXI.md` and lowercase `muxi.md`); a plain per-candidate `os.Stat` would double-copy on case-insensitive filesystems such as macOS APFS, where `stat("muxi.md")` also matches `MUXI.md`.
- **Tests** in `pkg/api/update_test.go` (`TestPreserveTuningFiles`): both files preserved with content intact, live copy overwriting a bundle copy, lowercase variant, no-op when absent, and per-file failure reporting.

## 0.20260514.1

### Deploy bundle move fix

- **Bundle extraction now lives under `<DataDir>/tmp`** instead of the system `$TMPDIR` (usually `/tmp`). On modern Linux distros `/tmp` is a tmpfs mount on a different filesystem than `/var/lib/muxi`, and the previous extract path forced an `os.Rename` from one to the other — which fails with `EXDEV` ("invalid cross-device link"). Both `deploy.go` (new formations) and `update.go` (existing-formation blue-green updates) were affected. Operators saw the failure as `Failed to move source to staging` with no underlying cause, because the deploy handler logged the real error to journald but only the masked message reached the client. `EnsureDirectories` already created `<DataDir>/tmp` at every server start, so this path is same-FS by construction; the change is a one-line `MkdirTemp` argument at each call site.
- **`safeRename` helper** in `pkg/api/util.go` wraps `os.Rename` with a `copyTreePreservingMode` + `RemoveAll` fallback when the kernel returns `EXDEV`. Defense in depth for operators with non-standard layouts (e.g., `MUXI_DATA_DIR` bind-mounted onto a different mount than the rest of `/var`). Non-`EXDEV` rename errors propagate verbatim so the real cause still reaches the log instead of being masked behind a copy failure. `copyTreePreservingMode` preserves per-file modes — distinct from the simpler `copyDir` in `draft.go` which defaults files to `0644` and would silently widen a formation's `0600 secrets.enc` to world-readable on the fallback path.
- **`os.MkdirAll(formationBaseDir, 0755)`** added at the top of update.go's directory-setup block. Closes the registry-without-dir window we hit during the 0.20260514.0 deploy: `muxi server delete cicd` wiped `/var/lib/muxi/formations/cicd` on disk while the registry auto-save raced and left the in-memory entry pointing at a now-missing path, so the subsequent deploy was routed through update.go (which previously assumed the dir existed) and `os.Rename(/tmp/extract-..., /var/lib/muxi/formations/cicd/staging)` failed with `ENOENT`, masked as the same generic "Failed to move source to staging" message. A trivial `MkdirAll` closes the hole without changing the happy path.
- **Tests** in `pkg/api/util_test.go`: same-FS rename, EXDEV-stubbed fallback (verifies tree copy + `0600` and `0700` mode preservation + source cleanup), non-EXDEV error propagation, and `MUXI_DATA_DIR` env-override flow-through. The EXDEV branch is exercised by stubbing the package-level `renameFn` variable rather than requiring two physical filesystems in the test environment.

## 0.20260514.0

### Platform fix

- **`docker run --platform` pin in runtime-runner spawn**: the docker-wrapped Singularity path now passes `--platform linux/<arch>` derived from the SIF filename on every `docker run`. runtime-runner became a multi-arch image (amd64 + arm64); on Apple Silicon, Docker started resolving `:latest` to the host-native arm64 manifest, and Apptainer inside the arm64 container correctly refused to launch the amd64 SIF — `muxi up` failed with `FATAL: ... the image's architecture (amd64) could not run on the host's (arm64)`. The pin locks runner-arch to SIF-arch regardless of what Docker has cached locally, so a `docker pull` or `docker system prune` between server runs can no longer silently break the spawn path.
- **`sifPlatform(path)` helper** in `process/spawn_common.go` parses the arch suffix from `muxi-runtime-<version>[-<variant>]-linux-<arch>.sif` (lean, pytorch, cuda variants all covered). Unparseable paths default to `linux/amd64` to preserve the previous behavior for test fixtures and any out-of-convention SIF names.
- **Stale comment on `runtime/resolver.go::getPlatform`** updated: it claimed runtime-runner was amd64-only and cited `cmd/server/commands.go::pullRuntimeRunner` as the invariant. That stopped being true when the runner went multi-arch. The comment now points at `process/spawn_common.go` as the source of truth for arch pinning, and flags the next step (resolver should pick native linux-arm64 SIFs once those ship so Apple Silicon can skip the Rosetta hop).

## 0.20260428.2

### Multilingual classification model pre-download

- **Second embedding model preloaded during `muxi-server init`**: `Xenova/multilingual-e5-small` joins `nomic-ai/nomic-embed-text-v1.5` in the cache. Formations that need non-English retrieval now skip the multi-hundred-MB first-deploy stall.
- **Quantized ONNX only** (~125 MB): the multilingual file list is the transformers.js layout — `config.json`, `tokenizer.json`, `tokenizer_config.json`, `special_tokens_map.json`, `onnx/model_quantized.onnx`. No `pytorch_model.bin`, no sentence-transformers metadata.
- **Same best-effort contract** as the Nomic download: failure prints a warning and lets the runtime fetch on first deploy. Both models share `<cacheDir>` and the bind-mount into formation containers, so any runtime variant picks them up automatically.
- **Init UX**: a second `* Setting up multilingual classification model... / ✓ Multilingual classification model ready` section mirrors the existing embeddings section, including the spinner-driven progress line.

## 0.20260428.1

### Self-update URL fix

- **Server self-update no longer 404s**: download URL aligned with the v-prefixed S3 layout (`https://pkg.muxi.org/server/v{VERSION}/{binary}`). Both the release workflow's S3 upload path and the in-binary download URL now agree on the `v` prefix, matching the git tag and GitHub release naming.

## 0.20260424.0

### Runtime-runner image config threading

- **Single source of truth for the runtime-runner image**: new `config.DefaultRuntimeRunnerImage` constant. All 7 API handlers, `buildDockerSingularityCommand`, `validator.go`, and `cmdInit` now resolve from the config field or fall back to the constant — previously `spawn_common.go` hardcoded the image name, silently ignoring an operator's `runtime.runtime_runner_image` override in `config.yaml`.
- **`SpawnConfig.RuntimeRunnerImage`** field threaded through all spawn paths including rollback's `finalSpawnConfig`.
- **`ValidateRuntimeAvailable(runnerImage)`** and **`GetRuntimeInfo(wrapperImage)`** accept the configured image so startup validation and runtime-info reporting match what the spawn path actually uses.

### SIF downloads from pkg.muxi.org

- **Default SIF mirror changed** from `github.com/muxi-ai/runtime/releases/download` to `https://pkg.muxi.org/runtime`. URL scheme simplified to `{baseURL}/{version}/{filename}` — no more GitHub-specific redirect parsing.
- **`fetchLatestVersion`** now reads a plain-text `latest.txt` from the mirror instead of parsing GitHub redirect headers. Simpler, no rate-limit concerns.
- **Server self-update URL** changed from `releases.muxi.org` to `pkg.muxi.org/server`.

### S3 release uploads

- **Server binaries uploaded to S3** on release: `s3://BUCKET/server/VERSION/muxi-server-{os}-{arch}` with `--acl public-read`. Mirrors the runtime's existing S3 layout.
- **Runtime `latest.txt`** uploaded to `s3://BUCKET/runtime/latest.txt` after each release so `fetchLatestVersion` can resolve "latest" from the S3 mirror.

### Test stability

- **`TestHandleRollback` and `TestHandleBundleDeploy_ValidBundle` no longer hang**: root cause was `formation.Load()` defaulting `MuxiRuntime` to `"latest"`, triggering a real SIF download from GitHub with no HTTP client timeout. Fixed by (1) pointing test `SIFBaseURL` at a non-routable address, (2) adding `DialContext` + `ResponseHeaderTimeout` to the downloader's HTTP transport, and (3) unifying health-check timeouts via `resolveHealthTimeout(cfg)` across all 6 spawn-and-wait handlers.

### Platform fix

- **SIF arch pinned to `linux-amd64` on macOS/Windows**: the runtime-runner Docker image is x86_64-only (Singularity ships no arm64 build), so Apple Silicon was downloading an arm64 SIF that the amd64 container couldn't load. `getPlatform()` now keys off `GOOS` instead of `GOARCH` on non-Linux hosts.

## 0.20260423.0

### Embedding model pre-download

- **`muxi-server init` pre-downloads the default embedding model** (`nomic-ai/nomic-embed-text-v1.5`, ~524 MiB) into the cache dir so the first formation deploy doesn't stall on a multi-hundred-MB fetch. Pure HTTP — works identically on Linux, macOS, and Windows.
- **Fast-path on re-init / upgrade**: if every model file is already present and non-zero size, init skips the HTTP download entirely and converges on the same `✓ Embeddings ready` confirmation. Safe to run `init` or `upgrade` repeatedly.
- **Atomic writes**: each file is written to `<file>.tmp` and atomically renamed. A killed process can't poison the cache with partial files — a subsequent init re-fetches cleanly.
- **Cache directory**: new `MUXI_CACHE_DIR` env var overrides the default `<data_dir>/cache` location. Self-healed on startup so the bind-mount into formation containers always has a writable target.

### Runtime variants (CPU / GPU / CUDA)

- **`muxi_runtime.variant`**: formations can now opt into GPU or CUDA runtime SIFs. Variant names enter the SIF filename as a suffix — `muxi-runtime-{version}-{variant}-linux-{arch}.sif` — so CPU, GPU, and CUDA builds coexist on the same host.
- **Variant validation** across 7 API handlers (deploy, update, restore, dev, start, restart, rollback) rejects unknown variants with a clear error instead of downloading a nonexistent SIF.
- **HF cache bind-mount** (`<cacheDir>` → `/opt/hf-cache`) wired on both native (Apptainer) and Docker-wrapper paths with `HF_HOME=/opt/hf-cache`. Any runtime variant can now reuse the pre-downloaded embedding model without re-fetching.

### Init UX polish

- **Single-line progress for Docker pulls**: runtime-runner and Skills RCE pulls now collapse Docker's 50+ lines of per-layer output into one animated line — `⠙ Layers 5/8 (62%)`. The braille spinner ticks every 100 ms so the line keeps animating during silent layer downloads.
- **Spinner for embedding download**: the HTTP download paints `⠙ 524 MiB downloaded` with the same spinner style, so all three setup sections feel consistent.
- **Dropped `--quiet`** on both Docker pulls. The old silent mode made init look frozen for minutes on multi-hundred-MB transfers; explicit progress is better.
- **`DOCKER_CLI_HINTS=false`** suppresses Docker Desktop's "What's next: docker scout quickview…" promotional footer that cluttered every pull.
- **Terser messaging**: all three setup sections use the same `* Setting up X... / ✓ X ready` pattern. Embedding model name and cache path are no longer printed — they're noise in an init transcript users read once.

Final init transcript on macOS (fresh machine):

```
* Setting up runtime-runner...
  ⠹ Layers 5/8 (62%)
✓ Runtime-runner ready

* Setting up Skills RCE...
  ⠹ Layers 3/4 (75%)
✓ Skills RCE ready

* Setting up embeddings...
  ⠙ 524 MiB downloaded
✓ Embeddings ready
```

---

## 0.20260402.0

### Fixes

- **Create tmp directory on startup**: `EnsureDirectories` now creates `{dataDir}/tmp` so `TMPDIR=/var/lib/muxi/tmp` works out of the box when deploying formations via Docker.
- **Default health check endpoint**: changed from `/health` to `/v1/health` to match the MUXI runtime API. Fixes formations failing health checks on first deploy.

## 0.20260401.1

### Fixes

- **Auto-install Apptainer on server start**: if `apptainer`/`singularity` is not found on Linux, `muxi-server start` now automatically installs Apptainer before proceeding. Solves the issue where Apptainer was lost on Docker container restarts.
- **Apptainer/Singularity lookup fix**: runtime validation and binary lookup now prefer `apptainer` over `singularity`, matching what the installer actually installs. Previously the server only looked for `singularity`, which doesn't exist after a standard Apptainer install.

## 0.20260401.0

### Fixes

- **Ubuntu-based Docker runtime image**: switched the runtime image from Alpine to Ubuntu so `muxi-server init` works in-container on Linux without failing distro detection for Apptainer installation.
- **Container runtime deps update**: replaced `apk`-based runtime dependencies with `apt` packages (`ca-certificates`, `docker.io`, `wget`) to match the Ubuntu base image.

## 0.20260323.0

### Fixes

- **Docker networking for host services**: added `--add-host localhost:host-gateway` and `--add-host host.docker.internal:host-gateway` to runtime-runner Docker commands so formations can reach host-local services (e.g. PostgreSQL) via `localhost` without changing connection strings
- **Release downloads via CDN**: switched `github.com/muxi-ai/*/releases/download/*` URLs to `releases.muxi.org/*/releases/download/*` for server/runtime download paths.

## 0.20260310.0

### Skills RCE Integration

- **Built-in code execution**: formations now ship with a managed RCE (Remote Code Execution) service for Skills
- **`muxi-server init`**: downloads RCE automatically (SIF on Linux, Docker image on macOS/Windows)
- **`muxi-server start`**: launches RCE as a managed process, injects `MUXI_RCE_URL` and `MUXI_RCE_TOKEN` into all formations
- **Auto port discovery**: if default port 7891 is occupied, scans upward for an available port

### Upgrade Command

- **`muxi-server upgrade`**: self-update the server binary, pull latest RCE, and migrate config
- Downloads latest server binary from GitHub releases (atomic swap with rollback)
- Adds missing config fields (e.g. RCE auth token) to existing configurations

### Fixes

- **HuggingFace model cache**: pass `HF_HOME=/opt/hf-cache` to SIF containers so the pre-cached embedding model is used instead of re-downloading on every startup (~80s), which caused health check timeouts
- **npm/npx in SIF containers**: npm and npx are symlinks that use relative `require('../lib/cli.js')`; bind-mounting the resolved path broke the import. Now creates wrapper scripts that invoke node with the full path to the npm module
- **Exact runtime version pinning**: versions like `muxi_runtime: "0.20260220.0"` were rejected by the resolver if not in the local registry. Now passes exact versions through to the downloader, which checks disk and downloads if needed
- **Restore path**: use downloader in the restore path to resolve `latest` runtime from GitHub instead of building a literal `muxi-runtime-latest-*.sif` filename
- **Runtime resolution**: always resolve `latest` runtime from GitHub instead of using stale locally-cached version
- **Host tools**: add `npm`, `npx`, `bun`, `uv`, `uvx`, `tar`, and `gzip` to tools bind-mounted into SIF containers

---

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





