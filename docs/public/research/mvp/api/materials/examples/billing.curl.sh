#!/usr/bin/env bash
# =============================================================================
#  Helix Thready — BILLING request collection (curl)
#  Tag: billing  |  Source of truth: ../../openapi.yaml
#  x-thready-maturity: design — schema fixed (subscription + metered from day
#  one, Q11); payment-provider integration lands in the deployment pack.
#  Subscription/usage require scope billing:read + account_admin.
# =============================================================================
set -uo pipefail
: "${THREADY_BASE:=https://dev.thready.hxd3v.com/v1}"
AUTH=(-H "Authorization: Bearer $THREADY_TOKEN")

# --- GET /plans — list subscription plans ------------------------------------
curl -sS "$THREADY_BASE/plans" "${AUTH[@]}"

# --- GET /accounts/{accountId}/subscription — current subscription -----------
curl -sS "$THREADY_BASE/accounts/$THREADY_ACCOUNT_ID/subscription" "${AUTH[@]}"

# --- PUT /accounts/{accountId}/subscription — change plan --------------------
curl -sS -X PUT "$THREADY_BASE/accounts/$THREADY_ACCOUNT_ID/subscription" "${AUTH[@]}" \
  -H 'Content-Type: application/json' \
  -d '{"plan_id":"pro-monthly"}'

# --- GET /accounts/{accountId}/usage — metered usage (current period) --------
curl -sS "$THREADY_BASE/accounts/$THREADY_ACCOUNT_ID/usage" "${AUTH[@]}"

# --- GET /accounts/{accountId}/usage — a named period ------------------------
curl -sS "$THREADY_BASE/accounts/$THREADY_ACCOUNT_ID/usage?period_start=2026-07-01T00:00:00Z" "${AUTH[@]}"
