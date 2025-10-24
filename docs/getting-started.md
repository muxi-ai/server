# Getting Started

Get your MUXI Server up and running in 5 minutes.

---

## Prerequisites

**Operating System:**
- Linux (Ubuntu 20.04+, Debian 11+, RHEL 8+)
- macOS 10.15+ (Intel or Apple Silicon)
- Windows 10/11 (x64 or ARM64)

**Hardware:**
- 2GB RAM minimum
- Terminal/PowerShell access

**Optional (for SIF runtime):**
- Docker Desktop (Windows/macOS)
- Singularity/Apptainer (Linux)

---

## Step 1: Install MUXI Server

### Quick Install

**Linux/macOS:**
```bash
curl -sSL https://get.muxi.ai | sudo bash
```

**Windows (PowerShell):**
```powershell
irm https://install.muxi.ai/windows.ps1 | iex
```

This installs:
- `muxi-server` - Server binary
- Creates configuration directory
- Sets up runtime directories

### Verify Installation

**Linux/macOS:**
```bash
muxi-server version
```

**Windows:**
```powershell
muxi-server version
```

**Expected output:**
```
MUXI Server v0.20251024.0
Commit: abc1234
Build: 2025-10-24T12:00:00Z
Go: 1.24.0
Platform: windows/amd64  (or linux/amd64, darwin/arm64, etc.)
```

---

## Step 2: Initialize Server

Generate authentication credentials:

**Linux/macOS:**
```bash
muxi-server init
```

**Windows:**
```powershell
muxi-server init
```

**Output:**
```
🔐 Initializing MUXI Server...

Server Name: My MUXI Server
Port [7890]: 7890
Admin Email: admin@example.com

✅ Configuration created
✅ Authentication keys generated

📁 Configuration: ~/.muxi/server/config.yaml (Unix)
📁 Configuration: %APPDATA%\muxi\server\config.yaml (Windows)

🔑 Authentication Keys:
   Key:    muxi_pk_EXAMPLE123...
   Secret: muxi_sk_EXAMPLE789...

⚠️  Keep your secret secure!
```

**Save these credentials!** You'll need them for the CLI tool to deploy formations.

---

## Step 3: Start the Server

**Linux/macOS:**
```bash
muxi-server serve
```

**Windows (foreground):**
```powershell
muxi-server serve
```

**Windows (background):**
```powershell
Start-Process muxi-server -ArgumentList "serve" -WindowStyle Hidden
```

**Expected Output:**
```
{"level":"info","time":"2025-10-24T10:00:00Z","message":"Starting MUXI Server"}
{"level":"info","config_dir":"/Users/you/.muxi/server","message":"Configuration loaded"}
{"level":"info","install_type":"User-level","message":"Installation type detected"}
{"level":"info","message":"Process manager initialized"}
{"level":"info","message":"Formation registry loaded"}
{"level":"info","port":7890,"message":"HTTP server listening"}
{"level":"info","message":"✓ MUXI Server ready"}
```

The server is now running on `http://localhost:7890`.

**Test it:**
```bash
curl http://localhost:7890/health
# {"status":"healthy","timestamp":"2025-10-24T10:00:00Z"}
```

---

## Step 4: Verify Server is Running

Open a new terminal and check the health endpoint:

```bash
curl http://localhost:7890/health
```

**Expected Response:**
```json
{
  "success": true,
  "data": {
    "status": "ok",
    "formations": 0,
    "port_pool": {
      "available": 1000,
      "allocated": 0,
      "total": 1000
    }
  }
}
```

✅ Server is running!

---

## Step 5: Deploy Your First Formation

### Option A: Using the Test Formation

Create a simple test formation:

```bash
cd ~/.muxi/server
cat > test-formation.yaml << 'EOF'
name: test-assistant
version: 1.0
description: Test formation

agents:
  - name: assistant
    role: helpful assistant
    model: gpt-4

api_keys:
  admin: "fa_test_admin_key"
  client: "fc_test_client_key"
EOF
```

### Option B: Use Dummy App (Development)

For testing the server without a real formation:

```bash
# Deploy dummy Python app
muxi formation deploy \
  --id test-api \
  --command python3 \
  --args test/dummy_app.py

# Or via API with authentication
```

---

## Step 6: Deploy via API

Since the CLI isn't built yet, deploy using the API with proper authentication:

