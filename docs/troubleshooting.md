# Troubleshooting

Common issues and solutions for MUXI Server.

---

## Server Issues

### Server Won't Start

**Symptoms:**
```
❌ Failed to start server: bind: address already in use
```

**Causes:**
1. Port 7890 is already in use
2. Another instance of muxi-server is running
3. Permission denied on port < 1024

**Solutions:**

**1. Check if port is in use:**

**Linux/macOS:**
```bash
lsof -i :7890
# Or
netstat -an | grep 7890
```

**Windows:**
```powershell
Get-NetTCPConnection -LocalPort 7890 -ErrorAction SilentlyContinue
# Or
netstat -ano | findstr :7890
```

**2. Kill existing process:**

**Linux/macOS:**
```bash
# Find process
ps aux | grep muxi-server

# Kill it
kill -9 <PID>
```

**Windows:**
```powershell
# Find process
Get-Process muxi-server

# Kill it
Stop-Process -Name "muxi-server" -Force

# Or by port
$port = Get-NetTCPConnection -LocalPort 7890 -ErrorAction SilentlyContinue
if ($port) {
    Stop-Process -Id $port.OwningProcess -Force
}
```

**3. Use different port:**
```yaml
# ~/.muxi/server/config.yaml
server:
  port: 8080
```

Or:
```bash
MUXI_SERVER_PORT=8080 muxi-server start
```

**4. Use sudo for privileged ports (< 1024):**
```bash
sudo muxi-server start
```

---

### Server Crashes on Startup

**Symptoms:**
```
❌ Server exited with code 1
```

**Check logs:**
```bash
# systemd
sudo journalctl -u muxi-server -n 100

# Manual
tail -f ~/.muxi/server/logs/server.log
```

**Common causes:**

**1. Invalid configuration:**
```bash
muxi-server config validate
```

**2. Missing directories:**
```bash
mkdir -p ~/.muxi/server/logs
mkdir -p ~/.muxi/server/rpc/formations
```

**3. Permission issues:**
```bash
# Check permissions
ls -la ~/.muxi/server

# Fix permissions
chmod -R 755 ~/.muxi/server
```

---

### Server Stops Unexpectedly

**Symptoms:**
Server runs for a while, then stops without explanation.

**Solutions:**

**1. Check disk space:**
```bash
df -h
```

**2. Check memory:**
```bash
free -h  # Linux
vm_stat  # macOS
```

**3. Check system logs:**
```bash
# Linux
sudo journalctl -xe

# macOS
tail -f /var/log/system.log
```

**4. Enable debug logging:**
```yaml
server:
  log_level: "debug"
```

---

## Configuration Issues

### Invalid Configuration

**Symptoms:**
```
❌ Error parsing config: yaml: line 5: mapping values are not allowed in this context
```

**Solutions:**

**1. Check YAML syntax:**
```bash
# Install yamllint
pip install yamllint

# Validate
yamllint ~/.muxi/server/config.yaml
```

**2. Common YAML mistakes:**

**Wrong:**
```yaml
server:
port: 3000  # ❌ Wrong indentation
```

**Right:**
```yaml
server:
  port: 3000  # ✅ Correct
```

**Wrong:**
```yaml
auth:
  key: MUXI_abc123  # ❌ Missing quotes
```

**Right:**
```yaml
auth:
  key: "MUXI_abc123"  # ✅ With quotes
```

---

### Config Not Loading

**Symptoms:**
Server uses default values instead of config file.

**Solutions:**

**1. Check file location:**
```bash
ls -la ~/.muxi/server/config.yaml
```

**2. Specify config explicitly:**
```bash
muxi-server start --config=/path/to/config.yaml
```

**3. Check file permissions:**
```bash
chmod 644 ~/.muxi/server/config.yaml
```

---

## Authentication Issues

### "Unauthorized" Error

**Symptoms:**
```
❌ 401 Unauthorized: Invalid credentials
```

**Solutions:**

**1. Check credentials match:**
```bash
# Server config
cat ~/.muxi/server/config.yaml | grep -A 3 "^auth:"

# CLI profile
cat ~/.muxi/profiles.yaml | grep -A 3 "^default:"
```

**2. Verify key format:**
- Key should start with `MUXI_`
- Secret should start with `sk_`
- Both case-sensitive

**3. Check auth is enabled:**
```yaml
auth:
  enabled: true
```

**4. Test with curl:**
```bash
# Compute signature manually
python3 <<EOF
import hmac, hashlib, base64, time
secret = "sk_your_secret"
timestamp = str(int(time.time()))
message = f"{timestamp};GET;/rpc/formations"
sig = base64.b64encode(hmac.new(secret.encode(), message.encode(), hashlib.sha256).digest()).decode()
print(f"Authorization: MUXI-HMAC key=MUXI_your_key, timestamp={timestamp}, signature={sig}")
EOF
```

