#!/usr/bin/env bash
# =============================================================================
#  Helix Thready — ASSETS request collection (curl)
#  Tag: assets  |  Source of truth: ../../openapi.yaml
#  Content is resolved by the Asset Service to a signed, Range-capable URL —
#  never a raw path. redownload/downloads are build_new (Download Manager P0).
# =============================================================================
set -uo pipefail
: "${THREADY_BASE:=https://dev.thready.hxd3v.com/v1}"
AUTH=(-H "Authorization: Bearer $THREADY_TOKEN")

# --- GET /assets — list assets for the tenant (filter by kind) ---------------
curl -sS "$THREADY_BASE/assets?limit=50" "${AUTH[@]}"
curl -sS "$THREADY_BASE/assets?kind=video&limit=50" "${AUTH[@]}"

# --- GET /assets/{assetId} — metadata (renditions, hash, sensitivity) --------
curl -sS "$THREADY_BASE/assets/$THREADY_ASSET_ID" "${AUTH[@]}"

# --- GET /assets/{assetId}/content — resolve to signed content (302/200) -----
# -L follows the 302 to the short-lived signed URL.
curl -sS -L "$THREADY_BASE/assets/$THREADY_ASSET_ID/content" "${AUTH[@]}" -o asset.bin -D -

# --- GET /assets/{assetId}/content — Range request (206 partial) -------------
curl -sS "$THREADY_BASE/assets/$THREADY_ASSET_ID/content" "${AUTH[@]}" \
  -H 'Range: bytes=0-1048575' -o asset.part -D -

# --- POST /assets/{assetId}/redownload — re-fetch a broken asset (202) -------
# x-thready-maturity: build_new — may 503 until the Download Manager lands.
curl -sS -X POST "$THREADY_BASE/assets/$THREADY_ASSET_ID/redownload" "${AUTH[@]}" \
  -H "Idempotency-Key: $THREADY_IDEMPOTENCY_KEY" -i

# --- GET /posts/{postId}/assets — assets linked to a post --------------------
curl -sS "$THREADY_BASE/posts/$THREADY_POST_ID/assets" "${AUTH[@]}"

# --- GET /downloads — list download jobs (Download Manager/Boba/MeTube) ------
curl -sS "$THREADY_BASE/downloads?limit=50" "${AUTH[@]}"
