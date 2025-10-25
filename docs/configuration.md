# Configuration

Complete configuration reference for MUXI Server.

---

## Configuration File

**Location:** `~/.muxi/server/config.yaml`

The configuration file is created automatically when you run `muxi-server init`. You can also create and edit it manually.

### Default Configuration

```yaml
server:
  port: 7890
  host: "0.0.0.0"
  log_level: "info"

auth:
  enabled: true
  key: "MUXI_abc123def456"
  secret: "sk_xyz789abc012"
  timestamp_tolerance: 300

formations:
  runtime_type: "native"
  port_range_start: 8000
  port_range_end: 9000
  logs_dir: "~/.muxi/server/logs"
  auto_restart: true
  max_restart_count: 10
  restart_delay: 1
  health_check_interval: 30
  health_check_timeout: 10

persistence:
  registry_file: "~/.muxi/server/registry.json"
  auto_save: true
  save_interval: 5
```

---

## Configuration Sections

### Server Settings

Controls the HTTP server behavior.

```yaml
server:
  port: 7890              # HTTP port for server
  host: "0.0.0.0"         # Bind address (0.0.0.0 = all interfaces)
  log_level: "info"       # Log level: debug, info, warn, error
```

**Options:**

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `port` | int | `7890` | HTTP port for management API and proxy |
| `host` | string | `"0.0.0.0"` | Bind address (`0.0.0.0` for all, `127.0.0.1` for localhost) |
| `log_level` | string | `"info"` | Logging level: `debug`, `info`, `warn`, `error` |

**Examples:**

```yaml
# Bind to localhost only (more secure)
server:
  host: "127.0.0.1"
  port: 7890

# Enable debug logging
server:
  log_level: "debug"

# Custom port
server:
  port: 8080
```

---

### Authentication Settings

Controls server authentication (HMAC-based).

```yaml
auth:
  enabled: true                      # Enable/disable authentication
  key: "MUXI_abc123def456"          # Public key identifier
  secret: "sk_xyz789abc012"         # Secret key for HMAC signing
  timestamp_tolerance: 300          # Time window in seconds (5 minutes)
```

**Options:**

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `enabled` | bool | `true` | Enable authentication |
| `key` | string | (generated) | Public key identifier (prefix: `MUXI_`) |
| `secret` | string | (generated) | Secret key for HMAC (prefix: `sk_`) |
| `timestamp_tolerance` | int | `300` | Time window for timestamp validation (seconds) |

**Examples:**

```yaml
# Disable auth for local development (⚠️ DANGEROUS!)
auth:
  enabled: false

# Tighter security (1 minute window)
auth:
  enabled: true
  timestamp_tolerance: 60

# Custom credentials
auth:
  enabled: true
  key: "MUXI_my_custom_key_123"
  secret: "sk_my_custom_secret_456"
```

**⚠️ Security Warning:**
- Never commit `config.yaml` with credentials to version control
- Only disable auth on localhost for development
- Use `timestamp_tolerance` >= 60 seconds to allow for clock skew

---

### Formation Settings

Controls how formations are spawned and managed.

```yaml
formations:
  runtime_type: "native"              # Runtime type: native, singularity, docker
  port_range_start: 8000              # Start of port pool
  port_range_end: 9000                # End of port pool
  logs_dir: "~/.muxi/server/logs"     # Directory for formation logs
  
  # Auto-restart behavior
  auto_restart: true                  # Enable auto-restart on crash
  max_restart_count: 10               # Max restart attempts
  restart_delay: 1                    # Delay between restarts (seconds)
  
  # Health checks
  health_check_interval: 30           # Check every N seconds
  health_check_timeout: 10            # Timeout for /health endpoint
  
  # Zero-downtime deployment (✨ NEW!)
  deployment:
    health_check:
      enabled: true                   # Enable zero-downtime deployments
      endpoint: "/health"             # Health endpoint path
      timeout: 30                     # Total timeout in seconds
      interval: 1                     # Poll interval in seconds
      max_retries: 30                 # Max health check attempts
    force_kill_timeout: 5             # Seconds before force-killing old version
    staging_health_delay: 2           # Delay before first health check
```

