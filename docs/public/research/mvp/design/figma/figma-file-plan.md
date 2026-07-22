<!--
  Title           : Helix Thready — Figma File Plan ("Thready — Design Library")
  Classification  : PUBLIC
  Location        : docs/public/research/mvp/design/figma/figma-file-plan.md
  Status          : Draft — v0.1
  Revision        : 1 (2026-07-22)
  Author          : Helix Thready documentation swarm (design · figma)
  Related         : ./README.md, ./figma-variables.json, ../opendesign/tokens.css,
                    ../opendesign/DESIGN.md, ../library/README.md, ../library/platform-map.md,
                    ../screens/web/README.md, ../screens/mobile/README.md,
                    ../screens/desktop/README.md, ../screens/tui/README.md,
                    ../ux-flows.md, ../motion/motion.md, ../../CONVENTIONS.md
-->

# Helix Thready — Figma File Plan ("Thready — Design Library")

| Rev | Date | Author | Change |
|-----|------|--------|--------|
| 1 | 2026-07-22 | swarm (design · figma) | Initial 8-page blueprint: foundations, 14 component sets with variant axes, web/mobile/desktop/TUI screen frames, platform overrides, prototype wiring, Plugin-API build order. Every value traced to `tokens.css` / `DESIGN.md` / `library/` / `screens/` — nothing invented |

The page-by-page blueprint of the **"Thready — Design Library"** Figma file, precise enough for
an agent with Figma Plugin API access (Figma MCP `use_figma`) to execute mechanically. The Figma
file **does not exist yet** — building it is `[OPEN: THREADY-DES-FIG-01]`. Until then this plan
plus [`figma-variables.json`](./figma-variables.json) *is* the deliverable.

## Table of contents

