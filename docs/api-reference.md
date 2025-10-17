# API Reference

Complete HTTP API reference for MUXI Server.

---

## Base URL

```
http://localhost:7890
```

**Production:**
```
https://api.yourserver.com:7890
```

> **Port 7890** is the official MUXI Server port (like Redis: 6379, PostgreSQL: 5432)

---

## Architecture Overview

MUXI Server provides three distinct routing namespaces:

```
┌────────────────────────────────────────────────────────┐
│ MUXI Server (Port 7890)                                │
├────────────────────────────────────────────────────────┤
│                                                        │
│ /health               → Server health (public)         │
│ /ping                 → Connectivity test (public)     │
│                                                        │
│ /rpc/*                → Management API (HMAC auth)     │
│   ├─ /rpc/formations      List formations             │
│   ├─ /rpc/deploy          Deploy formation            │
│   ├─ /rpc/stop/{id}       Stop formation              │
│   ├─ /rpc/restart/{id}    Restart formation           │
│   └─ /rpc/delete/{id}     Delete formation            │
│                                                        │
│ /api/{formation_id}/* → Formation proxy (no auth)     │
│   └─ Proxies to: 127.0.0.1:{port}/*                   │
│                                                        │
│ /api                  → 404 (formation ID required)    │
│ /*                    → 404 (not found)                │
│                                                        │
└────────────────────────────────────────────────────────┘
```

---

## Authentication

### Management API (`/rpc/*`)

**All `/rpc/*` endpoints require HMAC authentication.**

Authorization header format:

```
Authorization: MUXI-HMAC key={KEY}, timestamp={TIMESTAMP}, signature={SIGNATURE}
```

**Example:**
```
Authorization: MUXI-HMAC key=MUXI_abc123, timestamp=1705484123, signature=YWJjZGVm...
```

See [Authentication Guide](./authentication.md) for signature generation details.

### Formation Proxy (`/api/*`)

**No server authentication required.** Formations handle their own authentication.

MUXI Server acts as a transparent proxy - it forwards requests as-is and does not authenticate. Each formation implements its own auth (OAuth, JWT, API keys, etc.).

### Public Endpoints

These endpoints require **no authentication**:

- `GET /health` - Server health check
- `GET /ping` - Connectivity test

---

## Security Architecture

### Localhost-Only Formation Binding

**CRITICAL:** Formations bind to `127.0.0.1:{port}`, NOT `0.0.0.0:{port}`.

```
Formation binds to: 127.0.0.1:8001

External access:
  curl http://server.com:8001/api
  → CONNECTION REFUSED ❌
  (Port not exposed to internet)

Via MUXI Proxy:
  curl http://server.com:7890/api/my-formation/api
  → MUXI → 127.0.0.1:8001/api
  → ✅ Works (localhost)
```

**Why:**
- Formations are only accessible via MUXI Server proxy
- Direct external access to formation ports is impossible
- No firewall configuration needed
- Natural OS-level security

**Implementation:**

Your formation should bind to `HOST` environment variable:

**Python/FastAPI:**
```python
import os
host = os.getenv("HOST", "127.0.0.1")  # MUXI provides this
port = int(os.getenv("PORT", 8000))
uvicorn.run(app, host=host, port=port)
```

**Node/Express:**
```javascript
const host = process.env.HOST || '127.0.0.1';
const port = process.env.PORT || 8000;
app.listen(port, host);
```

> **Note:** This architecture is critical for Phase 3 (Singularity SIF runtime). SIF containers must also bind to localhost for security.

---

## Public Endpoints

### Server Health

Check server health status (no auth required).

**Endpoint:**
```
GET /health
```

**Example Request:**

```bash
curl http://localhost:7890/health
```

**Response (200 OK):**

```json
{
  "status": "healthy",
  "version": "1.0.0",
  "uptime": 86400,
  "formations": {
    "total": 5,
    "running": 4,
    "stopped": 1,
    "crashed": 0
  }
}
```

**Fields:**

