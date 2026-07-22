#!/usr/bin/env bash
# build-design-book.sh — assemble the Helix Thready Design Book PDF.
# Cover (inline logo + slogan SVG) · token/palette page · every screen PNG
# full-page with an "area · screen · theme" caption · component library ·
# motion spec. Requires the PNGs from screenshot-designs.sh under exports/png/.
#
# Output:  exports/design-book.pdf   (verified > 1 MB, pages > #screens)
# Usage:   bash build-design-book.sh
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DESIGN_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
PNG="$DESIGN_DIR/exports/png"
OUT_PDF="$DESIGN_DIR/exports/design-book.pdf"
ASSEMBLE="/tmp/design-book-assemble.html"
SAFE="/tmp/design-book-pdfsafe"          # downscaled copies of any >80MP image (PIL limit)
WEASYPRINT="${WEASYPRINT:-/home/milos/Factory/software/weasyprint/bin/weasyprint}"
[ -d "$PNG" ] || { echo "PNG dir missing — run screenshot-designs.sh first"; exit 1; }
mkdir -p "$SAFE"

# Pillow (WeasyPrint's raster loader) rejects images > 89,478,485 px as a
# "decompression bomb". Pre-shrink any offender into $SAFE and let the builder
# reference that copy. At true 2x no Thready page hits this, but stay robust.
while IFS= read -r f; do
  read w h < <(magick identify -format '%w %h' "$f" 2>/dev/null || echo "0 0")
  if [ $((w*h)) -gt 80000000 ]; then
    rel="${f#$PNG/}"; dst="$SAFE/${rel//\//__}"
    magick "$f" -resize '6000x14000>' "$dst" 2>/dev/null && echo "pdf-safe shrink: $rel"
  fi
done < <(find "$PNG" -name '*@2x.png')

node - "$DESIGN_DIR" "$PNG" "$ASSEMBLE" "$SAFE" <<'NODE'
import { readFileSync, readdirSync, writeFileSync, existsSync } from 'node:fs';
const [design, png, out, safe] = process.argv.slice(2);
const esc = s => String(s).replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;');
const readSvg = p => existsSync(p) ? readFileSync(p,'utf8').replace(/<\?xml[^>]*\?>/,'').trim() : '';
const logo = readSvg(`${design}/assets/logo-full.svg`);
const slogan = readSvg(`${design}/assets/footer-slogan.svg`);
const safePath = (area, file) => { const s = `${safe}/${area}__${file}`; return existsSync(s) ? s : `${png}/${area}/${file}`; };
const light = [['--brand','#b6e376'],['--brand-2','#abddc9'],['--brand-ink','#0a0f04'],['--bg','#ffffff'],['--surface','#ffffff'],['--surface-warm','#f1f5f9'],['--fg','#020817'],['--muted','#475569'],['--border','#e2e8f0'],['--border-strong','#64748b'],['--accent','#446e12'],['--accent-on','#ffffff'],['--success','#166534'],['--warn','#854d0e'],['--danger','#dc2626']];
const dark = [['--brand','#b6e376'],['--brand-2','#b7ebd6'],['--brand-ink','#0a0f04'],['--bg','#020817'],['--surface','#020817'],['--surface-warm','#1e293b'],['--fg','#f8fafc'],['--muted','#94a3b8'],['--border','#1e293b'],['--border-strong','#64748b'],['--accent','#b6e376'],['--accent-on','#0a0f04'],['--success','#16a34a'],['--warn','#eab308'],['--danger','#ef4444']];
const typeScale=[['xs','12px'],['sm','14px'],['base','16px'],['lg','20px'],['xl','24px'],['2xl','32px'],['3xl','48px'],['4xl','64px']];
const spacing=[['1','4px'],['2','8px'],['3','12px'],['4','16px'],['6','24px'],['8','32px'],['12','48px'],['16','64px']];
const motion=[['helix-spinner','spiral-loader','loop','120','60','15','freeze-poster'],['success-check','—','once','90','60','89','final-frame'],['error-cross','—','once','90','60','89','final-frame'],['processing-pulse','—','loop','120','60','30','freeze-poster'],['thread-sync','—','loop','120','60','30','freeze-poster'],['transition-fade-slide','—','once','90','60','89','instant-cut']];
const areaOrder=['web','mobile','desktop','tui','library','motion'];
const areaLabel={web:'Web',mobile:'Mobile',desktop:'Desktop',tui:'TUI',library:'Component Library',motion:'Motion'};
const shots=[];
for(const area of areaOrder){ const dir=`${png}/${area}`; if(!existsSync(dir))continue;
  const files=readdirSync(dir).filter(f=>f.endsWith('@2x.png'));
  const norm=f=>f.replace(/-dark@2x\.png$/,'').replace(/@2x\.png$/,'');
  files.sort((a,b)=>{const na=norm(a),nb=norm(b); if(na!==nb)return na<nb?-1:1; return (/-dark@2x/.test(a)?1:0)-(/-dark@2x/.test(b)?1:0);});
  for(const f of files){ shots.push({area,areaLabel:areaLabel[area],screen:norm(f),theme:/-dark@2x\.png$/.test(f)?'dark':'light',path:safePath(area,f)}); } }
