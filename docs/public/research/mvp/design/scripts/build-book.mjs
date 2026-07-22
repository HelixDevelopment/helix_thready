#!/usr/bin/env node
// -----------------------------------------------------------------------------
// Helix Thready — design-book HTML composer (→ WeasyPrint → design-book.pdf)
// Location: docs/public/research/mvp/design/scripts/build-book.mjs
//
// Emits exports/pdf-build/book.html referencing the rendered PNGs by RELATIVE
// path (WeasyPrint resolves them against the html's own location). Pages:
//   cover · contents · tokens · [every screen PNG, one per page, captioned] ·
//   component library · motion spec summary.
// -----------------------------------------------------------------------------
import { readFileSync, writeFileSync, existsSync, statSync } from 'node:fs';
import { join } from 'node:path';

const DES = '/home/milos/Factory/projects/tools_and_research/helix_thready/docs/public/research/mvp/design';
const OUT = join(DES, 'exports');
const esc = (s) => String(s).replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;');

// ---- Screen page order (mandatory screens only; light then dark) ------------
const AREAS = [
  { area:'web',    label:'Web portal',       names:['index','dashboard','channels','thread-explorer','post-detail','search','research-viewer','assets-browser','skills-manager','events-monitor','accounts-admin','billing','settings','login'] },
  { area:'mobile', label:'Mobile app',       names:['home-feed','channel-threads','post-detail','search','notifications','account','settings'] },
  { area:'desktop',label:'Desktop shell',    names:['desktop-shell'] },
  { area:'tui',    label:'Terminal (TUI)',   names:['tui-screens'] },
];
// index is a png-only prototype-shell extra; keep it in the book but flag it.
const EXTRA_FLAG = new Set(['web/index']);

const pages = [];
for (const grp of AREAS) {
  for (const name of grp.names) {
    for (const theme of ['light','dark']) {
      const rel = `../png/${grp.area}/${name}${theme==='dark'?'-dark':''}.png`;
      const abs = join(OUT, 'png', grp.area, `${name}${theme==='dark'?'-dark':''}.png`);
      if (!existsSync(abs)) { console.error('MISSING', abs); continue; }
      pages.push({ area:grp.area, label:grp.label, name, theme, rel,
                   extra: EXTRA_FLAG.has(`${grp.area}/${name}`) });
    }
  }
}

// ---- Tokens (parsed lightly from tokens.css light :root + dark block) -------
const colorTokens = [
  ['--brand','#b6e376','#b6e376'], ['--brand-2','#abddc9','#b7ebd6'], ['--brand-ink','#0a0f04','#0a0f04'],
  ['--bg','#ffffff','#020817'], ['--surface','#ffffff','#020817'], ['--surface-warm','#f1f5f9','#1e293b'],
  ['--fg','#020817','#f8fafc'], ['--muted','#475569','#94a3b8'],
  ['--border','#e2e8f0','#1e293b'], ['--border-strong','#64748b','#64748b'],
  ['--accent','#446e12','#b6e376'], ['--accent-on','#ffffff','#0a0f04'],
  ['--success','#166534','#16a34a'], ['--warn','#854d0e','#eab308'], ['--danger','#dc2626','#ef4444'],
];
const typeScale = [['xs','12px'],['sm','14px'],['base','16px'],['lg','20px'],['xl','24px'],['2xl','32px'],['3xl','48px'],['4xl','64px']];
const spacing = [['1','4'],['2','8'],['3','12'],['4','16'],['5','20'],['6','24'],['8','32'],['12','48']];
const radii = [['sm','8'],['md','12'],['lg','16'],['pill','9999']];

// ---- Motion manifest --------------------------------------------------------
const motion = JSON.parse(readFileSync(join(DES,'motion/motion-manifest.json'),'utf8'));

// ---- Counts -----------------------------------------------------------------
const mandCount = pages.filter(p=>!p.extra).length;
const today = '2026-07-22';

// ---- HTML -------------------------------------------------------------------
const swatch = (t) => `<div class="sw"><div class="chip" style="background:${t[1]}"></div>
  <div class="chip dk" style="background:${t[2]}"></div>
  <div class="swmeta"><code>${t[0]}</code><span>${t[1]} · <b>dark</b> ${t[2]}</span></div></div>`;

