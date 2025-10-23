#!/bin/bash
# Build SIF on Linux with Singularity/Apptainer installed
# Run this script on a Linux machine

set -e

VERSION=${1:-"0.1.0"}

echo "🔨 Building SIF on Linux"
echo "   Version: $VERSION"
echo ""

# Check for Singularity or Apptainer
if command -v singularity &> /dev/null; then
    BUILDER="singularity"
    echo "✅ Using Singularity"
elif command -v apptainer &> /dev/null; then
    BUILDER="apptainer"
    echo "✅ Using Apptainer"
else
    echo "❌ Error: Neither Singularity nor Apptainer found"
    echo ""
    echo "Install Singularity:"
    echo "  https://sylabs.io/guides/3.0/user-guide/installation.html"
    echo ""
    echo "Or install Apptainer:"
    echo "  https://apptainer.org/docs/admin/main/installation.html"
    exit 1
fi
echo ""

# Check if definition file exists
if [ ! -f "muxi-runtime-dummy.def" ]; then
    echo "❌ Error: muxi-runtime-dummy.def not found"
    echo "   Run this script from test/dummy-sif/ directory"
    exit 1
fi

# Build SIF from definition file
echo "Building SIF from definition file..."
$BUILDER build \
    --force \
    muxi-runtime-dummy-$VERSION.sif \
    muxi-runtime-dummy.def

echo ""
echo "✅ SIF file created!"
ls -lh muxi-runtime-dummy-$VERSION.sif
echo ""

# Test the SIF
echo "Testing SIF..."
$BUILDER exec muxi-runtime-dummy-$VERSION.sif python --version
echo ""

echo "✅ Build complete!"
echo ""
echo "Test with:"
echo "  $BUILDER exec muxi-runtime-dummy-$VERSION.sif python /app/dummy_app.py &"
echo "  sleep 3"
echo "  curl http://localhost:8000/health"
echo ""
echo "Or run as instance:"
echo "  $BUILDER instance start muxi-runtime-dummy-$VERSION.sif test"
echo "  $BUILDER exec instance://test python /app/dummy_app.py"