- [1. Ground truth & non-negotiables](#1-ground-truth--non-negotiables)
- [2. File-level conventions](#2-file-level-conventions)
- [3. Page 1 — Cover](#3-page-1--cover)
- [4. Page 2 — Foundations](#4-page-2--foundations)
- [5. Page 3 — Components (14 sets)](#5-page-3--components-14-sets)
- [6. Page 4 — Screens · Web (14 frames, 1440w)](#6-page-4--screens--web-14-frames-1440w)
- [7. Page 5 — Screens · Mobile (8 frames, 390w)](#7-page-5--screens--mobile-8-frames-390w)
- [8. Page 6 — Screens · Desktop + TUI](#8-page-6--screens--desktop--tui)
- [9. Page 7 — Platform overrides](#9-page-7--platform-overrides)
- [10. Page 8 — Prototype wiring](#10-page-8--prototype-wiring)
- [11. Build order & Plugin-API execution notes](#11-build-order--plugin-api-execution-notes)
- [12. Acceptance checklist](#12-acceptance-checklist)
- [13. Open items](#13-open-items)

## 1. Ground truth & non-negotiables

- **Tokens:** [`../opendesign/tokens.css`](../opendesign/tokens.css) is canonical; the Figma
  Variables in [`figma-variables.json`](./figma-variables.json) carry it verbatim
  `[VERIFIED — generated 1:1, hex → r/g/b floats]`. **Never restate a hex inline in Figma** —
  bind fills/strokes/radii/gaps/font-sizes to variables; anything unbindable gets an annotation
  naming its token.
- **Brand contract:** [`../opendesign/DESIGN.md`](../opendesign/DESIGN.md) — brand is decorative,
  accent is interactive, semantic is state; `--danger` never masked by brand; one brand-gradient
  focal element per screen; locked footer *"Made with ♥ by Helix Development"* on every screen.
- **Component inventory:** [`../library/README.md`](../library/README.md) — 14 groups, ~38
  components, ≈118 component-state cells; `components.html` is the visual source the Figma
  components must reproduce, not reinterpret.
- **Screens:** the `../screens/{web,mobile,desktop,tui}/` HTML artifacts are the mid/high-fi
  source of every frame — copy their **realistic Thready data** (no lorem ipsum) and their honest
  gap banners (`[GAP: 5.1]` Max stub, `[GAP: 2.1]` degraded search, etc.).
- **No invented values** `[CONSTITUTION §11.4.6]` — if a needed value is missing, flag it and use
  the nearest token.

## 2. File-level conventions

- **File name:** `Thready — Design Library`. **Pages** exactly as numbered below (Figma page
  names: `1 · Cover`, `2 · Foundations`, `3 · Components`, `4 · Screens — Web`,
  `5 · Screens — Mobile`, `6 · Screens — Desktop + TUI`, `7 · Platform Overrides`,
  `8 · Prototype Wiring`).
- **Modes:** collection `Thready / Color` has modes **Light** and **Dark**; components are built
  once and previewed in both by section frames with an explicit mode override (light section =
  Light mode, dark section = Dark mode). Collection `Thready / Structure` is single-mode.
- **Frame naming:** `web/<screen>`, `mobile/<screen>/<android|ios>`, `desktop/shell`,
  `tui/<screen>`, `overrides/<platform>` — stable names, referenced by the prototype wiring.
- **Fonts:** Space Grotesk (display, 600–700), Hanken Grotesk (body, 400–500; buttons 500,
  badges 600), JetBrains Mono (code/metadata, tabular figures)
  `[VERIFIED — DESIGN.md §3]`. Availability check + substitution rule in §11 step 2.
- **Elevation & focus (effect styles,** because shadows are not variables**):**
  `elev/ring` = inner-shadow-like 1px stroke or drop shadow `0 0 0 1px` in `border`;
  `elev/raised` = drop shadow `0 2px 8px` of `fg` at **8% opacity** (faithful translation of
  `color-mix(in oklab, var(--fg), transparent 92%)`); `focus/ring` = `0 0 0 3px` of `accent` at
  **30% opacity** (`transparent 70%`) `[VERIFIED formulas — tokens.css]`. Effect colors bind to
  the `fg`/`border`/`accent` variables so both modes resolve correctly.
- **Not carried as variables (deliberate,** mirrors `figma-variables.json` `_meta`**):** B-slot
  aliases `fg-2`/`meta`/`border-soft`; `accent-hover`/`accent-active` (oklab mixes — baked into
  hover-state variants per mode with an annotation naming the formula); `ease-standard`
  (prototype easing setting `cubic-bezier(0.2, 0, 0, 1)`); section rhythm 80/48/32 and gutters
  24/16/12 (layout annotations on screen frames); `tracking-display` −0.01em (text styles).

## 3. Page 1 — Cover

One frame `cover` (1440×1024):

- Brand mark from [`../assets/logo-mark.svg`](../assets/logo-mark.svg) (import the SVG — never
  redraw), title **Thready — Design Library** in Space Grotesk 700 / `text-3xl` 48, tagline
  *"read your threads, smarter"* `[VERIFIED — DESIGN.md §1 Voice & tone]` in Hanken Grotesk /
  `text-lg`.
- The brand gradient `brand → brand-2` as the single decorative focal element (one per screen —
  contract rule).
- Status block (JetBrains Mono, `text-sm`): file status **Draft v0.1**, source-of-truth pointers
  (this plan + `tokens.css`), the build date, and — if §11 step 2 substituted any font — the
  **red substitution notice** (see §11).
- Locked footer lockup: *"Made with ♥ by Helix Development"*, heart bound to `ds-heart`.

## 4. Page 2 — Foundations

Five section frames, each duplicated Light/Dark via mode override where color-bearing:

1. **`foundations/variables`** — two auto-layout tables rendering the imported collections:
   `Thready / Color` (16 variables × Light/Dark swatches + hex labels sourced from the variable
   values, not typed by hand) and `Thready / Structure` (25 variables: 8 spacing, 4 radius,
   8 text sizes, 2 leadings, 2 motion durations, container-max). Note under the table: `ds-heart`
   is an **alias → accent**; `fg-2`/`meta`/`border-soft` are CSS B-slot aliases not imported.
2. **`foundations/type`** — the type ramp in the three faces. Text styles to create (names
   `[DEFAULT — adjustable]`, values `[VERIFIED — tokens.css]`):

   | Style | Face / weight | Size (variable) | Line-height | Tracking |
   |---|---|---|---|---|
   | `display/4xl` | Space Grotesk 700 | `text-4xl` 64 | 120% (`leading-tight` 1.2) | −1% (−0.01em) |
   | `display/3xl` | Space Grotesk 700 | `text-3xl` 48 | 120% | −1% |
   | `heading/2xl` | Space Grotesk 600 | `text-2xl` 32 | 120% | −1% |
   | `heading/xl` | Space Grotesk 600 | `text-xl` 24 | 120% | −1% |
   | `title/lg` | Space Grotesk 600 | `text-lg` 20 | 120% | −1% |
   | `body/base` | Hanken Grotesk 400 | `text-base` 16 | 150% (`leading-body` 1.5) | 0 |
   | `body/medium` | Hanken Grotesk 500 | `text-base` 16 | 150% | 0 |
   | `small/sm` | Hanken Grotesk 400 | `text-sm` 14 | 150% | 0 |
   | `caption/xs` | Hanken Grotesk 400 | `text-xs` 12 | 150% | 0 |
   | `mono/base` | JetBrains Mono 400 | `text-base` 16 | 150% | 0 (tabular figures ON) |
   | `mono/sm` | JetBrains Mono 400 | `text-sm` 14 | 150% | 0 |

   Line-height ratios 1.5/1.2 are stored as unitless floats in the Structure collection but MUST
   be applied as **percentages** in text styles (Figma has no unitless line-height).
3. **`foundations/color-roles`** — the role grid from DESIGN.md §2 with the measured contrast
   annotations: accent **6.03:1** on white (light) / **13.56:1** on `#020817` (dark), brand
   **1.47:1** (decorative-only warning), `accent`≠`danger` contract note
   `[VERIFIED — DESIGN.md §2]`.
4. **`foundations/elevation`** — three cards demonstrating `elev/flat` (none), `elev/ring`
   (preferred in dark), `elev/raised`, plus the `focus/ring` demo; formulas printed verbatim.
5. **`foundations/motion`** — spec board: `motion-fast` **150ms** (hover/press),
   `motion-base` **200ms** (enter/exit), easing `cubic-bezier(0.2, 0, 0, 1)`; all motion honors
   `prefers-reduced-motion` `[VERIFIED — tokens.css]`. Reference (annotation, not embed —
   Figma can't play Lottie): the shipped Lottie set in [`../motion/`](../motion/README.md)
   (`helix-spinner`, `processing-pulse`, `success-check`, `error-cross`, `thread-sync`,
   `transition-fade-slide`).

## 5. Page 3 — Components (14 sets)

The 14 groups of [`../library/README.md`](../library/README.md) (anchors `#buttons` …
`#tooltips`), each as a Figma **component set** with variant properties. Visual truth is
`../library/components.html`; state styling (hover mixes, focus ring, disabled at reduced
opacity, error = `danger`) follows `DESIGN.md §4` and the shipped `.ds-btn` CSS.

| # | Group (library anchor) | Component set(s) | Variant properties | Cells |
|---|---|---|---|---|
| 1 | Buttons `#buttons` | `Button` | `variant` = primary / secondary / ghost / destructive; `state` = default / hover / focus / disabled / loading | **20** `[VERIFIED count — library README]` |
| 2 | Inputs `#inputs` | `Input` | `control` = text / select / checkbox / radio / switch / date; `state` = default(+hint) / focus / error(+hint) / disabled | **24** `[VERIFIED count — library README]` |
| 3 | Cards `#cards` | `Card` [`elevation` = flat / ring / raised]; `StatCard` (thready-stat KPI); `ThreadRow` [`state` = default / hover / focus] | axes at left | 7 |
| 4 | Tables `#tables` | `TableHeaderCell` [`sort` = none / asc / desc]; `TableRow` [`state` = default / hover]; `Table` (composite incl. pagination strip) ×2 | axes at left | 7 |
| 5 | Badges & chips `#badges-chips` | `Badge` [`tone` = success / warn / danger / neutral]; `TagChip` [`origin` = direct / indirect "(derived)"]; `ProcessingChip` [`state` = pending / processing / done / failed(+retry) / retrying] | axes at left | 11 |
| 6 | Navigation `#navigation` | `Topbar`; `Sidebar` [`state` = expanded / collapsed]; `SidebarItem` [`state` = default / active / hover]; `Tab` [`state` = default / active / focus]; `Breadcrumbs` | axes at left | 10 |
| 7 | Dialogs `#dialogs` | `Dialog` | `kind` = standard / destructive-confirm | 2 |
| 8 | Toasts & alerts `#toasts-alerts` | `Alert` [`tone` = success / warn / danger]; `Toast` [`role` = status / alert] | axes at left | 5 |
| 9 | Progress `#progress` | `ProgressBar` [`state` = determinate / indeterminate / failed]; `SpinnerRing`; `SpinnerHelix` (helix-motif) | axes at left | 5 |
| 10 | Avatars `#avatars` | `Avatar` [`size` = sm / md / lg; `presence` = none / online]; `AvatarGroup` | axes at left | 7 |
| 11 | Empty states `#empty-states` | `EmptyState` | `kind` = empty / error(+retry) | 2 |
| 12 | Skeletons `#skeletons` | `Skeleton` | `shape` = text-row / card | 2 |
| 13 | Pagination `#pagination` | `PaginationItem` [`state` = default / current / disabled]; `Pagination` (composite `‹ 1 [2] 3 ›`) | axes at left | 4 |
| 14 | Tooltips `#tooltips` | `Tooltip` | `trigger` = hover / focus | 2 |

**Count reconciliation (honest):** rows 1–2 are the library's own verified counts (20 + 24).
The remaining axes are derived from the exhibits listed in the library README; this plan
enumerates **108 static variant cells** against the library's **≈118 state cells** — the delta
is *behavioral* states (live table sorting/paging, toast auto-dismiss + hover-pause, `<dialog>`
open/close, theme cycling) that are interactions, not static variants. The builder MUST
reconcile the finished sets against `components.html` exhibit-by-exhibit and record the final
per-group counts on the page. The 14 sets fold the library's ~38 distinct components into
variant axes (e.g. `Input` folds 6 controls), so set-count ≠ component-count by design.
Component API names remain `[OPEN: THREADY-DES-11]`; upstream-vs-Thready split
`[OPEN: THREADY-DES-12]`.

**Page layout:** one section frame per group, Light column + Dark column (mode override), each
cell labeled `variant · state` in `mono/sm`. Both-mode preview target ≈ 2 × cells.

## 6. Page 4 — Screens · Web (14 frames, 1440w)

One frame per artifact in [`../screens/web/`](../screens/web/README.md), **1440×auto**, content
copied from the HTML (same realistic data, same state banners), composed from Page-3 instances.
Layout constants: `container-max` 1200 centered, 24px desktop gutters, 80px section rhythm
(annotated, not variables).

| Frame | Source artifact | Screen |
|---|---|---|
| `web/login` | `login.html` | Login / MFA |
| `web/dashboard` | `dashboard.html` | Dashboard (stats 128 / 1,204 / 17 / 42.1 TB, live activity, queue) |
| `web/channels` | `channels.html` | Channels list + Add-Channel wizard modal |
| `web/thread-explorer` | `thread-explorer.html` | Channel detail / thread list |
| `web/post-detail` | `post-detail.html` | Post detail / processing pipeline |
| `web/search` | `search.html` | Search (semantic/keyword/hybrid, scores 0.94/0.91/0.88) |
| `web/assets-browser` | `assets-browser.html` | Assets grid/list + detail drawer |
| `web/skills-manager` | `skills-manager.html` | Skills / Recipes + skill graph |
| `web/research-viewer` | `research-viewer.html` | Generated research doc |
| `web/accounts-admin` | `accounts-admin.html` | Admin — Accounts / Users & roles / Audit |
| `web/billing` | `billing.html` | Billing & usage (amounts stay `—` placeholders `[OPEN: THREADY-DES-SCR-01]`) |
| `web/settings` | `settings.html` | Settings — Prefs / Branding white-label (AA meter) / Messengers |
| `web/events-monitor` | `events-monitor.html` | Events monitor (live tail) |
| `web/index` | `index.html` | Prototype shell / gallery (becomes the Page-8 hub frame) |

Each frame ships Light; a Dark duplicate (`web/<screen>/dark`) is generated for the four
journey-critical screens (dashboard, post-detail, search, settings) — full dark duplication of
all 14 is deferred to the build `[DEFAULT — adjustable]`.

## 7. Page 5 — Screens · Mobile (8 frames, 390w)

Frames at **390×844** from [`../screens/mobile/`](../screens/mobile/README.md). Each screen gets
**two platform-annotation variants** — `mobile/<screen>/android` (Material 3 chrome: punch-hole
status bar, ← + predictive back, 80dp pill-indicator nav bar) and `mobile/<screen>/ios`
(Dynamic Island, `‹ Title` + edge swipe, 49pt tab bar + home indicator) — mirroring the
artifacts' chrome toggle. 8 screens × 2 = 16 frames.

| Frame base | Source artifact | Screen | Status |
|---|---|---|---|
| `mobile/home-feed` | `home-feed.html` | Home — live activity + processing + threads | grounded |
| `mobile/channels-list` | — **no HTML artifact** | Channels tab list (`#ml-papers` ✓ / `#films` ⭮ / `Max: dev-notes` ⚠) | **derived** from wireframes §6 IA (`CHLIST` node) + `channel-threads.html` chrome `[DEFAULT — adjustable]` `[OPEN: THREADY-DES-FIG-03]` |
| `mobile/channel-threads` | `channel-threads.html` | Channel detail — thread list | grounded |
| `mobile/post-detail` | `post-detail.html` | Post detail — tags, pipeline, Reprocess | grounded |
| `mobile/search` | `search.html` | Search — modes, scope, scored results | grounded |
| `mobile/notifications` | `notifications.html` | Notifications centre | derived surface `[OPEN: THREADY-DES-15]` |
| `mobile/account` | `account.html` | Account — switcher, RBAC, sign out | grounded |
| `mobile/settings` | `settings.html` | Settings — theme/language, branding (read-only `[OPEN: THREADY-DES-06]`), messengers | grounded |

Honesty banners carried into the frames: `Security-KMP` `[GAP: 7.3]` and `UI-Components-KMP`
`[GAP: 8.4]` are mobile **release gates**; Max adapter `[GAP: 5.1]`; degraded search
`[GAP: 2.1]`.

## 8. Page 6 — Screens · Desktop + TUI

- **`desktop/shell`** — one **1440×900** frame from
  [`../screens/desktop/desktop-shell.html`](../screens/desktop/README.md): Tauri window, per-OS
  title-bar strip (macOS traffic lights / Windows caption buttons / Linux CSD), native menu map,
  wrapped Dashboard (instance of `web/dashboard` content), tray popover mock + native
  notification mock both labeled `[OPEN: THREADY-DES-08]`, file-drop overlay (media-file case
  labeled `[OPEN: THREADY-DES-16]`).
- **Five TUI frames** from [`../screens/tui/tui-screens.html`](../screens/tui/README.md), each a
  **mono-text 80-column** frame: JetBrains Mono 16/120%, fixed width 800px
  (80ch ≈ 768px + padding `[DEFAULT — adjustable]`), **dark palette only** — the TUI defaults to
  the terminal's dark surface (design-system §7); box-drawing text pasted verbatim from the
  artifact so alignment survives.

| Frame | TUI screen | Status |
|---|---|---|
| `tui/dashboard` | 1 · Dashboard (key rail + live viewport + threads + statusbar) | grounded |
| `tui/channels` | 2 · Channels (inverse-accent selection) | grounded |
| `tui/thread-view` | 3 · Thread view / Post detail (409 idempotent retry) | grounded |
| `tui/search` | 4 · Search (degraded-embedder banner `[GAP: 2.1]`) | grounded |
| `tui/processing-queue` | 5 · Processing queue | derived `[DEFAULT — adjustable]` |

ANSI-16 mappings on these frames are annotations from
[`../screens/tui/lipgloss-theme.md`](../screens/tui/lipgloss-theme.md) and stay ASSUMED pending
terminal verification `[OPEN: THREADY-DES-17]`.

## 9. Page 7 — Platform overrides

Six annotation frames (`overrides/<platform>`), each a callout board sourced from
[`../library/platform-map.md`](../library/platform-map.md) §2/§4 and design-system §7 — the
per-platform customization deltas plus the **honest scaffold status** (never claim a scaffold
works):

| Frame | Content highlights | Status callout |
|---|---|---|
| `overrides/android` | Compose realizations (`TopAppBar`, `Snackbar`, `DatePickerDialog`…), focus → ripple, native date sheet, `ANIMATOR_DURATION_SCALE` reduced-motion | `UI-Components-KMP` = utilities-only scaffold, **foreign (Yole) palette** `[GAP: 8.4]` — all widget cells ASSUMED |
| `overrides/ios` | SwiftUI mappings (`Picker` instead of radio groups — documented divergence; `.redacted` skeletons; `UIAccessibility.isReduceMotionEnabled`) | no in-house SwiftUI package — column entirely ASSUMED `[OPEN: THREADY-DES-LIB-02]` |
| `overrides/harmonyos` | ArkTS mappings (`promptAction.showToast`, `Toggle(Switch)` accent track) | native via `helix_shims`, uninspected `[GAP: 8.5]` `[OPEN: THREADY-DES-LIB-03]` |
| `overrides/aurora` | Qt/QML mappings (`TableView`, `BusyIndicator`), layered icon JSON | `helix_design` = **empty scaffold** `[GAP: 8.2/8.3]`; density buckets `[OPEN: THREADY-DES-05]` |
| `overrides/desktop` | Tauri chrome, accelerators additive to verified web keys, close-to-tray, offline cache contract | tray/notifications scope `[OPEN: THREADY-DES-08]` |
| `overrides/tui` | Lipgloss role styles, `[r]etry` key binding, dimmed+`~` indirect tags, braille helix spinner | pattern VERIFIED (llms_verifier); Thready style set PROPOSED; ANSI picks ASSUMED `[OPEN: THREADY-DES-17]` |

## 10. Page 8 — Prototype wiring

The four [`../ux-flows.md`](../ux-flows.md) journeys, mirroring the working shell
`../screens/web/index.html` §4. Hub frame: `web/index`. Default transition: **Smart Animate,
200ms (`motion-base`), easing `cubic-bezier(0.2, 0, 0, 1)`**; overlays (wizard modal, drawers)
open as Figma overlays with **Dissolve 150ms (`motion-fast`)**; mobile back = swipe-right
gesture on iOS variants, back-hotspot on Android variants.

| Journey (ux-flows §) | Frame chain (trigger = ON_CLICK on the named hotspot) |
|---|---|
| 1 · Add channel (§2 + §2.1) | `web/login` [Sign in] → `web/dashboard` [Add channel] → `web/channels` [+ Add] ⇒ overlay wizard steps 1–4 (incl. messenger sign-in sub-flow §2.1 as nested overlay) → `web/thread-explorer` (backfilling 54/128 state) |
| 2 · Process post + Reprocess (§3 + §3.1) | `web/dashboard` [thread row] → `web/thread-explorer` [post] → `web/post-detail` [pipeline step] → `web/events-monitor` [research.md] → `web/research-viewer` [assets] → `web/assets-browser`; back on `web/post-detail`: [Reprocess] ⇒ confirm overlay → 202 state; second click ⇒ 409 toast state |
| 3 · Search (§4) | `web/search` [result 0.94 · post] → `web/post-detail`; [doc result] → `web/research-viewer` |
| 4 · Manage account (§5) | `web/accounts-admin` [Billing] → `web/billing` [Settings] → `web/settings` (branding editor: [Save] wired to the 422-AA-gate state, then success) |

Error/degraded states are wired as **variant swaps inside the frame** (e.g. search →
degraded-embedder banner), not separate journeys. Endpoint/event names on annotations stay
`[DEFAULT — adjustable]` until `[OPEN: THREADY-DES-10]` closes.

## 11. Build order & Plugin-API execution notes

Execute strictly in this order — later steps consume earlier ones:

1. **Create the file** `Thready — Design Library` (Figma MCP `figma-create-new-file`, or
   manually — see [`README.md`](./README.md) path B).
2. **Fonts preflight.** `figma.listAvailableFontsAsync()`; require *Space Grotesk*, *Hanken
   Grotesk*, *JetBrains Mono*. If Space Grotesk or Hanken Grotesk is unavailable → substitute
   **Inter** `[OPERATOR-specified fallback]`; if JetBrains Mono is unavailable → substitute
   **Roboto Mono** `[DEFAULT — adjustable]`. **Every substitution is flagged, never silent:**
   red notice on the Cover, `(SUBSTITUTED)` suffix on affected text-style descriptions, and a
   line in the build log.
3. **Variables.** Import [`figma-variables.json`](./figma-variables.json): strip `_`-prefixed
   keys, then replay via Plugin API — `figma.variables.createVariableCollection("Thready / Color")`
   (rename default mode → Light, `addMode("Dark")`), `createVariableCollection("Thready / Structure")`
   (default mode → Value), then per variable `createVariable(name, collection, type)` +
   `setValueForMode(...)` per mode + `scopes`/`description`/`codeSyntax` from the JSON;
   `ds-heart` set with `figma.variables.createVariableAlias(accent)` in both modes. (The REST
   bulk endpoint `POST /v1/files/{key}/variables` accepts the file as-is but requires an
   Enterprise token — see README.)
4. **Styles.** Text styles (§4 table, sizes bound to `text-*` variables) and effect styles
   (`elev/ring`, `elev/raised`, `focus/ring` — colors bound to variables).
5. **Page 2 Foundations** frames (§4).
6. **Page 3 Components** — one group per `use_figma` call (14 calls minimum; payload limits):
   build set → bind every fill/stroke/radius/gap/font-size to a variable → verify against
   `components.html` → label cells.
7. **Pages 4–6 Screens** — one page per `use_figma` call, screens composed from Page-3
   instances; explicit Dark mode override on dark sections; realistic data copied from the HTML
   artifacts.
8. **Page 7 Platform overrides** annotation frames.
9. **Page 8 Prototype wiring** — `setReactionsAsync` per the §10 table (Smart Animate 200ms /
   Dissolve 150ms, easing `cubic-bezier(0.2, 0, 0, 1)`).
10. **Acceptance pass** (§12) + record the run in this file's revision table and close/update
    `[OPEN: THREADY-DES-FIG-01]`.

**Standing rules for the executing agent:** one page per `use_figma` call; never hard-code a
hex — bind a variable or annotate the token name; never redraw brand assets — import the SVGs
from `../assets/`; when anything cannot be reproduced (font, Lottie, live behavior), annotate
the limitation on-canvas rather than approximating silently.

## 12. Acceptance checklist

- [ ] 41 variables imported (16 color × Light/Dark incl. `ds-heart` alias, 25 structure) —
      values diff clean against `tokens.css` (script-diff hex ↔ r/g/b floats).
- [ ] 11 text styles + 3 effect styles created; substitutions (if any) flagged on Cover.
- [ ] 14 component sets built; per-group cell counts recorded and reconciled vs `components.html`
      (plan enumerates 108 static cells; library counts ≈118 incl. behavioral states).
- [ ] Frames: 1 cover + 5 foundations + 14 web + 16 mobile (8 × android/ios) + 1 desktop +
      5 TUI + 6 overrides.
- [ ] 4 prototype journeys wired per §10; transitions 200ms/150ms on `cubic-bezier(0.2,0,0,1)`.
- [ ] Contrast annotations present (6.03:1 / 13.56:1 / 1.47:1); `danger` never brand-tinted.
- [ ] Honesty markers rendered on-canvas: `[GAP: 5.1]` Max, `[GAP: 2.1]` degraded search,
      `[GAP: 8.4]`/`[GAP: 7.3]` mobile gates, `[OPEN: THREADY-DES-08/15/16/17]`.
- [ ] Locked footer *"Made with ♥ by Helix Development"* on the cover and every screen frame,
      heart bound to `ds-heart`.

## 13. Open items

- `[OPEN: THREADY-DES-FIG-01]` — **the Figma file does not exist yet**; this plan is the
  executable blueprint. Requires Figma MCP OAuth (or manual path). Owner: design · figma.
- `[OPEN: THREADY-DES-FIG-02]` — variables import path decision: Plugin API replay (any plan)
  vs REST bulk `POST /v1/files/{key}/variables` (Enterprise-only token).
- `[OPEN: THREADY-DES-FIG-03]` — `mobile/channels-list` has no mid-fi HTML artifact; the frame
  is derived from wireframes §6 IA and must be back-ported into `../screens/mobile/` (or the
  frame count revised) once the screens area rules on it.
- Inherited, affecting this file: `[OPEN: THREADY-DES-04]` (Cyrillic subsets — affects Figma
  font choice for ru/sr-Cyrl frames), `[OPEN: THREADY-DES-09]` (these frames ARE the hi-fi
  pass — closing it happens by executing this plan), `[OPEN: THREADY-DES-11/12]` (component
  names), `[OPEN: THREADY-DES-13]` (PenPot mirror role), `[OPEN: THREADY-DES-02]`
  (PenPot/Lottie bridges are separate deliverables, not Figma work).

---

*Made with love ♥ by Helix Development.*
