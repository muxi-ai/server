# API Reference v2

Complete HTTP API reference for MUXI Server.

---

## Base URL

```
http://localhost:7890
```

**Production:**

```
https://api.yourserver.com
```

> **Port 7890** is the official MUXI Server port

---

## Architecture Overview

MUXI Server provides three distinct routing namespaces:

```
┌────────────────────────────────────────────────────────┐
│ MUXI Server (Port 7890)                                │
├────────────────────────────────────────────────────────┤
│                                                        │
│ /health                → Server health (public)        │
│ /ping                  → Connectivity test (public)    │
│ /docs                  → Documentation redirect        │
│                                                        │
│ /rpc/formations/*      → Formation management (auth)   │
│ /rpc/server/*          → Server management (auth)      │
│                                                        │
│ /api/{formation_id}/*  → Formation proxy (no auth)     │
│                                                        │
└────────────────────────────────────────────────────────┘
```

---

## Complete API Routes

### **Public Endpoints (no auth):**
```
GET  /health                            → Server health check
GET  /ping                              → Connectivity test
GET  /docs                              → Documentation (redirect)
```

### **Formation Management (HMAC auth):**
```
POST   /rpc/formations                  → Deploy new formation
GET    /rpc/formations                  → List formations
GET    /rpc/formations/{id}             → Get formation details
PUT    /rpc/formations/{id}             → Update formation (versioning)
DELETE /rpc/formations/{id}             → Delete formation
```

### **Formation Actions (HMAC auth):**
```
POST   /rpc/formations/{id}/stop        → Stop formation
POST   /rpc/formations/{id}/restart     → Restart formation
POST   /rpc/formations/{id}/rollback    → Rollback to previous version
```

### **Server Management (HMAC auth):**
```
GET    /rpc/server/status               → Server status and statistics
GET    /rpc/server/logs                 → Server audit logs
```

### **Formation Proxy (no auth):**
```
*      /api/{formation_id}/*            → Proxy to formation
                                          (formation handles auth)
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

MUXI Server acts as a transparent proxy - it forwards requests as-is. Each formation implements its own auth (OAuth, JWT, API keys, etc.).

### Public Endpoints

These endpoints require **no authentication**:

- `GET /health` - Server health check
- `GET /ping` - Connectivity test
- `GET /docs` - Documentation (redirects to muxi.org/docs)

---

## Security Architecture

### Localhost-Only Formation Binding

**CRITICAL:** Formations bind to `127.0.0.1:{port}`, NOT `0.0.0.0:{port}`.

```
Formation binds to: 127.0.0.1:8001

External access:
  curl http://server.com:8001/api
  → CONNECTION REFUSED ❌ (Port not exposed to internet)

Via MUXI Proxy:
  curl http://server.com:7890/api/my-formation/api
  → MUXI → 127.0.0.1:8001/api → ✅ Works (localhost)
```

**Why:**
- Formations only accessible via MUXI Server proxy
- Direct external access is impossible
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
  "formations": 5,
  "port_pool": {
    "total": 1000,
    "available": 995,
    "allocated": 5
  }
}
```

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

### Documentation

Access comprehensive MUXI Server documentation.

**Endpoint:**

```
GET /docs
```

**Example Request:**

```bash
curl http://localhost:7890/docs
```

**Response (302 Found):**

Redirects to: `https://muxi.org/docs`

**Use Cases:**
- Access complete documentation
- API reference
- Getting started guides
- Configuration examples
- Troubleshooting

**Note:** This endpoint redirects to the canonical documentation at muxi.org, which includes API references, guides, and examples.

---

## Formation Management

All formation management endpoints require HMAC authentication.

---

### Deploy Formation

Deploy a new formation to the server.

**Endpoint:**

