# Migration Guide: API v1 → v2

**Date:** 2025-10-19  
**Breaking Changes:** Yes  
**Estimated Migration Time:** 15-30 minutes

This guide helps you migrate from MUXI Server API v1 to v2 (API Architecture Refactor).

---

## Overview of Changes

The API refactor introduces:
- **New port:** 7890 (was 3000)
- **RESTful routes:** `/rpc/*` for management, `/api/*` for proxy
- **Formation versioning:** Update and rollback support
- **Server management:** Status and logs endpoints
- **Security:** Localhost-only formation binding

---

## Quick Migration Checklist

- [ ] Update server port: `3000` → `7890`
- [ ] Update management API routes: `/formations` → `/rpc/formations`
- [ ] Update proxy routes: `/v1/{id}` → `/api/{id}`
- [ ] Update HMAC signatures (use new paths)
- [ ] Verify formations bind to `127.0.0.1`
- [ ] Update integration tests
- [ ] Test formation deployment
- [ ] Test proxy access

---

## Detailed Migration Steps

### 1. Update Server Configuration

**Before (v1):**
```yaml
server:
  port: 3000
  host: "0.0.0.0"
```

**After (v2):**
```yaml
server:
  port: 7890        # New default port
  host: "0.0.0.0"   # Server remains externally accessible

formations:
  bind_host: "127.0.0.1"  # Formations bind to localhost only
  keep_backups: 1         # Enable versioning
  
logging:
  audit_log: "logs/audit.log"  # Audit logging
```

**Action:**
1. Update `~/.muxi/server/config.yaml`
2. Set `port: 7890`
3. Add `bind_host: "127.0.0.1"` under formations
4. Restart server

---

### 2. Update API Routes

#### Management API Routes

| Operation | Old Route (v1) | New Route (v2) |
|-----------|---------------|----------------|
| Health check | `GET /health` | `GET /health` ✅ (unchanged) |
| Ping | N/A | `GET /ping` ⭐ (new) |
| Deploy formation | `POST /formations/deploy` | `POST /rpc/formations/deploy` |
| List formations | `GET /formations` | `GET /rpc/formations` |
| Get formation | `GET /formations/{id}` | `GET /rpc/formations/{id}` |
| Update formation | N/A | `PUT /rpc/formations/{id}` ⭐ (new) |
| Stop formation | `POST /formations/{id}/stop` | `POST /rpc/formations/{id}/stop` |
| Restart formation | `POST /formations/{id}/restart` | `POST /rpc/formations/{id}/restart` |
| Rollback formation | N/A | `POST /rpc/formations/{id}/rollback` ⭐ (new) |
| Delete formation | `DELETE /formations/{id}` | `DELETE /rpc/formations/{id}` |
| Formation logs | `GET /formations/{id}/logs` | `GET /rpc/formations/{id}/logs` |
| Server status | N/A | `GET /rpc/server/status` ⭐ (new) |
| Server logs | N/A | `GET /rpc/server/logs` ⭐ (new) |

#### Proxy Routes

| Old Route (v1) | New Route (v2) |
|---------------|----------------|
| `GET /v1/{id}/*` | `GET /api/{id}/*` |
| `POST /v1/{id}/*` | `POST /api/{id}/*` |
| `PUT /v1/{id}/*` | `PUT /api/{id}/*` |
| `DELETE /v1/{id}/*` | `DELETE /api/{id}/*` |
| `PATCH /v1/{id}/*` | `PATCH /api/{id}/*` |

**Action:**
1. Find all client code calling MUXI Server
2. Replace `/formations` with `/rpc/formations`
3. Replace `/v1/` with `/api/`
4. Update port from `3000` to `7890`

---

### 3. Update Client Code Examples

#### Deploy Formation

**Before (v1):**
```bash
curl -X POST http://localhost:3000/formations/deploy \
  -H "Authorization: MUXI-HMAC key=$KEY, timestamp=$TS, signature=$SIG" \
  -H "Content-Type: application/gzip" \
  --data-binary "@formation.tar.gz"
```

**After (v2):**
```bash
curl -X POST http://localhost:7890/rpc/formations/deploy \
  -H "Authorization: MUXI-HMAC key=$KEY, timestamp=$TS, signature=$SIG" \
  -H "Content-Type: application/gzip" \
  --data-binary "@formation.tar.gz"
```

#### List Formations

**Before (v1):**
```bash
curl http://localhost:3000/formations \
  -H "Authorization: MUXI-HMAC key=$KEY, timestamp=$TS, signature=$SIG"
```

**After (v2):**
```bash
curl http://localhost:7890/rpc/formations \
  -H "Authorization: MUXI-HMAC key=$KEY, timestamp=$TS, signature=$SIG"
```

