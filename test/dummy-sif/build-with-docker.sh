#!/bin/bash
# Build SIF file using Docker container
# This script builds the SIF inside a Docker container, then extracts it
# Works on macOS without needing Singularity/Apptainer installed locally

set -e

VERSION=${1:-"0.1.0"}
OUTPUT_DIR="$(pwd)/output"

echo "🔨 Building SIF using Docker container"
echo "   Version: $VERSION"
echo "   Output: $OUTPUT_DIR"
echo ""

# Create output directory
mkdir -p "$OUTPUT_DIR"

# Step 1: Build the builder image
echo "Step 1: Building SIF builder Docker image..."
docker build \
    -f Dockerfile.builder \
    -t muxi-sif-builder:latest \
    .
echo "✅ Builder image ready"
echo ""

# Step 2: Run the builder container to create SIF
echo "Step 2: Building SIF inside container..."
echo "   This may take a few minutes..."
docker run --rm \
    --privileged \
    -v "$OUTPUT_DIR:/output" \
    muxi-sif-builder:latest

echo ""

# Step 3: Rename the SIF with version
if [ -f "$OUTPUT_DIR/muxi-runtime-dummy.sif" ]; then
    mv "$OUTPUT_DIR/muxi-runtime-dummy.sif" "$OUTPUT_DIR/muxi-runtime-dummy-$VERSION.sif"
    echo "✅ SIF file created!"
    ls -lh "$OUTPUT_DIR/muxi-runtime-dummy-$VERSION.sif"
    echo ""
    
    # Show file info
    echo "File details:"
    file "$OUTPUT_DIR/muxi-runtime-dummy-$VERSION.sif"
    echo ""
    
    echo "✅ Build complete!"
    echo ""
    echo "SIF file location:"
    echo "  $OUTPUT_DIR/muxi-runtime-dummy-$VERSION.sif"
    echo ""
    echo "To test (requires Singularity/Apptainer on Linux):"
    echo "  singularity exec $OUTPUT_DIR/muxi-runtime-dummy-$VERSION.sif python /app/dummy_app.py"
    echo ""
    echo "Or transfer to Linux machine for testing"
else
    echo "❌ Error: SIF file not created"
    echo "Check the build output above for errors"
    exit 1
fi