---

### "Request Expired" Error

**Symptoms:**
```
❌ 401 Request expired (timestamp too old)
```

**Cause:** Clock skew between client and server

**Solutions:**

**1. Sync system time:**
```bash
# macOS
sudo sntp -sS time.apple.com

# Linux (NTP)
sudo ntpdate pool.ntp.org

# Linux (systemd-timesyncd)
sudo timedatectl set-ntp true
```

**2. Check time difference:**
```bash
# Local time
date +%s

# Server time (via SSH)
ssh user@server date +%s

# Difference should be < 5 minutes (300 seconds)
```

**3. Increase tolerance (temporary):**
```yaml
auth:
  timestamp_tolerance: 600  # 10 minutes
```

---

### "Invalid Signature" Error

**Symptoms:**
```
❌ 401 Invalid signature
```

**Solutions:**

**1. Check CLI version:**
```bash
muxi --version
```

**2. Regenerate credentials:**
```bash
muxi-server init --rotate
```

**3. Debug signature generation:**
```bash
# Enable debug mode
muxi formation list --debug
```

---

## Windows-Specific Issues

### Docker Not Found

**Symptoms:**
```
❌ Error: Failed to start formation: docker: command not found
```

**Solution:**

1. **Install Docker Desktop:**
   - Download from https://www.docker.com/products/docker-desktop
   - Restart Windows after installation

2. **Verify Docker:**
```powershell
docker --version
docker ps
```

3. **Add Docker to PATH (if needed):**
```powershell
$env:Path += ";C:\Program Files\Docker\Docker\resources\bin"
```

---

### Windows Firewall Blocking

**Symptoms:**
- Server starts but can't connect from browser
- Formation deployment fails silently

**Solution:**

**Allow MUXI Server in Windows Firewall:**

```powershell
# PowerShell (Admin)
New-NetFirewallRule `
  -DisplayName "MUXI Server" `
  -Direction Inbound `
  -Action Allow `
  -Protocol TCP `
  -LocalPort 7890
```

**Or use GUI:**
1. Windows Defender Firewall → Advanced Settings
2. Inbound Rules → New Rule
3. Port → TCP → 7890
4. Allow the connection

---

### Permission Denied Errors

**Symptoms:**
```
❌ Access denied: cannot create directory
```

**Solution:**

**Run as Administrator (for system install):**
- Right-click PowerShell → "Run as Administrator"

**Or use user install (recommended):**
```powershell
# User-level install (no admin needed)
$env:MUXI_CONFIG_DIR = "$env:APPDATA\muxi\server"
muxi-server serve
```

---

### Antivirus Blocking

**Symptoms:**
- Binary won't run
- Formation processes killed immediately

**Solution:**

1. **Add exception in Windows Defender:**

```powershell
# PowerShell (Admin)
Add-MpPreference -ExclusionPath "$env:LOCALAPPDATA\muxi"
Add-MpPreference -ExclusionPath "$env:APPDATA\muxi"
```

2. **Or use Defender GUI:**
- Windows Security → Virus & threat protection
- Manage settings → Add exclusions
- Add folder: `%LOCALAPPDATA%\muxi`

---

### PowerShell Execution Policy

**Symptoms:**
```
install.ps1 cannot be loaded because running scripts is disabled
```

**Solution:**

```powershell
# Allow scripts for current user (recommended)
Set-ExecutionPolicy -ExecutionPolicy RemoteSigned -Scope CurrentUser

# Or bypass for single command
powershell -ExecutionPolicy Bypass -Command "irm https://install.muxi.org/windows.ps1 | iex"
```

---

### WSL 2 vs Native

**Question:** Should I use WSL 2 or native Windows binary?

**Answer:**
- **Native Windows:** Better for Windows-only development, simpler setup
- **WSL 2:** Better for Linux-like experience, if you're already using WSL

Both work great - choose based on your workflow!

---

## Formation Issues

### Formation Won't Deploy

**Symptoms:**
```
❌ Failed to deploy formation: command not found
```

**Solutions:**

**1. Check command exists:**
```bash
which python
which python3
```

**2. Use full path:**
```json
{
  "id": "my-api",
  "command": "/usr/bin/python3 app.py"
}
```

**3. Check working directory:**
```json
{
  "id": "my-api",
  "command": "python app.py",
  "working_dir": "/home/user/my-formation"
}
```

**4. Check file exists:**
```bash
ls -la /home/user/my-formation/app.py
```

---

### Formation Crashes Immediately

**Symptoms:**
Formation status shows `crashed` right after deployment.

**Solutions:**

**1. Check formation logs:**
```bash
muxi formation logs my-api
```

**2. Run command manually:**
```bash
cd /path/to/formation
python app.py
```

**3. Check dependencies:**
```bash
pip list
pip install -r requirements.txt
```

**4. Check port availability:**
```bash
lsof -i :8001
```

---

### Formation Keeps Restarting

**Symptoms:**
```
Formation restart_count: 5
Status: restarting
```

**Solutions:**

**1. Check logs for errors:**
```bash
muxi formation logs my-api --lines=500
```

**2. Check health endpoint:**
```bash
curl http://localhost:8001/health
```

**3. Disable auto-restart temporarily:**
```yaml
formations:
  auto_restart: false
