<!--
  Title           : Helix Thready — Diagram Render Status
  Classification  : PUBLIC
  Location        : docs/public/research/mvp/diagrams-render-status.md
  Status          : Active — v1.0
  Revision        : 1 (2026-07-22)
  Author          : Helix Thready documentation swarm (diagrams)
  Related         : ./CONVENTIONS.md · ./scripts/render-diagrams.sh · ./index.md
-->

# Helix Thready — Diagram Render Status

| Rev | Date | Author | Change |
|-----|------|--------|--------|
| 1 | 2026-07-22 | swarm (diagrams) | Initial status: 104/104 `.mmd` sources rendered to sibling `.svg` via mermaid-cli |

This file records the machine-verifiable state of the Mermaid diagram export step for the
`docs/public/research/mvp/` tree. Per `[CONSTITUTION §11.4.65]` (md→HTML/PDF sync) the `.mmd`
files are the **source of truth**; PNG/SVG export runs via the Docs Chain / `mermaid-cli` at
build time. `scripts/render-diagrams.sh` **is** that export step and is safe to re-run.

## Table of contents

1. [Result summary](#1-result-summary)
2. [Per-area breakdown](#2-per-area-breakdown)
3. [Tooling used (VERIFIED)](#3-tooling-used-verified)
4. [How to reproduce](#4-how-to-reproduce)
5. [Source change made during this render](#5-source-change-made-during-this-render)
6. [Open items](#6-open-items)

## 1. Result summary

`[RESEARCH]` **VERIFIED — every `.mmd` rendered.** All **104** Mermaid sources under
`docs/public/research/mvp/**/diagrams/` have a sibling `.svg` that is a genuine mermaid-cli
render (each contains the `aria-roledescription="…"` / `class="flowchart|…"` signature the
renderer emits — none are hand-faked placeholders).

| Metric | Count |
|--------|-------|
| `.mmd` sources under `**/diagrams/` | 104 |
| Sibling `.svg` that are real mermaid renders | 104 |
| Missing renders | 0 |
| Faked / placeholder SVGs | 0 |
| Deferred | 0 |

> Rendered PNG/SVG exported via Docs Chain (`[CONSTITUTION §11.4.65]`). SVG is the committed
> format (preferred — vector, diff-able). `.png` is available on demand via
> `scripts/render-diagrams.sh png`.

Note: the seven `.svg` under `design/assets/` (logos, launcher icons, footer slogan) are
authored brand assets, **not** diagram renders — they have no `.mmd` source and are out of
scope for this export step.

## 2. Per-area breakdown

| Area (`**/diagrams/`) | `.mmd` | real `.svg` |
|-----------------------|-------:|------------:|
| `api/diagrams` | 8 | 8 |
| `architecture/diagrams` | 18 | 18 |
| `database/diagrams` | 11 | 11 |
| `database/materials/diagrams` | 1 | 1 |
| `deployment/diagrams` | 14 | 14 |
| `deployment/materials/diagrams` | 1 | 1 |
| `design/diagrams` | 17 | 17 |
| `development/diagrams` | 11 | 11 |
| `development/materials/diagrams` | 1 | 1 |
| `testing/diagrams` | 9 | 9 |
| `user-guides/diagrams` | 13 | 13 |
| **Total** | **104** | **104** |

## 3. Tooling used (VERIFIED)

`[RESEARCH]` The render was performed on the host with locally-installed tooling — no network
fetch was required:

| Component | Version / path |
|-----------|----------------|
| `mmdc` (`@mermaid-js/mermaid-cli`) | 11.16.0 (global; also on `PATH`) |
| Node.js | v24.18.0 |
| Chromium (Puppeteer engine) | 138.0.7204.168 at `/usr/bin/chromium`, driven `--no-sandbox --disable-gpu` |

Because a system Chromium is present, `render-diagrams.sh` auto-detects it and writes a
Puppeteer config pointing at it; if only the Puppeteer-bundled Chromium is available the script
falls back to that. Where `mmdc` is absent the script falls back to `npx -y @mermaid-js/mermaid-cli`.

## 4. How to reproduce

From `docs/public/research/mvp/`:

```bash
scripts/render-diagrams.sh            # render ALL to .svg (preferred, idempotent)
scripts/render-diagrams.sh png        # render ALL to .png instead
scripts/render-diagrams.sh svg api    # only the api/ area, as .svg
```

The script exits non-zero and lists any file that fails to parse/render, so it doubles as a
pre-tag diagram gate. Single file, matching what the swarm ran:

```bash
mmdc -i <in.mmd> -o <out.svg> -p <puppeteer-config.json>
# puppeteer-config.json: {"executablePath":"/usr/bin/chromium","args":["--no-sandbox","--disable-gpu"]}
```

## 5. Source change made during this render

`[RESEARCH]` One source needed a **Mermaid-syntax fix** to parse — a genuine defect, not a
tooling gap: `development/materials/diagrams/sdk-publish-flow.mmd` used unquoted `{` inside two
square-bracket node labels (`GENCORE[gen/{go,rust,dart} core]`), which the parser reads as a
diamond-node start (`Parse error … got 'DIAMOND_START'`). The labels were quoted
(`GENCORE["gen/{go,rust,dart} core"]`) — identical displayed text, no semantic change — after
which it renders cleanly. This is the same quoting convention already applied to sibling sources
(e.g. labels containing `\n` such as `PP["port_prefix\n…"]`).

## 6. Open items

- `[OPEN: mmd-md-sync]` The `.mmd` files are the export source of truth, but the same Mermaid is
  also embedded inline in the area `.md` docs (per `CONVENTIONS §4`). The one syntax fix in §5
  was applied to the `.mmd` only (this task's write scope is `diagrams/`, `scripts/`, and this
  status file). The corresponding inline ```mermaid block in the owning `.md` should be re-synced
  by the doc owner so on-page rendering matches the exported SVG. `[CONSTITUTION §11.4.65]`
- `[OPEN: commit-artifacts]` The 104 `.svg` renders and the quoting fixes to the `.mmd` sources
  are present in the working tree; committing/pushing them to all four upstreams is left to the
  operator per `CONVENTIONS §8` (`[CONSTITUTION §2.1]`).

---

*Made with love ♥ by Helix Development.*
