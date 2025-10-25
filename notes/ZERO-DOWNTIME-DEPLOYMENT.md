# Zero-Downtime Deployment Strategy

**Feature:** Blue-Green Deployment for Formation Updates  
**Status:** Implementation Ready  
**Priority:** High - Production Reliability Feature  
**Target:** Phase 1.5 (Post-MVP Enhancement)

---

## Problem Statement

Currently, when updating a formation to a new version:
1. We stop the old version
2. Deploy the new version
3. Start the new version
4. **Risk:** If the new version fails to start or has runtime issues (bad config, missing secrets, syntax errors), we experience downtime until rollback

**Real-world scenarios that cause downtime:**
- Missing environment variables/secrets in new version
- Bad YAML indentation in `formation.yaml`
- Python syntax errors in new code
- Port binding conflicts
- Missing dependencies in new requirements.txt

---

## Solution: Blue-Green Deployment

### High-Level Strategy

**Keep the old version running until the new version proves healthy.**

```
┌───────────────────────────────────────────────────────────────┐
│ Current Flow (Risky - Downtime on Failure)                    │
├───────────────────────────────────────────────────────────────┤
│ 1. Stop old version (port 8001)                               │
│ 2. Deploy new version files                                   │
│ 3. Start new version on port 8001                             │
│ 4. ❌ If fails → DOWNTIME until manual rollback               │
└───────────────────────────────────────────────────────────────┘

┌───────────────────────────────────────────────────────────────┐
│ New Flow (Zero Downtime - Blue-Green)                         │
├───────────────────────────────────────────────────────────────┤
│ 1. Old version running on port 8001 (serving traffic)         │
│ 2. Deploy new version files to staging/                       │
│ 3. Allocate NEW port 8002 for new version                     │
│ 4. Start new version on port 8002 (staging)                   │
│ 5. Health check new version (30 second timeout)               │
│ 6. Decision:                                                   │
│    ✅ If healthy:                                              │
│       - Update proxy: /api/{id}/* → 8002                       │
│       - Stop old version on 8001                               │
│       - Force kill if doesn't stop gracefully                  │
│       - Release port 8001 back to pool                         │
│       - Move staging/ → current/, old current/ → previous/     │
│       - Return success                                         │
│    ❌ If unhealthy:                                            │
│       - Stop new version on 8002                               │
│       - Force kill if doesn't stop                             │
│       - Release port 8002 back to pool                         │
│       - Clean up staging/ directory                            │
│       - Old version STILL SERVING on 8001                      │
│       - Return error with diagnostics                          │
└───────────────────────────────────────────────────────────────┘
```

---

## Implementation Architecture

### 1. Enhanced Formation Struct

```go
// pkg/registry/formation.go
type Formation struct {
    // ... existing fields ...
    
    Port        int  `json:"port"`         // Current active port (serving traffic)
    StagingPort int  `json:"staging_port"` // Port for incoming version (0 if none)
    Deploying   bool `json:"deploying"`    // True during blue-green deployment
}
```

### 2. New Health Checker Component

```go
// pkg/process/health.go (NEW FILE)
type HealthChecker struct {
    Endpoint string        // default: "/health"
    Timeout  time.Duration // default: 30s
    Interval time.Duration // default: 1s
    MaxRetries int         // default: 30 attempts
}

// WaitForHealthy polls the formation's health endpoint until healthy or timeout
func (hc *HealthChecker) WaitForHealthy(port int) error {
    deadline := time.Now().Add(hc.Timeout)
    attempt := 0
    
    for time.Now().Before(deadline) && attempt < hc.MaxRetries {
        if err := hc.checkHealth(port); err == nil {
            return nil // Healthy!
        }
        
        attempt++
        time.Sleep(hc.Interval)
    }
    
    return fmt.Errorf("health check failed after %d attempts", hc.MaxRetries)
}

func (hc *HealthChecker) checkHealth(port int) error {
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    
    url := fmt.Sprintf("http://127.0.0.1:%d%s", port, hc.Endpoint)
    req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
    if err != nil {
        return err
    }
    
    resp, err := http.DefaultClient.Do(req)
    if err != nil {
        return err
    }
    defer resp.Body.Close()
    
    if resp.StatusCode >= 200 && resp.StatusCode < 300 {
        return nil
    }
    
    return fmt.Errorf("unhealthy: status %d", resp.StatusCode)
}
```

