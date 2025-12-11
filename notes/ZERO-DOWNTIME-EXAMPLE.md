# Zero-Downtime Deployment - Example Walkthrough

This document demonstrates the zero-downtime deployment feature with a concrete example.

---

## Setup

### Initial Formation (v1)

**File:** `my-formation/app.py`
```python
from fastapi import FastAPI
import uvicorn

app = FastAPI()

@app.get("/")
async def root():
    return {"message": "Hello from v1", "version": 1}

@app.get("/health")
async def health():
    return {"status": "healthy", "version": 1}

if __name__ == "__main__":
    port = int(os.getenv("PORT", 8000))
    uvicorn.run(app, host="127.0.0.1", port=port)
```

**File:** `my-formation/formation.afs`
```yaml
name: "My Formation"
version: "1.0.0"
command: "python"
args: ["app.py"]
```

**Package and deploy:**
```bash
cd my-formation
tar czf v1.tar.gz app.py formation.afs
curl -X POST http://localhost:7890/rpc/formations/deploy \
  -H "Content-Type: application/gzip" \
  --data-binary @v1.tar.gz
```

**Result:**
```json
{
  "id": "my-formation",
  "status": "running",
  "port": 8001,
  "pid": 54321,
  "message": "Formation deployed successfully"
}
```

Formation is now serving traffic on port 8001:
```bash
curl http://localhost:7890/api/my-formation/
# {"message": "Hello from v1", "version": 1}
```

---

## Scenario 1: Successful Update (Zero Downtime)

### New Version (v2) - Healthy

**Update:** `my-formation/app.py`
```python
from fastapi import FastAPI
import uvicorn

app = FastAPI()

@app.get("/")
async def root():
    return {"message": "Hello from v2 - NEW FEATURES!", "version": 2}

@app.get("/health")
async def health():
    return {"status": "healthy", "version": 2}  # ✅ Health check works

if __name__ == "__main__":
    port = int(os.getenv("PORT", 8000))
    uvicorn.run(app, host="127.0.0.1", port=port)
```

**Package and update:**
```bash
tar czf v2.tar.gz app.py formation.yaml
curl -X PUT http://localhost:7890/rpc/formations/my-formation \
  -H "Content-Type: application/gzip" \
  --data-binary @v2.tar.gz
```

### What Happens Behind the Scenes:

**Timeline:**

```
T+0s   : v1 running on 8001 (serving traffic)
         ↓
T+0s   : Deploy v2 request arrives
         ↓
T+0s   : Allocate port 8002 for staging
         ↓
T+0s   : Extract v2 to staging/ directory
         ↓
T+0.5s : Start v2 on port 8002 (staging)
         v1 STILL serving on 8001 ✅
         ↓
T+2.5s : Begin health checks (after 2s delay)
         v1 STILL serving on 8001 ✅
         ↓
T+2.6s : Poll http://127.0.0.1:8002/health
         Response: 200 OK ✅
         ↓
T+2.6s : v2 is healthy! Switch ports
         Update registry: active port = 8002
         ↓
T+2.7s : Stop v1 gracefully (SIGTERM)
         ↓
T+2.8s : v1 stopped
         ↓
T+2.8s : Release port 8001
         ↓
T+2.8s : Move staging → current, old current → previous
         ↓
T+2.9s : Update version history
         ↓
T+3s   : ✅ Deployment complete!
         v2 now serving on 8002
```

**Total Downtime:** 0 seconds! 🎉

**API Response:**
```json
{
  "id": "my-formation",
  "status": "running",
  "version": 2,
  "previous_version": 1,
  "port": 8002,
  "pid": 54322,
  "message": "Formation updated with zero downtime",
  "deployment_type": "blue-green"
}
```

**Verify:**
```bash
curl http://localhost:7890/api/my-formation/
# {"message": "Hello from v2 - NEW FEATURES!", "version": 2}
```

---

## Scenario 2: Failed Update (Zero Downtime Maintained!)

### New Version (v3) - Broken Health Endpoint

