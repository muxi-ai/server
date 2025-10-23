# How to Run a SIF File

## 🎯 Quick Answer

There are **4 main ways** to run a Singularity SIF file:

```bash
# 1. Run the default command (defined in %runscript)
singularity run myapp.sif

# 2. Execute a specific command
singularity exec myapp.sif python app.py

# 3. Get an interactive shell
singularity shell myapp.sif

# 4. Run as a background instance (service)
singularity instance start myapp.sif my-instance
```

---

## 📚 Detailed Explanation

### 1. `singularity run` - Run Default Command

Executes the **%runscript** section from the definition file.

```bash
# Run with default settings
singularity run muxi-runtime-dummy-0.1.0.sif

# This executes what's in %runscript:
# #!/bin/sh
# cd /app
# exec python dummy_app.py "$@"
```

**With arguments:**
```bash
singularity run muxi-runtime-dummy-0.1.0.sif --port 8001
# Passes --port 8001 to dummy_app.py
```

---

### 2. `singularity exec` - Execute Specific Command ⭐

**Most common for server deployments!**

```bash
# Execute specific command
singularity exec muxi-runtime-dummy-0.1.0.sif python /app/dummy_app.py --port 8000

# Check Python version
singularity exec muxi-runtime-dummy-0.1.0.sif python --version

# List files
singularity exec muxi-runtime-dummy-0.1.0.sif ls -la /app/
```

**With environment variables:**
```bash
singularity exec \
  --env PORT=8001 \
  --env HOST=127.0.0.1 \
  --env FORMATION_ID=test \
  muxi-runtime-dummy-0.1.0.sif \
  python /app/dummy_app.py
```

**With bind mounts:**
```bash
# Mount host directory into container
singularity exec \
  --bind /tmp:/container-tmp \
  --bind /host/data:/app/data \
  muxi-runtime-dummy-0.1.0.sif \
  python /app/dummy_app.py
```

---

### 3. `singularity shell` - Interactive Shell

Get a shell inside the container for debugging:

```bash
# Interactive shell
singularity shell muxi-runtime-dummy-0.1.0.sif

# Now you're inside the container:
Singularity> pwd
/home/user

Singularity> ls /app/
dummy_app.py  requirements.txt

Singularity> python /app/dummy_app.py &
# [1] 12345

Singularity> curl http://localhost:8000/health
{"status":"ok",...}

Singularity> exit
```

---

### 4. `singularity instance` - Background Service ⭐

**Best for long-running services!**

```bash
# Start instance
singularity instance start \
  --env PORT=8001 \
  muxi-runtime-dummy-0.1.0.sif \
  my-formation

# Instance now running in background

# List running instances
singularity instance list
INSTANCE NAME    PID      IMAGE
my-formation     12345    /path/to/muxi-runtime-dummy-0.1.0.sif

# Execute commands in the instance
singularity exec instance://my-formation python /app/dummy_app.py &

# Stop instance
singularity instance stop my-formation
```

---

## 🔧 How MUXI Server Will Run SIF

Based on our implementation, the server will use **`singularity exec`**:

### Server Spawn Logic

```go
// pkg/process/spawn.go

func Spawn(config SpawnConfig) (*Process, error) {
    if config.RuntimeType == "singularity" {
        sifPath := config.RuntimeSIFPath
        
        // Build singularity exec command
        args := []string{
            "exec",
            "--bind", "/tmp",  // Mount /tmp for temporary files
        }
        
        // Add environment variables
        for key, value := range config.Env {
            args = append(args, "--env", fmt.Sprintf("%s=%s", key, value))
        }
        
        // Add SIF file
        args = append(args, sifPath)
        
        // Add command to run (e.g., python /app/dummy_app.py)
        args = append(args, config.Command)
        args = append(args, config.Args...)
        
        cmd = exec.Command("singularity", args...)
    }
    
    // ... rest of spawn logic
}
```

### Resulting Command

```bash
singularity exec \
  --bind /tmp \
  --env PORT=8001 \
  --env HOST=127.0.0.1 \
  --env FORMATION_ID=my-formation \
  /path/to/muxi-runtime-dummy-0.1.0.sif \
  python /app/dummy_app.py
```

---

## 🧪 Testing the SIF File

### Test 1: Check it works
```bash
# On Linux machine with Singularity installed:
singularity exec muxi-runtime-dummy-0.1.0.sif python /app/dummy_app.py &

# Wait a moment for startup
sleep 3

# Test health endpoint
curl http://localhost:8000/health
# Expected: {"status":"ok","service":"dummy-formation",...}

# Test chat endpoint
curl -X POST http://localhost:8000/chat \
  -H "Content-Type: application/json" \
  -d '{"message":"Hello","user_id":"test"}'
# Expected: {"response":"Echo: Hello","user_id":"test",...}

# Kill the process
pkill -f dummy_app.py
```

### Test 2: With custom port
```bash
singularity exec \
  --env PORT=8001 \
  muxi-runtime-dummy-0.1.0.sif \
  python /app/dummy_app.py &

curl http://localhost:8001/health
```

