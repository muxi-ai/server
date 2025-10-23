# API Architecture Implementation Plan v2

**Goal:** Migrate from current API structure to new RESTful architecture

**Timeline:** ~4 hours of focused work  
**Risk Level:** Medium (routing changes + new features)  
**Test Coverage:** Must maintain 88.9%+ test coverage

---

## Current State vs Target State

### Current API Routes:
```
Port: 3000 (default)
Paths: ~/.muxi-server/*

/health                        → Health check (no auth)
/formations/deploy             → Deploy (HMAC auth)
/formations                    → List (HMAC auth)
/formations/{id}               → Get/Delete (HMAC auth)
/formations/{id}/stop          → Stop (HMAC auth)
/formations/{id}/restart       → Restart (HMAC auth)
/formations/{id}/logs          → Logs (HMAC auth)
/v1/{formation_id}/{path:.*}   → Proxy (no auth)
```

### Target API Routes:
```
Port: 7890 (official MUXI port)
Paths: /etc/muxi/server, /var/lib/muxi, /var/log/muxi

# Public
GET    /health                            → Health check (no auth)
GET    /ping                              → Ping (no auth)

# Formation Management
POST   /rpc/formations                    → Deploy (HMAC auth)
GET    /rpc/formations                    → List (HMAC auth)
GET    /rpc/formations/{id}               → Get (HMAC auth)
PUT    /rpc/formations/{id}               → Update version (HMAC auth) ⭐ NEW
DELETE /rpc/formations/{id}               → Delete (HMAC auth)

# Formation Actions
POST   /rpc/formations/{id}/stop          → Stop (HMAC auth)
POST   /rpc/formations/{id}/restart       → Restart (HMAC auth)
POST   /rpc/formations/{id}/rollback      → Rollback (HMAC auth) ⭐ NEW

# Server Management
GET    /rpc/server/status                 → Server info (HMAC auth) ⭐ NEW
GET    /rpc/server/logs                   → Audit log (HMAC auth) ⭐ NEW

# Formation Proxy
*      /api/{formation_id}/*              → Proxy (no auth)
```

---

## System Paths

### **Production (System Installation):**

```
/usr/local/bin/muxi-server     ← Binary
/etc/muxi/server/              ← Configuration
  config.yaml
  credentials.yaml
/var/lib/muxi/                 ← Data
  formations/
  registry.json
/var/log/muxi/                 ← Logs
  audit.log
  formations/
```

### **Development (Local):**

```
./muxi-server                  ← Binary
./config.yaml                  ← Configuration
./data/                        ← Data
./logs/                        ← Logs
```

---

## Phase 0: Path Migration (30 min)

### 0.1 Update All Path References

**File:** `src/pkg/config/config.go`

**Current:**

```go
configDir := filepath.Join(home, ".muxi-server")
```

**Change to:**

```go
// System paths (production)
configDir := "/etc/muxi/server"
dataDir := "/var/lib/muxi"
logDir := "/var/log/muxi"

// Development: Check for local config first
if fileExists("./config.yaml") {
    configDir = "."
    dataDir = "./data"
    logDir = "./logs"
}
```

### 0.2 Update Default Config Paths

```go
func DefaultConfig() *Config {
    return &Config{
        Server: ServerConfig{
            Host: "0.0.0.0",
            Port: 7890,
        },
        Formations: FormationsConfig{
            PortRangeStart: 8000,
            PortRangeEnd:   9000,
            BindHost:       "127.0.0.1",
            DataDir:        "/var/lib/muxi/formations",
        },
        Logging: LoggingConfig{
            Level:     "info",
            AuditLog:  "/var/log/muxi/audit.log",
        },
    }
}
```

### 0.3 Update All Tests

**Files:** All `*_test.go` files

Update any hardcoded paths:
- `.muxi-server` → Use temp directories in tests
- `3000` → `7890`

---

## Phase 1: Configuration Changes (30 min)

### 1.1 Update Port Default

**File:** `src/pkg/config/config.go`

