# CLI-Server Protocol

**Version:** 1.0  
**Status:** Specification  
**Last Updated:** 2025-10-17

---

## Overview

This document specifies how the MUXI CLI communicates with MUXI Server. This is the contract between the two components.

---

## Authentication

All CLI → Server requests use **HMAC-SHA256 authentication** (see AUTH.md for full details).

### Authorization Header

```
Authorization: MUXI-HMAC key={KEY}, timestamp={TIMESTAMP}, signature={SIGNATURE}
```

**Signature Computation:**
```
message = "{timestamp};{method};{path}"
signature = base64(HMAC-SHA256(secret, message))
```

**Example:**
```
timestamp = "1729180800"
method = "POST"
path = "/formations/deploy"
message = "1729180800;POST;/formations/deploy"
signature = base64(HMAC-SHA256("sk_secret_123", message))

Authorization: MUXI-HMAC key=MUXI_key_abc, timestamp=1729180800, signature=YWJjZGVm...
```

---

## Deploy Formation

### Endpoint

```
POST /formations/deploy
```

### Request Headers

```
Authorization: MUXI-HMAC key=..., timestamp=..., signature=...
Content-Type: application/gzip
Content-Length: <size>
```

### Request Body

**Format:** Gzipped tarball containing:

```
formation-bundle.tar.gz
├── formation.yaml          # Required: Formation configuration
├── app.py                  # Formation code
├── requirements.txt        # Python dependencies (optional)
├── .env                    # Environment variables (optional)
└── ... (other files)
```

**formation.yaml structure:**

```yaml
id: my-api                  # Required: Unique formation ID
name: "My API"              # Optional: Human-readable name
version: "1.0.0"            # Optional: Formation version
runtime: "python:3.13"      # Optional: Runtime specification

# Optional metadata
metadata:
  description: "My awesome API"
  author: "user@example.com"
  tags: ["chat", "ai"]

# Optional environment variables
env:
  MODEL_NAME: "gpt-4"
  API_KEY: "${SECRET_API_KEY}"  # Can reference secrets
```

### Creating the Bundle (CLI Side)

```bash
# CLI creates the bundle like this:
tar czf formation-bundle.tar.gz \
  formation.yaml \
  app.py \
  requirements.txt \
  .env
```

**Python example:**

```python
import tarfile
import gzip
import io

def create_formation_bundle(files: dict) -> bytes:
    """
    Create a gzipped tarball from files.
    
    Args:
        files: Dict of filename -> content
        
    Returns:
        Gzipped tarball bytes
    """
    # Create tar in memory
    tar_buffer = io.BytesIO()
    with tarfile.open(fileobj=tar_buffer, mode='w:gz') as tar:
        for filename, content in files.items():
            info = tarfile.TarInfo(name=filename)
            info.size = len(content)
            tar.addfile(info, io.BytesIO(content))
    
    return tar_buffer.getvalue()

# Usage
bundle = create_formation_bundle({
    'formation.yaml': yaml_content.encode(),
    'app.py': app_code.encode(),
    'requirements.txt': deps.encode()
})
```

### Server Processing

**Server steps:**

1. **Receive gzipped tarball**
2. **Extract to temporary directory**
   ```
   /tmp/muxi-formations/{uuid}/
   ├── formation.yaml
   ├── app.py
   └── ...
   ```
3. **Read formation.yaml** to get `id`
4. **Validate formation ID** (unique, valid format)
5. **Allocate port** from pool (e.g., 8001)
6. **Spawn formation** with environment:
   ```bash
   PORT=8001 python app.py
   ```
7. **Register mapping:** `formation_id` → `port`
8. **Move extracted files** to permanent location:
   ```
   ~/.muxi/server/formations/{formation_id}/
   ```
9. **Wait for health check** (optional)
10. **Return success response**

### Response (201 Created)

```json
{
  "id": "my-api",
  "status": "running",
  "port": 8001,
  "pid": 12345,
  "url": "http://localhost:3000/v1/my-api",
  "created_at": "2025-10-17T10:30:00Z"
}
```

### Error Responses

