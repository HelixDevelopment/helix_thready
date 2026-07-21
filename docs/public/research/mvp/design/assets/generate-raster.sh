#!/usr/bin/env bash
# =============================================================================
# Helix Thready — brand/launcher raster generator
# Location: docs/public/research/mvp/design/assets/generate-raster.sh
#
# Rasterizes the hand-authored master SVGs in THIS directory to every size in
# icon-export-matrix.md (Android mipmap + adaptive + monochrome, iOS AppIcon
# incl. dark/tinted, Windows .ico, macOS .icns components, Linux hicolor,
# favicon, PWA + maskable, HarmonyOS layered components, Aurora).
#
# HONEST-OUTPUT CONTRACT (matches the docs' "no bluff" rule):
#   * Detects a REAL SVG rasterizer (rsvg-convert > inkscape > resvg > a
#     Chromium/Chrome headless > ImageMagick) and SELF-TESTS it with a probe
#     render before use. If none renders, it prints a SKIP note and exits 0
#     WITHOUT writing any placeholder/fake PNGs.
#   * ImageMagick is accepted ONLY if its SVG path actually works here: on a box
#     with no librsvg/rsvg-convert delegate, IM's internal MSVG coder fails on
#     SVGs that use <text>/CSS/currentColor, so the probe will reject it and the
#     script falls through to the next renderer instead of emitting garbage.
#   * .ico / .icns BUNDLING needs extra tools (icotool|magick / png2icns|iconutil).
#     If absent, the per-size PNG *components* are still written and the bundling
#     step is SKIPPED with a note — never faked.
#
# Usage:  ./generate-raster.sh [OUTPUT_DIR]      (default: ./raster)
# =============================================================================
set -euo pipefail

SRC_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
OUT_DIR="${1:-$SRC_DIR/raster}"
TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT

# Master SVGs (must exist beside this script) --------------------------------
MASTER="$SRC_DIR/launcher-icon.svg"        # neutral gradient master (opaque icon body)
LIGHT="$SRC_DIR/launcher-icon-light.svg"   # light-surface variant
DARK="$SRC_DIR/launcher-icon-dark.svg"     # dark-surface variant (iOS 18 dark)
MONO="$SRC_DIR/launcher-icon-mono.svg"     # single-tone (Android monochrome / iOS tinted)
LOGO_FULL="$SRC_DIR/logo-full.svg"
LOGO_MARK="$SRC_DIR/logo-mark.svg"
FOOTER="$SRC_DIR/footer-slogan.svg"

note()  { printf '  \033[33m[note]\033[0m %s\n' "$*" >&2; }
info()  { printf '  \033[36m[..]\033[0m  %s\n'  "$*" >&2; }
ok()    { printf '  \033[32m[ok]\033[0m  %s\n'  "$*" >&2; }
die()   { printf '\033[31m[FAIL]\033[0m %s\n'   "$*" >&2; exit 1; }

for f in "$MASTER" "$LIGHT" "$DARK" "$MONO"; do
  [ -f "$f" ] || die "missing source SVG: $f"
done

# --- Renderer detection -------------------------------------------------------
# Each backend implements: r_<name> <in.svg> <out.png> <w> <h>
CHROME_BIN=""
for c in chromium chromium-browser google-chrome google-chrome-stable chrome; do
  command -v "$c" >/dev/null 2>&1 && { CHROME_BIN="$(command -v "$c")"; break; }
done
MAGICK_BIN=""
for m in magick convert; do
  command -v "$m" >/dev/null 2>&1 && { MAGICK_BIN="$(command -v "$m")"; break; }
done

r_rsvg()     { rsvg-convert -w "$3" -h "$4" -o "$2" "$1"; }
r_inkscape() { inkscape "$1" -w "$3" -h "$4" -o "$2" >/dev/null 2>&1; }
r_resvg()    { resvg -w "$3" -h "$4" "$1" "$2" >/dev/null 2>&1; }
r_magick()   { "$MAGICK_BIN" -background none "$1" -resize "${3}x${4}" -gravity center -extent "${3}x${4}" "$2" >/dev/null 2>&1; }

