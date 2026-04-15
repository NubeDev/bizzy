#!/usr/bin/env bash
#
# Full API test script for nube-server.
#
# Usage:
#   make start          # start servers first
#   make test-api       # run this script
#
#   # or directly:
#   bash scripts/test-api.sh [server-port] [fake-port]
#
set -euo pipefail

SERVER_PORT="${1:-8090}"
FAKE_PORT="${2:-9001}"
BASE="http://localhost:${SERVER_PORT}"
FAKE="http://localhost:${FAKE_PORT}"

PASS=0
FAIL=0
TOTAL=0

# ---- helpers ----

green()  { printf "\033[32m%s\033[0m" "$1"; }
red()    { printf "\033[31m%s\033[0m" "$1"; }
bold()   { printf "\033[1m%s\033[0m" "$1"; }
dim()    { printf "\033[2m%s\033[0m" "$1"; }

assert_status() {
  local label="$1" expected="$2" actual="$3" body="$4"
  TOTAL=$((TOTAL + 1))
  if [ "$actual" = "$expected" ]; then
    PASS=$((PASS + 1))
    printf "  $(green PASS)  %s\n" "$label"
  else
    FAIL=$((FAIL + 1))
    printf "  $(red FAIL)  %s — expected %s, got %s\n" "$label" "$expected" "$actual"
    printf "        %s\n" "$(dim "$body" | head -1)"
  fi
}

assert_json() {
  local label="$1" key="$2" expected="$3" body="$4"
  TOTAL=$((TOTAL + 1))
  local actual
  actual=$(echo "$body" | python3 -c "import sys,json; print(json.load(sys.stdin).get('$key',''))" 2>/dev/null || echo "PARSE_ERROR")
  if [ "$actual" = "$expected" ]; then
    PASS=$((PASS + 1))
    printf "  $(green PASS)  %s\n" "$label"
  else
    FAIL=$((FAIL + 1))
    printf "  $(red FAIL)  %s — expected %s=%s, got %s\n" "$label" "$key" "$expected" "$actual"
  fi
}

assert_contains() {
  local label="$1" needle="$2" body="$3"
  TOTAL=$((TOTAL + 1))
  if echo "$body" | grep -q "$needle"; then
    PASS=$((PASS + 1))
    printf "  $(green PASS)  %s\n" "$label"
  else
    FAIL=$((FAIL + 1))
    printf "  $(red FAIL)  %s — expected to contain '%s'\n" "$label" "$needle"
  fi
}

assert_count() {
  local label="$1" expected="$2" body="$3"
  TOTAL=$((TOTAL + 1))
  local actual
  actual=$(echo "$body" | python3 -c "import sys,json; print(len(json.load(sys.stdin)))" 2>/dev/null || echo "0")
  if [ "$actual" = "$expected" ]; then
    PASS=$((PASS + 1))
    printf "  $(green PASS)  %s\n" "$label"
  else
    FAIL=$((FAIL + 1))
    printf "  $(red FAIL)  %s — expected %s items, got %s\n" "$label" "$expected" "$actual"
  fi
}

http() {
  local method="$1" path="$2" token="${3:-}" data="${4:-}"
  local args=(-s -w "\n%{http_code}" -X "$method" "${BASE}${path}")
  args+=(-H "Content-Type: application/json")
  [ -n "$token" ] && args+=(-H "Authorization: Bearer $token")
  [ -n "$data" ]  && args+=(-d "$data")
  curl "${args[@]}" 2>/dev/null
}

split_response() {
  local raw="$1"
  BODY=$(echo "$raw" | sed '$d')
  STATUS=$(echo "$raw" | tail -1)
}

mcp_call() {
  local token="$1" session="$2" method="$3" params="$4"
  local id=$RANDOM
  curl -s -X POST "${BASE}/mcp" \
    -H "Authorization: Bearer $token" \
    -H "Content-Type: application/json" \
    -H "Accept: application/json, text/event-stream" \
    -H "Mcp-Session-Id: $session" \
    -d "{\"jsonrpc\":\"2.0\",\"id\":$id,\"method\":\"$method\",\"params\":$params}" \
    2>/dev/null | grep "^data:" | sed 's/^data: //' | head -1
}

