# Runtime Integration TODO - Phase 2 (YAML-based Formations)

**Status:** Ready to implement
**Dependencies:** Runtime SIF files available
**Timeline:** 1-2 weeks
**Priority:** HIGH

---

## Overview

Integrate MUXI Server with versioned MUXI Runtime SIF files to execute Phase 2 (YAML-based) formations using Singularity containers.

**Runtime Documentation:** See `../runtime/SERVER_INTEGRATION.md` for complete integration guide.

---

## Architecture Changes

### Before (Phase 1: app.py-based)
```go
// Direct Python execution
cmd := exec.Command("python", "app.py")
```

Formation contained FastAPI server code (`app.py`).

### After (Phase 2: YAML-based)
```go
// Singularity container execution
cmd := exec.Command("singularity", "exec",
    "--bind", formationDir + ":/formation",
    sifPath,
    "python", "-m", "muxi.utils.run_formation",
    "/formation/formation.afs",
    "--port", port,
    "--host", "127.0.0.1",
)
```

Formation is pure configuration (YAML), runtime provides server.

---

## Key Concepts

### 1. Versioned Runtime Distribution

**SIF Naming:**
```
muxi-runtime-{version}-{platform}.sif

Examples:
  muxi-runtime-0.2025.0-linux-amd64.sif
  muxi-runtime-0.2024.12-linux-amd64.sif
```

**Storage Location:**
```
~/.muxi/server/runtimes/
├── muxi-runtime-0.2025.0-linux-amd64.sif
├── muxi-runtime-0.2024.12-linux-amd64.sif
└── muxi-runtime-0.2024.11-linux-amd64.sif
```

### 2. Formation Directory Structure

```
~/.muxi/server/formations/{formation-id}/
├── current/                          ← Active version
│   ├── formation.yaml                ← Configuration
│   ├── .key                          ← Encryption key
│   ├── secrets.enc                   ← Secrets
│   ├── agents/                       ← Agent defs
│   ├── mcp/                          ← MCP servers
│   ├── a2a/                          ← A2A services
│   └── knowledge/                    ← Knowledge (confined)
├── previous/                         ← Backup
└── version.json                      ← Metadata
```

**Key Point:** Everything self-contained, single mount point.

### 3. Runtime Version Resolution

Formations specify runtime version in `formation.yaml`:
```yaml
runtime: "0.2025.0"    # Exact version
runtime: "0.2025"      # Latest 0.2025.x
runtime: "0"           # Latest 0.x.x
runtime: "latest"      # Absolute latest
```

Server resolves to exact version and pins it.

---

## Implementation Tasks

### 1. Runtime Resolver

**File:** `src/pkg/runtime/resolver.go` (new package)

```go
package runtime

type Resolver struct {
    runtimesDir string
    availableVersions []string
}

// Resolve resolves a version constraint to exact version
// "0.2025" → "0.2025.0" (latest 0.2025.x)
func (r *Resolver) Resolve(constraint string) (string, error)

// GetSIFPath returns path to SIF file for version
func (r *Resolver) GetSIFPath(version string) string

// ListAvailable returns all available runtime versions
func (r *Resolver) ListAvailable() []string

// Download downloads runtime SIF from registry (future)
func (r *Resolver) Download(version string) error
```

**Tests:** `src/pkg/runtime/resolver_test.go`

**Tasks:**
- [ ] Implement version constraint parsing
- [ ] Implement semantic version comparison
- [ ] Implement "latest" resolution
- [ ] Add SIF file discovery from runtimesDir
- [ ] Add platform detection (linux-amd64, darwin-arm64)
- [ ] Add unit tests (table-driven)
- [ ] Add error handling for missing runtimes

**Reference:** `../runtime/SERVER_INTEGRATION.md` - "Version Resolution" section

---

### 2. Update Process Spawning

**File:** `src/pkg/process/spawn_common.go`

Current code spawns Python directly. Need to:

