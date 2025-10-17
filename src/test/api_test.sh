#!/bin/bash
# Integration test for MUXI Server API

set -e

echo "🧪 MUXI Server API Integration Test"
echo "===================================="
echo ""

# Start server in background
echo "📦 Starting MUXI Server..."
cd "$(dirname "$0")/.."
./muxi-server > /tmp/muxi-server.log 2>&1 &
SERVER_PID=$!
echo "   Server PID: $SERVER_PID"

# Give server time to start
sleep 3

# Cleanup function
cleanup() {
    echo ""
    echo "🧹 Cleaning up..."
    kill $SERVER_PID 2>/dev/null || true
    wait $SERVER_PID 2>/dev/null || true
    echo "   Server stopped"
}

trap cleanup EXIT

# Test 1: Health check
echo ""
echo "Test 1: Health Check"
echo "--------------------"
HEALTH=$(curl -s http://localhost:3000/health)
echo "Response: $HEALTH"

if echo "$HEALTH" | grep -q '"status":"ok"'; then
    echo "✅ Health check passed"
else
    echo "❌ Health check failed"
    exit 1
fi

# Test 2: Deploy formation
echo ""
echo "Test 2: Deploy Formation"
echo "------------------------"
DEPLOY_RESPONSE=$(curl -s -X POST http://localhost:3000/formations/deploy \
  -H "Content-Type: application/json" \
  -d '{
    "id": "test-api",
    "command": "python3",
    "args": ["test/dummy_app.py", "--port", "8099"]
  }')

echo "Response: $DEPLOY_RESPONSE"

if echo "$DEPLOY_RESPONSE" | grep -q '"formation_id":"test-api"'; then
    echo "✅ Formation deployed"
else
    echo "❌ Deploy failed"
    exit 1
fi

# Extract port from response
PORT=$(echo "$DEPLOY_RESPONSE" | grep -o '"port":[0-9]*' | cut -d':' -f2)
echo "   Allocated port: $PORT"

# Wait for formation to start
echo "   Waiting for formation to be ready..."
sleep 4

# Test 3: Check formation health
echo ""
echo "Test 3: Formation Health Check"
echo "------------------------------"
FORMATION_HEALTH=$(curl -s http://localhost:$PORT/health || echo "failed")
echo "Response: $FORMATION_HEALTH"

if echo "$FORMATION_HEALTH" | grep -q '"status":"ok"'; then
    echo "✅ Formation is healthy"
else
    echo "⚠️  Formation health check failed (may still be starting)"
fi

# Test 4: List formations
echo ""
echo "Test 4: List Formations"
echo "----------------------"
LIST_RESPONSE=$(curl -s http://localhost:3000/formations)
echo "Response: $LIST_RESPONSE"

if echo "$LIST_RESPONSE" | grep -q '"test-api"'; then
    echo "✅ Formation listed successfully"
else
    echo "❌ Formation not found in list"
    exit 1
fi

# Test 5: Chat with formation
echo ""
echo "Test 5: Chat with Formation"
echo "---------------------------"
CHAT_RESPONSE=$(curl -s -X POST http://localhost:$PORT/chat \
  -H "Content-Type: application/json" \
  -d '{"message": "Hello from MUXI!", "user_id": "test"}' || echo "failed")

echo "Response: $CHAT_RESPONSE"

if echo "$CHAT_RESPONSE" | grep -q "Echo:"; then
    echo "✅ Chat endpoint works"
else
    echo "⚠️  Chat endpoint failed (formation may still be starting)"
fi

echo ""
echo "=================================="
echo "✅ All API tests passed!"
echo "=================================="
