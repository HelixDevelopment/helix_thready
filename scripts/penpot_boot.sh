#!/usr/bin/env bash
# =============================================================================
# PenPot Container Boot (helix_thready)
# -----------------------------------------------------------------------------
# Purpose: Boot the PenPot design-platform stack (frontend/backend/exporter +
#          postgres + valkey) via rootless podman, using the repo-root
#          compose.penpot.yml + gitignored .env.penpot secrets.
# Usage:   scripts/penpot_boot.sh [up|down|status]
#
# Pattern mirrors helix_track/scripts/helixtrack_boot.sh (CONST-051(B):
# project-specific compose lives in the consuming repo; the containers/
# submodule stays project-agnostic). Verification is anti-bluff: PASS is
# "the user can load PenPot and log in", not "compose returned 0".
# =============================================================================
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
COMPOSE_FILE="${ROOT}/compose.penpot.yml"
ENV_FILE="${ROOT}/.env.penpot"
ACTION="${1:-up}"

RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'
CYAN='\033[0;36m'; NC='\033[0m'
log_info()  { echo -e "${CYAN}[INFO]${NC} $*"; }
log_ok()    { echo -e "${GREEN}[OK]${NC} $*"; }
log_step()  { echo -e "${YELLOW}[STEP]${NC} $*"; }
log_error() { echo -e "${RED}[ERROR]${NC} $*"; }

[ -f "$COMPOSE_FILE" ] || { log_error "Compose file not found: ${COMPOSE_FILE}"; exit 1; }
[ -f "$ENV_FILE" ]     || { log_error "Secrets env not found: ${ENV_FILE} (gitignored; must be generated locally)"; exit 1; }
command -v podman-compose >/dev/null || { log_error "podman-compose is required"; exit 1; }

# Export secrets for compose ${VAR} substitution (never echoed — §11.4.10).
set -a; . "$ENV_FILE"; set +a
: "${PENPOT_VERSION:?}" "${PENPOT_PG_PASSWORD:?}" "${PENPOT_SECRET_KEY:?}" "${PENPOT_PUBLIC_URI:?}"

case "$ACTION" in
  down)
    log_step "Stopping PenPot stack..."
    podman-compose -f "$COMPOSE_FILE" down
    exit 0 ;;
  status)
    podman ps --filter name=penpot- --format '{{.Names}}\t{{.Status}}\t{{.Ports}}'
    exit 0 ;;
  up) ;;
  *) log_error "Unknown action: ${ACTION} (use up|down|status)"; exit 1 ;;
esac

echo ""
echo -e "${CYAN}== PenPot Container Boot (helix_thready) — v${PENPOT_VERSION} ==${NC}"
echo ""

log_step "Starting PenPot stack (rootless podman)..."
podman-compose -f "$COMPOSE_FILE" up -d 2>&1 | tail -8

log_info "Waiting for backend readiness via frontend proxy ${PENPOT_PUBLIC_URI}/readyz ..."
for i in $(seq 1 120); do
    if curl -sf "${PENPOT_PUBLIC_URI}/readyz" >/dev/null 2>&1; then
        log_ok "Backend ready (${i}s)"
        # Buffer body before grep: `grep -q` + pipefail turns curl's SIGPIPE
        # (exit 23) into a false negative on large bodies.
        FRONTEND_BODY="$(curl -sf --max-time 10 "${PENPOT_PUBLIC_URI}/" || true)"
        if grep -qi '<html' <<<"$FRONTEND_BODY"; then
            log_ok "Frontend serves HTML on ${PENPOT_PUBLIC_URI}"
        else
            log_error "Frontend did not return HTML on ${PENPOT_PUBLIC_URI}"
            exit 1
        fi
        log_info "Containers:"
        podman ps --filter name=penpot- --format '  {{.Names}}\t{{.Status}}'
        echo ""
        log_ok "PenPot is up: ${PENPOT_PUBLIC_URI} (credentials: ${ROOT}/.penpot-credentials)"
        exit 0
    fi
    sleep 1
done

echo ""
log_error "PenPot did not become ready within 120s"
log_info "Check logs: podman logs penpot-backend"
exit 1