**Changes:**
```go
// Add runtime resolver to ProcessManager
type ProcessManager struct {
    // ... existing fields ...
    runtimeResolver *runtime.Resolver
    runtimesDir     string
}

// Update spawnProcess to use Singularity
func (pm *ProcessManager) spawnProcess(proc *Process) error {
    // 1. Read formation.yaml to get runtime version
    formationConfig := readFormationYAML(proc.FormationDir)

    // 2. Resolve runtime version
    version, err := pm.runtimeResolver.Resolve(formationConfig.Runtime)
    if err != nil {
        return fmt.Errorf("failed to resolve runtime: %w", err)
    }

    // 3. Get SIF path
    sifPath := pm.runtimeResolver.GetSIFPath(version)
    if !fileExists(sifPath) {
        return fmt.Errorf("runtime not found: %s (version: %s)", sifPath, version)
    }

    // 4. Build Singularity command
    formationPath := filepath.Join(proc.FormationDir, "current", "formation.yaml")

    cmd := exec.Command("singularity", "exec",
        "--bind", filepath.Dir(formationPath) + ":/formation",
        sifPath,
        "python", "-m", "muxi.utils.run_formation",
        "/formation/formation.afs",
        "--port", fmt.Sprintf("%d", proc.Port),
        "--host", "127.0.0.1",
    )

    // 5. Rest of spawning logic (logs, PID, etc.)
    // ... existing code ...
}
```

**Tasks:**
- [ ] Add runtime resolver to ProcessManager
- [ ] Update spawnProcess to read formation.yaml
- [ ] Implement runtime version resolution
- [ ] Build Singularity exec command
- [ ] Update tests to mock Singularity
- [ ] Add integration tests with real SIF

**Files to modify:**
- `src/pkg/process/spawn_common.go`
- `src/pkg/process/manager.go` (add resolver field)
- `src/pkg/process/manager_unit_test.go` (update tests)

---

### 3. Health Check After Spawn

**File:** `src/pkg/process/monitor.go`

After spawning, wait for formation to be ready:

```go
// WaitForReady waits for formation API to be ready
func (pm *ProcessManager) WaitForReady(proc *Process) error {
    url := fmt.Sprintf("http://127.0.0.1:%d/", proc.Port)
    timeout := 30 * time.Second
    interval := 1 * time.Second

    deadline := time.Now().Add(timeout)

    for time.Now().Before(deadline) {
        resp, err := http.Get(url)
        if err == nil && resp.StatusCode == 200 {
            resp.Body.Close()
            log.Info().
                Str("formation_id", proc.FormationID).
                Int("port", proc.Port).
                Msg("Formation ready")
            return nil
        }
        if resp != nil {
            resp.Body.Close()
        }
        time.Sleep(interval)
    }

    return fmt.Errorf("formation failed to become ready within %s", timeout)
}
```

**Tasks:**
- [ ] Implement WaitForReady function
- [ ] Call from StartFormation after spawn
- [ ] Add configurable timeout
- [ ] Add unit tests with mock HTTP server
- [ ] Handle partial startup failures

---

### 4. Configuration Updates

**File:** `src/pkg/config/config.go`

Add runtime configuration:

```go
type Config struct {
    // ... existing fields ...

    Runtime struct {
        Type        string `yaml:"type"`         // "singularity" (default), "docker", "native"
        RuntimesDir string `yaml:"runtimes_dir"` // ~/.muxi/server/runtimes

        // Future: registry for downloading runtimes
        Registry struct {
            URL       string `yaml:"url"`
            AuthToken string `yaml:"auth_token"`
        } `yaml:"registry"`
    } `yaml:"runtime"`
}
```

**Default config template:**
```yaml
runtime:
  type: "singularity"
  runtimes_dir: "~/.muxi/server/runtimes"

  # Future: download runtimes from registry
  # registry:
  #   url: "https://registry.muxi.org"
  #   auth_token: "${MUXI_REGISTRY_TOKEN}"
```

**Tasks:**
- [ ] Add Runtime struct to Config
- [ ] Update default config template
- [ ] Add validation for runtimesDir
- [ ] Update docs/configuration.md

---

### 5. Formation Metadata Updates

**File:** `src/pkg/registry/formation.go`

Track exact runtime version used:

```go
type Formation struct {
    // ... existing fields ...

    // Runtime information
    RuntimeVersion string `json:"runtime_version"` // Exact version (e.g., "0.2025.0")
    RuntimePinned  string `json:"runtime_pinned"`  // Constraint from formation.yaml
}
```

