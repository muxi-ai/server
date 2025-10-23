# Dummy MUXI Runtime SIF

This directory contains a minimal test SIF (Singularity Image Format) for testing the MUXI Server's SIF execution capabilities.

> **Note:** This is NOT the full MUXI Runtime. This is a simplified test image that packages only `dummy_app.py` for server-side testing. The full runtime SIF will be built in the `runtime` repository.

## Purpose

- Test SIF building process
- Validate server's SIF execution logic
- Prove end-to-end deployment flow
- Establish build/release pipeline

## Building

### Prerequisites

- **Docker:** For building the base image
- **Singularity or Apptainer:** For converting Docker → SIF (Linux only)

### Build Docker Image

```bash
# Build Docker image
./build.sh 0.1.0

# Or manually:
docker build -t muxi-runtime-dummy:0.1.0 .
```

### Convert to SIF (Linux only)

```bash
# Using Singularity
singularity build muxi-runtime-dummy-0.1.0.sif docker-daemon://muxi-runtime-dummy:0.1.0

# Using Apptainer (Singularity fork)
apptainer build muxi-runtime-dummy-0.1.0.sif docker-daemon://muxi-runtime-dummy:0.1.0
```

### macOS: Build SIF File

Since Singularity doesn't run natively on macOS, use one of these methods:

#### Option 1: Use Definition File on Linux

Transfer files to Linux machine:
```bash
# On Linux with Singularity/Apptainer installed:
cd test/dummy-sif
singularity build muxi-runtime-dummy-0.1.0.sif muxi-runtime-dummy.def
```

#### Option 2: Convert Docker Image on Linux

```bash
# 1. On macOS: Save Docker image
docker save muxi-runtime-dummy:0.1.0 | gzip > muxi-runtime-dummy-0.1.0.tar.gz

# 2. Transfer to Linux machine

# 3. On Linux: Load and convert
docker load < muxi-runtime-dummy-0.1.0.tar.gz
singularity build muxi-runtime-dummy-0.1.0.sif docker-daemon://muxi-runtime-dummy:0.1.0
```

#### Option 3: Use GitHub Actions (Recommended)

See `.github/workflows/build-sif.yml` for automated SIF builds on Linux runners.

#### Option 4: Docker for Development

For server development without actual SIF:
```bash
docker run --rm -p 8000:8000 muxi-runtime-dummy:0.1.0
```

## Testing

### Test Docker Image

```bash
# Run directly
docker run --rm -p 8000:8000 muxi-runtime-dummy:0.1.0

# Test health endpoint
curl http://localhost:8000/health

# Test chat endpoint
curl -X POST http://localhost:8000/chat \
  -H "Content-Type: application/json" \
  -d '{"message": "Hello!", "user_id": "test-user"}'
```

### Test SIF (Linux only)

```bash
# Run in foreground
singularity exec muxi-runtime-dummy-0.1.0.sif python dummy_app.py --port 8000

# Run in background
singularity instance start muxi-runtime-dummy-0.1.0.sif test-instance python dummy_app.py --port 8000
singularity instance stop test-instance

# Execute command in running instance
singularity exec instance://test-instance python -c "print('Hello from SIF!')"
```

### Test with Environment Variables

```bash
# Docker
docker run --rm -p 8001:8001 \
  -e PORT=8001 \
  -e HOST=127.0.0.1 \
  -e FORMATION_ID=test-formation \
  muxi-runtime-dummy:0.1.0

# Singularity
PORT=8001 HOST=127.0.0.1 FORMATION_ID=test-formation \
  singularity exec muxi-runtime-dummy-0.1.0.sif python dummy_app.py
```

## File Structure

```
dummy-sif/
├── Dockerfile              # Docker image definition
├── requirements.txt        # Python dependencies
├── dummy_app.py           # Simple FastAPI app (copied from ../dummy_app.py)
├── build.sh               # Build script (Docker → SIF)
└── README.md              # This file
```

## Expected Output

```
🚀 Starting MUXI Dummy Formation
   Formation ID: test-formation
   Listening: 127.0.0.1:8001
   Endpoints:
      GET  http://127.0.0.1:8001/health
      POST http://127.0.0.1:8001/chat
      GET  http://127.0.0.1:8001/
```

## Integration with MUXI Server

Once the SIF is built, the MUXI Server will execute it like this:

```bash
# Server spawns formation
singularity exec \
  --bind /tmp \
  --env PORT=8001 \
  --env HOST=127.0.0.1 \
  --env FORMATION_ID=my-formation \
  muxi-runtime-dummy-0.1.0.sif \
  python /app/dummy_app.py
```

## Next Steps

1. ✅ Build Docker image (works on macOS)
2. ⏳ Convert to SIF (requires Linux or VM)
3. ⏳ Update server spawn logic to use SIF
4. ⏳ Test end-to-end deployment

## Differences from Full Runtime

This dummy SIF is minimal and only contains:
- Python 3.10
- FastAPI + Uvicorn
- dummy_app.py

The full runtime SIF (built in `runtime` repo) will include:
- Complete MUXI Runtime SDK
- All Python dependencies
- Agent framework
- Memory systems
- MCP protocol support
- Observability tools

## Building Full Runtime SIF

See the runtime repository at `/Users/ran/Projects/muxi/code/runtime/sif/` for the complete build process.
