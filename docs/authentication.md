# Authentication

Secure your MUXI Server with HMAC-based authentication.

---

## Overview

MUXI Server uses **HMAC authentication** (similar to AWS) with key/secret pairs. This provides strong security without requiring complex certificate management.

### Two-Layer Architecture

```
┌────────────────────────────────────────┐
│ Layer 1: Server Management API        │
│ Requires: Server credentials          │
│ Protects: Deploy, list, delete, etc.  │
└────────────────────────────────────────┘
           │
           ↓
┌────────────────────────────────────────┐
│ Layer 2: Formation API (Proxied)      │
│ Requires: Formation credentials        │
│ Protects: /chat, /workflow, etc.      │
└────────────────────────────────────────┘
```

**Important:** Server credentials ≠ Formation credentials!
- **Server credentials**: Deploy/manage formations (admin)
- **Formation credentials**: Use formation APIs (end users)

---

## Quick Start

### 1. Initialize Server

Generate server credentials:

```bash
muxi-server init
```

**Output:**
```
🔐 Generating authentication credentials...

   Key:    MUXI_e8f3a9b2c4d1f5e6
   Secret: sk_9f2e8d7c6b5a4f3e2d1c0b9a8f7e6d5c

📝 Saved to: ~/.muxi/server/config.yaml

⚠️  Keep your secret secure! Never share or commit it.
   Add to CLI: muxi config add-profile
```

### 2. Configure CLI

Add server credentials to CLI profile:

```bash
muxi config add-profile default \
  --key="MUXI_e8f3a9b2c4d1f5e6" \
  --secret="sk_9f2e8d7c6b5a4f3e2d1c0b9a8f7e6d5c" \
  --server="http://localhost:3000"
```

### 3. Verify Authentication

Test with a simple request:

```bash
muxi formation list
```

**Success:**
```
✓ Connected to server
Formations: 0
```

**Failure:**
```
❌ Unauthorized: Invalid credentials
```

---

## Server Configuration

### Location

Credentials are stored in: `~/.muxi/server/config.yaml`

### Format

```yaml
auth:
  enabled: true
  key: "MUXI_abc123def456"
  secret: "sk_xyz789abc012"
  timestamp_tolerance: 300
```

### Options

| Option | Description | Default |
|--------|-------------|---------|
| `enabled` | Enable authentication | `true` |
| `key` | Public key identifier | (generated) |
| `secret` | Secret key for HMAC | (generated) |
| `timestamp_tolerance` | Time window (seconds) | `300` (5 min) |

### Generate New Credentials

```bash
# Generate fresh credentials
muxi-server init

# Rotate credentials (keeps other config)
muxi-server init --rotate
```

### Disable Authentication

**⚠️ Development only!**

```yaml
auth:
  enabled: false
```

Or:

```bash
MUXI_AUTH_ENABLED=false muxi-server start
```

---

## CLI Configuration

### Profile File

Profiles are stored in: `~/.muxi/profiles.yaml`

### Profile Format

```yaml
default:
  key: "MUXI_abc123def456"
  secret: "sk_xyz789abc012"
  servers:
    - "http://localhost:3000"

production:
  key: "MUXI_prod_key_123"
  secret: "sk_prod_secret_456"
  servers:
    - "https://api.myserver.com"

staging:
  key: "MUXI_staging_789"
  secret: "sk_staging_012"
  servers:
    - "https://staging.myserver.com"
```

### Profile Commands

**Add profile:**
```bash
muxi config add-profile production \
  --key="MUXI_prod_123" \
  --secret="sk_prod_456" \
  --server="https://api.myserver.com"
```

**List profiles:**
```bash
muxi config list-profiles
```

**Output:**
```
Profiles:
  • default (http://localhost:3000)
  • production (https://api.myserver.com)
  • staging (https://staging.myserver.com)
```

**Show profile:**
```bash
muxi config show-profile production
```

**Output:**
```yaml
key: "MUXI_prod_123"
secret: "sk_prod_***" (hidden)
servers:
  - https://api.myserver.com
```

