# Testing MUXI Server

Quick guide for testing the HTTP proxy functionality.

---

## Prerequisites

```bash
# Install dependencies
pip install fastapi uvicorn

# Build the server
cd src
go build -o muxi-server ./cmd/server
```

---

## Quick Test

### 1. Start the Server

```bash
cd src
go run ./cmd/server
```

You should see:
```
{"level":"info","time":"...","message":"Starting MUXI Server"}
{"level":"info","addr":"0.0.0.0:3000","message":"Starting HTTP server"}
```

### 2. Run the Test Script

In another terminal:

```bash
cd src/test
./test_proxy.sh
```

This script will:
- ✅ Deploy a test formation
- ✅ Test proxy routing (`/v1/{formation_id}/*`)
- ✅ Compare direct vs proxied access
- ✅ Verify error handling (404 for missing formations)

---

## Manual Testing

### Deploy Formation

```bash
curl -X POST http://localhost:3000/formations/deploy \
  -H "Content-Type: application/json" \
  -d '{
    "id": "my-api",
    "command": "python test/dummy_app.py"
  }'
```

Response:
```json
{
  "id": "my-api",
  "port": 8001,
  "status": "running",
  "pid": 12345
}
```

### Access Formation via Proxy

**Health Check:**
```bash
curl http://localhost:3000/v1/my-api/health
```

Response:
```json
{
  "status": "ok",
  "service": "dummy-formation",
  "uptime_seconds": 12.34
}
```

**Chat Endpoint:**
```bash
curl -X POST http://localhost:3000/v1/my-api/chat \
  -H "Content-Type: application/json" \
  -d '{"message": "Hello!", "user_id": "test-123"}'
```

Response:
```json
{
  "response": "Echo: Hello!",
  "user_id": "test-123",
  "timestamp": 1729180800.123
}
```

### Compare Direct vs Proxy

**Direct access (formation port):**
```bash
curl http://localhost:8001/health
```

**Proxied access (server):**
```bash
curl http://localhost:3000/v1/my-api/health
```

Both should return the same response!

### List Formations

```bash
curl http://localhost:3000/formations
```

Response:
```json
{
  "formations": [
    {
      "id": "my-api",
      "port": 8001,
      "status": "running",
      "pid": 12345
    }
  ]
}
```

### Cleanup

```bash
# Stop and delete formation
curl -X DELETE http://localhost:3000/formations/my-api
```

---

## Test with Environment Variables

The dummy formation reads `PORT` from environment:

```bash
# Test formation standalone
PORT=9000 FORMATION_ID=test python test/dummy_app.py
```

Then access it:
```bash
curl http://localhost:9000/health
```

---

## Testing Proxy Error Handling

### Non-existent Formation (404)

```bash
curl -v http://localhost:3000/v1/nonexistent/health
```

Expected:
```
HTTP/1.1 404 Not Found
{"error": "Formation not found", "message": "No formation with id 'nonexistent'", "code": 404}
```

### Stopped Formation (503)

```bash
# Deploy and then stop
curl -X POST http://localhost:3000/formations/deploy \
  -d '{"id": "test", "command": "python test/dummy_app.py"}'

# Kill the process manually
kill <PID>

# Try to access
curl -v http://localhost:3000/v1/test/health
```

Expected:
```
HTTP/1.1 503 Service Unavailable
```

---

## Path Rewriting Examples

| Client Request | Proxied To |
|----------------|------------|
| `GET /v1/my-api/health` | `GET http://localhost:8001/health` |
| `POST /v1/my-api/chat` | `POST http://localhost:8001/chat` |
| `GET /v1/my-api/` | `GET http://localhost:8001/` |
| `GET /v1/my-api/deep/nested/path` | `GET http://localhost:8001/deep/nested/path` |

The `/v1/{formation_id}` prefix is **always stripped**.

---

## Headers Added by Proxy

The proxy adds these headers to formation requests:

```
X-Forwarded-For: <client-ip>
X-Forwarded-Proto: http
X-Forwarded-Host: localhost:3000
X-Formation-ID: my-api
```

Test it:

```bash
# Formation code to print headers
@app.get("/debug/headers")
async def debug_headers(request: Request):
    return dict(request.headers)
```

```bash
curl http://localhost:3000/v1/my-api/debug/headers
```

---

## Performance Testing

### Simple Load Test

```bash
# Install hey (HTTP load testing tool)
# brew install hey   # macOS
# go install github.com/rakyll/hey@latest

# Test proxy throughput
hey -n 1000 -c 10 http://localhost:3000/v1/my-api/health
```

### Measure Latency

```bash
# Direct access
time curl -s http://localhost:8001/health > /dev/null

# Proxied access
time curl -s http://localhost:3000/v1/my-api/health > /dev/null
```

Proxy overhead should be < 1ms.

---

## Troubleshooting

### Port Already in Use

```bash
# Check what's using port 3000
lsof -i :3000

# Or use different port
MUXI_SERVER_PORT=8080 go run ./cmd/server
```

### Formation Not Starting

```bash
# Check server logs
# Server logs to stdout

# Check formation logs (future)
tail -f ~/.muxi/server/logs/my-api.log
```

### Proxy Returns 404

```bash
# Verify formation is running
curl http://localhost:3000/formations

# Check formation ID matches
# IDs are case-sensitive!
```

### Proxy Returns 502 Bad Gateway

```bash
# Formation process crashed or not responding
# Check if process is running
ps aux | grep dummy_app

# Check formation port
lsof -i :<port>
```

---

## Next Steps

- [ ] Add HMAC authentication to management endpoints
- [ ] Implement formation bundle upload (gzipped tarball)
- [ ] Add `formation.yaml` parsing
- [ ] Build CLI tool with HMAC signing
- [ ] Add streaming logs endpoint
- [ ] Add WebSocket support for proxy

---

## See Also

- [CLI-PROTOCOL.md](../CLI-PROTOCOL.md) - CLI-Server communication spec
- [AUTH.md](../AUTH.md) - Authentication design
- [docs/](../docs/) - User documentation