const tokenPage = `
<section class="doc">
  <h2>Design tokens</h2>
  <p class="lede">The Helix Thready brand contract — the machine-readable twin of
  <code>opendesign/tokens.css</code>. Every screen in this book inlines these tokens; dark mode
  re-binds only the colour-bearing tokens (structural tokens are declared once).</p>
  <h3>Colour roles <span class="hint">left = light · right = dark</span></h3>
  <div class="swgrid">${colorTokens.map(swatch).join('')}</div>
  <div class="two">
    <div>
      <h3>Type scale</h3>
      <table class="tok"><tbody>${typeScale.map(([k,v])=>`<tr><td><code>--text-${k}</code></td><td>${v}</td></tr>`).join('')}</tbody></table>
      <p class="tiny">Display: Space Grotesk · Body: Hanken Grotesk · Mono: JetBrains Mono</p>
    </div>
    <div>
      <h3>Spacing (4px base)</h3>
      <div class="bars">${spacing.map(([k,v])=>`<div class="bar"><span style="width:${v}px"></span><code>--space-${k}</code> ${v}px</div>`).join('')}</div>
      <h3>Radius</h3>
      <table class="tok"><tbody>${radii.map(([k,v])=>`<tr><td><code>--radius-${k}</code></td><td>${v==='9999'?'pill':v+'px'}</td></tr>`).join('')}</tbody></table>
    </div>
  </div>
</section>`;

const motionRows = motion.animations.map(a=>`<tr>
  <td><code>${esc(a.id)}</code>${a.alias?`<br><span class="tiny">alias ${esc(a.alias)}</span>`:''}</td>
  <td>${esc(a.file)}</td><td>${a.loop?'loop':'once'}</td><td>${a.frames}f @ ${a.fr}fps</td>
  <td>poster ${a.posterFrame}</td><td>${esc(a.reducedMotion)}</td></tr>`).join('');

const motionPage = `
<section class="doc">
  <h2>Motion spec summary</h2>
  <p class="lede">Six Lottie animations (self-hosted JSON, no CDN). Each declares a
  <code>reducedMotion</code> fallback honoured under <code>prefers-reduced-motion: reduce</code>.
  Full preview board rendered on the facing page.</p>
  <table class="tok wide"><thead><tr><th>id</th><th>file</th><th>play</th><th>frames</th><th>poster</th><th>reduced-motion</th></tr></thead>
  <tbody>${motionRows}</tbody></table>
  <p class="tiny">Runtime bridge — web: ${esc(motion.runtime.web)} · desktop: ${esc(motion.runtime.desktop)} ·
  compose: ${esc(motion.runtime.compose)} · ios: ${esc(motion.runtime.ios)} · tui: ${esc(motion.runtime.tui)}.</p>
</section>
<section class="slide light"><img class="shot" src="../png/motion/preview.png" alt="motion preview board"/>
  <div class="cap"><b>motion</b> / preview board / light</div></section>`;

const slide = (p) => `<section class="slide ${p.theme}">
  <img class="shot" src="${p.rel}" alt="${esc(p.area)} ${esc(p.name)} ${p.theme}"/>
  <div class="cap"><b>${esc(p.area)}</b> / ${esc(p.name)} / ${p.theme}${p.extra?' <i>(prototype shell)</i>':''}</div>
</section>`;

// component-library pages (light + dark), full-bleed
const libSlides = ['','-dark'].map(sfx=>{
  const rel=`../png/library/components${sfx}.png`;
  if(!existsSync(join(OUT,'png','library',`components${sfx}.png`))) return '';
  const theme = sfx?'dark':'light';
  return `<section class="slide ${theme}"><img class="shot" src="${rel}" alt="component library ${theme}"/>
    <div class="cap"><b>library</b> / components sheet / ${theme}</div></section>`;
}).join('');