| Field | Type | Description |
|-------|------|-------------|
| `status` | string | Server status: `healthy`, `degraded`, `unhealthy` |
| `version` | string | Server version |
| `uptime` | int | Server uptime in seconds |
| `formations` | object | Formation count by status |

**Use Cases:**
- Load balancer health checks
- Kubernetes liveness probes
- Monitoring systems
- Simple connectivity verification

---

### Ping

Simple connectivity test (no auth required).

**Endpoint:**
```
GET /ping
```

**Example Request:**

```bash
curl http://localhost:7890/ping
```

**Response (200 OK):**

```
pong
```

**Use Cases:**
- Quick connectivity test
- Minimal overhead health check
- Network troubleshooting

---

## Management API (`/rpc/*`)

All management endpoints require HMAC authentication.

---

### Deploy Formation

Deploy a new formation to the server.

**Endpoint:**
```
POST /rpc/deploy
```

**Authentication:** Required (HMAC)

**Request Body:**

```json
{
  "id": "string (required)",
  "command": "string (required)",
  "args": ["string"],
  "env": {
    "KEY": "value"
  },
  "working_dir": "string (optional)",
  "auto_restart": true,
  "health_check_url": "/health"
}
```

**Fields:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `id` | string | Yes | Unique formation identifier (lowercase, hyphens, 3-50 chars) |
| `command` | string | Yes | Command to execute (e.g., `python`, `node`) |
| `args` | array | No | Command arguments (e.g., `["app.py"]`) |
| `env` | object | No | Environment variables as key-value pairs |
| `working_dir` | string | No | Working directory for the command |
| `auto_restart` | boolean | No | Auto-restart on crash (default: true) |
| `health_check_url` | string | No | Relative URL for health checks (e.g., `/health`) |

**Example Request:**

```bash
curl -X POST http://localhost:7890/rpc/deploy \
  -H "Authorization: MUXI-HMAC key=MUXI_abc123, timestamp=1705484123, signature=..." \
  -H "Content-Type: application/json" \
  -d '{
    "id": "chat-api",
    "command": "python",
    "args": ["app.py"],
    "env": {
      "MODEL": "gpt-4"
    },
    "auto_restart": true,
    "health_check_url": "/health"
  }'
```

**Response (201 Created):**

```json
{
  "id": "chat-api",
  "status": "running",
  "port": 8001,
  "pid": 12345,
  "url": "http://localhost:7890/api/chat-api",
  "health_url": "http://localhost:7890/api/chat-api/health",
  "created_at": "2025-01-17T10:30:00Z"
}
```

**Error Responses:**

| Code | Error | Description |
|------|-------|-------------|
| 400 | `Formation already exists` | Formation with this ID exists |
| 400 | `Invalid formation ID` | ID contains invalid characters or reserved name |
| 400 | `Command is required` | Missing command field |
| 401 | `Unauthorized` | Invalid or missing authentication |
| 409 | `No ports available` | Port pool exhausted (8000-9000) |
| 500 | `Failed to spawn formation` | Internal server error |

**Reserved Formation IDs:**

These IDs cannot be used (would conflict with server routes):
- `health`, `ping`, `rpc`, `server`, `admin`, `metrics`

---

### List Formations

Get a list of all formations.

**Endpoint:**
```
GET /rpc/formations
```

**Authentication:** Required (HMAC)

**Query Parameters:**

| Parameter | Type | Description |
|-----------|------|-------------|
| `status` | string | Filter by status: `running`, `stopped`, `crashed` |

**Example Request:**

```bash
curl http://localhost:7890/rpc/formations \
  -H "Authorization: MUXI-HMAC key=MUXI_abc123, timestamp=1705484123, signature=..."
```

**With filter:**

```bash
curl "http://localhost:7890/rpc/formations?status=running" \
  -H "Authorization: MUXI-HMAC key=..., timestamp=..., signature=..."
```

**Response (200 OK):**

