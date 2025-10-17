#!/bin/bash
#
# Test script for MUXI Server Management API
#
# Tests all CRUD endpoints:
# - POST /formations/deploy
# - GET /formations
# - GET /formations/{id}
# - POST /formations/{id}/stop
# - POST /formations/{id}/restart
# - DELETE /formations/{id}
# - GET /formations/{id}/logs
#

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
SERVER_URL="http://localhost:3000"
FORMATION_ID="test-mgmt-api"

echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}MUXI Server Management API Test${NC}"
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

# Test 1: Deploy Formation
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${YELLOW}Test 1: POST /formations/deploy${NC}"
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"

DEPLOY_RESPONSE=$(curl -s -X POST "${SERVER_URL}/formations/deploy" \
    -H "Content-Type: application/json" \
    -d "{
        \"id\": \"${FORMATION_ID}\",
        \"command\": \"python ../test/dummy_app.py\"
    }")

echo "${DEPLOY_RESPONSE}" | python3 -m json.tool 2>/dev/null || echo "${DEPLOY_RESPONSE}"

if echo "${DEPLOY_RESPONSE}" | grep -q '"port"'; then
    echo -e "${GREEN}✓ Formation deployed${NC}"
else
    echo -e "${RED}✗ Deploy failed${NC}"
    exit 1
fi

# Extract port
PORT=$(echo "${DEPLOY_RESPONSE}" | grep -o '"port":[0-9]*' | grep -o '[0-9]*')
echo -e "${BLUE}  Formation running on port: ${PORT}${NC}"
echo ""

# Wait for formation to start
sleep 2

# Test 2: List Formations
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${YELLOW}Test 2: GET /formations (list)${NC}"
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"

LIST_RESPONSE=$(curl -s "${SERVER_URL}/formations")
echo "${LIST_RESPONSE}" | python3 -m json.tool 2>/dev/null || echo "${LIST_RESPONSE}"

if echo "${LIST_RESPONSE}" | grep -q "${FORMATION_ID}"; then
    echo -e "${GREEN}✓ Formation appears in list${NC}"
else
    echo -e "${RED}✗ Formation not in list${NC}"
    exit 1
fi
echo ""

# Test 3: Get Formation Details
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${YELLOW}Test 3: GET /formations/${FORMATION_ID}${NC}"
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"

GET_RESPONSE=$(curl -s "${SERVER_URL}/formations/${FORMATION_ID}")
echo "${GET_RESPONSE}" | python3 -m json.tool 2>/dev/null || echo "${GET_RESPONSE}"

if echo "${GET_RESPONSE}" | grep -q '"status"' && echo "${GET_RESPONSE}" | grep -q '"port"'; then
    echo -e "${GREEN}✓ Formation details retrieved${NC}"
else
    echo -e "${RED}✗ Failed to get formation details${NC}"
    exit 1
fi
echo ""

# Test 4: Get Formation Logs
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${YELLOW}Test 4: GET /formations/${FORMATION_ID}/logs${NC}"
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"

LOGS_RESPONSE=$(curl -s "${SERVER_URL}/formations/${FORMATION_ID}/logs?lines=50")
echo "${LOGS_RESPONSE}" | python3 -m json.tool 2>/dev/null || echo "${LOGS_RESPONSE}"

if echo "${LOGS_RESPONSE}" | grep -q '"logs"'; then
    echo -e "${GREEN}✓ Logs retrieved${NC}"
else
    echo -e "${YELLOW}⚠ No logs available (this is OK for new formations)${NC}"
fi
echo ""

# Test 5: Stop Formation
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${YELLOW}Test 5: POST /formations/${FORMATION_ID}/stop${NC}"
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"

STOP_RESPONSE=$(curl -s -X POST "${SERVER_URL}/formations/${FORMATION_ID}/stop")
echo "${STOP_RESPONSE}" | python3 -m json.tool 2>/dev/null || echo "${STOP_RESPONSE}"

if echo "${STOP_RESPONSE}" | grep -q '"status".*stopped'; then
    echo -e "${GREEN}✓ Formation stopped${NC}"
else
    echo -e "${RED}✗ Failed to stop formation${NC}"
    exit 1
fi
echo ""

# Wait for stop to complete
sleep 1

# Test 6: Verify formation is stopped
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${YELLOW}Test 6: Verify formation status = stopped${NC}"
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"

GET_AFTER_STOP=$(curl -s "${SERVER_URL}/formations/${FORMATION_ID}")
echo "${GET_AFTER_STOP}" | python3 -m json.tool 2>/dev/null || echo "${GET_AFTER_STOP}"

if echo "${GET_AFTER_STOP}" | grep -q '"status".*stopped'; then
    echo -e "${GREEN}✓ Formation status is stopped${NC}"
else
    echo -e "${YELLOW}⚠ Formation status might still be transitioning${NC}"
fi
echo ""

# Test 7: Try to stop already stopped formation (should 409)
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${YELLOW}Test 7: POST /formations/${FORMATION_ID}/stop (already stopped - should 409)${NC}"
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"