**Options:**

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `runtime_type` | string | `"native"` | Runtime: `native` (Python), `singularity`, `docker` |
| `port_range_start` | int | `8000` | First port for formation allocation |
| `port_range_end` | int | `9000` | Last port for formation allocation |
| `logs_dir` | string | `~/.muxi/server/logs` | Directory for formation logs |
| `auto_restart` | bool | `true` | Automatically restart crashed formations |
| `max_restart_count` | int | `10` | Stop restarting after N attempts |
| `restart_delay` | int | `1` | Wait N seconds between restart attempts |
| `health_check_interval` | int | `30` | Check formation health every N seconds |
| `health_check_timeout` | int | `10` | Health check request timeout (seconds) |
| `deployment.health_check.enabled` | bool | `true` | Enable zero-downtime deployments ✨ NEW |
| `deployment.health_check.endpoint` | string | `"/health"` | Health endpoint path for deployments ✨ NEW |
| `deployment.health_check.timeout` | int | `30` | Deployment health check timeout (seconds) ✨ NEW |
| `deployment.health_check.interval` | int | `1` | Poll interval for deployment health checks ✨ NEW |
| `deployment.health_check.max_retries` | int | `30` | Max health check attempts during deployment ✨ NEW |
| `deployment.force_kill_timeout` | int | `5` | Seconds before force-killing old version ✨ NEW |
| `deployment.staging_health_delay` | int | `2` | Delay before starting deployment health checks ✨ NEW |

**Examples:**

```yaml
# More ports for many formations
formations:
  port_range_start: 8000
  port_range_end: 10000

# Disable auto-restart
formations:
  auto_restart: false

# Aggressive health checks
formations:
  health_check_interval: 10
  health_check_timeout: 5

# Use Singularity runtime (Phase 3+)
formations:
  runtime_type: "singularity"

# Custom log directory
formations:
  logs_dir: "/var/log/muxi/formations"

# Custom health endpoint for deployments (✨ NEW)
formations:
  deployment:
    health_check:
      endpoint: "/api/health"  # Use custom endpoint

# Longer timeout for slow-starting formations (✨ NEW)
formations:
  deployment:
    health_check:
      timeout: 120              # 2 minutes
      interval: 2               # Check every 2 seconds
    staging_health_delay: 10    # Wait 10 seconds before checking

# Disable zero-downtime deployments (✨ NEW)
formations:
  deployment:
    health_check:
      enabled: false  # Use traditional deployment (not recommended)
```

**Port Pool:**
- Server allocates ports sequentially from the pool
- Each formation gets one unique port
- Maximum formations = `port_range_end - port_range_start`
- Default: 1000 ports (8000-9000)

**Runtime Types:**
- `native` - Spawn Python processes directly (Phase 1)
- `singularity` - Use Singularity/Apptainer SIF images (Phase 3)
- `docker` - Use Docker containers (Future)

---

### Persistence Settings

Controls how formation state is saved.

```yaml
persistence:
  registry_file: "~/.muxi/server/registry.json"  # Registry file path
  auto_save: true                                # Auto-save on changes
  save_interval: 5                               # Debounce interval (seconds)
```

**Options:**

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `registry_file` | string | `~/.muxi/server/registry.json` | Path to registry file |
| `auto_save` | bool | `true` | Automatically save on changes |
| `save_interval` | int | `5` | Debounce save writes (seconds) |

**Examples:**

```yaml
# Custom registry location
persistence:
  registry_file: "/var/lib/muxi/registry.json"

# Disable auto-save (manual save only)
persistence:
  auto_save: false

# Save immediately (no debounce)
persistence:
  save_interval: 0
```

**Registry File Format:**

The registry file is JSON with formation metadata:

```json
{
  "formations": {
    "my-api": {
      "id": "my-api",
      "command": "python app.py",
      "port": 8001,
      "pid": 12345,
      "status": "running",
      "created_at": "2025-01-17T10:30:00Z",
      "updated_at": "2025-01-17T10:30:00Z",
      "restart_count": 0
    }
  }
}
```

---

## Environment Variables

You can override config values with environment variables:

| Variable | Config Path | Example |
|----------|-------------|---------|
| `MUXI_SERVER_PORT` | `server.port` | `MUXI_SERVER_PORT=8080` |
| `MUXI_SERVER_HOST` | `server.host` | `MUXI_SERVER_HOST=127.0.0.1` |
| `MUXI_LOG_LEVEL` | `server.log_level` | `MUXI_LOG_LEVEL=debug` |
| `MUXI_AUTH_ENABLED` | `auth.enabled` | `MUXI_AUTH_ENABLED=false` |
| `MUXI_PORT_START` | `formations.port_range_start` | `MUXI_PORT_START=8000` |
| `MUXI_PORT_END` | `formations.port_range_end` | `MUXI_PORT_END=9000` |