**400 Bad Request - Invalid bundle:**
```json
{
  "error": "Invalid formation bundle",
  "message": "formation.yaml not found in bundle",
  "code": 400
}
```

**400 Bad Request - Invalid formation.yaml:**
```json
{
  "error": "Invalid formation.yaml",
  "message": "Missing required field: id",
  "code": 400
}
```

**409 Conflict - Formation exists:**
```json
{
  "error": "Formation already exists",
  "message": "Formation with id 'my-api' is already deployed",
  "code": 409
}
```

**500 Internal Server Error - Spawn failed:**
```json
{
  "error": "Failed to spawn formation",
  "message": "Command exited with code 1",
  "code": 500
}
```

---

## Formation Runtime Contract

### Environment Variables

The server **MUST** provide these environment variables to every formation:

| Variable | Type | Description | Example |
|----------|------|-------------|---------|
| `PORT` | int | Port to listen on | `8001` |
| `FORMATION_ID` | string | Formation identifier | `my-api` |
| `MUXI_SERVER_URL` | string | Server base URL | `http://localhost:3000` |
| `MUXI_ENV` | string | Environment (dev/staging/prod) | `production` |

**Example formation startup:**

```python
import os
from fastapi import FastAPI

app = FastAPI()

# Formation MUST read PORT from environment
port = int(os.getenv('PORT', 8000))
formation_id = os.getenv('FORMATION_ID')

@app.get("/health")
def health():
    return {"status": "healthy", "formation_id": formation_id}

if __name__ == "__main__":
    import uvicorn
    uvicorn.run(app, host="0.0.0.0", port=port)
```

### Health Check Endpoint

Every formation **MUST** implement:

```
GET /health
```

**Response (200 OK):**
```json
{
  "status": "healthy"
}
```

**Or any 2xx status code with any body.**

Server will check this endpoint every 30 seconds.

---

## HTTP Proxy Routing

### URL Pattern

```
{server_url}/v1/{formation_id}/{path}
```

**Examples:**

```
Original request:
  POST http://localhost:3000/v1/my-api/chat

Proxied to formation:
  POST http://localhost:8001/chat

Headers:
  - All original headers forwarded (except Host)
  - X-Forwarded-For: client IP
  - X-Forwarded-Proto: http/https
  - X-Formation-ID: my-api
```

### Routing Logic

```
1. Parse URL: /v1/{formation_id}/{remaining_path}
2. Look up formation_id in registry → get port
3. If not found → 404 Formation Not Found
4. If formation unhealthy → 503 Service Unavailable
5. Proxy request to http://localhost:{port}/{remaining_path}
6. Forward response to client
```

### Path Rewriting

**Client sends:**
```
POST /v1/my-api/chat/completions
```

**Server forwards to formation:**
```
POST /chat/completions
```

The `/v1/{formation_id}` prefix is **stripped**.

### Header Forwarding

**Server adds these headers:**

```
X-Forwarded-For: <client-ip>
X-Forwarded-Proto: http
X-Forwarded-Host: localhost:3000
X-Formation-ID: my-api
X-Request-ID: <uuid>
```

**Server preserves:**
- All original request headers (except `Host`)
- All response headers from formation

### Error Responses

**404 Formation Not Found:**
```json
{
  "error": "Formation not found",
  "message": "No formation with id 'my-api'",
  "code": 404
}
```

**503 Service Unavailable:**
```json
{
  "error": "Formation unavailable",
  "message": "Formation 'my-api' is not healthy",
  "code": 503,
  "status": "unhealthy"
}
```

**502 Bad Gateway:**
```json
{
  "error": "Bad gateway",
  "message": "Formation did not respond",
  "code": 502
}
```

---

## List Formations

### Endpoint

```
GET /formations
```

### Request

```
GET /formations?status=running&limit=10&offset=0
Authorization: MUXI-HMAC key=..., timestamp=..., signature=...
```

### Response (200 OK)

