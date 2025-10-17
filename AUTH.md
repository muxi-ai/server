# MUXI Server Authentication

**Version:** 1.0  
**Status:** Design Document  
**Last Updated:** 2025-01-17

---

## Overview

MUXI Server uses **HMAC-based authentication** (like AWS) with key/secret pairs. Authentication is **two-layered**:

1. **Layer 1: Server Management API** - Protected with server admin credentials
2. **Layer 2: Formation API** - Proxied transparently, formations handle their own auth

---

## Design Principles

1. ✅ **Simple**: Just key + secret (like AWS credentials)
2. ✅ **Secure**: HMAC signing, no secrets transmitted
3. ✅ **Separation**: Server management ≠ Formation usage
4. ✅ **File-based**: No environment variables, only config files
5. ✅ **CLI-only**: SDKs talk to formations, not server management

---

## Architecture

```
┌─────────────────────────────────────────────────────┐
│ CLI (Admin)                                         │
│                                                     │
│ ~/.muxi/profiles.yaml                               │
│   key: "MUXI_abc123"                                │
│   secret: "sk_xyz789"                               │
└──────────────┬──────────────────────────────────────┘
               │
               │ HMAC-signed request
               │ Authorization: MUXI-HMAC key=..., timestamp=..., signature=...
               ↓
┌─────────────────────────────────────────────────────┐
│ MUXI Server                                         │
│                                                     │
│ ~/.muxi/server/config.yaml                          │
│   auth:                                             │
│     enabled: true                                   │
│     key: "MUXI_abc123"                              │
│     secret: "sk_xyz789"                             │
│                                                     │
│ Layer 1: Management API (Auth Required)            │
│   POST   /formations/deploy    🔒                  │
│   GET    /formations           🔒                  │
│   DELETE /formations/{id}      🔒                  │
│                                                     │
│ Layer 2: Proxy API (Transparent)                   │
│   /{formation_id}/*  →  Formation runtime          │
│   - No auth (pass-through)                         │
│   - Forward all headers                            │
└─────────────────────────────────────────────────────┘
               │
               │ Transparent proxy
               ↓
┌─────────────────────────────────────────────────────┐
│ Formation Runtime (Port 8001)                       │
│                                                     │
│ Formation handles its own auth:                     │
│   - Admin key: "fa_admin_123"                       │
│   - Client key: "fc_client_456"                     │
│                                                     │
│ Endpoints:                                          │
│   POST /chat                                        │
│   POST /workflow                                    │
│   GET  /health                                      │
└─────────────────────────────────────────────────────┘
```

---

## Configuration Files

### Server Configuration

**Location:** `~/.muxi/server/config.yaml`

```yaml
server:
  port: 3000
  host: "0.0.0.0"

auth:
  enabled: true
  key: "MUXI_abc123def456"
  secret: "sk_xyz789abc012def345"

formations:
  port_range_start: 8000
  port_range_end: 9000
  # ... other settings
```

**Key Format:**
- `key`: Public identifier, prefix `MUXI_`
- `secret`: Secret key, prefix `sk_`

**Generation:**
```bash
$ muxi-server init

🔐 Generating authentication credentials...

   Key:    MUXI_e8f3a9b2c4d1
   Secret: sk_9f2e8d7c6b5a4f3e2d1c0b9a8f7e6d5c

📝 Saved to: ~/.muxi/server/config.yaml

⚠️  Keep your secret secure!
   Add to CLI profile: muxi config add-profile
```

**Manual Editing:**
Users can edit `~/.muxi/server/config.yaml` directly to set/change credentials.

---

### CLI Configuration

**Location:** `~/.muxi/profiles.yaml`

```yaml
default:
  key: "MUXI_abc123def456"
  secret: "sk_xyz789abc012"
  servers:
    - "https://api.myserver.com"

production:
  key: "MUXI_prod_key_123"
  secret: "sk_prod_secret_456"
  servers:
    - "https://prod1.myserver.com"
    - "https://prod2.myserver.com"

staging:
  key: "MUXI_staging_789"
  secret: "sk_staging_012"
  servers:
    - "https://staging.myserver.com"
```

**CLI Usage:**
```bash
# Use default profile
muxi formation deploy app.yaml

# Use specific profile
muxi formation deploy app.yaml --profile=production

# Deploy to all servers in profile
muxi formation deploy app.yaml --profile=production --all-servers
```

**Configuration Commands:**
```bash
# Add profile
muxi config add-profile production \
  --key=MUXI_prod_123 \
  --secret=sk_prod_456 \
  --server=https://prod.myserver.com

# List profiles
muxi config list-profiles

# Show profile
muxi config show-profile production

# Delete profile
muxi config delete-profile staging
```

---

## Authentication Flow

### HMAC Signature Process

