# API Reference

Complete HTTP API reference for MUXI Server.

---

## Base URL

```
http://localhost:3000
```

**Production:**
```
https://api.yourserver.com
```

---

## Authentication

All management API endpoints require HMAC authentication.

### Authorization Header

```
Authorization: MUXI-HMAC key={KEY}, timestamp={TIMESTAMP}, signature={SIGNATURE}
```

**Example:**
```
Authorization: MUXI-HMAC key=MUXI_abc123, timestamp=1705484123, signature=YWJjZGVm...
```

See [Authentication Guide](./authentication.md) for details.

### Public Endpoints

These endpoints **do not** require authentication:

- `GET /health` - Server health check
- `/{formation_id}/*` - Formation proxy (formation handles auth)

---

## Management API

### Deploy Formation

Deploy a new formation to the server.

**Endpoint:**
```
POST /formations/deploy
```

**Authentication:** Required

**Request Body:**

```json
{
  "id": "string (required)",
  "command": "string (required)",
  "env": {
    "KEY": "value"
  },
  "working_dir": "string (optional)"
}
```

**Fields:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `id` | string | Yes | Unique formation identifier (lowercase, hyphens, 3-50 chars) |
| `command` | string | Yes | Command to execute (e.g., `python app.py`) |
| `env` | object | No | Environment variables as key-value pairs |
| `working_dir` | string | No | Working directory for the command |

**Example Request:**

```bash
curl -X POST http://localhost:3000/formations/deploy \
  -H "Authorization: MUXI-HMAC key=..., timestamp=..., signature=..." \
  -H "Content-Type: application/json" \
  -d '{
    "id": "chat-api",
    "command": "python app.py",
    "env": {
      "MODEL": "gpt-4",
      "PORT": "8001"
    }
  }'
```

**Response (201 Created):**

```json
{
  "id": "chat-api",
  "status": "running",
  "port": 8001,
  "pid": 12345,
  "url": "http://localhost:3000/chat-api",
  "created_at": "2025-01-17T10:30:00Z"
}
```

**Error Responses:**

| Code | Error | Description |
|------|-------|-------------|
| 400 | `Formation already exists` | Formation with this ID exists |
| 400 | `Invalid formation ID` | ID contains invalid characters |
| 400 | `Command is required` | Missing command field |
| 401 | `Unauthorized` | Invalid authentication |
| 500 | `Failed to spawn formation` | Internal server error |

---

### List Formations

Get a list of all formations.

**Endpoint:**
```
GET /formations
```

**Authentication:** Required

**Query Parameters:**

| Parameter | Type | Description |
|-----------|------|-------------|
| `status` | string | Filter by status: `running`, `stopped`, `crashed` |
| `limit` | int | Maximum number of results (default: 100) |
| `offset` | int | Pagination offset (default: 0) |

**Example Request:**

```bash
curl http://localhost:3000/formations \
  -H "Authorization: MUXI-HMAC key=..., timestamp=..., signature=..."
```

**With filters:**

```bash
curl "http://localhost:3000/formations?status=running&limit=10" \
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
      "url": "http://localhost:3000/chat-api",
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
      "url": "http://localhost:3000/workflow-engine",
      "created_at": "2025-01-17T10:31:00Z",
      "updated_at": "2025-01-17T10:31:00Z",
      "restart_count": 2,
      "health": "healthy"
    }
  ],
  "total": 2,
  "limit": 100,
  "offset": 0
}
```

**Error Responses:**

| Code | Error | Description |
|------|-------|-------------|
| 401 | `Unauthorized` | Invalid authentication |
| 500 | `Internal server error` | Server error |

---

### Get Formation

Get detailed information about a specific formation.

**Endpoint:**
```
GET /formations/{id}
```

**Authentication:** Required

**Path Parameters:**

| Parameter | Type | Description |
|-----------|------|-------------|
| `id` | string | Formation ID |

**Example Request:**

```bash
curl http://localhost:3000/formations/chat-api \
  -H "Authorization: MUXI-HMAC key=..., timestamp=..., signature=..."
```

