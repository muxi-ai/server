# API Architecture Implementation Plan

**Goal:** Migrate from current API structure to new architecture

**Timeline:** ~2-3 hours of focused work  
**Risk Level:** Medium (routing changes affect all endpoints)  
**Test Coverage:** Must maintain 88.9%+ test coverage

---

## Current State vs Target State

### Current API Routes:
```
Port: 3000 (default)

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
Port: 7890 (new default)

/health                        → Health check (no auth)
/ping                          → Ping (no auth) ← NEW
/rpc/deploy                    → Deploy (HMAC auth)
/rpc/formations                → List (HMAC auth)
/rpc/formations/{id}           → Get (HMAC auth)
/rpc/stop/{id}                 → Stop (HMAC auth)
/rpc/restart/{id}              → Restart (HMAC auth)
/rpc/delete/{id}               → Delete (HMAC auth)
/rpc/logs/{id}                 → Logs (HMAC auth)
/api/{formation_id}/*          → Proxy (no auth)
/api                           → 404 (formation ID required)
/*                             → 404 (catch-all)
```

---

## Phase 1: Configuration Changes (30 min)

### 1.1 Update Default Port

**File:** `src/pkg/config/config.go`

**Current:**
```go
Port int `yaml:"port"` // HTTP server port (default: 3000)
```

**Change to:**
```go
Port int `yaml:"port"` // HTTP server port (default: 7890)
```

**Also update default config:**
```go
func DefaultConfig() *Config {
    return &Config{
        Server: ServerConfig{
            Host: "0.0.0.0",
            Port: 7890,  // Changed from 3000
        },
        // ...
    }
}
```

### 1.2 Add Formation Bind Host Configuration

**File:** `src/pkg/config/config.go`

**Add to FormationsConfig:**
```go
type FormationsConfig struct {
    PortRangeStart int    `yaml:"port_range_start"`
    PortRangeEnd   int    `yaml:"port_range_end"`
    BindHost       string `yaml:"bind_host"` // ← ADD THIS
    LogsDir        string `yaml:"logs_dir"`
    // ...
}
```

**Update defaults:**
```go
Formations: FormationsConfig{
    PortRangeStart: 8000,
    PortRangeEnd:   9000,
    BindHost:       "127.0.0.1", // ← ADD THIS
    LogsDir:        filepath.Join(configDir, "logs"),
    // ...
}
```

### 1.3 Update Config Tests

**File:** `src/pkg/config/config_test.go`

**Update tests expecting port 3000:**
```go
// Find and update all:
// - expectedPort := 3000
// - if cfg.Server.Port != 3000
// - port: 3000

// Change to 7890
```

**Add test for BindHost:**
```go
func TestDefaultConfig_BindHost(t *testing.T) {
    cfg := config.DefaultConfig()
    if cfg.Formations.BindHost != "127.0.0.1" {
        t.Errorf("BindHost = %s, want 127.0.0.1", cfg.Formations.BindHost)
    }
}
```

---

## Phase 2: Formation Environment Variables (20 min)

### 2.1 Update Formation Environment Variables

**File:** `src/pkg/formation/formation.go`

**Current:**
```go
func (f *Formation) GetEnvironmentVars(port int, serverURL string) map[string]string {
    env := make(map[string]string)
    env["PORT"] = fmt.Sprintf("%d", port)
    env["FORMATION_ID"] = f.ID
    env["MUXI_SERVER_URL"] = serverURL
    env["MUXI_ENV"] = "production"
    return env
}
```

**Change to:**
```go
func (f *Formation) GetEnvironmentVars(port int, serverURL string, bindHost string) map[string]string {
    env := make(map[string]string)
    
    // Network binding (CRITICAL for security)
    env["PORT"] = fmt.Sprintf("%d", port)
    env["HOST"] = bindHost  // ← ADD THIS
    
    // Formation metadata
    env["FORMATION_ID"] = f.ID
    env["MUXI_SERVER_URL"] = serverURL
    env["MUXI_ENV"] = "production"
    
    // Metadata for formation.yaml injection
    env["_bind_host"] = bindHost  // ← ADD THIS
    env["_port"] = fmt.Sprintf("%d", port)
    
    return env
}
```