**Tasks:**
- [ ] Add RuntimeVersion and RuntimePinned fields
- [ ] Update formation create/update to resolve and store version
- [ ] Update API responses to include runtime info
- [ ] Update tests

---

### 6. API Endpoint Updates

**File:** `src/pkg/api/runtimes.go` (new)

Add runtime management endpoints:

```go
// GET /rpc/runtimes
// List available runtime versions
func HandleListRuntimes(w http.ResponseWriter, r *http.Request)

// POST /rpc/runtimes/download
// Download runtime from registry (future)
func HandleDownloadRuntime(w http.ResponseWriter, r *http.Request)

// GET /rpc/runtimes/{version}
// Get runtime details
func HandleGetRuntime(w http.ResponseWriter, r *http.Request)
```

**Tasks:**
- [ ] Implement HandleListRuntimes
- [ ] Update API docs
- [ ] Add authentication checks
- [ ] Add tests

---

### 7. Documentation Updates

**Files to update:**

1. **docs/runtime-architecture.md**
   - [ ] Add Phase 2 (YAML-based) section
   - [ ] Document Singularity execution
   - [ ] Add version resolution section
   - [ ] Add diagrams

2. **docs/formations.md**
   - [ ] Add runtime version specification
   - [ ] Update bundle structure (YAML, not app.py)
   - [ ] Add knowledge path security notes

3. **docs/configuration.md**
   - [ ] Add runtime configuration section
   - [ ] Document runtimes_dir setting

4. **docs/troubleshooting.md**
   - [ ] Add runtime not found errors
   - [ ] Add Singularity installation guide
   - [ ] Add version resolution issues

5. **STATUS.md**
   - [ ] Update next steps with runtime integration completion
   - [ ] Update blocked status

---

### 8. Integration Testing

**File:** `test/integration/runtime_test.go` (new)

**Test scenarios:**
- [ ] Deploy Phase 2 formation (YAML-based)
- [ ] Runtime version resolution
- [ ] Formation startup with SIF
- [ ] Health check and readiness
- [ ] API proxy to formation
- [ ] Formation update with same runtime version
- [ ] Formation update with different runtime version
- [ ] Multiple formations with different runtime versions
- [ ] Formation rollback
- [ ] Missing runtime error handling

**Test fixtures needed:**
```
test/fixtures/
├── formations/
│   ├── simple-chatbot/
│   │   ├── formation.yaml
│   │   ├── .key
│   │   └── secrets.enc
│   └── multi-agent/
│       └── ...
└── runtimes/
    └── muxi-runtime-0.2025.0-linux-amd64.sif (symlink to real SIF)
```

---

### 9. Backward Compatibility (Optional)

Support both Phase 1 (app.py) and Phase 2 (YAML) formations:

```go
// Detect formation type
func (pm *ProcessManager) detectFormationType(formationDir string) string {
    appPyPath := filepath.Join(formationDir, "current", "app.py")
    formationYamlPath := filepath.Join(formationDir, "current", "formation.yaml")

    if fileExists(formationYamlPath) {
        return "phase2"  // YAML-based
    } else if fileExists(appPyPath) {
        return "phase1"  // app.py-based
    }
    return "unknown"
}

// Spawn based on type
func (pm *ProcessManager) spawnProcess(proc *Process) error {
    formationType := pm.detectFormationType(proc.FormationDir)

    switch formationType {
    case "phase1":
        return pm.spawnPhase1(proc)
    case "phase2":
        return pm.spawnPhase2(proc)
    default:
        return fmt.Errorf("unknown formation type")
    }
}
```

**Tasks:**
- [ ] Implement formation type detection
- [ ] Keep Phase 1 spawn logic for backward compat
- [ ] Document both phases in user docs
- [ ] Add tests for both types

**Decision:** Discuss with team whether to support both or Phase 2 only.

---

## Dependencies

### Required

1. **Singularity/Apptainer installed**
   ```bash
   # Linux
   sudo apt-get install singularity-ce

   # macOS (via Docker)
   # Uses Docker-wrapped Singularity
   ```

2. **Runtime SIF files**
   ```bash
   mkdir -p ~/.muxi/server/runtimes

   # Copy from runtime build
   cp muxi-runtime-0.2025.0-linux-amd64.sif \
      ~/.muxi/server/runtimes/
   ```