**Response (200 OK):**

```json
{
  "id": "chat-api",
  "command": "python app.py",
  "working_dir": "/home/user/formations/chat-api",
  "env": {
    "MODEL": "gpt-4",
    "PORT": "8001"
  },
  "status": "running",
  "port": 8001,
  "pid": 12345,
  "url": "http://localhost:3000/chat-api",
  "created_at": "2025-01-17T10:30:00Z",
  "updated_at": "2025-01-17T10:30:00Z",
  "started_at": "2025-01-17T10:30:05Z",
  "restart_count": 0,
  "health": "healthy",
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
POST /formations/{id}/stop
```

**Authentication:** Required

**Path Parameters:**

| Parameter | Type | Description |
|-----------|------|-------------|
| `id` | string | Formation ID |

**Example Request:**

```bash
curl -X POST http://localhost:3000/formations/chat-api/stop \
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
| 409 | `Formation already stopped` | Formation is not running |

---

### Restart Formation

Restart a formation.

**Endpoint:**
```
POST /formations/{id}/restart
```

**Authentication:** Required

**Path Parameters:**

| Parameter | Type | Description |
|-----------|------|-------------|
| `id` | string | Formation ID |

**Example Request:**

```bash
curl -X POST http://localhost:3000/formations/chat-api/restart \
  -H "Authorization: MUXI-HMAC key=..., timestamp=..., signature=..."