const swatch=([t,h])=>`<div class="sw"><div class="chip" style="background:${h}"></div><div class="lbl"><code>${esc(t)}</code><span>${esc(h)}</span></div></div>`;
const screenPage=s=>`<section class="screen"><img src="file://${s.path}" alt="${esc(s.areaLabel)} ${esc(s.screen)} ${s.theme}"/><div class="cap"><span class="area">${esc(s.areaLabel)}</span><span class="sep">·</span><span class="scr">${esc(s.screen)}</span><span class="sep">·</span><span class="theme theme-${s.theme}">${s.theme}</span></div></section>`;
const html=`<!doctype html><html lang="en"><head><meta charset="utf-8"><title>Helix Thready — Design Book</title><style>
@page{size:A4;margin:10mm} @page :first{margin:0} *{box-sizing:border-box}
html{font-family:"Hanken Grotesk Variable",system-ui,sans-serif;color:#020817}
h1,h2{font-family:"Space Grotesk Variable",system-ui,sans-serif;letter-spacing:-0.01em}
code{font-family:"JetBrains Mono",ui-monospace,monospace}
.cover{height:297mm;width:210mm;display:flex;flex-direction:column;align-items:center;justify-content:center;text-align:center;background:radial-gradient(120% 90% at 50% 0%,#f1f5f9 0%,#ffffff 55%);page-break-after:always;padding:24mm}
.cover .logo svg{width:130mm;height:auto} .cover h1{font-size:30pt;margin:14mm 0 2mm}
.cover .subtitle{color:#475569;font-size:12pt;max-width:150mm} .cover .meta{margin-top:6mm;color:#64748b;font-size:9pt}
.cover .slogan{margin-top:auto} .cover .slogan svg{width:90mm;height:auto}
.cover .rule{width:60mm;height:4px;margin:8mm auto;background:linear-gradient(90deg,#b6e376,#abddc9);border-radius:2px}
.page{page-break-after:always} .page h2{font-size:18pt;margin:0 0 4mm}
.kicker{text-transform:uppercase;letter-spacing:.12em;font-size:8pt;color:#446e12;font-weight:700}
.grid{display:grid;grid-template-columns:repeat(3,1fr);gap:4mm}
.sw{display:flex;align-items:center;gap:3mm} .chip{width:16mm;height:16mm;border-radius:3mm;border:1px solid rgba(0,0,0,.12);flex:none}
.lbl{display:flex;flex-direction:column;line-height:1.3} .lbl code{font-size:9pt} .lbl span{font-size:8pt;color:#64748b}
.darkcard{background:#020817;color:#f8fafc;border-radius:4mm;padding:6mm;margin-top:5mm}
.darkcard .chip{border-color:rgba(255,255,255,.18)} .darkcard .lbl span{color:#94a3b8} .darkcard h2,.darkcard .kicker{color:#b6e376}
.meta-tables{display:flex;gap:10mm;margin-top:6mm} .meta-tables table{border-collapse:collapse;font-size:9pt}
.meta-tables th{text-align:left;color:#446e12;font-size:8pt;text-transform:uppercase;letter-spacing:.08em;padding-bottom:2mm}
.meta-tables td{padding:1mm 6mm 1mm 0;border-top:1px solid #e2e8f0}
.screen{page-break-after:always;height:277mm;display:flex;flex-direction:column;align-items:center;justify-content:center}
.screen img{max-width:190mm;max-height:258mm;object-fit:contain;border:1px solid #e2e8f0;border-radius:2mm}
.cap{margin-top:4mm;font-size:10pt;color:#334155} .cap .area{font-weight:700;color:#020817} .cap .scr{font-family:"JetBrains Mono",monospace}
.cap .sep{color:#94a3b8;margin:0 2mm} .cap .theme{text-transform:uppercase;letter-spacing:.06em;font-size:8pt;padding:.5mm 2mm;border-radius:2mm}
.cap .theme-light{background:#f1f5f9;color:#446e12} .cap .theme-dark{background:#020817;color:#b6e376}
table.motion{border-collapse:collapse;width:100%;font-size:9.5pt;margin-top:4mm}
table.motion th{background:#f1f5f9;text-align:left;padding:2.5mm 3mm;font-size:8pt;text-transform:uppercase;letter-spacing:.06em;color:#446e12}
table.motion td{padding:2.5mm 3mm;border-top:1px solid #e2e8f0}
</style></head><body>
<div class="cover"><div class="logo">${logo||'<h1>Helix Thready</h1>'}</div><div class="rule"></div><h1>Design Book</h1>
<div class="subtitle">Threads-reading companion by Helix Development — every MVP screen, the component library, and the motion system, rendered in light and dark from the self-contained OpenDesign source.</div>
<div class="meta">${shots.length} plates · ${shots.filter(s=>s.theme==='light').length} light + ${shots.filter(s=>s.theme==='dark').length} dark · generated ${new Date().toISOString().slice(0,10)}</div>
<div class="slogan">${slogan}</div></div>
<div class="page"><div class="kicker">OpenDesign brand contract · tokens.css</div><h2>Palette — Light</h2><div class="grid">${light.map(swatch).join('')}</div>
<div class="darkcard"><div class="kicker">Dark rebind — color-bearing tokens only</div><h2>Palette — Dark</h2><div class="grid">${dark.map(swatch).join('')}</div></div>
<div class="meta-tables"><table><tr><th>Type scale</th><th></th></tr>${typeScale.map(([k,v])=>`<tr><td><code>--text-${k}</code></td><td>${v}</td></tr>`).join('')}</table>
<table><tr><th>Spacing</th><th></th></tr>${spacing.map(([k,v])=>`<tr><td><code>--space-${k}</code></td><td>${v}</td></tr>`).join('')}</table>
<table><tr><th>Type families</th><th></th></tr><tr><td>Display</td><td>Space Grotesk</td></tr><tr><td>Body</td><td>Hanken Grotesk</td></tr><tr><td>Mono</td><td>JetBrains Mono</td></tr><tr><td>Themes</td><td>light · dark</td></tr></table></div></div>
${shots.map(screenPage).join('\n')}
<div class="page"><div class="kicker">motion-manifest.json · Lottie runtime</div><h2>Motion spec summary</h2>
<table class="motion"><tr><th>id</th><th>alias</th><th>loop</th><th>frames</th><th>fps</th><th>poster</th><th>reduced-motion</th></tr>
${motion.map(r=>`<tr><td><code>${esc(r[0])}</code></td><td>${esc(r[1])}</td><td>${esc(r[2])}</td><td>${esc(r[3])}</td><td>${esc(r[4])}</td><td>${esc(r[5])}</td><td><code>${esc(r[6])}</code></td></tr>`).join('')}</table>
<p style="font-size:9pt;color:#475569;margin-top:6mm">Runtimes — web: lottie-web · desktop: lottie-web (Tauri 2) · compose: lottie-compose · iOS: lottie-ios · TUI: none (Lipgloss). Static reduced-motion SVG fallbacks tracked as THREADY-MOT-03. Rendered animation frames are on the motion/preview plates above (captured mid-play).</p></div>
</body></html>`;
writeFileSync(out, html);
console.error(`assemble.html: ${shots.length} plates`);
NODE

echo "=== weasyprint -> PDF ==="
"$WEASYPRINT" --presentational-hints -u "file://$DESIGN_DIR/" "$ASSEMBLE" "$OUT_PDF"
SCREENS=$(find "$PNG" -name '*@2x.png' | wc -l)
SIZE=$(stat -c%s "$OUT_PDF"); SIZE_MB=$(awk "BEGIN{printf \"%.2f\",$SIZE/1048576}")
PAGES=$(pdfinfo "$OUT_PDF" 2>/dev/null | awk '/^Pages:/{print $2}')
echo "--- verify ---"; echo "size: ${SIZE_MB} MB (require >1)"; echo "pages: $PAGES (require > screens $SCREENS)"
ok=1; awk "BEGIN{exit !($SIZE>1048576)}" || { echo "FAIL size"; ok=0; }
[ "${PAGES:-0}" -gt "$SCREENS" ] || { echo "FAIL pages"; ok=0; }
[ "$ok" = 1 ] && echo "PDF OK (size + pages verified)" || { echo "PDF VERIFICATION FAILED"; exit 1; }
