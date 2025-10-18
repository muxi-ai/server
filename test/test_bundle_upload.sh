#!/bin/bash
#
# Test script for MUXI Server formation bundle upload
#
# Tests:
# - POST /rpc/formations/deploy with application/gzip content-type
# - Extract → Parse → Deploy → Spawn flow
#

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
SERVER_URL="http://localhost:7890"
BUNDLE_PATH="./test/rpc/formations/test-bundle.tar.gz"

echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}MUXI Server Bundle Upload Test${NC}"
echo -e "${BLUE}========================================${NC}"
echo ""

# Use system Python
PYTHON3="/usr/bin/python3"

# Helper function to generate HMAC signature
generate_signature() {
    local secret="$1"
    local timestamp="$2"
    local method="$3"
    local path="$4"
    
    $PYTHON3 -c "
import hmac
import hashlib
import base64
import sys

secret = sys.argv[1]
timestamp = sys.argv[2]
method = sys.argv[3]
path = sys.argv[4]

message = f'{timestamp};{method};{path}'
signature = hmac.new(secret.encode(), message.encode(), hashlib.sha256).digest()
print(base64.b64encode(signature).decode())
" "$secret" "$timestamp" "$method" "$path"
}

# Read credentials from config
CONFIG_FILE="$HOME/.muxi/server/config.yaml"
if [ ! -f "$CONFIG_FILE" ]; then
    echo -e "${RED}✗ Config file not found: $CONFIG_FILE${NC}"
    echo -e "${YELLOW}  Run: muxi-server init${NC}"
    exit 1
fi

# Extract key and secret from config (using grep/awk for simple YAML parsing)
echo -e "${YELLOW}→ Reading credentials from config...${NC}"
KEY=$(grep "key:" "$CONFIG_FILE" | grep -v "api_keys" | awk '{print $2}' | tr -d '*')
SECRET=$(grep "secret:" "$CONFIG_FILE" | awk '{print $2}')

if [ -z "$KEY" ] || [ -z "$SECRET" ]; then
    echo -e "${RED}✗ Failed to read credentials from config${NC}"
    exit 1
fi

echo -e "${GREEN}✓ Credentials loaded${NC}"
echo -e "  Key: ${KEY:0:10}...${KEY: -10}"
echo -e "  Secret: ${SECRET:0:10}...${SECRET: -10}"
echo ""

# Check if server is running
echo -e "${YELLOW}→ Checking if server is running...${NC}"
if ! curl -s "${SERVER_URL}/health" > /dev/null 2>&1; then
    echo -e "${RED}✗ Server is not running at ${SERVER_URL}${NC}"
    echo -e "${YELLOW}  Start it with: ./muxi-server${NC}"
    exit 1
fi
echo -e "${GREEN}✓ Server is running${NC}"
echo ""

# Check if bundle exists
echo -e "${YELLOW}→ Checking if bundle exists...${NC}"
if [ ! -f "$BUNDLE_PATH" ]; then
    echo -e "${RED}✗ Bundle not found: $BUNDLE_PATH${NC}"
    exit 1
fi
BUNDLE_SIZE=$(ls -lh "$BUNDLE_PATH" | awk '{print $5}')
echo -e "${GREEN}✓ Bundle found: $BUNDLE_PATH ($BUNDLE_SIZE)${NC}"
echo ""

# Test: Deploy Formation Bundle
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${YELLOW}Test: POST /rpc/formations/deploy (Bundle Upload)${NC}"
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"

# Generate signature for authentication
TIMESTAMP=$(date +%s)
METHOD="POST"
PATH="/rpc/formations/deploy"
SIGNATURE=$(generate_signature "$SECRET" "$TIMESTAMP" "$METHOD" "$PATH")

echo -e "${YELLOW}→ Uploading bundle...${NC}"
DEPLOY_RESPONSE=$(curl -s -w "\nHTTP_STATUS:%{http_code}" \
    -X POST "${SERVER_URL}/rpc/formations/deploy" \
    -H "Authorization: MUXI-HMAC key=$KEY, timestamp=$TIMESTAMP, signature=$SIGNATURE" \
    -H "Content-Type: application/gzip" \
    --data-binary "@$BUNDLE_PATH")

HTTP_STATUS=$(echo "$DEPLOY_RESPONSE" | grep HTTP_STATUS | cut -d: -f2)
BODY=$(echo "$DEPLOY_RESPONSE" | grep -v HTTP_STATUS)

echo ""
echo -e "${BLUE}Response Status:${NC} $HTTP_STATUS"
echo -e "${BLUE}Response Body:${NC}"
echo "$BODY" | $PYTHON3 -m json.tool 2>/dev/null || echo "$BODY"
echo ""

if [ "$HTTP_STATUS" == "201" ]; then
    echo -e "${GREEN}✅ Formation deployed successfully!${NC}"
    
    # Extract formation ID and port from response
    FORMATION_ID=$(echo "$BODY" | $PYTHON3 -c "import sys, json; print(json.load(sys.stdin)['formation_id'])" 2>/dev/null || echo "")
    PORT=$(echo "$BODY" | $PYTHON3 -c "import sys, json; print(json.load(sys.stdin)['port'])" 2>/dev/null || echo "")
    
    if [ -n "$FORMATION_ID" ] && [ -n "$PORT" ]; then
        echo ""
        echo -e "${BLUE}Formation Details:${NC}"
        echo -e "  ID: $FORMATION_ID"
        echo -e "  Port: $PORT"
        echo ""
        
        # Wait for formation to start
        echo -e "${YELLOW}→ Waiting for formation to start (3s)...${NC}"
        sleep 3
        
        # Test health endpoint
        echo -e "${YELLOW}→ Testing formation health endpoint...${NC}"
        HEALTH_RESPONSE=$(curl -s "http://localhost:$PORT/health" || echo "ERROR")
        
        if [[ "$HEALTH_RESPONSE" == *"healthy"* ]] || [[ "$HEALTH_RESPONSE" == *"ok"* ]]; then
            echo -e "${GREEN}✅ Formation is healthy!${NC}"
            echo -e "  Health: $HEALTH_RESPONSE"
        else
            echo -e "${RED}✗ Formation health check failed${NC}"
            echo -e "  Response: $HEALTH_RESPONSE"
        fi
    fi
else
    echo -e "${RED}✗ Deployment failed (HTTP $HTTP_STATUS)${NC}"
    exit 1
fi

echo ""
echo -e "${BLUE}========================================${NC}"
echo -e "${GREEN}✓ Bundle upload test completed!${NC}"
echo -e "${BLUE}========================================${NC}"