mcp_init() {
  local token="$1"
  local resp
  resp=$(curl -s -D- -X POST "${BASE}/mcp" \
    -H "Authorization: Bearer $token" \
    -H "Content-Type: application/json" \
    -H "Accept: application/json, text/event-stream" \
    -d '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26","capabilities":{},"clientInfo":{"name":"test-script","version":"1.0"}}}' \
    2>/dev/null)
  echo "$resp" | grep -i 'mcp-session-id' | tr -d '\r' | awk '{print $2}'
}

# ---- preflight ----

echo ""
echo "$(bold '=== Nube Server API Test Suite ===')"
echo ""
echo "  Server: $BASE"
echo "  Fake API: $FAKE"
echo ""

# Check servers are running.
if ! curl -s "${BASE}/health" > /dev/null 2>&1; then
  echo "$(red 'ERROR'): nube-server not running on :${SERVER_PORT}"
  echo "  Run: make start"
  exit 1
fi
if ! curl -s "${FAKE}/api/v1/health" > /dev/null 2>&1; then
  echo "$(red 'ERROR'): fakeserver not running on :${FAKE_PORT}"
  echo "  Run: make start"
  exit 1
fi

# ================================================================
# 1. HEALTH CHECK
# ================================================================
echo "$(bold '--- 1. Health Check ---')"

split_response "$(http GET /health)"
assert_status "GET /health returns 200" "200" "$STATUS" "$BODY"
assert_json   "status is ok" "status" "ok" "$BODY"

# ================================================================
# 2. BOOTSTRAP
# ================================================================
echo ""
echo "$(bold '--- 2. Bootstrap ---')"

split_response "$(http POST /bootstrap '' '{"workspaceName":"NubeIO","adminName":"Admin","adminEmail":"ap@nube-io.com"}')"
assert_status "POST /bootstrap returns 201" "201" "$STATUS" "$BODY"

ADMIN_TOKEN=$(echo "$BODY" | python3 -c "import sys,json; print(json.load(sys.stdin)['admin']['token'])")
WS_ID=$(echo "$BODY" | python3 -c "import sys,json; print(json.load(sys.stdin)['workspace']['id'])")
ADMIN_ID=$(echo "$BODY" | python3 -c "import sys,json; print(json.load(sys.stdin)['admin']['id'])")
echo "  $(dim "admin token: ${ADMIN_TOKEN:0:16}...")"
echo "  $(dim "workspace:   $WS_ID")"

# Reject re-bootstrap.
split_response "$(http POST /bootstrap '' '{"workspaceName":"Hack","adminName":"X","adminEmail":"x@x.com"}')"
assert_status "re-bootstrap rejected (409)" "409" "$STATUS" "$BODY"

# ================================================================
# 3. AUTH
# ================================================================
echo ""
echo "$(bold '--- 3. Authentication ---')"

split_response "$(http GET /users/me "$ADMIN_TOKEN")"
assert_status "GET /users/me with valid token" "200" "$STATUS" "$BODY"
assert_json   "returns admin email" "email" "ap@nube-io.com" "$BODY"

split_response "$(http GET /users/me "")"
assert_status "missing token rejected (401)" "401" "$STATUS" "$BODY"

split_response "$(http GET /users/me "bad-token-here")"
assert_status "bad token rejected (401)" "401" "$STATUS" "$BODY"

# ================================================================
# 4. USER MANAGEMENT
# ================================================================
echo ""
echo "$(bold '--- 4. User Management ---')"

# Create regular user.
split_response "$(http POST "/workspaces/$WS_ID/users" "$ADMIN_TOKEN" '{"name":"Joe","email":"joe@nube-io.com","role":"user"}')"
assert_status "create user Joe" "201" "$STATUS" "$BODY"
JOE_TOKEN=$(echo "$BODY" | python3 -c "import sys,json; print(json.load(sys.stdin)['token'])")
JOE_ID=$(echo "$BODY" | python3 -c "import sys,json; print(json.load(sys.stdin)['id'])")
echo "  $(dim "joe token: ${JOE_TOKEN:0:16}...")"

# Joe can't access admin routes.
split_response "$(http POST /workspaces "$JOE_TOKEN" '{"name":"hack"}')"
assert_status "joe blocked from admin routes (403)" "403" "$STATUS" "$BODY"

