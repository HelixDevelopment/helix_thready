<!--
  Title           : Helix Thready — Token-Bridge Codegen (CSS tokens → per-platform bindings)
  Classification  : PUBLIC
  Location        : docs/public/research/mvp/design/tokens-bridge/README.md
  Status          : Draft — v0.1
  Revision        : 1 (2026-07-22)
  Author          : Helix Thready documentation swarm (design · tokens-bridge)
  Related         : ../opendesign/tokens.css, ../design-system.md (§3/§7),
                    ../screens/tui/lipgloss-theme.md, ../library/platform-map.md (§2),
                    ../figma/figma-variables.json, ../../CONVENTIONS.md
-->

# Helix Thready — Token-Bridge Codegen

| Rev | Date | Author | Change |
|-----|------|--------|--------|
| 1 | 2026-07-22 | swarm (design · tokens-bridge) | Initial: `generate.mjs` (no-deps Node parser/emitter for `opendesign/tokens.css`), seven generated per-platform bindings, `--check` drift gate, honest validation record. Materializes the generator that `design-system.md §7`, `lipgloss-theme.md §8` and `platform-map.md §6` reference but until now did not exist (`[OPEN: THREADY-DES-LIB-04]` — see §6) |

## Table of contents

- [1. What this is (and the single-source rule)](#1-what-this-is-and-the-single-source-rule)
- [2. Generated targets & honest consumer status](#2-generated-targets--honest-consumer-status)
- [3. How to run](#3-how-to-run)
- [4. The `--check` drift gate](#4-the---check-drift-gate)
- [5. Validation record (2026-07-22, this machine)](#5-validation-record-2026-07-22-this-machine)
- [6. Closure state of THREADY-DES-LIB-04](#6-closure-state-of-thready-des-lib-04)
- [7. What is deliberately NOT exported](#7-what-is-deliberately-not-exported)
- [8. Open items](#8-open-items)

## 1. What this is (and the single-source rule)

`generate.mjs` is the **token-bridge codegen** mandated by
[design-system.md §7](../design-system.md#7-per-platform-fan-out): *"each non-web platform
consumes a **generated** binding, not a hand-kept copy, so a token change propagates."* It aligns
with the constitution design mandate (`[CONSTITUTION §11.4.162]` one design system for every
surface; `§11.4.6` no invented values — every emitted value is parsed, never restated).

**The single source of truth is [`../opendesign/tokens.css`](../opendesign/tokens.css).**
The generator parses its `:root` block (light + structural tokens), its
`@media (prefers-color-scheme: dark)` block and its `[data-theme="dark"], .dark` block into one
internal model, verifies the two dark blocks agree, and emits every target from that model.
Nothing in `generated/` may be edited by hand — every file carries a
`GENERATED — DO NOT EDIT` header with the sha256 of the `tokens.css` revision that produced it:

```
sha256(tokens.css) = f0f788d1ee73dc624b2006b597d6fb9ab618190b68bf2425c72edc36f0ca4eaf
```

The Lipgloss (Go) target additionally reads the **normative** mapping tables of
[`../screens/tui/lipgloss-theme.md §2`](../screens/tui/lipgloss-theme.md#2-token--color-map)
(truecolor → ANSI-256 → ANSI-16) and the generator **fails** if that document's truecolor column
ever drifts from `tokens.css` — the doc and the CSS are cross-locked.

Determinism: output ordering follows `tokens.css` declaration order; no timestamps are embedded —
regenerating from the same source is byte-identical (that is what makes `--check` a CI gate).

## 2. Generated targets & honest consumer status

Consumer statuses are carried verbatim from
[`platform-map.md §2`](../library/platform-map.md#2-per-repo-verification-results) (verified
2026-07-22 against repo HEADs). **A generated binding is a delivery-ready contract — it is NOT a
claim that the consumer app exists or consumes it today.**

| Target | File | Format | Intended consumer | Honest consumer status (platform-map §2) |
|---|---|---|---|---|
| Web / interchange | [`generated/web/tokens.json`](./generated/web/tokens.json) | **W3C DTCG** design tokens; color modes as token groups `thready-light` / `thready-dark`; `dimension`/`number`/`duration`/`cubicBezier`/`fontFamily` `$type`s; aliases as DTCG `{references}`. Doubles as the **PenPot 2.x design-tokens import file** (PenPot 2.x consumes W3C DTCG) — a concrete bridge for `[OPEN: THREADY-DES-02]` | PenPot import; any Style-Dictionary-class pipeline | Web layer itself is PRODUCTION-usable (`design_system` `.ds-*`) and consumes the CSS directly — this JSON is for tools, not the Angular app. PenPot import not yet exercised (no PenPot in this environment) |
| Compose / KMP | [`generated/compose/ThreadyColors.kt`](./generated/compose/ThreadyColors.kt) | Kotlin objects `ThreadyColors.LightColors`/`.DarkColors` (`Color(0xFF…)`), `ThreadySpacing`/`ThreadyRadius` (`Dp`), `ThreadyTypeScale` (`sp`/`em`), `ThreadyMotion`. Package `digital.vasic.thready.design` — the design-system §7 sample declares no package, so this is `[DEFAULT — adjustable]` | `UI-Components-KMP` | **Utilities-only scaffold, foreign-branded** (`Theme.kt` ships a Yole Material-red palette, zero widgets, no CI/publish) `[GAP: 8.4]`. This file is the contract that replaces the hand-kept palette when THREADY-DES-KMP-01 lands |
| SwiftUI | [`generated/swiftui/ThreadyTokens.swift`](./generated/swiftui/ThreadyTokens.swift) | `ThreadyTokens.LightColors`/`.DarkColors` (`Color(red:green:blue:)`, 6-decimal floats that round-trip to the source hex), `Spacing`/`Radius`/`TypeScale` (`CGFloat`) | (none today) | **No in-house SwiftUI package exists** `[OPEN: THREADY-DES-LIB-02]`; the sanctioned iOS path is KMP/Compose. Contract only, in case SwiftUI shims materialize |
| ArkTS / HarmonyOS | [`generated/arkts/thready_tokens.ets`](./generated/arkts/thready_tokens.ets) | `ThreadyColorLight`/`ThreadyColorDark` classes (ResourceColor-compatible hex strings), `ThreadySpacing`/`ThreadyRadius` (vp), `ThreadyTypeScale` (fp), `ThreadyMotion` (ms) | native ArkTS client via `helix_shims` | `helix_shims` interface **uninspected** `[GAP: 8.5]` `[OPEN: THREADY-DES-LIB-03]` — contract only |
| Qt / QML (Aurora) | [`generated/qml/ThreadyTokens.qml`](./generated/qml/ThreadyTokens.qml) | `pragma Singleton` `QtObject` with `light`/`dark` sub-objects (`color` properties) + `int`/`real` structure properties | `helix_design` Qt arm | **Verified empty scaffold** `[GAP: 8.2/8.3]`; Aurora path is native via `helix_shims` `[GAP: 8.5]` — contract only |
| TUI / Lipgloss | [`generated/lipgloss/thready_palette.go`](./generated/lipgloss/thready_palette.go) | `package theme`: dark truecolor vars (§3 names: `Accent`, `AccentOn`, `Fg`, `Muted`, `BorderColor`, `BorderHard`, `Success`, `Warn`, `DangerColor`, `Brand2`, + `Brand`/`Bg`/`SurfaceWarm`), pinned `CompleteColor` degradation set (§5 option 2) and `AdaptiveColor` light/dark set (§6) — matching `lipgloss-theme.md` **exactly**, with the doc's ASSUMED markers on every ANSI-256/16 pick carried through into the code comments | Thready TUI (Bubble Tea) | Bubble Tea + Lipgloss **pattern VERIFIED** in-house (`helix_track/llms_verifier/…/tui`); the Thready TUI itself is not built yet — styles are PROPOSED on the verified pattern (THREADY-DES-TUI-01) |
| Flutter | [`generated/flutter/thready_tokens.dart`](./generated/flutter/thready_tokens.dart) | `ThreadyColorsLight`/`ThreadyColorsDark` (`Color(0xFF…)` via `dart:ui`), `ThreadySpacing`/`ThreadyRadius`/`ThreadyTypeScale` (`double`), `ThreadyMotion` (`int` ms) | `helix_design` Flutter arm | **Verified empty scaffold** `[GAP: 8.2/8.3]` — contract only |

Token counts per target (colors count both modes; aliases included):
`tokens.json` **74** tokens · `ThreadyColors.kt` **71** declarations · `ThreadyTokens.swift` **71** ·
`thready_tokens.ets` **70** `static readonly` + 1 exported const = **71** · `ThreadyTokens.qml` **73**
properties · `thready_palette.go` **39** vars (13 roles × truecolor/Complete/Adaptive) ·
`thready_tokens.dart` **70** `static const` + 1 top-level const = **71**.

## 3. How to run

```bash
# From this directory (no npm install — zero dependencies, Node ≥ 18; node 24 verified):
node generate.mjs          # (re)generates ./generated/** and runs the self-check suite
node generate.mjs --check  # CI drift gate — see §4
```

After any change to `../opendesign/tokens.css` (or to the `lipgloss-theme.md §2` mapping tables),
re-run `node generate.mjs` and commit the regenerated outputs together with the source change.

## 4. The `--check` drift gate

`node generate.mjs --check` re-generates every target into a temp directory and:

1. **Drift:** byte-diffs each fresh output against the committed `generated/**` file (missing or
   differing file → FAIL, exit ≠ 0).
2. **Hex round-trip:** extracts every color literal from every output (including reconstructing
   hex from the SwiftUI float triplets) and asserts set-equality with the colors parsed from
   `tokens.css` — no invented, dropped, or mistyped color can survive.
3. **Structural self-checks:** balanced `{} () []` and exact expected symbol counts per target
   (the stand-in validation for toolchains not present on the machine — see §5).
4. **Source consistency:** the `@media` dark block and the `[data-theme="dark"], .dark` block of
   `tokens.css` must bind identical values; `lipgloss-theme.md §2`'s truecolor column must equal
   the parsed dark values; the DTCG output must `JSON.parse`.

Exercised both ways on 2026-07-22: pristine tree → 32/32 PASS, exit 0; a deliberately tampered
hex in `ThreadyColors.kt` → `FAIL drift`, exit 1; regenerate → PASS again.

## 5. Validation record (2026-07-22, this machine)

Per the no-bluff bar (`CONVENTIONS.md §7`) — what each output was **actually** validated with.
Toolchain availability was probed first: `node` v24.18.0 ✓, `go` 1.26.4 + `gofmt` ✓;
`kotlinc`, `swiftc`, `qmllint`, `dart`/`flutter`, `tsc` **MISSING** on this machine.

| Target | Validation method | Result |
|---|---|---|
| `web/tokens.json` | `JSON.parse` (node 24) + hex round-trip + DTCG `$type`/alias-reference structure emitted per spec | **PASS** (PenPot import itself not exercised — no PenPot here) |
| `compose/ThreadyColors.kt` | `kotlinc` **MISSING** → structural self-checks: balanced delimiters, 71/71 `val` declarations, hex round-trip | PASS (structural only — **not compiler-verified**) |
| `swiftui/ThreadyTokens.swift` | `swiftc` **MISSING** → structural self-checks incl. float→hex round-trip (6-decimal channels reconstruct the exact source hex) | PASS (structural only — **not compiler-verified**) |
| `arkts/thready_tokens.ets` | No ArkTS toolchain (and no `tsc`) → structural self-checks: balanced delimiters, 70/70 `static readonly`, hex round-trip | PASS (structural only — **not compiler-verified**) |
| `qml/ThreadyTokens.qml` | `qmllint` **MISSING** (also probed qt6 paths) → structural self-checks: balanced delimiters, 73/73 `readonly property`, hex round-trip | PASS (structural only — **not linter-verified**) |
| `lipgloss/thready_palette.go` | `gofmt -e` (parse) + `gofmt -l` (formatting) + **`go vet`** + **`go build`** in a temp module against real `github.com/charmbracelet/lipgloss v0.13.0` | **PASS** (compiles) |
| `flutter/thready_tokens.dart` | `dart` **MISSING** → structural self-checks: balanced delimiters, 70/70 `static const`, hex round-trip | PASS (structural only — **not compiler-verified**) |
| generator-level | dark-block consistency; `lipgloss-theme.md §2` truecolor ↔ `tokens.css` dark equality; drift gate positive + negative test | **PASS** |

The structural-only rows MUST be compiler-verified in the consumer repos' CI when the bindings are
wired in (§6) — that is part of the open half of THREADY-DES-LIB-04, not a formality.

## 6. Closure state of THREADY-DES-LIB-04

[`platform-map.md §6`](../library/platform-map.md#6-open-items) recorded: *"the token-bridge
codegen (CSS → Lipgloss / `ThreadyColors` / Flutter theme) referenced by every non-web cell does
not exist yet."* Honest state after this change:

- **CLOSED half:** the generator now **exists in-repo** (this directory), parses the canonical
  `tokens.css`, emits all seven bindings deterministically, and ships a CI-able `--check` drift
  gate. `lipgloss-theme.md §8`'s "this file specifies its TUI output, it does not claim the
  generator exists" is superseded for the *generator-existence* part.
- **OPEN half:** **wiring into the consumer repos remains open.** Nothing consumes these files
  yet: `UI-Components-KMP` still carries the foreign Yole palette `[GAP: 8.4]`, `helix_design` is
  an empty scaffold `[GAP: 8.2/8.3]`, the Thready TUI is unbuilt, ArkTS/Qt shims are uninspected
  `[GAP: 8.5]`. Tracked as THREADY-DES-KMP-01 / -FLUT-01 / -QT-01 / -TUI-01 in
  [component-library.md §10](../component-library.md#10-build-backlog--gaps); compiler
  verification of the structural-only targets (§5) lands with that wiring.

## 7. What is deliberately NOT exported

Mirrors the precedent set by [`../figma/figma-variables.json`](../figma/figma-variables.json)
`_meta.consumption_notes`:

- `--accent-hover`, `--accent-active`, `--elev-raised`, `--focus-ring` — CSS `color-mix()`
  formulas over `var()`; they have no static value. Native platforms derive hover/active/focus
  treatments from their own state systems over the exported base colors.
- `--elev-flat` (`none`) and `--elev-ring` (a shadow composite over `var(--border)`) — shadow
  composites, platform-specific by nature.
- Web-only `--font-*` stacks are exported **only** to the DTCG target (`fontFamily`); native
  targets load fonts through their own asset pipelines (`design-system.md §4`).
- B-slot aliases (`--fg-2`, `--meta`, `--border-soft`) and `--ds-heart` **are** exported — as real
  DTCG `{references}` / language-level aliases, preserving the alias semantics of `tokens.css`.

## 8. Open items

- `[OPEN: THREADY-DES-LIB-04]` — narrowed per §6: generator DONE, consumer wiring OPEN.
- `[OPEN: THREADY-DES-17]` — all ANSI-256/16 indices in the Go palette are ASSUMED nearest-color
  picks (carried verbatim, with markers, from `lipgloss-theme.md §2`); verify on real terminals.
- `[OPEN: THREADY-DES-02]` — PenPot bridge: `web/tokens.json` is the import file; an actual
  PenPot 2.x import run is an operator action (no PenPot in this environment).
- Compiler verification of the Kotlin / Swift / ArkTS / QML / Dart outputs in consumer-repo CI
  (kotlinc/swiftc/qmllint/dart were MISSING here — §5).

---

*Made with love ♥ by Helix Development.*