```
┌─────────┐                                  ┌──────────┐
│   CLI   │                                  │  Server  │
└────┬────┘                                  └────┬─────┘
     │                                            │
     │ 1. Load credentials from profile          │
     │    key = "MUXI_abc123"                    │
     │    secret = "sk_xyz789"                   │
     │                                            │
     │ 2. Prepare request                        │
     │    method = "POST"                        │
     │    path = "/formations/deploy"           │
     │    timestamp = unix_timestamp()           │
     │    body = {...}                           │
     │                                            │
     │ 3. Create signing string                  │
     │    message = "timestamp;method;path"      │
     │    message = "1705484123;POST;/formations/deploy"
     │                                            │
     │ 4. Sign with HMAC-SHA256                  │
     │    signature = HMAC-SHA256(secret, message)
     │    signature = base64(signature)          │
     │                                            │
     │ 5. Send request                           │
     ├───────────────────────────────────────────>│
     │ Authorization: MUXI-HMAC key=MUXI_abc123, │
     │                timestamp=1705484123,      │
     │                signature=base64(...)      │
     │                                            │
     │                        6. Validate         │
     │                           - Extract params │
     │                           - Check key      │
     │                           - Recompute sig  │
     │                           - Compare        │
     │                           - Check time     │
     │                                            │
     │                        7. ✅ or ❌         │
     │<────────────────────────────────────────────│
     │ Response (200 or 401)                     │
```

---

## Authentication Header Format

```
Authorization: MUXI-HMAC key=<KEY>, timestamp=<TIMESTAMP>, signature=<SIGNATURE>
```

**Example:**
```
Authorization: MUXI-HMAC key=MUXI_abc123, timestamp=1705484123, signature=YWJjZGVmZ2hpamtsbW5vcA==
```

**Parameters:**
- `key`: Public key identifier (e.g., `MUXI_abc123`)
- `timestamp`: Unix timestamp in seconds (e.g., `1705484123`)
- `signature`: Base64-encoded HMAC-SHA256 signature

---

## Signature Generation

### Signing String Format

```
{timestamp};{method};{path}
```

**Examples:**
```
1705484123;POST;/formations/deploy
1705484200;GET;/formations
1705484300;DELETE;/formations/my-api
```

### HMAC-SHA256 Calculation

**Pseudocode:**
```python
def generate_signature(secret, timestamp, method, path):
    message = f"{timestamp};{method};{path}"
    signature = hmac.new(
        secret.encode('utf-8'),
        message.encode('utf-8'),
        hashlib.sha256
    ).digest()
    return base64.b64encode(signature).decode('utf-8')
```

**Go Implementation:**
```go
func computeHMAC(secret, timestamp, method, path string) string {
    message := fmt.Sprintf("%s;%s;%s", timestamp, method, path)
    h := hmac.New(sha256.New, []byte(secret))
    h.Write([]byte(message))
    signature := h.Sum(nil)
    return base64.StdEncoding.EncodeToString(signature)
}
```

---

## Server-Side Validation

### Validation Steps

1. **Extract Authorization Header**
   - Parse `MUXI-HMAC` format
   - Extract `key`, `timestamp`, `signature`

2. **Validate Key**
   - Check if `key` matches `config.auth.key`
   - Constant-time comparison to prevent timing attacks

3. **Check Timestamp**
   - Parse timestamp to Unix time
   - Verify timestamp is within 5 minutes of current time
   - Prevents replay attacks

4. **Recompute Signature**
   - Create signing string: `{timestamp};{method};{path}`
   - Compute HMAC-SHA256 with `config.auth.secret`
   - Base64 encode result

5. **Compare Signatures**
   - Use constant-time comparison
   - If match → ✅ authenticated
   - If mismatch → ❌ 401 Unauthorized

### Error Responses

**Missing Authorization Header:**
```json
{
  "error": "Unauthorized",
  "message": "Missing authorization header",
  "code": 401
}
```

**Invalid Key:**
```json
{
  "error": "Unauthorized",
  "message": "Invalid key",
  "code": 401
}
```

**Expired Timestamp:**
```json
{
  "error": "Unauthorized",
  "message": "Request expired (timestamp too old)",
  "code": 401
}
```

**Invalid Signature:**
```json
{
  "error": "Unauthorized",
  "message": "Invalid signature",
  "code": 401
}
```

---

## Protected Endpoints

### Management API (Auth Required)

All server management endpoints require authentication:

- `POST /formations/deploy` - Deploy new formation
- `GET /formations` - List all formations
- `GET /formations/{id}` - Get formation details
- `POST /formations/{id}/restart` - Restart formation
- `POST /formations/{id}/stop` - Stop formation
- `DELETE /formations/{id}` - Delete formation
- `GET /formations/{id}/logs` - Get formation logs

### Proxy API (No Auth)

Formation proxy endpoints are **transparent** (no server auth):

- `/{formation_id}/*` - All requests proxied to formation

**Why?** The formation handles its own authentication. Server just proxies.

**Example:**
```bash
# Server doesn't check auth, just forwards
curl https://api.myserver.com/my-formation/chat \
  -H "Authorization: Bearer fc_formation_client_key" \
  -d '{"message": "Hello"}'

# Formation receives:
#   POST /chat
#   Authorization: Bearer fc_formation_client_key
# Formation validates its own key
```

---

## Security Considerations

### Advantages of HMAC

