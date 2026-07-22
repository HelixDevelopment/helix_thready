<!--
  Title           : Helix Thready — HarmonyOS (ArkTS) Customization Spec
  Classification  : PUBLIC
  Location        : docs/public/research/mvp/design/platforms/harmonyos.md
  Status          : Draft — v0.1
  Revision        : 1 (2026-07-22)
  Author          : Helix Thready documentation swarm (design · platforms)
  Related         : ./README.md, ./typography-substitution.md, ../library/platform-map.md,
                    ../screens/mobile/README.md, ../wireframes.md (§6.1/§6.2),
                    ../design-system.md (§6/§7), ../motion/motion.md (§5/§6),
                    ../assets/icon-export-matrix.md (§3.7), ../../CONVENTIONS.md
-->

# Helix Thready — HarmonyOS (ArkTS) Customization Spec

| Rev | Date | Author | Change |
|-----|------|--------|--------|
| 1 | 2026-07-22 | swarm (design · platforms) | Initial consolidation of the scattered per-screen ArkTS one-liners into one design contract: navigation idiom, system back, safe areas, system dark mode, Barrier-Free mapping, ArkUI substitutions table (derived), layered icon, motion honesty, i18n; minted `THREADY-DES-PLAT-03/-04` |

> **Honest status header.** The HarmonyOS client is a `helix_shims` **skeleton** `[GAP: 8.5]`
> — verified as such in [platform-map §2](../library/platform-map.md#2-per-repo-verification-results)
> ("Only reachable via native clients + `helix_shims` per ground truth; `helix_shims` interface
> not inspected — **ASSUMED** entire column"). **Nothing in this file is build-ready or claimed
> to work.** This spec is the **design contract the client builds against** when `[GAP: 8.5]`
> closes; every ArkUI API name below is **ASSUMED** until verified against real HarmonyOS SDK
> docs and the `helix_shims` contract — tracked as `[OPEN: THREADY-DES-PLAT-03]` (companion to
> the inherited `[OPEN: THREADY-DES-LIB-03]`).

## Table of contents

- [1. Position & sources consolidated](#1-position--sources-consolidated)
- [2. Navigation idiom (5-tab IA → ArkUI)](#2-navigation-idiom-5-tab-ia--arkui)
- [3. System back behavior](#3-system-back-behavior)
- [4. Safe areas & notch](#4-safe-areas--notch)
- [5. Dark mode via system](#5-dark-mode-via-system)
- [6. Barrier-Free accessibility mapping](#6-barrier-free-accessibility-mapping)
- [7. ArkUI component substitutions](#7-arkui-component-substitutions)
- [8. Layered icon integration](#8-layered-icon-integration)
- [9. Motion](#9-motion)
- [10. i18n](#10-i18n)
- [11. Open items](#11-open-items)

## 1. Position & sources consolidated

Before this file, HarmonyOS guidance lived as one-liners across: the per-screen notes panels of
all 11 mobile artifacts (mobile README §2 catalogue), the wireframes §6.1 platform table (a11y /
theme / font-scale row), platform-map §3 AR column + §4 notes (toast/tooltip rules), brand-assets
§5.1 / icon-export-matrix §3.7 (layered icon), and design-system §7 (fan-out row). This file
consolidates them and **extends** where the review found holes; the sources stay authoritative
for their own scope (screens for per-screen chrome, platform-map for per-component cells).

Typography for this platform is specified in
[typography-substitution.md](./typography-substitution.md) (bundling via the ArkTS
font-registration API — ASSUMED; fallback HarmonyOS Sans; `fp` scale mapping) and is not
repeated here.

## 2. Navigation idiom (5-tab IA → ArkUI)

The wireframes §6 IA `[VERIFIED structure]` — five bottom tabs (Home / Channels / Search /
Assets / More) with per-tab stacks — realizes in ArkUI as (all mappings ASSUMED):

- **Root:** ArkUI `Tabs` with a bottom `TabBar` mirroring the five-tab IA (the home-feed
  artifact's note already fixes this: "bottom `Tabs` bar mirrors this five-tab IA").
- **Per-tab stacks:** ArkUI `Navigation` + `NavPathStack` per tab, so each tab keeps its own
  back stack (Channels → threads → post → media), matching the Android/iOS behavior in the
  chrome-toggle spec (mobile README §1).
- **Sheets:** the Add-Channel wizard is a `bindSheet` panel (add-channel artifact note
  `[VERIFIED as the design intent, ASSUMED as API]`) — same step gating and data-preservation
  rules as the §3.4 wizard model.

Per-screen realization, consolidated from the artifact notes panels (each cell ASSUMED,
`[GAP: 8.5]`):

| Screen (artifact) | ArkUI realization (consolidated from the notes panels) |
|---|---|
| Home feed | `Tabs`/`List`/`Column` declarative components; live region for processing events |
| Channels | `List` with swipe-action items; Add-channel via bottom sheet; system back to Home |
| Add-Channel | `bindSheet` panel; back dismisses, preserving entered data |
| Channel threads | `List` + swipe-action items; system back returns to Channels list |
| Post detail | pipeline pane as ArkUI `Progress` rows; Barrier-Free announces per-step status |
| Search | ArkUI `Search` component + chips row |
| Assets grid | `WaterFlow` realizes the masonry; Barrier-Free labels mirror tile semantics |
| Media viewer | `Video` / `Image` components + `AVSession` for background audio; system back closes |
| Account / Settings | same card stack as Android/iOS; three-state theme choice (§5) |
| Notifications | system push via the platform notification kit (`[OPEN: THREADY-DES-15]` surface) |

## 3. System back behavior

`[DEFAULT — adjustable]`, extending the verified one-liners ("system back gesture/key maps to
the same back stack" — home-feed note):

1. Back pops the **current tab's** stack first (media viewer → post → threads → channels).
2. An open sheet/dialog consumes back as *dismiss* — and the Add-Channel sheet **preserves
   entered data** on dismissal (parity with the §3.4 wizard model: "Back / Esc never loses
   input" `[VERIFIED rule — add-channel artifact]`).
3. At a non-Home tab root, back returns to the Home tab; at Home root, back follows the system
   default (background the app). This two-step rule mirrors Android predictive-back convention
   and is a **proposal** — confirm against HarmonyOS HIG at integration
   (`[OPEN: THREADY-DES-PLAT-03]`).
4. Destructive flows never bind back as *confirm* — back always cancels.

## 4. Safe areas & notch

`[DEFAULT — adjustable]`, ASSUMED APIs: content respects system avoid areas (status bar,
punch-hole/notch, navigation/gesture area) via ArkUI safe-area handling (`expandSafeArea` only
for full-bleed surfaces — the media viewer background — with controls inset to the safe area).
The bottom tab bar sits above the gesture indicator area. The mobile artifacts' bezel/status-bar
cosmetics are illustrative chrome, not token-governed (mobile README §3 `[VERIFIED convention]`);
the HarmonyOS status-bar treatment follows the same rule.

## 5. Dark mode via system

Verified design intent (settings artifact note): "dark mode follows system + explicit override,
same three-state choice". Realization (ASSUMED APIs):

- The three-state Light/System/Dark control writes the app color-mode preference; **System**
  follows the OS dark mode, matching web mechanism semantics (theming §2) without reusing its
  CSS mechanics.
- Colors come **only** from the generated token binding (design-system §7 token bridge — the
  ArkTS output of the same codegen that emits `ThreadyColors`/Lipgloss; generator itself
  `[OPEN: THREADY-DES-LIB-04]`, does not exist yet). Dark values are the verified Thready dark
  theme tokens; no ArkUI default palette may leak into token-governed surfaces.
- Per-Account white-label re-tints via the same bridge from `GET /v1/accounts/{id}/branding`
  (theming §8 mechanism `[VERIFIED as stated mechanism]`); semantic colors are never overridden
  (`--danger` rule, design-system §6.2).

## 6. Barrier-Free accessibility mapping

The a11y contract (design-system §6 `[VERIFIED]`) realized on the Barrier-Free kit (wireframes
§6.1 row `[VERIFIED table]`; every ArkUI attribute name ASSUMED):

| Contract point (design-system §6) | HarmonyOS realization |
|---|---|
| WCAG 2.2 AA contrast | Carried by the tokens themselves (AA-pinned values) — no platform work beyond *using* the bridge |
| Brand ≠ text; danger ≠ brand | Same token discipline; `Progress`/chip states use semantic tokens only |
| Focus visible everywhere | Focus-highlight on every interactive component (ArkUI focus states); keyboard/remote focus order follows reading order |
| Keyboard + screen reader | Barrier-Free screen reading: `accessibilityText`/`accessibilityDescription` on every interactive; groups via `accessibilityGroup`; per-step pipeline status **announced** on change (post-detail note `[VERIFIED intent]`) |
| Reduced motion | Honor the system animation-reduction setting; freeze to poster frames per motion §5 (see §9) |
| Localized, direction-agnostic | §10; string resources per locale |

Long-press tooltips replace hover hints (platform-map §4 `[VERIFIED rule]`: "Compose/Flutter/
ArkTS show tooltips on long-press — hints that matter must also exist as visible helper text").

## 7. ArkUI component substitutions

Consolidated from platform-map §3 (AR column) + §4 notes; the three explicitly noted mappings
first, then the derived component-group mappings. **Every row is ASSUMED** (`[GAP: 8.5]`,
`[OPEN: THREADY-DES-PLAT-03]`) — the AR column has zero VERIFIED cells (platform-map §5).

| Component group | ArkUI substitution | Source |
|---|---|---|
| Toast / alert | `promptAction.showToast`; never the only channel for a critical error (also inline) | platform-map §4 (explicit) |
| Progress bar | `Progress` (det./indet.); failed+retry keeps `--danger` + focusable retry | platform-map §3/§4 (explicit) |
| Tooltip | long-press `Popup` hint + visible helper text for load-bearing hints | platform-map §4 (explicit) |
| Buttons (primary/secondary/ghost/destructive) | `Button` with token-bridge styles; destructive always `--danger` | derived from §3 AR column |
| Input / textarea | `TextInput`/`TextArea` + form hint slot for label/hint/error triple (`isError`-equivalent semantics travel with the message text) | derived |
| Select | `Select` | derived |
| Checkbox / radio / switch | `Checkbox` / `Radio` / `Toggle(ToggleType.Switch)` — accent track from the token bridge (platform-map §4) | derived |
| Date | `DatePickerDialog` (native picker wins on mobile — platform-map §4 rule) | derived |
| Card / stat card | `Column` + border/tokens; composites assembled, not forked | derived |
| Table → cards | On phones the table collapses to thread-row cards (platform-map §4 rule); `List` + header where a table survives | derived |
| Badge / chips | `Badge`; hashtag `Chip` (direct = brand fill, indirect = outline + `--muted` + non-color marker); processing chip = chip + progress, five states + `[r]`-equivalent retry (component-library §7 parity contract) | derived |
| Dialog / modal | `CustomDialog`; destructive action never default-focused | derived |
| Spinner / helix motif | `LoadingProgress`; helix-motif spinner via `Canvas` drawing | derived |
| Tabs / sidebar / topbar | `Tabs` / `SideBarContainer` / `Navigation` title bar | derived |
| Skeleton | opacity-pulse rows, only after ~150 ms (no flash — platform-map §4 rule) | derived |
| Empty / error state | centered composites; error page + idempotent retry | derived |
| Avatar / pagination / breadcrumbs / link | `Image`+circle text / button row / text row / `Span` + click | derived |
| Theme toggle / language picker | dark-mode config three-state (§5) / locale select (§10) | derived |

## 8. Layered icon integration

Grounded in [icon-export-matrix §3.7](../assets/icon-export-matrix.md#37-harmonyos)
`[VERIFIED — assets produced by the live `generate-raster.sh` run]`:

- `harmonyos/foreground.png` (216) + `harmonyos/background.png` (216) referenced from
  `resources/base/media/layered_image.json` (`foreground`/`background`); the solid/branded
  background layer is supplied at integration (brand-assets §5.1).
- `harmonyos/appgallery-1024.png` for the AppGallery listing.
- The **assets exist now; no HarmonyOS launcher is claimed** — wiring them into an actual `.hap`
  is `[GAP: 8.5]` work. The launcher icon carries the helix element, no letters
  `[VERIFIED — brand-assets]`.

## 9. Motion

What ground truth actually says — checked, not assumed: [motion.md §6](../motion/motion.md#6-runtime-integration)
names **exactly three** Lottie runtimes (`lottie-web`, `lottie-compose`, `lottie-ios`); neither
motion.md nor platform-map names **any** HarmonyOS Lottie runtime. Therefore:

- A community `@ohos/lottie` player is believed to exist in the OHOS ecosystem, but it is
  **unverified here — ASSUMED**, and adopting it (vs. reimplementing the small animation set in
  ArkUI `animateTo`) is `[OPEN: THREADY-DES-PLAT-04]`. `THREADY-MOT-01`'s verify-before-ship rule
  extends to any HarmonyOS runtime.
- **Guaranteed floor (no Lottie needed):** the five static `reducedMotionFallback` SVGs/posters
  (motion §5 `[VERIFIED delivered]`) render the loading/success/error/pulse/sync states; simple
  state transitions use ArkUI animation with the token durations `--motion-fast 150ms` /
  `--motion-base 200ms` on `--ease-standard cubic-bezier(0.2,0,0,1)` `[VERIFIED values]`.
- Reduced motion: the system animation-reduction setting gates all of it, freezing to the exact
  poster frames of motion §5 (equivalent-runtime rule: "both paths render the same state by
  construction" `[VERIFIED — motion §5]`).

## 10. i18n

- Locales: `en` / `ru` / `sr-Cyrl` `[OPERATOR §12]`, following the **system locale** by default
  with the same in-app language picker semantics as every surface (component-library §4
  `I18nService` key parity — including `footer.made` / `footer.by` / `a11y.love` for the locked
  attribution footer `[VERIFIED keys]`).
- String delivery via HarmonyOS resource qualifiers per locale (ASSUMED mechanism); the string
  *keys* stay the shared dictionary so translations are written once.
- Cyrillic rendering per [typography-substitution §5](./typography-substitution.md#5-cyrillic-coverage-ru--sr-cyrl)
  — HarmonyOS Sans coverage is an unverified expectation `[OPEN: THREADY-DES-PLAT-02]`.
- The token layer is direction-agnostic (design-system §6.6); all three locales are LTR.

## 11. Open items

- `[OPEN: THREADY-DES-PLAT-03]` — **verify every ArkUI mapping in this file** (Navigation/Tabs/
  NavPathStack, `bindSheet`, `WaterFlow`, `AVSession`, `promptAction`, `font.registerFont`,
  Barrier-Free attributes, safe-area APIs, back-at-tab-root rule) against the current HarmonyOS
  SDK/HIG **and** the `helix_shims` contract once it exists. Companion to the inherited
  `[OPEN: THREADY-DES-LIB-03]`.
- `[OPEN: THREADY-DES-PLAT-04]` — **HarmonyOS motion runtime**: verify `@ohos/lottie` (or
  choose ArkUI reimplementation); until then only the §9 guaranteed floor may be claimed.
- Inherited: `[GAP: 8.5]` (client skeleton — release gate, wireframes §6.2),
  `[OPEN: THREADY-DES-LIB-03]` (`helix_shims` uninspected), `[OPEN: THREADY-DES-LIB-04]`
  (token-bridge codegen does not exist), `[OPEN: THREADY-DES-15]` (notifications surface),
  `[OPEN: THREADY-DES-PLAT-01/-02/-08]` (typography, via
  [typography-substitution.md](./typography-substitution.md)).

---

*Made with love ♥ by Helix Development.*