# Headless Chromium renders BLANK (fully transparent) below a minimum window size
# (~128px here). So the chromium backend NEVER screenshots at a sub-CHROME_MIN size:
# it supersamples the SVG into a window whose smallest side is >= CHROME_MIN, then
# downscales to the exact target with ImageMagick. This also gives cleaner anti-aliasing
# on small icons. A sub-CHROME_MIN target therefore REQUIRES a downscaler (magick); if
# none is present the backend fails its probe and the script falls through / SKIPs
# (it never writes a blank/faked PNG).
CHROME_MIN=512
r_chromium() { # inline the SVG into a transparent HTML wrapper, screenshot >=CHROME_MIN, downscale
  local in="$1" out="$2" w="$3" h="$4" tmp="$TMP_DIR/w.html" big="$TMP_DIR/big.png"
  local f=1 fw=1 fh=1
  [ "$w" -lt "$CHROME_MIN" ] && fw=$(( (CHROME_MIN + w - 1) / w ))
  [ "$h" -lt "$CHROME_MIN" ] && fh=$(( (CHROME_MIN + h - 1) / h ))
  f=$fw; [ "$fh" -gt "$f" ] && f=$fh          # single integer factor keeps the aspect exact
  local iw=$(( w * f )) ih=$(( h * f ))
  { printf '<!doctype html><meta charset=utf-8><style>html,body{margin:0;padding:0;background:transparent}#b{width:%spx;height:%spx}#b svg{width:100%%;height:100%%;display:block}</style><div id=b>' "$iw" "$ih"
    sed -e 's/<?xml[^>]*?>//' -e 's/ width="[0-9]*"//' -e 's/ height="[0-9]*"//' "$in"
    printf '</div>'; } > "$tmp"
  "$CHROME_BIN" --headless=new --no-sandbox --disable-gpu --hide-scrollbars \
    --default-background-color=00000000 --force-device-scale-factor=1 \
    --screenshot="$big" --window-size="$iw,$ih" "file://$tmp" >/dev/null 2>&1 || return 1
  [ -s "$big" ] || return 1
  if [ "$f" -eq 1 ]; then
    mv "$big" "$out"
  elif [ -n "$MAGICK_BIN" ]; then
    "$MAGICK_BIN" "$big" -background none -filter Lanczos -resize "${w}x${h}!" "$out" >/dev/null 2>&1 || return 1
  else
    return 1   # sub-CHROME_MIN target needs a downscaler; probe() will have gated this out
  fi
  [ -s "$out" ] || return 1
}

# Probe a candidate: render the master to 32px and confirm a valid, NON-BLANK PNG appears.
# The blank check matters: headless Chromium happily writes a fully-transparent 32x32 PNG
# when asked for a sub-minimum window, which is the right dimensions but empty art. Without
# this check a broken renderer would be selected and every small icon would be faked-blank.
probe() {
  local fn="$1" p="$TMP_DIR/probe.png"
  rm -f "$p"
  "$fn" "$MASTER" "$p" 32 32 >/dev/null 2>&1 || return 1
  [ -s "$p" ] || return 1
  if [ -n "$MAGICK_BIN" ]; then
    local dim colors
    dim="$("$MAGICK_BIN" identify -format '%wx%h' "$p" 2>/dev/null || true)"
    [ "$dim" = "32x32" ] || return 1
    colors="$("$MAGICK_BIN" identify -format '%k' "$p" 2>/dev/null || echo 0)"
    case "$colors" in ''|*[!0-9]*) return 1 ;; esac
    [ "$colors" -gt 1 ] || return 1        # reject a blank / single-colour (empty) render
  fi
  return 0
}

RENDER=""; RENDER_NAME=""
select_renderer() {
  # ordered candidate list: "label|func|dependency"
  local list=(
    "rsvg-convert|r_rsvg|rsvg-convert"
    "inkscape|r_inkscape|inkscape"
    "resvg|r_resvg|resvg"
    "chromium|r_chromium|${CHROME_BIN}"
    "imagemagick|r_magick|${MAGICK_BIN}"
  )
  for entry in "${list[@]}"; do
    IFS='|' read -r label fn dep <<<"$entry"
    [ -n "$dep" ] || continue
    case "$label" in
      chromium|imagemagick) : ;;                       # dep is a resolved path
      *) command -v "$dep" >/dev/null 2>&1 || continue ;;
    esac
    if probe "$fn"; then RENDER="$fn"; RENDER_NAME="$label"; return 0; fi
    note "$label present but failed a probe render — skipping it."
  done
  return 1
}

echo "== Helix Thready raster generator =="
if ! select_renderer; then
  cat >&2 <<'EOF'

  [SKIP] No working SVG rasterizer found.
         Install ONE of: rsvg-convert (librsvg), inkscape, resvg, a Chromium/Chrome,
         or an ImageMagick built with a working SVG (librsvg) delegate.
         No PNGs were written (output is not faked).
EOF
  exit 0
fi
ok "renderer: $RENDER_NAME"