```go
type ServerConfig struct {
    Port int    `yaml:"port"` // Default: 7890
    Host string `yaml:"host"` // Default: 127.0.0.1
}
```

### 1.2 Add Formation Bind Host

```go
type FormationsConfig struct {
    PortRangeStart int    `yaml:"port_range_start"` // Default: 8000
    PortRangeEnd   int    `yaml:"port_range_end"`   // Default: 9000
    BindHost       string `yaml:"bind_host"`        // Default: 127.0.0.1 ⭐ NEW
    DataDir        string `yaml:"data_dir"`         // Default: /var/lib/muxi/formations
    KeepBackups    int    `yaml:"keep_backups"`     // Default: 1 ⭐ NEW
}
```

### 1.3 Add Logging Config

```go
type LoggingConfig struct {
    Level    string `yaml:"level"`     // Default: info
    AuditLog string `yaml:"audit_log"` // Default: /var/log/muxi/audit.log ⭐ NEW
}
```

### 1.4 Update Config Tests

**File:** `src/pkg/config/config_test.go`

- Update port expectations: 3000 → 7890
- Add BindHost tests
- Add KeepBackups tests
- Add AuditLog tests

---

## Phase 2: Environment Variables (20 min)

### 2.1 Update Formation Environment Variables

**File:** `src/pkg/formation/formation.go`

```go
func (f *Formation) GetEnvironmentVars(port int, serverURL string, bindHost string) map[string]string {
    env := make(map[string]string)
    
    // Network binding (CRITICAL for security)
    env["PORT"] = fmt.Sprintf("%d", port)
    env["HOST"] = bindHost  // ⭐ NEW
    
    // Formation metadata
    env["FORMATION_ID"] = f.ID
    env["MUXI_SERVER_URL"] = serverURL
    env["MUXI_ENV"] = "production"
    
    // Metadata for formation.yaml injection
    env["_bind_host"] = bindHost  // ⭐ NEW
    env["_port"] = fmt.Sprintf("%d", port)
    
    return env
}
```

### 2.2 Update All Callers

**Files:**
- `src/pkg/process/manager.go`
- `src/pkg/api/deploy.go`

Update all calls to `GetEnvironmentVars()`:

```go
// Before:
env := formation.GetEnvironmentVars(port, serverURL)

// After:
env := formation.GetEnvironmentVars(port, serverURL, m.config.Formations.BindHost)
```

### 2.3 Update Tests

**File:** `src/pkg/formation/formation_test.go`

```go
func TestGetEnvironmentVars(t *testing.T) {
    f := &Formation{ID: "test-api"}
    env := f.GetEnvironmentVars(8080, "http://localhost:7890", "127.0.0.1")
    
    tests := []struct {
        key  string
        want string
    }{
        {"PORT", "8080"},
        {"HOST", "127.0.0.1"},
        {"_bind_host", "127.0.0.1"},
        // ...
    }
    // ...
}
```

---

## Phase 3: RESTful Routing (90 min)

### 3.1 Add Ping Handler

**File:** `src/pkg/api/ping.go` (NEW)

```go
package api

import "net/http"

// HandlePing handles GET /ping
func (s *Server) HandlePing(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "text/plain")
    w.WriteHeader(http.StatusOK)
    w.Write([]byte("pong"))
}
```

### 3.2 Add Reserved Formation IDs Validation

**File:** `src/pkg/registry/validation.go` (NEW)

```go
package registry

import (
    "fmt"
    "regexp"
)

var (
    reservedIDs = map[string]bool{
        "health":  true,
        "ping":    true,
        "rpc":     true,
        "server":  true,
        "admin":   true,
        "metrics": true,
        "api":     true,
    }
    
    formationIDPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,48}[a-z0-9]$`)
)

