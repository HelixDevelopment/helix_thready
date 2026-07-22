<!--
  Title           : Helix Thready — Figma Import Kit (catalogue)
  Classification  : PUBLIC
  Location        : docs/public/research/mvp/design/figma/README.md
  Status          : Draft — v0.1 · blueprint proven in PenPot (2026-07-22); Figma materialization OPTIONAL
  Revision        : 2 (2026-07-22)
  Author          : Helix Thready documentation swarm (design · figma)
  Related         : ./figma-variables.json, ./figma-file-plan.md, ../opendesign/tokens.css,
                    ../opendesign/DESIGN.md, ../library/README.md, ../library/components-sheet.svg,
                    ../prototypes.md, ../index.md, ../../CONVENTIONS.md,
                    ../exports/penpot/verification/rpc-verification.json
-->

# Helix Thready — Figma Import Kit

| Rev | Date | Author | Change |
|-----|------|--------|--------|
| 1 | 2026-07-22 | swarm (design · figma) | Initial kit: complete token set as Figma Variables JSON (light+dark), 8-page file-plan blueprint for "Thready — Design Library", two materialization paths (MCP / manual); actual Figma file creation tracked as `[OPEN: THREADY-DES-FIG-01]` |
| 2 | 2026-07-22 | swarm (design · figma) | Disposition after the platform pivot (Constitution §11.4.220): the kit's blueprint was executed against **PenPot** (7 files, tokens 71/74, screens light+dark — evidence `../exports/penpot/verification/`); `FIG-01` re-scoped to the *optional, operator-triggered* Figma-cloud import; `FIG-03` closed (`../screens/mobile/channels.html` now exists) |