```
POST /rpc/formations
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

**Example Request:**

```bash
curl -X POST http://localhost:7890/rpc/formations \
  -H "Authorization: MUXI-HMAC key=MUXI_abc123, timestamp=1705484123, signature=..." \
  -H "Content-Type: application/json" \
  -d '{
    "id": "chat-api",
    "command": "python",
    "args": ["app.py"],
    "env": {
      "MODEL": "gpt-4"
    },
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
  "version": 1,
  "created_at": "2025-01-17T10:30:00Z"
}
```

**Error Responses:**

| Code | Error | Description |
|------|-------|-------------|
| 400 | `Formation already exists` | Formation with this ID exists |
| 400 | `Invalid formation ID` | ID contains invalid characters or reserved name |
| 401 | `Unauthorized` | Invalid or missing authentication |
| 409 | `No ports available` | Port pool exhausted (8000-9000) |
| 500 | `Failed to spawn formation` | Internal server error |

**Reserved Formation IDs:**

These IDs cannot be used (would conflict with server routes):
- `health`, `ping`, `rpc`, `server`, `admin`, `metrics`, `api`

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
      "version": 2,
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
      "version": 1,
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
  "working_dir": "/var/lib/muxi/formations/chat-api/current",
  "env": {
    "MODEL": "gpt-4",
    "PORT": "8001",
    "HOST": "127.0.0.1"
  },
  "status": "running",
  "port": 8001,
  "pid": 12345,
  "url": "http://localhost:7890/api/chat-api",
  "version": 2,
  "has_previous_version": true,
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

### Update Formation

Update a formation to a new version (keeps previous version for rollback).

**Endpoint:**

```
PUT /rpc/formations/{id}
```

**Authentication:** Required (HMAC)

**Request:** Multipart form data with bundle file

**Example Request:**

```bash
curl -X PUT http://localhost:7890/rpc/formations/chat-api \
  -H "Authorization: MUXI-HMAC key=..., timestamp=..., signature=..." \
  -F "bundle=@./chat-api-v2.tar.gz"
```

**Response (200 OK):**

```json
{
  "id": "chat-api",
  "status": "running",
  "version": 3,
  "previous_version": 2,
  "message": "Formation updated successfully"
}
```

**How it works:**

1. Stops current formation
2. Moves `current/` → `previous/` (backup)
3. Extracts new bundle to `current/`
4. Updates version metadata
5. Starts formation with new version

**Error Responses:**

| Code | Error | Description |
|------|-------|-------------|
| 401 | `Unauthorized` | Invalid authentication |
| 404 | `Formation not found` | Formation doesn't exist |
| 400 | `Invalid bundle` | Bundle format invalid |
| 500 | `Update failed` | Internal server error |

---

### Delete Formation

Permanently delete a formation.

**Endpoint:**

```
DELETE /rpc/formations/{id}
```

**Authentication:** Required (HMAC)

**Example Request:**

```bash
curl -X DELETE http://localhost:7890/rpc/formations/chat-api \
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

## Formation Actions

### Stop Formation

Stop a running formation.

**Endpoint:**

```
POST /rpc/formations/{id}/stop
```

**Authentication:** Required (HMAC)

**Example Request:**

```bash
curl -X POST http://localhost:7890/rpc/formations/chat-api/stop \
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

---

### Restart Formation

Restart a formation.

**Endpoint:**

```
POST /rpc/formations/{id}/restart
```

**Authentication:** Required (HMAC)

**Example Request:**

```bash
curl -X POST http://localhost:7890/rpc/formations/chat-api/restart \
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

---

### Rollback Formation

Rollback formation to previous version.

**Endpoint:**

```
POST /rpc/formations/{id}/rollback
```

**Authentication:** Required (HMAC)

**Example Request:**

```bash
curl -X POST http://localhost:7890/rpc/formations/chat-api/rollback \
  -H "Authorization: MUXI-HMAC key=..., timestamp=..., signature=..."
```

**Response (200 OK):**

```json
{
  "id": "chat-api",
  "status": "running",
  "version": 2,
  "previous_version": 3,
  "message": "Rolled back to previous version"
}
```

**How it works:**

