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

## Open items

- `[OPEN: THREADY-DES-11]` / `[OPEN: THREADY-DES-12]` — inherited: final API/class names and the
  upstream-vs-Thready split for all PROPOSED components.
- `[OPEN: THREADY-DES-LIB-01..04]` — React re-audit, SwiftUI package decision, `helix_shims`
  inspection, token-bridge codegen — see [platform-map.md §6](./platform-map.md#6-open-items).
- `[OPEN: THREADY-DES-04]` — inherited: Cyrillic subsets of the three variable faces; this
  library's font stacks fall back to system faces until verified.

---

*Made with love ♥ by Helix Development.*