3. **Test formations**
   ```bash
   # Use runtime's test formations
   cp -r ../runtime/e2e/tests/1_foundation/formations/formation-base \
      ./test/fixtures/formations/
   ```

---

## Testing Plan

### Unit Tests
- [ ] Runtime resolver tests (version parsing, comparison)
- [ ] Formation type detection tests
- [ ] Singularity command building tests
- [ ] Health check retry logic tests

### Integration Tests
- [ ] End-to-end formation deployment
- [ ] Multi-formation orchestration
- [ ] Version updates and rollbacks
- [ ] Error scenarios (missing runtime, spawn failure)

### Manual Testing
```bash
# 1. Build server
go build -o muxi-server ./src/cmd/server

# 2. Initialize server
./muxi-server init

# 3. Start server
./muxi-server serve

# 4. Deploy formation (in another terminal)
curl -X POST http://localhost:7890/rpc/formations/deploy \
  -H "Authorization: MUXI-HMAC-SHA256 Credential=..." \
  -H "Content-Type: application/gzip" \
  --data-binary @test-formation.tar.gz

# 5. Check formation status
curl http://localhost:7890/rpc/formations

# 6. Test formation proxy
curl http://localhost:7890/api/test-formation/

# 7. Check logs
tail -f ~/.muxi/server/logs/formation-test-formation.log
```

---

## Success Criteria

- [ ] Server spawns formations using Singularity exec
- [ ] Runtime version resolution works
- [ ] Formations start successfully and respond to health checks
- [ ] HTTP proxy routes to formations correctly
- [ ] Multiple formations with different runtime versions work
- [ ] Formation updates preserve data
- [ ] Rollbacks work correctly
- [ ] All tests pass (unit + integration)
- [ ] Documentation updated
- [ ] No performance regression (latency <5ms proxy overhead)

---

## Rollout Plan

### Phase 1: Development (Week 1)
- [ ] Implement runtime resolver
- [ ] Update process spawning
- [ ] Add health check logic
- [ ] Write unit tests

### Phase 2: Integration (Week 2)
- [ ] Add integration tests
- [ ] Manual testing with real formations
- [ ] Documentation updates
- [ ] Performance testing

### Phase 3: Deployment
- [ ] Code review
- [ ] Update CI/CD pipelines
- [ ] Deploy to staging
- [ ] Monitor and iterate

---

## Reference Links

**Primary Reference:**
- `../runtime/SERVER_INTEGRATION.md` - Complete integration guide from runtime team

**Related Docs:**
- `docs/runtime-architecture.md` - Server's runtime architecture
- `docs/formations.md` - Formation management
- `AGENTS.md` - Development guidelines

**Runtime Docs:**
- `../runtime/RUNTIME_VERSIONING.md` - Version management
- `../runtime/DOCKER_TESTING.md` - Testing guide

---

## Questions & Decisions

### Open Questions
1. **Backward compatibility:** Support both Phase 1 (app.py) and Phase 2 (YAML)?
   - **Recommendation:** Phase 2 only (simpler, cleaner)
   - **Alternative:** Support both with type detection

2. **Runtime registry:** Implement automatic download of missing runtimes?
   - **Recommendation:** Phase 2+ feature (manual install for now)
   - **Benefit:** Users don't need to manually download SIFs

3. **Platform detection:** Auto-detect platform or explicit config?
   - **Recommendation:** Auto-detect with config override

4. **Multiple runtime types:** Support Docker as fallback?
   - **Recommendation:** Singularity only on Linux, Docker-wrapped on macOS
   - **Note:** Docker has more overhead than Singularity

### Decisions Made
- ✅ Use Singularity exec (not run) for stateless execution
- ✅ Formations bind to 127.0.0.1 (localhost only)
- ✅ CLI arguments over environment variables
- ✅ Knowledge paths confined to formation directory
- ✅ Version resolution with pinning

---

## Contact

**Questions?**
- Runtime team: See `../runtime/` repository
- Server team: See this repository's maintainers
- Integration issues: Open issue with `integration` label

---

**Last Updated:** 2025-11-25
**Status:** Ready to implement
**Next Action:** Review with team and begin implementation