# render <src.svg> <out.png> <size>   (square)
render() { local s="$1" o="$2" px="$3"; mkdir -p "$(dirname "$o")"; "$RENDER" "$s" "$o" "$px" "$px" || die "render failed: $o"; }
# render_wh <src.svg> <out.png> <w> <h>
render_wh() { local s="$1" o="$2" w="$3" h="$4"; mkdir -p "$(dirname "$o")"; "$RENDER" "$s" "$o" "$w" "$h" || die "render failed: $o"; }

mkdir -p "$OUT_DIR"

# --- Bundler detection (ICO / ICNS) ------------------------------------------
ICO_TOOL=""; command -v icotool >/dev/null 2>&1 && ICO_TOOL="icotool" || { [ -n "$MAGICK_BIN" ] && ICO_TOOL="magick"; }
ICNS_TOOL=""; command -v png2icns >/dev/null 2>&1 && ICNS_TOOL="png2icns" || { command -v iconutil >/dev/null 2>&1 && ICNS_TOOL="iconutil"; }

make_ico() { # <out.ico> <png...>
  local out="$1"; shift
  case "$ICO_TOOL" in
    icotool) icotool -c -o "$out" "$@" ;;
    magick)  "$MAGICK_BIN" "$@" "$out" ;;
    *) note "no ICO bundler (icotool / ImageMagick) — left PNG components, skipped $out"; return 0 ;;
  esac && ok "ico: $out"
}

# =============================================================================
# WEB / PWA
# =============================================================================
info "web / favicon / PWA"
WEB="$OUT_DIR/web"
mkdir -p "$WEB"
cp "$MASTER" "$WEB/favicon.svg"
for s in 16 32 48; do render "$MASTER" "$WEB/favicon-$s.png" "$s"; done
make_ico "$WEB/favicon.ico" "$WEB/favicon-16.png" "$WEB/favicon-32.png" "$WEB/favicon-48.png"
render "$MASTER" "$WEB/icon-192.png" 192
render "$MASTER" "$WEB/icon-512.png" 512
render "$MASTER" "$WEB/maskable-192.png" 192   # art is within the Ø916 maskable safe zone (§3.1)
render "$MASTER" "$WEB/maskable-512.png" 512
render "$MASTER" "$WEB/apple-touch-icon.png" 180
ok "web set -> $WEB"

# =============================================================================
# ANDROID  (mipmap densities + adaptive 108dp + Android-13 monochrome + Play)
# =============================================================================
info "android mipmap / adaptive / monochrome"
AND="$OUT_DIR/android"
# legacy launcher png per density: mdpi/hdpi/xhdpi/xxhdpi/xxxhdpi
declare -A MIP=( [mdpi]=48 [hdpi]=72 [xhdpi]=96 [xxhdpi]=144 [xxxhdpi]=192 )
for d in "${!MIP[@]}"; do render "$MASTER" "$AND/mipmap-$d/ic_launcher.png" "${MIP[$d]}"; done
# adaptive foreground + monochrome at 108dp (px = 108 * density)
declare -A ADP=( [mdpi]=108 [hdpi]=162 [xhdpi]=216 [xxhdpi]=324 [xxxhdpi]=432 )
for d in "${!ADP[@]}"; do
  render "$MASTER" "$AND/mipmap-$d/ic_launcher_foreground.png" "${ADP[$d]}"
  render "$MONO"   "$AND/mipmap-$d/ic_launcher_monochrome.png" "${ADP[$d]}"
done
render "$MASTER" "$AND/play-store-512.png" 512
ok "android set -> $AND"

# =============================================================================
# iOS  AppIcon.appiconset  (modern 1024 + dark + tinted, plus legacy @1x/@2x/@3x)
# =============================================================================
info "iOS AppIcon (light / dark / tinted + legacy set)"
IOS="$OUT_DIR/ios/AppIcon.appiconset"
render "$MASTER" "$IOS/icon-1024.png"        1024
render "$DARK"   "$IOS/icon-1024-dark.png"   1024
render "$MONO"   "$IOS/icon-1024-tinted.png" 1024
# legacy px set (pt@scale): 20/29/40/60 @2x@3x, 76 @1x@2x, 83.5 @2x, plus @1x settings
for s in 20 29 40 58 60 76 80 87 120 152 167 180; do render "$MASTER" "$IOS/icon-${s}.png" "$s"; done
ok "iOS set -> $IOS"

# =============================================================================
# Windows  (.ico multi-resolution)
# =============================================================================
info "windows .ico"
WIN="$OUT_DIR/windows"
WIN_PNGS=()
for s in 16 24 32 48 64 128 256; do render "$MASTER" "$WIN/icon-$s.png" "$s"; WIN_PNGS+=("$WIN/icon-$s.png"); done
make_ico "$WIN/thready.ico" "${WIN_PNGS[@]}"
ok "windows set -> $WIN"