#### Access Formation Endpoint (Proxy)

**Before (v1):**
```bash
curl http://localhost:3000/v1/my-formation/chat \
  -H "Content-Type: application/json" \
  -d '{"message": "Hello!"}'
```

**After (v2):**
```bash
curl http://localhost:7890/api/my-formation/chat \
  -H "Content-Type: application/json" \
  -d '{"message": "Hello!"}'
```

---

### 4. Update HMAC Signatures

**CRITICAL:** HMAC signatures must use the new routes!

**Before (v1):**
```python
message = f"{timestamp};POST;/formations/deploy"
signature = hmac.new(secret.encode(), message.encode(), hashlib.sha256).digest()
```

**After (v2):**
```python
message = f"{timestamp};POST;/rpc/formations/deploy"  # Note: /rpc/formations
signature = hmac.new(secret.encode(), message.encode(), hashlib.sha256).digest()
```

**Action:**
1. Update all HMAC signature generation code
2. Use new route paths: `/rpc/formations/*`, `/rpc/server/*`
3. Test authentication with new routes

---

### 5. Update Formation Code (Optional but Recommended)

Formations now receive the `HOST` environment variable.

**Before (v1):**
```python
import os
from fastapi import FastAPI

app = FastAPI()
port = int(os.getenv("PORT", 8000))

if __name__ == "__main__":
    import uvicorn
    uvicorn.run(app, host="0.0.0.0", port=port)  # Old: binds to 0.0.0.0
```

**After (v2):**
```python
import os
from fastapi import FastAPI

app = FastAPI()
port = int(os.getenv("PORT", 8000))
host = os.getenv("HOST", "127.0.0.1")  # New: read from environment

if __name__ == "__main__":
    import uvicorn
    uvicorn.run(app, host=host, port=port)  # Binds to 127.0.0.1
```

**Why:**
- Security: Formations only accessible via MUXI proxy
- Prevents direct external access to formations
- Follows principle of least privilege

**Action:**
1. Update formation startup code to read `HOST` env var
2. Default to `127.0.0.1` if not set
3. Redeploy formations

---

### 6. New Features You Can Use

#### Formation Versioning

Update a running formation:

```bash
# Build new version
tar -czf formation-v2.tar.gz -C my-formation .

# Update formation (keeps previous version as backup)
curl -X PUT http://localhost:7890/rpc/formations/my-formation \
  -H "Authorization: MUXI-HMAC key=$KEY, timestamp=$TS, signature=$SIG" \
  -H "Content-Type: application/gzip" \
  --data-binary "@formation-v2.tar.gz"
```

Rollback if something goes wrong:

```bash
curl -X POST http://localhost:7890/rpc/formations/my-formation/rollback \
  -H "Authorization: MUXI-HMAC key=$KEY, timestamp=$TS, signature=$SIG"
```

#### Server Monitoring

Get server statistics:

```bash
curl http://localhost:7890/rpc/server/status \
  -H "Authorization: MUXI-HMAC key=$KEY, timestamp=$TS, signature=$SIG"
```

Response:
```json
{
  "success": true,
  "data": {
    "server": {
      "id": "macbook-abc123",
      "version": "1.0.0",
      "uptime": 3600
    },
    "formations": {
      "total": 5,
      "running": 4,
      "stopped": 1,
      "crashed": 0
    },
    "port_pool": {
      "total": 1000,
      "available": 995,
      "allocated": 5
    },
    "runtime": {
      "goroutines": 42,
      "go_version": "go1.21.0"
    }
  }
}
```

Get audit logs:

```bash
# Last 100 lines (default)
curl http://localhost:7890/rpc/server/logs \
  -H "Authorization: MUXI-HMAC key=$KEY, timestamp=$TS, signature=$SIG"

# Last 500 lines
curl "http://localhost:7890/rpc/server/logs?lines=500" \
  -H "Authorization: MUXI-HMAC key=$KEY, timestamp=$TS, signature=$SIG"
```

---

### 7. Reserved Formation IDs

**NEW:** Certain formation IDs are now reserved and cannot be used.

**Reserved IDs:**
- `health`
- `ping`
- `rpc`
- `server`
- `admin`
- `metrics`
- `api`

**Action:**
1. Check your formation IDs
2. Rename any formations using reserved IDs
3. Valid IDs: 3-50 chars, lowercase letters, numbers, hyphens only

---

### 8. Testing Your Migration

#### Test 1: Server Health