```json
{
  "formations": [
    {
      "id": "chat-api",
      "status": "running",
      "port": 8001,
      "pid": 12345,
      "url": "http://localhost:7890/api/chat-api",
      "created_at": "2025-01-17T10:30:00Z",
      "restart_count": 0,
      "healthy": true
    },
    {
      "id": "workflow-engine",
      "status": "running",
      "port": 8002,
      "pid": 12346,
      "url": "http://localhost:7890/api/workflow-engine",
      "created_at": "2025-01-17T10:31:00Z",
      "restart_count": 2,
      "healthy": true
    }
  ],
  "total": 2
}
```

---

### Get Formation

Get detailed information about a specific formation.

**Endpoint:**
```
GET /rpc/formations/{id}
```

**Authentication:** Required (HMAC)

**Path Parameters:**

| Parameter | Type | Description |
|-----------|------|-------------|
| `id` | string | Formation ID |

**Example Request:**

```bash
curl http://localhost:7890/rpc/formations/chat-api \
  -H "Authorization: MUXI-HMAC key=..., timestamp=..., signature=..."
```

**Response (200 OK):**

```json
{
  "id": "chat-api",
  "command": "python app.py",
  "working_dir": "/home/user/.muxi/server/formations/chat-api",
  "env": {
    "MODEL": "gpt-4",
    "PORT": "8001",
    "HOST": "127.0.0.1"
  },
  "status": "running",
  "port": 8001,
  "pid": 12345,
  "url": "http://localhost:7890/api/chat-api",
  "created_at": "2025-01-17T10:30:00Z",
  "started_at": "2025-01-17T10:30:05Z",
  "restart_count": 0,
  "auto_restart": true,
  "healthy": true,
  "last_health_check": "2025-01-17T10:35:00Z"
}
```

**Error Responses:**

| Code | Error | Description |
|------|-------|-------------|
| 401 | `Unauthorized` | Invalid authentication |
| 404 | `Formation not found` | Formation doesn't exist |

---

### Stop Formation

Stop a running formation.

**Endpoint:**
```
POST /rpc/stop/{id}
```

**Authentication:** Required (HMAC)

**Path Parameters:**

| Parameter | Type | Description |
|-----------|------|-------------|
| `id` | string | Formation ID |

**Example Request:**

```bash
curl -X POST http://localhost:7890/rpc/stop/chat-api \
  -H "Authorization: MUXI-HMAC key=..., timestamp=..., signature=..."
```

**Response (200 OK):**

```json
{
  "id": "chat-api",
  "status": "stopped",
  "message": "Formation stopped successfully"
}
```

**Error Responses:**

| Code | Error | Description |
|------|-------|-------------|
| 401 | `Unauthorized` | Invalid authentication |
| 404 | `Formation not found` | Formation doesn't exist |
| 500 | `Failed to stop` | Could not stop process |

---

### Restart Formation

Restart a formation.

**Endpoint:**
```
POST /rpc/restart/{id}
```

**Authentication:** Required (HMAC)

**Path Parameters:**

| Parameter | Type | Description |
|-----------|------|-------------|
| `id` | string | Formation ID |

**Example Request:**

```bash
curl -X POST http://localhost:7890/rpc/restart/chat-api \
  -H "Authorization: MUXI-HMAC key=..., timestamp=..., signature=..."
```

**Response (200 OK):**

```json
{
  "id": "chat-api",
  "status": "running",
  "message": "Formation restarted successfully",
  "restart_count": 3,
  "pid": 12456
}
```

**Error Responses:**

| Code | Error | Description |
|------|-------|-------------|
| 401 | `Unauthorized` | Invalid authentication |
| 404 | `Formation not found` | Formation doesn't exist |
| 500 | `Failed to restart` | Internal server error |

---

### Delete Formation

Permanently delete a formation.

**Endpoint:**
```
DELETE /rpc/delete/{id}
```

**Authentication:** Required (HMAC)

**Path Parameters:**

| Parameter | Type | Description |
|-----------|------|-------------|
| `id` | string | Formation ID |

