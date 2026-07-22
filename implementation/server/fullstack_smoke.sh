#!/usr/bin/env bash
#
# fullstack_smoke.sh — real, offline full-stack smoke for Helix Thready.
#
# Proves the WHOLE stack composes for real, with zero mocking:
#
#   real `thready` CLI binary  --HTTP-->  real `thready-server` binary
#         (over sdk_go)                     (real /v1 gateway over the REAL
#                                            domain modules: user_service,
#                                            semantic_search, skill_dispatch,
#                                            event_bus_service)
#
# It builds BOTH binaries, starts the server on a FREE loopback port, waits for
# /v1/health, then drives it with the real CLI: login (password-only seed user,
# NO TOTP) -> channels list -> skills -> search "vector database". It asserts the
# real output (a skills list and a real search hit appear), shuts the server down
# cleanly (trap, on success AND error), prints a PASS/FAIL summary, and exits
# non-zero on any failure.
#
# Stdlib-only Go; the script uses only standard POSIX/Unix tooling (bash, curl,
# python3 for a free port with an `ss` fallback). No jq dependency (the login
# token is parsed from the CLI's --json output with sed).
#
# Auth note: the standard seed user (user@thready.test) is provisioned with NO
# TOTP secret, so the server's real-wired auth adapter requires email+password
# ONLY (MFAEnabled=false) — avoiding the RFC 6238 time-based-code problem the
# admin/root tiers have. Its scopes (posts:read, search:read, skills:read, …)
# cover every command this smoke runs, so all three are authorized (no 403).
#
# The signing secret is runtime-loaded and the server fails closed without it, so
# we export a throwaway THREADY_JWT_SECRET before boot.

set -uo pipefail

# ---------------------------------------------------------------------------
# Paths & scratch
# ---------------------------------------------------------------------------
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
IMPL_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
BUILD_DIR="$(mktemp -d)"
SERVER_LOG="$BUILD_DIR/server.log"
SRV_PID=""

PASS=0
FAIL=0

pass() { PASS=$((PASS + 1)); printf 'PASS  %s\n' "$1"; }
fail() { FAIL=$((FAIL + 1)); printf 'FAIL  %s\n' "$1"; }

cleanup() {
  if [ -n "$SRV_PID" ] && kill -0 "$SRV_PID" 2>/dev/null; then
    kill "$SRV_PID" 2>/dev/null
    wait "$SRV_PID" 2>/dev/null
  fi
  rm -rf "$BUILD_DIR"
}
trap cleanup EXIT INT TERM

section() { printf '\n=== %s ===\n' "$1"; }

pick_free_port() {
  if command -v python3 >/dev/null 2>&1; then
    python3 -c 'import socket; s=socket.socket(); s.bind(("127.0.0.1",0)); print(s.getsockname()[1]); s.close()' && return
  fi
  # Fallback: scan a small high range for a port with no listener.
  local p
  for p in $(seq 21000 21099); do
    if ! ss -Htln 2>/dev/null | awk '{print $4}' | grep -q ":${p}\$"; then
      echo "$p"; return
    fi
  done
  echo 0
}

# ---------------------------------------------------------------------------
# 1. Build both binaries
# ---------------------------------------------------------------------------
section "build binaries"

# The server is a go.work member -> build in workspace mode (GOPROXY=off keeps it
# fully offline; all deps are local workspace/replace modules).
if ( cd "$IMPL_DIR/server" && GOPROXY=off go build -o "$BUILD_DIR/thready-server" ./cmd/thready-server ); then
  pass "build thready-server"
else
  fail "build thready-server"
fi

# The cli + sdk_go are NOT workspace members -> build with GOWORK=off so the
# cli module's own `replace digital.vasic.threadysdk => ../sdk_go` resolves.
if ( cd "$IMPL_DIR/cli" && GOWORK=off GOPROXY=off go build -o "$BUILD_DIR/thready" ./cmd/thready ); then
  pass "build thready CLI"
else
  fail "build thready CLI"
fi

if [ "$FAIL" -ne 0 ]; then
  section "SUMMARY"
  printf 'build failed — aborting. PASS=%d FAIL=%d\n' "$PASS" "$FAIL"
  exit 1
fi

SERVER_BIN="$BUILD_DIR/thready-server"
CLI_BIN="$BUILD_DIR/thready"

# ---------------------------------------------------------------------------
# 2. Start the server on a free port; wait for /v1/health = 200
# ---------------------------------------------------------------------------
section "start thready-server"

PORT="$(pick_free_port)"
if [ -z "$PORT" ] || [ "$PORT" = "0" ]; then
  fail "allocate a free port"
  exit 1
fi
printf 'chosen free port: %s\n' "$PORT"

# Runtime-loaded, fail-closed signing secret (throwaway, per-run).
RAND_HEX="$(head -c 16 /dev/urandom | od -An -tx1 | tr -d ' \n')"
export THREADY_JWT_SECRET="smoke-throwaway-${RAND_HEX}"

