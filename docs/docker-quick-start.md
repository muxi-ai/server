# Docker Quick Start

**Run MUXI Server in Docker without installing anything locally!**

Perfect for users who:
- Don't want to install binaries on their machine
- Want to quickly test MUXI Server
- Prefer containerized environments
- Need easy cleanup

---

## Prerequisites

**Only requirement:** Docker Desktop installed

- **macOS/Windows:** [Download Docker Desktop](https://docker.com/products/docker-desktop)
- **Linux:** `apt install docker.io` or `yum install docker`

---

## Quick Start (One Command)

```bash
docker run -d \
  --name muxi-server \
  -p 7890:7890 \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -v ~/.muxi/server:/root/.muxi/server \
  ghcr.io/muxi-ai/muxi-server:latest

# Server is now running!
# Access at: http://localhost:7890
```

**Check it's working:**
```bash
curl http://localhost:7890/health
# {"status": "healthy"}
```

---

## Even Easier: Docker Compose

### 1. Download docker-compose.yml

```bash
curl -O https://raw.githubusercontent.com/muxi-ai/server/main/docker-compose.yml
```

### 2. Start server

```bash
docker-compose up -d
```

That's it! Server is running.

### 3. View logs

```bash
docker-compose logs -f
```

### 4. Stop server

```bash
docker-compose down
```

---

## What Gets Installed?

**Nothing!** Everything runs in containers:
- MUXI Server runs in a container
- Formations run in separate containers
- Data stored in Docker volume (or bind mount)

**Cleanup:**
```bash
docker-compose down -v  # Removes containers and volumes
```

---

## Deploy Your First Formation

Once the server is running:

```bash
# Download example formation
curl -O https://raw.githubusercontent.com/muxi-ai/server/main/test/formations/base-formation.tar.gz

# Deploy it
curl -X POST \
  -H "Content-Type: application/gzip" \
  --data-binary @base-formation.tar.gz \
  http://localhost:7890/rpc/formations/deploy

# Access formation
curl http://localhost:7890/api/foundation-test-base/health
```

---

## Configuration

### Environment Variables

Edit `docker-compose.yml` to customize:

```yaml
environment:
  MUXI_LOG_LEVEL: debug          # Logging: debug, info, warn, error
  MUXI_PORT: 7890                # Server port
  MUXI_RUNTIME_TYPE: docker      # Use Docker for formations
```

### Persistent Data

By default, data is stored in a Docker volume:

```bash
# View data
docker volume inspect muxi-server_muxi-data

# Backup data
docker run --rm -v muxi-server_muxi-data:/data -v $(pwd):/backup \
  alpine tar czf /backup/muxi-backup.tar.gz /data
```

**Want easier access?** Use a bind mount instead:

```yaml
# docker-compose.yml
volumes:
  - ./data:/root/.muxi/server  # Data in ./data directory
```

---

## Docker Socket Explained

### What is it?

```yaml
volumes:
  - /var/run/docker.sock:/var/run/docker.sock
```

This gives the MUXI Server container access to the **host's Docker daemon**.

### Why needed?

MUXI Server needs to spawn formation containers:

```
Your Machine
├── Docker Daemon
    ├── muxi-server (container 1)
    ├── formation-1 (container 2) ← spawned by muxi-server
    └── formation-2 (container 3) ← spawned by muxi-server
```

Formations run as **sibling containers**, not nested.

### Is it safe?

**⚠️ Security Note:**

This gives the container **full control** over Docker. Only use:
- ✅ Local development/testing
- ✅ Trusted environments
- ❌ NOT for production (use native install instead)

**For production, use native install:**
```bash
# Linux
curl -sSL https://get.muxi.ai | bash

# Much more secure!
```

---

## Common Tasks

### View Running Containers

```bash
# All MUXI-related containers
docker ps --filter "label=muxi"

# Or just formations
docker ps --filter "name=formation-"
```

### View Logs

```bash
# Server logs
docker-compose logs -f muxi-server

# Specific formation logs
docker logs formation-my-agent
```

### Stop/Start Server

```bash
# Stop
docker-compose stop

# Start
docker-compose start

# Restart
docker-compose restart
```

### Update Server

```bash
# Pull latest image
docker-compose pull

# Recreate container
docker-compose up -d
```

### Access Server Shell

```bash
docker exec -it muxi-server sh

# Inside container:
ls /root/.muxi/server/formations
muxi-server version
```

---

## Troubleshooting

### "Cannot connect to Docker daemon"

**Problem:** Docker isn't running

**Solution:**
```bash
# macOS/Windows: Start Docker Desktop

# Linux: Start Docker service
sudo systemctl start docker
```

### "Permission denied: /var/run/docker.sock"

**Problem:** User doesn't have Docker permissions

**Solution (Linux):**
```bash
# Add user to docker group
sudo usermod -aG docker $USER

# Log out and back in
```

### "Port 7890 already in use"

**Problem:** Something else is using port 7890

**Solution:**
```yaml
# Change port in docker-compose.yml
ports:
  - "8080:7890"  # Use port 8080 instead
```

### "Formation containers not starting"

**Problem:** Docker socket not mounted correctly

**Verify:**
```bash
# Check socket is mounted
docker exec muxi-server ls -la /var/run/docker.sock

# Should show: srw-rw---- root docker
```

### "Server exits immediately"

**Check logs:**
```bash
docker-compose logs muxi-server

# Common issues:
# - Port conflict
# - Invalid config
# - Missing Docker socket
```

---

## Comparison: Docker vs Native Install

| Feature | Docker Install | Native Install |
|---------|---------------|----------------|
| **Installation** | `docker-compose up` | `curl \| bash` + init |
| **Requirements** | Docker only | Go binary + Singularity/Docker |
| **Performance** | ⭐⭐⭐ Good | ⭐⭐⭐⭐⭐ Optimal |
| **Production** | ⚠️ Not recommended | ✅ Recommended |
| **Security** | ⚠️ Socket access | ✅ Isolated |
| **Updates** | `docker pull` | Package manager |
| **Cleanup** | `docker-compose down -v` | Uninstall binary |
| **Best for** | Quick testing, demos | Production, development |

---

## When to Use Docker

**Perfect for:**
- ✅ Quick testing/evaluation
- ✅ Demos and workshops
- ✅ CI/CD pipelines
- ✅ Temporary environments
- ✅ Users afraid to install software

**Not ideal for:**
- ❌ Production servers (use native)
- ❌ Maximum performance needs
- ❌ Security-sensitive environments

---

## Next Steps

**After testing with Docker, consider:**

1. **For production:** Install natively
   ```bash
   # Linux
   curl -sSL https://get.muxi.ai | bash
   
   # macOS
   brew install muxi-ai/tap/muxi-server
   ```

2. **Learn more:**
   - [API Reference](api-reference.md)
   - [Formation Development](formations.md)
   - [Deployment Guide](deployment.md)

---

## Summary

**Docker makes MUXI Server easy:**

```bash
# One command to run
docker-compose up -d

# One command to stop
docker-compose down

# One command to clean up
docker-compose down -v
```

**No installation, no configuration, just works!** ✨

But remember: **For production, use native install** for better security and performance.

---

**Questions?** Check the [troubleshooting guide](troubleshooting.md) or [open an issue](https://github.com/muxi-ai/server/issues).