**Example Request:**

```bash
curl -X DELETE http://localhost:7890/rpc/delete/chat-api \
  -H "Authorization: MUXI-HMAC key=..., timestamp=..., signature=..."
```

**Response (200 OK):**

```json
{
  "id": "chat-api",
  "message": "Formation deleted successfully"
}
```

**Error Responses:**

| Code | Error | Description |
|------|-------|-------------|
| 401 | `Unauthorized` | Invalid authentication |
| 404 | `Formation not found` | Formation doesn't exist |
| 500 | `Failed to delete` | Internal server error |

---

### Get Formation Logs

Retrieve formation logs.

**Endpoint:**
```
GET /rpc/logs/{id}
```

**Authentication:** Required (HMAC)

**Path Parameters:**

| Parameter | Type | Description |
|-----------|------|-------------|
| `id` | string | Formation ID |

**Query Parameters:**

| Parameter | Type | Description |
|-----------|------|-------------|
| `lines` | int | Number of lines to return (default: 100, max: 10000) |

**Example Request:**

```bash
# Get last 100 lines
curl http://localhost:7890/rpc/logs/chat-api \
  -H "Authorization: MUXI-HMAC key=..., timestamp=..., signature=..."

# Get last 1000 lines
curl "http://localhost:7890/rpc/logs/chat-api?lines=1000" \
  -H "Authorization: MUXI-HMAC key=..., timestamp=..., signature=..."
```

**Response (200 OK):**

```
[2025-01-17 10:30:05] [INFO] Starting formation
[2025-01-17 10:30:06] [INFO] Binding to 127.0.0.1:8001
[2025-01-17 10:30:15] [INFO] Health check: OK
[2025-01-17 10:31:23] [INFO] Request: POST /chat
```

**Error Responses:**

| Code | Error | Description |
|------|-------|-------------|
| 401 | `Unauthorized` | Invalid authentication |
| 404 | `Formation not found` | Formation doesn't exist |
| 404 | `Log file not found` | No logs available |

---

## Formation Proxy (`/api/*`)

### Formation Routes

All formation routes are proxied through the server with **no server authentication**.

**Pattern:**
```
/api/{formation_id}/*
```

**Authentication:** Formation-specific (formation handles auth)

**How it works:**

1. Client sends: `http://server.com:7890/api/chat-api/v1/users`
2. MUXI looks up formation `chat-api` (port 8001)
3. MUXI strips `/api/chat-api` prefix
4. MUXI proxies to: `http://127.0.0.1:8001/v1/users`
5. Formation processes request (handles its own auth)
6. MUXI returns response to client

**Example:**

```bash
# Formation: chat-api on localhost:8001
# Formation route: POST /v1/chat

# Access via MUXI proxy:
curl -X POST http://localhost:7890/api/chat-api/v1/chat \
  -H "Authorization: Bearer <formation-token>" \
  -H "Content-Type: application/json" \
  -d '{"message": "Hello!"}'

# MUXI proxies to: http://127.0.0.1:8001/v1/chat
```

**Headers Forwarded:**

All request headers are forwarded to the formation, except:
- `Host` (not forwarded)

Additional headers added by MUXI:
- `X-Forwarded-For: <client-ip>`
- `X-Forwarded-Proto: http` or `https`
- `X-Forwarded-Host: <original-host>`
- `X-Formation-ID: <formation-id>`

**Status Codes:**

- Formation's response code is returned as-is
- `404` if formation not found
- `503` if formation is stopped or unhealthy
- `502` if formation fails to respond

**Query Parameters:**

All query parameters are preserved and forwarded:

```bash
# Client sends:
curl http://localhost:7890/api/my-api/search?q=test&limit=10

# MUXI proxies to:
http://127.0.0.1:8001/search?q=test&limit=10
```

---

### Formation Authentication

**CRITICAL:** Formations handle their own authentication!

MUXI Server does NOT authenticate `/api/*` requests. Each formation implements its own security:

**Examples:**

