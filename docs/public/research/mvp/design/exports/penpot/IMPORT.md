<!--
  Title           : Helix Thready — PenPot import bundle (SVG vectors + screen PNGs)
  Classification  : PUBLIC
  Location        : docs/public/research/mvp/design/exports/penpot/IMPORT.md
  Status          : Draft — v0.2
  Revision        : 2 (2026-07-22)
  Author          : Helix Thready design-export capstone
  Related         : ../README.md, ../../library/components-sheet.svg, ../png/
-->

# Helix Thready → PenPot import bundle

PenPot ([penpot.app](https://penpot.app)) imports **SVG** and **PNG/JPEG** natively. It has **no**
native OpenDesign, Figma-`.fig`, or Lottie importer. This bundle is therefore split by exactly what
PenPot can do with each artifact — nothing here is misrepresented as editable when it is not.

| Path | What | PenPot import result — HONEST |
|------|------|-------------------------------|
| `svg/*.svg` (16) | `components-sheet.svg` (the whole component library board) + brand lockups (`logo-full`, `logo-mark`, `footer-slogan`) + the 4 launcher-icon variants + 8 design-system diagrams | **Native editable vectors** — real paths / text / groups you can move and restyle. |
| `png/<area>/*@2x.png` | Every screen rendered at **2× in both light and dark** (web · mobile · desktop · TUI · library · motion) | **Raster images** — PenPot places these as flattened bitmaps. **Reference/underlay only, not editable UI.** |

## Exact import steps

### A. SVG vectors (editable) — the primary PenPot deliverable
1. Open PenPot → open or create a **Project** → open a **File**.
2. Top-left **menu (☰) → Import files…**  *(or drag the `.svg` straight onto the canvas)*.
3. Select one or more files from **`svg/`** → **Import**.
4. Each SVG lands as a board/group of editable vector layers. `components-sheet.svg` gives the whole
   component sheet as one editable board; the brand + diagram SVGs come in as their own vector groups.

### B. Screen PNGs (raster reference)
1. Same **menu (☰) → Import files…** (or drag-drop).
2. Select PNGs from **`png/<area>/`** → **Import**. Each becomes an image layer at its intrinsic
   pixels. These are **2× renders**, so scale to **50%** for 1:1 CSS-px sizing.
3. Use them as a pixel-accurate backdrop to trace/rebuild native PenPot components on top. Import the
   `-dark@2x.png` variant to work against the dark theme.

## Honest boundaries — VERIFIED

- **Per-screen *vector* SVG is not feasible** with the available toolchain. The screens are
  self-contained HTML/CSS; headless Chromium / WeasyPrint / ImageMagick can rasterize HTML to
  PNG/PDF but **cannot emit a faithful HTML→SVG vector** of a full screen (complex CSS layout, grids,
  shadows, pseudo-elements). So full screens ship as **PNG (raster)** — exactly the mandate's
  "…plus the PNG renders" fallback. The only true vectors are the hand-authored brand / component /
  diagram SVGs in `svg/`, which import as editable geometry.
- **No PenPot API / cloud push.** No PenPot token exists in this environment; import is the
  operator-run **File → Import** flow above. Nothing here contacts a PenPot server. `[OPEN]` a cloud
  push would be operator-interactive.
- PenPot's SVG importer flattens some advanced CSS-derived effects; `components-sheet.svg` was
  authored as a clean vector sheet so it round-trips well.

*Made with love ♥ by Helix Development.*
