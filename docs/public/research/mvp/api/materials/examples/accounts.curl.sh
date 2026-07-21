#!/usr/bin/env bash
# =============================================================================
#  Helix Thready — ACCOUNTS + USERS request collection (curl)
#  Tags: accounts, users  |  Source of truth: ../../openapi.yaml
#  Three-tier RBAC: root sees all; account_admin administers its account; user
#  reads self. x-required-roles per operation is enforced server-side, not here.
# =============================================================================
set -uo pipefail
: "${THREADY_BASE:=https://dev.thready.hxd3v.com/v1}"
AUTH=(-H "Authorization: Bearer $THREADY_TOKEN")

# --- GET /accounts — list (root=all, others=own memberships) -----------------
curl -sS "$THREADY_BASE/accounts?limit=50" "${AUTH[@]}"

# --- POST /accounts — create (creator becomes account_admin) -----------------
curl -sS -X POST "$THREADY_BASE/accounts" "${AUTH[@]}" \
  -H 'Content-Type: application/json' \
  -d '{"name":"Tenant One","slug":"tenant-one"}'

# --- GET /accounts/{accountId} -----------------------------------------------
curl -sS "$THREADY_BASE/accounts/$THREADY_ACCOUNT_ID" "${AUTH[@]}"

# --- PATCH /accounts/{accountId} — settings (name, retention override) --------
curl -sS -X PATCH "$THREADY_BASE/accounts/$THREADY_ACCOUNT_ID" "${AUTH[@]}" \
  -H 'Content-Type: application/json' \
  -d '{"id":"'"$THREADY_ACCOUNT_ID"'","name":"Tenant One (renamed)","retention_days":365,"created_at":"2026-01-01T00:00:00Z"}'

# --- DELETE /accounts/{accountId} — root only (204) --------------------------
curl -sS -X DELETE "$THREADY_BASE/accounts/$THREADY_ACCOUNT_ID" "${AUTH[@]}" -i

# --- PUT /accounts/{accountId}/branding — white-label -------------------------
curl -sS -X PUT "$THREADY_BASE/accounts/$THREADY_ACCOUNT_ID/branding" "${AUTH[@]}" \
  -H 'Content-Type: application/json' \
  -d '{"primary_color":"#B6E376","slogan":"Made with love ♥ by Helix Development"}'

# --- GET /accounts/{accountId}/users — list users in an account --------------
curl -sS "$THREADY_BASE/accounts/$THREADY_ACCOUNT_ID/users?limit=50" "${AUTH[@]}"

# --- POST /accounts/{accountId}/users — invite a user ------------------------
curl -sS -X POST "$THREADY_BASE/accounts/$THREADY_ACCOUNT_ID/users" "${AUTH[@]}" \
  -H 'Content-Type: application/json' \
  -d '{"email":"newuser@t1.example","role":"user"}'

# --- GET /users/{userId} — self, or admin within the tenant ------------------
curl -sS "$THREADY_BASE/users/$THREADY_USER_ID" "${AUTH[@]}"

# --- PATCH /users/{userId} — update role/profile within the tenant -----------
curl -sS -X PATCH "$THREADY_BASE/users/$THREADY_USER_ID" "${AUTH[@]}" \
  -H 'Content-Type: application/json' \
  -d '{"id":"'"$THREADY_USER_ID"'","email":"newuser@t1.example","role":"account_admin","created_at":"2026-01-01T00:00:00Z"}'
