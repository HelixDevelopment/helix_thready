#!/usr/bin/env bash
# =============================================================================
# Helix Thready rest_gateway — REAL deployment smoke test.
#
# Builds the cmd/gateway binary, starts it on a free port, and drives the live
# /v1 HTTP surface with real curls, asserting the status code + body of each.
# Every printed result comes from an actual HTTP round-trip against the running
# server — nothing is stubbed or pre-baked. Exits nonzero if any check fails.
#
# Constitution: proves the built binary actually serves the API (the host leg of
# the rootless-Podman deployment mandate, §11.4.161). The container leg lives in
# podman_smoke.sh / Containerfile.
# =============================================================================
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
GATEWAY_SRC="${SCRIPT_DIR}/../rest_gateway"
BIN="$(mktemp -t thready-gateway.XXXXXX)"
WORK="$(mktemp -d -t thready-smoke.XXXXXX)"

PASS=0
FAIL=0
SERVER_PID=""

cleanup() {
  if [[ -n "${SERVER_PID}" ]] && kill -0 "${SERVER_PID}" 2>/dev/null; then
    kill "${SERVER_PID}" 2>/dev/null || true
    wait "${SERVER_PID}" 2>/dev/null || true
  fi
  rm -f "${BIN}" 2>/dev/null || true
  rm -rf "${WORK}" 2>/dev/null || true
}
trap cleanup EXIT

hr() { printf -- '-------------------------------------------------------------------------------\n'; }

# check <label> <condition-description> <actual> <expected>  (pass iff actual==expected)
check() {
  local label="$1" actual="$2" expected="$3"
  if [[ "${actual}" == "${expected}" ]]; then
    printf 'PASS  %-42s  got=%s (want=%s)\n' "${label}" "${actual}" "${expected}"
    PASS=$((PASS + 1))
  else
    printf 'FAIL  %-42s  got=%s (want=%s)\n' "${label}" "${actual}" "${expected}"
    FAIL=$((FAIL + 1))
  fi
}

# check_contains <label> <haystack> <needle>
check_contains() {
  local label="$1" haystack="$2" needle="$3"
  if [[ "${haystack}" == *"${needle}"* ]]; then
    printf 'PASS  %-42s  body contains %s\n' "${label}" "${needle}"
    PASS=$((PASS + 1))
  else
    printf 'FAIL  %-42s  body missing  %s\n' "${label}" "${needle}"
    FAIL=$((FAIL + 1))
  fi
}

echo "== Helix Thready rest_gateway — deployment smoke test =="
echo "host:        $(uname -srm)"
echo "go:          $(go version 2>/dev/null || echo 'go not found')"
echo "date (UTC):  $(date -u +%Y-%m-%dT%H:%M:%SZ)"
hr

# ---- 1. Build ---------------------------------------------------------------
echo "[build] GOWORK=off go build -o ${BIN} ./cmd/gateway  (in ${GATEWAY_SRC})"
( cd "${GATEWAY_SRC}" && GOWORK=off go build -o "${BIN}" ./cmd/gateway )
echo "[build] OK -> ${BIN}"
ls -l "${BIN}"
hr

# ---- 2. Pick a free TCP port ------------------------------------------------
PORT="$(python3 - <<'PY'
import socket
s = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
s.bind(("127.0.0.1", 0))
print(s.getsockname()[1])
s.close()
PY
)"
BASE="http://127.0.0.1:${PORT}"
echo "[run] chosen free port: ${PORT}"

# ---- 3. Start the server in the background ----------------------------------
GATEWAY_ADDR=":${PORT}" "${BIN}" >"${WORK}/server.log" 2>&1 &
SERVER_PID=$!
echo "[run] started gateway pid=${SERVER_PID} on ${BASE}"

# ---- 4. Poll /v1/health until ready (timeout ~15s) --------------------------
READY=0
for i in $(seq 1 75); do
  if ! kill -0 "${SERVER_PID}" 2>/dev/null; then
    echo "[run] FATAL: server process exited during startup; log follows:"
    cat "${WORK}/server.log"
    exit 1
  fi
  code="$(curl -s -o /dev/null -w '%{http_code}' "${BASE}/v1/health" 2>/dev/null || echo 000)"
  if [[ "${code}" == "200" ]]; then
    READY=1
    echo "[run] health ready after $((i * 200))ms (attempt ${i})"
    break
  fi
  sleep 0.2