# Admin impersonation.
BODY=$(curl -s "${BASE}/users/me" -H "Authorization: Bearer $ADMIN_TOKEN" -H "X-Act-As-User: $JOE_ID")
assert_json "admin impersonates joe" "name" "Joe" "$BODY"

# List workspace users.
split_response "$(http GET "/workspaces/$WS_ID/users" "$ADMIN_TOKEN")"
assert_status "list workspace users" "200" "$STATUS" "$BODY"
assert_count  "2 users in workspace" "2" "$BODY"

# ================================================================
# 5. TOKEN ROTATION
# ================================================================
echo ""
echo "$(bold '--- 5. Token Rotation ---')"

split_response "$(http POST "/users/$JOE_ID/token" "$JOE_TOKEN")"
assert_status "joe rotates own token" "200" "$STATUS" "$BODY"
JOE_NEW_TOKEN=$(echo "$BODY" | python3 -c "import sys,json; print(json.load(sys.stdin)['token'])")

split_response "$(http GET /users/me "$JOE_TOKEN")"
assert_status "old token rejected (401)" "401" "$STATUS" "$BODY"

split_response "$(http GET /users/me "$JOE_NEW_TOKEN")"
assert_status "new token works" "200" "$STATUS" "$BODY"

JOE_TOKEN="$JOE_NEW_TOKEN"

# ================================================================
# 6. APP CATALOG
# ================================================================
echo ""
echo "$(bold '--- 6. App Catalog ---')"

split_response "$(http GET /apps "$ADMIN_TOKEN")"
assert_status "list apps" "200" "$STATUS" "$BODY"
assert_contains "rubix-developer in catalog" "rubix-developer" "$BODY"
assert_contains "nube-marketing in catalog" "nube-marketing" "$BODY"

split_response "$(http GET /apps/rubix-developer "$ADMIN_TOKEN")"
assert_status "get rubix-developer detail" "200" "$STATUS" "$BODY"
assert_contains "has OpenAPI" "hasOpenAPI" "$BODY"
assert_contains "has JS tools" "hasTools" "$BODY"
assert_contains "debug_network prompt" "debug_network" "$BODY"

split_response "$(http GET /apps/nonexistent "$ADMIN_TOKEN")"
assert_status "unknown app returns 404" "404" "$STATUS" "$BODY"

# ================================================================
# 7. APP INSTALL
# ================================================================
echo ""
echo "$(bold '--- 7. App Install ---')"

split_response "$(http POST /apps/rubix-developer/install "$ADMIN_TOKEN" \
  "{\"settings\":{\"rubix_host\":\"$FAKE\",\"rubix_token\":\"test-secret\"}}")"
assert_status "admin installs rubix-developer" "201" "$STATUS" "$BODY"
RUBIX_INSTALL_ID=$(echo "$BODY" | python3 -c "import sys,json; print(json.load(sys.stdin)['id'])")

split_response "$(http POST /apps/nube-marketing/install "$ADMIN_TOKEN" '{}')"
assert_status "admin installs nube-marketing" "201" "$STATUS" "$BODY"
MARKETING_INSTALL_ID=$(echo "$BODY" | python3 -c "import sys,json; print(json.load(sys.stdin)['id'])")

split_response "$(http POST /apps/rubix-developer/install "$ADMIN_TOKEN" '{}')"
assert_status "duplicate install rejected (409)" "409" "$STATUS" "$BODY"

split_response "$(http GET /app-installs "$ADMIN_TOKEN")"
assert_status "list admin installs" "200" "$STATUS" "$BODY"
assert_count  "admin has 2 installs" "2" "$BODY"

# Joe installs marketing only.
split_response "$(http POST /apps/nube-marketing/install "$JOE_TOKEN" '{}')"
assert_status "joe installs nube-marketing" "201" "$STATUS" "$BODY"

# ================================================================
# 8. MCP — PER-USER TOOL SCOPING
# ================================================================
echo ""
echo "$(bold '--- 8. MCP Per-User Scoping ---')"

# Admin session — should see rubix-developer tools + marketing prompts.
ADMIN_SESSION=$(mcp_init "$ADMIN_TOKEN")
TOTAL=$((TOTAL + 1))
if [ -n "$ADMIN_SESSION" ]; then
  PASS=$((PASS + 1))
  printf "  $(green PASS)  admin MCP session initialized\n"