### Generate HMAC Signature

```python
# save as sign_request.py
import hmac
import hashlib
import base64
import time
import sys

def sign_request(secret, method, path):
    timestamp = str(int(time.time()))
    message = f"{timestamp};{method};{path}"
    signature = hmac.new(
        secret.encode(),
        message.encode(),
        hashlib.sha256
    ).digest()
    sig_b64 = base64.b64encode(signature).decode()
    return timestamp, sig_b64

if __name__ == "__main__":
    secret = sys.argv[1]
    method = sys.argv[2]
    path = sys.argv[3]
    ts, sig = sign_request(secret, method, path)
    print(f"timestamp={ts}")
    print(f"signature={sig}")
```

### Deploy Formation

```bash
# Get credentials from ~/.muxi/server/config.yaml
KEY="MUXI_e8f3a9b2c4d1"
SECRET="sk_9f2e8d7c6b5a4f3e2d1c0b9a8f7e6d5c"

# Generate signature
python3 sign_request.py "$SECRET" "POST" "/rpc/formations/deploy"

# Use the output in curl
TIMESTAMP="1705484123"  # from script output
SIGNATURE="base64_signature_here"  # from script output

curl -X POST http://localhost:7890/rpc/formations/deploy \
  -H "Authorization: MUXI-HMAC key=$KEY, timestamp=$TIMESTAMP, signature=$SIGNATURE" \
  -H "Content-Type: application/json" \
  -d '{
    "id": "my-first-formation",
    "command": "python3",
    "args": ["test/dummy_app.py", "--port", "8001"]
  }'
```

**Response:**
```json
{
  "success": true,
  "data": {
    "formation_id": "my-first-formation",
    "port": 8001,
    "status": "starting",
    "url": "http://localhost:7890",
    "health_url": "http://localhost:8001/health",
    "pid": 12345
  }
}
```

---

## Step 7: Verify Formation is Running

```bash
# Check formation health
curl http://localhost:8001/health

# List all formations (requires auth)
curl http://localhost:7890/rpc/formations \
  -H "Authorization: MUXI-HMAC key=$KEY, timestamp=$TIMESTAMP, signature=$SIGNATURE"
```

---

## Step 8: Access Your Formation

Formations are accessible on their allocated ports:

```bash
# Health check (no auth from formation)
curl http://localhost:8001/health

# Chat endpoint (depends on formation's auth)
curl -X POST http://localhost:8001/chat \
  -H "Content-Type: application/json" \
  -d '{"message": "Hello!", "user_id": "user_123"}'
```

---

## Development Mode (Disable Authentication)

For local testing, you can disable authentication:

**Edit `~/.muxi/server/config.yaml`:**
```yaml
auth:
  enabled: false  # Disable auth for development
```

**Restart server:**
```bash
# Stop server (Ctrl+C)
# Start again
muxi-server start
```

Now you can deploy without authentication:

```bash
curl -X POST http://localhost:7890/rpc/formations/deploy \
  -H "Content-Type: application/json" \
  -d '{
    "id": "test-formation",
    "command": "python3",
    "args": ["test/dummy_app.py"]
  }'
```

⚠️ **Warning:** Never disable auth in production!

---

## Next Steps

Now that your server is running:

1. **[Learn about Configuration](./configuration.md)** - Customize server settings
2. **[Set up Authentication](./authentication.md)** - Secure your server properly
3. **[Manage Formations](./formations.md)** - Deploy and manage formations
4. **[Read API Reference](./api-reference.md)** - Integrate programmatically

---

## Common Issues

### Server Won't Start

**Error:** `Failed to bind to port 7890`

**Solution:** Port 7890 (MUXI Port) is already in use. Either:
- Stop the process using port 7890
- Change the port in `~/.muxi/server/config.yaml`:
  ```yaml
  server:
    port: 7891
  ```

### Can't Deploy Formation

**Error:** `401 Unauthorized`

**Solution:** Check authentication:
- Verify key/secret in `~/.muxi/server/config.yaml`
- Ensure HMAC signature is correct
- Check timestamp is current (not expired)

### Formation Crashes Immediately

**Check logs:**
```bash
tail -f ~/.muxi/server/logs/formation-{id}-out.log
tail -f ~/.muxi/server/logs/formation-{id}-err.log
```

---

**Next:** [Installation Guide →](./installation.md)