```json
{
  "formations": [
    {
      "id": "my-api",
      "name": "My API",
      "version": "1.0.0",
      "status": "running",
      "port": 8001,
      "pid": 12345,
      "url": "http://localhost:3000/v1/my-api",
      "health": "healthy",
      "created_at": "2025-10-17T10:30:00Z",
      "updated_at": "2025-10-17T10:30:00Z",
      "restart_count": 0
    }
  ],
  "total": 1,
  "limit": 10,
  "offset": 0
}
```

---

## Get Formation Details

### Endpoint

```
GET /formations/{id}
```

### Response (200 OK)

```json
{
  "id": "my-api",
  "name": "My API",
  "version": "1.0.0",
  "status": "running",
  "port": 8001,
  "pid": 12345,
  "url": "http://localhost:3000/v1/my-api",
  "health": "healthy",
  "working_dir": "/home/user/.muxi/server/formations/my-api",
  "env": {
    "PORT": "8001",
    "FORMATION_ID": "my-api",
    "MODEL_NAME": "gpt-4"
  },
  "created_at": "2025-10-17T10:30:00Z",
  "updated_at": "2025-10-17T10:30:00Z",
  "started_at": "2025-10-17T10:30:05Z",
  "restart_count": 0,
  "last_health_check": "2025-10-17T10:35:00Z"
}
```

---

## Delete Formation

### Endpoint

```
DELETE /formations/{id}
```

### Response (200 OK)

```json
{
  "id": "my-api",
  "message": "Formation deleted successfully"
}
```

**Server actions:**
1. Stop the formation process (SIGTERM, then SIGKILL)
2. Release the port back to pool
3. Remove from registry
4. Optionally delete files from `~/.muxi/server/formations/{id}/`

---

## Stop/Restart Formation

### Stop

```
POST /formations/{id}/stop
```

**Response:**
```json
{
  "id": "my-api",
  "status": "stopped",
  "message": "Formation stopped successfully"
}
```

### Restart

```
POST /formations/{id}/restart
```

**Response:**
```json
{
  "id": "my-api",
  "status": "restarting",
  "message": "Formation restarting",
  "restart_count": 3
}
```

---

## Get Logs

### Endpoint

```
GET /formations/{id}/logs?lines=100
```

### Response (200 OK)

```json
{
  "id": "my-api",
  "logs": [
    "[2025-10-17 10:30:05] INFO: Starting formation my-api",
    "[2025-10-17 10:30:06] INFO: Listening on port 8001",
    "[2025-10-17 10:30:15] INFO: Health check passed"
  ],
  "lines": 3,
  "total_lines": 150
}
```

---

## Server Health

### Endpoint

```
GET /health
```

**No authentication required.**

### Response (200 OK)

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

---

## CLI Implementation Notes

### Profile Storage

CLI stores credentials in `~/.muxi/profiles.yaml`:

```yaml
default:
  key: "MUXI_abc123"
  secret: "sk_xyz789"
  server: "http://localhost:3000"

production:
  key: "MUXI_prod_key"
  secret: "sk_prod_secret"
  server: "https://api.myserver.com"
```

### Commands

```bash
# Deploy formation
muxi formation deploy ./my-formation/

# List formations
muxi formation list

# Get details
muxi formation get my-api

# Logs
muxi formation logs my-api --follow

# Delete
muxi formation delete my-api

# Profile management
muxi config add-profile production \
  --key=MUXI_prod_key \
  --secret=sk_prod_secret \
  --server=https://api.myserver.com
```

### Deploy Command Flow

```bash
$ muxi formation deploy ./my-formation/

1. Read formation.yaml from ./my-formation/formation.yaml
2. Validate formation.yaml (has required 'id' field)
3. Create gzipped tarball of directory
4. Load credentials from ~/.muxi/profiles.yaml (default profile)
5. Compute HMAC signature
6. POST to /formations/deploy with gzip body
7. Display result
```

---

## Version History

| Version | Date | Changes |
|---------|------|---------|
| 1.0 | 2025-10-17 | Initial specification |

---

## See Also

- [AUTH.md](./AUTH.md) - HMAC authentication details
- [PRD.md](./PRD.md) - Product requirements
- [docs/api-reference.md](./docs/api-reference.md) - User-facing API docs