1. **Secret Never Transmitted**
   - Only signature is sent over network
   - Even if HTTPS is compromised, secret is safe

2. **Replay Attack Prevention**
   - Timestamp expires in 5 minutes
   - Old requests automatically rejected

3. **Tamper Detection**
   - Any modification to request invalidates signature
   - Method, path changes detected

4. **No Shared Secrets in Transit**
   - Unlike Bearer tokens, secret stays on both ends
   - Similar to AWS SigV4

### Timestamp Window

**Default:** 5 minutes

```yaml
auth:
  timestamp_tolerance: 300  # 5 minutes in seconds
```

**Why 5 minutes?**
- Allows for clock skew between client/server
- Short enough to prevent replay attacks
- Long enough to handle network delays

### Key Rotation

To rotate credentials:

1. **Generate new key/secret:**
   ```bash
   muxi-server init --rotate
   ```

2. **Update server config:**
   ```yaml
   auth:
     key: "MUXI_new_key_789"
     secret: "sk_new_secret_012"
   ```

3. **Update CLI profiles:**
   ```bash
   muxi config add-profile production \
     --key=MUXI_new_key_789 \
     --secret=sk_new_secret_012 \
     --server=https://api.myserver.com
   ```

4. **Restart server:**
   ```bash
   systemctl restart muxi-server
   ```

---

## Development Mode

For local development, authentication can be disabled:

```yaml
# ~/.muxi/server/config.yaml
auth:
  enabled: false
```

**When disabled:**
- All management endpoints are open
- No authentication required
- **⚠️ Only use on localhost!**

**Use case:** Local testing, development environments

---

## CLI Examples

### Deploy Formation

```bash
# Default profile
muxi formation deploy app.yaml

# Specific profile
muxi formation deploy app.yaml --profile=production

# Override server
muxi formation deploy app.yaml --server=https://custom.com
```

### List Formations

```bash
muxi formation list
muxi formation list --profile=production
```

### Delete Formation

```bash
muxi formation delete my-api
muxi formation delete my-api --profile=production
```

---

## SDK Usage (Formation Access)

SDKs talk to **formations**, not the server management API.

```python
from muxi import MuxiClient

# SDK connects to formation (proxied through server)
client = MuxiClient(
    server="https://api.myserver.com/my-formation",  # Formation endpoint
    admin_key="fa_admin_formation_key",               # Formation's admin key
    client_key="fc_client_formation_key"              # Formation's client key
)

# These go to the formation, not server management
response = client.chat("Hello!", user_id="user_123")
workflow = client.run_workflow("onboarding", params={...})

client.close()
```

**Key Point:** SDK uses formation's credentials, NOT server credentials!

---

## Implementation Checklist

### Server Side

- [ ] Add auth config to `pkg/config/config.go`
- [ ] Implement HMAC signature generation
- [ ] Implement HMAC signature validation
- [ ] Create auth middleware for management API
- [ ] Apply middleware to protected endpoints
- [ ] Exclude proxy routes from auth
- [ ] Add `muxi-server init` command (generate credentials)
- [ ] Add auth enable/disable config option
- [ ] Add timestamp tolerance config
- [ ] Add audit logging for auth failures
- [ ] Add tests for auth logic

### CLI Side (Future)

- [ ] Profile management commands
- [ ] Load credentials from `~/.muxi/profiles.yaml`
- [ ] HMAC signature generation in HTTP client
- [ ] Send `Authorization` header with requests
- [ ] Handle 401 errors gracefully
- [ ] Profile switching (`--profile` flag)
- [ ] Multi-server deployment support

---

## Testing

### Test Cases

1. **Valid Request**
   - Correct key, secret, timestamp
   - Should return 200

2. **Invalid Key**
   - Wrong key
   - Should return 401

3. **Invalid Signature**
   - Tampered signature
   - Should return 401

4. **Expired Timestamp**
   - Timestamp > 5 minutes old
   - Should return 401

5. **Auth Disabled**
   - Config: `auth.enabled: false`
   - Should allow all requests

6. **Proxy Routes**
   - `/{formation_id}/*`
   - Should not require server auth

### Manual Testing

```bash
# 1. Start server with auth
muxi-server start

# 2. Try without auth (should fail)
curl -X POST http://localhost:3000/formations/deploy
# Expected: 401 Unauthorized

# 3. Try with valid auth (should succeed)
# Generate signature with key/secret
curl -X POST http://localhost:3000/formations/deploy \
  -H "Authorization: MUXI-HMAC key=MUXI_abc123, timestamp=..., signature=..."
# Expected: 201 Created

# 4. Try proxy route (should work without auth)
curl http://localhost:3000/my-formation/health
# Expected: 200 OK (from formation)
```

---

## References

- AWS Signature Version 4: https://docs.aws.amazon.com/general/latest/gr/signature-version-4.html
- HMAC RFC 2104: https://www.rfc-editor.org/rfc/rfc2104
- HTTP Authentication RFC 7235: https://www.rfc-editor.org/rfc/rfc7235

---

**Document Version:** 1.0  
**Last Updated:** 2025-01-17  
**Status:** Ready for Implementation