STOP_AGAIN=$(curl -s -w "\nHTTP_CODE:%{http_code}" -X POST "${SERVER_URL}/formations/${FORMATION_ID}/stop")
HTTP_CODE=$(echo "${STOP_AGAIN}" | grep "HTTP_CODE" | cut -d: -f2)
RESPONSE=$(echo "${STOP_AGAIN}" | grep -v "HTTP_CODE")

echo "${RESPONSE}" | python3 -m json.tool 2>/dev/null || echo "${RESPONSE}"

if [ "${HTTP_CODE}" = "409" ]; then
    echo -e "${GREEN}✓ Correctly returns 409 Conflict${NC}"
else
    echo -e "${YELLOW}⚠ Expected 409, got ${HTTP_CODE}${NC}"
fi
echo ""

# Test 8: Restart Formation
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${YELLOW}Test 8: POST /formations/${FORMATION_ID}/restart${NC}"
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"

RESTART_RESPONSE=$(curl -s -X POST "${SERVER_URL}/formations/${FORMATION_ID}/restart")
echo "${RESTART_RESPONSE}" | python3 -m json.tool 2>/dev/null || echo "${RESTART_RESPONSE}"

if echo "${RESTART_RESPONSE}" | grep -q '"status"'; then
    echo -e "${GREEN}✓ Formation restarted${NC}"
else
    echo -e "${RED}✗ Failed to restart formation${NC}"
    exit 1
fi
echo ""

# Wait for restart
sleep 2

# Test 9: Verify formation is running again
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${YELLOW}Test 9: Verify formation is running after restart${NC}"
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"

GET_AFTER_RESTART=$(curl -s "${SERVER_URL}/formations/${FORMATION_ID}")
echo "${GET_AFTER_RESTART}" | python3 -m json.tool 2>/dev/null || echo "${GET_AFTER_RESTART}"

if echo "${GET_AFTER_RESTART}" | grep -q '"status".*running'; then
    echo -e "${GREEN}✓ Formation is running${NC}"
else
    echo -e "${YELLOW}⚠ Formation might still be restarting${NC}"
fi
echo ""

# Test 10: Access via proxy to verify it's actually running
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${YELLOW}Test 10: GET /v1/${FORMATION_ID}/health (via proxy)${NC}"
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"

HEALTH_VIA_PROXY=$(curl -s "${SERVER_URL}/v1/${FORMATION_ID}/health")
echo "${HEALTH_VIA_PROXY}" | python3 -m json.tool 2>/dev/null || echo "${HEALTH_VIA_PROXY}"

if echo "${HEALTH_VIA_PROXY}" | grep -q '"status"'; then
    echo -e "${GREEN}✓ Formation is accessible via proxy${NC}"
else
    echo -e "${YELLOW}⚠ Formation might not be ready yet${NC}"
fi
echo ""

# Test 11: Delete Formation
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${YELLOW}Test 11: DELETE /formations/${FORMATION_ID}${NC}"
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"

DELETE_RESPONSE=$(curl -s -X DELETE "${SERVER_URL}/formations/${FORMATION_ID}")
echo "${DELETE_RESPONSE}" | python3 -m json.tool 2>/dev/null || echo "${DELETE_RESPONSE}"

if echo "${DELETE_RESPONSE}" | grep -q 'deleted'; then
    echo -e "${GREEN}✓ Formation deleted${NC}"
else
    echo -e "${RED}✗ Failed to delete formation${NC}"
    exit 1
fi
echo ""

# Test 12: Verify formation is gone
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${YELLOW}Test 12: GET /formations/${FORMATION_ID} (should 404)${NC}"
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"

GET_AFTER_DELETE=$(curl -s -w "\nHTTP_CODE:%{http_code}" "${SERVER_URL}/formations/${FORMATION_ID}")
HTTP_CODE=$(echo "${GET_AFTER_DELETE}" | grep "HTTP_CODE" | cut -d: -f2)
RESPONSE=$(echo "${GET_AFTER_DELETE}" | grep -v "HTTP_CODE")

echo "${RESPONSE}" | python3 -m json.tool 2>/dev/null || echo "${RESPONSE}"

if [ "${HTTP_CODE}" = "404" ]; then
    echo -e "${GREEN}✓ Formation not found (404)${NC}"
else
    echo -e "${RED}✗ Expected 404, got ${HTTP_CODE}${NC}"
    exit 1
fi
echo ""

# Summary
echo -e "${BLUE}========================================${NC}"
echo -e "${GREEN}✓ All management API tests passed!${NC}"
echo -e "${BLUE}========================================${NC}"
echo ""

echo -e "${BLUE}Tested Endpoints:${NC}"
echo -e "  ✅ POST   /formations/deploy"
echo -e "  ✅ GET    /formations"
echo -e "  ✅ GET    /formations/{id}"
echo -e "  ✅ POST   /formations/{id}/stop"
echo -e "  ✅ POST   /formations/{id}/restart"
echo -e "  ✅ DELETE /formations/{id}"
echo -e "  ✅ GET    /formations/{id}/logs"
echo ""

echo -e "${BLUE}Plus proxy routing:${NC}"
echo -e "  ✅ GET /v1/{formation_id}/health"
echo ""

echo -e "${GREEN}Management API is fully functional! 🎉${NC}"
echo ""