```

**4. Increase restart delay:**
```yaml
formations:
  restart_delay: 10  # Wait 10 seconds
```

**5. Check resource usage:**
```bash
# CPU/Memory
ps aux | grep my-api

# Or
top -p <PID>
```

---

### Formation Not Accessible

**Symptoms:**
```
❌ curl: (7) Failed to connect to localhost port 7890
```

**Solutions:**

**1. Check formation status:**
```bash
muxi formation status my-api
```

**2. Check formation is running:**
```bash
muxi formation list
```

**3. Test direct connection:**
```bash
# Get port from status
curl http://localhost:8001/health
```

**4. Test proxy:**
```bash
curl http://localhost:7890/my-api/health
```

**5. Check firewall:**
```bash
# Linux
sudo iptables -L

# macOS
/usr/libexec/ApplicationFirewall/socketfilterfw --getglobalstate
```

---

### "Formation Not Found" Error

**Symptoms:**
```
❌ 404 Formation not found
```

**Solutions:**

**1. List formations:**
```bash
muxi formation list
```

**2. Check formation ID:**
```bash
# IDs are case-sensitive!
muxi formation get my-api    # ✅
muxi formation get My-API    # ❌
```

**3. Check registry file:**
```bash
cat ~/.muxi/server/registry.json | jq
```

---

## Port Issues

### "Port Already in Use" Error

**Symptoms:**
```
❌ Failed to bind port 8001: address already in use
```

**Solutions:**

**1. Find process using port:**
```bash
lsof -i :8001
```

**2. Kill process:**
```bash
kill -9 <PID>
```

**3. Change port range:**
```yaml
formations:
  port_range_start: 9000
  port_range_end: 10000
```

---

### "No Ports Available" Error

**Symptoms:**
```
❌ Failed to allocate port: no ports available
```

**Cause:** All ports in the pool are in use

**Solutions:**

**1. Check current formations:**
```bash
muxi formation list
```

**2. Delete unused formations:**
```bash
muxi formation delete old-api
```

**3. Increase port range:**
```yaml
formations:
  port_range_start: 8000
  port_range_end: 10000  # 2000 ports instead of 1000
```

---

## Performance Issues

### Server Running Slow

**Symptoms:**
- High response times
- Formations take long to deploy
- Health checks timing out

**Solutions:**

**1. Check CPU usage:**
```bash
top
htop
```

**2. Check memory:**
```bash
free -h
```

**3. Check disk I/O:**
```bash
iostat -x 1
```

**4. Reduce health check frequency:**
```yaml
formations:
  health_check_interval: 60  # Check every minute
```

**5. Increase health check timeout:**
```yaml
formations:
  health_check_timeout: 30  # 30 seconds
```

---

### High Memory Usage

**Symptoms:**
Server or formations consuming too much memory.

**Solutions:**

**1. Check formation memory:**
```bash
ps aux --sort=-%mem | head -20
```

**2. Reduce worker count:**
```json
{
  "command": "uvicorn app:app --workers 2"  # Instead of 8
}
```

**3. Enable memory limits (future):**
```yaml
formations:
  memory_limit: "1GB"
```

---

## Log Issues

### Logs Not Appearing

**Symptoms:**
`muxi formation logs my-api` returns empty or "not found"

**Solutions:**

**1. Check log directory:**
```bash
ls -la ~/.muxi/server/logs/
```

**2. Check permissions:**
```bash
chmod 755 ~/.muxi/server/logs
```

**3. Check formation is writing logs:**
```bash
tail -f ~/.muxi/server/logs/my-api.log
```

**4. Check log configuration:**
```yaml
formations:
  logs_dir: "~/.muxi/server/logs"
```

---

### Logs Too Large

**Symptoms:**
Log files growing too large, filling disk.

**Solutions:**

**1. Enable log rotation:**
```yaml
formations:
  log_rotation:
    max_size: "100MB"
    max_files: 5
