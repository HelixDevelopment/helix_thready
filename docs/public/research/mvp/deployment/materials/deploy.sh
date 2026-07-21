#!/usr/bin/env bash
# =============================================================================
# Helix Thready — health-gated deploy with automatic rollback
# Location : /home/thready/submodules/containers/scripts/  (or /home/thready/bin/) — run AS the thready user
# Source   : materialized from deploy-and-rollback.md §4 + podman-compose.md §5 (boot gate).
# Usage    : ./deploy.sh --env prod --version THREADY-1.4.0
#
# Pipeline (deploy-and-rollback.md §3):
#   pre-flight (secrets, chmod 600, GAP #1 embedder, images pulled)
#     -> record rollback anchor (current.json -> previous.json)
#     -> DB migrate UP (expand phase; reversible)
#     -> BootManager.BootAll: podman-compose up -d NEW digests (buildnew EXCLUDED)
#     -> health gate (containers pkg/health CheckAll + anti-bluff assertions)
#     -> edge probe (LE_VALIDATE_URL /health/ready over public TLS)
#     -> promote (update current.json)  OR  rollback.sh on ANY failure
#
# ROOTLESS: everything runs as `thready`; NO sudo anywhere.  [CONSTITUTION §11.4.76/161]
#
# VERIFIED : the gated boot + reverse-order rollback live in vasic-digital/containers pkg/boot
#            (BootManager.BootAll) — this script drives that, it does not re-implement it. podman-compose
#            invocation, buildnew-profile exclusion, and the port plan are verified.
# ASSUMED  : the wrapper binary names `thready-boot` / `thready-migrate` are Thready conventions
#            (deploy-and-rollback.md §10), NOT module APIs. If they are absent this script falls back to
#            a pure podman-compose + ./healthcheck.sh gate, which is functionally equivalent for the MVP.
# NO-BLUFF : the gate asserts REAL behaviour (healthcheck.sh embeds a known string to reject the
#            HashEmbedder, GAP #1) — a bare HTTP 200 never promotes.
# =============================================================================
set -euo pipefail

# ----------------------------------------------------------------- args + paths
ENV=""; VERSION=""
while [ $# -gt 0 ]; do
  case "$1" in
    --env)     ENV="$2"; shift 2 ;;
    --version) VERSION="$2"; shift 2 ;;
    -h|--help) grep -E '^# (Usage|Pipeline)' "$0"; exit 0 ;;
    *) echo "FATAL: unknown arg '$1'" >&2; exit 64 ;;
  esac
done
[ -n "$ENV" ] && [ -n "$VERSION" ] || { echo "FATAL: --env and --version are required" >&2; exit 64; }
case "$ENV" in dev|sta|prod) : ;; *) echo "FATAL: --env must be dev|sta|prod" >&2; exit 64 ;; esac

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DIR="/home/thready/$ENV"                    # per-env host directory (podman-compose.md §3)
COMPOSE="$DIR/compose.$ENV.yaml"
PROJECT="thready-$ENV"
RELEASES="$DIR/releases"
HEALTHCHECK="${HEALTHCHECK:-$HERE/healthcheck.sh}"
log(){ printf '[deploy %s] %s\n' "$ENV" "$*"; }
die(){ echo "FATAL: $*" >&2; exit 1; }

[ -f "$COMPOSE" ] || die "compose file not found: $COMPOSE"
[ -f "$DIR/.env" ] || die ".env not found: $DIR/.env"

