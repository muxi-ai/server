#!/bin/bash
# Build and optionally push the MUXI runtime-runner Docker image
#
# This image enables running SIF files on macOS/Windows via Docker.
# It contains Singularity and acts as a transparent wrapper.
#
# Usage:
#   ./build-runtime-runner.sh           # Build locally
#   ./build-runtime-runner.sh --push    # Build and push to Docker Hub

set -e

# Docker registry options:
# Option 1: Docker Hub with muxiai username
# IMAGE_NAME="muxiai/runtime-runner"

# Option 2: GitHub Container Registry (recommended, like faissx)
IMAGE_NAME="ghcr.io/muxi-ai/runtime-runner"

VERSION="1.0.0"
LATEST_TAG="${IMAGE_NAME}:latest"
VERSION_TAG="${IMAGE_NAME}:${VERSION}"

echo "🐳 Building MUXI Runtime Runner Docker image..."
echo ""
echo "   Image: ${IMAGE_NAME}"
echo "   Version: ${VERSION}"
echo ""

# Build the image
docker build \
    -f Dockerfile.runtime-runner \
    -t "${LATEST_TAG}" \
    -t "${VERSION_TAG}" \
    .

echo ""
echo "✅ Build complete!"
echo ""
echo "   Tags:"
echo "     - ${LATEST_TAG}"
echo "     - ${VERSION_TAG}"
echo ""

# Verify the image works
echo "🔍 Verifying Singularity in container..."
docker run --rm "${LATEST_TAG}" --version

echo ""
echo "✅ Verification passed!"

# Push if requested
if [[ "$1" == "--push" ]]; then
    echo ""
    echo "📤 Pushing to Docker Hub..."
    docker push "${LATEST_TAG}"
    docker push "${VERSION_TAG}"
    echo ""
    echo "✅ Push complete!"
    echo ""
    echo "   Pull with: docker pull ${LATEST_TAG}"
fi

echo ""
echo "📝 Test the image locally:"
echo ""
echo "   # Run a simple test"
echo "   docker run --rm ${LATEST_TAG} --version"
echo ""
echo "   # Run a SIF file (when you have one)"
echo "   docker run --rm -v \$(pwd)/output:/sif ${LATEST_TAG} exec /sif/muxi-runtime-dummy-0.1.0.sif python --version"
echo ""