const html = `<!DOCTYPE html><html lang="en"><head><meta charset="utf-8"><title>Helix Thready — Design Book</title>
<style>
  @page { size: A4; margin: 0; }
  @page :first { margin: 0; }
  * { box-sizing: border-box; -weasy-hyphens: none; }
  html,body { margin:0; padding:0; font-family:"Hanken Grotesk","Helvetica Neue",Arial,sans-serif; color:#020817; }
  code { font-family:"JetBrains Mono",ui-monospace,monospace; font-size:.85em; }
  .slide { position:relative; width:210mm; height:297mm; display:flex; align-items:center; justify-content:center;
           page-break-after:always; break-after:page; overflow:hidden; padding:10mm; }
  .slide.light { background:#eef2f7; }
  .slide.dark  { background:#020817; }
  .slide img.shot { max-width:100%; max-height:100%; object-fit:contain; box-shadow:0 3px 22px rgba(2,8,23,.28); }
  .cap { position:absolute; left:9mm; bottom:8mm; background:rgba(255,255,255,.92); color:#020817;
         border:1px solid #cbd5e1; border-radius:9999px; padding:4px 12px; font-size:10pt; letter-spacing:.01em; }
  .slide.dark .cap { background:rgba(15,23,42,.9); color:#f8fafc; border-color:#334155; }
  .cap b { color:#446e12; } .slide.dark .cap b { color:#b6e376; } .cap i { opacity:.7; font-style:italic; }
  /* cover */
  .cover { width:210mm; height:297mm; page-break-after:always; break-after:page; position:relative;
           background:linear-gradient(150deg,#b6e376 0%,#abddc9 46%,#eef2f7 47%,#ffffff 100%);
           display:flex; flex-direction:column; }
  .cover .top { flex:0 0 46%; display:flex; align-items:center; justify-content:center; }
  .cover .logo { width:120mm; }
  .cover .bot { flex:1; padding:16mm 20mm; display:flex; flex-direction:column; gap:5mm; }
  .cover h1 { font-family:"Space Grotesk","Hanken Grotesk",sans-serif; font-size:34pt; margin:0; letter-spacing:-.01em; color:#020817; }
  .cover .sub { font-size:14pt; color:#334155; margin:0; }
  .cover .meta { margin-top:auto; font-size:10.5pt; color:#475569; line-height:1.6; }
  .cover .slogan { font-size:13pt; color:#446e12; font-weight:600; }
  .cover .heart { color:#dc2626; }
  /* doc pages */
  .doc { width:210mm; min-height:297mm; padding:20mm; page-break-after:always; break-after:page; }
  .doc h2 { font-family:"Space Grotesk",sans-serif; font-size:24pt; margin:0 0 4mm; color:#020817; border-bottom:3px solid #b6e376; padding-bottom:3mm; }
  .doc h3 { font-family:"Space Grotesk",sans-serif; font-size:13pt; margin:8mm 0 3mm; color:#020817; }
  .doc h3 .hint,.doc .hint { font-weight:400; font-size:9pt; color:#64748b; }
  .lede { font-size:11pt; color:#334155; line-height:1.55; max-width:150mm; }
  .swgrid { display:grid; grid-template-columns:repeat(3,1fr); gap:5mm; }
  .sw { border:1px solid #e2e8f0; border-radius:10px; overflow:hidden; }
  .sw .chip { height:12mm; } .sw .chip.dk { height:7mm; border-top:1px solid #e2e8f0; }
  .sw .swmeta { padding:2.5mm 3mm; } .sw .swmeta code { font-weight:600; display:block; font-size:9.5pt; }
  .sw .swmeta span { font-size:7.7pt; color:#64748b; }
  .two { display:grid; grid-template-columns:1fr 1fr; gap:10mm; margin-top:4mm; }
  table.tok { width:100%; border-collapse:collapse; font-size:10pt; }
  table.tok td,table.tok th { border-bottom:1px solid #e2e8f0; padding:2mm 2mm; text-align:left; }
  table.tok.wide { font-size:9pt; } table.tok.wide code { font-size:8.5pt; }
  .bars .bar { display:flex; align-items:center; gap:3mm; font-size:9pt; color:#475569; margin:2mm 0; }
  .bars .bar span { display:inline-block; height:5mm; background:#446e12; border-radius:3px; }
  .tiny { font-size:8.5pt; color:#64748b; line-height:1.5; } .lede code,.tiny code { color:#446e12; }
</style></head><body>

<div class="cover">
  <div class="top"><img class="logo" src="../../assets/logo-full.svg" alt="Helix Thready"/></div>
  <div class="bot">
    <h1>Helix Thready — Design Book</h1>
    <p class="sub">MVP design package · screens, tokens, component library & motion</p>
    <div class="meta">
      <div><b>${mandCount}</b> screen renders (light + dark, deviceScaleFactor 2) across web · mobile · desktop · TUI.</div>
      <div>Tokens · component library · motion spec · 22 genuine OD&nbsp;Figma capture IRs.</div>
      <div>Generated ${today} · self-contained HTML → PNG (headless Chromium) → PDF (WeasyPrint).</div>
      <div class="slogan">Made with <span class="heart">&#9829;</span> by Helix Development</div>
    </div>
  </div>
</div>

<section class="doc">
  <h2>What's in this book</h2>
  <p class="lede">A single portable record of the Helix Thready MVP design package. Each screen is
  a self-contained OpenDesign-style HTML artifact with inlined brand tokens and first-class light +
  dark themes; the pages here are 2× headless-Chromium renders, one screen per page, captioned
  <code>area / screen / theme</code>.</p>
  <table class="tok"><thead><tr><th>Section</th><th>Contents</th></tr></thead><tbody>
    <tr><td>Design tokens</td><td>Colour roles (light + dark), type scale, spacing, radius</td></tr>
    <tr><td>Web portal</td><td>14 screens × 2 themes (incl. prototype shell)</td></tr>
    <tr><td>Mobile app</td><td>7 screens × 2 themes (390px device frame)</td></tr>
    <tr><td>Desktop shell</td><td>1 screen × 2 themes</td></tr>
    <tr><td>Terminal (TUI)</td><td>1 screen × 2 themes</td></tr>
    <tr><td>Component library</td><td>Full component sheet, light + dark</td></tr>
    <tr><td>Motion spec</td><td>6 Lottie animations + preview board</td></tr>
  </tbody></table>
</section>

${tokenPage}
${pages.map(slide).join('\n')}
${libSlides}
${motionPage}
</body></html>`;

writeFileSync(join(OUT,'pdf-build','book.html'), html);
console.log('wrote book.html  screen-slides:', pages.length, ' (mandatory', mandCount, ')');
