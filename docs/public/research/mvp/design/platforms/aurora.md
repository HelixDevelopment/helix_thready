<!--
  Title           : Helix Thready — Aurora OS (Qt / Silica) Customization Spec
  Classification  : PUBLIC
  Location        : docs/public/research/mvp/design/platforms/aurora.md
  Status          : Draft — v0.1
  Revision        : 1 (2026-07-22)
  Author          : Helix Thready documentation swarm (design · platforms)
  Related         : ./README.md, ./typography-substitution.md, ../library/platform-map.md,
                    ../screens/mobile/README.md, ../wireframes.md (§6.1/§6.2),
                    ../design-system.md (§6/§7), ../theming.md, ../motion/motion.md (§5/§6),
                    ../brand-assets.md (§5.1), ../../CONVENTIONS.md
-->

# Helix Thready — Aurora OS (Qt / Silica) Customization Spec

| Rev | Date | Author | Change |
|-----|------|--------|--------|
| 1 | 2026-07-22 | swarm (design · platforms) | Initial consolidation of the scattered per-screen Aurora one-liners into one design contract: Silica page-stack navigation, remorse-timer pattern, ambience-vs-token tension (honest), Qt Accessibility mapping, component substitutions, density buckets (THREADY-DES-05 inherited), motion honesty, keyboard/hardware back; minted `THREADY-DES-PLAT-05/-06/-07` |

