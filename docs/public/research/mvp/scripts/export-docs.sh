#!/usr/bin/env bash
#
# export-docs.sh — Markdown -> HTML + PDF export pipeline for the Helix
# Thready public documentation tree (docs/public/research/mvp/).
#
# Satisfies:
#   [CONSTITUTION §11.4.65] every non-source .md has PDF + HTML siblings,
#                           kept in sync (this script IS the export step).
#   [CONSTITUTION §11.4.106] Docs Chain — the .docs_chain/contexts/*.yaml
#                           config drives this same script as exec transforms.
#
# Per file, for EVERY *.md under docs/public/research/mvp/:
#   a. PREPROCESS mermaid: each ```mermaid fenced block is extracted to a
#      temp .mmd, rendered with mmdc -> .svg, and inlined into the HTML as a
#      <figure class="mermaid-figure"><svg .../></figure>. If a block fails
#      to render it is left as a plain fenced code block (honest fallback —
#      never faked, never aborts the run).
#   b. md -> standalone HTML5 via pandoc (--toc --standalone --css <rel>
#      --metadata title=<H1>). The CSS path is computed per file depth.
#   c. HTML -> PDF via weasyprint.
#   d. Writes <name>.html and <name>.pdf next to each <name>.md.
#
# Idempotent: a file is skipped when its .html AND .pdf siblings already
# exist and are newer than the .md (and newer than doc.css + this script).
# Set FORCE=1 to re-generate unconditionally.
#
# Per-file error isolation: one file failing never aborts the whole run.
#
# ── Usage ─────────────────────────────────────────────────────────────
#   scripts/export-docs.sh                     # bulk: whole mvp/ tree
#   FORCE=1 scripts/export-docs.sh             # bulk, ignore freshness
#   scripts/export-docs.sh <in.md> <out.html> --mode md2html   # single
#   scripts/export-docs.sh <in.html> <out.pdf> --mode html2pdf # single
#
# The two single-file modes are the exec-transform entry points used by the
# Docs Chain engine (.docs_chain/contexts/thready-docs.yaml): the engine
# invokes `<exec> <input> <output> <args…>`, so `--mode <x>` arrives after
# the input/output paths. Both modes run the SAME code paths as the bulk
# run, so exec-driven and bulk-driven output are identical.
#
# ── Tooling (override via env) ────────────────────────────────────────
#   PANDOC      default: /home/milos/Factory/software/pandoc/bin/pandoc
#   WEASYPRINT  default: /home/milos/Factory/software/weasyprint/bin/weasyprint
#   MMDC        default: mmdc (on PATH)
#
set -euo pipefail

# ── Resolve locations ─────────────────────────────────────────────────
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
MVP_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
ASSETS_CSS="$MVP_ROOT/assets/doc.css"

PANDOC="${PANDOC:-/home/milos/Factory/software/pandoc/bin/pandoc}"
WEASYPRINT="${WEASYPRINT:-/home/milos/Factory/software/weasyprint/bin/weasyprint}"
if [ -n "${MMDC:-}" ]; then
  MMDC_BIN=("$MMDC")
elif command -v mmdc >/dev/null 2>&1; then
  MMDC_BIN=(mmdc)
elif command -v npx >/dev/null 2>&1; then
  MMDC_BIN=(npx -y @mermaid-js/mermaid-cli)
else
  MMDC_BIN=()
fi

# ── Working dir (temp, auto-cleaned) ──────────────────────────────────
WORK="$(mktemp -d "${TMPDIR:-/tmp}/helix-export.XXXXXX")"
cleanup() { rm -rf "$WORK"; }
trap cleanup EXIT

# ── Mermaid render config: system Chromium, sandbox-safe; text labels ──
# htmlLabels:false forces <text> labels (real SVG) instead of <foreignObject>
# HTML, which WeasyPrint cannot rasterise — so diagrams render in the PDF too.
PCFG="$WORK/puppeteer.json"
CHROME="${PUPPETEER_EXECUTABLE_PATH:-}"
if [ -z "$CHROME" ]; then
  for c in chromium chromium-browser google-chrome google-chrome-stable; do
    if command -v "$c" >/dev/null 2>&1; then CHROME="$(command -v "$c")"; break; fi
  done
fi
if [ -n "$CHROME" ]; then
  printf '{"executablePath":"%s","args":["--no-sandbox","--disable-gpu"]}\n' "$CHROME" > "$PCFG"
else
  printf '{"args":["--no-sandbox","--disable-gpu"]}\n' > "$PCFG"