The bridge that materializes the Thready design system as a Figma library file — now an
**optional** one. Per [Constitution §11.4.220] the primary materialization lives in **PenPot**
and is **done** (see [Disposition](#disposition-2026-07-22)); this kit remains the executable
path for the optional Figma import/export target. It packs the canonical tokens
([`../opendesign/tokens.css`](../opendesign/tokens.css)) into Figma's Variables format and
specifies, page by page, the Figma file an agent (or a designer) builds from the existing
library and screen artifacts.

**Honest status:** **no Figma file exists — and per Constitution §11.4.220 none is required.**
The kit's blueprint **has been executed, against PenPot** (evidence:
[`../exports/penpot/verification/`](../exports/penpot/verification/)); the Figma materialization
is now the *optional* remainder of `[OPEN: THREADY-DES-FIG-01]` (re-scoped — see
[Open items](#open-items)), operator-triggered. Nothing in this directory claims a Figma
artifact exists. (The hi-fi-frames intent of `[OPEN: THREADY-DES-09]` is now carried by the
PenPot materialization; its disposition is owned by the design index.)

## Disposition (2026-07-22)

Constitution §11.4.220 (*open-first design tooling*, User mandate 2026-07-22) makes
**self-hosted PenPot the primary design platform**, consumed only via the dedicated PenPot
submodule (working name `vasic-digital/PenPot`); proprietary tools — Figma included — are
**optional import/export targets, never source of truth**, and design sources live in-repo in
open formats.

**The blueprint this kit specifies has been executed — against PenPot, not Figma.** On the local
instance (`http://localhost:9001`), project **"Helix Thready"**, the 8-page plan transposed
essentially 1:1 into **7 PenPot files**
`[VERIFIED — ../exports/penpot/verification/rpc-verification.json, verifiedAt 2026-07-22T15:42:41]`:

| Kit blueprint | PenPot materialization | Evidence (`rpc-verification.json`) |
|---|---|---|
| Page 1 Cover + Page 2 Foundations | **01 Foundations** — cover folded in (`logo-full`, `logo-mark`, `footer-slogan`, launcher icons are top-level there) | 324 objects; `topLevel` list |
| Variables layer ([`figma-variables.json`](./figma-variables.json)) | PenPot **token sets** `thready-light` / `thready-dark` / `thready-structure` + themes `Thready/Light` · `Thready/Dark`, imported from the token-bridge — **71 of 74** (PenPot's token model has no `duration`/`cubicBezier` types; motion durations + easing are carried as annotations instead) | token sets 19 + 19 + 33 |
| Page 3 Components (14 sets, variant axes) | **02 Components** — `components-sheet` + `library/components` light/dark imported as **editable vectors**; **native PenPot component sets pending rebuild** (variant axes not yet re-modeled as components) | 973 objects |
| Page 4 Screens · Web (14 × 1440w) | **03 Screens — Web**: 14 screens × light/dark | 28 frames / 28 images |
| Page 5 Screens · Mobile (8 × 390w) | **04 Screens — Mobile**: **11** screens × light/dark (incl. `mobile/channels` and `mobile/notifications`) | 22 frames / 22 images |
| Page 6 Screens · Desktop + TUI | **05 Screens — Desktop + TUI** | 4 frames / 4 images |
| Page 7 Platform overrides | **06 Platform Overrides** (`web-portal-ia`, `mobile-navigation`, `tui-navigation`, `white-label-cascade`) | 590 objects |
| Page 8 Prototype wiring | **07 Prototypes** (`prototype-hub/web-index` light/dark; interactive wiring pending) | 2 frames / 2 images |

The kit's SVG sources were imported as **editable vectors** and the PNG screen renders as
**underlays** (60 image objects across the 7 files
`[VERIFIED — rpc-verification.json objectCountsByType]`); browser **hydration is complete** —
per-file screenshots `hydrate-01…07.png` and `hydration-log.jsonl` sit next to the RPC evidence
in [`../exports/penpot/verification/`](../exports/penpot/verification/).

Consequences for this kit:

- **`FIG-01` re-scoped** (see [Open items](#open-items)): the Figma materialization is an
  optional, operator-triggered import — the blueprint itself is already proven.
- **[`figma-variables.json`](./figma-variables.json) remains valid, unchanged, for that optional
  Figma path** — it is Figma-specific by design and unaffected by the pivot.
- **`FIG-03` closed**: `mobile/channels-list` gained its mid-fi HTML source.
- Nothing below was retro-fitted to pretend it targeted PenPot: the plan reads as written
  (Figma vocabulary intact) and doubles as the specification the PenPot build followed — the
  transposition record lives in the plan's own disposition note.

## Catalogue

| File | What it is | How to use |
|---|---|---|
| [`figma-variables.json`](./figma-variables.json) | The **complete token set as Figma Variables**: collection `Thready / Color` (16 variables × **Light**/**Dark** modes, incl. `ds-heart` as a `VARIABLE_ALIAS` → `accent`) + collection `Thready / Structure` (25 FLOAT variables: spacing 4–48, radius sm/md/lg/pill, type sizes 12–64, leadings, motion 150/200, container-max 1200). Shape: Figma **Variables REST API bulk-change** payload with temporary ids; per-variable `scopes`, `description` (carrying the verbatim hex + provenance) and `codeSyntax.WEB`. Every hex is verbatim from `tokens.css`; floats are hex/255 — **nothing invented** | Strip the documentation keys (`jq 'del(._meta) \| walk(if type == "object" then del(._hex) else . end)'`), then POST to `/v1/files/{key}/variables` (**Enterprise token required**) or replay 1:1 through the Plugin API (`figma.variables.*`) — step 3 of the file plan's build order |
| [`figma-file-plan.md`](./figma-file-plan.md) | The **page-by-page blueprint** of the "Thready — Design Library" file: Cover · Foundations (variables, type ramp in Space Grotesk / Hanken Grotesk / JetBrains Mono, color roles light+dark, elevation, motion) · **14 component sets** with variant axes · Screens Web (14 × 1440w) / Mobile (8 × 390w, Android+iOS chrome variants) / Desktop + TUI (1 + 5 mono 80-col) · Platform overrides (6 boards) · Prototype wiring (4 ux-flows journeys) — plus the **build order & Plugin-API execution notes** (fonts preflight with `listAvailableFontsAsync`, Inter fallback flagged never silent, one page per `use_figma` call). **Proven blueprint** — executed against PenPot; its top disposition note records the transposition | Read top-to-bottom before building; execute §11 build order strictly; check off §12 acceptance |
| `README.md` (this file) | Catalogue + the two materialization paths + the PenPot disposition | Start here |

## Materialization path A — Figma MCP (`use_figma` Plugin API)

*(Optional — operator-triggered; see Disposition.)* Requires **OAuth** to Figma (the Figma MCP
server's `authenticate` flow) and a seat that can run plugin code.

1. Authenticate the Figma MCP server (OAuth) and create the empty file
   `Thready — Design Library`.
2. Drive the Plugin API through `use_figma`, executing
   [`figma-file-plan.md` §11](./figma-file-plan.md#11-build-order--plugin-api-execution-notes)
   in order: fonts preflight → variables (replay `figma-variables.json` via
   `figma.variables.createVariableCollection` / `createVariable` / `setValueForMode` /
   `createVariableAlias`) → text+effect styles → components → screens → overrides → prototype
   reactions. **One page per `use_figma` call.**
3. Run the acceptance checklist (file-plan §12), record the build in the revision tables, and
   close `[OPEN: THREADY-DES-FIG-01]`.

## Materialization path B — manual import

*(Optional — operator-triggered; see Disposition.)* No plugin execution; a designer with any
Figma plan.

1. Create the file `Thready — Design Library` with the 8 pages named in the file plan.
2. **Components:** drag [`../library/components-sheet.svg`](../library/components-sheet.svg)
   into the Components page — it is a valid standalone SVG with grouped, named layers, so it
   imports as structured vectors (light panel + dark panel). Use it as the tracing/reference
   base and rebuild the 14 component sets per file-plan §5, binding to variables as you go.
3. **Variables:** Figma's UI has **no built-in JSON variables import** — use either the REST
   bulk endpoint (`POST /v1/files/{key}/variables`, Enterprise token) with the stripped JSON, or
   a variables-import plugin that accepts the collection/mode/variable structure, or enter the
   41 variables by hand from the JSON (the `description` fields carry the verbatim hex values —
   no hex ever needs to be re-derived). Path decision tracked as `[OPEN: THREADY-DES-FIG-02]`.
4. Continue with screens/overrides/wiring per the file plan; the same acceptance checklist
   applies.

## Provenance (no bluff)

- Every color/number in the kit traces to [`../opendesign/tokens.css`](../opendesign/tokens.css)
  `[VERIFIED]`; contrast claims (accent 6.03:1 light / 13.56:1 dark, brand 1.47:1 decorative
  only) come from [`../opendesign/DESIGN.md`](../opendesign/DESIGN.md) §2 measurements.
- Component axes and counts trace to [`../library/README.md`](../library/README.md) (14 groups,
  ~38 components, ≈118 state cells — the file plan reconciles its 108 static variant cells
  against that number honestly, §5).
- Screen frames trace 1:1 to the artifacts under `../screens/{web,mobile,desktop,tui}/`.
  `mobile/channels-list`, non-artifact-backed at Rev 1, is now covered by
  [`../screens/mobile/channels.html`](../screens/mobile/channels.html)
  (`[CLOSED: THREADY-DES-FIG-03]`); the notifications flag `[OPEN: THREADY-DES-15]` stays as
  inherited.
- Deliberately **not** in the variables payload (documented in the JSON `_meta` and file-plan
  §2): CSS-only constructs — B-slot aliases, `color-mix()` hover/elevation/focus formulas,
  easing curve, section rhythm/gutters, font families, tracking.
- The PenPot execution evidence is first-party RPC output plus hydration screenshots:
  [`../exports/penpot/verification/rpc-verification.json`](../exports/penpot/verification/rpc-verification.json)
  `[VERIFIED — read for this revision]`.

## Open items

- `[OPEN: THREADY-DES-FIG-01]` — **re-scoped 2026-07-22** (was: *"create the actual Figma
  file… until then no Figma artifact exists"* — i.e. "no Figma file exists yet — this kit is
  the executable plan"). Current scope: **the Figma file is an optional import target.** The
  blueprint was proven by the PenPot materialization
  ([`../exports/penpot/verification/`](../exports/penpot/verification/)); `FIG-01` stays open
  **only** as the optional Figma-cloud import (path A or B above), **operator-triggered**, and
  is a dependency of nothing. Figma is an import/export target, never source of truth
  `[CONSTITUTION §11.4.220]`. [`figma-variables.json`](./figma-variables.json) remains valid,
  unchanged, for this path.
- `[OPEN: THREADY-DES-FIG-02]` — variables import mechanism (Enterprise REST vs Plugin-API
  replay vs plugin) — only relevant if `FIG-01`'s optional import is exercised.
- `[CLOSED: THREADY-DES-FIG-03]` — mid-fi HTML source for `mobile/channels-list` now exists:
  [`../screens/mobile/channels.html`](../screens/mobile/channels.html) (artifact Rev 1, added
  in the mobile catalogue Rev 2, 2026-07-22; re-verified present on disk for this revision).
  The corresponding `mobile/channels` frame is materialized light + dark in PenPot file
  *04 Screens — Mobile*
  `[VERIFIED — ../exports/penpot/verification/rpc-verification.json topLevel]`.
- Inherited: `[OPEN: THREADY-DES-02]` (PenPot/Lottie bridges — the PenPot half is materially
  advanced by the §11.4.220 pivot; disposition owned by the index),
  `[OPEN: THREADY-DES-04]` (Cyrillic subsets), `[OPEN: THREADY-DES-09]` (hi-fi frames — intent
  now carried by the PenPot materialization; disposition owned by the index),
  `[OPEN: THREADY-DES-11/12]` (component API names / upstream split),
  `[OPEN: THREADY-DES-13]` (PenPot's role — ruled by Constitution §11.4.220; disposition owned
  by the index).

---

*Made with love ♥ by Helix Development.*
