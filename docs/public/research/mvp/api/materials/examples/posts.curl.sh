#!/usr/bin/env bash
# =============================================================================
#  Helix Thready — POSTS / THREADS request collection (curl)
#  Tag: posts  |  Source of truth: ../../openapi.yaml
#  A "post" is a complete thread = root + full organic reply chain; Thready's
#  own system replies are excluded and never processed.
# =============================================================================
set -uo pipefail
: "${THREADY_BASE:=https://dev.thready.hxd3v.com/v1}"
AUTH=(-H "Authorization: Bearer $THREADY_TOKEN")

# --- GET /posts — list/filter (channel, hashtag, category, status, cursor) ---
curl -sS "$THREADY_BASE/posts?limit=50" "${AUTH[@]}"

# --- GET /posts — only failed posts in one channel ---------------------------
curl -sS "$THREADY_BASE/posts?channel_id=$THREADY_CHANNEL_ID&status=failed&limit=50" "${AUTH[@]}"

# --- GET /posts — by hashtag -------------------------------------------------
curl -sS "$THREADY_BASE/posts?hashtag=research&limit=50" "${AUTH[@]}"

# --- GET /posts — next page via opaque cursor --------------------------------
curl -sS "$THREADY_BASE/posts?limit=50&cursor=b3f0opaqueCursorFromPreviousPage" "${AUTH[@]}"

# --- GET /posts/{postId} — a single post -------------------------------------
curl -sS "$THREADY_BASE/posts/$THREADY_POST_ID" "${AUTH[@]}"

# --- GET /posts/{postId}/thread — full thread (root + organic replies) -------
curl -sS "$THREADY_BASE/posts/$THREADY_POST_ID/thread" "${AUTH[@]}"

# --- GET /posts/{postId}/assets — assets linked to a post --------------------
# (attachments + generated + web renditions; belongs to the assets tag)
curl -sS "$THREADY_BASE/posts/$THREADY_POST_ID/assets" "${AUTH[@]}"
