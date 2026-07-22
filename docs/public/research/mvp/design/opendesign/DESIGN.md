<!--
  Title           : Helix Thready — OpenDesign Brand Contract (DESIGN.md)
  Classification  : PUBLIC
  Location        : docs/public/research/mvp/design/opendesign/DESIGN.md
  Status          : Draft — v0.1
  Revision        : 1 (2026-07-22)
  Author          : Helix Thready documentation swarm (design · opendesign)
  Related         : ./tokens.css, ./TOOLING.md, ../design-system.md, ../theming.md,
                    ../brand-assets.md, ../../CONVENTIONS.md
  Format          : OpenDesign 9-section DESIGN.md schema
                    [VERIFIED — open-design/docs/spec.md §2 bet #4 ("DESIGN.md files following
                    the awesome-claude-design 9-section schema") + design-systems/README.md
                    ("normalized 9-section DESIGN.md files"; legacy file shape: H1 title, then
                    a "> Category:" blockquote) + design-systems/default/DESIGN.md +
                    design-systems/claude/DESIGN.md (numbered-section exemplar)]
  Note            : The "> Category:" blockquote sits IMMEDIATELY after the H1 because the
                    OpenDesign picker parses "the line immediately after the H1"
                    [VERIFIED — design-systems/README.md "Legacy File Shape"]. The
                    CONVENTIONS.md revision table therefore follows the blockquote, not the
                    H1 directly — a deliberate, documented ordering to satisfy both contracts.
-->

# Thready

> Category: Helix Development
> Threads-reading companion by Helix Development. Chartreuse-to-mint spiral brand on an
> AA-tuned slate; Space Grotesk display, Hanken Grotesk body; light + dark first-class.

| Rev | Date | Author | Change |
|-----|------|--------|--------|
| 1 | 2026-07-22 | swarm (design · opendesign) | Initial brand contract in the verified OpenDesign 9-section `DESIGN.md` schema; every value derived from `../design-system.md` / `../theming.md` / `../brand-assets.md` — nothing invented |
| 2 | 2026-07-22 | swarm (design · opendesign · verify) | Lens A/B compliance per the contributor authoring guide `[VERIFIED — open-design/docs/design-systems.md]`: added the **Font labels for catalog extraction** block (§3 — daemon parser regexes), an in-file `:root {}` + `[data-theme="dark"]` token block (§2), and real component CSS with a targeted `prefers-reduced-motion` gate (§4). Values unchanged |
| 3 | 2026-07-22 | swarm (design · decisions) | `THREADY-DES-03` closed by operator ruling — heart stays `--ds-heart: var(--accent)`; §2 heart line states the decision (love-red dropped). Values unchanged |

**Machine-readable twin:** [`tokens.css`](./tokens.css) — the compiled CSS custom properties for
this contract, following the OpenDesign token schema
(`packages/contracts/src/design-systems/token-schema.ts`) with light + dark blocks.
**Tooling status:** [`TOOLING.md`](./TOOLING.md).

## Table of contents

