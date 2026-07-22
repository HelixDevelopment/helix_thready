<!--
  Title           : Helix Thready — Design Library (catalogue)
  Classification  : PUBLIC
  Location        : docs/public/research/mvp/design/library/README.md
  Status          : Draft — v0.1
  Revision        : 1 (2026-07-22)
  Author          : Helix Thready documentation swarm (design)
  Related         : ./components.html, ./components-sheet.svg, ./platform-map.md,
                    ../component-library.md, ../design-system.md, ../theming.md,
                    ../../CONVENTIONS.md
-->

# Helix Thready — Design Library

| Rev | Date | Author | Change |
|-----|------|--------|--------|
| 1 | 2026-07-22 | swarm (design) | Initial library: living component page (all states, light+dark), overview SVG sheet, 8-platform reusability matrix |
| 2 | 2026-07-22 | swarm (design · review-fixes) | Consistency fixes from the adversarial platform review: explicit reconciliation of the "~38 distinct components" count with the 36×8 matrix (names the merged/excluded exhibits and the two matrix-only rows; 38 − 2 − 1 − 1 + 2 = 36); added the **Marker vocabulary** note mapping VERIFIED/PROPOSED/ASSUMED ↔ `[VERIFIED]`/`[RESEARCH]`/`[GAP: id]` for cross-file audits |

The **reusable design-library artifact** for Helix Thready: a self-contained, living rendering of
every component in every state on the Thready theme, plus an importable overview sheet and the
cross-platform reusability matrix. It *instantiates* the specs in
[component-library.md](../component-library.md), [design-system.md](../design-system.md) and
[theming.md](../theming.md) — it never contradicts them.

## Catalogue

| File | What it is | How to use |
|---|---|---|
| [`components.html`](./components.html) | The **living component library** — every component × state × light/dark, realistic Thready content, zero external requests (no CDNs; fonts fall back to the system stack unless the design-system faces are installed) | Open in any browser. Toggle **Theme: system → light → dark** (top right; stamps `data-theme`, persists to `localStorage['thready-theme']`). Anchor per group: `#buttons #inputs #cards #tables #badges-chips #navigation #dialogs #toasts-alerts #progress #avatars #empty-states #skeletons #pagination #tooltips` |
| [`components-sheet.svg`](./components-sheet.svg) | **One-sheet overview** of the core set, light panel + dark panel, hard-coded theme literals | Valid standalone SVG — import into PenPot / Figma (grouped, named layers) or view in a browser |
| [`platform-map.md`](./platform-map.md) | **Reusability matrix**: 36 components × 8 platforms (Angular `.ds-*` → React → Compose → Flutter → SwiftUI → ArkTS → Qt → TUI/Lipgloss) with per-cell VERIFIED / PROPOSED / ASSUMED markers and per-component customization notes | Read before implementing any component on any platform |

## Coverage

- **14 component groups**, ~38 distinct components, **75 rendered exhibits**,
  ≈ **118 component-state cells** — each auditable in **both** themes via the toggle
  (≈ 236 rendered states total).
