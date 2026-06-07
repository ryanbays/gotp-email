#!/usr/bin/env bash

set -euo pipefail

APP_NAME="otp-service"
PORT="8080"
BASE_URL="http://localhost:${PORT}"

API_KEY="test-api-key"

APP_DIR="../src"
BIN="/tmp/${APP_NAME}"

echo "Building project..."

cd "$APP_DIR"
go mod tidy
go build -o "$BIN" .

echo -n "Build and run the service locally? [y/N]: "
read -r RUN_LOCAL

if [[ "${RUN_LOCAL}" =~ ^[Yy]$ ]]; then
    echo "Building project..."

    cd "$APP_DIR"
    go mod tidy
    go build -o "$BIN" .

    echo "Starting service..."

    # Run in background
    API_KEY="$API_KEY" RULES_PATH="$APP_DIR/config/rules.json" "$BIN" &
    PID=$!

    cleanup() {
        echo "Cleaning up..."
        kill "$PID" || true
    }
    trap cleanup EXIT
else
    echo "Assuming service is already running..."
    echo -n "Enter API key: "
    read -r API_KEY
fi

# ----------------------------
# Wait for health
# ----------------------------
echo "Waiting for service..."
for i in {1..10}; do
    if curl -s "$BASE_URL/health" | grep -q ok; then
        echo "Service is up"
        break
    fi
    sleep 1
done

# ----------------------------
# Test data
# ----------------------------
INBOX="testinbox"

EMAIL_PAYLOAD=$(cat <<EOF
{
  "from": "uber@uber.com",
  "to": "${INBOX}@example.com",
  "raw": "<html><body><td class='p2b'>Your code is 123456</td></body></html>"
}
EOF
)

# ----------------------------
# 1. Health check
# ----------------------------
echo "Testing /health"
curl -s "$BASE_URL/health"

echo -e "\n"

# ----------------------------
# 2. Inbound email
# ----------------------------
echo "Testing /inbound-email"

curl -s -X POST "$BASE_URL/inbound-email" \
  -H "Content-Type: application/json" \
  -H "X-API-Key: $API_KEY" \
  -d "$EMAIL_PAYLOAD"

echo -e "\n"

sleep 1

# ----------------------------
# 3. Get OTP (protected)
# ----------------------------
echo "Testing /otp/:inbox"

curl -s "$BASE_URL/otp/$INBOX" \
  -H "X-API-Key: $API_KEY"

echo -e "\n"

# ----------------------------
# 4. Get history
# ----------------------------
echo "Testing /otp/:inbox/history"

curl -s "$BASE_URL/otp/$INBOX/history" \
  -H "X-API-Key: $API_KEY"

echo -e "\n"

# ----------------------------
# 5. Email cache
# ----------------------------
echo "Testing /email-cache/:inbox"

curl -s "$BASE_URL/email-cache/$INBOX" \
  -H "X-API-Key: $API_KEY"

echo -e "\n"

echo "All tests completed successfully"