else
  FAIL=$((FAIL + 1))
  printf "  $(red FAIL)  admin MCP session init failed\n"
fi

# List admin tools.
TOOLS_RESP=$(mcp_call "$ADMIN_TOKEN" "$ADMIN_SESSION" "tools/list" '{}')
TOOL_COUNT=$(echo "$TOOLS_RESP" | python3 -c "import sys,json; print(len(json.load(sys.stdin)['result']['tools']))" 2>/dev/null || echo "0")
TOTAL=$((TOTAL + 1))
if [ "$TOOL_COUNT" -ge 9 ]; then
  PASS=$((PASS + 1))
  printf "  $(green PASS)  admin has $TOOL_COUNT tools (OpenAPI + JS)\n"
else
  FAIL=$((FAIL + 1))
  printf "  $(red FAIL)  admin tool count: expected >=9, got $TOOL_COUNT\n"
fi

# Check specific tools exist.
assert_contains "has JS tool device_summary" "rubix-developer.device_summary" "$TOOLS_RESP"
assert_contains "has JS tool restart_device" "rubix-developer.restart_device" "$TOOLS_RESP"
assert_contains "has OpenAPI tool listDevices" "rubix-developer.listDevices" "$TOOLS_RESP"

# List admin prompts.
PROMPTS_RESP=$(mcp_call "$ADMIN_TOKEN" "$ADMIN_SESSION" "prompts/list" '{}')
PROMPT_COUNT=$(echo "$PROMPTS_RESP" | python3 -c "import sys,json; print(len(json.load(sys.stdin)['result']['prompts']))" 2>/dev/null || echo "0")
TOTAL=$((TOTAL + 1))
if [ "$PROMPT_COUNT" -ge 3 ]; then
  PASS=$((PASS + 1))
  printf "  $(green PASS)  admin has $PROMPT_COUNT prompts\n"
else
  FAIL=$((FAIL + 1))
  printf "  $(red FAIL)  admin prompt count: expected >=3, got $PROMPT_COUNT\n"
fi

# Joe session — should see only marketing prompts, no tools.
JOE_SESSION=$(mcp_init "$JOE_TOKEN")
JOE_TOOLS=$(mcp_call "$JOE_TOKEN" "$JOE_SESSION" "tools/list" '{}')
JOE_TOOL_COUNT=$(echo "$JOE_TOOLS" | python3 -c "import sys,json; print(len(json.load(sys.stdin)['result']['tools']))" 2>/dev/null || echo "0")
TOTAL=$((TOTAL + 1))
if [ "$JOE_TOOL_COUNT" = "0" ]; then
  PASS=$((PASS + 1))
  printf "  $(green PASS)  joe has 0 tools (marketing is prompt-only)\n"
else
  FAIL=$((FAIL + 1))
  printf "  $(red FAIL)  joe tool count: expected 0, got $JOE_TOOL_COUNT\n"
fi

JOE_PROMPTS=$(mcp_call "$JOE_TOKEN" "$JOE_SESSION" "prompts/list" '{}')
JOE_PROMPT_COUNT=$(echo "$JOE_PROMPTS" | python3 -c "import sys,json; print(len(json.load(sys.stdin)['result']['prompts']))" 2>/dev/null || echo "0")
TOTAL=$((TOTAL + 1))
if [ "$JOE_PROMPT_COUNT" = "2" ]; then
  PASS=$((PASS + 1))
  printf "  $(green PASS)  joe has 2 prompts (marketing only)\n"
else
  FAIL=$((FAIL + 1))
  printf "  $(red FAIL)  joe prompt count: expected 2, got $JOE_PROMPT_COUNT\n"
fi

# ================================================================
# 9. MCP — JS TOOL EXECUTION
# ================================================================
echo ""
echo "$(bold '--- 9. JS Tool Execution ---')"

# Call device_summary — should hit fakeserver and return device counts.
SUMMARY_RESP=$(mcp_call "$ADMIN_TOKEN" "$ADMIN_SESSION" "tools/call" \
  '{"name":"rubix-developer.device_summary","arguments":{}}')
