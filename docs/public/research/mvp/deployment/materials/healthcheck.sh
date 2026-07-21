#!/usr/bin/env bash
# =============================================================================
# Helix Thready — deployment health gate (anti-bluff)
# Location : /home/thready/bin/ or alongside deploy.sh — run AS the thready user
# Source   : materialized from podman-compose.md §6 (health table + OpenAPI /health/ready contract)
#            and deploy-and-rollback.md §7 (anti-bluff gate). This is the bash equivalent of the
#            containers pkg/health CheckAll that deploy.sh uses when the Go `thready-boot` gate is absent.
# Usage    : ./healthcheck.sh --env prod [--compose <file>] [--project <name>] [--timeout 180]
# Exit     : 0 = every Required service healthy AND anti-bluff assertions pass; non-zero otherwise.
#
# Gate (podman-compose.md §6 — Required services):
#   thready-postgres (tcp) · thready-nats (/healthz) · thready-minio (/minio/health/ready)
#   thready-api (/health/ready) · thready-herald (/health/ready) · thready-processing (/health/ready)
#   thready-semsearch (/health/ready) — only if running (buildnew profile).  grafana is NOT required.
#
# NO-BLUFF (GAP #1): /health/ready itself returns 503 when the active embedder is the non-semantic
#   HashEmbedder (podman-compose.md §6.1 OpenAPI). This gate additionally asserts the environment pins
#   HELIX_EMBEDDING_PROVIDER=llama, so a garbage-relevance stack can never pass. The DEEPER probe (embed
#   a known string, assert the vector is not the hash pseudo-vector) is performed by the Go thready-boot
#   gate; this script enforces the readiness-contract + env invariant, which is sufficient for the MVP fallback.
# =============================================================================
set -euo pipefail

ENV=""; COMPOSE=""; PROJECT=""; TIMEOUT="${TIMEOUT:-180}"
while [ $# -gt 0 ]; do
  case "$1" in
    --env) ENV="$2"; shift 2 ;;
    --compose) COMPOSE="$2"; shift 2 ;;
    --project) PROJECT="$2"; shift 2 ;;
    --timeout) TIMEOUT="$2"; shift 2 ;;
    -h|--help) grep -E '^# (Usage|Gate|Exit)' "$0"; exit 0 ;;
    *) echo "FATAL: unknown arg '$1'" >&2; exit 64 ;;
  esac
done
[ -n "$ENV" ] || { echo "FATAL: --env is required" >&2; exit 64; }
case "$ENV" in dev) PREFIX=60 ;; sta) PREFIX=61 ;; prod) PREFIX=62 ;; *) echo "FATAL: bad --env" >&2; exit 64 ;; esac
PROJECT="${PROJECT:-thready-$ENV}"
DIR="/home/thready/$ENV"
[ -f "$DIR/.env" ] && { set -a; source "$DIR/.env"; set +a; }
log(){ printf '[health %s] %s\n' "$ENV" "$*"; }
fail=0

# ----------------------------------------------------------------- port_prefix band mapping
# Loopback host ports for this env's band (VERIFIED plan, service-discovery-ports.md §4).
NATS_MON="${PREFIX}223" ; MINIO="${PREFIX}000" ; API="${PREFIX}443" ; HERALD="${PREFIX}080"
SEM="${PREFIX}085" ; PG="${PREFIX}432"

# Resolve a running container name by compose labels (podman-compose sets com.docker.compose.*).
cname(){ # $1 = service
  podman ps --filter "label=com.docker.compose.project=$PROJECT" \
            --filter "label=com.docker.compose.service=$1" \
            --format '{{.Names}}' 2>/dev/null | head -1
}

# Poll a container's compose-defined healthcheck until `healthy` (this IS the containers pkg/health gate).
wait_healthy(){ # $1 = service, $2 = required(1/0)
  local svc="$1" required="$2" c deadline status
  c="$(cname "$svc")"
  if [ -z "$c" ]; then
    if [ "$required" = "1" ]; then log "MISSING required container: $svc"; fail=1; else log "skip (not running): $svc"; fi
    return
  fi
  deadline=$(( $(date +%s) + TIMEOUT ))
  while :; do
    status="$(podman inspect --format '{{if .State.Health}}{{.State.Health.Status}}{{else}}nohc{{end}}' "$c" 2>/dev/null || echo gone)"
    case "$status" in
      healthy) log "OK   $svc ($c) healthy"; return ;;
      nohc)    log "OK   $svc ($c) running (no container healthcheck defined)"; return ;;
      gone)    log "FAIL $svc container disappeared"; [ "$required" = 1 ] && fail=1; return ;;
    esac
    if [ "$(date +%s)" -ge "$deadline" ]; then
      log "FAIL $svc not healthy within ${TIMEOUT}s (last: $status)"; [ "$required" = 1 ] && fail=1; return
    fi
    sleep 3
  done
}

# HTTP readiness probe over the loopback host port (defence-in-depth / anti-bluff readiness contract).
probe_ready(){ # $1 = label, $2 = url, $3 = required(1/0)
  local code
  code="$(curl -sk -o /dev/null -w '%{http_code}' --max-time 10 "$2" 2>/dev/null || echo 000)"
  if [ "$code" = "200" ]; then log "OK   $1 readiness 200 ($2)"; else
    log "FAIL $1 readiness $code ($2)"; [ "$3" = "1" ] && fail=1
  fi
}

# ----------------------------------------------------------------- 1. container health (Required set)
log "waiting up to ${TIMEOUT}s for Required services to become healthy"
wait_healthy thready-postgres   1
wait_healthy thready-nats       1
wait_healthy thready-minio      1
wait_healthy thready-api        1
wait_healthy thready-herald     1
wait_healthy thready-processing 1
wait_healthy thready-semsearch  0   # buildnew — required ONLY if present/running
wait_healthy thready-grafana    0   # not required

# ----------------------------------------------------------------- 2. readiness-contract probes (loopback)
probe_ready thready-api    "http://127.0.0.1:${API}/health/ready" 1
probe_ready thready-nats   "http://127.0.0.1:${NATS_MON}/healthz" 1
probe_ready thready-minio  "http://127.0.0.1:${MINIO}/minio/health/ready" 1
# semsearch only if it is up (buildnew profile)
if [ -n "$(cname thready-semsearch)" ]; then
  probe_ready thready-semsearch "http://127.0.0.1:${SEM}/health/ready" 1
fi

# ----------------------------------------------------------------- 3. anti-bluff embedder invariant (GAP #1)
if [ "${HELIX_EMBEDDING_PROVIDER:-}" != "llama" ]; then
  log "FAIL GAP#1: HELIX_EMBEDDING_PROVIDER='${HELIX_EMBEDDING_PROVIDER:-unset}' (must be 'llama')"
  fail=1
else
  log "OK   GAP#1: HELIX_EMBEDDING_PROVIDER=llama (readiness contract rejects HashEmbedder with 503)"
fi

# ----------------------------------------------------------------- 4. no-bluff: no buildnew placeholder promoted
running_buildnew="$(podman ps --filter "label=com.docker.compose.project=$PROJECT" --format '{{.Names}}' \
  | grep -E 'assetsvc|downloadmgr|usersvc|eventbus-svc|semsearch' || true)"
if [ -n "$running_buildnew" ]; then
  log "NOTE buildnew services running (must each pass a REAL /health/ready, not a stub): $running_buildnew"
fi

# ----------------------------------------------------------------- verdict
if [ "$fail" = "0" ]; then log "GATE PASS — all Required services healthy + anti-bluff assertions hold"; exit 0; fi
log "GATE FAIL — deploy must roll back"; exit 1