# =============================================================================
# macOS  (.icns components + bundle if a packer exists)
# =============================================================================
info "macOS .icns components"
MAC="$OUT_DIR/macos"
ICONSET="$MAC/thready.iconset"
# Apple iconset naming (unique px: 16 32 64 128 256 512 1024)
render "$MASTER" "$ICONSET/icon_16x16.png"        16
render "$MASTER" "$ICONSET/icon_16x16@2x.png"     32
render "$MASTER" "$ICONSET/icon_32x32.png"        32
render "$MASTER" "$ICONSET/icon_32x32@2x.png"     64
render "$MASTER" "$ICONSET/icon_128x128.png"      128
render "$MASTER" "$ICONSET/icon_128x128@2x.png"   256
render "$MASTER" "$ICONSET/icon_256x256.png"      256
render "$MASTER" "$ICONSET/icon_256x256@2x.png"   512
render "$MASTER" "$ICONSET/icon_512x512.png"      512
render "$MASTER" "$ICONSET/icon_512x512@2x.png"   1024
case "$ICNS_TOOL" in
  png2icns) png2icns "$MAC/thready.icns" \
              "$ICONSET/icon_16x16.png" "$ICONSET/icon_32x32.png" "$ICONSET/icon_128x128.png" \
              "$ICONSET/icon_256x256.png" "$ICONSET/icon_512x512.png" "$ICONSET/icon_512x512@2x.png" \
              >/dev/null 2>&1 && ok "icns: $MAC/thready.icns" ;;
  iconutil) iconutil -c icns -o "$MAC/thready.icns" "$ICONSET" && ok "icns: $MAC/thready.icns" ;;
  *) note "no .icns packer (png2icns / iconutil) — wrote $ICONSET/*.png; run 'iconutil -c icns $ICONSET' on macOS." ;;
esac
ok "macOS components -> $ICONSET"

# =============================================================================
# Linux  (hicolor theme PNGs + scalable SVG)
# =============================================================================
info "linux hicolor"
LIN="$OUT_DIR/linux/hicolor"
for s in 16 22 24 32 48 64 128 256 512; do
  render "$MASTER" "$LIN/${s}x${s}/apps/thready.png" "$s"
done
mkdir -p "$LIN/scalable/apps"; cp "$MASTER" "$LIN/scalable/apps/thready.svg"
ok "linux set -> $LIN"

# =============================================================================
# HarmonyOS (layered components) + Aurora  — [GAP: 8.5] native clients are
# scaffolds; these assets are produced now, but no HarmonyOS/Aurora launcher
# is claimed to ship. Sizes marked [RESEARCH] in icon-export-matrix.md.
# =============================================================================
info "HarmonyOS layered components + Aurora [GAP 8.5 / RESEARCH]"
HAR="$OUT_DIR/harmonyos"
render "$MASTER" "$HAR/foreground.png" 216
render "$MASTER" "$HAR/background.png" 216      # solid/branded bg supplied at integration
render "$MASTER" "$HAR/appgallery-1024.png" 1024
AUR="$OUT_DIR/aurora"
for s in 86 108 128 172 250; do render "$MASTER" "$AUR/thready-$s.png" "$s"; done
note "HarmonyOS/Aurora sizes are [RESEARCH]; re-verify against current OS packaging docs (THREADY-DES-05)."

# =============================================================================
# Brand lockups (web PNGs) — bonus, from the wordmark / mark / slogan SVGs
# =============================================================================
info "brand lockups (raster convenience copies)"
BR="$OUT_DIR/brand"
if [ -f "$LOGO_FULL" ]; then for h in 64 128; do render_wh "$LOGO_FULL" "$BR/logo-full-h${h}.png" "$(( h*1060/400 ))" "$h"; done; fi
if [ -f "$LOGO_MARK" ]; then for s in 128 256 512; do render "$LOGO_MARK" "$BR/logo-mark-$s.png" "$s"; done; fi
if [ -f "$FOOTER"   ]; then render_wh "$FOOTER" "$BR/footer-slogan-h44.png" "$(( 44*560/44 ))" 44; fi
ok "brand lockups -> $BR"

echo
ok "done. renderer=$RENDER_NAME  ico=${ICO_TOOL:-none}  icns=${ICNS_TOOL:-none}  out=$OUT_DIR"
[ -n "$ICO_TOOL" ]  || note "ICO bundling was skipped (no icotool/ImageMagick) — PNG components remain."
[ -n "$ICNS_TOOL" ] || note "ICNS bundling was skipped (no png2icns/iconutil) — .iconset PNGs remain."
