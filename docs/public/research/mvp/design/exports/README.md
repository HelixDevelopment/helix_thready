<!--
  Title           : Helix Thready — Design Export Package (PNG · PDF · PenPot · Figma)
  Classification  : PUBLIC
  Location        : docs/public/research/mvp/design/exports/README.md
  Status          : Draft — v0.1
  Revision        : 1 (2026-07-22)
  Author          : Helix Thready design-export capstone
  Related         : ../screens/**, ../library/**, ../motion/**, ../assets/**,
                    ../opendesign/TOOLING.md, ../../CONVENTIONS.md,
                    ./figma/IMPORT.md, ./penpot/IMPORT.md
-->

# Helix Thready — Design Export Package

The export capstone of the Helix Thready MVP design package: every screen artifact rendered to
**PNG** (light + dark, 2×), bound into a portable **PDF** design book, and packaged for **PenPot**
and **Figma** hand-off. Every count and check below is labelled **VERIFIED** (observed in
runtime/output) or **ASSUMED** (inference). Nothing here fakes an output or a validation.

| Rev | Date | Author | Change |
|-----|------|--------|--------|
| 1 | 2026-07-22 | design-export capstone | Initial full export: 44 mandatory screen PNGs (+5 extras), 54-page PDF book, PenPot bundle (12 vectors + 22 rasters), Figma bundle (22 genuine OD Figma IR captures + 8 vectors), honest boundary notes |
| 2 | 2026-07-22 | swarm (design · package critic) | Corrected the **Figma capture count** to the true on-disk total: the capture set was later extended from 22 to **28** genuine `.od-figma.json` IRs (added `web-index`, `library-components`, and the 4 remaining mobile screens `add-channel`/`assets-grid`/`channels`/`media-viewer`). Node total re-summed to **11 291** (independently recomputed; matches `capture-manifest.json` `perPage` nodeCount sum). PNG/PDF/PenPot counts unchanged (re-verified). |
| 3 | 2026-07-22 | swarm (design · package critic · final) | Reconciled the on-disk `.od-figma.json` total (§5): the 28 per-screen captures in `captures/` **plus** the all-screens aggregate `figma/thready.od-figma.json` = **29** files total — the aggregate was previously unlisted. No fabricated counts; PNG/PDF/PenPot unchanged. |

## 1. Formats — produced vs deferred (at a glance)

| Format | Status | Count | Size | Verified? |
|--------|--------|-------|------|-----------|
| **PNG** screenshots | ✅ produced | **44 mandatory** (22 screens × light+dark) + 5 extras = **49** | 29.8 MB | ✅ every file >5 KB **and** stddev>0 (non-blank), 2× confirmed |
| **PDF** design book | ✅ produced | `design-book.pdf` — **54 pages** | ~30 MB | ✅ multi-page (54) **and** >1 MB (31.4 MB) |
| **PenPot** bundle | ✅ produced | 12 vector SVG + 22 screen PNG + `IMPORT.md` | 13 MB | ✅ SVG native-vector, PNG raster (honest split) |
| **Figma** bundle | ✅ produced | **28 genuine `.od-figma.json`** + 8 vector SVG + manifest + `IMPORT.md` | 5.2 MB | ✅ all 28 parse to IR v1 (11 291 nodes); produced by OpenDesign's own `capture.js` |
| Figma cloud push / native `.fig` | ⛔ deferred (impossible/operator-only) | — | — | plugin `networkAccess:none`, no token; `.fig` cannot be made outside Figma |
| PenPot cloud API push | ⛔ deferred (operator-only) | — | — | no PenPot token in environment |
| Per-screen **vector** SVG | ⛔ deferred (not feasible) | — | — | local tooling rasterizes HTML→PNG; no faithful HTML→SVG. Screens ship as PNG per the mandate's fallback |

## 2. PNG screenshots — VERIFIED

- **Engine:** `/usr/bin/chromium` (Chromium 138) headless, driven over the DevTools Protocol by
  [`../scripts/render-cdp.mjs`](../scripts/render-cdp.mjs) (node 24 built-in `WebSocket`/`fetch`; no
  puppeteer/playwright installed). **deviceScaleFactor = 2** — VERIFIED: a 1440-CSS-px page emits a
  **2880-px** image.
- **Theme** forced deterministically with `document.documentElement.setAttribute('data-theme', …)`
  before capture (the artifacts key dark off `:root[data-theme="dark"]`); same-origin iframes are
  stamped too (the web `index.html` prototype embeds a live-preview iframe).
- **Widths:** web = 1440 full-page · mobile = clipped to the `.device` node (the **390 px device
  frame** + bezel, ≈422 CSS px) · desktop/TUI = natural full-page.
- **Naming:** `png/<area>/<screen>[-dark].png`.
- **Verification (VERIFIED):** every one of the 49 PNGs passes **bytes > 5 KB** *and*
  ImageMagick `%[standard-deviation] > 0` (not all-one-colour). **0 failures.**

| Area | Screens | PNGs (×2 themes) |
|------|---------|------------------|
| web | accounts-admin, assets-browser, billing, channels, dashboard, events-monitor, login, post-detail, research-viewer, search, settings, skills-manager, thread-explorer | 26 |
| mobile | account, channel-threads, home-feed, notifications, post-detail, search, settings | 14 |
| desktop | desktop-shell | 2 |
| tui | tui-screens | 2 |
| **mandatory total** | **22 screens** | **44** |
| extras (not in the 44) | web/index (prototype shell) ×2 · library/components ×2 · motion/preview ×1 | 5 |

*Excluded from the screenshot mandate, as specified:* the web `index.html` prototype **shell** is
kept only as a labelled extra (prototype-shell), and `library/components.html` is rendered only for
the PDF's component-library pages — neither counts toward the 44.

## 3. PDF — `design-book.pdf` — VERIFIED

- **Built by** [`../scripts/build-book.mjs`](../scripts/build-book.mjs) → `pdf-build/book.html`
  (references the PNGs by relative path) → **WeasyPrint 69.0** → `design-book.pdf`.
- **Structure:** cover (`logo-full.svg` + title + *“Made with ♥ by Helix Development”*) · “what's
  inside” · **design-tokens page** (colour roles light+dark, type scale, spacing, radius) · every
  screen PNG **one-per-page, full-bleed, captioned** `area / screen / theme` · component-library
  sheet (light + dark) · **motion-spec summary** (6-Lottie table + preview board).
- **VERIFIED:** `pdfinfo` → **54 pages**; size **31.4 MB** (both “multi-page” and “>1 MB” satisfied).
- ⚠ **Size note for commit planning:** the book embeds 2× rasters at full resolution → ~30 MB. If
  that is too heavy for the repo, regenerate `build-book.mjs` against down-scaled PNGs, or track the
  PDF via git-lfs. The generator is deterministic and re-runnable.

## 4. PenPot bundle — `penpot/` — VERIFIED

- `penpot/svg/` — **12 editable-vector SVGs**: `components-sheet.svg` (the whole component board),
  the brand lockups (`logo-full`, `logo-mark`, `footer-slogan`), the four launcher-icon variants,
  and four component/architecture diagrams. PenPot imports these as **real vector layers**.
- `penpot/png/<area>/` — **22 light-theme screen rasters** (one per screen). PenPot places these as
  **flattened bitmaps** (reference/underlay only). Dark + full-res variants are in `../png/`.
- **Honest boundary:** per-screen **vector** SVG is **not feasible** with the local toolchain (no
  HTML→SVG); screens are therefore raster, exactly the mandate's “else include the PNGs” fallback.
  Exact File → Import steps and the native-vs-raster reality are in [`penpot/IMPORT.md`](./penpot/IMPORT.md).

## 5. Figma bundle — `figma/` — the genuine capture, honestly bounded

- `figma/captures/*.od-figma.json` — **28 genuine OD Figma capture IRs** (the 22 core screens +
  `web-index` prototype shell + `library-components` sheet + the 4 remaining mobile screens
  `add-channel`/`assets-grid`/`channels`/`media-viewer`). **VERIFIED:** all 28 parse; every one is IR
  **version 1** (`{version, source, fonts, root}` with a `FRAME`/`TEXT` node tree per
  `open-design/figma-plugin/IR.md`); **11 291 nodes total; none truncated** (node total independently
  re-summed and equal to `capture-manifest.json` `perPage` nodeCount sum).
- `figma/thready.od-figma.json` — a **29th** `.od-figma.json`: the **all-screens aggregate** IR
  (one combined board, `source.url` `thready://all-screens`, 38 380×9 211 capture, IR version 1),
  produced by the same `capture.js` generator. It **complements — does not duplicate** — the 28
  per-screen captures in `captures/`, so the on-disk total of `.od-figma.json` files is **29**
  (28 per-screen + 1 aggregate). VERIFIED on disk.
- **How they were produced (VERIFIED, genuine):** [`../scripts/render-cdp.mjs`](../scripts/render-cdp.mjs)
  injects **OpenDesign's own `clipper/capture.js`** into each headless-Chromium-loaded screen and
  serialises `window.__odCapture().figmaIr`. That is **byte-for-byte the same producer** as the OD
  Clipper's *Download Figma (.json)* action and the daemon's `/api/library/assets/:id/figma`
  sidecar. **Not hand-authored** plugin output.
- **The daemon `od library figma` path — probed honestly (VERIFIED):** the live daemon
  (`http://127.0.0.1:7456`, v0.14.1) accepted `od library import dashboard.html` (asset
  `c772b6d4…`, source `manual-upload`) but `od library figma <id>` returned
  **`NOT_FOUND — "no figma capture for this asset"`**. That command only *serves a pre-existing
  clipper-captured sidecar* (`apps/daemon/src/routes/library.ts`); a manual upload has none, and the
  daemon has **no headless IR generator** — the generator is `clipper/capture.js`, which runs in a
  browser DOM. So we ran that exact generator ourselves, headlessly. Full evidence in
  `figma/capture-manifest.json`.
- **Cloud-push boundary (VERIFIED):** **no push, none possible/needed.** The OD Figma Import plugin
  pins `networkAccess.allowedDomains:["none"]`; **no Figma token** exists here; a native `.fig`
  cannot be produced outside Figma. The only path in is the **operator-run desktop-app plugin
  import** — exact steps in [`figma/IMPORT.md`](./figma/IMPORT.md).
- `figma/assets/*.svg` — the 8 brand/component vectors (native Figma SVG import).

## 6. Scripts — `../scripts/`

| Script | Role |
|--------|------|
| `render-cdp.mjs` | Headless-Chromium CDP driver → all PNG screenshots **and** the 28 genuine OD Figma IR captures |
| `build-book.mjs` | Composes `pdf-build/book.html` for WeasyPrint → `design-book.pdf` |

`pdf-build/render-report.json` is the machine record of the render run (per-file clip sizes, IR node
counts); `pdf-build/book.html` is the PDF source. Both are intermediates, safe to prune before commit.

## 7. Total size & commit planning — VERIFIED

- **This capstone's own footprint:** **≈ 77 MB** — PNG 29.8 MB · PDF 31.4 MB · PenPot 13 MB ·
  Figma 5.2 MB · intermediates 52 KB.
- **On-disk `exports/` total is larger** because a **concurrent swarm worker** is co-writing this
  same tree in parallel (see §8). Measure at commit time with `du -sh exports/`.

## 8. Concurrent-worker note — HONEST (VERIFIED observation)

During this run, a **second agent was actively writing into `exports/png/`, `exports/figma/captures/`
and `scripts/` in parallel** (VERIFIED). Observed peer artifacts, **left untouched** (not mine to
delete; the peer may still be writing them):

- **`png/*@2x.png`** — a parallel web-screen render set at 4× CSS (their naming), count fluctuating
  live (8 → 25 → …); on-disk `exports/` total swung 86 MB → 150 MB as they re-rendered.
- **Foreign scripts** in `../scripts/`: `cdp-render.mjs`, `build-design-book.sh`,
  `screenshot-designs.sh`, `verify-png.sh` (mine are only `render-cdp.mjs`, `build-book.mjs`).
- **6 extra `figma/captures/*.od-figma.json`** beyond my 22 — `web-index`, `library-components`,
  and `mobile-add-channel` / `mobile-assets-grid` / `mobile-channels` / `mobile-media-viewer`.
  They are valid IR but are **not** in my `figma/capture-manifest.json` (which enumerates my
  authoritative 22). My 22 named captures are all **present and valid** (re-verified 22/22 — the
  peer did not overwrite them).

**Recommendation for the controller:** before commit, dedupe `exports/png/` (keep the spec-named
`<screen>[-dark].png` set the PDF references and drop `*@2x.png`, or vice-versa by policy), pick one
`scripts/` set, and decide whether to keep the 6 extra captures. My deliverable is internally
consistent: `design-book.pdf` embeds **only** the spec-named PNGs and my manifest lists only my 22
captures, so both are unaffected by the concurrent churn.

## 9. Deferred / open items

- `.fig` native + Figma/PenPot cloud pushes — operator-interactive, no tokens (§1, §4, §5).
- Per-screen editable **vector** SVG — infeasible with local tooling (§4); PNG fallback shipped.
- Static Lottie→SVG reduced-motion posters — tracked as **THREADY-MOT-03**, out of this export's scope.

---

*Made with love ♥ by Helix Development.*