func ValidateFormationID(id string) error {
    if id == "" {
        return fmt.Errorf("formation ID cannot be empty")
    }
    
    if len(id) < 3 || len(id) > 50 {
        return fmt.Errorf("formation ID must be 3-50 characters")
    }
    
    if reservedIDs[id] {
        return fmt.Errorf("formation ID %q is reserved", id)
    }
    
    if !formationIDPattern.MatchString(id) {
        return fmt.Errorf("formation ID must contain only lowercase letters, numbers, and hyphens")
    }
    
    return nil
}
```

### 3.3 Update Server Routes

**File:** `src/pkg/api/server.go`

```go
func (s *Server) setupRoutes() {
    // Middleware
    s.router.Use(s.loggingMiddleware)
    s.router.Use(s.corsMiddleware)

    // ====================================
    // PUBLIC ENDPOINTS (no auth)
    // ====================================
    s.router.HandleFunc("/health", s.HandleHealth).Methods(http.MethodGet)
    s.router.HandleFunc("/ping", s.HandlePing).Methods(http.MethodGet)

    // ====================================
    // MANAGEMENT API /rpc/* (requires auth)
    // ====================================
    rpc := s.router.PathPrefix("/rpc").Subrouter()
    rpc.Use(s.authMiddleware.Authenticate)
    rpc.Use(s.auditMiddleware)  // ⭐ NEW: Audit logging

    // Formation management
    rpc.HandleFunc("/formations", s.HandleDeploy).Methods(http.MethodPost)
    rpc.HandleFunc("/formations", s.HandleList).Methods(http.MethodGet)
    rpc.HandleFunc("/formations/{id}", s.HandleGet).Methods(http.MethodGet)
    rpc.HandleFunc("/formations/{id}", s.HandleUpdate).Methods(http.MethodPut)      // ⭐ NEW
    rpc.HandleFunc("/formations/{id}", s.HandleDelete).Methods(http.MethodDelete)
    
    // Formation actions
    rpc.HandleFunc("/formations/{id}/stop", s.HandleStop).Methods(http.MethodPost)
    rpc.HandleFunc("/formations/{id}/restart", s.HandleRestart).Methods(http.MethodPost)
    rpc.HandleFunc("/formations/{id}/rollback", s.HandleRollback).Methods(http.MethodPost)  // ⭐ NEW
    
    // Server management
    rpc.HandleFunc("/server/status", s.HandleServerStatus).Methods(http.MethodGet)  // ⭐ NEW
    rpc.HandleFunc("/server/logs", s.HandleServerLogs).Methods(http.MethodGet)      // ⭐ NEW

    // ====================================
    // FORMATION PROXY /api/* (no auth)
    // ====================================
    s.router.PathPrefix("/api/{formation_id}/{path:.*}").HandlerFunc(s.proxyHandler.ProxyRequest)
    s.router.PathPrefix("/api/{formation_id}").HandlerFunc(s.proxyHandler.ProxyRequest)
    s.router.HandleFunc("/api", s.handle404).Methods(http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete)

    // ====================================
    // CATCH-ALL (404)
    // ====================================
    s.router.NotFoundHandler = http.HandlerFunc(s.handle404)
}

