<!--
  Title           : Helix Thready — React Token Re-bind (Remediation Contract)
  Classification  : PUBLIC
  Location        : docs/public/research/mvp/design/platforms/react-token-rebind.md
  Status          : Draft — v0.1
  Revision        : 1 (2026-07-22)
  Author          : Helix Thready documentation swarm (design · platforms)
  Related         : ./README.md, ../library/platform-map.md (§2), ../design-system.md (§3/§7),
                    ../opendesign/tokens.css, ../theming.md, ../../CONVENTIONS.md
-->

# Helix Thready — React Token Re-bind (Remediation Contract)

| Rev | Date | Author | Change |
|-----|------|--------|--------|
| 1 | 2026-07-22 | swarm (design · platforms) | Initial remediation contract for `THREADY-DES-LIB-01` / `[GAP: 8.6]`: verified finding restated, required re-bind approach (CSS custom properties, no Tailwind hex literals), variant→token map, acceptance criteria |

**Scope guard.** This is **not** a React component spec — the component contracts live in
[component-library.md](../component-library.md) and the per-cell realizations in
[platform-map.md §3](../library/platform-map.md#3-the-matrix). This file is the **remediation
contract** that turns the flagged `UI-Components-React` package from "files exist" into
"token-bound", i.e. the *definition of done* for the styling half of
`[OPEN: THREADY-DES-LIB-01]` / `THREADY-DES-REACT-01`. Whether React is adopted at all remains
"only if a React surface is needed" `[VERIFIED — design-system §7]` — this contract does not
create a React surface.

## 1. What was found (verified, 2026-07-22)

From [platform-map §2](../library/platform-map.md#2-per-repo-verification-results), second
verification pass (`gh`, file contents) `[VERIFIED]`:

- `vasic-digital/UI-Components-React` ships 13 component files at HEAD (`Avatar`, `Badge`,
  `Button`, `Card`, `EmptyState`, `ErrorBoundary`, `Input`, `LoadingSpinner`, `Progress`,
  `Select`, `Switch`, `Tabs`, `Textarea`), each with a test — **file existence verified**.
- `Button.tsx` **content** inspected: variants `primary|secondary|outline|ghost|destructive`,
  sizes `sm|md|lg`, a `loading` spinner prop — **but `variantClasses` hard-codes a Tailwind
  palette** (`bg-blue-600`, gray neutrals, `focus-visible:ring-blue-500`); **no design-system
  token is referenced anywhere**.
- Consequence recorded there: the package is `SCAFFOLD/FLAGGED` `[GAP: 8.6]`; "the blocking work
  is the token bridge …, then re-audit."

So the failure mode is precise: the components are structurally fine but **paint the wrong
product** — a blue Tailwind default instead of the Thready token palette — and are deaf to
theming (light/dark re-binds, per-Account white-label) by construction, since Tailwind literal
classes resolve to fixed hex at build time.

## 2. Required re-bind approach

**Single rule: every color the package emits must resolve through a CSS custom property from the
canonical token set — zero Tailwind palette literals, zero hex literals, in `src/components/`.**

- **Token source:** the canonical custom properties — `tokens/core.css` + `tokens/themes/
  thready.css` (design-system §3), machine-readable twin in
  [opendesign/tokens.css](../opendesign/tokens.css). The React package **consumes** these vars;
  it never redeclares values (the "generated binding, never a hand-kept copy" rule,
  design-system §7 — for React the "binding" is trivial: CSS vars are native to the platform).
- **Mechanism** (either satisfies this contract; pick one package-wide):
  1. **Plain CSS vars in class styles** — component classes styled with
     `background: var(--accent); color: var(--accent-on); box-shadow: var(--focus-ring)` etc.,
     mirroring the shipped `.ds-btn` contract verbatim (opendesign/DESIGN.md §4 reference CSS).
  2. **Tailwind v4 `@theme` bridge** — Tailwind v4 theme colors declared *as* the token vars
     (the `tailwind-v4.css` route platform-map §2 names), so utilities like `bg-accent`
     compile to `var(--accent)`. Allowed **only** if the emitted CSS references the vars —
     re-baking hex defeats the purpose.
- **Dark mode & white-label come for free and must not be reimplemented:** because values live
  in the vars, the three sanctioned theme mechanisms (`prefers-color-scheme`,
  `:root[data-theme="dark"]`, `.dark` — theming §2 `[VERIFIED]`) and the per-Account
  `:root[data-account=…]` injection (theming §6) re-tint the package with **no** `dark:` color
  utilities and no React theme context for colors.

### Variant → token map (normative)

Derived 1:1 from the shipped `.ds-btn` contract + design-system §5/§6 `[VERIFIED sources; the
mapping to React variant names is the contract this file adds]`:

| `Button.tsx` variant | Replace (found) | With (token binding) |
|---|---|---|
| `primary` | `bg-blue-600` + white | fill `var(--accent)`, ink `var(--accent-on)`, hover `var(--accent-hover)`, active `var(--accent-active)` |
| `secondary` | gray neutrals | fill `var(--surface-warm)`, ink `var(--fg)`, border `var(--border)` |
| `outline` | gray border/ink | transparent fill, border `var(--border-strong)`, ink `var(--fg)` |
| `ghost` | gray ink | transparent fill, ink `var(--accent)` |
| `destructive` | (red literal) | **`var(--danger)`** fill + AA-checked ink — never the brand family `[VERIFIED rule — design-system §6.2]` |
| focus (all) | `focus-visible:ring-blue-500` | `box-shadow: var(--focus-ring)` on `:focus-visible` |
| disabled (all) | gray literals | `var(--muted)` ink, reduced-opacity token surface; no pointer events |

Structure tokens ride along: radius `var(--radius-sm)`, body face weight 500, spacing
`var(--space-*)`, motion `var(--motion-fast)`/`var(--ease-standard)` with the component-level
`prefers-reduced-motion` gate (never a global `*` — the shipped `.ds-btn` pattern `[VERIFIED]`).
The same substitution discipline applies to the other 12 components (`Badge` → semantic badge
tokens; `Progress`/`LoadingSpinner` → `--accent` track / `--danger` failed; `Input`/`Textarea`/
`Select` → `--border`, focus ring, `--danger` error; etc.) using their `.ds-*`/composite
contracts in [component-library.md](../component-library.md) as the source.

## 3. Acceptance criteria (closes the styling half of the gap)

`[GAP: 8.6]` / `THREADY-DES-LIB-01` is closable for styling when **all** of the following hold:

1. **Zero literals:** no Tailwind palette classes (`bg-blue-*`, `text-gray-*`,
   `ring-blue-*`, …) and no raw hex/rgb/hsl color literals under `src/components/` — enforced by
   a lint/grep gate in the package CI, not by review memory.
2. **Var resolution proven:** rendered output (computed styles) for every variant×state
   references the token vars; toggling `data-theme="dark"` and injecting a `data-account`
   white-label re-tints components **without rebuild**.
3. **Variant map conformance:** the §2 table verified per component against the living library
   ([library/components.html](../library/components.html)) in light **and** dark.
4. **Semantic safety:** `destructive`/error styling resolves to `--danger` under every theme and
   white-label (the never-masked rule, theming §10.2).
5. **State completeness re-audit:** the remainder of `THREADY-DES-LIB-01` — each of the 13 files
   actually implements hover/focus/disabled/loading/error as the library specifies — executed
   component-by-component (this contract fixes *paint*; the state audit is its sibling).
6. **A11y unchanged or better:** focus ring visible via `--focus-ring`; AA contrast re-checked
   with the existing meter for every new pairing.
7. **Visual-regression hook:** the re-bound components enter the theme×state
   `ScreenDiff`/`VisualRegression` bank once CI exists `[GAP: 9.3]` — until then, criteria 1–6
   are the (manually evidenced) gate.

Ownership: the build work is `THREADY-DES-REACT-01` (index
[workable-items registry](../index.md#workable-items-registry)); upstreaming the re-bind to
`vasic-digital/UI-Components-React` follows the same upstream-contribution path as the other
proposed extensions (`[OPEN: THREADY-DES-12]`). No new open item is minted here — this file
*narrows* existing ones to an executable definition of done.

---

*Made with love ♥ by Helix Development.*