done
if [[ "${READY}" != "1" ]]; then
  echo "[run] FATAL: server never became ready; log follows:"
  cat "${WORK}/server.log"
  exit 1
fi
hr

# ---- 5. Real HTTP checks ----------------------------------------------------

echo "### CHECK 1 — GET /v1/health  (public; expect 200 + JSON)"
code="$(curl -s -o "${WORK}/health.json" -w '%{http_code}' "${BASE}/v1/health")"
echo "HTTP ${code}  <-  GET ${BASE}/v1/health"
echo "body: $(cat "${WORK}/health.json")"
check          "health.status_code"        "${code}" "200"
check          "health.body_is_json"       "$(jq -r 'type' "${WORK}/health.json" 2>/dev/null || echo notjson)" "object"
check_contains "health.body_status_ok"     "$(cat "${WORK}/health.json")" '"status":"ok"'
hr

echo "### CHECK 2 — GET /v1/channels WITHOUT auth  (protected; expect 401)"
code="$(curl -s -o "${WORK}/noauth.json" -w '%{http_code}' "${BASE}/v1/channels")"
echo "HTTP ${code}  <-  GET ${BASE}/v1/channels   (no Authorization header)"
echo "body: $(cat "${WORK}/noauth.json")"
check          "channels_noauth.status_code" "${code}" "401"
check_contains "channels_noauth.err_code"    "$(cat "${WORK}/noauth.json")" '"code":"unauthenticated"'
hr

echo "### CHECK 3 — POST /v1/auth/login  (seeded user; expect 200 + token)"
LOGIN_BODY='{"email":"user@thready.test","password":"userpassword-123"}'
echo "request body: ${LOGIN_BODY}   (seeded creds from rest_gateway/services.go: SeedUserEmail / SeedUserPass, no TOTP)"
code="$(curl -s -o "${WORK}/login.json" -w '%{http_code}' \
  -H 'Content-Type: application/json' \
  -X POST --data "${LOGIN_BODY}" "${BASE}/v1/auth/login")"
echo "HTTP ${code}  <-  POST ${BASE}/v1/auth/login"
TOKEN="$(jq -r '.access_token // empty' "${WORK}/login.json" 2>/dev/null || true)"
TOKEN_TYPE="$(jq -r '.token_type // empty' "${WORK}/login.json" 2>/dev/null || true)"
# Show a redacted view of the token (first 24 chars) — it is a real signed JWT.
echo "token_type: ${TOKEN_TYPE}   access_token[0:24]: ${TOKEN:0:24}...   (len=${#TOKEN})"
check "login.status_code"        "${code}" "200"
check "login.token_type_bearer"  "${TOKEN_TYPE}" "Bearer"
if [[ -n "${TOKEN}" ]]; then
  echo "PASS  login.access_token_present            token len=${#TOKEN}"
  PASS=$((PASS + 1))
else
  echo "FAIL  login.access_token_present            token is EMPTY"
  FAIL=$((FAIL + 1))
fi
# A real HS256 JWT is exactly three dot-separated base64url segments.
DOTS="$(awk -F. '{print NF-1}' <<<"${TOKEN}")"
check "login.token_is_jwt_3parts" "${DOTS}" "2"
hr

echo "### CHECK 4 — GET /v1/channels WITH Bearer token  (expect 200)"
code="$(curl -s -o "${WORK}/channels.json" -w '%{http_code}' \
  -H "Authorization: Bearer ${TOKEN}" "${BASE}/v1/channels")"
echo "HTTP ${code}  <-  GET ${BASE}/v1/channels   (Authorization: Bearer <token>)"
echo "body: $(cat "${WORK}/channels.json")"
check          "channels_auth.status_code" "${code}" "200"
check_contains "channels_auth.has_data"    "$(cat "${WORK}/channels.json")" '"data"'
check_contains "channels_auth.seed_channel" "$(cat "${WORK}/channels.json")" '"chan-1"'
hr

# ---- 6. Summary -------------------------------------------------------------
echo "== server access log (structured JSON, proves the requests hit the server) =="
cat "${WORK}/server.log"
hr
echo "== SUMMARY =="
echo "checks passed: ${PASS}"
echo "checks failed: ${FAIL}"
if [[ "${FAIL}" -ne 0 ]]; then
  echo "VERDICT: FAIL"
  exit 1
fi
echo "VERDICT: PASS"
exit 0
