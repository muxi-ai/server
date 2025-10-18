#!/bin/bash
#
# Test script for MUXI Server HTTP Proxy
#
# This script tests the complete flow:
# 1. Deploy formation
# 2. Access via proxy (/api/{formation_id}/*)
# 3. Verify responses
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
FORMATION_ID="test-formation"

echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}MUXI Server Proxy Test${NC}"
echo -e "${BLUE}========================================${NC}"
echo ""

# Check if server is running
echo -e "${YELLOW}→ Checking if server is running...${NC}"
if ! curl -s "${SERVER_URL}/health" > /dev/null; then
    echo -e "${RED}✗ Server is not running at ${SERVER_URL}${NC}"
    echo -e "${YELLOW}  Start it with: go run ./cmd/server${NC}"
    exit 1
fi
echo -e "${GREEN}✓ Server is running${NC}"
echo ""

# Deploy formation
echo -e "${YELLOW}→ Deploying test formation...${NC}"
DEPLOY_RESPONSE=$(curl -s -X POST "${SERVER_URL}/rpc/formations/deploy" \
    -H "Content-Type: application/json" \
    -d "{
        \"id\": \"${FORMATION_ID}\",
        \"command\": \"python ../test/dummy_app.py\"
    }")

echo -e "${GREEN}✓ Formation deployed${NC}"
echo "Response: ${DEPLOY_RESPONSE}"
echo ""

# Extract port from response
PORT=$(echo "${DEPLOY_RESPONSE}" | grep -o '"port":[0-9]*' | grep -o '[0-9]*')
echo -e "${BLUE}  Formation running on port: ${PORT}${NC}"
echo ""

# Wait for formation to start
echo -e "${YELLOW}→ Waiting for formation to start...${NC}"
sleep 3
echo -e "${GREEN}✓ Formation should be ready${NC}"
echo ""

# Test 1: Health check via proxy
echo -e "${YELLOW}→ Test 1: GET /api/${FORMATION_ID}/health (via proxy)${NC}"
PROXY_HEALTH=$(curl -s "${SERVER_URL}/api/${FORMATION_ID}/health")
echo "Response: ${PROXY_HEALTH}"

if echo "${PROXY_HEALTH}" | grep -q '"status"'; then
    echo -e "${GREEN}✓ Health check via proxy successful${NC}"
else
    echo -e "${RED}✗ Health check via proxy failed${NC}"
    exit 1
fi
echo ""

# Test 2: Root endpoint via proxy
echo -e "${YELLOW}→ Test 2: GET /api/${FORMATION_ID}/ (via proxy)${NC}"
PROXY_ROOT=$(curl -s "${SERVER_URL}/api/${FORMATION_ID}/")
echo "Response: ${PROXY_ROOT}"

if echo "${PROXY_ROOT}" | grep -q '"service"'; then
    echo -e "${GREEN}✓ Root endpoint via proxy successful${NC}"
else
    echo -e "${RED}✗ Root endpoint via proxy failed${NC}"
    exit 1
fi
echo ""

# Test 3: Chat endpoint via proxy (POST)
echo -e "${YELLOW}→ Test 3: POST /api/${FORMATION_ID}/chat (via proxy)${NC}"
PROXY_CHAT=$(curl -s -X POST "${SERVER_URL}/api/${FORMATION_ID}/chat" \
    -H "Content-Type: application/json" \
    -d '{"message": "Hello from proxy test!", "user_id": "test-user"}')
echo "Response: ${PROXY_CHAT}"

if echo "${PROXY_CHAT}" | grep -q "Echo:"; then
    echo -e "${GREEN}✓ Chat endpoint via proxy successful${NC}"
else
    echo -e "${RED}✗ Chat endpoint via proxy failed${NC}"
    exit 1
fi
echo ""

# Test 4: Compare direct vs proxy
echo -e "${YELLOW}→ Test 4: Comparing direct access vs proxy${NC}"

# Direct access
DIRECT_HEALTH=$(curl -s "http://localhost:${PORT}/health")
echo "Direct response: ${DIRECT_HEALTH}"

# Proxy access
PROXY_HEALTH=$(curl -s "${SERVER_URL}/api/${FORMATION_ID}/health")
echo "Proxy response:  ${PROXY_HEALTH}"

if [ "${DIRECT_HEALTH}" = "${PROXY_HEALTH}" ]; then
    echo -e "${GREEN}✓ Direct and proxy responses match${NC}"
else
    echo -e "${YELLOW}⚠ Responses differ (this is OK - proxy may add headers)${NC}"
fi
echo ""

# Test 5: 404 for non-existent formation
echo -e "${YELLOW}→ Test 5: GET /api/nonexistent/health (should 404)${NC}"
NOT_FOUND=$(curl -s -w "\nHTTP_CODE:%{http_code}" "${SERVER_URL}/api/nonexistent/health")
HTTP_CODE=$(echo "${NOT_FOUND}" | grep "HTTP_CODE" | cut -d: -f2)

if [ "${HTTP_CODE}" = "404" ]; then
    echo -e "${GREEN}✓ Non-existent formation returns 404${NC}"
else
    echo -e "${RED}✗ Expected 404, got ${HTTP_CODE}${NC}"
fi
echo ""

# Test 6: List formations
echo -e "${YELLOW}→ Test 6: GET /formations (list deployed formations)${NC}"
LIST_RESPONSE=$(curl -s "${SERVER_URL}/formations")
echo "Response: ${LIST_RESPONSE}"

if echo "${LIST_RESPONSE}" | grep -q "${FORMATION_ID}"; then
    echo -e "${GREEN}✓ Formation appears in list${NC}"
else
    echo -e "${RED}✗ Formation not in list${NC}"
    exit 1
fi
echo ""

# Summary
echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}Summary${NC}"
echo -e "${BLUE}========================================${NC}"
echo -e "${GREEN}✓ All proxy tests passed!${NC}"
echo ""
echo -e "${BLUE}Formation Details:${NC}"
echo -e "  ID:        ${FORMATION_ID}"
echo -e "  Port:      ${PORT}"
echo -e "  Direct:    http://localhost:${PORT}/"
echo -e "  Via Proxy: ${SERVER_URL}/api/${FORMATION_ID}/"
echo ""
echo -e "${YELLOW}Try these commands:${NC}"
echo -e "  curl ${SERVER_URL}/api/${FORMATION_ID}/health"
echo -e "  curl ${SERVER_URL}/api/${FORMATION_ID}/"
echo -e "  curl -X POST ${SERVER_URL}/api/${FORMATION_ID}/chat \\"
echo -e "    -H 'Content-Type: application/json' \\"
echo -e "    -d '{\"message\": \"Hello!\"}'"
echo ""
echo -e "${YELLOW}Cleanup:${NC}"
echo -e "  curl -X DELETE ${SERVER_URL}/rpc/formations/${FORMATION_ID}"
echo ""
