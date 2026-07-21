#!/usr/bin/env bash
# =============================================================================
#  Helix Thready — EVENTS request collection (curl)
#  Tag: events  |  Source of truth: ../../openapi.yaml + ../../event-bus-contract.md
#  x-thready-maturity: build_new (client-facing JetStream wrapper is BUILD-NEW).
#  REST here = catalog, sticky snapshots, event-sink CRUD. The live WS/SSE
#  streams are specified in event-bus-contract.md (§5) — SSE is curl-able below;
#  WebSocket is NOT in openapi.yaml (OpenAPI cannot model a WS upgrade).
# =============================================================================
set -uo pipefail
: "${THREADY_BASE:=https://dev.thready.hxd3v.com/v1}"
AUTH=(-H "Authorization: Bearer $THREADY_TOKEN")

# --- GET /events — the event catalog (types, sticky flag, required scopes) ---
curl -sS "$THREADY_BASE/events" "${AUTH[@]}"

# --- GET /events/{entityType}/{entityId}/sticky — last-value snapshot --------
# entityType: channel|post|processing|asset|download. 404 if none retained.
curl -sS "$THREADY_BASE/events/post/$THREADY_POST_ID/sticky" "${AUTH[@]}"

# --- GET /event-sinks — list outbound-webhook sinks (secrets never returned) -
curl -sS "$THREADY_BASE/event-sinks?limit=50" "${AUTH[@]}"

# --- POST /event-sinks — register a sink (HMAC secret returned ONCE) ---------
curl -sS -X POST "$THREADY_BASE/event-sinks" "${AUTH[@]}" \
  -H "Idempotency-Key: $THREADY_IDEMPOTENCY_KEY" \
  -H 'Content-Type: application/json' \
  -d '{"url":"https://ci.example.org/thready-hook","event_types":["processing.*","asset.ready"],"max_attempts":8}'

# --- GET /event-sinks/{sinkId} — fetch a sink (without its secret) -----------
curl -sS "$THREADY_BASE/event-sinks/$THREADY_SINK_ID" "${AUTH[@]}"

# --- PATCH /event-sinks/{sinkId} — enable/disable or change subscribed types -
curl -sS -X PATCH "$THREADY_BASE/event-sinks/$THREADY_SINK_ID" "${AUTH[@]}" \
  -H 'Content-Type: application/json' \
  -d '{"active":false,"event_types":["processing.completed"],"max_attempts":10}'

# --- DELETE /event-sinks/{sinkId} — delete (cascades the delivery ledger) ----
curl -sS -X DELETE "$THREADY_BASE/event-sinks/$THREADY_SINK_ID" "${AUTH[@]}" -i

# --- SSE stream (event-bus-contract.md §5; NOT in openapi.yaml) --------------
# One-way Server-Sent Events; -N disables buffering. Resume with Last-Event-ID.
curl -sS -N "$THREADY_BASE/events/stream?types=processing.completed,asset.ready" \
  "${AUTH[@]}" -H 'Accept: text/event-stream'
# Resume after a drop from the last seen event id:
#   curl -sS -N "$THREADY_BASE/events/stream?types=processing.completed" \
#     "${AUTH[@]}" -H 'Accept: text/event-stream' -H 'Last-Event-ID: 8f0e-uuid'

# --- WebSocket (bidirectional; event-bus-contract.md §5) ---------------------
# Not curl-able; use a WS client, e.g.:
#   websocat "wss://dev.thready.hxd3v.com/v1/events/ws" -H "Authorization: Bearer $THREADY_TOKEN"
# then send: {"op":"subscribe","types":["processing.*"],"account_id":"..."}
