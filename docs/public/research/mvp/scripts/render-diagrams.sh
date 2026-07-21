#!/usr/bin/env bash
#
# render-diagrams.sh — render every Mermaid `.mmd` source under
#   docs/public/research/mvp/**/diagrams/ to a sibling raster/vector file.
#
# Constitution §11.4.65 (md→HTML/PDF sync): the `.mmd` files are the source of
# truth; PNG/SVG export runs via the Docs Chain / mermaid-cli at build time.
# This script IS that export step and is safe to re-run (idempotent overwrite).
#
# Usage:
#   scripts/render-diagrams.sh            # render all to .svg (preferred)
#   scripts/render-diagrams.sh png        # render all to .png
#   scripts/render-diagrams.sh svg api    # only the api/ area, as .svg
#
# Requirements (per CONVENTIONS §4): Node.js + @mermaid-js/mermaid-cli (`mmdc`),
# plus a Chromium/Chrome that Puppeteer can drive. If `mmdc` is not on PATH the
# script falls back to `npx -y @mermaid-js/mermaid-cli`. A system Chromium is
# auto-detected and driven with --no-sandbox so this works in CI/containers.
#
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
MVP_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

FMT="${1:-svg}"          # svg (default) | png | pdf
AREA="${2:-}"            # optional area filter, e.g. api, architecture, database

# --- pick a renderer -------------------------------------------------------
if command -v mmdc >/dev/null 2>&1; then
  MMDC=(mmdc)
elif command -v npx >/dev/null 2>&1; then
  MMDC=(npx -y @mermaid-js/mermaid-cli)
else
  echo "ERROR: neither 'mmdc' nor 'npx' found on PATH." >&2
  echo "Install with: npm i -g @mermaid-js/mermaid-cli" >&2
  exit 127
fi

# --- Puppeteer config: prefer a system Chromium, sandbox-safe args ---------
PCFG="$(mktemp)"
trap 'rm -f "$PCFG"' EXIT
CHROME="${PUPPETEER_EXECUTABLE_PATH:-}"
if [ -z "$CHROME" ]; then
  for c in chromium chromium-browser google-chrome google-chrome-stable; do
    if command -v "$c" >/dev/null 2>&1; then CHROME="$(command -v "$c")"; break; fi
  done
fi
if [ -n "$CHROME" ]; then
  printf '{"executablePath":"%s","args":["--no-sandbox","--disable-gpu"]}\n' "$CHROME" > "$PCFG"
else
  # No system Chromium found; rely on the one bundled with Puppeteer.
  printf '{"args":["--no-sandbox","--disable-gpu"]}\n' > "$PCFG"
fi

# --- render loop -----------------------------------------------------------
SEARCH_ROOT="$MVP_ROOT"
[ -n "$AREA" ] && SEARCH_ROOT="$MVP_ROOT/$AREA"

total=0; ok=0; fail=0
declare -a FAILED=()

while IFS= read -r -d '' mmd; do
  total=$((total + 1))
  out="${mmd%.mmd}.${FMT}"
  if "${MMDC[@]}" -i "$mmd" -o "$out" -p "$PCFG" >/dev/null 2>&1 && [ -s "$out" ]; then
    ok=$((ok + 1))
    printf 'OK    %s\n' "${mmd#"$MVP_ROOT"/}"
  else
    fail=$((fail + 1))
    FAILED+=("${mmd#"$MVP_ROOT"/}")
    printf 'FAIL  %s\n' "${mmd#"$MVP_ROOT"/}"
  fi
done < <(find "$SEARCH_ROOT" -type f -path '*/diagrams/*.mmd' -print0 | sort -z)

echo "--------------------------------------------------------------------"
echo "Rendered ${ok}/${total} diagram(s) to .${FMT} (${fail} failed)."
if [ "$fail" -gt 0 ]; then
  printf '  failed: %s\n' "${FAILED[@]}"
  exit 1
fi
