<!--
  Title           : Helix Thready — Motion Package (Lottie catalogue)
  Classification  : PUBLIC
  Location        : docs/public/research/mvp/design/motion/README.md
  Status          : Active — v1.1
  Revision        : 2 (2026-07-22)
  Author          : Helix Thready documentation swarm (design/motion)
  Related         : ./motion.md, ./preview.html, ./motion-manifest.json,
                    ../prototypes.md, ../design-system.md, ../ux-flows.md, ../../CONVENTIONS.md
-->

# Helix Thready — Motion Package (Lottie catalogue)

| Rev | Date | Author | Change |
|-----|------|--------|--------|
| 1 | 2026-07-22 | swarm (design/motion) | Initial package: six hand-authored Lottie assets, motion spec, self-contained preview, manifest, validation record |
| 2 | 2026-07-22 | swarm (design · motion) | Static `reducedMotionFallback` SVGs delivered (asset half of `THREADY-MOT-03` closed — see motion.md §8 for the narrowed remainder); manifest v2 names them per animation; preview gains a static-fallback / reduced-motion demonstration toggle |

This directory is the concrete **motion deliverable** of the design area — the
"lots of Lottie animations" the request mandates
([prototypes.md §6](../prototypes.md#6-motion--transition-spec)), realized as
**hand-authored vector Lottie** (v5.x schema, shape layers only, no rasters, no
design-tool export involved). It partially delivers the Lottie half of
`[OPEN: THREADY-DES-02]`; see [motion.md §8](./motion.md#8-gaps--open-items) for what
remains open.

**Upstream:** [design-system.md §5](../design-system.md#5-spacing-radius-elevation-motion)
(motion tokens, reduced-motion rule), [prototypes.md §6](../prototypes.md#6-motion--transition-spec)
(choreography spec), [ux-flows.md](../ux-flows.md) (the journeys each asset binds to).
**Downstream:** app motion runtimes (`lottie-web` / `lottie-compose` / `lottie-ios`),
the visual-regression bank (`[GAP: 9.3]`).

## Inventory

| File | What it is | Frames @ fps | Loop | Plays on | Validation |
|------|------------|--------------|------|----------|------------|
| [`helix-spinner.json`](./helix-spinner.json) | The double-helix mark weaving/rotating (two phase-animated sine strands, lime + teal) — loading | 120 @ 60 (2 s) | yes, seam-exact | app boot, route loads, resolve waits | parse ✅ · structure ✅ · lottie-web render ✅ |
| [`success-check.json`](./success-check.json) | Lime disc pop + ink check drawn by trim-path + accent ring ping | 90 @ 60 (1.5 s) | one-shot | `201` created, `200` saved, `processing.completed` | parse ✅ · structure ✅ · lottie-web render ✅ |
| [`error-cross.json`](./error-cross.json) | Danger disc pop + snow cross drawn + shake + ring ping (`--danger`, never brand) | 90 @ 60 (1.5 s) | one-shot | `processing.failed`, `422`/`403`, AA-fail | parse ✅ · structure ✅ · lottie-web render ✅ |
| [`processing-pulse.json`](./processing-pulse.json) | Breathing lime core + two staggered teal ripple rings — indeterminate work | 120 @ 60 (2 s) | yes, seam-exact | pipeline step running/queued, reprocess in flight | parse ✅ · structure ✅ · lottie-web render ✅ |
| [`thread-sync.json`](./thread-sync.json) | Messenger node ⇄ Thready node with dots shuttling along the thread — messenger sync | 120 @ 60 (2 s) | yes, seam-exact | channel backfill / live sync, session established | parse ✅ · structure ✅ · lottie-web render ✅ |
| [`transition-fade-slide.json`](./transition-fade-slide.json) | Shared-axis X fade+slide between two cards at real 300 ms timing, with segment markers | 90 @ 60 (1.5 s) | one-shot | route/page + wizard-step transitions (reference rendering) | parse ✅ · structure ✅ · lottie-web render ✅ |
| [`spiral-static.svg`](./spiral-static.svg) | Reduced-motion fallback for `helix-spinner` — the exact frame-15 poster state (interpolated strand paths + eased strand opacities from the Lottie keyframes) | static | — | `prefers-reduced-motion` swap | xmllint ✅ · geometry derived from Lottie keyframes ✅ |
| [`check.svg`](./check.svg) | Reduced-motion fallback for `success-check` — final frame 89 (disc at rest, check fully drawn; invisible ring ping omitted) | static | — | `prefers-reduced-motion` swap | xmllint ✅ · geometry derived from Lottie keyframes ✅ |
| [`cross-static.svg`](./cross-static.svg) | Reduced-motion fallback for `error-cross` — final frame 89 (disc at rest after shake, cross fully drawn; invisible ring ping omitted) | static | — | `prefers-reduced-motion` swap | xmllint ✅ · geometry derived from Lottie keyframes ✅ |
| [`pulse-static.svg`](./pulse-static.svg) | Reduced-motion fallback for `processing-pulse` — the exact frame-30 poster state (core at 114 %, both rings at their interpolated scale/opacity) | static | — | `prefers-reduced-motion` swap | xmllint ✅ · geometry derived from Lottie keyframes ✅ |
| [`sync-static.svg`](./sync-static.svg) | Reduced-motion fallback for `thread-sync` — the exact frame-30 poster state (nodes + thread, carrier dot at its eased x 365.78) | static | — | `prefers-reduced-motion` swap | xmllint ✅ · geometry derived from Lottie keyframes ✅ |
| [`motion.md`](./motion.md) | The motion spec: tokens, interaction map, reduced-motion contract, runtime integration, honest validation record | — | — | — | — |
| [`motion-manifest.json`](./motion-manifest.json) | Runtime manifest v2 (ids, loop, poster frames, reduced-motion mode + per-animation `reducedMotionFallback` SVG, segment markers) — concrete form of the prototypes §6 excerpt | — | — | — | parse ✅ |
| [`preview.html`](./preview.html) | Self-contained preview: vendored **lottie-web 5.13.0** (MIT) inline + the six JSONs embedded verbatim; theme toggle, per-asset scrub/replay, reduced-motion poster behavior, and a "Show static fallbacks" toggle (auto-engaged under `prefers-reduced-motion`) with the five fallback SVGs embedded inline. No CDN, zero network requests. | — | — | — | jsdom smoke test ✅ (re-run after fallback toggle) · Chromium light/dark/reduced-motion screenshots ✅ (rev 1 page) |
| [`diagrams/motion-asset-map.mmd`](./diagrams/motion-asset-map.mmd) | Mermaid source of the journey→asset map in motion.md §4 | — | — | — | — |

**Validation meaning** (full record: [motion.md §7](./motion.md#7-validation-record-honest)):
*parse* = `python3 json.load`; *structure* = required top-level keys (`v fr ip op w h layers`),
5.x schema, `ty: 4` shape layers only with non-empty `shapes`, monotonic keyframes, colors
in [0, 1], empty `assets`, exact loop seams; *lottie-web render* = loaded and frame-seeked in
lottie-web 5.13.0 (SVG renderer, headless under jsdom) producing real geometry, with
animation-progression assertions (trim draws, transforms move, shapes morph). All six were
additionally rendered in **headless Chromium** (real Blink/Skia compositor) as a multi-frame
light+dark contact sheet and reviewed. **Not** verified: physical-display pixels,
`lottie-compose`/`lottie-ios` runtimes — tracked `[OPEN: THREADY-MOT-01]`.

## Reduced-motion fallbacks (static SVGs)

Each playable animation names its static fallback in
[`motion-manifest.json`](./motion-manifest.json) (`reducedMotionFallback`); the SVG
renders the **exact state the reduced-motion contract freezes on** — the poster frame
for the loopers, the final frame for the one-shots — with geometry taken from the
Lottie keyframes (interpolated where the frame falls between keyframes; see each SVG's
header comment for the derivation). `transition-fade-slide` deliberately has
`reducedMotionFallback: null` — its reduced-motion behavior is an `instant-cut`, no
artwork exists on purpose. The `empty-channels.svg` fallback named in the prototypes §6
excerpt belongs to the illustration-grade set that is still open under
`[OPEN: THREADY-DES-02]` and is therefore intentionally absent.

**Coloring `[DEFAULT — adjustable]`:** fills/strokes are written as
`style="fill: var(--brand, #B6E376)"` — the literal fallback values are the Lottie's
own baked Thready colors (dark-surface tuned, [motion.md §6](./motion.md#6-runtime-integration)),
so a standalone `<img>` load matches the animations exactly, while inlining the SVG
into a token-bearing page themes it via `--brand`/`--brand-2`/`--brand-ink`/`--danger`
— the SVG analogue of the documented load-time remap. `currentColor` was not usable:
these are multi-color assets. The snow cross strokes in `cross-static.svg` stay
literal `#F8FAFC` by design (must remain light-on-danger in both themes; no on-danger
token exists).

## How to preview

Open [`preview.html`](./preview.html) in any browser — it is fully self-contained
(player vendored inline; works offline). Or load any `.json` into a standard Lottie
player. With `prefers-reduced-motion: reduce` active, the page swaps every slot to its
static fallback SVG (the production behavior); the header's "Show static fallbacks" /
"Show animations" toggle demonstrates the swap in either mode, per the
[motion.md §5](./motion.md#5-reduced-motion-contract-prefers-reduced-motion) contract.

## Color provenance

All colors are the Thready theme tokens
([design-system.md §3.2](../design-system.md#32-the-thready-brand-theme)) in Lottie 0–1 RGBA form:

| Token | Hex | Lottie array | Used in |
|-------|-----|--------------|---------|
| `--brand` (lime) | `#B6E376` | `[0.714, 0.890, 0.463]` | spinner strand, check disc, pulse core, sync node/dot, incoming card |
| `--accent` light | `#446E12` | `[0.267, 0.431, 0.071]` | success ring ping; light-theme remap target ([motion.md §6](./motion.md#6-runtime-integration)) |
| `--brand-2` light (teal) | `#ABDDC9` | `[0.671, 0.867, 0.788]` | spinner strand, ripple ring A, sync node/thread, outgoing card |
| `--brand-2` dark (teal) | `#B7EBD6` | `[0.718, 0.922, 0.839]` | ripple ring B, sync return dot |
| `--brand-ink` | `#0A0F04` | `[0.039, 0.059, 0.016]` | check stroke on the lime disc (13.15:1 — the documented pairing, design-system §3.2) |
| ink (`--fg` light) | `#020817` | `[0.008, 0.031, 0.090]` | card text bars (at 30 % fill opacity) |
| snow (`--fg` dark) | `#F8FAFC` | `[0.973, 0.980, 0.988]` | cross strokes |
| `--danger` light | `#DC2626` | `[0.863, 0.149, 0.149]` | error disc + ping — semantic error is never brand-colored ([design-system §6](../design-system.md#6-accessibility-contract)) |

The set is tuned for dark surfaces (lime **is** the dark accent); light-surface and
white-label retinting is a documented load-time remap, [motion.md §6](./motion.md#6-runtime-integration).

## How these were authored

Hand-authored as Lottie JSON (no After Effects, no Figma plugin, no downloads) via a
deterministic generator script that computes the geometry — sine-through-bezier strand
keyframes with analytic tangents for the spinner, visibility-aware seam-exact keyframes
for the loopers — and every eased keyframe carries the shipped token curve
`--ease-standard cubic-bezier(0.2, 0, 0, 1)` `[VERIFIED — design-system §5]`. The
validator that gated them (and caught two real seam defects during authoring) is part
of the record in [motion.md §7](./motion.md#7-validation-record-honest).

---

*Made with love ♥ by Helix Development.*