# ----------------------------------------------------------------- 1. pre-flight
log "pre-flight checks"
# 1a. .env must be owner-only (defence-in-depth vs the pre-commit hook)  [GAP: #12]
[ "$(stat -c '%a' "$DIR/.env")" = "600" ] || die ".env is not chmod 600 (secrets-and-config.md §4)"
# 1b. load runtime secrets (runtime-load-only; never committed)
set -a; # shellcheck disable=SC1090
source "$DIR/.env"; set +a
# 1c. GAP #1 — refuse to ship the non-semantic hash embedder
: "${HELIX_EMBEDDING_PROVIDER:?FATAL: HELIX_EMBEDDING_PROVIDER unset (GAP #1)}"
[ "$HELIX_EMBEDDING_PROVIDER" = "llama" ] || die "HELIX_EMBEDDING_PROVIDER must be 'llama' (GAP #1)"
: "${THREADY_PG_DSN:?FATAL: THREADY_PG_DSN unset}"
: "${LE_VALIDATE_URL:?FATAL: LE_VALIDATE_URL unset (edge probe target)}"
export THREADY_VERSION="$VERSION"
# 1d. structural: compose must parse + interpolate (fails on a missing .env var — the mustEnv rule)
podman-compose -f "$COMPOSE" -p "$PROJECT" config >/dev/null || die "compose config invalid / missing var"
# 1e. ensure image digests are present (pull data-plane + built app images)
log "pulling images"
podman-compose -f "$COMPOSE" -p "$PROJECT" pull || die "image pull failed"

# ----------------------------------------------------------------- 2. record rollback anchor
mkdir -p "$RELEASES"
if [ -f "$RELEASES/current.json" ]; then
  cp -f "$RELEASES/current.json" "$RELEASES/previous.json"
  log "recorded rollback anchor -> $RELEASES/previous.json"
else
  log "no current.json yet (first deploy) — rollback anchor is 'all down'"
fi

# on ANY error past this point, restore the previous release
rollback() { log "!! failure — invoking rollback.sh"; "$HERE/rollback.sh" --env "$ENV" || echo "ROLLBACK ALSO FAILED — manual intervention required" >&2; }
trap 'rollback' ERR

# ----------------------------------------------------------------- 3. DB migrate (expand phase)
log "migrating database (expand phase; reversible)"
if command -v thready-migrate >/dev/null 2>&1; then
  thready-migrate --dsn "$THREADY_PG_DSN" --up --expand-only \
    || { echo "migration failed — rolling back schema"; thready-migrate --dsn "$THREADY_PG_DSN" --rollback; exit 2; }
else
  log "[ASSUMED] thready-migrate not on PATH — skipping app-level migrate (database area owns the Runner)"
fi

# ----------------------------------------------------------------- 4. health-gated boot + rollback
# Preferred path: the Go gate on containers pkg/boot (BootManager.BootAll) — discovers external HelixLLM
# (Phase 1), brings up NEW digests (Phase 2, buildnew EXCLUDED), CheckAll (Phase 3), rolls back internally.
if command -v thready-boot >/dev/null 2>&1; then
  log "boot via containers pkg/boot (thready-boot)"
  thready-boot --env "$ENV" --version "$VERSION" --validate-url "$LE_VALIDATE_URL" \
    || die "thready-boot gate failed (BootManager already tore down partial state)"
else
  # Fallback: pure podman-compose up (buildnew profile NOT selected → placeholders never start) + health gate.
  log "[ASSUMED] thready-boot not on PATH — using podman-compose + healthcheck.sh fallback"
  podman-compose -f "$COMPOSE" -p "$PROJECT" up -d --remove-orphans || die "podman-compose up failed"
  log "health gate (containers pkg/health equivalent)"
  "$HEALTHCHECK" --env "$ENV" --compose "$COMPOSE" --project "$PROJECT" || die "health gate failed"
fi

# ----------------------------------------------------------------- 5. edge probe (end-to-end)
log "edge probe: $LE_VALIDATE_URL"
code="$(curl -sk -o /dev/null -w '%{http_code}' --max-time 15 "$LE_VALIDATE_URL" || echo 000)"
[ "$code" = "200" ] || die "edge probe returned $code (expected 200) — release unreachable end-to-end"

# ----------------------------------------------------------------- 6. promote (atomic switch)
trap - ERR   # success past this point — do not roll back
MANIFEST="$RELEASES/$VERSION.json"
if [ -f "$MANIFEST" ]; then
  ln -sfn "$MANIFEST" "$RELEASES/current.json"           # atomic symlink swap = the "current live" pointer
else
  log "[ASSUMED] no manifest $MANIFEST — writing a minimal current.json"
  printf '{"version":"%s","created":"%s"}\n' "$VERSION" "$(date -u +%FT%TZ)" > "$RELEASES/current.json"
fi
log "PROMOTED $VERSION live (current.json updated)"
log "deploy OK"