**Delete profile:**
```bash
muxi config delete-profile staging
```

### Using Profiles

**Default profile:**
```bash
muxi formation deploy app.yaml
```

**Specific profile:**
```bash
muxi formation deploy app.yaml --profile=production
```

**Override server:**
```bash
muxi formation deploy app.yaml --server=https://custom.com
```

**Deploy to all servers in profile:**
```bash
muxi formation deploy app.yaml --profile=production --all-servers
```

---

## How Authentication Works

### HMAC Signature Process

1. **CLI prepares request:**
   - Method: `POST`
   - Path: `/formations/deploy`
   - Timestamp: Current Unix time

2. **CLI creates signing string:**
   ```
   {timestamp};{method};{path}
   ```
   Example: `1705484123;POST;/formations/deploy`

3. **CLI computes HMAC-SHA256:**
   ```
   HMAC-SHA256(secret, signing_string)
   ```

4. **CLI sends request with header:**
   ```
   Authorization: MUXI-HMAC key=MUXI_abc123, timestamp=1705484123, signature=YWJj...
   ```

5. **Server validates:**
   - Checks key exists
   - Recomputes signature
   - Compares (constant-time)
   - Checks timestamp (within 5 minutes)

### Why HMAC?

✅ **Secret never transmitted** - Only signature is sent  
✅ **Replay attack prevention** - Timestamps expire  
✅ **Tamper detection** - Any change invalidates signature  
✅ **No shared secrets in transit** - Like AWS SigV4

---

## Protected Endpoints

### Management API (Auth Required)

