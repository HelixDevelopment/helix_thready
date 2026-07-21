#!/usr/bin/env bash
# =============================================================================
# Helix Thready — release rollback (restore the previous known-good release)
# Location : /home/thready/submodules/containers/scripts/ (or /home/thready/bin/) — run AS the thready user
# Source   : materialized from deploy-and-rollback.md §6 (layered rollback) + §4 (restorePrevious).
# Usage    : ./rollback.sh --env prod            # restore releases/previous.json
#            ./rollback.sh --env prod --to THREADY-1.3.0   # restore a specific manifest
#
# What rolls back (deploy-and-rollback.md §6):
#   Health-gate / edge-probe failure -> BootManager reverse-order Down of NEW containers + up PREVIOUS digests.
#   Because artifacts are pinned by digest, rollback is deterministic: bring the EXACT previous digests up.
#   Expand-only migrations removed nothing the previous code relied on, so data written by the failed
#   release stays valid; irreversible data ops are deferred to a later CONTRACT release.
#
# NOTE: certificate rollback is handled INDEPENDENTLY by the lets_encrypt risk-free gate
#       (tls-lets-encrypt.md §7) — a cert failure never blocks, and is never blocked by, an app rollback.
#
# VERIFIED : reverse-order teardown lives in containers pkg/boot (BootManager.rollback). This script
#            performs the release-LEVEL restore (previous.json digests) on top of that container-level rollback.
# ASSUMED  : `thready-boot --restore` / `thready-migrate` wrapper names are Thready conventions; a pure
#            podman-compose fallback is provided.
# =============================================================================
set -euo pipefail

ENV=""; TO=""
while [ $# -gt 0 ]; do
  case "$1" in
    --env) ENV="$2"; shift 2 ;;
    --to)  TO="$2";  shift 2 ;;
    -h|--help) grep -E '^# (Usage|What)' "$0"; exit 0 ;;
    *) echo "FATAL: unknown arg '$1'" >&2; exit 64 ;;
  esac
done
[ -n "$ENV" ] || { echo "FATAL: --env is required" >&2; exit 64; }
case "$ENV" in dev|sta|prod) : ;; *) echo "FATAL: --env must be dev|sta|prod" >&2; exit 64 ;; esac

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DIR="/home/thready/$ENV"
COMPOSE="$DIR/compose.$ENV.yaml"
PROJECT="thready-$ENV"
RELEASES="$DIR/releases"
HEALTHCHECK="${HEALTHCHECK:-$HERE/healthcheck.sh}"
log(){ printf '[rollback %s] %s\n' "$ENV" "$*"; }
die(){ echo "FATAL: $*" >&2; exit 1; }

[ -f "$DIR/.env" ] && { set -a; source "$DIR/.env"; set +a; }

# ----------------------------------------------------------------- resolve target
TARGET="$RELEASES/previous.json"
[ -n "$TO" ] && TARGET="$RELEASES/$TO.json"
if [ ! -f "$TARGET" ]; then
  log "no rollback target ($TARGET) — bringing the stack DOWN cleanly (nothing to restore to)"
  podman-compose -f "$COMPOSE" -p "$PROJECT" down || true
  die "no previous release to restore; stack is down"
fi
PREV_VERSION="$(grep -oE '"version"[[:space:]]*:[[:space:]]*"[^"]+"' "$TARGET" | head -1 | sed -E 's/.*"([^"]+)"$/\1/')"
log "restoring previous release: ${PREV_VERSION:-unknown}  (manifest: $TARGET)"

# ----------------------------------------------------------------- 1. restore previous digests
if command -v thready-boot >/dev/null 2>&1; then
  # Preferred: the Go gate restores the exact previous digests and re-checks health.
  thready-boot --env "$ENV" --restore "$TARGET" || die "thready-boot --restore failed"
else
  # Fallback: down the current containers, pin THREADY_VERSION to the previous, up again.
  log "[ASSUMED] thready-boot absent — podman-compose fallback restore"
  export THREADY_VERSION="${PREV_VERSION:-$THREADY_VERSION}"
  podman-compose -f "$COMPOSE" -p "$PROJECT" down || true
  podman-compose -f "$COMPOSE" -p "$PROJECT" up -d --remove-orphans || die "restore up failed"
fi

# ----------------------------------------------------------------- 2. (optional) contract-phase migration rollback
# Only if the failed release applied a schema change that must be undone. Expand-only migrations are
# backward-compatible, so this is usually a NO-OP; the down script is exercised in testing (§5).
if command -v thready-migrate >/dev/null 2>&1 && [ "${ROLLBACK_MIGRATION:-0}" = "1" ]; then
  log "rolling back migration (contract phase)"
  thready-migrate --dsn "${THREADY_PG_DSN:?}" --rollback || die "migration rollback failed"
fi

# ----------------------------------------------------------------- 3. re-verify health after restore
if [ -x "$HEALTHCHECK" ]; then
  log "verifying restored stack health"
  "$HEALTHCHECK" --env "$ENV" --compose "$COMPOSE" --project "$PROJECT" \
    || die "restored stack is UNHEALTHY — manual intervention required"
fi

# ----------------------------------------------------------------- 4. re-point current.json + reload edge
ln -sfn "$TARGET" "$RELEASES/current.json"
podman kill -s HUP caddy 2>/dev/null || log "edge HUP reload skipped (caddy not found — fine if nginx or edge down)"
log "ROLLED BACK to ${PREV_VERSION:-previous} (current.json re-pointed); served site restored"