SUMMARY_TEXT=$(echo "$SUMMARY_RESP" | python3 -c "import sys,json; print(json.load(sys.stdin)['result']['content'][0]['text'])" 2>/dev/null || echo "")
TOTAL=$((TOTAL + 1))
if echo "$SUMMARY_TEXT" | python3 -c "import sys,json; d=json.load(sys.stdin); assert d['total']==3" 2>/dev/null; then
  PASS=$((PASS + 1))
  printf "  $(green PASS)  device_summary returned 3 devices\n"
  TOTAL=$((TOTAL + 1))
  ONLINE=$(echo "$SUMMARY_TEXT" | python3 -c "import sys,json; print(int(json.load(sys.stdin)['online']))")
  OFFLINE=$(echo "$SUMMARY_TEXT" | python3 -c "import sys,json; print(int(json.load(sys.stdin)['offline']))")
  if [ "$ONLINE" = "2" ] && [ "$OFFLINE" = "1" ]; then
    PASS=$((PASS + 1))
    printf "  $(green PASS)  device_summary: online=$ONLINE, offline=$OFFLINE\n"
  else
    FAIL=$((FAIL + 1))
    printf "  $(red FAIL)  device_summary: expected online=2,offline=1, got online=$ONLINE,offline=$OFFLINE\n"
  fi
else
  FAIL=$((FAIL + 1))
  printf "  $(red FAIL)  device_summary failed: %s\n" "$SUMMARY_TEXT"
fi

# Call restart_device.
RESTART_RESP=$(mcp_call "$ADMIN_TOKEN" "$ADMIN_SESSION" "tools/call" \
  '{"name":"rubix-developer.restart_device","arguments":{"id":"dev-001"}}')
RESTART_TEXT=$(echo "$RESTART_RESP" | python3 -c "import sys,json; print(json.load(sys.stdin)['result']['content'][0]['text'])" 2>/dev/null || echo "")
assert_contains "restart_device returns success" "restarted" "$RESTART_TEXT"

# Call restart on nonexistent device.
BAD_RESP=$(mcp_call "$ADMIN_TOKEN" "$ADMIN_SESSION" "tools/call" \
  '{"name":"rubix-developer.restart_device","arguments":{"id":"dev-999"}}')
BAD_TEXT=$(echo "$BAD_RESP" | python3 -c "import sys,json; print(json.load(sys.stdin)['result']['content'][0]['text'])" 2>/dev/null || echo "")
assert_contains "restart nonexistent device returns error" "not found" "$BAD_TEXT"

# ================================================================
# 10. APP MANAGEMENT
# ================================================================
echo ""
echo "$(bold '--- 10. App Management ---')"

# Disable an install.
split_response "$(http PATCH "/app-installs/$MARKETING_INSTALL_ID" "$ADMIN_TOKEN" '{"enabled":false}')"
assert_status "disable marketing install" "200" "$STATUS" "$BODY"
assert_json   "install is disabled" "enabled" "False" "$BODY"

# Re-enable.
split_response "$(http PATCH "/app-installs/$MARKETING_INSTALL_ID" "$ADMIN_TOKEN" '{"enabled":true}')"
assert_status "re-enable marketing install" "200" "$STATUS" "$BODY"

# Uninstall.
split_response "$(http DELETE "/app-installs/$MARKETING_INSTALL_ID" "$ADMIN_TOKEN")"
assert_status "uninstall marketing" "200" "$STATUS" "$BODY"

split_response "$(http GET /app-installs "$ADMIN_TOKEN")"
assert_count  "admin has 1 install after uninstall" "1" "$BODY"

# ================================================================
# 11. APP RELOAD
# ================================================================
echo ""
echo "$(bold '--- 11. App Reload ---')"

split_response "$(http POST /admin/reload-apps "$ADMIN_TOKEN")"
assert_status "admin reloads apps" "200" "$STATUS" "$BODY"

split_response "$(http POST /admin/reload-apps "$JOE_TOKEN")"
assert_status "joe blocked from reload (403)" "403" "$STATUS" "$BODY"

# ================================================================
# SUMMARY
# ================================================================
echo ""
echo "$(bold '=== Results ===')"
echo ""
if [ "$FAIL" = "0" ]; then
  echo "  $(green "ALL $TOTAL TESTS PASSED")"
else
  echo "  $(green "$PASS passed"), $(red "$FAIL failed") out of $TOTAL"
fi
echo ""

exit "$FAIL"
