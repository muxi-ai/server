#!/bin/bash
# Build actual SIF file using Singularity in Docker container

set -e

VERSION=${1:-"0.1.0"}

echo "🔨 Building REAL SIF file using Singularity"
echo "   Version: $VERSION"
echo ""

# Step 1: Build Docker image first (if not exists)
if ! docker image inspect muxi-runtime-dummy:$VERSION >/dev/null 2>&1; then
    echo "Step 1: Building Docker image first..."
    ./build.sh $VERSION
    echo ""
else
    echo "Step 1: Docker image already exists ✓"
    echo ""
fi

# Step 2: Save Docker image to tar
echo "Step 2: Saving Docker image to tar..."
docker save muxi-runtime-dummy:$VERSION | gzip > muxi-runtime-dummy-$VERSION.tar.gz
echo "✅ Saved: muxi-runtime-dummy-$VERSION.tar.gz"
echo ""

# Step 3: Run Singularity in Docker to convert
echo "Step 3: Converting to SIF using Singularity in Docker..."
echo "   This uses the official Singularity Docker image"
echo ""

# Create output directory
mkdir -p output

# Run Singularity in Docker
docker run --rm --privileged \
    -v /var/run/docker.sock:/var/run/docker.sock \
    -v "$(pwd)":/work \
    -w /work \
    quay.io/singularity/singularity:v3.11.4 \
    build muxi-runtime-dummy-$VERSION.sif docker-daemon://muxi-runtime-dummy:$VERSION

if [ -f "muxi-runtime-dummy-$VERSION.sif" ]; then
    echo ""
    echo "✅ SIF file created!"
    ls -lh muxi-runtime-dummy-$VERSION.sif
    echo ""
    echo "Test with:"
    echo "  singularity exec muxi-runtime-dummy-$VERSION.sif python /app/dummy_app.py"
else
    echo ""
    echo "❌ SIF creation failed"
    echo ""
    echo "Alternative: Build on Linux machine"
    echo "  1. Transfer: muxi-runtime-dummy-$VERSION.tar.gz"
    echo "  2. Load: docker load < muxi-runtime-dummy-$VERSION.tar.gz"
    echo "  3. Build: singularity build runtime.sif docker-daemon://muxi-runtime-dummy:$VERSION"
fi
