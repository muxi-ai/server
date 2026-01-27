#!/bin/bash
# Test SIF Integration - Manual Test Script

set -e

echo "======================================"
echo "🧪 Testing SIF Runtime Integration"
echo "======================================"
echo ""

# Configuration
FORMATION_DIR="$HOME/.muxi/server/formations/test-runtime-integration/current"
SIF_PATH="$HOME/.muxi/server/runtimes/muxi-runtime-0.2025.0-darwin-arm64.sif"
PORT=8001
HOST="127.0.0.1"  # For Singularity on Linux
DOCKER_HOST="0.0.0.0"  # For Docker testing (need 0.0.0.0 to be accessible from host)

echo "📋 Configuration:"
echo "   Formation: $FORMATION_DIR"
echo "   SIF: $SIF_PATH"
echo "   Port: $PORT"
echo "   Host: $HOST"
echo ""

# Check if formation exists
if [ ! -f "$FORMATION_DIR/formation.yaml" ]; then
    echo "❌ Formation not found: $FORMATION_DIR/formation.yaml"
    exit 1
fi

# Check if SIF exists
if [ ! -f "$SIF_PATH" ]; then
    echo "❌ SIF not found: $SIF_PATH"
    exit 1
fi

echo "✅ Formation found"
echo "✅ SIF found ($(ls -lh $SIF_PATH | awk '{print $5}'))"
echo ""

# Check if Singularity is available (on Linux)
if command -v singularity &> /dev/null; then
    echo "✅ Singularity found: $(singularity --version)"
    echo ""
    echo "🚀 Starting formation with Singularity..."
    echo ""

    singularity exec \
        --bind "$FORMATION_DIR:/formation" \
        "$SIF_PATH" \
        python -m muxi.utils.run_formation \
        /formation/formation.afs \
        --port $PORT \
        --host $HOST
else
    echo "⚠️  Singularity not available on macOS"
    echo "   Using Docker to wrap Singularity execution..."
    echo ""
    echo "🚀 Starting formation with Docker-wrapped Singularity..."
    echo ""

    # On macOS, we'll use Docker directly since Singularity requires Linux kernel
    # For real server deployment, this would use native Singularity on Linux
    echo "   Note: On Linux production, this would use native Singularity"
    echo "   For macOS testing, falling back to Docker..."
    echo ""

    docker run --rm \
        -v "$FORMATION_DIR:/formation:ro" \
        -e PORT=$PORT \
        -e HOST=$DOCKER_HOST \
        -p $PORT:$PORT \
        muxi-runtime:0.2025.0 \
        /formation/formation.afs \
        --port $PORT \
        --host $DOCKER_HOST
fi
