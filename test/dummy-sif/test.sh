#!/bin/bash
# Test script for dummy MUXI Runtime SIF

set -e

VERSION=${1:-"0.1.0"}
IMAGE="muxi-runtime-dummy:$VERSION"

echo "🧪 Testing Dummy MUXI Runtime"
echo "   Image: $IMAGE"
echo ""

# Check if image exists
if ! docker image inspect $IMAGE >/dev/null 2>&1; then
    echo "❌ Error: Image $IMAGE not found"
    echo "   Run: ./build.sh $VERSION"
    exit 1
fi

echo "✅ Image found"
echo ""

# Start container
echo "Starting container..."
CONTAINER_ID=$(docker run --rm -d -p 8000:8000 $IMAGE)
echo "   Container ID: ${CONTAINER_ID:0:12}"
echo ""

# Wait for startup
echo "Waiting for startup..."
sleep 3
echo ""

# Test health endpoint
echo "Testing /health endpoint..."
HEALTH_RESPONSE=$(curl -s http://localhost:8000/health)
echo "   Response: $HEALTH_RESPONSE"

if echo "$HEALTH_RESPONSE" | grep -q '"status":"ok"'; then
    echo "   ✅ Health check passed"
else
    echo "   ❌ Health check failed"
    docker stop $CONTAINER_ID >/dev/null
    exit 1
fi
echo ""

# Test chat endpoint
echo "Testing /chat endpoint..."
CHAT_RESPONSE=$(curl -s -X POST http://localhost:8000/chat \
    -H "Content-Type: application/json" \
    -d '{"message":"Test message","user_id":"test-user"}')
echo "   Response: $CHAT_RESPONSE"

if echo "$CHAT_RESPONSE" | grep -q '"response":"Echo: Test message"'; then
    echo "   ✅ Chat endpoint passed"
else
    echo "   ❌ Chat endpoint failed"
    docker stop $CONTAINER_ID >/dev/null
    exit 1
fi
echo ""

# Test with environment variables
echo "Testing with custom environment variables..."
docker stop $CONTAINER_ID >/dev/null 2>&1 || true
sleep 2  # Give time for port to be released

CONTAINER_ID=$(docker run --rm -d -p 8001:8001 \
    -e PORT=8001 \
    -e HOST=0.0.0.0 \
    -e FORMATION_ID=test-formation \
    $IMAGE)
echo "   Container ID: ${CONTAINER_ID:0:12}"
sleep 5

ENV_HEALTH=$(curl -s http://localhost:8001/health 2>/dev/null || echo "{}")
if echo "$ENV_HEALTH" | grep -q '"status":"ok"'; then
    echo "   ✅ Environment variables working"
else
    echo "   ❌ Environment variables failed: $ENV_HEALTH"
    docker stop $CONTAINER_ID >/dev/null 2>&1 || true
    exit 1
fi
echo ""

# Cleanup
echo "Stopping container..."
docker stop $CONTAINER_ID >/dev/null
echo ""

echo "✅ All tests passed!"
echo ""
echo "Summary:"
echo "  - Health endpoint: ✅"
echo "  - Chat endpoint: ✅"
echo "  - Environment variables: ✅"
echo ""
echo "Ready for server integration!"
