#!/usr/bin/env bash
# =============================================================================
#  Helix Thready — SEARCH request collection (curl)
#  Tag: search  |  Source of truth: ../../openapi.yaml
#  x-thready-maturity: build_new. SLO < 500 ms p95. FAIL-LOUD: 503 unavailable
#  if the HashEmbedder stub is active (needs HELIX_EMBEDDING_PROVIDER=llama);
#  the response `embedder` field echoes the active provider.
# =============================================================================
set -uo pipefail
: "${THREADY_BASE:=https://dev.thready.hxd3v.com/v1}"
AUTH=(-H "Authorization: Bearer $THREADY_TOKEN")

# --- POST /search — hybrid over posts + generated materials ------------------
curl -sS -X POST "$THREADY_BASE/search" "${AUTH[@]}" \
  -H 'Content-Type: application/json' \
  -d '{"query":"self-hosted vector database benchmarks","mode":"hybrid","sources":["posts","generated"],"top_k":20,"rerank":true}'

# --- POST /search — pure semantic, filtered by category + hashtag ------------
curl -sS -X POST "$THREADY_BASE/search" "${AUTH[@]}" \
  -H 'Content-Type: application/json' \
  -d '{"query":"documentary about deep sea","mode":"semantic","sources":["posts"],"filters":{"categories":["documentary"],"hashtags":["ocean"]},"top_k":10}'

# --- POST /search — keyword mode across posts, generated, and assets ---------
curl -sS -X POST "$THREADY_BASE/search" "${AUTH[@]}" \
  -H 'Content-Type: application/json' \
  -d '{"query":"gitlab runner cache","mode":"keyword","sources":["posts","generated","assets"],"rerank":false}'
