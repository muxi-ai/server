#!/bin/bash
# Test script for MUXI Server authentication

set -e

echo "🔐 MUXI Server Authentication Test"
echo "===================================="
echo ""

# Helper function to generate HMAC signature (requires Python)
generate_signature() {
    local secret=$1
    local timestamp=$2
    local method=$3
    local path=$4
    
    python3 -c "
import hmac
import hashlib
import base64

secret = '$secret'
message = f'{timestamp};{method};{path}'
signature = hmac.new(secret.encode(), message.encode(), hashlib.sha256).digest()
print(base64.b64encode(signature).decode())
"
}

# Server should be running with auth enabled

KEY="MUXI_test123"
SECRET="sk_testsecret456"
TIMESTAMP=$(date +%s)
METHOD="POST"
PATH="/formations/deploy"

echo "Test Configuration:"
echo "  Key: $KEY"
echo "  Secret: $SECRET"
echo "  Timestamp: $TIMESTAMP"
echo "  Method: $METHOD"
echo "  Path: $PATH"
echo ""

# Generate signature
SIGNATURE=$(generate_signature "$SECRET" "$TIMESTAMP" "$METHOD" "$PATH")
echo "Generated Signature: $SIGNATURE"
echo ""

# Test 1: Request without auth (should fail)
echo "Test 1: No Authorization Header"
echo "--------------------------------"
RESPONSE=$(curl -s -w "\\nHTTP_STATUS:%{http_code}" \
  -X POST http://localhost:3000/formations/deploy \
  -H "Content-Type: application/json" \
  -d '{"command":"echo","args":["test"]}')

HTTP_STATUS=$(echo "$RESPONSE" | grep HTTP_STATUS | cut -d: -f2)
BODY=$(echo "$RESPONSE" | grep -v HTTP_STATUS)

echo "Status: $HTTP_STATUS"
echo "Body: $BODY"

if [ "$HTTP_STATUS" == "401" ]; then
    echo "✅ Correctly rejected (401)"
else
    echo "❌ Expected 401, got $HTTP_STATUS"
fi

echo ""

# Test 2: Request with valid auth (should succeed if auth enabled, or succeed if disabled)
echo "Test 2: With Valid Authorization"
echo "---------------------------------"
RESPONSE=$(curl -s -w "\\nHTTP_STATUS:%{http_code}" \
  -X POST http://localhost:3000/formations/deploy \
  -H "Authorization: MUXI-HMAC key=$KEY, timestamp=$TIMESTAMP, signature=$SIGNATURE" \
  -H "Content-Type: application/json" \
  -d '{"id":"test-auth","command":"python3","args":["test/dummy_app.py","--port","8099"]}')

HTTP_STATUS=$(echo "$RESPONSE" | grep HTTP_STATUS | cut -d: -f2)
BODY=$(echo "$RESPONSE" | grep -v HTTP_STATUS)

echo "Status: $HTTP_STATUS"
echo "Body: $BODY"

if [ "$HTTP_STATUS" == "201" ] || [ "$HTTP_STATUS" == "200" ]; then
    echo "✅ Request succeeded ($HTTP_STATUS)"
elif [ "$HTTP_STATUS" == "401" ]; then
    echo "⚠️  Rejected with 401 (check if key/secret match server config)"
else
    echo "❌ Unexpected status: $HTTP_STATUS"
fi

echo ""

# Test 3: Request with invalid key
echo "Test 3: Invalid Key"
echo "-------------------"
RESPONSE=$(curl -s -w "\\nHTTP_STATUS:%{http_code}" \
  -X POST http://localhost:3000/formations/deploy \
  -H "Authorization: MUXI-HMAC key=WRONG_KEY, timestamp=$TIMESTAMP, signature=$SIGNATURE" \
  -H "Content-Type: application/json" \
  -d '{"command":"echo","args":["test"]}')

HTTP_STATUS=$(echo "$RESPONSE" | grep HTTP_STATUS | cut -d: -f2)
BODY=$(echo "$RESPONSE" | grep -v HTTP_STATUS)

echo "Status: $HTTP_STATUS"
if [ "$HTTP_STATUS" == "401" ]; then
    echo "✅ Correctly rejected (401)"
else
    echo "❌ Expected 401, got $HTTP_STATUS"
fi

echo ""

# Test 4: Health check (should work without auth)
echo "Test 4: Health Check (No Auth Required)"
echo "----------------------------------------"
RESPONSE=$(curl -s -w "\\nHTTP_STATUS:%{http_code}" \
  http://localhost:3000/health)

HTTP_STATUS=$(echo "$RESPONSE" | grep HTTP_STATUS | cut -d: -f2)
BODY=$(echo "$RESPONSE" | grep -v HTTP_STATUS)

echo "Status: $HTTP_STATUS"
echo "Body: $BODY"

if [ "$HTTP_STATUS" == "200" ]; then
    echo "✅ Health check works without auth"
else
    echo "❌ Expected 200, got $HTTP_STATUS"
fi

echo ""
echo "===================================="
echo "✅ Authentication tests complete!"