### 2.2 Update Manager to Pass BindHost

**File:** `src/pkg/process/manager.go`

**Find all calls to `GetEnvironmentVars()` and add bindHost parameter:**

```go
// Before:
formationEnv := formation.GetEnvironmentVars(port, serverURL)

// After:
formationEnv := formation.GetEnvironmentVars(port, serverURL, m.config.Formations.BindHost)
```

### 2.3 Update Formation Tests

**File:** `src/pkg/formation/formation_test.go`

**Update test calls:**
```go
// Before:
env := f.GetEnvironmentVars(8080, "http://localhost:3000")

// After:
env := f.GetEnvironmentVars(8080, "http://localhost:7890", "127.0.0.1")

// Add assertions:
if env["HOST"] != "127.0.0.1" {
    t.Errorf("HOST = %q, want 127.0.0.1", env["HOST"])
}
if env["_bind_host"] != "127.0.0.1" {
    t.Errorf("_bind_host = %q, want 127.0.0.1", env["_bind_host"])
}
```

---

## Phase 3: Routing Restructure (60 min)

### 3.1 Add Ping Handler

**File:** `src/pkg/api/ping.go` (NEW)

```go
package api

import "net/http"

// HandlePing handles GET /ping
// Returns "pong" for simple connectivity testing
func (s *Server) HandlePing(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "text/plain")
    w.WriteHeader(http.StatusOK)
    w.Write([]byte("pong"))
}
```

### 3.2 Update Server Routes

**File:** `src/pkg/api/server.go`

**Replace `setupRoutes()` method:**

```go
func (s *Server) setupRoutes() {
    // Add logging middleware (all routes)
    s.router.Use(s.loggingMiddleware)

    // Add CORS middleware (all routes, for development)
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

    // Formation management
    rpc.HandleFunc("/deploy", s.HandleDeploy).Methods(http.MethodPost)
    rpc.HandleFunc("/formations", s.HandleList).Methods(http.MethodGet)
    rpc.HandleFunc("/formations/{id}", s.HandleGet).Methods(http.MethodGet)
    rpc.HandleFunc("/stop/{id}", s.HandleStop).Methods(http.MethodPost)
    rpc.HandleFunc("/restart/{id}", s.HandleRestart).Methods(http.MethodPost)
    rpc.HandleFunc("/delete/{id}", s.HandleDelete).Methods(http.MethodDelete)
    rpc.HandleFunc("/logs/{id}", s.HandleLogs).Methods(http.MethodGet)

    // ====================================
    // FORMATION PROXY /api/* (no auth)
    // ====================================
    // Pattern: /api/{formation_id}/*
    // Example: /api/my-api/v1/chat → http://127.0.0.1:8001/v1/chat
    
    // With path (most routes)
    s.router.PathPrefix("/api/{formation_id}/{path:.*}").HandlerFunc(s.proxyHandler.ProxyRequest)
    
    // Without path (root endpoint)
    s.router.PathPrefix("/api/{formation_id}").HandlerFunc(s.proxyHandler.ProxyRequest)
    
    // /api with no formation ID → 404
    s.router.HandleFunc("/api", s.handle404).Methods(http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete)

    // ====================================
    // CATCH-ALL (404)
    // ====================================
    s.router.NotFoundHandler = http.HandlerFunc(s.handle404)
}

// handle404 returns a 404 error
func (s *Server) handle404(w http.ResponseWriter, r *http.Request) {
    RespondError(w, http.StatusNotFound, "not_found", "Endpoint not found")
}
```

### 3.3 Update Proxy Handler

**File:** `src/pkg/proxy/proxy.go`

**Update route variable extraction:**

```go
// Before:
formationID := mux.Vars(r)["formation_id"]
path := mux.Vars(r)["path"]

// After:
vars := mux.Vars(r)
formationID := vars["formation_id"]
path := vars["path"]

// Path may be empty for root requests like /api/my-formation
// That's OK - proxy to formation root
```

**Update URL construction:**