### 3. Enhanced Port Pool

**No changes needed!** Current implementation already supports:
- Allocating multiple ports
- Releasing ports back to pool
- Thread-safe operations with mutex

### 4. Updated Deployment Flow

```go
// pkg/api/update.go - HandleUpdate() refactored

func (s *Server) HandleUpdate(w http.ResponseWriter, r *http.Request) {
    formationID := mux.Vars(r)["id"]
    
    // 1. Get existing formation
    existingFormation, err := s.registry.Get(formationID)
    if err != nil {
        RespondError(w, http.StatusNotFound, "Formation not found")
        return
    }
    
    // 2. Mark as deploying (prevents concurrent updates)
    if err := s.registry.SetDeploying(formationID, true); err != nil {
        RespondError(w, http.StatusConflict, "Formation is already being updated")
        return
    }
    defer s.registry.SetDeploying(formationID, false)
    
    // 3. Save uploaded bundle to temp file
    tmpFile, bundleData := saveBundle(r.Body)
    defer cleanupTemp(tmpFile)
    
    // 4. Allocate NEW port for staging version
    stagingPort, err := s.registry.AllocatePort(formationID + "-staging")
    if err != nil {
        RespondError(w, http.StatusInsufficientStorage, "No available ports")
        return
    }
    
    // 5. Extract bundle to staging directory
    stagingDir := filepath.Join(formationBaseDir, "staging")
    if err := extractBundleToStaging(tmpFile, stagingDir); err != nil {
        s.registry.ReleasePort(stagingPort)
        RespondError(w, http.StatusBadRequest, "Failed to extract bundle")
        return
    }
    
    // 6. Start staging process on new port
    stagingProc, err := s.processManager.Start(SpawnConfig{
        ID:      formationID + "-staging",
        Port:    stagingPort,
        WorkDir: stagingDir,
        // ... other config ...
    })
    if err != nil {
        cleanup(stagingDir, stagingPort)
        RespondError(w, http.StatusInternalServerError, "Failed to start staging")
        return
    }
    
    // 7. Health check staging version (30 second timeout)
    healthChecker := NewHealthChecker(30*time.Second, 1*time.Second)
    if err := healthChecker.WaitForHealthy(stagingPort); err != nil {
        // FAILURE PATH: Staging unhealthy
        s.processManager.ForceKill(formationID + "-staging")
        cleanup(stagingDir, stagingPort)
        
        s.logger.Error().
            Str("formation_id", formationID).
            Err(err).
            Msg("Staging health check failed - keeping old version")
        
        RespondError(w, http.StatusBadRequest, 
            fmt.Sprintf("New version failed health check: %v. Old version still running.", err))
        return
    }
    
    // 8. SUCCESS PATH: Staging is healthy!
    
    // 8a. Update registry: switch active port
    s.registry.SwitchPort(formationID, stagingPort)
    
    // 8b. Stop old version (force kill if necessary)
    oldPort := existingFormation.Port
    if err := s.processManager.Stop(formationID); err != nil {
        s.processManager.ForceKill(formationID)
    }
    
    // 8c. Release old port
    s.registry.ReleasePort(oldPort)
    
    // 8d. Move directories: staging → current, old current → previous
    moveVersionDirectories(formationBaseDir)
    
    // 8e. Update version metadata
    updateVersionHistory(formationBaseDir, bundleData)
    
    // 8f. Rename staging process to primary
    s.processManager.Rename(formationID+"-staging", formationID)
    
    s.logger.Info().
        Str("formation_id", formationID).
        Int("old_port", oldPort).
        Int("new_port", stagingPort).
        Msg("Zero-downtime deployment successful")
    
    RespondSuccess(w, map[string]interface{}{
        "id":      formationID,
        "status":  "running",
        "port":    stagingPort,
        "message": "Formation updated with zero downtime",
    })
}
```

---

## Configuration

