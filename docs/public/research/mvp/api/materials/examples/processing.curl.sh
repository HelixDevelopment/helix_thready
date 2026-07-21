#!/usr/bin/env bash
# =============================================================================
#  Helix Thready — PROCESSING request collection (curl)
#  Tag: processing  |  Source of truth: ../../openapi.yaml
#  triggerProcessing/triggerReprocessing/redownload are build_new — the dispatch
#  engine is BUILD-NEW atop helix_skills; single-claim => a lost race returns 409.
#  Inbound callbacks are HMAC-signed (X-Thready-Signature), NOT JWT.
# =============================================================================
set -uo pipefail
: "${THREADY_BASE:=https://dev.thready.hxd3v.com/v1}"
AUTH=(-H "Authorization: Bearer $THREADY_TOKEN")

# --- POST /posts/{postId}/process — trigger processing (idempotent 202) ------
# Single-claim: a concurrent claim that loses gets 409 conflict.
curl -sS -X POST "$THREADY_BASE/posts/$THREADY_POST_ID/process" "${AUTH[@]}" \
  -H "Idempotency-Key: $THREADY_IDEMPOTENCY_KEY" \
  -H 'Content-Type: application/json' \
  -d '{"force":false}' -i

# --- POST /posts/{postId}/process — override the auto-selected skills --------
curl -sS -X POST "$THREADY_BASE/posts/$THREADY_POST_ID/process" "${AUTH[@]}" \
  -H 'Content-Type: application/json' \
  -d '{"force":true,"skills":["video.download","research.deep"]}' -i

# --- POST /posts/{postId}/reprocess — force a fresh reprocess (admin) --------
curl -sS -X POST "$THREADY_BASE/posts/$THREADY_POST_ID/reprocess" "${AUTH[@]}" \
  -H "Idempotency-Key: $THREADY_IDEMPOTENCY_KEY" \
  -H 'Content-Type: application/json' \
  -d '{"force":true}' -i

# --- GET /posts/{postId}/processing — current state + progress ---------------
curl -sS "$THREADY_BASE/posts/$THREADY_POST_ID/processing" "${AUTH[@]}"

# --- GET /processing/jobs — list processing jobs -----------------------------
curl -sS "$THREADY_BASE/processing/jobs?limit=50" "${AUTH[@]}"

# --- POST /processing/callbacks/{provider} — inbound 3rd-party completion -----
# Authenticated by a per-provider HMAC-SHA256 over the RAW body (hmacAuth),
# NOT a JWT. Idempotent on job_id. providers: boba|metube|download_manager.
PROVIDER="metube"
BODY='{"job_id":"dl-9931","provider":"metube","state":"completed","progress":1.0,"result_asset_ref":"asset:5f2a","error":null}'
SIG=$(printf '%s' "$BODY" | openssl dgst -sha256 -hmac "$THREADY_HMAC_SECRET" -binary | xxd -p -c256)
curl -sS -X POST "$THREADY_BASE/processing/callbacks/$PROVIDER" \
  -H "X-Thready-Signature: sha256=$SIG" \
  -H 'Content-Type: application/json' \
  -H "Idempotency-Key: $THREADY_IDEMPOTENCY_KEY" \
  --data-raw "$BODY" -i

# --- Same path, a Boba failed-torrent callback -------------------------------
PROVIDER="boba"
BODY='{"job_id":"dl-9932","provider":"boba","state":"failed","progress":0.12,"result_asset_ref":null,"error":"no seeders"}'
SIG=$(printf '%s' "$BODY" | openssl dgst -sha256 -hmac "$THREADY_HMAC_SECRET" -binary | xxd -p -c256)
curl -sS -X POST "$THREADY_BASE/processing/callbacks/$PROVIDER" \
  -H "X-Thready-Signature: sha256=$SIG" \
  -H 'Content-Type: application/json' \
  --data-raw "$BODY" -i
