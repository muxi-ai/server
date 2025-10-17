#!/bin/bash
#
# Simple test script for MUXI Server formation bundle upload
# Uses openssl for HMAC signature generation
#

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

SERVER_URL="http://localhost:3000"
BUNDLE_PATH="./test/formations/test-bundle.tar.gz"
CONFIG_FILE="$HOME/.muxi/server/config.yaml"

echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}MUXI Server Bundle Upload Test${NC}"
echo -e "${BLUE}========================================${NC}"
echo ""

# Read credentials from config (simple grep/awk parsing)
echo -e "${YELLOW}→ Reading credentials from config...${NC}"
KEY=$(grep "key:" "$CONFIG_FILE" | grep -v "api_keys" | awk '{print $2}' | tr -d '*')
SECRET=$(grep "secret:" "$CONFIG_FILE" | awk '{print $2}')

if [ -z "$KEY" ] || [ -z "$SECRET" ]; then
    echo -e "${RED}✗ Failed to read credentials from config${NC}"
    exit 1
fi

echo -e "${GREEN}✓ Credentials loaded${NC}"
echo ""

# Check if server is running
echo -e "${YELLOW}→ Checking if server is running...${NC}"
if ! /usr/bin/curl -s "${SERVER_URL}/health" > /dev/null 2>&1; then
    echo -e "${RED}✗ Server is not running at ${SERVER_URL}${NC}"
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

# Generate HMAC signature using openssl
TIMESTAMP=$(date +%s)
METHOD="POST"
PATH="/formations/deploy"

echo -e "${YELLOW}→ Generating HMAC signature...${NC}"
MESSAGE="${TIMESTAMP};${METHOD};${PATH}"
SIGNATURE=$(echo -n "$MESSAGE" | /usr/bin/openssl dgst -sha256 -hmac "$SECRET" -binary | /usr/bin/base64)

echo -e "${GREEN}✓ Signature generated${NC}"
echo ""

# Upload bundle
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${YELLOW}Test: POST /formations/deploy (Bundle Upload)${NC}"
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"

echo -e "${YELLOW}→ Uploading bundle...${NC}"
RESPONSE=$(/usr/bin/curl -s -w "\nHTTP_STATUS:%{http_code}" \
    -X POST "${SERVER_URL}/formations/deploy" \
    -H "Authorization: MUXI-HMAC key=$KEY, timestamp=$TIMESTAMP, signature=$SIGNATURE" \
    -H "Content-Type: application/gzip" \
    --data-binary "@$BUNDLE_PATH")

HTTP_STATUS=$(echo "$RESPONSE" | grep HTTP_STATUS | cut -d: -f2)
BODY=$(echo "$RESPONSE" | grep -v HTTP_STATUS)

echo ""
echo -e "${BLUE}Response Status:${NC} $HTTP_STATUS"
echo -e "${BLUE}Response Body:${NC}"
echo "$BODY"
echo ""

if [ "$HTTP_STATUS" == "201" ]; then
    echo -e "${GREEN}✅ Formation deployed successfully!${NC}"
    echo ""
    echo -e "${BLUE}Check the deployed formation.yaml for metadata:${NC}"
    echo -e "${YELLOW}→ Looking for formation files...${NC}"
    
    FORMATION_DIR=$(find ~/.muxi/server/formations -name "test-chat-api" 2>/dev/null | head -1)
    if [ -n "$FORMATION_DIR" ]; then
        echo -e "${GREEN}✓ Found formation directory: $FORMATION_DIR${NC}"
        echo ""
        echo -e "${BLUE}formation.yaml metadata:${NC}"
        grep -E "^_(server_id|deployment_mode):" "$FORMATION_DIR/formation.yaml" || echo "No metadata fields found"
    else
        echo -e "${YELLOW}⚠️  Formation directory not found${NC}"
    fi
else
    echo -e "${RED}✗ Deployment failed (HTTP $HTTP_STATUS)${NC}"
    exit 1
fi

echo ""
echo -e "${BLUE}========================================${NC}"
echo -e "${GREEN}✓ Bundle upload test completed!${NC}"
echo -e "${BLUE}========================================${NC}"