PORT="$PORT" "$SERVER_BIN" >"$SERVER_LOG" 2>&1 &
SRV_PID=$!
printf 'server pid: %s\n' "$SRV_PID"

BASE="http://127.0.0.1:${PORT}"
HEALTHY=0
for _ in $(seq 1 100); do
  if ! kill -0 "$SRV_PID" 2>/dev/null; then
    break # server died during startup
  fi
  code="$(curl -s -o /dev/null -w '%{http_code}' "${BASE}/v1/health" 2>/dev/null || true)"
  if [ "$code" = "200" ]; then HEALTHY=1; break; fi
  sleep 0.1
done

if [ "$HEALTHY" -eq 1 ]; then
  pass "GET /v1/health -> 200"
else
  fail "server did not become healthy"
  section "server log"; cat "$SERVER_LOG"
  section "SUMMARY"
  printf 'PASS=%d FAIL=%d\n' "$PASS" "$FAIL"
  exit 1
fi

# ---------------------------------------------------------------------------
# 3. Drive the server with the REAL CLI over loopback http
# ---------------------------------------------------------------------------
export THREADY_BASE_URL="$BASE"

SEED_EMAIL="user@thready.test"
SEED_PASS="userpassword-123"

# --- login (password-only; token passed via env, never argv) ---
section "CLI: login (password-only seed user, no TOTP)"
LOGIN_OUT="$(THREADY_PASSWORD="$SEED_PASS" "$CLI_BIN" login --email "$SEED_EMAIL" --json 2>&1)"
LOGIN_RC=$?
printf '%s\n' "$LOGIN_OUT"
# Parse the access token from the pretty-printed --json output (no jq needed).
TOKEN="$(printf '%s' "$LOGIN_OUT" | sed -n 's/.*"access_token"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p')"
if [ "$LOGIN_RC" -eq 0 ] && [ -n "$TOKEN" ]; then
  pass "login obtained an access token (len=${#TOKEN})"
else
  fail "login did not return a token (rc=$LOGIN_RC)"
fi

# Subsequent commands authenticate with the obtained bearer token via env.
export THREADY_TOKEN="$TOKEN"

# --- channels list (standard user is authorized: role user + posts:read) ---
section "CLI: channels list"
CH_OUT="$("$CLI_BIN" channels list 2>&1)"; CH_RC=$?
printf '%s\n' "$CH_OUT"
if [ "$CH_RC" -eq 0 ] && printf '%s' "$CH_OUT" | grep -q "general"; then
  pass "channels list returned the seed channel 'general'"
else
  fail "channels list (rc=$CH_RC) did not show the seed channel"
fi

# --- skills (assert the skills list actually appears) ---
section "CLI: skills"
SK_OUT="$("$CLI_BIN" skills 2>&1)"; SK_RC=$?
printf '%s\n' "$SK_OUT"
if [ "$SK_RC" -eq 0 ] \
  && printf '%s' "$SK_OUT" | grep -q "video.download" \
  && printf '%s' "$SK_OUT" | grep -q "thread.reply"; then
  pass "skills list appears (video.download … thread.reply)"
else
  fail "skills list (rc=$SK_RC) missing expected entries"
fi

# --- search "vector database" (assert a real search hit appears) ---
section 'CLI: search "vector database"'
SE_OUT="$("$CLI_BIN" search "vector database" 2>&1)"; SE_RC=$?
printf '%s\n' "$SE_OUT"
if [ "$SE_RC" -eq 0 ] && printf '%s' "$SE_OUT" | grep -q "vectordb.md"; then
  pass "search returned a real hit (top: vectordb.md)"
else
  fail "search (rc=$SE_RC) did not return the expected hit"
fi

# ---------------------------------------------------------------------------
# 4. Clean shutdown (kill + wait); then summary
# ---------------------------------------------------------------------------
section "shutdown"
if [ -n "$SRV_PID" ] && kill -0 "$SRV_PID" 2>/dev/null; then
  kill "$SRV_PID" 2>/dev/null
  wait "$SRV_PID" 2>/dev/null
  SHUT_RC=$?
  SRV_PID=""
  printf 'server stopped (wait rc=%s)\n' "$SHUT_RC"
  pass "server shut down cleanly"
else
  fail "server was not running at shutdown (crashed?)"
  section "server log"; cat "$SERVER_LOG"
fi

section "server log"
cat "$SERVER_LOG"

section "SUMMARY"
printf 'PASS=%d  FAIL=%d\n' "$PASS" "$FAIL"
if [ "$FAIL" -eq 0 ]; then
  printf 'RESULT: PASS — full stack composed end-to-end for real.\n'
  exit 0
else
  printf 'RESULT: FAIL — see failures above.\n'
  exit 1
fi