```go
// Build target URL
var targetPath string
if path != "" {
    targetPath = "/" + path
} else {
    // Root endpoint: /api/my-formation → formation:port/
    targetPath = "/"
}

targetURL := fmt.Sprintf("http://127.0.0.1:%d%s", formation.Port, targetPath)
```

### 3.4 Add Reserved Formation IDs Validation

**File:** `src/pkg/registry/validation.go` (NEW)

```go
package registry

import (
    "fmt"
    "regexp"
)

var (
    // Reserved formation IDs that cannot be used (conflict with server routes)
    reservedIDs = map[string]bool{
        "health":  true,
        "ping":    true,
        "rpc":     true,
        "server":  true,
        "admin":   true,
        "metrics": true,
        "api":     true,
    }

    // Formation ID pattern: lowercase letters, numbers, hyphens, 3-50 chars
    formationIDPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,48}[a-z0-9]$`)
)

// ValidateFormationID checks if a formation ID is valid
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

### 3.5 Update Deploy Handler to Validate ID

**File:** `src/pkg/api/deploy.go`

**Add validation:**

```go
// Validate formation ID
if err := registry.ValidateFormationID(req.ID); err != nil {
    RespondError(w, http.StatusBadRequest, "invalid_formation_id", err.Error())
    return
}
```

---

## Phase 4: Update Tests (45 min)

### 4.1 Update API Tests

**Files to update:**
- `src/pkg/api/api_test.go`
- `src/pkg/api/handlers_test.go`
- `src/pkg/api/deploy_test.go`
- `src/pkg/api/middleware_test.go`
- `src/pkg/api/bundle_test.go`

**Changes needed:**

1. **Port changes:**
   ```go
   // Before:
   "http://localhost:3000/v1/test-api"
   
   // After:
   "http://localhost:7890/api/test-api"
   ```

2. **Route changes:**
   ```go
   // Before:
   req := httptest.NewRequest("POST", "/formations/deploy", body)
   
   // After:
   req := httptest.NewRequest("POST", "/rpc/deploy", body)
   ```

3. **Proxy path changes:**
   ```go
   // Before:
   "/v1/my-api/endpoint"
   
   // After:
   "/api/my-api/endpoint"
   ```

### 4.2 Add Ping Tests

**File:** `src/pkg/api/ping_test.go` (NEW)

```go
package api

import (
    "net/http"
    "net/http/httptest"
    "testing"
)

func TestHandlePing(t *testing.T) {
    server := setupTestServer(t)

    req := httptest.NewRequest("GET", "/ping", nil)
    w := httptest.NewRecorder()

    server.router.ServeHTTP(w, req)

    if w.Code != http.StatusOK {
        t.Errorf("Status = %d, want %d", w.Code, http.StatusOK)
    }

    body := w.Body.String()
    if body != "pong" {
        t.Errorf("Body = %q, want %q", body, "pong")
    }

    contentType := w.Header().Get("Content-Type")
    if contentType != "text/plain" {
        t.Errorf("Content-Type = %q, want text/plain", contentType)
    }
}

func TestHandlePing_NoAuth(t *testing.T) {
    server := setupTestServer(t)

    // Ping should work without auth
    req := httptest.NewRequest("GET", "/ping", nil)
    w := httptest.NewRecorder()

    server.router.ServeHTTP(w, req)

    if w.Code != http.StatusOK {
        t.Errorf("Ping without auth: Status = %d, want %d", w.Code, http.StatusOK)
    }
}
```

### 4.3 Add Reserved ID Tests

**File:** `src/pkg/registry/validation_test.go` (NEW)

```go
package registry

import "testing"

func TestValidateFormationID(t *testing.T) {
    tests := []struct {
        name    string
        id      string
        wantErr bool
    }{
        {"valid lowercase", "my-api", false},
        {"valid with numbers", "api-v2", false},
        {"valid long", "my-long-formation-name-here", false},
        {"reserved: health", "health", true},
        {"reserved: ping", "ping", true},
        {"reserved: rpc", "rpc", true},
        {"reserved: api", "api", true},
        {"too short", "ab", true},
        {"too long", "this-is-a-very-long-formation-id-that-exceeds-fifty-characters", true},
        {"uppercase", "My-API", true},
        {"spaces", "my api", true},
        {"underscore", "my_api", true},
        {"empty", "", true},
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

### 4.4 Update Formation Tests

**File:** `src/pkg/formation/formation_test.go`

**Update port and add HOST checks:**

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
        {"FORMATION_ID", "test-api"},
        {"MUXI_SERVER_URL", "http://localhost:7890"},
        {"_bind_host", "127.0.0.1"},
        {"_port", "8080"},
    }
    
    for _, tt := range tests {
        if env[tt.key] != tt.want {
            t.Errorf("%s = %q, want %q", tt.key, env[tt.key], tt.want)
        }
    }
}
```

