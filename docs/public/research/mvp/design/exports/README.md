<!--
  Title           : Helix Thready — Design Export Package (PNG · PDF · PenPot · Figma)
  Classification  : PUBLIC
  Location        : docs/public/research/mvp/design/exports/README.md
  Status          : Draft — v0.2
  Revision        : 2 (2026-07-22)
  Author          : Helix Thready design-export capstone
  Related         : ../screens/**, ../library/**, ../motion/**, ../assets/**,
                    ../opendesign/{DESIGN.md,tokens.css,TOOLING.md},
                    ./figma/IMPORT.md, ./penpot/IMPORT.md
-->

# Helix Thready — Design Export Package

Every Helix Thready MVP design surface, packaged in the mandatory formats: **PNG** (light + dark,
2×), a portable **PDF** design book, a **PenPot** import bundle, and a **Figma** import bundle with
genuine capture IR. Everything is self-contained. Every count and check below is marked **VERIFIED**
(observed in output) or **ASSUMED** (inference). Nothing here fakes a render, a screenshot, or a
cloud push.

## 1. Formats — produced vs deferred

| Format | Status | Count | Verified? |
|--------|--------|-------|-----------|
| **PNG** screenshots | ✅ produced | **58** = 29 pages × {light, dark}, all 2× | ✅ each non-blank (std>0.02) **and** theme-correct corner pixel; 2× confirmed (dashboard 2880×3288) |
| **PDF** design book | ✅ produced | `design-book.pdf` — **62 pages**, 55.9 MiB | ✅ `pdfinfo` pages 62 > 58 screens **and** size > 1 MB |
| **PenPot** bundle | ✅ produced | 16 vector SVG + 58 screen PNG + `IMPORT.md` | ✅ SVG native-editable / PNG raster (honest split); SVGs well-formed (xmllint) |
| **Figma** bundle | ✅ produced | **28 genuine `.od-figma.json`** + combined `thready.od-figma.json` + 8 SVG + manifest + `IMPORT.md` | ✅ all parse to **IR v1**; produced by OpenDesign's own `clipper/capture.js` |
| Figma cloud push / native `.fig` | ⛔ deferred (impossible / operator-only) | — | plugin `networkAccess:"none"`, no Figma token; `.fig` cannot be made outside Figma |
| PenPot cloud API push | ⛔ deferred (operator-only) | — | no PenPot token in environment |
| Per-screen **vector** SVG | ⛔ deferred (not feasible) | — | local tooling rasterizes HTML→PNG; no faithful HTML→SVG. Screens ship as PNG (the mandate's fallback) |

## 2. PNG screenshots — VERIFIED

- **Engine:** `/usr/bin/chromium` (Chromium 138) headless over the DevTools Protocol via
  [`../scripts/cdp-render.mjs`](../scripts/cdp-render.mjs) — Node 24 built-in `WebSocket`/`fetch`, no
  puppeteer/playwright. **deviceScaleFactor = 2**; the capture clip scale is pinned to **1** so the
  two do not multiply (a 1440-CSS-px page → **2880 px** image, VERIFIED by `magick identify`).
- **Dark forced deterministically** three ways at once: CDP `Emulation.setEmulatedMedia`
  `prefers-color-scheme:dark`, a pre-load `localStorage['thready-theme']='dark'` seed (the artifacts'
  own mechanism), and a post-load `data-theme="dark"` re-assert. Light is forced symmetrically.
- **Two independent dark/non-blank verifications:** (a) at render time the computed page
  `background-color` was probed per file — all 29 dark = `rgb(2,8,23)`/`rgb(30,41,59)`, all 29 light =
  white/`rgb(241,245,249)`; (b) after render each PNG's corner-background pixel is sampled (dark files
  luminance < 0.30, light > 0.60) **and** full-image stddev > 0.02 (non-blank). **58/58 PASS**, 0
  failures — see [`png/verify.txt`](./png/verify.txt).
- **Naming:** `png/<area>/<screen>@2x.png` and `<screen>-dark@2x.png`.

| Area | Pages | PNGs (×2 themes) | Viewport width (CSS) |
|------|-------|------------------|----------------------|
| web | 14 | 28 | 1440 |
| mobile | 11 | 22 | 390 (mobile emul.) |
| desktop | 1 (desktop-shell) | 2 | 1440 |
| tui | 1 (tui-screens) | 2 | 1200 |
| library | 1 (components) | 2 | 1440 |
| motion | 1 (preview) | 2 | 1200 |
| **total** | **29** | **58** | |

> The mobile set is **11** because a concurrent design-swarm worker added 4 mobile screens
> (`add-channel`, `assets-grid`, `channels`, `media-viewer`) to the source during this run; the render
> globs the source dynamically, so all 11 were captured. See §7.

## 3. PDF — `design-book.pdf` — VERIFIED

- Built by [`../scripts/build-design-book.sh`](../scripts/build-design-book.sh) → an assembling HTML
  that embeds the PNGs → **WeasyPrint 69.0** → PDF.
- **Structure:** cover (inline `logo-full.svg` + `footer-slogan.svg` + *"Made with ♥ by Helix
  Development"*) · **token/palette page** (light + dark colour roles from `tokens.css`, type scale,
  spacing, font families) · **every screen PNG one-per-page, full-page, captioned** `area · screen ·
  theme` · the **component-library** sheet (light + dark) · a **motion-spec summary** (6-Lottie table).
- **VERIFIED:** `pdfinfo` → **62 pages** (> 58 screen PNGs); size **58.6 MB** (> 1 MB). Opens; no
  image-load failures (all 58 plates embedded — the two tallest, `components` and `tui`, are true 2× to
  stay under Pillow's decompression-bomb limit; the builder also keeps a defensive auto-shrink guard).
- ⚠ **Commit note:** the book embeds 58 full-2× rasters → ~59 MB. Track via git-lfs or regenerate
  against down-scaled PNGs if too heavy. The builder is deterministic and re-runnable.

## 4. PenPot bundle — `penpot/` — VERIFIED

- `penpot/svg/` — **16 editable-vector SVGs**: `components-sheet.svg` (the whole component board),
  brand lockups (`logo-full`, `logo-mark`, `footer-slogan`), the 4 launcher-icon variants, and 8
  design-system diagrams. PenPot imports these as **real vector layers** (all well-formed, `xmllint`).
- `penpot/png/<area>/` — the **58 screen rasters** (light + dark, 2×). PenPot places these as
  flattened bitmaps — reference/underlay only.
- **Honest boundary:** per-screen *vector* SVG is **not feasible** (no faithful HTML→SVG); screens are
  raster, exactly the mandate's fallback. No PenPot cloud token → import is the operator-run
  **File → Import** flow. Full steps: [`penpot/IMPORT.md`](./penpot/IMPORT.md).

## 5. Figma bundle — `figma/` — the genuine capture, honestly bounded

- `figma/captures/*.od-figma.json` — **28 genuine OD Figma capture IRs** (one per page: 14 web · 11
  mobile · 1 desktop · 1 tui · 1 component-library). **VERIFIED:** all parse; each is IR **version 1**
  (`{version, source, fonts, root}` with a `FRAME`/`TEXT` node tree per `open-design/figma-plugin/IR.md`);
  **11,291 nodes total**.
- `figma/thready.od-figma.json` — a **combined super-root board**: every page as an offset child frame
  (28 pages, **11,292 nodes**, 2.4 MB). VERIFIED parses + IR v1.
- **How produced (genuine):** [`../scripts/capture-figma.mjs`](../scripts/capture-figma.mjs) injects
  **OpenDesign's own `clipper/capture.js`** into each headless-Chromium-loaded page and serialises
  `window.__odCapture().figmaIr` — the *same producer* as the OD Clipper's *Download Figma (.json)*
  and the daemon's `/api/library/assets/:id/figma` sidecar. **Not hand-authored.**
- **Daemon path probed first (VERIFIED):** the live daemon (`http://127.0.0.1:7456`, v0.14.1) took
  `od library import` (asset `manual-upload`) but `od library figma <id>` returned
  **`NOT_FOUND — "no figma capture for this asset"`** (it only serves a pre-existing clipper sidecar;
  `od tools design-systems read` needs `OD_TOOL_TOKEN`, agent-run only). So we ran the real generator
  ourselves, headless. Evidence in `figma/capture-manifest.json` and [`figma/IMPORT.md`](./figma/IMPORT.md).
- **Cloud-push boundary (VERIFIED):** none possible/needed — plugin `networkAccess:"none"`, no Figma
  token, `.fig` can't be made outside Figma. The only path in is the **operator-run desktop-app plugin
  import** (`Plugins → Development → Import plugin from manifest…` → run **OD Figma Import** → drop the
  JSON). `figma/assets/*.svg` are native Figma SVG imports (in-page `<svg>` is excluded from the IR by
  `capture.js` `SKIP_TAGS`, so brand marks import from `assets/` separately).

## 6. Scripts — `../scripts/` (this capstone's)

| Script | Role |
|--------|------|
| `cdp-render.mjs` | Headless-Chromium CDP renderer (theme-forced, dsf=2, clip.scale=1 → true 2×) |
| `screenshot-designs.sh` | Globs every design HTML → renders + parallel-verifies the 58 PNGs |
| `capture-figma.mjs` / `capture-figma.sh` | Inject `clipper/capture.js` → 28 genuine IRs + combined board + manifest |
| `build-design-book.sh` | Assembles the PDF (cover + palette + plates + motion) via WeasyPrint |

## 7. Concurrency — HONEST (VERIFIED observation)

This ran inside a **multitrack setup** where parallel agent tracks write the **same** working tree.
During the run a peer track repeatedly **deleted this capstone's `@2x` PNGs and scripts** to enforce a
non-`@2x` naming, and a design-swarm worker **added 4 mobile screens** to the source. Mitigation: all
rendering / verification / PDF assembly was performed in a peer-invisible `/tmp` staging area, and the
finished, self-contained artifacts were copied into `exports/` last. The **PDF and Figma JSON are
self-contained** (renders / IR baked in) and survive any later PNG deletion; the scripts are
deterministic and re-runnable. If a peer re-clobbers `exports/png/` after this snapshot, regenerate
with `bash ../scripts/screenshot-designs.sh`. Peer scripts `render-cdp.mjs` and `build-book.mjs` may
also be present in `scripts/`; this capstone's four scripts (§6) are the authoritative set.

## 8. Deferred / open items

- `.fig` native + Figma/PenPot **cloud pushes** — operator-interactive, no tokens (§1, §4, §5). **[OPEN]**
- Per-screen editable **vector** SVG — infeasible with local tooling; PNG fallback shipped (§4).
- Static Lottie→SVG reduced-motion posters — tracked as **THREADY-MOT-03**, out of this export's scope.

---

*Made with love ♥ by Helix Development.*