```

**2. Manually clean logs:**
```bash
rm ~/.muxi/server/logs/*.log.*
```

**3. Set up logrotate (Linux):**
```bash
sudo tee /etc/logrotate.d/muxi <<EOF
/home/user/.muxi/server/logs/*.log {
    daily
    rotate 7
    compress
    missingok
    notifempty
}
EOF
```

---

## Network Issues

### Cannot Connect to Server

**Symptoms:**
```
❌ curl: (7) Failed to connect
```

**Solutions:**

**1. Check server is running:**
```bash
# Process
ps aux | grep muxi-server

# Port
lsof -i :7890
```

**2. Check bind address:**
```yaml
server:
  host: "0.0.0.0"  # Not 127.0.0.1 for external access
```

**3. Check firewall:**
```bash
# Linux
sudo ufw status
sudo ufw allow 3000

# macOS
sudo /usr/libexec/ApplicationFirewall/socketfilterfw --getglobalstate
```

**4. Test locally first:**
```bash
curl http://localhost:7890/health
```

**5. Test remote:**
```bash
curl http://<server-ip>:7890/health
```

---

### Proxy Not Working

**Symptoms:**
```
❌ 502 Bad Gateway
```

**Solutions:**

**1. Check formation is running:**
```bash
muxi formation status my-api
```

**2. Test formation directly:**
```bash
curl http://localhost:<port>/health
```

**3. Check server logs:**
```bash
sudo journalctl -u muxi-server -f
```

---

## Debugging

### Enable Debug Logging

```yaml
server:
  log_level: "debug"
```

Or:

```bash
MUXI_LOG_LEVEL=debug muxi-server start
```

### View Server Logs

**systemd:**
```bash
sudo journalctl -u muxi-server -f
```

**Manual:**
```bash
tail -f ~/.muxi/server/logs/server.log
```

### View Formation Logs

```bash
tail -f ~/.muxi/server/logs/my-api.log
```

### Debug HTTP Requests

**Use verbose curl:**
```bash
curl -v http://localhost:7890/rpc/formations
```

**Use httpie:**
```bash
http -v http://localhost:7890/rpc/formations
```

### Check System Resources

```bash
# CPU & Memory
top

# Disk space
df -h

# Open files
lsof | wc -l

# Network connections
netstat -an | grep ESTABLISHED | wc -l
```

---

## Getting Help

### Collect Debug Info

Before asking for help, collect this information:

```bash
# 1. Server version
muxi-server version

# 2. Server status
systemctl status muxi-server

# 3. Server logs (last 100 lines)
sudo journalctl -u muxi-server -n 100

# 4. Formation list
muxi formation list

# 5. Configuration
cat ~/.muxi/server/config.yaml

# 6. System info
uname -a
free -h
df -h
```

### Community Support

- **GitHub Issues:** [github.com/muxi-ai/server/issues](https://github.com/muxi-ai/server/issues)
- **Discussions:** [github.com/muxi-ai/server/discussions](https://github.com/muxi-ai/server/discussions)
- **Discord:** [discord.gg/muxi](https://discord.gg/muxi)

### Commercial Support

- **Email:** support@muxi.org
- **Response time:** Within 24 hours (business days)
- **Enterprise support:** Available for production deployments

---

## FAQ

### Can I run multiple instances of muxi-server?

Yes, but they must use different ports and data directories:

```bash
# Instance 1
MUXI_SERVER_PORT=3000 muxi-server start

# Instance 2
MUXI_SERVER_PORT=3001 \
  MUXI_DATA_DIR=~/.muxi/server2 \
  muxi-server start
```

### Can I run muxi-server in Docker?

Yes! See [Installation Guide](./installation.md#docker).

### How many formations can I run?

Limited by:
- Available ports (default: 1000 ports = 1000 formations)
- System resources (CPU, memory)
- Typical: 10-50 formations per server

### Can I use custom domains?

Yes, with a reverse proxy:

```nginx
server {
    listen 80;
    server_name myapi.com;
    location / {
        proxy_pass http://localhost:7890/my-api;
    }
}
```

### Can I scale horizontally?

Not yet. Phase 2+ will support:
- Multiple muxi-server instances
- Formation distribution across servers
- Load balancing

### Is there a GUI?

Not yet. Planned for future release:
- Web dashboard
- Formation monitoring
- Real-time logs
- Resource graphs

---

## Known Issues

### Issue: Health Checks Sometimes Fail

**Status:** Investigating  
**Workaround:** Increase `health_check_timeout`:

```yaml
formations:
  health_check_timeout: 30
```

### Issue: Log Rotation Not Working

**Status:** Fixed in v1.1.0  
**Workaround:** Manually rotate logs or use logrotate

### Issue: High CPU Usage on macOS

**Status:** Known issue with fsevents  
**Workaround:** Disable file watching (future config option)

---

## Next Steps

- [Getting Started](./getting-started.md)
- [Configuration Guide](./configuration.md)
- [API Reference](./api-reference.md)

---

**Still stuck?** [Open an issue on GitHub](https://github.com/muxi-ai/server/issues/new)
