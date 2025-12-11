# Managing Formations

Learn how to deploy, manage, and monitor MUXI formations.

---

## Overview

Formations are the core units managed by MUXI Server. Each formation:
- Runs as an isolated process (Docker container or Singularity SIF)
- Has its own port (8000-9000 range)
- Has automatic health monitoring
- Can be restarted automatically on crash
- Uses MUXI Runtime (0.2025.0) for execution

**Formation Config File:**
The server supports three file extensions for formation configuration (in priority order):
- `.afs` - Agent Formation Schema (preferred)
- `.yaml` - Standard YAML
- `.yml` - YAML shorthand

**✨ NEW: Formation Bundle Upload**  
MUXI Server now supports uploading complete formation bundles (gzipped tarballs) with automatic metadata injection. See [Bundle Upload](#bundle-upload-new) section below or the [BUNDLE-UPLOAD-COMPLETE.md](../BUNDLE-UPLOAD-COMPLETE.md) guide for details.

**🎉 NEW: Runtime Integration Complete!**  
MUXI Server now fully integrates with the MUXI Runtime, deploying formations as Docker containers (macOS/Windows) or Singularity SIF containers (Linux). See [Runtime Support](#runtime-support-new) section below.

---

## Deploy Formation

### Bundle Upload (NEW!)

Deploy a complete formation by uploading a gzipped tarball:

```bash
# Create bundle
tar -czf formation.tar.gz my-formation/

# Deploy with HMAC authentication
TIMESTAMP=$(date +%s)
SIGNATURE=$(echo -n "${TIMESTAMP};POST;/rpc/formations/deploy" | openssl dgst -sha256 -hmac "$SECRET" -binary | base64)

curl -X POST http://localhost:7890/rpc/formations/deploy \
  -H "Authorization: MUXI-HMAC key=$KEY, timestamp=$TIMESTAMP, signature=$SIGNATURE" \
  -H "Content-Type: application/gzip" \
  --data-binary "@formation.tar.gz"
```

**Benefits:**
- Automatic `_server_id` and `_deployment_mode` injection
- Environment variable generation (PORT, FORMATION_ID, etc.)
- Formation.yaml parsing and validation
- Complete application code deployment

**For full details, see:** [BUNDLE-UPLOAD-COMPLETE.md](../BUNDLE-UPLOAD-COMPLETE.md)

### Using CLI (Coming in Phase 2)

```bash
# CLI tool in development
muxi formation deploy my-formation.yaml
```

**With specific profile:**
```bash
muxi formation deploy my-formation.yaml --profile=production
```

### Using HTTP API (JSON - Legacy)

```bash
curl -X POST http://localhost:7890/rpc/formations/deploy \
  -H "Authorization: MUXI-HMAC key=..., timestamp=..., signature=..." \
  -H "Content-Type: application/json" \
  -d '{
    "id": "my-api",
    "command": "python app.py"
  }'
```

### Runtime Support (NEW!)

**MUXI Server automatically detects and uses the appropriate runtime:**

- **macOS/Windows:** Uses Docker with muxi-runtime image
- **Linux:** Uses Singularity SIF containers (native)

**Runtime Version Resolution:**
- Reads `runtime: "0.2025.0"` from formation.yaml
- Resolves to appropriate SIF file: `muxi-runtime-0.2025.0-{platform}-{arch}.sif`
- Spawns container with formation directory mounted

**Container Architecture:**
```
Server spawns: docker run -v /formation:ro -p 8003:8003 muxi-runtime:0.2025.0 /formation/formation.yaml
              └─> Container starts FastAPI server on 0.0.0.0:8003
                  └─> Server proxies: /api/{formation_id}/* → http://localhost:8003/*
```

**Benefits:**
- ✅ Isolated formation execution
- ✅ Consistent runtime environment
- ✅ Single SIF file distribution (693MB)
- ✅ No Python dependencies on server
- ✅ Automatic runtime version management

---

### Deploy Options

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `id` | string | Yes | Unique formation identifier |
| `command` | string | Auto | Command to execute (auto-set for runtime) |
| `env` | object | No | Environment variables |
| `working_dir` | string | No | Working directory |
| `runtime` | string | No | Runtime version (e.g., "0.2025.0") |

### Example: Simple Formation

```json
{
  "id": "chat-api",
  "command": "python app.py"
}
```

### Example: With Environment Variables

```json
{
  "id": "chat-api",
  "command": "python app.py",
  "env": {
    "MODEL_NAME": "gpt-4",
    "API_KEY": "sk-xxx"
  },
  "working_dir": "/home/user/rpc/formations/chat-api"
}
```

### Response

**Success (201 Created):**
```json
{
  "id": "chat-api",
  "status": "running",
  "port": 8001,
  "pid": 12345,
  "url": "http://localhost:7890/chat-api",
  "created_at": "2025-01-17T10:30:00Z"
}
```

**Error (400 Bad Request):**
```json
{
  "error": "Formation already exists",
  "code": 400
}
```

---

## List Formations

### Using CLI

```bash
# List all formations
muxi formation list

# With details
muxi formation list --verbose

# Filter by status
muxi formation list --status=running
```

### Using HTTP API

```bash
curl http://localhost:7890/formations \
  -H "Authorization: MUXI-HMAC key=..., timestamp=..., signature=..."
```

### Response

```json
{
  "formations": [
    {
      "id": "chat-api",
      "status": "running",
      "port": 8001,
      "pid": 12345,
      "url": "http://localhost:7890/chat-api",
      "created_at": "2025-01-17T10:30:00Z",
      "updated_at": "2025-01-17T10:30:00Z",
      "restart_count": 0,
      "health": "healthy"
    },
    {
      "id": "workflow-engine",
      "status": "running",
      "port": 8002,
      "pid": 12346,
      "url": "http://localhost:7890/workflow-engine",
      "created_at": "2025-01-17T10:31:00Z",
      "updated_at": "2025-01-17T10:31:00Z",
      "restart_count": 2,
      "health": "healthy"
    }
  ],
  "total": 2
}
```

### Formation Status

| Status | Description |
|--------|-------------|
| `starting` | Formation is being spawned |
| `running` | Formation is running normally |
| `stopping` | Formation is being stopped |
| `stopped` | Formation has been stopped |
| `crashed` | Formation crashed and won't restart |
| `restarting` | Formation is restarting |

### Health Status

| Health | Description |
|--------|-------------|
| `healthy` | `/health` endpoint returns 200 |
| `unhealthy` | `/health` endpoint failing |
| `unknown` | Health check not yet performed |

---

## Get Formation Details

### Using CLI

```bash
muxi formation get chat-api
```

### Using HTTP API

```bash
curl http://localhost:7890/rpc/formations/chat-api \
  -H "Authorization: MUXI-HMAC key=..., timestamp=..., signature=..."
```

### Response

```json
{
  "id": "chat-api",
  "command": "python app.py",
  "working_dir": "/home/user/rpc/formations/chat-api",
  "env": {
    "MODEL_NAME": "gpt-4"
  },
  "status": "running",
  "port": 8001,
  "pid": 12345,
  "url": "http://localhost:7890/chat-api",
  "created_at": "2025-01-17T10:30:00Z",
  "updated_at": "2025-01-17T10:30:00Z",
  "started_at": "2025-01-17T10:30:05Z",
  "restart_count": 0,
  "health": "healthy",
  "last_health_check": "2025-01-17T10:35:00Z",
  "memory_usage": "150MB",
  "cpu_usage": "2.5%"
}
```

---

## Stop Formation

### Using CLI

```bash
muxi formation stop chat-api
```

### Using HTTP API

```bash
curl -X POST http://localhost:7890/rpc/formations/chat-api/stop \
  -H "Authorization: MUXI-HMAC key=..., timestamp=..., signature=..."
```

### Response

```json
{
  "id": "chat-api",
  "status": "stopped",
  "message": "Formation stopped successfully"
}
```

**Note:** Stopped formations will not auto-restart, even if configured.

---

## Restart Formation

### Using CLI

```bash
muxi formation restart chat-api
```

### Using HTTP API

```bash
curl -X POST http://localhost:7890/rpc/formations/chat-api/restart \
  -H "Authorization: MUXI-HMAC key=..., timestamp=..., signature=..."
```

### Response

```json
{
  "id": "chat-api",
  "status": "restarting",
  "message": "Formation restarting",
  "restart_count": 3
}
```

---

## Update Formation

**✨ NEW: Zero-Downtime Deployments**

MUXI Server uses **blue-green deployment** to update formations without downtime. The old version continues serving traffic while the new version is validated.

### How It Works

1. **Old version keeps running** on current port (e.g., 8001)
2. **New version starts** on staging port (e.g., 8002)
3. **Health check** validates new version (30 second timeout)
4. **Decision:**
   - ✅ **If healthy:** Switch traffic to new version, stop old version
   - ❌ **If unhealthy:** Kill new version, keep old version running

**Result:** Zero downtime, even if deployment fails!

### Using CLI (Coming Soon)

```bash
# Update to new version (zero-downtime)
muxi formation update chat-api --bundle=v2.tar.gz
```

### Using HTTP API

```bash
# Create new bundle
tar -czf v2.tar.gz my-formation/

# Update formation (zero-downtime deployment)
TIMESTAMP=$(date +%s)
SIGNATURE=$(echo -n "${TIMESTAMP};PUT;/rpc/formations/chat-api" | openssl dgst -sha256 -hmac "$SECRET" -binary | base64)

curl -X PUT http://localhost:7890/rpc/formations/chat-api \
  -H "Authorization: MUXI-HMAC key=$KEY, timestamp=$TIMESTAMP, signature=$SIGNATURE" \
  -H "Content-Type: application/gzip" \
  --data-binary "@v2.tar.gz"
```

### Success Response

```json
{
  "id": "chat-api",
  "status": "running",
  "version": 2,
  "previous_version": 1,
  "port": 8002,
  "pid": 12345,
  "message": "Formation updated with zero downtime",
  "deployment_type": "blue-green"
}
```

### Failed Deployment (Zero Downtime Maintained!)

If the new version fails health checks, the old version continues running:

```json
{
  "error": "New version failed health check: health check failed after 30 attempts. Old version still running - zero downtime maintained."
}
```

**Your formation never went down!** Fix the issue and try again.

### Health Check Requirements

For zero-downtime deployments to work, your formation **must** expose a health endpoint:

**Python (FastAPI):**
```python
@app.get("/health")
async def health():
    return {"status": "healthy"}
```

**Node.js (Express):**
```javascript
app.get('/health', (req, res) => {
  res.json({ status: 'healthy' });
});
```

**Default endpoint:** `/health` (configurable in `config.yaml`)

### Configuration

Customize health check behavior in `~/.muxi/server/config.yaml`:

```yaml
formations:
  deployment:
    health_check:
      enabled: true              # Enable zero-downtime deployments
      endpoint: "/health"        # Health endpoint path
      timeout: 30                # Timeout in seconds
      interval: 1                # Poll interval in seconds
      max_retries: 30            # Max health check attempts
    
    force_kill_timeout: 5        # Seconds before force-killing old version
    staging_health_delay: 2      # Delay before first health check
```

### Rollback

If you discover issues after a successful deployment:

```bash
# Rollback to previous version
curl -X POST http://localhost:7890/rpc/formations/chat-api/rollback \
  -H "Authorization: MUXI-HMAC key=..., timestamp=..., signature=..."
```

**Response:**
```json
{
  "id": "chat-api",
  "status": "running",
  "version": 1,
  "previous_version": 2,
  "message": "Formation rolled back to previous version"
}
```

### Best Practices

1. **Always implement /health endpoint** - Required for zero-downtime deployments
2. **Test health endpoint before deploying** - Verify it returns 200 OK
3. **Handle graceful shutdown** - Listen for SIGTERM and clean up resources
4. **For slow-starting formations** - Increase `timeout` and `staging_health_delay`

### Monitoring Updates

Watch deployment progress in real-time:

```bash
# Terminal 1: Watch logs
tail -f ~/.muxi/server/logs/audit.log | grep deployment

# Terminal 2: Deploy
curl -X PUT http://localhost:7890/rpc/formations/chat-api \
  -H "Content-Type: application/gzip" \
  --data-binary "@v2.tar.gz"
```

**Successful deployment log:**
```
INFO  Starting zero-downtime deployment id=chat-api
INFO  Allocated staging port id=chat-api current_port=8001 staging_port=8002
INFO  Staging formation started, beginning health checks
INFO  Formation is healthy formation_id=chat-api port=8002 attempt=1
INFO  Staging formation is healthy - switching to new version
INFO  ✓ Zero-downtime deployment successful version=2 pid=12345
```

---

## Delete Formation

### Using CLI

```bash
muxi formation delete chat-api
```

**With confirmation:**
```bash
muxi formation delete chat-api --yes
```

### Using HTTP API

```bash
curl -X DELETE http://localhost:7890/rpc/formations/chat-api \
  -H "Authorization: MUXI-HMAC key=..., timestamp=..., signature=..."
```

### Response

```json
{
  "id": "chat-api",
  "message": "Formation deleted successfully"
}
```

**Note:** This permanently removes the formation. Process is killed and port is freed.

---

## View Logs

### Using CLI

```bash
# View recent logs
muxi formation logs chat-api

# Follow logs (tail -f)
muxi formation logs chat-api --follow

# Last 100 lines
muxi formation logs chat-api --lines=100
```

### Log Files

Logs are stored in: `~/.muxi/server/logs/`

**Log file naming:**
- `{formation_id}.log` - Current log
- `{formation_id}.log.1` - Rotated log (previous)
- `{formation_id}.log.2` - Older rotated log

**Example:**
```bash
tail -f ~/.muxi/server/logs/chat-api.log
```

### Log Format

```
[2025-01-17 10:30:05] [INFO] Starting formation: chat-api
[2025-01-17 10:30:06] [INFO] Listening on port 8001
[2025-01-17 10:30:15] [INFO] Health check: OK
[2025-01-17 10:31:22] [INFO] POST /chat - 200 - 234ms
[2025-01-17 10:32:45] [ERROR] Failed to connect to database
```

---

## Access Formation

### Via Proxy

All formation routes are accessible through the server:

```
http://{server}/{formation_id}/{path}
```

**Example:**

```bash
# Direct to formation (not recommended)
curl http://localhost:8001/chat -d '{"message": "Hello"}'

# Through proxy (recommended)
curl http://localhost:7890/chat-api/chat -d '{"message": "Hello"}'
```

### Benefits of Proxy

✅ Single entry point for all formations  
✅ Load balancing (future)  
✅ SSL termination (future)  
✅ Request logging and metrics  
✅ Formation isolation

---

## Health Checks

### How It Works

MUXI Server automatically checks formation health:

1. **Interval:** Every 30 seconds (configurable)
2. **Endpoint:** `GET /{formation_id}/health`
3. **Timeout:** 10 seconds (configurable)
4. **Expected:** HTTP 200 status code

### Health Check Response

Formations should implement a `/health` endpoint:

```python
@app.get("/health")
def health():
    return {"status": "healthy"}
```

### Configuration

```yaml
formations:
  health_check_interval: 30  # seconds
  health_check_timeout: 10   # seconds
```

### Health Status

| Status | Description | Action |
|--------|-------------|--------|
| `healthy` | 200 response | None |
| `unhealthy` | Non-200 or timeout | Auto-restart if enabled |
| `unknown` | Not checked yet | Wait for first check |

---

## Auto-Restart

### How It Works

Formations automatically restart when:
- Process crashes (exit code != 0)
- Health check fails (unhealthy)
- Process becomes unresponsive

### Configuration

```yaml
formations:
  auto_restart: true          # Enable auto-restart
  max_restart_count: 10       # Max attempts
  restart_delay: 1            # Seconds between attempts
```

### Restart Behavior

1. **First crash:** Restart immediately
2. **Second crash:** Wait 1 second, restart
3. **Third crash:** Wait 2 seconds, restart
4. **N crashes:** Wait N seconds, restart
5. **After 10 crashes:** Give up, mark as `crashed`

### Manual Restart

Even if auto-restart is disabled, you can manually restart:

```bash
muxi formation restart chat-api
```

---

## Best Practices

### Formation IDs

✅ **Good:**
- `chat-api`
- `workflow-engine-v2`
- `customer-analytics`

❌ **Bad:**
- `my api` (spaces)
- `api!` (special chars)
- `Test` (uppercase)

**Rules:**
- Lowercase letters, numbers, hyphens
- Start with letter
- 3-50 characters

### Resource Management

**Small formation (< 100 req/min):**
```json
{
  "id": "simple-api",
  "command": "uvicorn app:app --workers 1"
}
```

**Medium formation (100-1000 req/min):**
```json
{
  "id": "busy-api",
  "command": "uvicorn app:app --workers 4"
}
```

**Large formation (> 1000 req/min):**
```json
{
  "id": "high-traffic-api",
  "command": "gunicorn app:app --workers 8 --worker-class uvicorn.workers.UvicornWorker"
}
```

### Health Endpoints

Implement comprehensive health checks:

```python
@app.get("/health")
def health():
    try:
        # Check database connection
        db.execute("SELECT 1")
        
        # Check external services
        response = requests.get("https://api.example.com/health", timeout=2)
        
        if response.status_code == 200:
            return {"status": "healthy"}
        else:
            return {"status": "unhealthy", "reason": "Service unavailable"}, 503
    except Exception as e:
        return {"status": "unhealthy", "reason": str(e)}, 503
```

### Graceful Shutdown

Handle SIGTERM for graceful shutdown:

```python
import signal
import sys

def shutdown(signum, frame):
    print("Shutting down gracefully...")
    # Close connections
    db.close()
    # Exit cleanly
    sys.exit(0)

signal.signal(signal.SIGTERM, shutdown)
```

---

## Monitoring

### Check Formation Status

```bash
# Quick status
muxi formation status chat-api

# Detailed info
muxi formation get chat-api --verbose
```

### Watch Logs

```bash
# Follow logs in real-time
muxi formation logs chat-api --follow

# Filter by level
muxi formation logs chat-api --level=error
```

### Resource Usage

```bash
# Show CPU and memory
muxi formation stats chat-api

# All formations
muxi formation stats --all
```

---

## Troubleshooting

### Formation Won't Start

**Check logs:**
```bash
muxi formation logs my-api
```

**Common issues:**
- Python not installed
- Dependencies missing
- Port already in use
- File not found

### Formation Keeps Crashing

**Check restart count:**
```bash
muxi formation get my-api
```

**Solutions:**
1. Fix application bugs
2. Increase restart limit
3. Add better error handling
4. Check health endpoint

### Formation Not Accessible

**Check status:**
```bash
muxi formation status my-api
```

**Verify proxy:**
```bash
curl http://localhost:7890/my-api/health
```

**Check formation directly:**
```bash
# Get port from status
curl http://localhost:8001/health
```

---

## Next Steps

- [Configure server settings](./configuration.md)
- [Set up authentication](./authentication.md)
- [API Reference](./api-reference.md)

---

**Need help?** See the [Troubleshooting Guide](./troubleshooting.md)
