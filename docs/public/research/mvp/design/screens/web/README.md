<!--
  Title           : Helix Thready — Web Portal Screen Designs (OpenDesign-style HTML artifacts)
  Classification  : PUBLIC
  Location        : docs/public/research/mvp/design/screens/web/README.md
  Status          : Draft — v0.1
  Revision        : 1 (2026-07-22)
  Author          : Helix Thready documentation swarm (design)
  Related         : ../../design-system.md, ../../wireframes.md, ../../ux-flows.md,
                    ../../component-library.md, ../../theming.md, ../../../CONVENTIONS.md
-->

# Helix Thready — Web Portal Screen Designs

| Rev | Date | Author | Change |
|-----|------|--------|--------|
| 1 | 2026-07-22 | swarm (design) | Initial full set: 13 screens + interactive prototype shell (`index.html`), each a self-contained OpenDesign-style HTML artifact with inlined Thready tokens, light+dark, per-screen state contracts, realistic Thready data |

## Table of contents

- [1. What this is](#1-what-this-is)
- [2. How the artifacts are built](#2-how-the-artifacts-are-built)
- [3. Screen catalogue (files · purpose · states)](#3-screen-catalogue-files--purpose--states)
- [4. Interactive prototype shell](#4-interactive-prototype-shell)
- [5. Accessibility & theming contract](#5-accessibility--theming-contract)
- [6. Open items](#6-open-items)

## 1. What this is

The **full Web-portal screen design set** for Thready, rendered as OpenDesign-style **single-page
HTML artifacts** — one `.html` per screen, each fully **self-contained** (no CDNs, no external CSS,
no external fonts; system font fallbacks for Space Grotesk / Hanken Grotesk / JetBrains Mono).
These are the high-fidelity realization of the structural contract in
[wireframes.md](../../wireframes.md) §3, driven by the journeys in
[ux-flows.md](../../ux-flows.md), composed from the primitives/composites in
[component-library.md](../../component-library.md), themed by
[design-system.md](../../design-system.md) §3 and [theming.md](../../theming.md).

Open any file directly in a browser, or start from **[`index.html`](./index.html)** — the
interactive prototype shell that walks the ux-flows journeys across all screens.

## 2. How the artifacts are built

- **Tokens inlined, verbatim.** Every artifact embeds the theme-invariant core tokens
  (design-system.md §3.1) and the Thready brand theme (§3.2): base `#B6E376`, accents
  `#446E12` (light) / `#B6E376` (dark) — both AA-measured — teal secondaries `#ABDDC9`/`#B7EBD6`,
  fg `#020817`/`#F8FAFC`, neutrals `#475569`/`#94A3B8`. `../opendesign/tokens.css` is a
  **reference, not a dependency** — the artifacts never load it.
- **Light + dark, three sanctioned mechanisms** (theming.md §2):
  `@media (prefers-color-scheme: dark)` for *system*, `:root[data-theme="dark"]` for the explicit
  choice, persisted to `localStorage['thready-theme']`. Every screen carries a **visible
  Light/Dark/System toggle** (`ds-theme-toggle` contract, `aria-pressed`).
- **Shared chrome.** Every screen has the embedded logo-mark SVG header (same geometry as
  `../../assets/logo-mark.svg`), the global search (`/` shortcut), language picker (en/ru/sr-Cyrl),
  account switcher, and the locked `.ds-footer` slogan **“Made with ♥ by Helix Development”** —
  heart in `--ds-heart` = `--accent` (`#446E12` light / `#B6E376` dark, `[OPEN: THREADY-DES-03]`).
- **Realistic Thready data only** — channels (`#ml-papers`, `#films`, `#nlp`, `Max: dev-notes`),
  posts with direct/indirect hashtags (`#Research`, `#Video ◌ (derived)`, `#ToDownload`),
  processing states, semantic-search scores (0.94/0.91/0.88). No lorem ipsum.
- **Per-screen state contracts.** Each screen ends with a *“Screen states — design contract”*
  strip rendering (or documenting) its loading/skeleton, empty, error and screen-specific states
  per wireframes.md §1.1 and the component-library.md §5b lifecycle.
- **Forms** show blur/submit/server validation with hints, specific error messages, and
  `aria-describedby` wiring; destructive/gated actions demo their `409`/`422` behavior in-page.

## 3. Screen catalogue (files · purpose · states)

| File | Screen (wireframes ref) | Realistic content | States shown/documented |
|------|------------------------|-------------------|-------------------------|
| [`login.html`](./login.html) | Login / MFA (§3.2) | credentials + TOTP step, session policy, Caps-Lock hint | default · validating · error (generic 401, no enumeration) · locked/cooldown · MFA-required (Esc back) · SSO round-trip · **no-account-membership empty** |
| [`dashboard.html`](./dashboard.html) | Dashboard (§3.3) | stat row 128/1,204/17/42.1 TB · live WS/SSE activity · processing queue (63%/31%/failed→retry) · recent-threads table | skeleton · empty (add first channel) · error · live-degraded (reconnect) · failed-job retry (409-guarded) |
| [`channels.html`](./channels.html) | Channels list + **Add-Channel wizard modal** (§3.4) | 5 channels incl. `Max: dev-notes` ⚠ auth `[GAP: 5.1]`; 5-step wizard with Resolve preview (“ML Papers · public · 128 msgs”) | skeleton · empty · error · wizard: resolving (aria-busy) / 422 unresolvable / 403 private / auto-type banner (§Q32) · optimistic add (channel.added) · Max auth |
| [`thread-explorer.html`](./thread-explorer.html) | Channel detail / thread list (§3.5) | root + **organic reply chain** (↩12, expandable), system replies separated & excluded, per-post status glyphs ✓⭮◷⚠, indirect-tag badge | skeleton · backfilling (54/128) · empty · error · re-auth needed · paused |
| [`post-detail.html`](./post-detail.html) | Post detail / processing (§3.6) | root + tags (direct ● / indirect ○ “(derived)”) · reply log (3 system status replies) · full pipeline classify→download(63%)→convert→analyze(failed→retry)→research→reply with precedence caption · **skills run** table (additive dispatch) · generated assets (raw / …-web / research.md) · **Reprocess** (confirm → 202; second click → 409 toast) | skeleton · pipeline-live · step-failed + idempotent retry · reprocess-409 · empty replies · error |
| [`search.html`](./search.html) | Search (§3.7) | query “papers about retrieval-augmented generation”; mode semantic/keyword/hybrid; scope posts/docs/assets (≥1 enforced); advanced filters (type/channel/tag/origin); scored results 0.94/0.91/0.88 → post/doc; 214 ms vs <500 ms SLO | idle (recent) · skeleton · empty + clear-filters · timeout nudge · **degraded HashEmbedder** (`[GAP: 2.1]`, scores hidden) · scope-invalid |
| [`assets-browser.html`](./assets-browser.html) | Assets (§3.8) | grid/list toggle; tiles video-web.mp4 / heat-1995.mkv (broken link) / cover.jpg (OCR ✓) / deep-work.pdf (🔒 sensitive) / nocturne-op9-2.flac / research.md; **detail drawer**: renditions, sha256, linked posts, stream (Range/HLS), re-download, sensitive lock | skeleton · empty · error · broken-link → re-download · sensitive/locked · streaming |
| [`skills-manager.html`](./skills-manager.html) | Skills / Recipes (§3.9) | SVG **skill graph** atomic→composite→umbrella; recipes Research v3 (#Research, order research>reply, multi-pass steps), Movies v5, Notes fallback (§Q32); “test on sample post” fixture modal | skeleton · defaults-only empty · test-run preview · error · 409 version conflict — plus the `[GAP: 4.1]` BUILD-NEW dispatch-engine honesty banner |
| [`research-viewer.html`](./research-viewer.html) | Generated research doc (post-detail asset; theming §9) | research.md with TOC, findings, benchmark table, sources & pass provenance (Research v3); actions: open source post, re-run (409-guarded), .md / Docs-Chain PDF export; account-brand note | skeleton · generating · stale (post reprocessed) · empty · error |
| [`accounts-admin.html`](./accounts-admin.html) | Admin — Accounts / Users & roles / Audit (§3.10) | **three-tier RBAC** explainer (Root / Account Admin / Standard); users table root@/alice@/bob@ (invited); per-Account roles; invite modal (email validation, idempotent no-op); append-only audit log | skeleton · invite-pending/resend · idempotent invite · **last-Root-Admin guard** (disabled + tooltip) · 403 forbidden · empty audit |
| [`billing.html`](./billing.html) | Billing & usage (§3.10) | plan **Pro** + metered 1.2M posts · 42 TB · 3,481 research runs; invoices INV-2026-005…007 (Docs-Chain PDFs) | skeleton · past-due · meter-near-cap · empty invoices · error · 403 — monetary figures are placeholders (`[OPEN]` below) |
| [`settings.html`](./settings.html) | Settings — Prefs / **Branding white-label** / Messengers (§3.11) | theme+language prefs, keyboard map; branding editor for Acme (#12A3FF/#0D6EFD, accents #0B5ED7 6.1:1 ✓ / #7DB3FF 7.2:1 ✓) with **working live AA meter** (WCAG math = server `ValidateAccent`), logo uploads, slogan, locked-attribution note, live preview (`data-account` scope), Save gated on AA; Telegram session ✓ / Max sign-in (interactive vs env) | default · dirty guard · previewing · validating/**422 + suggestion #446E12** · success (audit-logged) · sign-in await-code · missing-env 422 |
| [`events-monitor.html`](./events-monitor.html) | Events monitor (Live activity / `thready events tail` mirror) | live table of `post.received · processing.progress · processing.completed · processing.failed · channel.added` with JSON payloads; type/channel filters, pause/tail; CLI NDJSON card | live · reconnecting (backoff) · offline (frozen values) · paused (buffered) · empty-filtered · error (401 re-auth) |
| [`index.html`](./index.html) | **Interactive prototype shell** | ux-flows journeys 1–4 as clickable steppers driving an inline preview of every screen + full gallery | — (shell) |

## 4. Interactive prototype shell

[`index.html`](./index.html) wires the four [ux-flows.md](../../ux-flows.md) journeys to the
artifacts: **1 · Add channel** (login → dashboard → channels wizard → messenger sign-in sub-flow
§2.1 → thread-explorer backfill), **2 · Process post + Reprocess** (dashboard → thread-explorer →
post-detail → events-monitor → research-viewer → assets), **3 · Search** (search → post/doc), and
**4 · Manage account** (accounts-admin → billing → settings branding AA gate). Each step loads the
target screen into an embedded preview with prev/next stepping; the gallery links every artifact.

## 5. Accessibility & theming contract

- WCAG 2.2 AA with the shipped tokens: text on accent uses `--accent-on`; `--brand`/`--brand-2`
  are **decorative only**; `--danger` is never masked by brand color.
- `:focus-visible` shows the 3px `--focus-ring` on every interactive element; skip-to-content link;
  modals are native `<dialog>` (focus trap + `Esc`); toasts use `role="status"`/`role="alert"`;
  progress uses `role="progressbar"` + `aria-valuenow`; indirect chips carry “(derived)” in the
  accessible name (never color-only); reduced-motion disables all transitions/shimmer.
- The AA meter in `settings.html` implements the same sRGB relative-luminance ratio as the server
  gate (theming.md §10) and demos the `422 accent_below_wcag_aa` path with the measured ratio and
  the `#446E12` suggestion.

## 6. Open items

- `[OPEN: THREADY-DES-SCR-01]` **Billing pricing** — plan tiers beyond “Pro”, unit prices, included
  quotas and grace-period policy are not defined in the ground truth; `billing.html` ships the
  layout contract with placeholder amounts (`—`) and a visible note.
- `[OPEN: THREADY-DES-SCR-02]` **Research doc versioning** — diff/history between successive
  research generations after a reprocess is unspecified; `research-viewer.html` shows latest + a
  “stale” state only.
- `[OPEN: THREADY-DES-SCR-03]` **Events retention** — history depth/queryable window for the
  events monitor beyond the live tail is unspecified.
- `[OPEN: THREADY-DES-SCR-04]` **Forgot-password flow** — referenced by wireframes.md §3.2 but not
  specified; `login.html` links it with an inline note, no fake flow is designed.
- Inherited from ground truth (noted in-page where relevant): `[OPEN: THREADY-DES-03]` heart color
  (accent vs love-red — artifacts follow the current default `--ds-heart: var(--accent)`),
  `[OPEN: THREADY-DES-06]` Account-Admin self-branding, `[OPEN: THREADY-DES-10]` final
  endpoint/event names, `[OPEN: THREADY-DES-11]` composite API names; gaps `[GAP: 5.1]` Max
  adapter, `[GAP: 4.1]` dispatch engine, `[GAP: 2.1]` HashEmbedder, `[GAP: 2.6]` OCR,
  `[GAP: 6.5]` MeTube webhook are rendered as honest in-UI states/banners, never as working
  features.

---

*Made with love ♥ by Helix Development.*