- [1. Visual Theme & Atmosphere](#1-visual-theme--atmosphere)
- [2. Color Palette & Roles](#2-color-palette--roles)
- [3. Typography Rules](#3-typography-rules)
- [4. Component Stylings](#4-component-stylings)
- [5. Layout Principles](#5-layout-principles)
- [6. Depth & Elevation](#6-depth--elevation)
- [7. Do's and Don'ts](#7-dos-and-donts)
- [8. Responsive Behavior](#8-responsive-behavior)
- [9. Agent Prompt Guide](#9-agent-prompt-guide)

## 1. Visual Theme & Atmosphere

Thready is a **threads-reading companion**: calm, content-first, quietly organic. The brand mark
is a **spiral — a thread unspooling into a helix coil** — read simultaneously as the Helix
Development helix, a thread of posts, and an adaptive organic shell
`[VERIFIED — ../brand-assets.md §2]`. The palette flows the same way the mark does: a
chartreuse/lime green (`#B6E376`) sweeping into a soft mint/teal (`#ABDDC9`), both eyedrop-captured
from `Logo.png` pixel evidence, never guessed `[VERIFIED — ../design-system.md §3.2]`.

The surfaces stay out of the way: brand-neutral **slate** neutrals (white / `#020817` night,
`#475569` / `#94A3B8` muted) carried from the AA-verified `helix-green` theme of the shared
`vasic-digital/design_system` — Thready is a **brand theme layered on that shared system, never a
fork** `[VERIFIED — ../design-system.md §1]`. Light and dark are **both first-class**, with an
explicit choice and a system default `[OPERATOR]`. One design system covers **every** surface —
web, desktop, mobile, and the TUI `[OPERATOR — ../design-system.md §1]`.

**Key characteristics:**

- Spiral/thread brand mark, **letter-free by construction** (the OS prints the app title)
  `[OPERATOR — ../brand-assets.md §1]`
- Chartreuse→mint brand gradient, **decorative only** — text always uses the AA-pinned accent
- Slate neutrals shared with every Helix brand theme; brand color isolated so a white-label
  re-tints without touching structure `[VERIFIED — ../design-system.md §3]`
- Space Grotesk display / Hanken Grotesk body / JetBrains Mono code — all variable, self-hosted
- Light + dark palettes ship together; the user's mode choice selects which renders
- The locked footer: *"Made with ♥ by Helix Development"* — never removed, even white-labeled
  `[OPERATOR — ../theming.md §3]`

### Brand identity — logo assets

All masters live beside this contract (relative refs from this folder)
`[VERIFIED — files inspected]`:

| Asset | File | Use |
|-------|------|-----|
| Launcher icon (master, neutral) | [`../assets/launcher-icon.svg`](../assets/launcher-icon.svg) | Letter-free double-helix + thread motif; source of every OS export |
| Launcher icon — light variant | [`../assets/launcher-icon-light.svg`](../assets/launcher-icon-light.svg) | Light home screens / white UIs; fill `#446E12 → #B6E376` |
| Launcher icon — dark variant | [`../assets/launcher-icon-dark.svg`](../assets/launcher-icon-dark.svg) | Dark home screens / `#020817` UIs; fill `#B6E376 → #B7EBD6` |
| Launcher icon — monochrome | [`../assets/launcher-icon-mono.svg`](../assets/launcher-icon-mono.svg) | Android themed/monochrome, iOS 18 tinted, notifications; `currentColor` |
| Brand mark (tight crop) | [`../assets/logo-mark.svg`](../assets/logo-mark.svg) | Lockups, headers, avatars |
| Full logo lockup (mark + wordmark) | [`../assets/logo-full.svg`](../assets/logo-full.svg) | Headers / marketing **only**; never inside the launcher icon |
| Footer slogan lockup | [`../assets/footer-slogan.svg`](../assets/footer-slogan.svg) | "Made with ♥ by Helix Development" attribution |

Clear-space ≥ one outer ribbon width (≈ ⅛ of the icon box); misuse rules (no letters, no recolor
outside brand/mono, no skew, keep transparency) per `[VERIFIED — ../brand-assets.md §10]`.

### Voice & tone

- **Warm, human, understated.** The one emotional flourish is the locked slogan *"Made with love
  ♥ by Helix Development"* (the heart carries the accessible name "love")
  `[VERIFIED — ../brand-assets.md §8]`. Tagline: *"read your threads, smarter"*
  `[VERIFIED — ../brand-assets.md §8.1, login/splash]`.
- **Precise and honest.** Surfaces state what is real; unresolved items are marked, never papered
  over — mirroring the documentation "no bluff" bar `[VERIFIED — ../../CONVENTIONS.md §7]`.
- **Localized by contract, not by habit.** Every UI string goes through the shipped `I18nService`
  keys (`footer.made` / `footer.by` / `a11y.love` / `a11y.language`); shipped locales
  en / ru / sr-Cyrl `[OPERATOR — ../design-system.md §4]`.
- Calm, reading-oriented microcopy; short sentences; no exclamation-mark enthusiasm
  `[DEFAULT — adjustable]`.

## 2. Color Palette & Roles

Every value below is carried verbatim from the Thready theme
(`tokens/themes/thready.css` design, `[VERIFIED — ../design-system.md §3.2]`) which extends the
measured, AA-verified `helix-green` theme. Nothing here is invented `[CONSTITUTION §11.4.6]`.

### Brand (decorative only — never body text)

- **Brand base** (`#B6E376`) — chartreuse/lime; eyedrop **mean of 1,625,855 green-dominant logo
  pixels** `[VERIFIED]`. Only **1.47:1** on white — logo marks, gradients, one focal element.
- **Brand secondary — light** (`#ABDDC9`) — mint/teal; eyedrop mean of the `Logo.png` mint region
  (n = 618,886) `[VERIFIED]`.
- **Brand secondary — dark** (`#B7EBD6`) — the brighter mint **median**, used on dark surfaces so
  the mark holds `[VERIFIED]`.
- **Brand ink** (`#0A0F04`) — readable ink on a brand fill (13.15:1) `[VERIFIED]`.

### Accent (interactive / text-safe — AA-pinned)

- **Accent — light** (`#446E12`) deep green = **6.03:1 on white** ✅; `--accent-on: #FFFFFF`.
- **Accent — dark** (`#B6E376`) the logo lime *is* the dark accent = **13.56:1 on `#020817`** ✅;
  `--accent-on: #0A0F04`. `[VERIFIED — measured, design_system THEMES.md]`

### Surfaces & neutrals (brand-neutral slate, shared with all Helix themes)

| Role | Light | Dark |
|------|-------|------|
| Background `--bg` | `#FFFFFF` | `#020817` |
| Surface `--surface` | `#FFFFFF` | `#020817` |
| Warm/tertiary surface `--surface-warm` | `#F1F5F9` | `#1E293B` |
| Foreground `--fg` | `#020817` | `#F8FAFC` |
| Muted `--muted` | `#475569` | `#94A3B8` |
| Border `--border` | `#E2E8F0` | `#1E293B` |
| Strong border `--border-strong` (load-bearing boundaries ≥3:1) | `#64748B` | `#64748B` |

### Semantic (state, never decoration — never overridden by a brand)

| Role | Light | Dark |
|------|-------|------|
| Success | `#166534` | `#16A34A` |
| Warn | `#854D0E` | `#EAB308` |
| Danger | `#DC2626` | `#EF4444` |

`--accent` (brand) and `--danger` (error) are **separate tokens by contract** — a brand color can
never mask an error signal `[VERIFIED — ../design-system.md §6.2]`.

### Heart glyph

`--ds-heart` stays `var(--accent)` (AA-legible in both modes)
`[VERIFIED precedent — ../brand-assets.md §8]` — **decided**: operator ruling 2026-07-22
(`[CLOSED: THREADY-DES-03]`, owned by [../brand-assets.md §8](../brand-assets.md)) keeps the
accent-green heart (white-label-safe; AA in both modes); the classic love-red alternative is
dropped.

### White-label override points

Per-Account white-labeling swaps **only** these tokens, exactly as the three shipped themes prove
works (`helix-green` / `vasic-red` / `helix-ota-blue`) `[VERIFIED — ../theming.md §3]`:

| Override point | Allowed | Guardrail |
|----------------|:-------:|-----------|
| `--brand`, `--brand-2` (decorative) | ✅ | any hue; never used as text |
| `--accent`, `--accent-on` (light + dark pair) | ✅ | server-validated **WCAG AA ≥ 4.5:1** or rejected (`422`) `[VERIFIED — ../theming.md §10]` |
| Product logo (light + dark, transparent) | ✅ | Asset-service refs |
| Slogan / tagline | ✅ | free text |
| Structural tokens (type/space/radius/motion) | ❌ | one system across all Accounts |
| Neutral surfaces + semantic tokens | ❌ (default) | AA-tuned; advanced override gated behind full AA re-validation |
| Helix Development attribution footer | ❌ | always present |

Dark-mode selection uses the three sanctioned mechanisms: `@media (prefers-color-scheme: dark)`
(system), `:root[data-theme="dark"]` (explicit), `.dark` (framework compat)
`[VERIFIED — ../theming.md §2]`. See [`tokens.css`](./tokens.css) for the compiled blocks.

### Color token blocks (light default + dark override)

The color-bearing tokens, in the `:root {}` + `[data-theme="dark"]` override pattern the
authoring guide requires (`[VERIFIED — open-design/docs/design-systems.md §3]`; the full
pasteable set — structural tokens and all three dark mechanisms — is [`tokens.css`](./tokens.css)):

```css
:root {
  --theme-id: "thready";
  --brand: #b6e376;   --brand-2: #abddc9;   --brand-ink: #0a0f04;  /* decorative only */
  --bg: #ffffff;      --surface: #ffffff;   --surface-warm: #f1f5f9;
  --fg: #020817;      --muted: #475569;
  --border: #e2e8f0;  --border-strong: #64748b;
  --accent: #446e12;  --accent-on: #ffffff;   /* 6.03:1 on white [VERIFIED] */
  --success: #166534; --warn: #854d0e;        --danger: #dc2626;
  --ds-heart: var(--accent);
  --fg-2: var(--fg);  --meta: var(--muted);   --border-soft: var(--border); /* B-slot aliases */
}

[data-theme="dark"] {
  --brand: #b6e376;   --brand-2: #b7ebd6;   --brand-ink: #0a0f04;
  --bg: #020817;      --surface: #020817;   --surface-warm: #1e293b;
  --fg: #f8fafc;      --muted: #94a3b8;
  --border: #1e293b;  --border-strong: #64748b;
  --accent: #b6e376;  --accent-on: #0a0f04;   /* logo lime IS the dark accent — 13.56:1 [VERIFIED] */
  --success: #16a34a; --warn: #eab308;        --danger: #ef4444;
  --ds-heart: var(--accent);
}
```

## 3. Typography Rules

Three shipped **variable** faces, self-hosted via `@fontsource` — no external CDN (offline/CSP
posture) `[VERIFIED — ../design-system.md §4]`:

- **Display / headings:** `"Space Grotesk Variable", ui-sans-serif, system-ui, sans-serif` —
  weights 600–700, `h1–h3`, hero
- **Body / UI:** `"Hanken Grotesk Variable", ui-sans-serif, system-ui, sans-serif` —
  weights 400–500 (buttons 500, badges 600 `[VERIFIED — .ds-btn / .ds-badge]`)
- **Mono / code:** `"JetBrains Mono", ui-monospace, "SF Mono", Menlo, monospace` — code, post
  payloads, CLI transcripts, hashtags, IDs; **tabular figures** for aligned columns

Font labels for catalog extraction:

Display: "Space Grotesk Variable", ui-sans-serif, system-ui, sans-serif
Body: "Hanken Grotesk Variable", ui-sans-serif, system-ui, sans-serif
Mono: "JetBrains Mono", ui-monospace, "SF Mono", Menlo, monospace

**Scale (px):** 12 · 14 · 16 · 20 · 24 · 32 · 48 · 64 (`--text-xs` … `--text-4xl`)
**Line-height:** 1.5 body / 1.2 headings. **Letter-spacing:** −0.01em on display sizes.
`[VERIFIED — core.css, ../design-system.md §3.1]`

Weight steps are subsets of one variable `wght` axis — no extra font downloads. Cyrillic coverage
(ru, sr-Cyrl) is mandatory `[OPERATOR §12]`; subset verification is tracked as
`[OPEN: THREADY-DES-04]`. Logical properties keep the type layer direction-agnostic.

## 4. Component Stylings

Principles first (non-negotiable `[VERIFIED — ../design-system.md §1/§6]`):

- **Consume, never fork** — every widget comes from the shared `.ds-*` set + Thready extensions;
  no bespoke one-off components, **TUI included**.
- **Brand is decorative, accent is interactive, semantic is state** — three disjoint color roles.
- **Focus visible everywhere** — `--focus-ring` (3px accent-tinted) on `:focus-visible`; shipped
  `.ds-btn`, `.ds-input`, toggle, and picker all implement it `[VERIFIED]`.
- Keyboard + screen-reader complete: correct accessible names (theme toggle `aria-pressed`,
  language picker is a native `<select>`) `[VERIFIED]`.

Stylings (token-bound; values from `[VERIFIED — ../design-system.md §5]`):

- **Buttons:** `--radius-sm` (8px), body face at weight 500, primary = `--accent` fill with
  `--accent-on` label; hover/active via `--accent-hover` / `--accent-active` (oklab mixes);
  focus via `--focus-ring`.
- **Cards:** `--surface` on `--border` 1px, `--radius-md` (12px); prefer `--elev-ring` over
  shadows in dark.
- **Inputs:** 1px `--border`, `--radius-sm`, accent-tinted focus ring; error state uses
  `--danger`, never the brand.
- **Chips / toggles / badges:** `--radius-pill`; badges weight 600 `[VERIFIED]`.
- **Sheets / large surfaces:** `--radius-lg` (16px).
- **Footer:** `.ds-footer` with the heart lockup — lucide SVG heart in `--ds-heart`, ordered
  fallbacks SVG → `♥` → the word "love" `[VERIFIED — ../brand-assets.md §8]`.

### Reference CSS (shipped button contract, verbatim)

The exact shipped `.ds-btn` — every color a token reference, focus via `--focus-ring`,
reduced-motion gated on the component (never a global `*`)
`[VERIFIED — design_system components.css, reproduced in ../component-library.md §3]`:

```css
.ds-btn { display:inline-flex; align-items:center; gap:var(--space-2);
  font:500 var(--text-base)/1 var(--font-body); padding:var(--space-3) var(--space-5);
  border-radius:var(--radius-sm); border:1px solid transparent; cursor:pointer;
  transition: background-color var(--motion-fast) var(--ease-standard),
              color var(--motion-fast) var(--ease-standard),
              box-shadow var(--motion-fast) var(--ease-standard); }
.ds-btn:focus-visible { outline:none; box-shadow:var(--focus-ring); }
.ds-btn--primary   { background:var(--accent); color:var(--accent-on); }
.ds-btn--primary:hover  { background:var(--accent-hover); }
.ds-btn--primary:active { background:var(--accent-active); }
.ds-btn--secondary { background:transparent; color:var(--fg); border-color:var(--border-strong); }
.ds-btn--secondary:hover { background:var(--surface-warm); }
.ds-btn--ghost     { background:transparent; color:var(--accent); }
.ds-btn--ghost:hover { background:color-mix(in oklab, var(--accent), transparent 90%); }
@media (prefers-reduced-motion: reduce) { .ds-btn { transition:none; } }
```

Badges tint by formula — `color-mix(in oklab, <semantic>, transparent 90%)` — so a theme or
white-label swap re-tints every semantic badge with no new CSS `[VERIFIED — ../component-library.md §3]`.

### Forms: validation + hints (mandated)

`[OPERATOR — "full forms validations, hints"]` Every field ships a **hint** line (`--muted`,
`--text-sm`, wired via `aria-describedby`) and an inline **error** state (`--danger` text +
border, `aria-invalid`, `aria-describedby` to the message). Errors are instructive, never bare —
they carry the failing value and a concrete fix, the shape the branding editor's AA gate models
(`422` returns the measured ratio + an AA-passing suggestion) `[VERIFIED — ../theming.md §7/§10]`.
The full lifecycle (Loading → Skeleton ≥150ms → Content/Empty/Error → Validating → InvalidInline)
is the single state machine in [`../component-library.md §5b`](../component-library.md#5b-component-state-lifecycle).

## 5. Layout Principles

`[VERIFIED — core.css layout tokens, ../design-system.md §3.1/§5]`

- Content container **max 1200px** (`--container-max`), centered.
- Gutters: **24px** desktop / **16px** tablet / **12px** phone.
- Spacing on a **4px base**: 4 · 8 · 12 · 16 · 20 · 24 · 32 · 48 (`--space-1` … `--space-12`).
- Section vertical rhythm: **80px** desktop / **48px** tablet / **32px** phone
  (`--section-y-*`).
- Whitespace is the primary separator; the reading pane is the hero of every screen
  `[DEFAULT — adjustable]`.

## 6. Depth & Elevation

Three levels only `[VERIFIED — ../design-system.md §3.1/§5]`:

| Level | Token | Treatment | Use |
|-------|-------|-----------|-----|
| Flat | `--elev-flat` | `none` | default |
| Ring | `--elev-ring` | `0 0 0 1px var(--border)` | hairline lift; **preferred in dark** (avoids muddy shadows) |
| Raised | `--elev-raised` | `0 2px 8px` fg-mix at 8% (`color-mix` in oklab) | dropdowns, modals, floating elements |

No neumorphism, no glassmorphism `[DEFAULT — adjustable, aligned with the shared system]`.

### Motion

- Two durations: `--motion-fast` **150ms** (hover/press, control state) and `--motion-base`
  **200ms** (enter/exit, disclosure), on `--ease-standard: cubic-bezier(0.2, 0, 0, 1)`
  `[VERIFIED]`.
- **All motion honors `prefers-reduced-motion: reduce`** — the shipped components already gate
  transitions on it `[VERIFIED — components.css]`.
- Larger choreographed transitions (route changes, skeleton→content) extend these in
  [`../prototypes.md`](../prototypes.md) `[DEFAULT — adjustable]`.

## 7. Do's and Don'ts

### Do

- ✅ Use `--accent` for every interactive/text emphasis; it is the only AA-pinned green.
- ✅ Keep the brand gradient (`#B6E376 → #ABDDC9`/`#B7EBD6`) for marks, gradients, one focal
  element per screen.
- ✅ Ship light **and** dark for anything brand-colored; dark uses the brighter mint median.
- ✅ Route every string through i18n keys (en / ru / sr-Cyrl); test Cyrillic.
- ✅ Keep the Helix Development attribution footer on every surface — including white-labels.
- ✅ Use `--border-strong` for load-bearing boundaries (≥3:1 non-text contrast).
- ✅ Respect `prefers-reduced-motion` and reduced transparency.

### Don't

- ❌ Never set body text in `--brand` or `--brand-2` (1.47:1 / ~2:1 on white)
  `[VERIFIED — ../design-system.md §6.2]`.
- ❌ Never use brand color for destructive UI — errors are always `--danger`.
- ❌ Never invent hex values outside this palette `[CONSTITUTION §11.4.6]` — if a request needs
  one, surface a warning and use the closest existing token.
- ❌ Never put letters inside the launcher icon `[OPERATOR — ../brand-assets.md §1]`.
- ❌ Never load fonts from an external CDN — all faces are self-hosted.
- ❌ Never re-bind structural tokens per theme or per Account — brand color only.

## 8. Responsive Behavior

Three tiers, bound to the section-rhythm and gutter tokens `[VERIFIED tiers — core.css]`; the
pixel boundaries follow the OpenDesign starter defaults `[DEFAULT — adjustable]`:

- **Desktop ≥ 1024px:** 24px gutters, 80px section rhythm, full multi-column layout.
- **Tablet 640–1023px:** 16px gutters, 48px section rhythm, condensed nav.
- **Phone < 640px:** 12px gutters, 32px section rhythm, single column.

Touch targets ≥ 44×44px on touch surfaces `[DEFAULT — adjustable]`. Layout uses logical
properties (`padding-inline` / `margin-inline`) so nothing assumes LTR
`[VERIFIED — ../design-system.md §6.6]`.

### Per-platform notes

The CSS custom properties are canonical; every non-web platform consumes a **generated** binding
(OpenDesign `tokensToJson` → per-platform codegen), never a hand-kept copy
`[VERIFIED mechanism — ../design-system.md §7]`. Honest status per the gap register:

| Platform | Mechanism | Status | Notes |
|----------|-----------|--------|-------|
| **Web — Angular 19 (product) / 22 (marketing)** | `@vasic-digital/design-system` `.ds-*` + `ThemeService`/`I18nService` adapters | PRODUCTION-usable (web foundation) `[GAP: 8.1]` | Primary surface; Thready adds one theme file + `DS_CONFIG` (`storagePrefix: 'thready'`, `defaultTheme: 'system'`) |
| **Desktop — Tauri 2** | Wraps the Angular web UI | Inherits web | OS-chrome overrides only; no separate token work |
| **Mobile — KMP / Compose (Android, iOS/SwiftUI host)** | `UI-Components-KMP` + token-bridge codegen (Compose `Color`/`Dp`) | **SCAFFOLD**, no CI/publish `[GAP: 8.4]` | Fetches effective branding at login; do not claim it works today |
| **Mobile alt — ArkTS (HarmonyOS) / Qt (Aurora)** | native clients via `helix_shims`; `helix_design` tokens | **SCAFFOLD** `[GAP: 8.2/8.3/8.5]` | Only HarmonyOS/Aurora path; layered icon JSON ready |
| **TUI — Lipgloss (Bubble Tea)** | Generated Go palette from the same tokens | Pattern verified in-house `[VERIFIED]` | Dark palette default: `Accent #B6E376`, `Fg #F8FAFC`, `Muted #94A3B8`, `Border #1E293B`, `Danger #EF4444`, `Success #16A34A`; heart = `♥` glyph |

## 9. Agent Prompt Guide

### Quick color reference

- Accent (light): "deep green (#446E12)" on white — the only text-safe green in light mode
- Accent (dark): "logo lime (#B6E376)" on night (#020817)
- Brand gradient: "chartreuse (#B6E376) → mint (#ABDDC9 light / #B7EBD6 dark)" — decorative only
- Page background: "#FFFFFF light / night (#020817) dark"
- Text: "ink (#020817) light / snow (#F8FAFC) dark"; muted "slate (#475569 / #94A3B8)"
- Border: "#E2E8F0 light / #1E293B dark"; strong boundary "#64748B"
- Danger stays "#DC2626 / #EF4444" — never the brand green family

### Example component prompts

- "Create a primary button: deep-green (#446E12) fill, white label, 8px radius, Hanken Grotesk
  500, 150ms hover to the oklab-darkened accent, 3px accent-tinted focus ring."
- "Design a post card on `--surface` with a 1px #E2E8F0 border, 12px radius, title in Space
  Grotesk 600 at 20px, body in Hanken Grotesk at 16px/1.5, metadata in JetBrains Mono with
  tabular figures."
- "Build the dark variant: night (#020817) surface, snow (#F8FAFC) text, lime (#B6E376) accent
  with dark ink (#0A0F04) on fills, ring elevation instead of shadows."
- "Add the footer: 'Made with ♥ by Helix Development', heart in var(--ds-heart), SVG → ♥ → 'love'
  fallbacks, localized via footer.made / footer.by."

### Iteration guide

1. Paste the [`tokens.css`](./tokens.css) `:root` + dark blocks into the artifact's first
   `<style>` and reference everything via `var(--…)` — never restate hex inline.
2. One brand-gradient focal element per screen, at most.
3. Always author both modes; check the accent flips (#446E12 ↔ #B6E376) and `--accent-on`
   flips (#FFFFFF ↔ #0A0F04).
4. Errors and destructive actions: `--danger` only.
5. If a value is missing, do **not** invent it — flag it and use the nearest token
   `[CONSTITUTION §11.4.6]`.

---

*Made with love ♥ by Helix Development.*