### 4.5 Update Proxy Tests

**File:** `src/pkg/proxy/proxy_test.go`

**Update route patterns:**

```go
// Before:
req := httptest.NewRequest("GET", "/v1/test-api/endpoint", nil)

// After:
req := httptest.NewRequest("GET", "/api/test-api/endpoint", nil)
```

---

## Phase 5: Documentation Updates (20 min)

### 5.1 Update README

**File:** `README.md`

**Update Quick Start examples:**

```bash
# Before:
curl http://localhost:3000/health

# After:
curl http://localhost:7890/health
```

### 5.2 Update Getting Started Guide

**File:** `docs/getting-started.md`

**Update all URLs:**
- Port 3000 → 7890
- `/formations/*` → `/rpc/*`
- `/v1/{formation}/*` → `/api/{formation}/*`

### 5.3 Update Other Docs

**Files:**
- `docs/configuration.md` - Port defaults
- `docs/formations.md` - Environment variables (add HOST)
- `docs/troubleshooting.md` - Port references

---

## Phase 6: Integration Tests (20 min)

### 6.1 Update Test Scripts

**Files:**
- `test/api_test.sh`
- `test/test_auth.sh`
- `test/test_proxy.sh`
- `test/test_management_api.sh`

**Changes:**

```bash
# Before:
BASE_URL="http://localhost:3000"
curl $BASE_URL/formations/deploy

# After:
BASE_URL="http://localhost:7890"
curl $BASE_URL/rpc/deploy
```

### 6.2 Update Test Formation

**File:** `test/dummy_app.py`

**Update to read HOST env var:**

```python
import os
import uvicorn
from fastapi import FastAPI

app = FastAPI()

@app.get("/health")
def health():
    return {"status": "healthy"}

@app.get("/chat")
def chat():
    return {"message": "Hello from dummy app!"}

if __name__ == "__main__":
    host = os.getenv("HOST", "127.0.0.1")  # ← ADD THIS
    port = int(os.getenv("PORT", 8000))
    
    print(f"Starting dummy app on {host}:{port}")
    uvicorn.run(app, host=host, port=port)
```

---

## Phase 7: Verification & Testing (30 min)

### 7.1 Run All Tests

```bash
cd src

# Run all unit tests
go test ./... -v

# Check coverage (should stay at 88.9%+)
go test ./... -coverprofile=coverage.out
go tool cover -func=coverage.out | grep total
```

### 7.2 Build and Manual Test

```bash
# Build
go build -o ../muxi-server ./cmd/server

# Run server
../muxi-server start

# Test endpoints (in another terminal)
curl http://localhost:7890/health
curl http://localhost:7890/ping

# Test deploy (with auth)
curl -X POST http://localhost:7890/rpc/deploy \
  -H "Authorization: MUXI-HMAC ..." \
  -d '{"id":"test","command":"python","args":["test/dummy_app.py"]}'

# Test proxy
curl http://localhost:7890/api/test/health
```

### 7.3 Test Reserved IDs

```bash
# Should fail with "reserved" error
curl -X POST http://localhost:7890/rpc/deploy \
  -H "Authorization: MUXI-HMAC ..." \
  -d '{"id":"health","command":"python","args":["app.py"]}'

curl -X POST http://localhost:7890/rpc/deploy \
  -H "Authorization: MUXI-HMAC ..." \
  -d '{"id":"rpc","command":"python","args":["app.py"]}'
```

### 7.4 Test 404 Routes

```bash
# Should return 404
curl http://localhost:7890/api
curl http://localhost:7890/random-endpoint
curl http://localhost:7890/rpc/invalid
```

