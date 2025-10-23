#!/bin/bash
# Build script for dummy MUXI Runtime SIF
# This creates a minimal test image for server-side testing

set -e

VERSION=${1:-"0.1.0"}
PLATFORM=${2:-"linux/amd64"}

echo "🔨 Building Dummy MUXI Runtime SIF"
echo "   Version: $VERSION"
echo "   Platform: $PLATFORM"
echo ""

# Step 1: Build Docker image
echo "Step 1: Building Docker image..."
docker build \
    --platform "$PLATFORM" \
    -t muxi-runtime-dummy:$VERSION \
    -t muxi-runtime-dummy:latest \
    .

echo "✅ Docker image built: muxi-runtime-dummy:$VERSION"
echo ""

# Step 2: Convert to SIF (requires Singularity/Apptainer)
echo "Step 2: Converting to SIF..."
if command -v singularity &> /dev/null; then
    echo "   Using Singularity..."
    singularity build \
        "muxi-runtime-dummy-$VERSION-$(uname -m).sif" \
        "docker-daemon://muxi-runtime-dummy:$VERSION"
    echo "✅ SIF created: muxi-runtime-dummy-$VERSION-$(uname -m).sif"
elif command -v apptainer &> /dev/null; then
    echo "   Using Apptainer..."
    apptainer build \
        "muxi-runtime-dummy-$VERSION-$(uname -m).sif" \
        "docker-daemon://muxi-runtime-dummy:$VERSION"
    echo "✅ SIF created: muxi-runtime-dummy-$VERSION-$(uname -m).sif"
else
    echo "⚠️  Singularity/Apptainer not found - skipping SIF conversion"
    echo "   Docker image is ready: muxi-runtime-dummy:$VERSION"
    echo ""
    echo "   To convert to SIF on Linux:"
    echo "   1. Save image: docker save muxi-runtime-dummy:$VERSION | gzip > muxi-runtime-dummy-$VERSION.tar.gz"
    echo "   2. Transfer to Linux machine"
    echo "   3. Load image: docker load < muxi-runtime-dummy-$VERSION.tar.gz"
    echo "   4. Convert: singularity build muxi-runtime-dummy-$VERSION.sif docker-daemon://muxi-runtime-dummy:$VERSION"
fi

echo ""
echo "✅ Build complete!"
echo ""
echo "Test with Docker:"
echo "  docker run --rm -p 8000:8000 muxi-runtime-dummy:$VERSION"
echo ""
echo "Test with Singularity (if SIF created):"
echo "  singularity exec muxi-runtime-dummy-$VERSION-*.sif python dummy_app.py"
