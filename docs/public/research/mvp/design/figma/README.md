<!--
  Title           : Helix Thready — Figma Import Kit (catalogue)
  Classification  : PUBLIC
  Location        : docs/public/research/mvp/design/figma/README.md
  Status          : Draft — v0.1
  Revision        : 1 (2026-07-22)
  Author          : Helix Thready documentation swarm (design · figma)
  Related         : ./figma-variables.json, ./figma-file-plan.md, ../opendesign/tokens.css,
                    ../opendesign/DESIGN.md, ../library/README.md, ../library/components-sheet.svg,
                    ../prototypes.md, ../index.md, ../../CONVENTIONS.md
-->

# Helix Thready — Figma Import Kit

| Rev | Date | Author | Change |
|-----|------|--------|--------|
| 1 | 2026-07-22 | swarm (design · figma) | Initial kit: complete token set as Figma Variables JSON (light+dark), 8-page file-plan blueprint for "Thready — Design Library", two materialization paths (MCP / manual); actual Figma file creation tracked as `[OPEN: THREADY-DES-FIG-01]` |

The **bridge that materializes the Thready design system as a real Figma library file**. It packs
the canonical tokens ([`../opendesign/tokens.css`](../opendesign/tokens.css)) into Figma's
Variables format and specifies, page by page, the Figma file an agent (or a designer) builds from
the existing library and screen artifacts.

**Honest status:** **no Figma file exists yet.** This kit is the *executable plan* — the
variables payload plus a blueprint precise enough to execute mechanically. Creating the actual
"Thready — Design Library" Figma file is `[OPEN: THREADY-DES-FIG-01]`. Nothing in this directory
claims otherwise. (It also *implements* the hi-fi-frames intent of `[OPEN: THREADY-DES-09]` —
that item closes only when the file is built.)

## Catalogue

| File | What it is | How to use |
|---|---|---|
| [`figma-variables.json`](./figma-variables.json) | The **complete token set as Figma Variables**: collection `Thready / Color` (16 variables × **Light**/**Dark** modes, incl. `ds-heart` as a `VARIABLE_ALIAS` → `accent`) + collection `Thready / Structure` (25 FLOAT variables: spacing 4–48, radius sm/md/lg/pill, type sizes 12–64, leadings, motion 150/200, container-max 1200). Shape: Figma **Variables REST API bulk-change** payload with temporary ids; per-variable `scopes`, `description` (carrying the verbatim hex + provenance) and `codeSyntax.WEB`. Every hex is verbatim from `tokens.css`; floats are hex/255 — **nothing invented** | Strip the documentation keys (`jq 'del(._meta) \| walk(if type == "object" then del(._hex) else . end)'`), then POST to `/v1/files/{key}/variables` (**Enterprise token required**) or replay 1:1 through the Plugin API (`figma.variables.*`) — step 3 of the file plan's build order |
| [`figma-file-plan.md`](./figma-file-plan.md) | The **page-by-page blueprint** of the "Thready — Design Library" file: Cover · Foundations (variables, type ramp in Space Grotesk / Hanken Grotesk / JetBrains Mono, color roles light+dark, elevation, motion) · **14 component sets** with variant axes · Screens Web (14 × 1440w) / Mobile (8 × 390w, Android+iOS chrome variants) / Desktop + TUI (1 + 5 mono 80-col) · Platform overrides (6 boards) · Prototype wiring (4 ux-flows journeys) — plus the **build order & Plugin-API execution notes** (fonts preflight with `listAvailableFontsAsync`, Inter fallback flagged never silent, one page per `use_figma` call) | Read top-to-bottom before building; execute §11 build order strictly; check off §12 acceptance |
| `README.md` (this file) | Catalogue + the two materialization paths | Start here |

## Materialization path A — Figma MCP (`use_figma` Plugin API)

Requires **OAuth** to Figma (the Figma MCP server's `authenticate` flow) and a seat that can run
plugin code.

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

No plugin execution; a designer with any Figma plan.

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
- Screen frames trace 1:1 to the artifacts under `../screens/{web,mobile,desktop,tui}/`; the two
  non-artifact-backed frames are flagged (`mobile/channels-list`
  `[OPEN: THREADY-DES-FIG-03]`, notifications `[OPEN: THREADY-DES-15]`).
- Deliberately **not** in the variables payload (documented in the JSON `_meta` and file-plan
  §2): CSS-only constructs — B-slot aliases, `color-mix()` hover/elevation/focus formulas,
  easing curve, section rhythm/gutters, font families, tracking.

## Open items

- `[OPEN: THREADY-DES-FIG-01]` — **create the actual Figma file** by executing the plan (path A
  or B); until then no Figma artifact exists.
- `[OPEN: THREADY-DES-FIG-02]` — variables import mechanism (Enterprise REST vs Plugin-API
  replay vs plugin).
- `[OPEN: THREADY-DES-FIG-03]` — `mobile/channels-list` frame lacks a mid-fi HTML source.
- Inherited: `[OPEN: THREADY-DES-02]` (PenPot/Lottie bridges — out of scope here),
  `[OPEN: THREADY-DES-04]` (Cyrillic subsets), `[OPEN: THREADY-DES-09]` (hi-fi frames — closed
  by executing this kit), `[OPEN: THREADY-DES-11/12]` (component API names / upstream split),
  `[OPEN: THREADY-DES-13]` (PenPot's role).

---

*Made with love ♥ by Helix Development.*
