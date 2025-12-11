# How Formations Run

**A simple guide to understanding MUXI Server's runtime execution**

---

## Quick Overview

When you deploy a formation to MUXI Server, it runs in an isolated container using **Singularity** (a container technology like Docker, but optimized for production workloads).

```bash
$ muxi formation deploy my-formation.tar.gz

✓ Formation deployed: my-formation
✓ Running on: http://localhost:7890/api/my-formation
✓ Health check: http://localhost:8001/health
```

Behind the scenes, MUXI Server:
1. Extracts your formation files
2. Loads the appropriate runtime (based on formation config)
3. Spawns an isolated container for your formation
4. Starts health monitoring

> **Note:** Formation config files support three extensions (in priority order): `.afs` (Agent Formation Schema - preferred), `.yaml`, and `.yml`.

---

## Requirements

### Linux (Production Servers)

**Required:**
```bash
# Ubuntu/Debian
sudo apt install singularity-container

# RHEL/CentOS/Fedora
sudo yum install singularity
```

**That's it!** Formations run natively with optimal performance.

### macOS (Development)

**Required:**
```bash
# Install Docker Desktop
brew install --cask docker

# Or download from: https://docker.com/products/docker-desktop
```

**That's it!** MUXI Server handles the rest automatically.

### Windows (Development)

