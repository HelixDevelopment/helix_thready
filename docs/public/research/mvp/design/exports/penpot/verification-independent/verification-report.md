<!--
  Title           : Helix Thready → PenPot import — INDEPENDENT verification report
  Classification  : PUBLIC
  Location        : docs/public/research/mvp/design/exports/penpot/verification-independent/verification-report.md
  Status          : Final — v1.0
  Revision        : 1 (2026-07-22)
  Author          : Independent verifier (separate agent from the importer; RPC + CDP evidence only)
  Related         : ../IMPORT.md, ../verification/ (importer's own evidence), snapshots.jsonl, *.png
-->

# Helix Thready → PenPot import — independent verification

Independent verification of the PenPot import performed against the live instance at
`http://localhost:9001` (PenPot **2.17.0**), authenticated statelessly via
`Authorization: Token <token>` against `/api/rpc/command/*`. All numbers below were read
**directly from the instance** by this verifier — none are copied from the importer's claims.
[VERIFIED — RPC responses captured in `snapshots.jsonl`, 18 snapshots, 10:21–11:11 UTC]

## 1. Verdict table

| Import-plan item | Verdict | Independent evidence |
|---|---|---|
| **Project structure** (project "Helix Thready", 7 files "01 Foundations" … "07 Prototypes") | **VERIFIED-PRESENT** | `get-projects` → project `6e660a50-7c2a-8049-8008-5d974fe499d1` with `count: 7`; `get-project-files` lists all 7 expected names. |
| **Design tokens** (W3C DTCG sets/themes) | **VERIFIED-PRESENT** | Files 01 + 02 each carry `data.tokensLib`: sets `thready-light` (19), `thready-dark` (19), `thready-structure` (33) = **71 tokens**; themes `Thready/Light` (active) + `Thready/Dark`; `$metadata.activeSets = [thready-structure, thready-light]`. Source DTCG file has 74 → 71/74 imported (3 alias/unsupported dropped). Files 03–07 carry no tokensLib (by design). |
| **SVG vectors** (editable geometry) | **VERIFIED-PRESENT** | File 01: 325 objects incl. layer groups `token-fan-out`, `design-area-map`, 4 `launcher-icon-*`, `footer-slogan`, `logo-full`, `logo-mark` (screenshot `file-01-foundations.png`). File 02: 974 objects incl. `components-sheet`, `component-taxonomy`, `component-state-lifecycle`. File 06: 591 objects. Workspace screenshots show real vector layer trees, not flattened bitmaps. |
| **PNG underlays** (2× screen renders as boards) | **VERIFIED-PRESENT** | File 03: **28 frames**/65 objects; file 04: **22 frames**/45 objects; file 05: 4 frames/9 objects. Viewer render `file-03-screens-web-viewer.png` shows the full-fidelity dark `web/thread-explorer` screen (1 of 28 boards). |
| **Hydration** (thumbnails rendered) | **PARTIAL** | Object thumbnails exist for **6/7 files** (`get-file-object-thumbnails`: 01→2, 02→2, 03→6, 04→2, 05→4, 06→**0**, 07→2). File-level `thumbnailId` only on 07. Dashboard (`dashboard-recent-final.png`) shows rendered card thumbnails for 03+04 but placeholder glyphs for 02+06. |
| **Prototype interactions/flows** (post-import wiring) | **VERIFIED-PRESENT** (placement differs from claim) | 15 interactions total: **12 in file 03**, **3 in file 04** (not in "07 Prototypes", which has 0). 2 flows: `web-primary` (03, page-level `flows`) + `mobile-primary` (04). |

## 2. Timestamped snapshot table (RPC poll, ~90 s cadence)

Total objects = sum over all 7 files, including 1 root frame per page (7 baseline).

| UTC time | Total objects | Tokens (Σ files) | Notable state (file: revn) |
|---|---|---|---|
| 10:21:25 | 9 | 0 | Baseline: 7 empty files; 03 revn 1 (2 objects) |
| 10:23:04 – 10:29:04 | 9 | 0 | Importer preparing; 01 revn 0→2 |
| 10:30:35 | 13 | 0 | 01 writing (revn 4) |
| 10:32:05 | 21 | 0 | 01 revn 5 |
| 10:36:36 | 119 | 0 | 03 revn 3 (28 frames), 04 revn 1 (22 frames) — PNG underlays landed |
| 10:38:06 | 127 | 0 | 02/05/07 first writes |
| 10:39:37 | 1,100 | 0 | 01 revn 16 (325 obj) — SVG vector burst |
| 10:41:07 | 2,006 | 0 | 02 revn 6 (974 obj), 06 revn 4 (591 obj) |
| 10:51:27 | 2,006 | 0* | Post-completion check (*parser missed DTCG shape — fixed) |
| 10:52:27 | 2,006 | 142 (71×2) | Corrected parser: tokensLib visible in 01+02 |
| **11:11:13 (final)** | **2,014** | **142 (71×2)** | 03 revn 6 (65 obj, interactions), 04 revn 2 |

## 3. Final per-file state (11:11:13 UTC) vs importer claims

Importer counts exclude the per-page root frame; mine include it (claimed = mine − 1). All match.

| File | id (suffix) | revn | Objects (mine / claimed) | Frames | Token sets / tokens / themes |
|---|---|---|---|---|---|
| 01 Foundations | …5d9757f5eba9 | 16 | 325 / 324 ✓ | 2 | 3 / 71 / 2 ✓ |
| 02 Components | …5d97583457f7 | 6 | 974 / 973 ✓ | 2 | 3 / 71 / 2 ✓ |
| 03 Screens — Web | …5d975850be0f | 6 | 65 (was 57=56+1 at claim time) ✓ | 28 | — |
| 04 Screens — Mobile | …5d97587fa515 | 2 | 45 / 44 ✓ | 22 | — |
| 05 Screens — Desktop + TUI | …5d975891ead1 | 1 | 9 / 8 ✓ | 4 | — |
| 06 Platform Overrides | …5d9758a3f0f7 | 4 | 591 / 590 ✓ | 0 | — |
| 07 Prototypes | …5d9758b7309a | 1 | 5 / 4 ✓ | 2 | — |

**No mismatches** in object counts. One placement deviation: the claimed "15 interactions, 2 flows"
are real but live in files 03 (12 interactions + flow `web-primary`) and 04 (3 interactions + flow
`mobile-primary`), not in file 07.

## 4. RPC evidence (trimmed)

`get-profile` → auth OK: `{"email":"…","fullname":"Milos Vasic","defaultTeamId":"15d3067e-…-5d91f2863c25"}`

`get-projects?team-id=15d3067e-…` →
```json
[{"id":"6e660a50-7c2a-8049-8008-5d974fe499d1","name":"Helix Thready","count":7},
 {"id":"15d3067e-de6d-8170-8008-5d91f287c4e2","name":"Drafts","count":0}]
```

`get-file?id=…eba9` (01 Foundations) → `data.tokensLib` (trimmed):
```json
{"thready-light":{"color":{"brand":{…},"accent":{…}, …19 tokens}},
 "thready-dark":{"color":{…19 tokens}},
 "thready-structure":{"number":{…},"dimension":{…},"fontFamily":{…} = 33 tokens},
 "$themes":[{"group":"Thready","name":"Light","selectedTokenSets":{"thready-structure":"enabled","thready-light":"enabled"}},
            {"group":"Thready","name":"Dark","selectedTokenSets":{"thready-structure":"enabled","thready-dark":"enabled"}}],
 "$metadata":{"tokenSetOrder":["thready-light","thready-dark","thready-structure"],
              "activeThemes":["Thready/Light"],"activeSets":["thready-structure","thready-light"]}}
```

`get-file?id=…be0f` (03) page-level `flows` →
```json
{"f4c2d05f-…":{"id":"f4c2d05f-…","name":"web-primary","startingFrame":"70850c94-…"}}
```

Full raw snapshots: `snapshots.jsonl` (18 lines, one JSON snapshot each).

## 5. Screenshot index (CDP, disposable `chromedp/headless-shell` @ 1440×900, host network, port 9333)

| File | Size | What it shows |
|---|---|---|
| `dashboard.png` | 156 KB | First dashboard visit post-login; 07 Prototypes card with rendered thumbnail; PenPot onboarding modal partially obscures view (anomaly 2) |
| `dashboard-recent-final.png` | 210 KB | Final dashboard: "Helix Thready — 7 files"; rendered thumbnails for 03+04, placeholder glyphs for 02+06 (hydration = PARTIAL) |
| `dashboard-project-helix-thready.png` | 25 KB | Project files view, mid-import (10:41 UTC) |
| `dashboard-project-helix-thready-final.png` | 6 KB | **Failed render** — project-files route came up nearly empty in headless capture (anomaly 3); superseded by `dashboard-recent-final.png` |
| `file-01-foundations.png` | 80 KB | Workspace: vector layer tree (token-fan-out, design-area-map, launcher icons, logos, motion previews); Thready logo vectors on canvas; TOKENS tab present |
| `file-02-components.png` | 145 KB | Workspace: components-sheet + taxonomy + lifecycle vectors, light/dark library boards |
| `file-03-screens-web-viewer.png` | 266 KB | Viewer: full-fidelity dark `web/thread-explorer` board (1/28) — workspace capture crashed (anomaly 1) |
| `file-04-screens-mobile-viewer.png` | 133 KB | Viewer: mobile screen board — same workspace-capture fallback |
| `file-05-screens-desktop-tui.png` | 176 KB | Workspace: desktop + TUI boards rendered |
| `file-06-platform-overrides.png` | 57 KB | Workspace: platform-override vector content |
| `file-07-prototypes.png` | 161 KB | Workspace: prototype boards rendered |

Tooling kept alongside: `snapshot.py` (RPC poller), `cdp_shots.py` / `cdp_resume.py` (CDP driver),
`files.json` (id map), `snapshots.jsonl` (raw evidence).

## 6. Instance anomalies observed

1. **`Page.captureScreenshot` → "Internal error"** on the workspace views of files 03 and 04 only
   (the two files with 28/22 large 2× PNG textures), reproducible across fresh tabs, clip capture,
   JPEG, and `fromSurface:false`; CDP also hit transient connection timeouts while those pages were
   rendering. Files 05/06/07 captured fine. Workaround: PenPot **view mode** rendered and captured
   both files without issue. This is a headless-capture limitation under heavy texture load, not
   data corruption — the RPC data and viewer renders are complete.
2. **Onboarding modal** ("Help us get to know you") blocked the first dashboard screenshot; removed
   from DOM (cosmetic only) for the final captures.
3. **Project-files dashboard route** rendered nearly empty twice in headless capture
   (`dashboard-project-helix-thready-final.png`, 6 KB); the Recent view was used as hydration
   evidence instead.
4. **Hydration incomplete at verification close**: file 06 has zero object thumbnails; only file 07
   has a file-level `thumbnailId`; dashboard cards for 02+06 show placeholder glyphs. Expected to
   self-heal as PenPot renders thumbnails lazily on access.
5. **Token count 71 vs 74 source tokens** — 3 source entries (alias/unsupported types, e.g.
   `--border-soft → var(--border)`) did not land as importable DTCG tokens. Matches the importer's
   own "71/74" claim; not a regression found by this verifier.
6. No RPC errors, no 5xx, no file-read failures across all 18 snapshots (`fileErrors=0` throughout).

## 7. Provenance

- Verifier ran **independently** of the importer; no writes were made to any PenPot file — RPC
  reads (`get-profile`, `get-teams`, `get-projects`, `get-project-files`, `get-team-recent-files`,
  `get-file`, `get-file-object-thumbnails`) and UI screenshots only. [VERIFIED — command list]
- Disposable browser container `thready-verify-cdp` (image `chromedp/headless-shell`, host network,
  CDP on 127.0.0.1:9333) — created for this verification and removed afterwards. Neither
  `helixcode-chromedp` nor `penpot-network` was touched.
- Credentials were read from the gitignored `.penpot-credentials`; no secret values appear in this
  report or in any evidence file.

*Made with love ♥ by Helix Development.*