1. Stops formation
2. Swaps `current/` ↔ `previous/`
3. Updates version metadata
4. Starts formation with previous version

**Error Responses:**

| Code | Error | Description |
|------|-------|-------------|
| 401 | `Unauthorized` | Invalid authentication |
| 404 | `Formation not found` | Formation doesn't exist |
| 400 | `No previous version` | No backup available to rollback to |
| 500 | `Rollback failed` | Internal server error |

---

## Server Management

### Server Status

Get server status and statistics.

**Endpoint:**

```
GET /rpc/server/status
```

**Authentication:** Required (HMAC)

**Example Request:**

```bash
curl http://localhost:7890/rpc/server/status \
  -H "Authorization: MUXI-HMAC key=..., timestamp=..., signature=..."
```

**Response (200 OK):**

```json
{
  "server": {
    "id": "muxi-prod-abc123",
    "version": "1.0.0",
    "uptime": 86400
  },
  "formations": {
    "total": 5,
    "running": 4,
    "stopped": 1
  },
  "port_pool": {
    "total": 1000,
    "available": 995,
    "allocated": 5
  },
  "runtime": {
    "goroutines": 42,
    "go_version": "go1.21.5"
  }
}
```

**Fields:**

| Field | Type | Description |
|-------|------|-------------|
| `server.id` | string | Unique server identifier |
| `server.version` | string | MUXI Server version |
| `server.uptime` | int | Server uptime in seconds |
| `formations.total` | int | Total formations |
| `formations.running` | int | Running formations |
| `formations.stopped` | int | Stopped formations |
| `port_pool.total` | int | Total ports in pool |
| `port_pool.available` | int | Available ports |
| `port_pool.allocated` | int | Allocated ports |

---

### Server Logs

Get server audit logs.

**Endpoint:**

```
GET /rpc/server/logs
```

**Authentication:** Required (HMAC)

**Query Parameters:**

| Parameter | Type | Description |
|-----------|------|-------------|
| `lines` | int | Number of lines to return (default: 100, max: 10000) |

**Example Request:**

```bash
# Get last 100 lines
curl http://localhost:7890/rpc/server/logs \
  -H "Authorization: MUXI-HMAC key=..., timestamp=..., signature=..."

# Get last 500 lines
curl "http://localhost:7890/rpc/server/logs?lines=500" \
  -H "Authorization: MUXI-HMAC key=..., timestamp=..., signature=..."
```

**Response (200 OK):**

```
{"time":"2025-01-17T10:30:15Z","level":"info","method":"POST","path":"/rpc/formations","auth_key":"MUXI_abc123","remote_addr":"192.168.1.100","status":201,"duration_ms":142}
{"time":"2025-01-17T10:31:22Z","level":"info","method":"GET","path":"/rpc/formations","auth_key":"MUXI_abc123","remote_addr":"192.168.1.100","status":200,"duration_ms":5}
{"time":"2025-01-17T10:32:10Z","level":"warn","method":"POST","path":"/rpc/formations/my-api/stop","auth_key":"MUXI_abc123","remote_addr":"192.168.1.100","status":404,"error":"formation not found"}
```

**Log Format:** JSON lines (newline-delimited JSON)

**Log Fields:**

| Field | Description |
|-------|-------------|
| `time` | ISO 8601 timestamp |
| `level` | Log level (info, warn, error) |
| `method` | HTTP method |
| `path` | Request path |
| `auth_key` | HMAC key used (not secret!) |
| `remote_addr` | Client IP address |
| `status` | HTTP status code |
| `duration_ms` | Request duration in milliseconds |
| `error` | Error message (if any) |

**Use Cases:**
- Audit trail of all management operations
- Security monitoring
- Debugging authentication issues
- Performance monitoring

---

## Formation Proxy

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

MUXI Server does NOT authenticate `/api/*` requests. Each formation implements its own security.

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

MUXI forwards these headers as-is to the formation.

---

### Formation Logs