fi
MMCFG="$WORK/mermaid.json"
printf '{"theme":"neutral","htmlLabels":false,"flowchart":{"htmlLabels":false,"useMaxWidth":true},"class":{"htmlLabels":false},"er":{"useMaxWidth":true},"sequence":{"useMaxWidth":true}}\n' > "$MMCFG"

# ── Running header/footer partial (injected before body; §doc.css) ────
RUNNING_HTML="$WORK/running.html"
{
  printf '<div class="rh-header">Helix Thready — Helix Development</div>\n'
  printf '<div class="rh-footer">Made with love <span class="heart">&#9829;</span> by Helix Development</div>\n'
} > "$RUNNING_HTML"

# ── Helpers ───────────────────────────────────────────────────────────
log()  { printf '%s\n' "$*"; }
warn() { printf '%s\n' "$*" >&2; }

# Relative path from a doc's final location to assets/doc.css.
css_relpath_for() {  # $1 = absolute md (or html) path
  local f rel dir depth prefix i
  f="$1"
  rel="${f#"$MVP_ROOT"/}"
  dir="$(dirname "$rel")"
  if [ "$dir" = "." ]; then depth=0; else depth="$(awk -F/ '{print NF}' <<<"$dir")"; fi
  prefix=""
  for ((i=0; i<depth; i++)); do prefix+="../"; done
  printf '%sassets/doc.css' "$prefix"
}

# Document title = text of the first top-level `# ` heading, else filename.
doc_title() {  # $1 = md path
  local t
  t="$(awk '
    /^[[:space:]]*```/ { infence = !infence }
    !infence && /^# / { sub(/^# +/, ""); print; exit }
  ' "$1")"
  if [ -z "$t" ]; then
    t="$(basename "${1%.md}")"
  fi
  printf '%s' "$t"
}

# Strip everything before the first <svg and drop blank lines, so the inline
# SVG is a single contiguous raw-HTML block that pandoc passes through cleanly.
clean_svg() {  # $1 = svg file  -> stdout
  python3 - "$1" <<'PY'
import sys
d = open(sys.argv[1], encoding="utf-8", errors="replace").read()
i = d.find("<svg")
d = d[i:] if i >= 0 else d
sys.stdout.write("".join(l for l in d.splitlines(keepends=True) if l.strip()))
PY
}

# ── Mermaid preprocess: md -> md with inline SVG figures (or fenced fallback)
preprocess_md() {  # $1 = src md, $2 = dest preprocessed md ; echoes "rendered/total"
  local src="$1" dest="$2"
  local pph="$WORK/pre.$$.md"

  # Clear any per-file scratch from a previous doc so injection never leaks
  # a stale SVG across files in a bulk run.
  rm -f "$WORK"/block_*.mmd "$WORK"/block_*.svg "$WORK"/block_*.snippet \
        "$WORK"/inject_*.svg "$WORK/COUNT"

  # Pass 1: pull mermaid blocks out to $WORK/block_N.mmd, drop the doc H1,
  # leave a @@HELIX_MERMAID_N@@ placeholder in their place.
  awk -v workdir="$WORK" '
    BEGIN { n=0; inmermaid=0; infence=0; stripped=0 }
    !inmermaid && !infence && /^[[:space:]]*```[ ]*mermaid[[:space:]]*$/ {
      n++; blockfile = workdir "/block_" n ".mmd"; printf "" > blockfile; close(blockfile);
      inmermaid=1; next
    }
    inmermaid && /^[[:space:]]*```[[:space:]]*$/ {
      inmermaid=0; print "@@HELIX_MERMAID_" n "@@"; next
    }
    inmermaid { print $0 >> (workdir "/block_" n ".mmd"); next }
    /^[[:space:]]*```/ { infence = !infence; print $0; next }
    !infence && !stripped && /^# / { stripped=1; next }
    { print $0 }
    END { print n+0 > (workdir "/COUNT") }
  ' "$src" > "$pph"

  local count; count="$(cat "$WORK/COUNT" 2>/dev/null || echo 0)"
  local rendered=0 i
  for ((i=1; i<=count; i++)); do
    local mmd="$WORK/block_$i.mmd" svg="$WORK/block_$i.svg" snip="$WORK/block_$i.snippet"
    local ok=0
    if [ "${#MMDC_BIN[@]}" -gt 0 ] && [ -s "$mmd" ]; then
      if "${MMDC_BIN[@]}" -i "$mmd" -o "$svg" -p "$PCFG" -c "$MMCFG" -b white \
           >>"$WORK/mmdc.log" 2>&1 && [ -s "$svg" ]; then
        ok=1
      fi
    fi
    if [ "$ok" -eq 1 ]; then
      # Stash the cleaned SVG and leave only a tiny placeholder div in the
      # markdown. The real SVG is injected into the HTML *after* pandoc (see
      # md2html), so pandoc never parses SVG content — no $-math / emphasis /
      # link corruption of the diagram, and no fragile raw-HTML-block edge cases.
      clean_svg "$svg" > "$WORK/inject_$i.svg"
      printf '\n<div class="mermaid-placeholder" data-mmd="%s"></div>\n' "$i" > "$snip"
      rendered=$((rendered + 1))
    else
      # Honest fallback: keep the original diagram as a fenced code block.
      { printf '\n```mermaid\n'; cat "$mmd"; printf '\n```\n'; } > "$snip"
    fi
  done

  # Pass 2: splice each snippet back in place of its placeholder.
  awk -v workdir="$WORK" '
    /^@@HELIX_MERMAID_[0-9]+@@[[:space:]]*$/ {
      m=$0; gsub(/[^0-9]/,"",m); snip = workdir "/block_" m ".snippet";
      while ((getline line < snip) > 0) print line; close(snip); next
    }
    { print }
  ' "$pph" > "$dest"

  rm -f "$pph"
  printf '%s/%s' "$rendered" "$count"
}