> **Honest status header.** The Aurora client is a `helix_shims` **skeleton** `[GAP: 8.5]`, and
> its would-be token package (`helix_design` Qt arm) is a **verified empty scaffold**
> `[GAP: 8.2/8.3]` ([platform-map §2](../library/platform-map.md#2-per-repo-verification-results)).
> The QT column of the platform matrix has **zero VERIFIED cells** (platform-map §5). Nothing in
> this file is build-ready or claimed to work; it is the **design contract the client builds
> against**, and every Silica/Qt API name is **ASSUMED** until verified against current Aurora OS
> SDK docs and the `helix_shims` contract — `[OPEN: THREADY-DES-PLAT-05]` (companion to the
> inherited `[OPEN: THREADY-DES-LIB-03]`).

## Table of contents

- [1. Position & sources consolidated](#1-position--sources-consolidated)
- [2. Navigation idiom (5-tab IA → Silica page stack)](#2-navigation-idiom-5-tab-ia--silica-page-stack)
- [3. Remorse-timer pattern for destructive actions](#3-remorse-timer-pattern-for-destructive-actions)
- [4. Ambience-aware theming vs. the Thready tokens](#4-ambience-aware-theming-vs-the-thready-tokens)
- [5. Qt Accessibility mapping](#5-qt-accessibility-mapping)
- [6. Component substitutions (Qt/Silica)](#6-component-substitutions-qtsilica)
- [7. Density buckets](#7-density-buckets)
- [8. Motion](#8-motion)
- [9. Keyboard & hardware back](#9-keyboard--hardware-back)
- [10. Open items](#10-open-items)

## 1. Position & sources consolidated

Aurora guidance previously lived as one-liners in: the 11 mobile artifacts' notes panels
(SilicaListView/pulley/remorse/ambience mentions), wireframes §6.1 (Qt Accessibility / ambience /
Qt font scaling row), platform-map §3 QT column + §4, brand-assets §5.1 + icon-export-matrix §3.8
(density-bucket PNGs), and design-system §7. This file consolidates and extends them; the
sources stay authoritative for their own scope. Typography (Qt font database bundling — ASSUMED;
system-default fallback; scale mapping) is in
[typography-substitution.md](./typography-substitution.md).

## 2. Navigation idiom (5-tab IA → Silica page stack)

Silica's native idiom is a **page stack** (push/pop, edge-swipe back) plus **pulley menus** —
not a bottom tab bar. The wireframes §6 IA `[VERIFIED structure]` is five bottom tabs. The
resolution, already sketched by the home-feed artifact note ("page stack + bottom tab strip")
and kept honest here:

- **Tab layer** `[DEFAULT — adjustable]`: a custom bottom tab strip renders the five-tab IA for
  cross-platform parity (same IA everywhere — wireframes §6). This deliberately *deviates from
  pure Silica idiom*; the alternative (replatforming tabs onto pulley/attached pages) would
  fragment the parity contract. The trade-off is recorded, not hidden — revisit inside
  `[OPEN: THREADY-DES-PLAT-05]` with Aurora HIG evidence.
- **Stack layer** (ASSUMED APIs): within a tab, Silica `PageStack` push/pop; edge-swipe back
  pops (the shared back pattern — home-feed note `[VERIFIED intent]`).
- **Pulley menus** host per-screen secondary actions, consolidated from the artifact notes:
  Channels — Add-channel + filters in the pulley; threads — Re-sync/Pause/Recipes; Search —
  scope switches. Primary actions stay on-surface (parity rule: a pulley may *duplicate*, never
  *hide*, a load-bearing action `[DEFAULT — adjustable]`).
- **Sheets:** Add-Channel is a Silica `Dialog` page pushed from the pulley (add-channel note),
  with the §3.4 wizard data-preservation rule intact (dismiss never loses input).

Per-screen consolidation (each cell ASSUMED, `[GAP: 8.5]`):

| Screen | Qt/Silica realization (from the notes panels) |
|---|---|
| Home feed | Qt Quick/Silica page stack + bottom tab strip; ambience-tint policy per §4 |
| Channels | `SilicaListView`; Add-channel + filters via pulley; edge-swipe back |
| Add-Channel | Silica `Dialog` page pushed from the pulley menu |
| Channel threads | `SilicaListView` + pulley (Re-sync/Pause/Recipes); edge-swipe back |
| Post detail | pipeline rows with Qt Accessibility; **remorse timer** on Reprocess (§3) |
| Search | Silica `SearchField` header item over the list |
| Assets grid | `SilicaGridView`; filters via pulley; Qt font scaling per wireframes §6.1 |
| Media viewer | Qt Multimedia (`MediaPlayer`/`VideoOutput`) under Silica chrome; edge-swipe back |
| Account / Settings | identical card stack; ambience-aware theme row (§4); role-gated Admin card |
| Notifications | Aurora events-view integration (`[OPEN: THREADY-DES-15]` surface) |

## 3. Remorse-timer pattern for destructive actions

**Verified precedent:** the post-detail artifact's Aurora note specifies a "remorse-timer
pattern for Reprocess confirm" `[VERIFIED — screens/mobile/post-detail.html notes panel]`. This
file promotes it from a one-liner to the platform rule `[DEFAULT — adjustable]`:

- On Aurora, destructive or hard-to-undo actions use the Sailfish/Aurora **remorse** pattern —
  an inline countdown ("Reprocessing in 5s — tap to cancel") that executes on expiry — *instead
  of* the modal confirm dialog other platforms use. Same decision semantics, platform-native
  delivery; an allowed deviation under the parity contract because states and outcomes are
  identical.
- Scope: Reprocess (verified precedent), Remove channel, Pause channel bulk actions, Sign out.
  **Not** used where the web contract mandates stronger friction (typed confirmation), if any
  such flow lands — remorse never *weakens* a confirm contract.
- Semantics: the countdown row is announced via Qt Accessibility (state + remaining action);
  cancel is a full-row tap target (≥ 44px); the destructive label uses `--danger` — remorse
  styling never masks the semantic color (design-system §6.2 rule).
- Timer length ~5 s `[DEFAULT — adjustable]`; the `RemorsePopup`/remorse-item API availability
  in current Aurora Silica is **ASSUMED** — `[OPEN: THREADY-DES-PLAT-05]`. Fallback if absent:
  the standard confirm dialog (platform-map §3 `Dialog` row).
- Idempotency guard: expiry fires the same idempotent action the other platforms use (e.g.
  Reprocess 409 "already running" handling — mobile README §3), so a double-fire is harmless.

## 4. Ambience-aware theming vs. the Thready tokens

**The tension, documented honestly.** Silica **ambiences** derive an accent/tint from the
user-chosen ambience (wallpaper), i.e. *the OS wants to color your app*. Thready mandates the
opposite: every surface is colored by the **token palette** (design-system §3) plus per-Account
white-label (theming §8), with semantic colors untouchable. Both cannot fully win. Ground truth
already contains the arbitration seed: wireframes §6.1 pins the Aurora theme source as
"bridge + Silica ambience", and the settings artifact note says "ambience-aware; **explicit
choice still wins** (parity contract)" `[VERIFIED intents]`.

Resolution `[DEFAULT — adjustable]`, final policy `[OPEN: THREADY-DES-PLAT-07]`:

1. **Tokens always win for load-bearing UI:** text, controls, focus, chips, progress, and every
   **semantic** color (`--danger`/`--warn`/`--success`) come from the token bridge. A
   white-label Account brand must survive any ambience; an error must survive both
   (theming §10.2 rule).
2. **Ambience may tint decorative background layers only** — the page background wash behind
   token-colored surfaces — so the app still feels ambience-native without breaking AA or brand.
   If contrast of any token pairing over an ambience-tinted layer cannot be guaranteed, the
   layer falls back to token `--bg` (AA is non-negotiable, design-system §6.1).
3. **Explicit theme choice wins over ambience-implied lightness:** the Light/System/Dark
   three-state behaves as on every platform; "System" maps to the ambience's light/dark
   character (ASSUMED API for reading it).

## 5. Qt Accessibility mapping

The design-system §6 contract on Qt Accessibility (wireframes §6.1 row `[VERIFIED table]`; all
attribute names ASSUMED):

| Contract point | Qt/Aurora realization |
|---|---|
| WCAG 2.2 AA contrast | Carried by tokens; §4.2 guards ambience layers |
| Brand ≠ text; danger ≠ brand | Token discipline; remorse rows keep `--danger` (§3) |
| Focus visible everywhere | `Accessible.focused`/focus highlight on every interactive; visible focus for hardware-keyboard use (§9) |
| Keyboard + screen reader | `Accessible.role`/`.name`/`.description` on every interactive; pipeline rows expose per-step status ("Qt Accessibility on pipeline rows" — post-detail note `[VERIFIED intent]`); state changes announced |
| Reduced motion | System animation-reduction (ASSUMED availability) gates §8; poster-frame freeze |
| Localized, direction-agnostic | ru / sr-Cyrl / en; string keys shared with the fleet (`I18nService` dictionary, incl. `footer.made`/`footer.by`/`a11y.love` `[VERIFIED keys]`) |

Screen-reader maturity on Aurora is **not asserted** — actual capability on target devices is
part of `[OPEN: THREADY-DES-PLAT-05]` verification; the design contract stands regardless.

## 6. Component substitutions (Qt/Silica)

Derived from platform-map §3 (QT column) + §4, upgraded with the Silica-specific counterparts the
mobile artifact notes verified as design intent. **Every row ASSUMED** (`[GAP: 8.5]`):

| Component group | Qt/Silica substitution | Source |
|---|---|---|
| Buttons | `Button` + QML token styles; danger palette for destructive; loading = `enabled:false` + `BusyIndicator` | platform-map §3 |
| Input / field | `TextField` + label/error `Text` triple (border tint, message, accessible state) | platform-map §3/§4 |
| Select / date | `ComboBox` / `Calendar` popup — native pickers win on mobile (platform-map §4 rule) | platform-map §3 |
| Checkbox / radio / switch | `CheckBox` / `RadioButton` / `Switch`, accent track via token bridge | platform-map §3/§4 |
| Card / stat | `Frame`/QML rect composites | platform-map §3 |
| List / table | **`SilicaListView`** (upgraded from generic `TableView` for phone surfaces — table collapses to thread-row cards per platform-map §4); `TableView` only on genuinely tabular admin surfaces | notes panels + §4 |
| Grid | **`SilicaGridView`** (assets masonry approximation) | assets-grid note |
| Search | **Silica `SearchField`** header item | search note |
| Badge / chips | pill `Rectangle`; hashtag pill + outline (indirect = dimmed + non-color marker); processing chip = pill + `BusyIndicator`, five states + focusable retry | platform-map §3 + component-library §7 |
| Dialog | `Dialog` (Silica page-style) — but destructive confirms prefer the remorse pattern (§3) | platform-map §3 + §3 above |
| Toast | overlay `Popup`; never the only channel for a critical error | platform-map §3/§4 |
| Progress / spinner | `ProgressBar` / `BusyIndicator`; helix-motif via QML `Canvas`/`Shape`, frozen under reduced motion | platform-map §3/§4 |
| Tooltip | `ToolTip` on long-press; load-bearing hints also visible helper text | platform-map §4 |
| Skeleton | animated rects, ≥150 ms delay before showing | platform-map §3/§4 |
| Empty / error | centered composites + retry | platform-map §3 |
| Media | Qt Multimedia `MediaPlayer`/`VideoOutput` under Silica chrome | media-viewer note |
| Theme toggle / language | palette swap three-state (§4) / `ComboBox` locale | platform-map §3 |

## 7. Density buckets

Inherited, unresolved: Aurora icon PNGs are produced at **86 / 108 / 128 / 172 / 250 px**
(`[VERIFIED — generate-raster.sh run; icon-export-matrix §3.8]`), but the bucket set itself is
`[RESEARCH]` and must be re-verified against current Aurora packaging docs —
`[OPEN: THREADY-DES-05]` **inherited, not re-litigated here**. Layout consequence
`[DEFAULT — adjustable]`: express paddings/targets via Silica `Theme` units (ASSUMED:
`Theme.paddingSmall/Medium/Large`, `Theme.itemSizeSmall/…`) mapped from the nearest token spacing
steps, rather than raw px, so density scaling is the OS's job; the mapping table is produced with
the token bridge (`[OPEN: THREADY-DES-LIB-04]`).

## 8. Motion

Checked against ground truth: [motion.md §6](../motion/motion.md#6-runtime-integration) names
`lottie-web` / `lottie-compose` / `lottie-ios` — **no Qt or Aurora Lottie runtime is named
anywhere** in motion.md or platform-map. Honest consequence:

- **Lottie on Aurora is unverified and not designed-in.** Candidate routes (rlottie-based QML
  players, Qt add-ons) are **ASSUMED to exist at best** — evaluation is
  `[OPEN: THREADY-DES-PLAT-06]`; `THREADY-MOT-01`'s verify-before-ship rule applies.
- **Guaranteed floor:** Qt Quick animations (`NumberAnimation`, `Behavior`, states/transitions)
  with the token durations `--motion-fast 150ms` / `--motion-base 200ms` on `--ease-standard
  cubic-bezier(0.2,0,0,1)` `[VERIFIED values — design-system §5]`, plus the five delivered
  static poster SVGs (motion §5 `[VERIFIED]`) for loading/success/error/pulse/sync states. The
  helix spinner may alternatively be a QML `Canvas` rotation of the static mark.
- Reduced motion freezes to the exact §5 poster frames (equivalent-runtime rule); remorse
  countdowns (§3) remain **textual** under reduced motion (the timer is meaning, not
  decoration — it keeps counting, only decorative easing is dropped).

## 9. Keyboard & hardware back

- **Edge-swipe back is the primary back gesture** (= the shared back pattern, home-feed note
  `[VERIFIED intent]`); the back-stack rules of [harmonyos.md §3](./harmonyos.md#3-system-back-behavior)
  apply identically (sheet-dismiss preserves input; back never confirms a destructive action).
  Aurora devices with a hardware back key (ASSUMED to exist on some targets) map it to the same
  pop.
- **Hardware keyboard** (attached/BT) `[DEFAULT — adjustable]`: adopt the verified web keyboard
  model (wireframes §1.2) additively — `/` or `Ctrl+K` focus search, `Esc` = pop/dismiss
  (equivalent of edge-swipe), `Tab`/`Enter`/`Space` focus + activate. Same rule as desktop
  (desktop README §3.1): **never rebind a verified web binding**; platform accelerators are
  additive. No global system-wide shortcut is claimed.

## 10. Open items

- `[OPEN: THREADY-DES-PLAT-05]` — **verify every Silica/Qt mapping in this file** (PageStack,
  pulley API, `RemorsePopup`/remorse item, `SilicaListView`/`SilicaGridView`/`SearchField`,
  Qt Multimedia on Aurora, ambience-read API, `Theme` units, screen-reader maturity, hardware
  back-key existence) against the current Aurora OS SDK/HIG and the `helix_shims` contract.
  Companion to the inherited `[OPEN: THREADY-DES-LIB-03]`.
- `[OPEN: THREADY-DES-PLAT-06]` — **Aurora motion runtime**: evaluate/verify any Lottie-capable
  QML player vs. committing to the §8 Qt-native floor; until then only the floor may be claimed.
- `[OPEN: THREADY-DES-PLAT-07]` — **ambience-vs-token policy**: ratify (or amend) the §4
  arbitration — tokens win for load-bearing UI, ambience tints decorative layers only, explicit
  choice wins — with AA evidence over real ambiences.
- Inherited: `[OPEN: THREADY-DES-05]` (density buckets `[RESEARCH]`), `[GAP: 8.2/8.3/8.5]`
  (empty `helix_design` Qt arm; client skeleton — release gate, wireframes §6.2),
  `[OPEN: THREADY-DES-LIB-03/-04]`, `[OPEN: THREADY-DES-15]` (notifications surface),
  `[OPEN: THREADY-DES-PLAT-01/-02/-08]` (typography, via
  [typography-substitution.md](./typography-substitution.md)).

---

*Made with love ♥ by Helix Development.*