---

## Implementation Checklist

### Phase 1: Configuration ✅
- [ ] Update default port to 7890
- [ ] Add BindHost to FormationsConfig
- [ ] Update config tests
- [ ] Run: `go test ./pkg/config/... -v`

### Phase 2: Environment Variables ✅
- [ ] Update GetEnvironmentVars() signature
- [ ] Add HOST and _bind_host env vars
- [ ] Update Manager calls
- [ ] Update formation tests
- [ ] Run: `go test ./pkg/formation/... -v`

### Phase 3: Routing ✅
- [ ] Create ping.go handler
- [ ] Update setupRoutes() in server.go
- [ ] Update proxy handler for new paths
- [ ] Create validation.go with reserved IDs
- [ ] Update deploy.go to validate IDs
- [ ] Add handle404 method
- [ ] Run: `go test ./pkg/api/... -v`

### Phase 4: Tests ✅
- [ ] Update all test URLs (3000 → 7890)
- [ ] Update all test routes (/formations → /rpc, /v1 → /api)
- [ ] Create ping_test.go
- [ ] Create validation_test.go
- [ ] Update formation tests for HOST
- [ ] Update proxy tests for new paths
- [ ] Run: `go test ./... -v`
- [ ] Check coverage: `go test ./... -cover`

### Phase 5: Documentation ✅
- [ ] Update README.md
- [ ] Update docs/getting-started.md
- [ ] Update docs/configuration.md
- [ ] Update docs/formations.md
- [ ] Update docs/troubleshooting.md

### Phase 6: Integration Tests ✅
- [ ] Update test/*.sh scripts
- [ ] Update test/dummy_app.py
- [ ] Run integration tests

### Phase 7: Verification ✅
- [ ] All unit tests pass
- [ ] Coverage stays at 88.9%+
- [ ] Build succeeds
- [ ] Manual testing successful
- [ ] Reserved IDs rejected
- [ ] 404 routes work correctly

---

## Rollback Plan

If issues arise:

```bash
# Rollback changes
git diff > api-refactor.patch
git reset --hard HEAD

# Or revert specific commits
git revert <commit-hash>
```

**Keep old routes temporarily?**

If we want backward compatibility, we could add deprecated route aliases:

```go
// DEPRECATED: Old routes (will be removed in v2.0)
deprecated := s.router.PathPrefix("/formations").Subrouter()
deprecated.Use(s.deprecationWarningMiddleware)
deprecated.Use(s.authMiddleware.Authenticate)
deprecated.HandleFunc("/deploy", s.HandleDeploy).Methods(http.MethodPost)
// ... etc
```

---

## Risk Assessment

| Risk | Impact | Mitigation |
|------|--------|------------|
| Breaking API clients | High | Add deprecated routes temporarily |
| Test coverage drops | Medium | Run coverage checks after each phase |
| Port conflicts | Low | 7890 rarely used |
| Formation binding fails | Medium | Test with dummy_app.py extensively |
| Reserved ID edge cases | Low | Comprehensive validation tests |

---

## Estimated Timeline

| Phase | Time | Complexity |
|-------|------|------------|
| 1. Configuration | 30 min | Low |
| 2. Environment Vars | 20 min | Low |
| 3. Routing | 60 min | Medium |
| 4. Tests | 45 min | Medium |
| 5. Documentation | 20 min | Low |
| 6. Integration Tests | 20 min | Low |
| 7. Verification | 30 min | Low |
| **Total** | **~3.5 hours** | **Medium** |

---

## Success Criteria

✅ All unit tests pass  
✅ Test coverage ≥ 88.9%  
✅ Server starts on port 7890  
✅ `/health` and `/ping` work without auth  
✅ `/rpc/*` routes require HMAC auth  
✅ `/api/{formation}/*` proxies correctly  
✅ Formations bind to 127.0.0.1  
✅ Reserved IDs are rejected  
✅ 404 routes work correctly  
✅ Documentation updated  
✅ Integration tests pass  

---

**Ready to implement?** Let me know if you want me to start, or if you'd like to review/adjust the plan first!