# ── Stage b: md -> html ───────────────────────────────────────────────
md2html() {  # $1 = src md, $2 = out html ; echoes "rendered/total" of diagrams
  local src="$1" out="$2"
  local pre="$WORK/final.$$.md"
  local ratio; ratio="$(preprocess_md "$src" "$pre")"
  local title cssrel
  title="$(doc_title "$src")"
  cssrel="$(css_relpath_for "$src")"

  local tmp="$out.tmp.$$"
  # tex_math_dollars is DISABLED: this corpus is shell/SQL/config heavy, so a
  # bare `$` means a shell var or literal — never inline math. Leaving it on
  # mis-pairs stray `$` in prose (and in any passthrough) into math spans.
  "$PANDOC" "$pre" \
    --from=markdown-tex_math_dollars \
    --to=html5 \
    --standalone \
    --toc --toc-depth=3 \
    --css "$cssrel" \
    --metadata title="$title" \
    --metadata lang=en \
    --include-before-body "$RUNNING_HTML" \
    --syntax-highlighting=tango \
    -o "$tmp"

  # Inject the rendered mermaid SVGs into the HTML, replacing the placeholder
  # divs pandoc passed through verbatim. Done post-pandoc so SVG content is
  # never markdown-parsed.
  python3 - "$tmp" "$WORK" <<'PY'
import sys, os, re, glob
html_path, work = sys.argv[1], sys.argv[2]
with open(html_path, encoding="utf-8") as fh:
    html = fh.read()
for svgf in sorted(glob.glob(os.path.join(work, "inject_*.svg"))):
    n = re.search(r"inject_(\d+)\.svg$", svgf).group(1)
    with open(svgf, encoding="utf-8") as fh:
        svg = fh.read()
    figure = '<figure class="mermaid-figure">\n' + svg + '\n</figure>'
    # pandoc re-emits the raw <div> as a native (multi-line) Div, so match the
    # whole empty placeholder div by its data-mmd marker, not an exact string.
    pat = re.compile(r'<div\b[^>]*\bdata-mmd="%s"[^>]*>.*?</div>' % re.escape(n),
                     re.DOTALL)
    html = pat.sub(lambda m: figure, html, count=1)
with open(html_path, "w", encoding="utf-8") as fh:
    fh.write(html)
PY

  mv -f "$tmp" "$out"
  rm -f "$pre"
  printf '%s' "$ratio"
}

# ── Stage c: html -> pdf ──────────────────────────────────────────────
html2pdf() {  # $1 = src html, $2 = out pdf
  local src="$1" out="$2"
  local tmp="$out.tmp.$$"
  "$WEASYPRINT" "$src" "$tmp" >>"$WORK/weasyprint.log" 2>&1
  mv -f "$tmp" "$out"
}

