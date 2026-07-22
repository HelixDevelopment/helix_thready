#!/usr/bin/env bash
# screenshot-designs.sh — render every Helix Thready design HTML to a 2x PNG
# in BOTH light and dark themes, then verify each PNG is non-blank and that the
# dark renders are actually dark. Reusable; no npm deps (Node >=22 CDP driver).
#
# Outputs:  <design>/exports/png/<area>/<screen>[-dark]@2x.png
# Verify:   <design>/exports/png/verify.txt
# Usage:    bash screenshot-designs.sh
set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DESIGN_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
OUT="$DESIGN_DIR/exports/png"
CHROMIUM="${CHROMIUM:-/usr/bin/chromium}"
MAGICK="${MAGICK:-magick}"
PORT="${CDP_PORT:-9351}"
MANIFEST="/tmp/screenshot-designs-manifest.json"
REPORT="/tmp/screenshot-designs-report.json"
mkdir -p "$OUT"/{web,mobile,desktop,tui,library,motion}

# ---- build render manifest (area -> width/mobile map, globbed file list) ----
node - "$DESIGN_DIR" "$OUT" "$CHROMIUM" "$PORT" "$REPORT" "$MANIFEST" <<'NODE'
import { readdirSync, writeFileSync } from 'node:fs';
const [design, out, chromium, port, report, manifestPath] = process.argv.slice(2);
const areas = [
  { area: 'web',     dir: `${design}/screens/web`,     width: 1440, mobile: false },
  { area: 'mobile',  dir: `${design}/screens/mobile`,  width: 390,  mobile: true  },
  { area: 'desktop', dir: `${design}/screens/desktop`, width: 1440, mobile: false },
  { area: 'tui',     dir: `${design}/screens/tui`,     width: 1200, mobile: false },
  { area: 'library', dir: `${design}/library`,         width: 1440, mobile: false, only: ['components.html'] },
  { area: 'motion',  dir: `${design}/motion`,          width: 1200, mobile: false, only: ['preview.html'] },
];
const jobs = [];
for (const a of areas) {
  let files = readdirSync(a.dir).filter(f => f.endsWith('.html'));
  if (a.only) files = files.filter(f => a.only.includes(f));
  files.sort();
  for (const f of files) {
    const base = f.replace(/\.html$/, '');
    for (const theme of ['light', 'dark']) {
      const suffix = theme === 'dark' ? '-dark' : '';
      jobs.push({ url: `file://${a.dir}/${f}`, out: `${out}/${a.area}/${base}${suffix}@2x.png`, theme, width: a.width, mobile: a.mobile });
    }
  }
}
writeFileSync(manifestPath, JSON.stringify({ chromium, port: Number(port), settleMs: 800, reportOut: report, jobs }, null, 2));
console.error(`manifest: ${jobs.length} jobs`);
NODE

echo "=== rendering (deviceScaleFactor=2) ==="
node "$SCRIPT_DIR/cdp-render.mjs" "$MANIFEST"

# ---- verify: non-blank (std>0.02) + dark renders dark / light renders light ----
echo "=== verifying (parallel) ==="
VERIFY="$OUT/verify.txt"; : > "$VERIFY"
cat > /tmp/_verify1.sh <<'SH'
f="$1"; ROOT="$2"; MAGICK="${MAGICK:-magick}"
read pix std <<<"$($MAGICK "$f" -resize '800x800>' -format '%[pixel:p{1,1}] %[fx:standard_deviation]' info: 2>/dev/null)"
lum=$(printf '%s' "$pix" | sed -E 's/.*\(([0-9]+),([0-9]+),([0-9]+).*/\1 \2 \3/' | awk '{print ($1*0.299+$2*0.587+$3*0.114)/255}')
size=$(stat -c%s "$f"); v=PASS
awk "BEGIN{exit !($std>0.02 && $size>4000)}" 2>/dev/null || v=BLANK
case "$f" in
  *-dark@2x.png) awk "BEGIN{exit !($lum<0.30)}" 2>/dev/null || v=NOTDARK ;;
  *) awk "BEGIN{exit !($lum>0.60)}" 2>/dev/null || v=NOTLIGHT ;;
esac
printf '%-8s %-46s size=%-9s cornerLum=%-8.4f std=%s\n' "$v" "${f#$ROOT/}" "$size" "$lum" "$std"
SH
find "$OUT" -name '*@2x.png' -print0 | sort -z | xargs -0 -P8 -I{} bash /tmp/_verify1.sh {} "$OUT" >> "$VERIFY" 2>/dev/null
light=$(grep -cE '[^-]@2x.png' "$VERIFY"); dark=$(grep -c -- '-dark@2x.png' "$VERIFY")
pass=$(grep -c '^PASS' "$VERIFY"); fail=$(grep -vc '^PASS' "$VERIFY")
echo "light: $light   dark: $dark   total: $((light+dark))   PASS: $pass   FAIL: $fail"
echo "detail -> $VERIFY"
grep -vE '^PASS' "$VERIFY" && { echo "VERIFICATION FAILURES"; exit 1; } || echo "all PNGs verified"