**Formations implement their own `/logs` endpoint if needed:**

```bash
# If formation has GET /logs endpoint:
curl http://localhost:7890/api/my-formation/logs

# This proxies to: http://127.0.0.1:8001/logs
# Formation handles this request
```

**Server logs (stdout/stderr):** Stored at `/var/log/muxi/formations/{id}.log` (file system, not API endpoint)

---

## Formation Versioning

### Version Management

MUXI Server automatically tracks formation versions:

**Directory Structure:**

```
/var/lib/muxi/formations/
  my-api/
    current/              ← Active version
    previous/             ← Previous version (for rollback)
    version.json          ← Version metadata
```

**Version Metadata (`version.json`):**

```json
{
  "current_version": 2,
  "current_deployed_at": "2025-01-17T14:00:00Z",
  "current_bundle_hash": "sha256:ghi789...",
  "previous_version": 1,
  "previous_deployed_at": "2025-01-17T12:00:00Z",
  "previous_bundle_hash": "sha256:def456..."
}
```

### Deployment Flow

**Initial deployment:**

```bash
POST /rpc/formations {"id": "my-api", ...}
→ Extracts to my-api/current/
→ version.json: current_version=1
```

**Update (new version):**

```bash
PUT /rpc/formations/my-api {...}
→ Moves my-api/current/ → my-api/previous/
→ Extracts new to my-api/current/
→ version.json: current_version=2, previous_version=1
→ Restarts formation
```

**Rollback:**

```bash
POST /rpc/formations/my-api/rollback
→ Swaps: current/ ↔ previous/
→ Updates version.json
→ Restarts formation
```

**Configuration:**

```yaml
formations:
  keep_backups: 1  # Number of backups to keep (default: 1)
```

---

## Environment Variables

Formations receive these environment variables automatically:

| Variable | Example | Description |
|----------|---------|-------------|
| `PORT` | `8001` | Port to bind to |
| `HOST` | `127.0.0.1` | **Host to bind to (always localhost)** |
| `FORMATION_ID` | `chat-api` | Formation identifier |
| `MUXI_SERVER_URL` | `http://localhost:7890` | MUXI Server URL |
| `MUXI_ENV` | `production` | Environment (always "production") |
| `_bind_host` | `127.0.0.1` | Enforced bind host (for security) |
| `_port` | `8001` | Same as PORT (for consistency) |

Plus any custom environment variables specified in the deployment request.

**Usage Example (Python):**

```python
import os

# CRITICAL: Use HOST and PORT for binding (security!)
host = os.getenv("HOST", "127.0.0.1")
port = int(os.getenv("PORT", 8000))

# Optional: Use metadata
formation_id = os.getenv("FORMATION_ID")
server_url = os.getenv("MUXI_SERVER_URL")

print(f"Starting {formation_id} on {host}:{port}")
print(f"Proxied via: {server_url}/api/{formation_id}")

# Bind to localhost for security!
uvicorn.run(app, host=host, port=port)
```

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
- Port released when formation is deleted
- First available port selected

**Example:**

```bash
# Deploy formation 1 → Port 8000
# Deploy formation 2 → Port 8001
# Delete formation 1 → Port 8000 released
# Deploy formation 3 → Port 8000 (reused)
```

---

## Configuration

Server configuration in `/etc/muxi/server/config.yaml`:

```yaml
server:
  id: "muxi-prod-abc123"
  port: 7890                      # MUXI Server port
  host: "127.0.0.1"               # Bind to localhost (use reverse proxy)
  
formations:
  port_range_start: 8000          # Formation port pool start
  port_range_end: 9000            # Formation port pool end
  bind_host: "127.0.0.1"          # Formations bind to localhost
  data_dir: "/var/lib/muxi/formations"
  keep_backups: 1                 # Number of version backups to keep

logging:
  level: "info"
  audit_log: "/var/log/muxi/audit.log"

auth:
  enabled: true
  credentials_file: "/etc/muxi/server/credentials.yaml"
```