**OAuth 2.0:**
```bash
curl http://localhost:7890/api/my-api/users \
  -H "Authorization: Bearer eyJhbGciOiJIUzI1NiIs..."
```

**API Key:**
```bash
curl http://localhost:7890/api/my-api/data \
  -H "X-API-Key: sk_live_abc123..."
```

**JWT:**
```bash
curl http://localhost:7890/api/my-api/admin \
  -H "Authorization: JWT eyJhbGciOiJIUzI1NiIs..."
```

**Basic Auth:**
```bash
curl http://localhost:7890/api/my-api/protected \
  -u "username:password"
```

MUXI forwards these headers as-is to the formation.

---

### Formation Logs

Formations can use `/api/{formation}/logs` if they implement a `/logs` endpoint:

```bash
# If formation has GET /logs endpoint:
curl http://localhost:7890/api/my-formation/logs

# This proxies to: http://127.0.0.1:8001/logs
# Formation handles this request
```

**Note:** This is different from server management logs at `/rpc/logs/{id}` which shows stdout/stderr.

---

## Error Responses

All errors follow this format:

```json
{
  "error": "Error type",
  "message": "Human-readable error message",
  "code": 400
}
```

### Common Status Codes

| Code | Status | Description |
|------|--------|-------------|
| 200 | OK | Request succeeded |
| 201 | Created | Resource created |
| 400 | Bad Request | Invalid request parameters |
| 401 | Unauthorized | Authentication required or failed |
| 404 | Not Found | Resource doesn't exist |
| 409 | Conflict | Resource conflict (e.g., already exists) |
| 500 | Internal Server Error | Server error |
| 502 | Bad Gateway | Formation failed to respond |
| 503 | Service Unavailable | Formation stopped or unhealthy |

---

## Port Allocation

MUXI Server allocates ports from a pool:

**Port Range:** 8000-9000 (1000 ports)

**Allocation Strategy:**
- Automatic assignment on deployment
- Port released when formation deleted
- First available port selected
- Port returned in deployment response

**Example:**

```bash
# Deploy formation 1
curl -X POST http://localhost:7890/rpc/deploy -d '{"id":"api-1",...}'
# Response: {"port": 8000, ...}

# Deploy formation 2
curl -X POST http://localhost:7890/rpc/deploy -d '{"id":"api-2",...}'
# Response: {"port": 8001, ...}

# Delete formation 1
curl -X DELETE http://localhost:7890/rpc/delete/api-1
# Port 8000 released back to pool

# Deploy formation 3
curl -X POST http://localhost:7890/rpc/deploy -d '{"id":"api-3",...}'
# Response: {"port": 8000, ...} (reused)
```

---

## Environment Variables

Formations receive these environment variables automatically:

| Variable | Example | Description |
|----------|---------|-------------|
| `PORT` | `8001` | Port to bind to |
| `HOST` | `127.0.0.1` | Host to bind to (always localhost) |
| `_server_id` | `muxi-abc123` | Unique server identifier |
| `_deployment_mode` | `server` | Deployment mode (always "server") |
| `_bind_host` | `127.0.0.1` | Enforced bind host (for security) |
| `_port` | `8001` | Same as PORT (for consistency) |

Plus any custom environment variables specified in deployment request.

**Usage Example (Python):**

```python
import os

# Use HOST and PORT for binding (CRITICAL for security)
host = os.getenv("HOST", "127.0.0.1")  # MUXI provides this
port = int(os.getenv("PORT", 8000))

# Optional: Use server metadata
server_id = os.getenv("_server_id")
deployment_mode = os.getenv("_deployment_mode")

print(f"Starting on {host}:{port}")
print(f"Server ID: {server_id}, Mode: {deployment_mode}")

# Bind to localhost for security!
uvicorn.run(app, host=host, port=port)
```

---

## Complete Example Flow