```bash
# Should work on new port
curl http://localhost:7890/health
# Expected: {"success":true,"data":{"status":"ok",...}}

# Should also work
curl http://localhost:7890/ping
# Expected: {"success":true,"data":{"message":"pong"}}
```

#### Test 2: Deploy Formation

```bash
# Create test formation
mkdir test-formation
cd test-formation
cat > formation.yaml << 'EOF'
schema: muxi.ai/formation/v1
id: test-migration
name: Test Migration
version: 1.0.0
runtime:
  built_in_mcps: true
EOF

# Package it
tar -czf ../test-migration.tar.gz .
cd ..

# Deploy using new route
curl -X POST http://localhost:7890/rpc/formations/deploy \
  -H "Authorization: MUXI-HMAC key=$KEY, timestamp=$TS, signature=$SIG" \
  -H "Content-Type: application/gzip" \
  --data-binary "@test-migration.tar.gz"
```

#### Test 3: Access via Proxy

```bash
# Access formation via new proxy route
curl http://localhost:7890/api/test-migration/health
# Expected: Formation's health response
```

#### Test 4: List Formations

```bash
curl http://localhost:7890/rpc/formations \
  -H "Authorization: MUXI-HMAC key=$KEY, timestamp=$TS, signature=$SIG"
# Expected: List of all formations including test-migration
```

---

## Common Migration Issues

### Issue 1: Connection Refused

**Symptom:**
```
curl: (7) Failed to connect to localhost port 3000: Connection refused
```

**Solution:**
Update to port `7890`:
```bash
curl http://localhost:7890/health  # Not 3000!
```

---

### Issue 2: 404 Not Found

**Symptom:**
```
curl http://localhost:7890/formations
# {"code":404,"error":"Not Found","message":"endpoint not found"}
```

**Solution:**
Use new route with `/rpc/` prefix:
```bash
curl http://localhost:7890/rpc/formations  # Note: /rpc/
```

---

### Issue 3: Formation Not Accessible

**Symptom:**
```
curl http://localhost:3000/v1/my-formation/health
# Connection refused or 404
```

**Solution:**
Use new proxy route and port:
```bash
curl http://localhost:7890/api/my-formation/health  # /api/ not /v1/
```

---

### Issue 4: Authentication Fails

**Symptom:**
```
{"code":401,"error":"Unauthorized","message":"Invalid signature"}
```

**Solution:**
Update HMAC signature to use new route path:
```python
# OLD (wrong):
message = f"{timestamp};POST;/formations/deploy"

# NEW (correct):
message = f"{timestamp};POST;/rpc/formations/deploy"
```

---

### Issue 5: Formation Can't Be Accessed Directly

**Symptom:**
Formation port responds on `127.0.0.1:8001` but not from external network.

**Solution:**
This is **expected behavior**. Formations bind to localhost only for security.

Access via MUXI proxy instead:
```bash
# Don't access formation directly
curl http://server-ip:8001/health  # ❌ Won't work from external

# Access via proxy
curl http://server-ip:7890/api/my-formation/health  # ✅ Works
```

---

## Rollback to v1 (Emergency)

If you need to rollback to API v1:

1. **Check out previous commit:**
   ```bash
   git checkout <commit-before-refactor>
   ```

2. **Rebuild server:**
   ```bash
   cd src
   go build -o ../muxi-server ./cmd/server
   ```

3. **Update config:**
   ```yaml
   server:
     port: 3000  # Revert to old port
   ```

4. **Restart server**

**Note:** This loses all v2 features (versioning, rollback, server management).

---

## Support

If you encounter issues during migration:

1. **Check logs:**
   ```bash
   # Server logs
   tail -f ~/.muxi/server/logs/server.log
   
   # Formation logs
   tail -f ~/.muxi/server/logs/formation-{id}-out.log
   ```

2. **Verify configuration:**
   ```bash
   muxi-server config show
   ```

3. **Review documentation:**
   - [API Reference](./docs/api-reference.md)
   - [Troubleshooting Guide](./docs/troubleshooting.md)

4. **Test with verbose output:**
   ```bash
   curl -v http://localhost:7890/rpc/formations
   ```

---

## Post-Migration Checklist

After completing migration:

- [ ] All client applications updated
- [ ] Integration tests passing
- [ ] Formation deployments working
- [ ] Proxy access working
- [ ] HMAC authentication working
- [ ] Monitor server logs for errors
- [ ] Test formation updates and rollbacks
- [ ] Verify audit logging
- [ ] Document any custom integrations updated

---

**Migration Complete!** 🎉

Your MUXI Server is now running API v2 with:
- RESTful routes
- Formation versioning and rollback
- Server management endpoints
- Enhanced security (localhost-only formations)
- Audit logging

Enjoy the new features!