**Update:** `my-formation/app.py`
```python
from fastapi import FastAPI
import uvicorn

app = FastAPI()

@app.get("/")
async def root():
    return {"message": "Hello from v3", "version": 3}

# ❌ OOPS! Forgot to add /health endpoint
# (Or it's broken, or crashes immediately)

if __name__ == "__main__":
    port = int(os.getenv("PORT", 8000))
    uvicorn.run(app, host="127.0.0.1", port=port)
```

**Package and update:**
```bash
tar czf v3.tar.gz app.py formation.yaml
curl -X PUT http://localhost:7890/rpc/formations/my-formation \
  -H "Content-Type: application/gzip" \
  --data-binary @v3.tar.gz
```

### What Happens Behind the Scenes:

**Timeline:**

```
T+0s   : v2 running on 8002 (serving traffic)
         ↓
T+0s   : Deploy v3 request arrives
         ↓
T+0s   : Allocate port 8003 for staging
         ↓
T+0s   : Extract v3 to staging/ directory
         ↓
T+0.5s : Start v3 on port 8003 (staging)
         v2 STILL serving on 8002 ✅
         ↓
T+2.5s : Begin health checks
         v2 STILL serving on 8002 ✅
         ↓
T+2.6s : Poll http://127.0.0.1:8003/health
         ❌ 404 Not Found
         ↓
T+3.6s : Poll again... ❌ 404 Not Found
         v2 STILL serving on 8002 ✅
         ↓
T+4.6s : Poll again... ❌ 404 Not Found
         v2 STILL serving on 8002 ✅
         ↓
         ... (continues for 30 seconds) ...
         v2 STILL serving on 8002 ✅
         ↓
T+32.5s: Health check timeout!
         ↓
T+32.5s: ❌ v3 failed health check
         Force kill v3 (SIGKILL)
         ↓
T+32.6s: Release port 8003
         ↓
T+32.7s: Clean up staging/ directory
         ↓
T+32.8s: v2 STILL running on 8002 ✅
         ↓
T+33s  : ✅ Deployment failed safely
         v2 continues serving traffic!
```

**Total Downtime:** 0 seconds! 🎉

**API Response (HTTP 400 Bad Request):**
```json
{
  "error": "New version failed health check: health check failed after 30 attempts (timeout: 30s). Old version still running - zero downtime maintained."
}
```

**Verify v2 still running:**
```bash
curl http://localhost:7890/api/my-formation/
# {"message": "Hello from v2 - NEW FEATURES!", "version": 2}
```

**No downtime occurred!** The user can fix v3 and try again.

---

## Scenario 3: Deployment with Custom Health Endpoint

### Configuration

**Update:** `~/.muxi/server/config.yaml`
```yaml
formations:
  deployment:
    health_check:
      enabled: true
      endpoint: "/api/health"  # Custom endpoint
      timeout: 60              # Longer timeout
      interval: 2              # Check every 2 seconds
```

**Formation with custom endpoint:**
```python
from fastapi import FastAPI
import uvicorn

app = FastAPI()

@app.get("/")
async def root():
    return {"message": "Hello from v4"}

@app.get("/api/health")  # Custom endpoint
async def custom_health():
    return {"status": "healthy", "custom": True}

if __name__ == "__main__":
    port = int(os.getenv("PORT", 8000))
    uvicorn.run(app, host="127.0.0.1", port=port)
```

**Deploy:**
```bash
tar czf v4.tar.gz app.py formation.yaml
curl -X PUT http://localhost:7890/rpc/formations/my-formation \
  -H "Content-Type: application/gzip" \
  --data-binary @v4.tar.gz
```

**Health check will use:** `http://127.0.0.1:{staging_port}/api/health`

---

## Scenario 4: Manual Rollback

If you deploy v3 successfully but discover issues later:

```bash
# Rollback to previous version
curl -X POST http://localhost:7890/rpc/formations/my-formation/rollback

# Response:
{
  "id": "my-formation",
  "status": "running",
  "version": 2,
  "previous_version": 3,
  "message": "Formation rolled back to previous version"
}
```

This uses the traditional rollback mechanism (not zero-downtime) since you're explicitly reverting.

---

## Monitoring Deployments

