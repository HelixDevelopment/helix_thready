#!/usr/bin/env bash
# capture-figma.sh — drive the genuine headless OD Figma IR capture for every
# design page and validate the output. See capture-figma.mjs for the mechanism
# (injects the clipper's own capture.js and reads window.__odCapture().figmaIr).
#
# Output: exports/figma/captures/*.od-figma.json, thready.od-figma.json,
#         capture-manifest.json
# Usage:  bash capture-figma.sh
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DESIGN_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
FIGMA_OUT="$DESIGN_DIR/exports/figma"
CAPTURE_JS="/home/milos/Factory/projects/tools_and_research/.opendesign-src/open-design/clipper/capture.js"
JOBS="/tmp/capture-figma-jobs.json"
export CDP_PORT="${CDP_PORT:-9371}"
export CHROMIUM="${CHROMIUM:-/usr/bin/chromium}"
[ -f "$CAPTURE_JS" ] || { echo "clipper capture.js not found at $CAPTURE_JS"; exit 1; }
mkdir -p "$FIGMA_OUT"

node - "$DESIGN_DIR" "$JOBS" <<'NODE'
import { readdirSync, writeFileSync } from 'node:fs';
const [design, jobsPath] = process.argv.slice(2);
const areas = [
  { area: 'web',     dir: `${design}/screens/web`,     width: 1440, mobile: false },
  { area: 'mobile',  dir: `${design}/screens/mobile`,  width: 390,  mobile: true  },
  { area: 'desktop', dir: `${design}/screens/desktop`, width: 1440, mobile: false },
  { area: 'tui',     dir: `${design}/screens/tui`,     width: 1200, mobile: false },
  { area: 'library', dir: `${design}/library`,         width: 1440, mobile: false, only: ['components.html'] },
];
const jobs = [];
for (const a of areas) {
  let files = readdirSync(a.dir).filter(f => f.endsWith('.html'));
  if (a.only) files = files.filter(f => a.only.includes(f));
  files.sort();
  for (const f of files) jobs.push({ url: `file://${a.dir}/${f}`, name: `${a.area}-${f.replace(/\.html$/,'')}`, width: a.width, mobile: a.mobile });
}
writeFileSync(jobsPath, JSON.stringify(jobs, null, 2));
console.error(`figma jobs: ${jobs.length}`);
NODE

node "$SCRIPT_DIR/capture-figma.mjs" "$CAPTURE_JS" "$FIGMA_OUT" "$JOBS"