```

**Response (200 OK):**

```json
{
  "id": "chat-api",
  "status": "restarting",
  "message": "Formation restarting",
  "restart_count": 3
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
DELETE /formations/{id}
```

**Authentication:** Required

**Path Parameters:**

| Parameter | Type | Description |
|-----------|------|-------------|
| `id` | string | Formation ID |

**Example Request:**

```bash
curl -X DELETE http://localhost:3000/formations/chat-api \
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
GET /formations/{id}/logs
```

**Authentication:** Required

**Path Parameters:**

| Parameter | Type | Description |
|-----------|------|-------------|
| `id` | string | Formation ID |

**Query Parameters:**

| Parameter | Type | Description |
|-----------|------|-------------|
| `lines` | int | Number of lines to return (default: 100) |
| `follow` | bool | Stream logs (default: false) |

**Example Request:**

```bash
# Get last 100 lines
curl http://localhost:3000/formations/chat-api/logs \
  -H "Authorization: MUXI-HMAC key=..., timestamp=..., signature=..."

# Get last 1000 lines
curl "http://localhost:3000/formations/chat-api/logs?lines=1000" \
  -H "Authorization: MUXI-HMAC key=..., timestamp=..., signature=..."
```

**Response (200 OK):**

```json
{
  "id": "chat-api",
  "logs": [
    "[2025-01-17 10:30:05] [INFO] Starting formation",
    "[2025-01-17 10:30:06] [INFO] Listening on port 8001",
    "[2025-01-17 10:30:15] [INFO] Health check: OK"
  ],
  "lines": 3,
  "total_lines": 150
}
```

**Error Responses:**

| Code | Error | Description |
|------|-------|-------------|
| 401 | `Unauthorized` | Invalid authentication |
| 404 | `Formation not found` | Formation doesn't exist |
| 404 | `Log file not found` | No logs available |

---

## Public API

### Server Health

Check server health status.

**Endpoint:**
```
GET /health
```

**Authentication:** Not required

**Example Request:**

```bash
curl http://localhost:3000/health
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

---

## Proxy API

### Formation Routes

All formation routes are proxied through the server.

**Pattern:**
```
/{formation_id}/*
```

**Authentication:** Formation-specific (not server auth)

**Example:**

```bash
# Formation: chat-api listening on port 8001
# Route: POST /chat

# Direct (not recommended):
curl http://localhost:8001/chat -d '{"message": "Hello"}'

# Via proxy (recommended):
curl http://localhost:3000/chat-api/chat -d '{"message": "Hello"}'
```

**How it works:**

1. Request comes to: `http://localhost:3000/chat-api/chat`
2. Server looks up formation: `chat-api`
3. Server finds formation port: `8001`
4. Server proxies to: `http://localhost:8001/chat`
5. Formation handles authentication and responds
6. Server returns response to client

**Headers forwarded:**
- All request headers (except `Host`)
- Formation response headers

**Status codes:**
- Formation response code is returned as-is
- `404` if formation not found
- `503` if formation is unhealthy

---

## Error Responses

All errors follow this format:

```json
{
  "error": "Error type",
  "message": "Human-readable error message",
  "code": 400,
  "request_id": "abc123"
}
```

### Common Status Codes

| Code | Status | Description |
|------|--------|-------------|
| 200 | OK | Request succeeded |
| 201 | Created | Resource created |
| 400 | Bad Request | Invalid request parameters |
| 401 | Unauthorized | Authentication required or failed |
| 403 | Forbidden | Authenticated but not authorized |
| 404 | Not Found | Resource doesn't exist |
| 409 | Conflict | Resource conflict (e.g., already exists) |
| 429 | Too Many Requests | Rate limit exceeded |
| 500 | Internal Server Error | Server error |
| 503 | Service Unavailable | Server overloaded or maintenance |

---

## Rate Limiting

**Current:** Not implemented

**Future:**
- 1000 requests per minute per API key
- Header: `X-RateLimit-Remaining: 950`
- Header: `X-RateLimit-Reset: 1705484123`

---

## Pagination

List endpoints support pagination:

**Query Parameters:**
- `limit` - Results per page (default: 100, max: 1000)
- `offset` - Number of items to skip (default: 0)

**Response:**
```json
{
  "formations": [...],
  "total": 250,
  "limit": 100,
  "offset": 0,
  "next": "/formations?limit=100&offset=100"
}
```

---

## Webhooks (Future)

**Not yet implemented**

Future webhook support for events:
- `formation.deployed`
- `formation.crashed`
- `formation.restarted`
- `formation.deleted`

---

## SDKs

### Official SDKs

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

### SDK Example

**Python:**

```python
from muxi import MuxiClient

# SDK for server management (uses server credentials)
client = MuxiClient(
    server="http://localhost:3000",
    key="MUXI_abc123",
    secret="sk_xyz789"
)

# Deploy formation
formation = client.deploy_formation(
    id="chat-api",
    command="python app.py"
)

# List formations
formations = client.list_formations()

# Stop formation
client.stop_formation("chat-api")
```

---

## Versioning

**Current Version:** v1 (implicit)

**Future:** Explicit versioning in URL or header

```
/v2/formations/deploy
```

Or:

```
API-Version: 2
```

---

## CORS

**Default:** Disabled

**Enable in config:**

```yaml
server:
  cors_enabled: true
  cors_origins:
    - "https://app.example.com"
    - "https://dashboard.example.com"
```

---

## Examples

### Complete Deployment Flow

```bash
# 1. Check server health
curl http://localhost:3000/health

# 2. Deploy formation
curl -X POST http://localhost:3000/formations/deploy \
  -H "Authorization: MUXI-HMAC key=..., timestamp=..., signature=..." \
  -H "Content-Type: application/json" \
  -d '{
    "id": "my-api",
    "command": "python app.py"
  }'

# 3. Wait for startup (or poll status)
sleep 5

# 4. Check formation health
curl http://localhost:3000/my-api/health

# 5. Use formation
curl http://localhost:3000/my-api/chat \
  -H "Content-Type: application/json" \
  -d '{"message": "Hello!"}'

# 6. View logs
curl http://localhost:3000/formations/my-api/logs \
  -H "Authorization: MUXI-HMAC key=..., timestamp=..., signature=..."

# 7. Clean up
curl -X DELETE http://localhost:3000/formations/my-api \
  -H "Authorization: MUXI-HMAC key=..., timestamp=..., signature=..."
```

---

## Next Steps

- [Getting Started](./getting-started.md)
- [Authentication Guide](./authentication.md)
- [Managing Formations](./formations.md)

---

**Need help?** See the [Troubleshooting Guide](./troubleshooting.md)