**Required:**
1. Install WSL 2: `wsl --install`
2. Install Docker Desktop from [docker.com](https://docker.com/products/docker-desktop)

**That's it!** MUXI Server handles the rest automatically.

---

## How It Works

### Each Formation Gets Its Own Container

When you deploy a formation, MUXI Server creates a dedicated container just for it:

```
Your Server
  │
  ├─ Formation: my-chatbot
  │   Port: 8001
  │   Status: Running
  │   
  ├─ Formation: data-analyzer  
  │   Port: 8002
  │   Status: Running
  │
  └─ Formation: workflow-engine
      Port: 8003
      Status: Running
```

**Each formation:**
- Runs independently (one can crash without affecting others)
- Has its own port (automatically assigned)
- Has isolated resources (CPU, memory, disk)
- Starts/stops independently

### Platform-Specific Execution

MUXI Server automatically uses the best method for your platform:

**On Linux:**
- Uses **Singularity** natively
- Maximum performance (~50ms startup)
- Zero overhead
- Production-optimized

**On macOS/Windows:**
- Uses **Docker Desktop** to provide Linux
- Good performance (~200-500ms startup)
- Automatic (pulled once, cached forever)
- Development-optimized

You don't need to do anything different - MUXI Server detects your platform and handles it automatically.

---

## First-Time Setup

### Linux

```bash
# 1. Install Singularity
sudo apt install singularity-container

# 2. Initialize MUXI Server
muxi-server init

# 3. Start the server
muxi-server serve

# Ready!
```

### macOS

```bash
# 1. Install Docker Desktop
brew install --cask docker

# 2. Start Docker Desktop
open -a Docker

# 3. Initialize MUXI Server
muxi-server init

# 4. Start the server
muxi-server serve

# Ready! (First deployment pulls runtime image - takes 1-2 min once)
```

### Windows

```powershell
# 1. Install WSL 2
wsl --install

# 2. Install Docker Desktop
# Download from docker.com and install

# 3. Start Docker Desktop from Start Menu

# 4. Initialize MUXI Server
muxi-server init

# 5. Start the server
muxi-server serve

# Ready! (First deployment pulls runtime image - takes 1-2 min once)
```

---

## Deploying a Formation

### Simple Deployment

```bash
$ muxi formation deploy my-formation.tar.gz

Uploading formation...
✓ Formation extracted
✓ Configuration validated
✓ Runtime resolved: 0.1.0
✓ Port allocated: 8001
✓ Container started

Formation "my-formation" is now running!

  API URL:    http://localhost:7890/api/my-formation
  Health URL: http://localhost:8001/health
  Status:     starting → healthy (usually 5-10 seconds)
```

### What Happens

1. **Upload** - Your formation bundle is uploaded to the server
2. **Extract** - Files are extracted to `~/.muxi/server/formations/my-formation/`
3. **Validate** - `formation.yaml` is checked for correctness
4. **Runtime** - Server finds the appropriate runtime (e.g., `0.1.0`)
5. **Spawn** - A new container is created and started
6. **Monitor** - Server begins health checks every 10 seconds

### What You Get

- **Dedicated port** - Your formation gets a unique port (8001, 8002, etc.)
- **Automatic restart** - If your formation crashes, it restarts automatically
- **Health monitoring** - Server checks `/health` endpoint regularly
- **API access** - Available at `http://localhost:7890/api/your-formation-id`

---

## Managing Formations

### Check Status

```bash
$ muxi formation list

ID              Status    Port   Uptime   Restarts
my-chatbot      healthy   8001   2h 15m   0
data-analyzer   healthy   8002   45m      1
workflow-engine starting  8003   5s       0
```

### Stop a Formation

```bash
$ muxi formation stop my-chatbot

Stopping formation "my-chatbot"...
✓ Container stopped
✓ Resources cleaned up
✓ Port 8001 released
```

### Restart a Formation

```bash
$ muxi formation restart my-chatbot

Restarting formation "my-chatbot"...
✓ Container stopped
✓ Container started
✓ Formation is running on port 8001
```

### View Logs

```bash
$ muxi formation logs my-chatbot

[2025-10-23 10:30:15] INFO: Formation starting...
[2025-10-23 10:30:16] INFO: Loading configuration...
[2025-10-23 10:30:17] INFO: Server listening on 0.0.0.0:8001
[2025-10-23 10:30:18] INFO: Health check endpoint ready
```

### Delete a Formation

```bash
$ muxi formation delete my-chatbot

⚠️  This will permanently delete "my-chatbot" and all its data.
Continue? [y/N] y

Deleting formation "my-chatbot"...
✓ Container stopped
✓ Files removed
✓ Configuration cleaned up
```

---

## Understanding Performance

### Linux (Production)

```
Formation startup:  ~50ms
Memory usage:       Formation only (no container overhead)
CPU usage:          Formation only (no container overhead)
Network latency:    Native (no virtualization)

Perfect for production servers!
```

### macOS/Windows (Development)

```
Formation startup:  ~200-500ms
Memory usage:       Formation + ~100MB (Docker layer)
CPU usage:          Formation + minimal overhead
Network latency:    +1-2ms (negligible)

Great for local development and testing!
```

**Note:** First deployment on macOS/Windows downloads the runtime image (~120MB). This takes 1-2 minutes once, then it's cached and instant.

---

## Troubleshooting

### "Singularity not found" (Linux)

**What happened:** Singularity isn't installed

**Fix:**
```bash
sudo apt install singularity-container
# Then restart server
```

### "Docker not found" (macOS/Windows)

**What happened:** Docker Desktop isn't installed

**Fix:**
```bash
# macOS
brew install --cask docker

# Windows: Download from docker.com
```

Then start Docker Desktop and restart MUXI Server.

### "Cannot connect to Docker daemon"

**What happened:** Docker Desktop isn't running

**Fix:**
- macOS: Open Docker Desktop from Applications
- Windows: Start Docker Desktop from Start Menu
- Wait for Docker icon to show "running" (green)

### Formation stuck in "starting"

**What happened:** Formation isn't starting properly

**Debug:**
```bash
# Check logs
muxi formation logs my-formation

# Common issues:
# - Port conflicts (formation can't bind to port)
# - Missing dependencies (check formation.yaml)
# - Syntax errors in formation code
```

### Slow first deployment (macOS/Windows)

**What happened:** Downloading runtime image for first time

**This is normal!** The runtime image (~120MB) downloads once and is cached. Subsequent deployments are instant.

```bash
Pulling runtime image (first time only)...
[===================>] 120MB/120MB
✓ Image cached - future deployments will be instant
```

### "Platform does not match" warning

**What happened:** Running on Apple Silicon Mac (ARM)

**Impact:** None! This warning is cosmetic. Docker handles the architecture difference automatically.

---

## Advanced Usage

### Multiple Formations

Deploy as many formations as you want:

```bash
muxi formation deploy chatbot.tar.gz
muxi formation deploy analyzer.tar.gz
muxi formation deploy workflow.tar.gz

# Each gets:
# - Own container (isolated)
# - Own port (8001, 8002, 8003, ...)
# - Own resources (CPU, memory)
# - Independent lifecycle
```

### Custom Runtime Versions

Specify the runtime version in `formation.yaml`:

```yaml
# formation.yaml
id: my-formation
runtime: "0.1.0"  # Specific version
# or
runtime: "0.1"    # Latest 0.1.x version
# or
runtime: "latest" # Latest available
```

MUXI Server automatically finds and uses the correct runtime.

### Resource Limits (Coming Soon)

Future versions will support resource limits:

```yaml
# formation.yaml
resources:
  cpu: "2"        # 2 CPU cores
  memory: "4GB"   # 4GB RAM
  disk: "10GB"    # 10GB storage
```

---

## Behind the Scenes

### What's in a Formation Container?

```
Container
  ├── MUXI Runtime Environment
  │   ├── Python 3.10+
  │   ├── FastAPI
  │   ├── MUXI SDK
  │   └── Common ML libraries
  │
  ├── Your Formation Files
  │   ├── formation.yaml
  │   ├── agents/
  │   ├── tools/
  │   └── workflows/
  │
  └── Environment Variables
      ├── PORT=8001
      ├── FORMATION_ID=my-formation
      └── MUXI_SERVER_URL=http://...
```

### Communication Flow

```
Your Request
  ↓
http://localhost:7890/api/my-formation/chat
  ↓
MUXI Server (receives request)
  ↓
Proxies to → Formation Container (port 8001)
  ↓
Formation handles request
  ↓
Response flows back through server
  ↓
Your Application
```

### Automatic Features

**Health Monitoring:**
- Server checks `/health` endpoint every 10 seconds
- Marks formation as "unhealthy" if checks fail
- Automatically restarts unhealthy formations

**Auto-Restart:**
- If a formation crashes, server restarts it automatically
- Maximum 10 restart attempts
- Exponential backoff between attempts

**Resource Cleanup:**
- When you stop a formation, container is removed automatically
- Temporary files cleaned up
- Port released for reuse

---

## Best Practices

### Development

- **Start server in foreground:** `muxi-server serve` (see logs in real-time)
- **Check logs often:** `muxi formation logs <id>` (debug issues quickly)
- **Use health endpoint:** Implement `/health` in your formation (server monitors it)

### Production

- **Run as service:** Use systemd (Linux) or launchd (macOS)
- **Monitor resource usage:** `muxi server status` (check CPU, memory, disk)
- **Set up log rotation:** Prevent log files from growing too large
- **Backup formations:** Keep your formation bundles in version control

### Security

- **Bind to localhost:** Formations bind to `127.0.0.1` by default (not accessible from outside)
- **Use MUXI proxy:** Access formations through server (`/api/formation-id/*`)
- **Implement authentication:** In your formation code (MUXI Server doesn't authenticate formation requests)

---

## FAQ

### Do I need to know Docker/Singularity?

**No!** MUXI Server handles all container management automatically. Just deploy your formation and it works.

### Can I use my own containers?

Currently, formations must use MUXI runtimes (SIF files). Custom containers are a future enhancement.

### How many formations can I run?

Limited only by your system resources (CPU, memory, ports). Each formation is lightweight.

### What if I don't have Singularity/Docker?

Run `muxi-server init` - it will guide you through installation or offer to install automatically (Linux only).

### Can formations communicate with each other?

Yes! Formations can call each other through the MUXI Server API:
```python
# Inside formation-a
import requests
response = requests.get("http://muxi-server:7890/api/formation-b/chat")
```

### What happens when I restart the server?

All formations stop and must be manually restarted. Use `muxi-server` as a service to auto-start on system boot.

---

## Summary

**MUXI Server makes it simple to run AI formations in production:**

✅ **Deploy once** - Works on Linux, macOS, and Windows  
✅ **Isolated** - Each formation runs independently  
✅ **Automatic** - Health monitoring, auto-restart, resource cleanup  
✅ **Fast** - Native performance on Linux, good performance on dev machines  
✅ **Simple** - Just deploy and it works!

**Next steps:**
- [Create your first formation](formations.md)
- [Deploy to MUXI Server](deployment.md)
- [API Reference](api-reference.md)

---

**Questions?** Check the [troubleshooting guide](troubleshooting.md) or [open an issue](https://github.com/muxi-ai/server/issues).
