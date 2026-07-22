<!--
  Title           : Helix Thready — Platform Customization Specs (Catalogue)
  Classification  : PUBLIC
  Location        : docs/public/research/mvp/design/platforms/README.md
  Status          : Draft — v0.1
  Revision        : 1 (2026-07-22)
  Author          : Helix Thready documentation swarm (design · platforms)
  Related         : ./typography-substitution.md, ./harmonyos.md, ./aurora.md,
                    ./react-token-rebind.md, ../library/platform-map.md,
                    ../screens/mobile/README.md, ../screens/desktop/README.md,
                    ../screens/tui/lipgloss-theme.md, ../design-system.md,
                    ../../CONVENTIONS.md
-->

# Helix Thready — Platform Customization Specs

| Rev | Date | Author | Change |
|-----|------|--------|--------|
| 1 | 2026-07-22 | swarm (design · platforms) | Initial catalogue: per-platform DEVIATION specs closing the adversarial-review finding ("adequate-but-scattered for Android/iOS, THIN for HarmonyOS/Aurora, MISSING typography substitution everywhere") — typography-substitution, dedicated HarmonyOS + Aurora specs, React token-rebind remediation contract; minted the `THREADY-DES-PLAT-*` open-item family |

## 1. What this directory is

Per-platform **customization/deviation specs**: the places where a target platform must
legitimately diverge from the canonical web realization of the design system, specified once and
cross-linked, instead of living as scattered one-liners.

The division of labor with the neighbouring areas is deliberate:

- **[library/platform-map.md](../library/platform-map.md)** *maps components* — every library
  component × 8 platform realizations, with per-cell VERIFIED / PROPOSED / ASSUMED markers (§3)
  and per-component customization notes (§4). It answers "*what* realizes this component on
  platform X".
- **This directory** specifies **per-platform deviations** — the platform-wide contracts that cut
  *across* components: typography substitution, navigation idiom, system-back behavior, safe
  areas, platform theming interactions (ambience, system dark mode), accessibility API mapping,
  motion runtime availability, i18n. It answers "*how* platform X as a whole departs from the web
  baseline, and what must never depart".