### Watch Deployment in Real-Time

**Terminal 1:** Watch server logs
```bash
tail -f ~/.muxi/server/logs/audit.log | grep -E "(deployment|health)"
```

**Terminal 2:** Deploy new version
```bash
curl -X PUT http://localhost:7890/rpc/formations/my-formation \
  -H "Content-Type: application/gzip" \
  --data-binary @v5.tar.gz
```

**Terminal 1 output:**
```
INFO  Starting zero-downtime deployment id=my-formation
INFO  Allocated staging port id=my-formation current_port=8002 staging_port=8003
INFO  Staging formation started, beginning health checks id=my-formation staging_port=8003 staging_pid=54323
INFO  Starting health checks for staging formation formation_id=my-formation port=8003 endpoint=/health timeout=30s
INFO  Formation is healthy formation_id=my-formation port=8003 attempt=1
INFO  Staging formation is healthy - switching to new version id=my-formation staging_port=8003
INFO  ✓ Zero-downtime deployment successful id=my-formation version=5 pid=54323
```

### Check Formation Status

```bash
curl http://localhost:7890/rpc/formations/my-formation
```

**Response:**
```json
{
  "id": "my-formation",
  "name": "My Formation",
  "status": "running",
  "port": 8003,
  "pid": 54323,
  "version": {
    "current": 5,
    "previous": 4
  },
  "deployed_at": "2025-10-25T19:00:00Z",
  "uptime": "5m23s",
  "healthy": true,
  "last_health_check": "2025-10-25T19:05:15Z"
}
```

---

## Best Practices

### 1. Always Implement /health Endpoint

**Good:**
```python
@app.get("/health")
async def health():
    # Verify dependencies
    try:
        # Check database connection
        await db.ping()
        # Check cache connection
        await cache.ping()
        return {"status": "healthy"}
    except Exception as e:
        return {"status": "unhealthy", "error": str(e)}, 503
```

**Bad:**
```python
# No health endpoint - deployment will fail!
```

### 2. Test Health Endpoint Before Deploying

```bash
# Test locally
python app.py &
sleep 2
curl http://localhost:8000/health
# Should return 200 OK

# Then deploy
tar czf bundle.tar.gz app.py formation.yaml
curl -X PUT http://localhost:7890/rpc/formations/my-formation \
  -H "Content-Type: application/gzip" \
  --data-binary @bundle.tar.gz
```

### 3. Configure Timeout for Slow-Starting Formations

If your formation takes time to initialize (e.g., loading models):

```yaml
formations:
  deployment:
    health_check:
      timeout: 120              # 2 minutes
      staging_health_delay: 10  # Wait 10 seconds before checking
```

### 4. Handle Graceful Shutdown

**Good:**
```python
import signal

def shutdown_handler(signum, frame):
    print("Shutting down gracefully...")
    # Close database connections
    # Flush logs
    sys.exit(0)

signal.signal(signal.SIGTERM, shutdown_handler)
```

This allows the old version to clean up properly when stopped.

---

## Troubleshooting

### Health Check Keeps Failing

**Check logs:**
```bash
tail -f ~/.muxi/server/logs/formation-my-formation-staging.log
```

**Common issues:**
- Formation crashed on startup
- Wrong port (check if formation is binding to correct PORT env var)
- Health endpoint returns non-2xx status
- Formation too slow to start (increase timeout)

### Port Exhaustion

**Error:** "No available ports for staging deployment"

**Solution:**
- Increase port range in config:
```yaml
formations:
  port_range_start: 8000
  port_range_end: 9000  # 1000 ports available
```
- Or stop unused formations to free ports

### Staging Process Stuck

If deployment hangs, the staging process may be stuck:

**Find and kill manually:**
```bash
ps aux | grep my-formation-staging
kill -9 <PID>
```

The next deployment will clean up automatically.

---

## Summary

✅ **Zero-downtime deployments work!**
✅ **Old version serves traffic during update**
✅ **Failed updates don't cause downtime**
✅ **Easy to configure and use**
✅ **Production-ready**

Your formations can now be updated without any service interruption! 🚀
