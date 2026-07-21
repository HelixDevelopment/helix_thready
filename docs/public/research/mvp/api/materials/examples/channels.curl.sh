#!/usr/bin/env bash
# =============================================================================
#  Helix Thready — CHANNELS (+ messengers) request collection (curl)
#  Tags: channels, messengers  |  Source of truth: ../../openapi.yaml
#  Maturity: registerChannel=foundation, syncChannel=build_new — those may 503
#  `unavailable` until the Herald MTProto reader / Max adapter gaps close.
# =============================================================================
set -uo pipefail
: "${THREADY_BASE:=https://dev.thready.hxd3v.com/v1}"
AUTH=(-H "Authorization: Bearer $THREADY_TOKEN")

# --- GET /messengers — supported platforms + capabilities/maturity -----------
curl -sS "$THREADY_BASE/messengers" "${AUTH[@]}"

# --- GET /channels — list registered channels (account_id honoured for root) -
curl -sS "$THREADY_BASE/channels?limit=50" "${AUTH[@]}"

# --- POST /channels — register a channel/group (telegram|max) ----------------
# x-thready-maturity: foundation. 422 if external_ref is not resolvable.
curl -sS -X POST "$THREADY_BASE/channels" "${AUTH[@]}" \
  -H "Idempotency-Key: $THREADY_IDEMPOTENCY_KEY" \
  -H 'Content-Type: application/json' \
  -d '{"messenger":"telegram","external_ref":"https://t.me/+inviteHash","poll_interval_seconds":300}'

# --- GET /channels/{channelId} -----------------------------------------------
curl -sS "$THREADY_BASE/channels/$THREADY_CHANNEL_ID" "${AUTH[@]}"

# --- POST /channels/{channelId}/sync — incremental poll (async 202) ----------
curl -sS -X POST "$THREADY_BASE/channels/$THREADY_CHANNEL_ID/sync" "${AUTH[@]}" \
  -H "Idempotency-Key: $THREADY_IDEMPOTENCY_KEY" \
  -H 'Content-Type: application/json' \
  -d '{"backfill":false}' -i

# --- POST /channels/{channelId}/sync — full backfill since a date ------------
# May 503 (unavailable) on Telegram/Max until the reader/adapter gap closes.
curl -sS -X POST "$THREADY_BASE/channels/$THREADY_CHANNEL_ID/sync" "${AUTH[@]}" \
  -H 'Content-Type: application/json' \
  -d '{"backfill":true,"since":"2026-01-01T00:00:00Z"}' -i

# --- DELETE /channels/{channelId} — stop reading (204) -----------------------
curl -sS -X DELETE "$THREADY_BASE/channels/$THREADY_CHANNEL_ID" "${AUTH[@]}" -i