These endpoints require server credentials:

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/formations/deploy` | POST | Deploy formation |
| `/formations` | GET | List formations |
| `/formations/{id}` | GET | Get formation details |
| `/formations/{id}/restart` | POST | Restart formation |
| `/formations/{id}/stop` | POST | Stop formation |
| `/formations/{id}` | DELETE | Delete formation |
| `/formations/{id}/logs` | GET | Get formation logs |

### Proxy API (No Server Auth)

These are transparently proxied to formations:

| Endpoint | Description |
|----------|-------------|
| `/{formation_id}/*` | All formation routes |

**Example:**

```bash
# Requires formation credentials (not server credentials)
curl https://api.myserver.com/my-formation/chat \
  -H "Authorization: Bearer fc_formation_client_key" \
  -d '{"message": "Hello"}'
```

The formation validates its own credentials. Server just proxies the request.

---

## Security Best Practices

### Credential Management

✅ **DO:**
- Store credentials in config files only
- Use different credentials for each environment (dev, staging, prod)
- Rotate credentials regularly
- Keep secrets out of version control
- Use `.gitignore` for config files

❌ **DON'T:**
- Hardcode credentials in scripts
- Share credentials via email/Slack
- Commit credentials to git
- Use same credentials across environments
- Log secrets

### Credential Storage

**Good:**
```yaml
# ~/.muxi/server/config.yaml (not in git)
auth:
  key: "MUXI_prod_123"
  secret: "sk_prod_456"
```

**Bad:**
```bash
# deploy.sh (in git)
export MUXI_KEY="MUXI_prod_123"
export MUXI_SECRET="sk_prod_456"  # ❌ NEVER DO THIS!
```

### .gitignore

Always add to `.gitignore`:

```gitignore
# MUXI credentials
.muxi/
~/.muxi/server/config.yaml
~/.muxi/profiles.yaml
config.yaml
profiles.yaml
```

---

## Key Rotation

### When to Rotate

- **Regular schedule:** Every 90 days
- **Security incident:** Immediately
- **Employee departure:** Within 24 hours
- **Suspected compromise:** Immediately

### Rotation Process

**1. Generate new credentials:**

```bash
muxi-server init --rotate
```

**Output:**
```
🔐 Rotating credentials...

Old Key:    MUXI_old_key_123
New Key:    MUXI_new_key_789
New Secret: sk_new_secret_012

📝 Updated: ~/.muxi/server/config.yaml
⚠️  Update CLI profiles with new credentials!
```

**2. Update CLI profiles:**

```bash
muxi config add-profile production \
  --key="MUXI_new_key_789" \
  --secret="sk_new_secret_012" \
  --server="https://api.myserver.com"
```

**3. Restart server:**

```bash
# systemd
sudo systemctl restart muxi-server

# Or manual
muxi-server stop
muxi-server start
```

**4. Test:**

```bash
muxi formation list --profile=production
```

---

## Troubleshooting

### "Unauthorized" Error

**Symptoms:**
```
❌ Unauthorized: Invalid credentials
```

**Solutions:**

1. **Check credentials match:**
   ```bash
   # Server
   cat ~/.muxi/server/config.yaml | grep -A 3 "^auth:"
   
   # CLI
   cat ~/.muxi/profiles.yaml | grep -A 3 "^default:"
   ```

2. **Verify credentials are correct:**
   - Key should start with `MUXI_`
   - Secret should start with `sk_`
   - Both should match exactly

3. **Check auth is enabled:**
   ```yaml
   auth:
     enabled: true  # Should be true
   ```

### "Request Expired" Error

**Symptoms:**
```
❌ Request expired (timestamp too old)
```

**Cause:** Clock skew between client and server

**Solutions:**

1. **Sync system time:**
   ```bash
   # macOS
   sudo sntp -sS time.apple.com
   
   # Linux
   sudo ntpdate pool.ntp.org
   ```

2. **Increase tolerance (not recommended):**
   ```yaml
   auth:
     timestamp_tolerance: 600  # 10 minutes
   ```

### "Missing Authorization Header"

**Symptoms:**
```
❌ Missing authorization header
```

**Solutions:**

1. **Check CLI profile:**
   ```bash
   muxi config show-profile default
   ```

2. **Verify profile is being used:**
   ```bash
   muxi formation list --profile=default -v
   ```

3. **Check server logs:**
   ```bash
   sudo journalctl -u muxi-server -n 50
   ```

---

## Advanced Configuration

### Custom Timestamp Tolerance

Adjust time window for signature validation:

```yaml
auth:
  timestamp_tolerance: 60  # 1 minute (stricter)
```

**Use cases:**
- Tighter security: 60 seconds
- Clock skew issues: 600 seconds (10 minutes)
- Default: 300 seconds (5 minutes)

### Multiple Keys (Future)

Support for multiple valid keys (rolling rotation):

```yaml
auth:
  enabled: true
  keys:
    - key: "MUXI_key_1"
      secret: "sk_secret_1"
    - key: "MUXI_key_2"
      secret: "sk_secret_2"
```

This allows zero-downtime rotation.

---

## Manual Authentication (API)

If you're not using the CLI, you can make authenticated requests manually.

### 1. Compute Signature

**Python example:**

```python
import hmac
import hashlib
import base64
import time

def compute_signature(secret, method, path):
    timestamp = str(int(time.time()))
    message = f"{timestamp};{method};{path}"
    
    signature = hmac.new(
        secret.encode('utf-8'),
        message.encode('utf-8'),
        hashlib.sha256
    ).digest()
    
    signature_b64 = base64.b64encode(signature).decode('utf-8')
    
    return timestamp, signature_b64

# Example
timestamp, signature = compute_signature(
    "sk_xyz789abc012",
    "POST",
    "/formations/deploy"
)
```

### 2. Send Request

```bash
curl -X POST http://localhost:3000/formations/deploy \
  -H "Authorization: MUXI-HMAC key=MUXI_abc123, timestamp=1705484123, signature=YWJj..." \
  -H "Content-Type: application/json" \
  -d '{"id": "my-api", "command": "python app.py"}'
```

---

## Next Steps

- [Deploy your first formation](./formations.md)
- [Configure your server](./configuration.md)
- [API Reference](./api-reference.md)

---

**Need help?** See the [Troubleshooting Guide](./troubleshooting.md)