- **[screens/*/README.md](../screens/mobile/README.md)** carry **per-screen chrome** — the
  Android/iOS chrome toggle (status bar, app bar/back pattern, navigation bar, gesture area:
  mobile README §1), the Tauri per-OS title bar/menus/tray (desktop README §3), the TUI terminal
  theme ([lipgloss-theme.md](../screens/tui/lipgloss-theme.md)). Those stay authoritative for
  screen-level rendering; this directory never duplicates a screen.

**Android/iOS status.** The adversarial review rated Android/iOS coverage *adequate-but-scattered*
(mobile README §1 chrome toggle, wireframes §6.1 a11y/theming table, platform-map §4 notes) — so
no dedicated Android/iOS file is authored here; the one Android/iOS hole (typography) is closed by
[typography-substitution.md](./typography-substitution.md), which covers **all** platforms.
HarmonyOS and Aurora were rated *THIN* and get dedicated consolidating specs.

## 2. Catalogue

| File | Content | Status |
|------|---------|--------|
| [typography-substitution.md](./typography-substitution.md) | **The missing spec**: per-platform font strategy for the three brand faces (Space Grotesk / Hanken Grotesk / JetBrains Mono) — bundling feasibility per platform, honest fallback stacks (`[DEFAULT — adjustable]`), Cyrillic coverage discipline (THREADY-DES-04 inherited), dynamic-type / font-scale mapping of the token type ramp | new spec; bundling decision `[OPEN: THREADY-DES-PLAT-01]` |
| [harmonyos.md](./harmonyos.md) | Dedicated **ArkTS / HarmonyOS** customization spec: ArkUI navigation of the 5-tab IA, system back, safe areas, system dark mode, Barrier-Free a11y mapping, consolidated ArkUI component-substitutions table, layered icon integration, motion runtime honesty, i18n | design contract; client is a skeleton `[GAP: 8.5]` |
| [aurora.md](./aurora.md) | Dedicated **Aurora OS / Qt / Silica** customization spec: Silica page-stack navigation, remorse-timer pattern for destructive actions, ambience-vs-token tension (documented honestly), Qt Accessibility mapping, component substitutions, density buckets (`THREADY-DES-05` inherited), motion honesty, keyboard/hardware back | design contract; client is a skeleton `[GAP: 8.5]` |
| [react-token-rebind.md](./react-token-rebind.md) | **Remediation contract** (not a full React spec) for `THREADY-DES-LIB-01` / `[GAP: 8.6]`: the verified finding (`Button.tsx` hard-codes a Tailwind palette, zero token references), the required re-bind approach (CSS custom properties from `tokens.css`), and the acceptance criteria that close the gap | remediation contract |

## 3. Reading order & rules

1. Read [design-system.md §7](../design-system.md#7-per-platform-fan-out) first — the fan-out
   table and the token-bridge mechanism are the frame every file here hangs on.
2. Then [platform-map.md](../library/platform-map.md) §2 (what is actually verified per repo) —
   the honesty baseline these specs must never overstate.
3. Then the file for your platform. Cross-cutting typography applies to **every** platform.

Non-negotiables that no per-platform deviation may touch (restated from
[design-system §6](../design-system.md#6-accessibility-contract) and
[opendesign/DESIGN.md](../opendesign/DESIGN.md)):

- **Semantic colors are state, never decoration** — destructive UI is always `--danger`; no
  platform theme, ambience, or white-label may mask it `[VERIFIED — THEMES.md rule]`.
- **Parity contract** — identical interaction states + a11y semantics on every platform, enforced
  by the `ScreenDiff`/`VisualRegression` bank once CI lands `[GAP: 9.3]` (wireframes §6.1).
- **Tokens are the single upstream** — every platform consumes a *generated* binding, never a
  hand-kept copy (design-system §7).

## 4. Open items (family index)

The `THREADY-DES-PLAT-*` family minted by this directory; each is owned by the file that mints it
and registered in the canonical [../index.md](../index.md#open-items) registry:

| ID | Summary | Owning file |
|----|---------|-------------|
| `THREADY-DES-PLAT-01` | Per-platform font **bundling decision** + license/redistribution verification for the three brand faces | [typography-substitution.md §7](./typography-substitution.md#7-open-items) |
| `THREADY-DES-PLAT-02` | **Fallback-stack Cyrillic** (ru / sr-Cyrl) coverage verification per OS face | [typography-substitution.md §7](./typography-substitution.md#7-open-items) |
| `THREADY-DES-PLAT-03` | **ArkUI API verification** of every derived HarmonyOS mapping (navigation, components, font registration, Barrier-Free) | [harmonyos.md §11](./harmonyos.md#11-open-items) |
| `THREADY-DES-PLAT-04` | **HarmonyOS motion runtime** — no verified Lottie player; `@ohos/lottie` is ASSUMED | [harmonyos.md §11](./harmonyos.md#11-open-items) |
| `THREADY-DES-PLAT-05` | **Silica/Qt API verification** of every derived Aurora mapping (remorse timer, Silica components, Qt Multimedia) | [aurora.md §10](./aurora.md#10-open-items) |
| `THREADY-DES-PLAT-06` | **Aurora motion runtime** — no verified Lottie player; Qt-native animation fallback decision | [aurora.md §10](./aurora.md#10-open-items) |
| `THREADY-DES-PLAT-07` | **Ambience-vs-token policy** — final rule for what (if anything) Silica ambience may tint | [aurora.md §10](./aurora.md#10-open-items) |
| `THREADY-DES-PLAT-08` | **Dynamic-type clamp matrix** — max font-scale each platform must survive against the token ramp | [typography-substitution.md §7](./typography-substitution.md#7-open-items) |

---

*Made with love ♥ by Helix Development.*