- **Reconciliation with the 36×8 matrix** ([platform-map.md §3](./platform-map.md#3-the-matrix)):
  the ~38 distinct components collapse to **36** matrix rows because three exhibits counted here
  have no dedicated row — the **secondary** and **ghost** button variants share the single
  "Button primary/secondary/ghost" row, the **toast** shares the single "Toast / alert" row with
  the alert, and the **thread-row** card exhibit has no row of its own (it is a Thready composite
  of the Card + Badge + Hashtag-chip rows, specced in
  [component-library.md §6.1](../component-library.md#61-additional-component-contracts)) —
  while the matrix conversely adds two rows that are not distinct exhibits: "Button
  loading/disabled" (a state pairing) and "Field (label/hint/error)" (a wrapper contract).
  Net: 38 − 2 − 1 − 1 + 2 = 36.
- Buttons: 4 variants (primary / secondary / ghost / destructive) × 5 states
  (default / hover / focus-visible / disabled / loading) = 20 cells.
- Inputs: 6 controls (text / select / checkbox / radio / switch / date) × 4 states
  (default+hint / focus / error+hint / disabled) = 24 cells.
- Plus: cards (incl. `thready-stat` KPI + thread-row), a **live** sortable + paginated table,
  semantic badges, hashtag chips (direct vs. AI-indirect), processing-state chips
  (pending / processing / done / failed+retry / retrying), topbar / sidebar / tabs / breadcrumbs,
  static + live `<dialog>` modals, alerts + live toasts, determinate / indeterminate / failed
  progress, ring spinners + the **helix-motif spinner**, avatars (sizes / presence / group),
  empty + error states, skeleton loaders, pagination, and CSS-only tooltips (hover **and** focus).
- Interactive behaviors are real, not mocked: theme cycling, column sorting (`aria-sort`),
  table paging, `showModal()` dialog, auto-dismissing toast (pauses on hover). All motion honors
  `prefers-reduced-motion`.

## Provenance (no bluff)

- **VERIFIED** (reproduced verbatim; re-checked at source via `gh` on 2026-07-22): the token
  architecture and the shipped `.ds-*` set — `.ds-container/.ds-section`, `.ds-btn`
  (`--primary/--secondary/--ghost`), `.ds-card`(`--raised`), `.ds-input`, `.ds-link`,
  `.ds-nav`(`__links/__link`), `.ds-footer`, `.ds-badge`(`--success/--warn/--danger`),
  `.ds-brand-mark` — from `vasic-digital/design_system` `components/css/components.css`.
- **PROPOSED** `[DEFAULT — adjustable]`: everything else (`.ds-btn--danger`, `.ds-select`,
  `.ds-check/.ds-radio/.ds-switch`, `.ds-table`, `.ds-tabs`, `.ds-breadcrumbs`, `.ds-sidebar`,
  `.ds-dialog`, `.ds-alert/.ds-toast`, `.ds-progress`, `.ds-spinner`, `.ds-avatar`, `.ds-empty`,
  `.ds-skeleton`, `.ds-pagination`, `.ds-tip`, and the `thready-*` composites). Upstream-candidate
  names pending `[OPEN: THREADY-DES-11/12]`.
- Theme values are the ground-truth Thready theme: brand `#B6E376`, teal secondary
  `#ABDDC9` (light) / `#B7EBD6` (dark), accent `#446E12` light / `#B6E376` dark, fg `#020817` /
  `#F8FAFC`, neutrals `#475569` / `#94A3B8`; faces Space Grotesk / Hanken Grotesk / JetBrains Mono.
- Platform verification detail (what is real vs. scaffold per repo) lives in
  [platform-map.md §2](./platform-map.md#2-per-repo-verification-results).

**Marker vocabulary (cross-file equivalence).** Two provenance vocabularies coexist in this design
area, both implementing the same CONVENTIONS §3/§7 "no bluff" discipline. This library and its
platform map (plus the Lipgloss theme material in design-system.md §7) use **VERIFIED / PROPOSED /
ASSUMED**; the asset documents ([icon-export-matrix.md](../assets/icon-export-matrix.md),
brand-assets.md) use **`[VERIFIED]` / `[RESEARCH]` / `[GAP: id]`**. The mapping for cross-file
audits: **VERIFIED ≈ `[VERIFIED]`** (checked at source on a dated pass — repo content via `gh`, or
platform-canonical docs); **PROPOSED ≈ a `[DEFAULT — adjustable]` proposal backed by `[RESEARCH]`**
(a value this design area chooses and stands behind, but that must be confirmed — upstream naming
or current OS docs — at integration); **ASSUMED ≈ `[GAP: id]`** (the consuming package/subsystem is
a scaffold or was not inspected; nothing so marked may be claimed to work). `[RESEARCH]` has no
exact single counterpart here: when it flags an externally sourced value pending re-verification it
reads as PROPOSED; when the consumer itself is unverified it reads as ASSUMED. Neither vocabulary
is being rewritten into the other — this note is the documented equivalence.

## Open items

- `[OPEN: THREADY-DES-11]` / `[OPEN: THREADY-DES-12]` — inherited: final API/class names and the
  upstream-vs-Thready split for all PROPOSED components.
- `[OPEN: THREADY-DES-LIB-01..04]` — React re-audit, SwiftUI package decision, `helix_shims`
  inspection, token-bridge codegen — see [platform-map.md §6](./platform-map.md#6-open-items).
- `[OPEN: THREADY-DES-04]` — inherited: Cyrillic subsets of the three variable faces; this
  library's font stacks fall back to system faces until verified.

---

*Made with love ♥ by Helix Development.*