### Test 3: Interactive debugging
```bash
singularity shell muxi-runtime-dummy-0.1.0.sif

# Inside container:
python /app/dummy_app.py --port 8002 &
curl http://localhost:8002/health
exit
```

---

## 🚀 Common Singularity Options

### Binding/Mounting
```bash
# Mount single directory
--bind /host/path:/container/path

# Mount multiple directories
--bind /data:/data,/logs:/logs

# Mount as read-only
--bind /data:/data:ro

# Mount entire /tmp
--bind /tmp
```

### Environment Variables
```bash
# Single variable
--env VAR=value

# Multiple variables
--env VAR1=value1 --env VAR2=value2

# Pass through host env vars
--env-file env.txt
```

### Networking
```bash
# Use host network (default)
# Container uses host's network stack

# Isolated network (requires root/fakeroot)
--network none
```

### Working Directory
```bash
# Set working directory
--pwd /app

# Example:
singularity exec --pwd /app myapp.sif python app.py
```

### User/Permissions
```bash
# Run as specific user (requires fakeroot)
--fakeroot

# Keep environment
--cleanenv  # Clean environment
--containall  # Isolated environment
```

---

## 📖 Complete Command Reference

### Basic Execution
```bash
# Run default runscript
singularity run myapp.sif

# Execute specific command
singularity exec myapp.sif /bin/bash

# Interactive shell
singularity shell myapp.sif
```

### With Options
```bash
# Full example with all common options
singularity exec \
  --bind /tmp:/tmp \
  --bind /data:/app/data:ro \
  --env PORT=8000 \
  --env HOST=127.0.0.1 \
  --env DEBUG=false \
  --pwd /app \
  muxi-runtime-dummy-0.1.0.sif \
  python dummy_app.py --verbose
```

### Instance Management
```bash
# Start named instance
singularity instance start myapp.sif instance-name

# List instances
singularity instance list

# Execute in instance
singularity exec instance://instance-name command

# Stop instance
singularity instance stop instance-name

# Stop all instances
singularity instance stop --all
```

---

## 🔍 Inspecting SIF Files

```bash
# Show SIF metadata
singularity inspect muxi-runtime-dummy-0.1.0.sif

# Show definition file used to build
singularity inspect --deffile muxi-runtime-dummy-0.1.0.sif

# Show environment variables
singularity inspect --environment muxi-runtime-dummy-0.1.0.sif

# Show runscript
singularity inspect --runscript muxi-runtime-dummy-0.1.0.sif

# Show help text
singularity inspect --helpfile muxi-runtime-dummy-0.1.0.sif

# Show labels
singularity inspect --labels muxi-runtime-dummy-0.1.0.sif
```

---

## ⚠️ Important Notes

### 1. **Singularity/Apptainer Required**
```bash
# Check if installed
which singularity
# or
which apptainer

# Check version
singularity --version
# Singularity version 3.11.4

# If not installed:
# See: https://sylabs.io/docs/
```

### 2. **Permissions**
- SIF files run as **your user** (not root)
- Home directory is mounted by default
- Can't modify files inside SIF (read-only)

### 3. **Networking**
- Uses host network by default
- Can bind to any port (no special permissions needed)
- Localhost works as expected

### 4. **File System**
- SIF is read-only
- Can mount writable directories with `--bind`
- Temp files go to `/tmp` (usually mounted)

---

## 🎯 For MUXI Server

The server will primarily use:

```bash
singularity exec \
  --bind /tmp \
  --env PORT=8001 \
  --env HOST=127.0.0.1 \
  --env FORMATION_ID=formation-id \
  /path/to/runtime.sif \
  python /app/formation-code.py
```

**Why `exec` not `run`?**
- More explicit control
- Can pass custom commands
- Easier to debug
- Better for process management

**Why not `instance`?**
- Server manages process lifecycle
- Easier to kill/restart
- Simpler PID tracking
- Standard process management tools work

---

## 📚 Additional Resources

- **Singularity Docs:** https://sylabs.io/docs/
- **Quick Start:** https://sylabs.io/guides/latest/user-guide/quick_start.html
- **CLI Reference:** https://sylabs.io/guides/latest/user-guide/cli.html
- **Apptainer:** https://apptainer.org/ (Singularity fork)

---

## ✅ Summary

**To run our dummy SIF:**
```bash
# Simple:
singularity exec muxi-runtime-dummy-0.1.0.sif python /app/dummy_app.py

# Production (what MUXI Server will do):
singularity exec \
  --bind /tmp \
  --env PORT=8001 \
  --env HOST=127.0.0.1 \
  muxi-runtime-dummy-0.1.0.sif \
  python /app/dummy_app.py
```

**Key Points:**
1. Use `singularity exec` for executing commands
2. Use `--env` for environment variables
3. Use `--bind` for mounting directories
4. SIF file is read-only (immutable)
5. Runs as your user (not root)
6. Works like Docker but simpler!
