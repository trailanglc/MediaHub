#!/usr/bin/env bash
# Smoke tests for auth/route security against a running API.
# Usage:
#   export API_URL=http://localhost:8080
#   export OWNER_EMAIL=owner@example.com
#   export OWNER_PASSWORD='your-password'
#   ./scripts/security-smoke.sh

set -euo pipefail

API_URL="${API_URL:-http://localhost:8080}"
COOKIE_JAR="$(mktemp)"
trap 'rm -f "$COOKIE_JAR"' EXIT

red() { printf '\033[31m%s\033[0m\n' "$*"; }
green() { printf '\033[32m%s\033[0m\n' "$*"; }

expect_status() {
  local name="$1" want="$2" got="$3" body="$4"
  if [[ "$got" != "$want" ]]; then
    red "FAIL $name: expected HTTP $want, got $got — $body"
    exit 1
  fi
  green "OK   $name ($want)"
}

echo "=== MediaHub security smoke (API: $API_URL) ==="

# A: unauthenticated
code=$(curl -s -o /tmp/sec-body.txt -w '%{http_code}' "$API_URL/api/auth/me")
expect_status "unauthenticated /api/auth/me" "401" "$code" "$(cat /tmp/sec-body.txt)"

code=$(curl -s -o /tmp/sec-body.txt -w '%{http_code}' "$API_URL/api/system/health")
expect_status "unauthenticated /api/system/health" "401" "$code" "$(cat /tmp/sec-body.txt)"

# CORS
cors=$(curl -s -D - -o /dev/null -X OPTIONS "$API_URL/api/auth/login" \
  -H "Origin: https://evil.example" \
  -H "Access-Control-Request-Method: POST" | tr -d '\r' | grep -i '^access-control-allow-origin:' || true)
if echo "$cors" | grep -qi 'evil.example'; then
  red "FAIL CORS reflects evil origin: $cors"
  exit 1
fi
green "OK   CORS does not reflect evil origin"

if [[ -z "${OWNER_EMAIL:-}" || -z "${OWNER_PASSWORD:-}" ]]; then
  echo "Skip login tests: set OWNER_EMAIL and OWNER_PASSWORD"
  exit 0
fi

# Owner login
code=$(curl -s -c "$COOKIE_JAR" -b "$COOKIE_JAR" -o /tmp/sec-body.txt -w '%{http_code}' \
  -X POST "$API_URL/api/auth/login" \
  -H 'Content-Type: application/json' \
  -d "{\"email\":\"$OWNER_EMAIL\",\"password\":\"$OWNER_PASSWORD\"}")
expect_status "owner login" "200" "$code" "$(cat /tmp/sec-body.txt)"

code=$(curl -s -b "$COOKIE_JAR" -o /tmp/sec-body.txt -w '%{http_code}' "$API_URL/api/system/health")
expect_status "owner system health" "200" "$code" "$(cat /tmp/sec-body.txt)"

# Logout revokes access
code=$(curl -s -b "$COOKIE_JAR" -c "$COOKIE_JAR" -o /dev/null -w '%{http_code}' \
  -X POST "$API_URL/api/auth/logout")
expect_status "logout" "204" "$code" ""

code=$(curl -s -b "$COOKIE_JAR" -o /tmp/sec-body.txt -w '%{http_code}' "$API_URL/api/auth/me")
expect_status "me after logout" "401" "$code" "$(cat /tmp/sec-body.txt)"

green "=== All smoke checks passed ==="