```yaml
# ~/.muxi-server/config.yaml
formations:
  # ... existing config ...
  
  # Zero-downtime deployment settings
  deployment:
    health_check:
      enabled: true              # Enable health checks during deployment
      endpoint: "/health"        # Formation health endpoint
      timeout: 30                # Total timeout in seconds
      interval: 1                # Poll interval in seconds
      max_retries: 30            # Max health check attempts
    
    force_kill_timeout: 5        # Seconds to wait before force-killing old version
```

---

## Edge Cases & Handling

### 1. Port Exhaustion
**Scenario:** No available ports for staging version  
**Handling:**
- Return HTTP 507 Insufficient Storage
- Old version continues serving
- User must free up ports or increase range

### 2. Health Check False Positives
**Scenario:** `/health` returns 200 but formation is broken  
**Mitigation:**
- Configurable health endpoint (allow custom paths)
- Future: Allow custom validation scripts
- User can still rollback manually if issues discovered later

### 3. Proxy Race Conditions
**Scenario:** Requests arrive during port switch  
**Handling:**
- Use mutex lock in registry during port update
- Atomic port switch in Formation struct
- **Accept ~1 second downtime** (as per user feedback - "we can afford it")

### 4. Old Process Won't Stop
**Scenario:** Old formation hangs on SIGTERM  
**Handling:**
- Wait `force_kill_timeout` (5 seconds)
- Send SIGKILL (force kill)
- Log warning if force kill fails
- Continue anyway (we can't block new version)

### 5. Concurrent Updates
**Scenario:** Two clients try to update same formation simultaneously  
**Handling:**
- Use `Deploying` flag in Formation struct
- First update sets flag, second gets HTTP 409 Conflict
- Flag cleared after deployment (success or failure)

### 6. Extraction/Parse Errors
**Scenario:** Bundle is malformed, YAML syntax error  
**Handling:**
- Fail BEFORE allocating staging port
- No cleanup needed
- Old version never affected

### 7. Startup Crash
**Scenario:** New version starts but crashes immediately  
**Handling:**
- Health check will fail (no HTTP response)
- Auto-cleanup triggered
- Old version still serving

---

## Testing Strategy

### Unit Tests

```go
// pkg/process/health_test.go
func TestHealthChecker_Success(t *testing.T) {
    // Mock HTTP server returning 200 OK
    // Verify WaitForHealthy() succeeds quickly
}

func TestHealthChecker_Timeout(t *testing.T) {
    // Mock HTTP server returning 500
    // Verify WaitForHealthy() times out after 30s
}

func TestHealthChecker_SlowStart(t *testing.T) {
    // Mock server: return 500 for 5s, then 200
    // Verify WaitForHealthy() waits and eventually succeeds
}
```

### Integration Tests

```go
// pkg/api/update_test.go
func TestUpdate_ZeroDowntime_Success(t *testing.T) {
    // 1. Deploy v1 formation
    // 2. Verify v1 serving on /api/{id}/
    // 3. Deploy v2 (healthy)
    // 4. Verify v2 serving on /api/{id}/
    // 5. Verify v1 process stopped
    // 6. Verify port released
}

func TestUpdate_ZeroDowntime_UnhealthyStaging(t *testing.T) {
    // 1. Deploy v1 formation
    // 2. Verify v1 serving
    // 3. Deploy v2 with broken /health endpoint
    // 4. Verify update returns error
    // 5. Verify v1 STILL serving (zero downtime)
    // 6. Verify v2 process killed
    // 7. Verify staging port released
}

func TestUpdate_ZeroDowntime_MalformedBundle(t *testing.T) {
    // 1. Deploy v1 formation
    // 2. Deploy v2 with bad YAML
    // 3. Verify update fails early (before staging)
    // 4. Verify v1 still running
}
```

### Manual Testing

```bash
# Test 1: Successful deployment
curl -X POST http://localhost:7890/rpc/formations/deploy \
  -H "Content-Type: application/gzip" \
  --data-binary @v1.tar.gz

# Verify v1 running
curl http://localhost:7890/api/test-formation/health
# → 200 OK (v1)

# Deploy v2
curl -X PUT http://localhost:7890/rpc/formations/test-formation \
  -H "Content-Type: application/gzip" \
  --data-binary @v2.tar.gz

# Verify v2 running
curl http://localhost:7890/api/test-formation/health
# → 200 OK (v2)

# Check logs
tail -f ~/.muxi/server/logs/audit.log
# → Should show: "Zero-downtime deployment successful"


# Test 2: Failed deployment (broken health)
# Create v3 with broken /health endpoint
# Deploy v3
curl -X PUT http://localhost:7890/rpc/formations/test-formation \
  -H "Content-Type: application/gzip" \
  --data-binary @v3-broken.tar.gz
# → Should return HTTP 400 with error

# Verify v2 STILL running
curl http://localhost:7890/api/test-formation/health
# → 200 OK (v2 - zero downtime achieved!)
```

---

## Benefits

✅ **Zero Downtime** - Old version serves traffic during deployment  
✅ **Automatic Validation** - New version must pass health check  
✅ **Instant Rollback** - Failed deployments don't affect running service  
✅ **Better Error Reporting** - Users know immediately if deployment failed  
✅ **Production-Grade** - Industry-standard deployment pattern  
✅ **Simple Implementation** - ~200 lines of new code, minimal changes  
✅ **No Breaking Changes** - Existing deployments work as-is  

---

## Risks & Mitigations

| Risk | Impact | Mitigation |
|------|--------|------------|
| Port exhaustion | Can't deploy | Clear error message, suggest increasing port range |
| Health check false positives | Deploy broken code | Allow custom health endpoints, manual rollback |
| ~1 second downtime during switch | Brief interruption | Acceptable per user feedback |
| Old process hangs | Port not released | Force kill after timeout |
| Concurrent updates | Race conditions | Lock with `Deploying` flag |

---

## Implementation Phases

### Phase 1: Core Infrastructure ✅
- [x] Document feature in notes/
- [ ] Implement `pkg/process/health.go` (HealthChecker)
- [ ] Add `StagingPort` and `Deploying` fields to Formation
- [ ] Add configuration for health check settings

### Phase 2: Deployment Flow
- [ ] Refactor `HandleUpdate()` to use blue-green pattern
- [ ] Add force-kill logic for old processes
- [ ] Add staging directory management
- [ ] Add port switching logic

### Phase 3: Testing & Polish
- [ ] Write unit tests for HealthChecker
- [ ] Write integration tests for zero-downtime flow
- [ ] Update API documentation
- [ ] Update user guide (docs/formations.md)

### Phase 4: Optional Enhancements (Future)
- [ ] Custom health check scripts
- [ ] Configurable health validation logic
- [ ] Deployment metrics (time to healthy, failure rate)
- [ ] Notification hooks (webhook on deployment success/failure)

---

## Timeline

- **Core Implementation:** 4-6 hours
- **Testing:** 2-3 hours
- **Documentation:** 1-2 hours
- **Total:** ~8-11 hours (1-2 days)

---

## Future Enhancements

1. **Canary Deployments**
   - Route 10% traffic to new version
   - Monitor error rates
   - Gradually increase traffic

2. **Custom Health Scripts**
   - Allow formations to define custom health checks
   - Run scripts in formation environment
   - More robust validation

3. **Deployment Webhooks**
   - Notify external systems on deployment events
   - Integrate with Slack, PagerDuty, etc.

4. **Deployment Metrics**
   - Track deployment success/failure rates
   - Measure time to healthy
   - Alert on deployment failures

---

## Conclusion

This zero-downtime deployment feature transforms MUXI Server from a basic process manager into a **production-grade orchestration platform**. It's a killer feature that:

- Eliminates deployment risk
- Provides automatic validation
- Follows industry best practices (blue-green deployment)
- Requires minimal code changes (~300 lines total)
- Has no breaking changes to existing API

**Recommendation: IMPLEMENT IMMEDIATELY** - This is a must-have for production deployments and differentiates MUXI from simpler process managers.

---

**Status:** Ready for Implementation  
**Approved:** Yes (User confirmed)  
**Next Steps:** Create GitHub issue, implement Phase 1
