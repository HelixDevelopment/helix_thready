#!/usr/bin/env bash
# =============================================================================
#  Helix Thready — AUTH request collection (curl)
#  Tag: auth  |  Source of truth: ../../openapi.yaml (tag: auth)
#  Run:  source .env  (or env.example)  then  bash auth.curl.sh   [or copy lines]
#  NOTE: login/refresh/jwks/oauth2-authorize are unauthenticated (security: []).
# =============================================================================
set -uo pipefail
: "${THREADY_BASE:=https://dev.thready.hxd3v.com/v1}"

# --- POST /auth/login — standard user (TOTP optional) ------------------------
curl -sS -X POST "$THREADY_BASE/auth/login" \
  -H 'Content-Type: application/json' \
  -d '{"email":"'"$THREADY_EMAIL"'","password":"'"$THREADY_PASSWORD"'"}'

# --- POST /auth/login — admin tier (TOTP mandatory) --------------------------
curl -sS -X POST "$THREADY_BASE/auth/login" \
  -H 'Content-Type: application/json' \
  -d '{"email":"admin@t1.example","password":"correct-horse-battery","totp":"'"$THREADY_TOTP"'"}'

# --- POST /auth/refresh — rotate the refresh token (old one is revoked) ------
curl -sS -X POST "$THREADY_BASE/auth/refresh" \
  -H 'Content-Type: application/json' \
  -d '{"refresh_token":"'"$THREADY_REFRESH"'"}'

# --- GET /auth/me — the authenticated principal + effective scopes -----------
curl -sS "$THREADY_BASE/auth/me" \
  -H "Authorization: Bearer $THREADY_TOKEN"

# --- POST /auth/logout — revoke current access + refresh (204) ---------------
curl -sS -X POST "$THREADY_BASE/auth/logout" \
  -H "Authorization: Bearer $THREADY_TOKEN" -i

# --- POST /auth/mfa/totp/enroll — begin TOTP enrolment -----------------------
curl -sS -X POST "$THREADY_BASE/auth/mfa/totp/enroll" \
  -H "Authorization: Bearer $THREADY_TOKEN"

# --- POST /auth/mfa/totp/verify — confirm enrolment with a code (204) --------
curl -sS -X POST "$THREADY_BASE/auth/mfa/totp/verify" \
  -H "Authorization: Bearer $THREADY_TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"code":"'"$THREADY_TOTP"'"}' -i

# --- GET /auth/oauth2/authorize — start an external-provider link (302) ------
# Follow the redirect (-L) in a browser; providers: dropbox|gdrive|onedrive.
curl -sS -i "$THREADY_BASE/auth/oauth2/authorize?provider=dropbox&redirect_uri=https%3A%2F%2Fapp.example%2Foauth%2Fcb" \
  -H "Authorization: Bearer $THREADY_TOKEN"

# --- GET /api-keys — list the caller's API keys (masked) ---------------------
curl -sS "$THREADY_BASE/api-keys?limit=50" \
  -H "Authorization: Bearer $THREADY_TOKEN"

# --- POST /api-keys — mint a scoped API key (secret returned ONCE) -----------
curl -sS -X POST "$THREADY_BASE/api-keys" \
  -H "Authorization: Bearer $THREADY_TOKEN" \
  -H "Idempotency-Key: $THREADY_IDEMPOTENCY_KEY" \
  -H 'Content-Type: application/json' \
  -d '{"name":"ci-automation","scopes":["posts:read","search:read"]}'

# --- DELETE /api-keys/{keyId} — revoke an API key (204) ----------------------
curl -sS -X DELETE "$THREADY_BASE/api-keys/$THREADY_KEY_ID" \
  -H "Authorization: Bearer $THREADY_TOKEN" -i

# --- GET /.well-known/jwks.json — public JWKS (unauthenticated) --------------
curl -sS "$THREADY_BASE/.well-known/jwks.json"