---

## Complete Example Flow

```bash
# 1. Check server health (public, no auth)
curl http://localhost:7890/health
# {"status": "healthy", "formations": 0}

# 2. Access documentation (redirects to muxi.org/docs)
curl http://localhost:7890/docs
# 302 redirect to https://muxi.org/docs

# 3. Deploy formation (requires HMAC auth)
curl -X POST http://localhost:7890/rpc/formations \
  -H "Authorization: MUXI-HMAC key=MUXI_abc123, timestamp=1705484123, signature=..." \
  -H "Content-Type: application/json" \
  -d '{
    "id": "my-api",
    "command": "python",
    "args": ["app.py"],
    "health_check_url": "/health"
  }'
# {"id": "my-api", "port": 8001, "version": 1, ...}

# 4. Access formation via proxy (formation handles auth)
curl http://localhost:7890/api/my-api/v1/chat \
  -H "Authorization: Bearer <formation-token>" \
  -d '{"message": "Hello!"}'

# 5. Update formation (new version)
curl -X PUT http://localhost:7890/rpc/formations/my-api \
  -H "Authorization: MUXI-HMAC ..." \
  -F "bundle=@./my-api-v2.tar.gz"
# {"id": "my-api", "version": 2, "previous_version": 1}

# 6. Rollback if needed
curl -X POST http://localhost:7890/rpc/formations/my-api/rollback \
  -H "Authorization: MUXI-HMAC ..."
# {"id": "my-api", "version": 1, "previous_version": 2}

# 7. Check server status
curl http://localhost:7890/rpc/server/status \
  -H "Authorization: MUXI-HMAC ..."
# {"server": {...}, "formations": {"total": 1, "running": 1}, ...}

# 8. View audit logs
curl http://localhost:7890/rpc/server/logs?lines=50 \
  -H "Authorization: MUXI-HMAC ..."
# (JSON lines audit log)

# 9. Stop formation
curl -X POST http://localhost:7890/rpc/formations/my-api/stop \
  -H "Authorization: MUXI-HMAC ..."

# 10. Delete formation
curl -X DELETE http://localhost:7890/rpc/formations/my-api \
  -H "Authorization: MUXI-HMAC ..."
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
- **OS-level** - The Operating system enforces localhost binding
- **Proxy control** - All traffic goes through MUXI Server
- **Isolation** - Formations can't see each other

### Why No Auth on `/api/*`?

- **Separation of concerns** - Formations handle their own auth
- **Flexibility** - Each formation can use OAuth, JWT, API keys, etc.
- **Transparency** - MUXI is a dumb proxy, not an auth gateway
- **Standards** - Formations use standard auth patterns
- **No bottleneck** - Auth logic in formations, not server

### Why Keep Only 1 Backup?

- **Simplicity** - Covers 90% of rollback needs
- **Storage** - Doesn't waste disk space
- **Fast rollback** - Simple swap operation
- **Configurable** - Can increase if needed

---

## System Installation

**MUXI Server is installed system-wide for production:**

```
/usr/local/bin/muxi-server     ← Binary
/etc/muxi/server/              ← Configuration
/var/lib/muxi/                 ← Data (formations, registry)
/var/log/muxi/                 ← Logs (audit + formations)
```

**Install via script:**

```bash
curl -fsSL https://install.muxi.ai | sudo bash
```

See [Installation Guide](./installation.md) for details.

---

## Next Steps

- [Getting Started Guide](./getting-started.md) - Quick setup and first deployment
- [Authentication Guide](./authentication.md) - HMAC signature generation
- [Formation Development](./formations.md) - Building formations for MUXI
- [Configuration Reference](./configuration.md) - Server configuration options
- [Installation Guide](./installation.md) - System installation and setup
- [Troubleshooting](./troubleshooting.md) - Common issues and solutions

---

**Questions?** See the [Troubleshooting Guide](./troubleshooting.md) or open an issue on GitHub.