# ── Full pipeline for one md (bulk mode) ──────────────────────────────
process_one() {  # $1 = md path ; returns 0 ok/skip, 1 fail
  local md="$1"
  local html="${md%.md}.html" pdf="${md%.md}.pdf"

  # Idempotent freshness skip (unless FORCE=1).
  if [ "${FORCE:-0}" != "1" ] && [ -f "$html" ] && [ -f "$pdf" ] \
     && [ "$html" -nt "$md" ] && [ "$pdf" -nt "$md" ] \
     && [ "$html" -nt "$ASSETS_CSS" ] && [ "$html" -nt "${BASH_SOURCE[0]}" ]; then
    log "SKIP  ${md#"$MVP_ROOT"/}  (up to date)"
    return 0
  fi

  local ratio
  if ! ratio="$(md2html "$md" "$html")"; then
    warn "FAIL  ${md#"$MVP_ROOT"/}  (pandoc/md2html)"
    return 1
  fi
  if ! html2pdf "$html" "$pdf"; then
    warn "FAIL  ${md#"$MVP_ROOT"/}  (weasyprint/html2pdf)"
    return 1
  fi
  # Track diagram-render shortfalls without failing the file.
  local rendered="${ratio%/*}" total="${ratio#*/}"
  if [ "${total:-0}" -gt 0 ] && [ "$rendered" != "$total" ]; then
    log "OK*   ${md#"$MVP_ROOT"/}  (diagrams ${ratio} rendered; rest kept as fenced source)"
    printf '%s\n' "${md#"$MVP_ROOT"/} (${ratio} diagrams)" >> "$WORK/partial-diagrams.log"
  else
    log "OK    ${md#"$MVP_ROOT"/}  (diagrams ${ratio})"
  fi
  return 0
}

# ══════════════════════════════════════════════════════════════════════
# Argument handling: exec single-file mode vs bulk mode
# ══════════════════════════════════════════════════════════════════════
MODE=""
POS=()
while [ $# -gt 0 ]; do
  case "$1" in
    --mode) MODE="${2:-}"; shift 2 ;;
    --mode=*) MODE="${1#--mode=}"; shift ;;
    -h|--help) sed -n '2,40p' "${BASH_SOURCE[0]}"; exit 0 ;;
    *) POS+=("$1"); shift ;;
  esac
done

# Preflight tool check.
[ -x "$PANDOC" ] || command -v "$PANDOC" >/dev/null 2>&1 || { warn "ERROR: pandoc not found ($PANDOC)"; exit 127; }
[ -x "$WEASYPRINT" ] || command -v "$WEASYPRINT" >/dev/null 2>&1 || { warn "ERROR: weasyprint not found ($WEASYPRINT)"; exit 127; }
[ "${#MMDC_BIN[@]}" -gt 0 ] || warn "WARN: mmdc not found — mermaid blocks will fall back to fenced source."

if [ -n "$MODE" ]; then
  # ── Single-file exec mode (Docs Chain transform entry point) ────────
  in="$(realpath "${POS[0]:?input path required}")"
  out="${POS[1]:?output path required}"
  case "$MODE" in
    md2html)  md2html  "$in" "$out" >/dev/null ;;
    html2pdf) html2pdf "$in" "$out" ;;
    *) warn "ERROR: unknown --mode '$MODE' (expected md2html|html2pdf)"; exit 2 ;;
  esac
  exit 0
fi

# ── Bulk mode: whole tree ─────────────────────────────────────────────
total=0 ok=0 fail=0 skip=0
declare -a FAILED=()
while IFS= read -r -d '' md; do
  total=$((total + 1))
  # process_one runs inside an `if` so set -e is suspended within it →
  # a failure in one file cannot abort the run (per-file isolation).
  before_skip=0
  if [ "${FORCE:-0}" != "1" ] && [ -f "${md%.md}.html" ] && [ -f "${md%.md}.pdf" ] \
     && [ "${md%.md}.html" -nt "$md" ] && [ "${md%.md}.pdf" -nt "$md" ] \
     && [ "${md%.md}.html" -nt "$ASSETS_CSS" ] && [ "${md%.md}.html" -nt "${BASH_SOURCE[0]}" ]; then
    before_skip=1
  fi
  if process_one "$md"; then
    if [ "$before_skip" -eq 1 ]; then skip=$((skip + 1)); else ok=$((ok + 1)); fi
  else
    fail=$((fail + 1)); FAILED+=("${md#"$MVP_ROOT"/}")
  fi
done < <(find "$MVP_ROOT" -type f -name '*.md' -print0 | sort -z)

echo "────────────────────────────────────────────────────────────────────"
echo "Docs export: ${total} markdown file(s) — ${ok} generated, ${skip} skipped (fresh), ${fail} failed."
if [ -f "$WORK/partial-diagrams.log" ]; then
  echo "Files with partial diagram render (rest kept as honest fenced source):"
  sed 's/^/  - /' "$WORK/partial-diagrams.log"
fi
if [ "$fail" -gt 0 ]; then
  echo "Failed files:"
  printf '  - %s\n' "${FAILED[@]}"
  exit 1
fi
echo "All markdown exported to HTML + PDF siblings. [CONSTITUTION §11.4.65]"