func (s *Server) handle404(w http.ResponseWriter, r *http.Request) {
    RespondError(w, http.StatusNotFound, "not_found", "Endpoint not found")
}
```

### 3.4 Update Proxy Handler

**File:** `src/pkg/proxy/proxy.go`

Update for new `/api/{formation_id}/*` pattern:

```go
func (h *Handler) ProxyRequest(w http.ResponseWriter, r *http.Request) {
    vars := mux.Vars(r)
    formationID := vars["formation_id"]
    path := vars["path"]
    
    // Get formation
    formation, exists := h.registry.Get(formationID)
    if !exists {
        http.Error(w, "Formation not found", http.StatusNotFound)
        return
    }
    
    // Build target URL
    var targetPath string
    if path != "" {
        targetPath = "/" + path
    } else {
        targetPath = "/"
    }
    
    // Proxy to 127.0.0.1 (formations bind to localhost)
    targetURL := fmt.Sprintf("http://127.0.0.1:%d%s", formation.Port, targetPath)
    
    // ... rest of proxy logic
}
```

### 3.5 Update Deploy Handler

**File:** `src/pkg/api/deploy.go`

Add ID validation:

```go
func (s *Server) HandleDeploy(w http.ResponseWriter, r *http.Request) {
    // ... parse request ...
    
    // Validate formation ID
    if err := registry.ValidateFormationID(req.ID); err != nil {
        RespondError(w, http.StatusBadRequest, "invalid_formation_id", err.Error())
        return
    }
    
    // ... rest of deploy logic ...
}
```

---

## Phase 4: Formation Versioning (60 min)

### 4.1 Version Tracking Structure

**File:** `src/pkg/formation/version.go` (NEW)

```go
package formation

import (
    "encoding/json"
    "os"
    "path/filepath"
    "time"
)

type Version struct {
    Version      int       `json:"version"`
    DeployedAt   time.Time `json:"deployed_at"`
    BundleHash   string    `json:"bundle_hash"`
    BackupPath   string    `json:"backup_path"`
}

type VersionHistory struct {
    CurrentVersion  int      `json:"current_version"`
    PreviousVersion int      `json:"previous_version"`
    Current         *Version `json:"current"`
    Previous        *Version `json:"previous"`
}

func LoadVersionHistory(formationDir string) (*VersionHistory, error) {
    path := filepath.Join(formationDir, "version.json")
    
    if !fileExists(path) {
        // First deployment
        return &VersionHistory{CurrentVersion: 0}, nil
    }
    
    data, err := os.ReadFile(path)
    if err != nil {
        return nil, err
    }
    
    var history VersionHistory
    if err := json.Unmarshal(data, &history); err != nil {
        return nil, err
    }
    
    return &history, nil
}

func (vh *VersionHistory) Save(formationDir string) error {
    path := filepath.Join(formationDir, "version.json")
    data, err := json.MarshalIndent(vh, "", "  ")
    if err != nil {
        return err
    }
    return os.WriteFile(path, data, 0644)
}
```

### 4.2 Update Handler

**File:** `src/pkg/api/update.go` (NEW)

```go
package api

import (
    "net/http"
    "github.com/gorilla/mux"
)

// HandleUpdate handles PUT /rpc/formations/{id}
func (s *Server) HandleUpdate(w http.ResponseWriter, r *http.Request) {
    formationID := mux.Vars(r)["id"]
    
    // Get existing formation
    formation, exists := s.registry.Get(formationID)
    if !exists {
        RespondError(w, http.StatusNotFound, "not_found", "Formation not found")
        return
    }
    
    // Parse bundle upload (same as deploy)
    // ...
    
    // Load version history
    history, err := formation.LoadVersionHistory(formation.Dir)
    if err != nil {
        RespondError(w, http.StatusInternalServerError, "version_error", err.Error())
        return
    }
    
    // Backup current version
    currentDir := filepath.Join(formation.Dir, "current")
    previousDir := filepath.Join(formation.Dir, "previous")
    
    // Remove old previous if exists
    if fileExists(previousDir) {
        os.RemoveAll(previousDir)
    }
    
    // Move current → previous
    if err := os.Rename(currentDir, previousDir); err != nil {
        RespondError(w, http.StatusInternalServerError, "backup_error", err.Error())
        return
    }
    
    // Extract new version to current/
    // ...
    
    // Update version history
    history.PreviousVersion = history.CurrentVersion
    history.Previous = history.Current
    history.CurrentVersion++
    history.Current = &Version{
        Version:    history.CurrentVersion,
        DeployedAt: time.Now(),
        BundleHash: computeHash(bundleData),
        BackupPath: "current",
    }
    history.Save(formation.Dir)
    
    // Restart formation with new version
    s.processManager.Restart(formationID)
    
    RespondSuccess(w, map[string]interface{}{
        "id":      formationID,
        "version": history.CurrentVersion,
        "message": "Formation updated successfully",
    })
}
```

### 4.3 Rollback Handler

**File:** `src/pkg/api/rollback.go` (NEW)

```go
package api

import (
    "net/http"
    "os"
    "path/filepath"
    "github.com/gorilla/mux"
)

// HandleRollback handles POST /rpc/formations/{id}/rollback
func (s *Server) HandleRollback(w http.ResponseWriter, r *http.Request) {
    formationID := mux.Vars(r)["id"]
    
    // Get formation
    formation, exists := s.registry.Get(formationID)
    if !exists {
        RespondError(w, http.StatusNotFound, "not_found", "Formation not found")
        return
    }
    
    // Load version history
    history, err := formation.LoadVersionHistory(formation.Dir)
    if err != nil {
        RespondError(w, http.StatusInternalServerError, "version_error", err.Error())
        return
    }
    
    // Check if previous version exists
    previousDir := filepath.Join(formation.Dir, "previous")
    if !fileExists(previousDir) {
        RespondError(w, http.StatusBadRequest, "no_previous", "No previous version to rollback to")
        return
    }
    
    // Stop formation
    s.processManager.Stop(formationID)
    
    // Swap: current <-> previous
    currentDir := filepath.Join(formation.Dir, "current")
    tempDir := filepath.Join(formation.Dir, "temp")
    
    os.Rename(currentDir, tempDir)
    os.Rename(previousDir, currentDir)
    os.Rename(tempDir, previousDir)
    
    // Swap version history
    tempVersion := history.Current
    history.Current = history.Previous
    history.Previous = tempVersion
    
    tempNum := history.CurrentVersion
    history.CurrentVersion = history.PreviousVersion
    history.PreviousVersion = tempNum
    
    history.Save(formation.Dir)
    
    // Restart with previous version
    s.processManager.Start(formationID)
    
    RespondSuccess(w, map[string]interface{}{
        "id":      formationID,
        "version": history.CurrentVersion,
        "message": "Rolled back to previous version",
    })
}
```

---

## Phase 5: Audit Logging (45 min)

### 5.1 Audit Logger Setup

**File:** `src/pkg/api/audit.go` (NEW)

```go
package api

import (
    "net/http"
    "os"
    "time"
    "github.com/rs/zerolog"
)

type responseWriter struct {
    http.ResponseWriter
    status int
}

func (w *responseWriter) WriteHeader(status int) {
    w.status = status
    w.ResponseWriter.WriteHeader(status)
}

// auditMiddleware logs all /rpc/* requests to audit log
func (s *Server) auditMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        start := time.Now()
        
        // Wrap response writer to capture status
        wrapped := &responseWriter{ResponseWriter: w, status: 200}
        
        // Process request
        next.ServeHTTP(wrapped, r)
        
        // Log to audit log
        s.auditLogger.Info().
            Str("method", r.Method).
            Str("path", r.URL.Path).
            Str("auth_key", getAuthKeyFromRequest(r)).
            Str("remote_addr", r.RemoteAddr).
            Int("status", wrapped.status).
            Dur("duration_ms", time.Since(start)).
            Msg("RPC request")
    })
}

func getAuthKeyFromRequest(r *http.Request) string {
    // Extract key from Authorization header (not secret!)
    auth := r.Header.Get("Authorization")
    // Parse "MUXI-HMAC key=MUXI_abc123, ..."
    // Return just the key
    return "MUXI_abc123" // Simplified
}
```

### 5.2 Setup Audit Logger in Server

**File:** `src/pkg/api/server.go`

```go
type Server struct {
    router         *mux.Router
    httpServer     *http.Server
    config         *config.Config
    processManager *process.Manager
    registry       *registry.Registry
    authMiddleware *auth.Middleware
    proxyHandler   *proxy.Handler
    logger         *zerolog.Logger
    auditLogger    *zerolog.Logger  // ⭐ NEW
}

func NewServer(...) *Server {
    // ... existing setup ...
    
    // Setup audit logger
    auditFile, err := os.OpenFile(
        cfg.Logging.AuditLog,
        os.O_CREATE|os.O_APPEND|os.O_WRONLY,
        0644,
    )
    if err != nil {
        logger.Fatal().Err(err).Msg("Failed to open audit log")
    }
    
    auditLogger := zerolog.New(auditFile).With().Timestamp().Logger()
    
    server := &Server{
        // ...
        auditLogger: &auditLogger,
    }
    
    return server
}
```

### 5.3 Server Logs Handler

**File:** `src/pkg/api/server_logs.go` (NEW)

```go
package api

import (
    "bufio"
    "net/http"
    "os"
    "strconv"
)

// HandleServerLogs handles GET /rpc/server/logs
func (s *Server) HandleServerLogs(w http.ResponseWriter, r *http.Request) {
    // Get lines parameter (default: 100)
    linesStr := r.URL.Query().Get("lines")
    lines := 100
    if linesStr != "" {
        if n, err := strconv.Atoi(linesStr); err == nil {
            lines = n
        }
    }
    
    // Read audit log
    file, err := os.Open(s.config.Logging.AuditLog)
    if err != nil {
        RespondError(w, http.StatusInternalServerError, "log_error", err.Error())
        return
    }
    defer file.Close()
    
    // Read last N lines
    var logLines []string
    scanner := bufio.NewScanner(file)
    for scanner.Scan() {
        logLines = append(logLines, scanner.Text())
        if len(logLines) > lines {
            logLines = logLines[1:] // Keep last N lines
        }
    }
    
    // Return as plain text
    w.Header().Set("Content-Type", "text/plain")
    w.WriteHeader(http.StatusOK)
    for _, line := range logLines {
        w.Write([]byte(line + "\n"))
    }
}
```

### 5.4 Server Status Handler

**File:** `src/pkg/api/server_status.go` (NEW)

```go
package api

import (
    "net/http"
    "runtime"
    "time"
)

var serverStartTime = time.Now()

// HandleServerStatus handles GET /rpc/server/status
func (s *Server) HandleServerStatus(w http.ResponseWriter, r *http.Request) {
    uptime := time.Since(serverStartTime)
    
    // Get formation counts
    formations := s.registry.List()
    runningCount := 0
    stoppedCount := 0
    for _, f := range formations {
        if f.Status == "running" {
            runningCount++
        } else {
            stoppedCount++
        }
    }
    
    // Get port pool status
    available, allocated, total := s.registry.PortPoolStatus()
    
    status := map[string]interface{}{
        "server": map[string]interface{}{
            "id":      s.config.Server.ID,
            "version": "1.0.0", // TODO: Get from build
            "uptime":  int(uptime.Seconds()),
        },
        "formations": map[string]interface{}{
            "total":   len(formations),
            "running": runningCount,
            "stopped": stoppedCount,
        },
        "port_pool": map[string]interface{}{
            "total":     total,
            "available": available,
            "allocated": allocated,
        },
        "runtime": map[string]interface{}{
            "goroutines": runtime.NumGoroutine(),
            "go_version": runtime.Version(),
        },
    }
    
    RespondSuccess(w, status)
}
```

---

## Phase 6: Update Tests (90 min)

### 6.1 Update API Tests

**Files to update:**
- `src/pkg/api/*_test.go`

**Changes:**
1. Port: 3000 → 7890
2. Routes: `/formations/*` → `/rpc/formations/*`
3. Proxy: `/v1/{id}/*` → `/api/{id}/*`

### 6.2 Add New Endpoint Tests

**File:** `src/pkg/api/ping_test.go` (NEW)
**File:** `src/pkg/api/update_test.go` (NEW)
**File:** `src/pkg/api/rollback_test.go` (NEW)
**File:** `src/pkg/api/server_status_test.go` (NEW)
**File:** `src/pkg/api/server_logs_test.go` (NEW)

### 6.3 Add Validation Tests

**File:** `src/pkg/registry/validation_test.go` (NEW)

```go
func TestValidateFormationID(t *testing.T) {
    tests := []struct {
        name    string
        id      string
        wantErr bool
    }{
        {"valid", "my-api", false},
        {"reserved: health", "health", true},
        {"reserved: rpc", "rpc", true},
        {"too short", "ab", true},
        {"uppercase", "My-API", true},
        // ...
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := ValidateFormationID(tt.id)
            if (err != nil) != tt.wantErr {
                t.Errorf("ValidateFormationID(%q) error = %v, wantErr %v", tt.id, err, tt.wantErr)
            }
        })
    }
}
```

### 6.4 Add Versioning Tests

**File:** `src/pkg/formation/version_test.go` (NEW)

Test version history save/load, backup/restore logic.

---

## Phase 7: Documentation Updates (30 min)

### 7.1 Update README

**File:** `README.md`

- Port 3000 → 7890
- Installation section (install script)
- System paths (/etc, /var/lib, /var/log)

### 7.2 Update API Reference

**File:** `docs/api-reference.md`

- All route updates
- Add PUT /rpc/formations/{id}
- Add POST /rpc/formations/{id}/rollback
- Add GET /rpc/server/status
- Add GET /rpc/server/logs
- Remove /rpc/formations/{id}/logs

### 7.3 Update Other Docs

- `docs/getting-started.md` - Installation, system paths
- `docs/configuration.md` - New config options
- `docs/formations.md` - HOST env var
- `docs/installation.md` - Install script usage

---

## Phase 8: Integration Tests (30 min)

### 8.1 Update Test Scripts

**Files:**
- `test/api_test.sh`
- `test/test_auth.sh`
- `test/test_proxy.sh`
- `test/test_management_api.sh`

**Changes:**
- Port 3000 → 7890
- Routes: `/formations/*` → `/rpc/formations/*`
- Proxy: `/v1/*` → `/api/*`

### 8.2 Update Dummy App

**File:** `test/dummy_app.py`

```python
import os
import uvicorn
from fastapi import FastAPI

app = FastAPI()

@app.get("/health")
def health():
    return {"status": "healthy"}

if __name__ == "__main__":
    host = os.getenv("HOST", "127.0.0.1")  # ⭐ NEW
    port = int(os.getenv("PORT", 8000))
    uvicorn.run(app, host=host, port=port)
```

---

## Phase 9: Verification (45 min)

### 9.1 Run All Tests

```bash
cd src

# Unit tests
go test ./... -v

# Coverage
go test ./... -coverprofile=coverage.out
go tool cover -func=coverage.out | grep total

# Should be ≥88.9%
```

### 9.2 Build and Manual Test

```bash
# Build
go build -o ../muxi-server ./cmd/server

# Run locally (development mode)
../muxi-server start --config ./dev-config.yaml

# Test endpoints
curl http://localhost:7890/health
curl http://localhost:7890/ping

# Test deploy (with auth)
curl -X POST http://localhost:7890/rpc/formations \
  -H "Authorization: MUXI-HMAC ..." \
  -d '{"id":"test","command":"python","args":["test/dummy_app.py"]}'

# Test proxy
curl http://localhost:7890/api/test/health

# Test update
curl -X PUT http://localhost:7890/rpc/formations/test \
  -H "Authorization: MUXI-HMAC ..." \
  --data-binary @bundle.tar.gz

# Test rollback
curl -X POST http://localhost:7890/rpc/formations/test/rollback \
  -H "Authorization: MUXI-HMAC ..."

# Test server status
curl http://localhost:7890/rpc/server/status \
  -H "Authorization: MUXI-HMAC ..."

# Test audit logs
curl http://localhost:7890/rpc/server/logs?lines=50 \
  -H "Authorization: MUXI-HMAC ..."
```

### 9.3 Test Reserved IDs

```bash
# Should fail
curl -X POST http://localhost:7890/rpc/formations \
  -d '{"id":"health","command":"python","args":["app.py"]}'
# Error: "formation ID \"health\" is reserved"
```

---

## Implementation Checklist

### Phase 0: Path Migration ✅
- [ ] Update config paths to system paths
- [ ] Update default config
- [ ] Update all tests
- [ ] Run: `go test ./pkg/config/... -v`

### Phase 1: Configuration ✅
- [ ] Update default port to 7890
- [ ] Add BindHost to FormationsConfig
- [ ] Add KeepBackups to FormationsConfig
- [ ] Add LoggingConfig with AuditLog
- [ ] Update config tests
- [ ] Run: `go test ./pkg/config/... -v`

### Phase 2: Environment Variables ✅
- [ ] Update GetEnvironmentVars() signature
- [ ] Add HOST and _bind_host env vars
- [ ] Update all callers
- [ ] Update formation tests
- [ ] Run: `go test ./pkg/formation/... -v`

### Phase 3: RESTful Routing ✅
- [ ] Create ping.go handler
- [ ] Create validation.go with reserved IDs
- [ ] Update setupRoutes() in server.go
- [ ] Update proxy handler for /api/*
- [ ] Update deploy handler with validation
- [ ] Add handle404 method
- [ ] Run: `go test ./pkg/api/... -v`

### Phase 4: Formation Versioning ✅
- [ ] Create version.go with version tracking
- [ ] Create update.go handler (PUT)
- [ ] Create rollback.go handler
- [ ] Update formation directory structure
- [ ] Add version tests
- [ ] Run: `go test ./pkg/formation/... -v`

### Phase 5: Audit Logging ✅
- [ ] Create audit.go with middleware
- [ ] Setup audit logger in server
- [ ] Create server_logs.go handler
- [ ] Create server_status.go handler
- [ ] Add audit tests
- [ ] Run: `go test ./pkg/api/... -v`

### Phase 6: Tests ✅
- [ ] Update all test URLs (port, routes)
- [ ] Create ping_test.go
- [ ] Create validation_test.go
- [ ] Create update_test.go
- [ ] Create rollback_test.go
- [ ] Create server_status_test.go
- [ ] Create server_logs_test.go
- [ ] Run: `go test ./... -v`
- [ ] Check coverage: `go test ./... -cover`

### Phase 7: Documentation ✅
- [ ] Update README.md
- [ ] Update docs/api-reference.md
- [ ] Update docs/getting-started.md
- [ ] Update docs/configuration.md
- [ ] Update docs/formations.md
- [ ] Update docs/installation.md

### Phase 8: Integration Tests ✅
- [ ] Update test/*.sh scripts
- [ ] Update test/dummy_app.py
- [ ] Run integration tests

### Phase 9: Verification ✅
- [ ] All unit tests pass
- [ ] Coverage stays ≥88.9%
- [ ] Build succeeds
- [ ] Manual testing successful
- [ ] Reserved IDs rejected
- [ ] Versioning works
- [ ] Rollback works
- [ ] Audit logging works
- [ ] Server status works

---

## Success Criteria

✅ All unit tests pass  
✅ Test coverage ≥ 88.9%  
✅ Server starts on port 7890  
✅ System paths used (/etc, /var/lib, /var/log)  
✅ `/health` and `/ping` work without auth  
✅ `/rpc/formations/*` routes work (POST, GET, PUT, DELETE)  
✅ `/rpc/formations/{id}/stop|restart|rollback` work  
✅ `/rpc/server/status` and `/rpc/server/logs` work  
✅ `/api/{formation}/*` proxies correctly  
✅ Formations bind to 127.0.0.1  
✅ Reserved IDs are rejected  
✅ Formation versioning works (PUT + rollback)  
✅ Audit logging captures all /rpc/* requests  
✅ Documentation updated  
✅ Integration tests pass  

---

## Timeline Summary

| Phase | Time | Complexity |
|-------|------|------------|
| 0. Path Migration | 30 min | Low |
| 1. Configuration | 30 min | Low |
| 2. Environment Vars | 20 min | Low |
| 3. RESTful Routing | 90 min | Medium |
| 4. Formation Versioning | 60 min | Medium |
| 5. Audit Logging | 45 min | Medium |
| 6. Tests | 90 min | Medium |
| 7. Documentation | 30 min | Low |
| 8. Integration Tests | 30 min | Low |
| 9. Verification | 45 min | Low |
| **Total** | **~7 hours** | **Medium** |

---

**Note:** Install script and interactive init are separate tasks (tracked in GitHub issue).

**Ready to implement?**