```bash
# 1. Check server health (public, no auth)
curl http://localhost:7890/health
# {"status": "healthy", "formations": {"total": 0, ...}}

# 2. Deploy formation (requires HMAC auth)
curl -X POST http://localhost:7890/rpc/deploy \
  -H "Authorization: MUXI-HMAC key=MUXI_abc123, timestamp=1705484123, signature=..." \
  -H "Content-Type: application/json" \
  -d '{
    "id": "my-api",
    "command": "python",
    "args": ["app.py"],
    "health_check_url": "/health"
  }'
# {"id": "my-api", "port": 8001, "url": "http://localhost:7890/api/my-api", ...}

# 3. Wait for formation startup
sleep 2

# 4. Access formation via proxy (no server auth, formation handles auth)
curl http://localhost:7890/api/my-api/v1/chat \
  -H "Authorization: Bearer <formation-token>" \
  -H "Content-Type: application/json" \
  -d '{"message": "Hello!"}'
# Formation handles the Bearer token, returns response

# 5. Check formation health through proxy
curl http://localhost:7890/api/my-api/health
# {"status": "healthy"}

# 6. List formations (requires HMAC auth)
curl http://localhost:7890/rpc/formations \
  -H "Authorization: MUXI-HMAC key=..., timestamp=..., signature=..."
# {"formations": [{"id": "my-api", "status": "running", ...}], "total": 1}

# 7. View formation logs (requires HMAC auth)
curl http://localhost:7890/rpc/logs/my-api?lines=100 \
  -H "Authorization: MUXI-HMAC key=..., timestamp=..., signature=..."
# [2025-01-17 10:30:05] [INFO] Starting...

# 8. Restart formation (requires HMAC auth)
curl -X POST http://localhost:7890/rpc/restart/my-api \
  -H "Authorization: MUXI-HMAC key=..., timestamp=..., signature=..."
# {"id": "my-api", "status": "running", "message": "Formation restarted"}

# 9. Stop formation (requires HMAC auth)
curl -X POST http://localhost:7890/rpc/stop/my-api \
  -H "Authorization: MUXI-HMAC key=..., timestamp=..., signature=..."
# {"id": "my-api", "status": "stopped"}

# 10. Delete formation (requires HMAC auth)
curl -X DELETE http://localhost:7890/rpc/delete/my-api \
  -H "Authorization: MUXI-HMAC key=..., timestamp=..., signature=..."
# {"id": "my-api", "message": "Formation deleted successfully"}
```

---

## Configuration

Server configuration in `~/.muxi-server/config.yaml`:

```yaml
server:
  port: 7890                    # Default MUXI port
  host: "0.0.0.0"              # Server binds to all interfaces
  
formations:
  port_range_start: 8000       # Formation port pool start
  port_range_end: 9000         # Formation port pool end
  bind_host: "127.0.0.1"       # Formations bind to localhost
  logs_dir: "~/.muxi-server/logs"
  
  auto_restart: true           # Auto-restart crashed formations
  max_restart_count: 10        # Max restart attempts
  restart_delay: 1             # Delay between restarts (seconds)

auth:
  enabled: true                # HMAC auth for /rpc/*
  credentials_file: "~/.muxi-server/credentials.yaml"
```

---

## Architecture Decisions

### Why Port 7890?

Like Redis (6379), PostgreSQL (5432), and MySQL (3306), MUXI has its own signature port:

- **7890** - Sequential, memorable (7-8-9-0)
- Not used by major services
- Professional port range (1024-49151)
- Easy to remember and type

### Why `/rpc/*` for Management?

- Industry standard (gRPC, JSON-RPC, Kubernetes)
- Clear "remote procedure call" semantics
- Short and efficient
- Separates management from formation traffic

### Why `/api/{formation_id}/*` for Proxy?

- **Zero collisions** - `/api` prefix prevents conflicts
- **Fast routing** - O(1) prefix check
- **Future-proof** - Server can use any path outside `/api`
- **Clear intent** - Immediately obvious what this is
- **Industry standard** - AWS API Gateway, K8s pattern

### Why Localhost-Only Formations?