**Example:**

```bash
# Override port and enable debug logging
MUXI_SERVER_PORT=8080 MUXI_LOG_LEVEL=debug muxi-server start
```

**Precedence:**
1. Environment variables (highest)
2. Config file
3. Default values (lowest)

---

## Configuration Examples

### Local Development

```yaml
server:
  port: 7890
  host: "127.0.0.1"  # Localhost only
  log_level: "debug"

auth:
  enabled: false  # ⚠️ Development only!

formations:
  port_range_start: 8000
  port_range_end: 8010  # Small pool for testing
  auto_restart: true
  health_check_interval: 10  # Faster checks for dev
```

### Production Server

```yaml
server:
  port: 7890
  host: "0.0.0.0"  # Accept external connections
  log_level: "info"

auth:
  enabled: true
  key: "MUXI_prod_secure_key_xyz"
  secret: "sk_prod_secure_secret_abc"
  timestamp_tolerance: 300

formations:
  runtime_type: "singularity"  # Use SIF containers
  port_range_start: 8000
  port_range_end: 9000
  logs_dir: "/var/log/muxi/formations"
  
  auto_restart: true
  max_restart_count: 10
  restart_delay: 5  # Wait longer between restarts
  
  health_check_interval: 30
  health_check_timeout: 10

persistence:
  registry_file: "/var/lib/muxi/registry.json"
  auto_save: true
  save_interval: 10  # Less frequent saves
```

### High-Capacity Server

```yaml
server:
  port: 7890
  host: "0.0.0.0"

auth:
  enabled: true

formations:
  port_range_start: 8000
  port_range_end: 18000  # 10,000 ports!
  logs_dir: "/mnt/logs/muxi"
  
  auto_restart: true
  max_restart_count: 5  # Fail faster
  restart_delay: 10  # Wait longer between attempts
  
  health_check_interval: 60  # Less frequent (many formations)
  health_check_timeout: 15

persistence:
  registry_file: "/mnt/data/muxi/registry.json"
  auto_save: true
  save_interval: 30  # Larger debounce (performance)
```

---

## Validation

The server validates configuration on startup:

### Port Validation

```
❌ Error: port_range_start must be less than port_range_end
❌ Error: port must be between 1 and 65535
❌ Error: port_range requires at least 10 ports
```

### Auth Validation

```
❌ Error: auth.key must start with "MUXI_"
❌ Error: auth.secret must start with "sk_"
❌ Error: auth.key must be at least 20 characters
❌ Error: auth.secret must be at least 32 characters
```

### Path Validation

```
❌ Error: logs_dir does not exist or is not writable
❌ Error: registry_file directory does not exist
```

---

## Configuration Management

### Initialize Configuration

```bash
# Generate default config with credentials
muxi-server init

# Regenerate credentials (keep other settings)
muxi-server init --rotate
```

### Validate Configuration

```bash
# Check config file for errors
muxi-server config validate

# Show current configuration
muxi-server config show
```

### Edit Configuration

```bash
# Open in default editor
muxi-server config edit

# Or edit directly
vi ~/.muxi/server/config.yaml
```

---

## Troubleshooting

### Server Won't Start

**Check config syntax:**
```bash
muxi-server config validate
```

**Common issues:**
- YAML syntax error (indentation, colons)
- Invalid port number
- Auth credentials malformed
- Log directory doesn't exist

### Port Already in Use

```
❌ Error: bind: address already in use (port 7890)
```

**Solutions:**
1. Change port in config: `server.port: 8080`
2. Stop other process using port 7890
3. Use environment variable: `MUXI_SERVER_PORT=8080 muxi-server start`

### Authentication Not Working

**Check credentials match:**
```bash
# Server config
cat ~/.muxi/server/config.yaml | grep -A 3 "^auth:"

# CLI profile
cat ~/.muxi/profiles.yaml | grep -A 3 "^default:"
```

### Formations Not Starting

**Check port pool:**
```yaml
formations:
  port_range_start: 8000
  port_range_end: 8010  # Only 10 ports available!
```

**Check logs directory:**
```bash
ls -la ~/.muxi/server/logs
# Should be writable
```

---

## Next Steps

- [Set up authentication](./authentication.md)
- [Deploy your first formation](./formations.md)
- [API Reference](./api-reference.md)

---

**Need help?** See the [Troubleshooting Guide](./troubleshooting.md)
