<!--
  Title           : Helix Thready — TUI Lipgloss Theme (brand tokens → Lipgloss styles)
  Classification  : PUBLIC
  Location        : docs/public/research/mvp/design/screens/tui/lipgloss-theme.md
  Status          : Draft — v0.1
  Revision        : 1 (2026-07-22)
  Author          : Helix Thready documentation swarm (design · screens)
  Related         : ./tui-screens.html, ./README.md, ../../design-system.md (§7),
                    ../../wireframes.md (§5.1–5.3), ../../../CONVENTIONS.md
-->

# Helix Thready — TUI Lipgloss Theme

| Rev | Date | Author | Change |
|-----|------|--------|--------|
| 1 | 2026-07-22 | swarm (design · screens) | Initial: token→`lipgloss.Color` map with VERIFIED/ASSUMED provenance, ANSI-256/16 degradation, role styles, adaptive option |

## Table of contents

- [1. Provenance rules](#1-provenance-rules)
- [2. Token → color map](#2-token--color-map)
- [3. Palette package (Go)](#3-palette-package-go)
- [4. Role styles (verified bindings, tokenized)](#4-role-styles-verified-bindings-tokenized)
- [5. Degradation: truecolor → ANSI-256 → ANSI-16](#5-degradation-truecolor--ansi-256--ansi-16)
- [6. Light-terminal / adaptive option](#6-light-terminal--adaptive-option)
- [7. White-label re-tinting](#7-white-label-re-tinting)
- [8. Open items](#8-open-items)

## 1. Provenance rules

Per the no-bluff bar (`CONVENTIONS.md §7`), every value below is tagged:

- **VERIFIED** — the hex value comes verbatim from `tokens/themes/thready.css`
  ([design-system.md §3.2](../../design-system.md#32-the-thready-brand-theme), itself
  `[VERIFIED — IN-HOUSE: design_system]`), or the style-role pattern comes from the inspected
  in-house Bubble Tea program `helix_track/llms_verifier/llm-verifier/tui`
  ([wireframes.md §5.2](../../wireframes.md#52-lipgloss-style-bindings)).
- **ASSUMED** — a mapping judgment made here (nearest ANSI-256/16 index, light-terminal adaptive
  pairing, `CompleteColor` usage) that ground truth does not pin down. Assumed values MUST be
  re-verified at integration (run Lipgloss's own degradation and eyeball on a 16-color terminal).
- The Go palette *shape* below follows the generated-bindings sample in
  [design-system.md §7](../../design-system.md#7-per-platform-fan-out), which is itself
  `[DEFAULT — adjustable]` — the palette is **generated from `thready.css`**, never hand-kept.

The TUI defaults to the **dark** theme values ("the TUI defaults to the terminal's dark surface",
design-system §7 `[VERIFIED — the generated sample uses the dark palette]`).

## 2. Token → color map

| Role | CSS token | Truecolor (dark) | Truecolor provenance | ANSI-256 | ANSI-16 | Mapping provenance |
|------|-----------|------------------|----------------------|----------|---------|--------------------|
| Accent / selection bg | `--accent` | `#B6E376` | **VERIFIED** | `150` | `10` bright-green | ASSUMED |
| Accent ink (on accent) | `--accent-on` | `#0A0F04` | **VERIFIED** | `16` | `0` black | ASSUMED |
| Foreground | `--fg` | `#F8FAFC` | **VERIFIED** | `255` | `15` bright-white | ASSUMED |
| Muted / help / meta | `--muted` | `#94A3B8` | **VERIFIED** | `103` | `7` white | ASSUMED |
| Border / statusbar bg | `--border` | `#1E293B` | **VERIFIED** | `236` | `8` bright-black | ASSUMED |
| Strong boundary | `--border-strong` | `#64748B` | **VERIFIED** | `102` | `8` bright-black | ASSUMED |
| Success | `--success` | `#16A34A` | **VERIFIED** | `35` | `2` green | ASSUMED |
| Warn | `--warn` | `#EAB308` | **VERIFIED** | `178` | `11` bright-yellow | ASSUMED |
| Danger | `--danger` | `#EF4444` | **VERIFIED** | `203` | `9` bright-red | ASSUMED |
| Brand (decorative) | `--brand` | `#B6E376` | **VERIFIED** | `150` | `10` bright-green | ASSUMED |
| Brand-2 / titles | `--brand-2` | `#B7EBD6` | **VERIFIED** (Logo.png median, dark) | `152` | `14` bright-cyan | ASSUMED |
| Background | `--bg` | `#020817` | **VERIFIED** | `232` | `0` black (terminal default) | ASSUMED |
| Surface-warm | `--surface-warm` | `#1E293B` | **VERIFIED** | `236` | `8` bright-black | ASSUMED |

Light-theme counterparts (`--accent #446E12`, `--fg #020817`, `--muted #475569`, …) are equally
**VERIFIED** from the same theme file and are used only by the adaptive option in §6.

## 3. Palette package (Go)

Generated from `tokens/themes/thready.css` by the token-export step (OpenDesign `tokensToJson` →
codegen, design-system §7). Hex values **VERIFIED**; the code shape extends the design-system §7
sample `[DEFAULT — adjustable]`:

```go
// Package theme is GENERATED from tokens/themes/thready.css — do not edit by hand.
// Dark palette: the TUI defaults to the terminal's dark surface (design-system §7).
package theme

import "github.com/charmbracelet/lipgloss"

var (
    Accent      = lipgloss.Color("#B6E376") // --accent (dark)          [VERIFIED]
    AccentOn    = lipgloss.Color("#0A0F04") // --accent-on              [VERIFIED]
    Fg          = lipgloss.Color("#F8FAFC") // --fg                     [VERIFIED]
    Muted       = lipgloss.Color("#94A3B8") // --muted                  [VERIFIED]
    BorderColor = lipgloss.Color("#1E293B") // --border                 [VERIFIED]
    BorderHard  = lipgloss.Color("#64748B") // --border-strong          [VERIFIED]
    Success     = lipgloss.Color("#16A34A") // --success                [VERIFIED]
    Warn        = lipgloss.Color("#EAB308") // --warn                   [VERIFIED]
    DangerColor = lipgloss.Color("#EF4444") // --danger                 [VERIFIED]
    Brand2      = lipgloss.Color("#B7EBD6") // --brand-2 (dark median)  [VERIFIED]
)
```

## 4. Role styles (verified bindings, tokenized)

The role set is the **verified** binding table of wireframes §5.2 — the in-house reference builds
exactly these roles with raw ANSI indices (`"205"`, `"39"`, `"240"`, `"62"`, `"241"`
`[VERIFIED — llm-verifier app.go]`); Thready re-binds them to the generated palette:

```go
// tui/theme_bindings.go — roles VERIFIED (llm-verifier pattern); values from §3.
var (
    Title   = lipgloss.NewStyle().Foreground(Accent).Bold(true)      // was Color("205")
    Header  = lipgloss.NewStyle().Foreground(Brand2).Bold(true)      // pane titles (Thready addition, ASSUMED)
    NavOn   = lipgloss.NewStyle().Foreground(Accent)                 // active tab   (was "39")
    NavOff  = lipgloss.NewStyle().Foreground(Muted)                  // inactive tab (was "240")
    Border  = lipgloss.NewStyle().BorderForeground(BorderColor)      // header/footer border (was "62")
    Help    = lipgloss.NewStyle().Foreground(Muted)                  // key hints    (was "241")
    Danger  = lipgloss.NewStyle().Foreground(DangerColor)            // failed steps / retry
    Ok      = lipgloss.NewStyle().Foreground(Success)                // ✓ processed  (ASSUMED addition)
    Warning = lipgloss.NewStyle().Foreground(Warn)                   // ⚠ auth / degraded (ASSUMED addition)

    // Selection + button: accent fill with accent-on ink — mirrors design-system §7 Button [DEFAULT — adjustable].
    Selected  = lipgloss.NewStyle().Foreground(AccentOn).Background(Accent)
    Button    = lipgloss.NewStyle().Foreground(AccentOn).Background(Accent).Padding(0, 2).Bold(true)
    StatusBar = lipgloss.NewStyle().Foreground(Muted).Background(lipgloss.Color("#1E293B")) // --surface-warm dark
    Heart     = lipgloss.NewStyle().Foreground(Accent) // ♥ U+2665 in the locked footer (brand-assets §8)
)
```

## 5. Degradation: truecolor → ANSI-256 → ANSI-16

Ground truth says only: "Under a non-truecolor terminal the tokens degrade to the nearest
256-color; border *styles* (not just color) keep boundaries legible" (wireframes §5.2
`[VERIFIED]`). Two implementation options — both **ASSUMED** until tested on a 16-color terminal:

1. **Automatic (default):** `lipgloss.Color("#B6E376")` — Lipgloss/termenv degrades to the
   nearest supported color per the terminal's advertised profile. Zero code, but the ANSI-16
   result is not pinned.
2. **Pinned (recommended for the 16-color floor):** `lipgloss.CompleteColor` fixes every profile
   explicitly, using the §2 table:

```go
// ASSUMED — pin the degradation so a 16-color terminal gets a deliberate palette, not a guess.
var AccentC = lipgloss.CompleteColor{TrueColor: "#B6E376", ANSI256: "150", ANSI: "10"}
var MutedC  = lipgloss.CompleteColor{TrueColor: "#94A3B8", ANSI256: "103", ANSI: "7"}
var DangerC = lipgloss.CompleteColor{TrueColor: "#EF4444", ANSI256: "203", ANSI: "9"}
// … full set per the §2 table.
```

Legibility caveat (also noted in [tui-screens.html](./tui-screens.html)): the true `--border`
`#1E293B` is nearly invisible on the `#020817` terminal background (~1.3:1). That is consistent
with the web rule that borders are subtle, but on ANSI-16 terminals the border maps to
`8` bright-black, which typically renders ≈ `#475569`-ish — *more* visible than truecolor. The
mockups draw box glyphs with that ANSI-8 approximation for legibility. If truecolor borders prove
illegible in practice, promote box glyphs to `--border-strong` `#64748B` — tracked as
`[OPEN: THREADY-DES-17]`.

## 6. Light-terminal / adaptive option

Ground truth fixes the **dark** default; nothing forbids a light terminal. **ASSUMED** option
using the equally-verified light token values:

```go
// ASSUMED — adaptive pairing from the same theme file (light values are VERIFIED tokens).
var AccentA = lipgloss.AdaptiveColor{Light: "#446E12", Dark: "#B6E376"} // --accent light/dark
var FgA     = lipgloss.AdaptiveColor{Light: "#020817", Dark: "#F8FAFC"} // --fg
var MutedA  = lipgloss.AdaptiveColor{Light: "#475569", Dark: "#94A3B8"} // --muted
var DangerA = lipgloss.AdaptiveColor{Light: "#DC2626", Dark: "#EF4444"} // --danger
```

Adopting this is a product decision (the TUI currently ships dark-only) — fold into
`[OPEN: THREADY-DES-17]`.

## 7. White-label re-tinting

Because the palette is generated from the same token source as every other surface, a per-Account
white-label re-tints the TUI without touching Go: the resolver emits a regenerated Lipgloss
palette from the effective brand, read at login from `GET /v1/accounts/{id}/branding`
(theming §8 `[VERIFIED — stated mechanism]`). Only `Accent`/`AccentOn`/`Brand2` change; neutrals
and semantics stay fixed, so `--danger` can never be masked by a brand color (theming §10.2).

## 8. Open items

- `[OPEN: THREADY-DES-17]` — TUI degradation decisions: (a) automatic vs. pinned
  `CompleteColor` ANSI-16 mapping (all §2 ANSI columns are ASSUMED nearest-color picks);
  (b) border glyph color under truecolor (`--border` vs. `--border-strong`); (c) whether to ship
  the light-terminal adaptive palette (§6). Verify on real 16-color and truecolor terminals.
- The generated-palette codegen step itself is part of `THREADY-DES-DS-01` / the token-bridge
  workable items (design-system §9) — this file specifies its TUI output, it does not claim the
  generator exists.

---

*Made with love ♥ by Helix Development.*