- **Security** - Formations not directly accessible externally
- **Simplicity** - No firewall configuration needed
- **OS-level** - Operating system enforces localhost binding
- **Proxy control** - All traffic goes through MUXI Server
- **Isolation** - Formations can't see each other

### Why No Auth on `/api/*`?

- **Separation of concerns** - Formations handle their own auth
- **Flexibility** - Each formation can use OAuth, JWT, API keys, etc.
- **Transparency** - MUXI is a dumb proxy, not an auth gateway
- **Standards** - Formations use standard auth patterns
- **No bottleneck** - Auth logic in formations, not server

---

## Phase 3 Preparation: SIF Runtime

**CRITICAL NOTE for Singularity/Apptainer SIF Implementation:**

When implementing SIF container runtime in Phase 3, ensure:

1. **Localhost Binding:**
   ```bash
   singularity run \
     --env HOST=127.0.0.1 \
     --env PORT=8001 \
     formation.sif
   ```

2. **Container Network Configuration:**
   - Containers must bind to `127.0.0.1` inside the container
   - Do NOT use bridge networking that exposes ports externally
   - Use host networking with localhost binding

3. **Port Mapping:**
   ```bash
   # Correct: Host networking, app binds to localhost
   singularity run --network=none --env HOST=127.0.0.1 formation.sif
   
   # Wrong: Bridge network exposing ports
   # singularity run --network=bridge formation.sif
   ```

4. **Security Verification:**
   - Test that formation port is NOT accessible from external IP
   - Verify formation IS accessible from 127.0.0.1
   - Ensure MUXI proxy can reach formation

This architecture decision (localhost-only) is foundational and must be maintained through all runtime implementations.

---

## Future Enhancements

### Not Yet Implemented

**Rate Limiting:**
- Per-formation rate limits
- Global server rate limits
- Headers: `X-RateLimit-Remaining`, `X-RateLimit-Reset`

**Metrics:**
- `/metrics` endpoint (Prometheus format)
- Formation request counts
- Response times
- Error rates

**Webhooks:**
- `formation.deployed`
- `formation.crashed`
- `formation.restarted`
- `formation.deleted`

**Streaming Logs:**
- `/rpc/logs/{id}?follow=true` (Server-Sent Events)

**Health Check Configuration:**
- Configurable health check intervals
- Custom health check endpoints per formation
- Health check timeouts

**Multi-Server:**
- Server registration API
- Formation telemetry aggregation
- Multi-server deployment coordination

---

## SDKs (Planned)

### Official SDKs (Phase 2+)

**Python:**
```bash
pip install muxi-sdk
```

**JavaScript/TypeScript:**
```bash
npm install @muxi/sdk
```

**Go:**
```bash
go get github.com/muxi-ai/sdk-go
```

### SDK Example (Python):

```python
from muxi import MuxiClient

# Initialize client with server credentials
client = MuxiClient(
    server="http://localhost:7890",
    key="MUXI_abc123",
    secret="sk_xyz789"
)

# Deploy formation
formation = client.deploy_formation(
    id="chat-api",
    command="python",
    args=["app.py"],
    env={"MODEL": "gpt-4"}
)

# List formations
formations = client.list_formations(status="running")

# Get formation details
info = client.get_formation("chat-api")

# Restart formation
client.restart_formation("chat-api")

# Stop formation
client.stop_formation("chat-api")

# Delete formation
client.delete_formation("chat-api")

# Get logs
logs = client.get_logs("chat-api", lines=100)
```

---

## Next Steps

- [Getting Started Guide](./getting-started.md) - Quick setup and first deployment
- [Authentication Guide](./authentication.md) - HMAC signature generation
- [Formation Development](./formations.md) - Building formations for MUXI
- [Configuration Reference](./configuration.md) - Server configuration options
- [Troubleshooting](./troubleshooting.md) - Common issues and solutions

---

**Questions?** See the [Troubleshooting Guide](./troubleshooting.md) or open an issue on GitHub.
