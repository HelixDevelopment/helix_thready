<!--
  Title           : Helix Thready — Marketing Site Screen Designs (Angular 22 public site)
  Classification  : PUBLIC
  Location        : docs/public/research/mvp/design/screens/marketing/README.md
  Status          : Draft — v0.1
  Revision        : 1 (2026-07-22)
  Author          : Helix Thready documentation swarm (design)
  Related         : ../web/README.md, ../../design-system.md, ../../brand-assets.md,
                    ../../theming.md, ../../opendesign/DESIGN.md, ../../opendesign/tokens.css,
                    ../../library/platform-map.md, ../../../CONVENTIONS.md
-->

# Helix Thready — Marketing Site Screen Designs

| Rev | Date | Author | Change |
|-----|------|--------|--------|
| 1 | 2026-07-22 | swarm (design) | Initial minimal viable marketing design: 3 pages (`landing` / `features` / `download`), self-contained OpenDesign-style HTML matching the `../web/` construction pattern; scope decision recorded honestly; mints `[OPEN: THREADY-DES-MKT-01]` and `[OPEN: THREADY-DES-MKT-02]`; full claims-traceability register (§5) |

## Table of contents

- [1. Scope decision — recorded honestly](#1-scope-decision--recorded-honestly)
- [2. Verified vs. default](#2-verified-vs-default)
- [3. How the artifacts are built](#3-how-the-artifacts-are-built)
- [4. Page catalogue](#4-page-catalogue)
- [5. Claims-traceability register (no bluff)](#5-claims-traceability-register-no-bluff)
- [6. Accessibility & theming contract](#6-accessibility--theming-contract)
- [7. Open items](#7-open-items)

## 1. Scope decision — recorded honestly

The **Angular 22 marketing site was named but never scoped.** The ground truth names it in
exactly these places:

- [design-system.md](../../design-system.md) rev-2 entry: *"clarified Angular 19 (product) vs 22
  (marketing) per Q17"*, and the §7 note `[Q17]`: *"**Angular 22** (standalone/signals, SSR +
  SSG/prerender, Tailwind v4 on OpenDesign tokens) is for **marketing / public sites**. Both
  consume the same `@vasic-digital/design-system` tokens + `.ds-*` set."*
- [DESIGN.md](../../opendesign/DESIGN.md) §8 per-platform table, row *"Web — Angular 19
  (product) / 22 (marketing)"*.
- [theming.md](../../theming.md) §2: the pre-paint head script exists precisely *"to avoid a flash
  of the wrong theme on SSR/prerender (**Angular 22 marketing** + Tauri)"*.
- [brand-assets.md](../../brand-assets.md) §3/§10: the wordmark/full lockup is reserved for
  *"headers/**marketing** only"*.

But **no marketing screen was ever designed**: [wireframes.md](../../wireframes.md) and
[ux-flows.md](../../ux-flows.md) cover only the product portal, and `screens/` contained only
`web/`, `desktop/`, `mobile/`, `tui/`. The design-coverage audit therefore found **zero design
artifacts** for the marketing app.

**Decision taken here `[DEFAULT — adjustable]`:** this directory establishes the **minimal viable
marketing design** — three pages (home, features, download) that a public site cannot exist
without — using only claims traceable to existing design/product docs. It deliberately does
**not** invent pricing pages, blog, docs portal, legal pages, SEO metadata or analytics: those are
minted as open items below rather than papered over `[CONVENTIONS §7 — no bluff]`:

- `[OPEN: THREADY-DES-MKT-01]` — **final marketing IA / copy sign-off** (page set, information
  architecture, all headlines/body copy, and whether any self-host claim is ever made).
- `[OPEN: THREADY-DES-MKT-02]` — **SEO / analytics / legal pages scope** (meta/OG/sitemap,
  analytics or its deliberate absence, privacy policy, terms, imprint, store-listing legal
  footprint).

**Brand scope note.** The marketing site always renders the **system-default Thready / Helix
Development brand — it is never white-labeled**. Per-Account white-labeling is a product-portal
concern: overrides are set per Account by the Root Admin and resolve for signed-in Account
contexts ([theming.md](../../theming.md) §1/§3); a public page has no Account context. The locked
attribution footer renders here exactly as everywhere else.

## 2. Verified vs. default

| Class | Items |
|-------|-------|
| **[VERIFIED]** (carried verbatim, never invented) | Every color/typography/spacing/radius/motion token (from [`tokens.css`](../../opendesign/tokens.css) / design-system.md §3.1–3.2 — no new hex anywhere); the tagline **"read your threads, smarter"** (brand-assets.md §8.1, login/splash); logo-usage rules (full lockup = headers/marketing only; launcher icon letter-free; clear-space ≥ ⅛ box — brand-assets.md §3/§10); the locked footer *"Made with ♥ by Helix Development"* with heart accessible-name "love" (brand-assets.md §8); accent AA ratios (6.03:1 light / 13.56:1 dark); the three dark-mode mechanisms (theming.md §2); every platform status in the availability matrix (design-system.md §7 + platform-map.md §2/§5); every product-capability claim (register in §5) |
| **[DEFAULT — adjustable]** (proposed by this directory, pending `THREADY-DES-MKT-01`) | The three-page set itself; all marketing copy (headlines, card text, section order); the screenshot-slot placeholder treatment (links to live artifacts until Docs Chain captures exist); the `go install` command shape on `download.html`; the "planned" store-channel chips; the decision to render store badges as neutral text chips until listings are real |

## 3. How the artifacts are built

Identical construction to the [web portal set](../web/README.md) — one `.html` per page, fully
**self-contained** (no CDNs, no external CSS/fonts; system fallbacks for Space Grotesk / Hanken
Grotesk / JetBrains Mono):

- **Tokens inlined, verbatim** from design-system.md §3.1 (core) + §3.2 (thready theme);
  [`../../opendesign/tokens.css`](../../opendesign/tokens.css) is a **reference, not a
  dependency**.
- **Light + dark, three sanctioned mechanisms** (theming.md §2): `@media (prefers-color-scheme:
  dark)`, `:root[data-theme="dark"]`, persisted to `localStorage['thready-theme']`, with the
  pre-paint head script and a visible **Light/Dark/System toggle** (`aria-pressed`) on every page.
- **One brand-gradient focal element per screen, at most** (DESIGN.md §7/§9): the `landing.html`
  hero lockup is the only gradient focal element in the whole set; the small header spiral is a
  permitted brand *mark*; `features.html` and `download.html` add no gradient focal element
  (launcher icons on `download.html` are the brand assets themselves).
- **Logo rules respected**: mark + wordmark lockup ("logo-full" composition) appears only in
  headers/hero — marketing surfaces, exactly where brand-assets.md §10 permits it; the launcher
  icons on `download.html` are referenced **relatively** from
  [`../../assets/`](../../assets/) (`launcher-icon-light.svg` / `-dark.svg` / `-mono.svg`) and
  swap with the theme per brand-assets.md §4.
- **Locked footer** on every page, with the "attribution persists under any white-label" note.
- **Provenance comments** at the top of every file; each page ends with a *"Page states — design
  contract"* strip (states, focal-element accounting, copy contract, open items).
- **Marketing copy contract**: warm, human, understated; calm reading-oriented microcopy; **no
  exclamation marks** (DESIGN.md §1 voice & tone). Honesty banners (`.honesty`) render product
  status plainly — scaffolds are "in development", never shipping.

## 4. Page catalogue

| File | Purpose | Contents | Honesty devices |
|------|---------|----------|-----------------|
| [`landing.html`](./landing.html) | Home | Hero lockup (mark + wordmark + tagline "read your threads, smarter") — **the** gradient focal element; 4 feature cards (reading / pipeline / search / surfaces), each linking the real screen artifact; 4 screenshot **slots** (dashboard, search, post-detail, TUI) that link to the living designs pending Docs Chain captures; get-it CTA row (Web / Desktop / Android / iOS / HarmonyOS / Aurora / TUI) with status badges; locked footer | MVP status banner; "in development" badges; slot placeholders never fake screenshots |
| [`features.html`](./features.html) | Deeper feature grid | 8 in-depth cards (channels+backfill, threads, direct/derived tags, skills/recipes, research docs, assets, live events/CLI, teams/RBAC/white-label); **per-platform availability matrix** (8 rows, statuses verbatim from design-system.md §7 / platform-map.md); AI pipeline explainer (Ingest → Process → Skills → Search, with the real precedence line and event names); "How it is run" (no CDNs, operated infra, backups) | Gap tags inline (`[GAP: 5.1/4.1/2.6/8.x]`); honesty banner under the matrix; the self-host claim explicitly **not** made — routed to `[OPEN: THREADY-DES-MKT-01]` |
| [`download.html`](./download.html) | Get-the-app | 7 platform cards with real launcher-icon SVG variants (theme-swapped, relative refs); per-card install-channel chips (real vs **planned**); desktop (Tauri 2, per-OS format status) and TUI (`go install` / binary, `[DEFAULT — adjustable]`) rows; links into `../desktop/`, `../mobile/`, `../tui/` designs | Top honesty banner ("no store listing is live, no installer published"); neutral text chips instead of trademarked store badges; every scaffold labelled with its gap ID |

## 5. Claims-traceability register (no bluff)

Every product claim made on the three pages, with its source. Marketing artifacts are **not**
exempt from the documentation bar `[CONVENTIONS §7]`.

| # | Claim (as marketed) | Source |
|---|--------------------|--------|
| 1 | Tagline "read your threads, smarter" | [brand-assets.md](../../brand-assets.md) §8.1 (login/splash row); [DESIGN.md](../../opendesign/DESIGN.md) §1 voice & tone `[VERIFIED]` |
| 2 | Channels via a 5-step add-wizard with live resolve preview and visible backfill progress | [../web/README.md](../web/README.md) §3 rows `channels.html`, `thread-explorer.html` (backfilling 54/128); wireframes §3.4 |
| 3 | Telegram sessions designed-in; **Max adapter in development** (never claimed working) | [../web/README.md](../web/README.md) rows `settings.html` (Telegram ✓ / Max sign-in) and `channels.html` (`Max: dev-notes` ⚠ auth) — `[GAP: 5.1]` |
| 4 | Threads keep organic reply chains; system replies separated and excluded from counts | [../web/README.md](../web/README.md) row `thread-explorer.html` |
| 5 | Direct vs derived hashtags; "(derived)" is visible and in the accessible name, never color-only | [../web/README.md](../web/README.md) rows `post-detail.html`, `dashboard.html`; [platform-map.md](../../library/platform-map.md) §4 hashtag-chip note |
| 6 | Pipeline classify → download → convert → analyze → research → reply; precedence `download > convert > analyze > research > reply`; idempotent retry ("nothing is processed twice") | [../web/README.md](../web/README.md) row `post-detail.html`; `../web/dashboard.html` precedence caption ("retry is idempotent (single-claim — never double-processes)") |
| 7 | Live progress over WebSocket/SSE, no polling; event names `post.received · processing.progress · processing.completed · processing.failed · channel.added`; CLI `thready events tail` NDJSON | [../web/README.md](../web/README.md) rows `dashboard.html` ("live WS/SSE"), `events-monitor.html` |
| 8 | Search: semantic / keyword / hybrid; scope posts/docs/assets; scored results; 214 ms measured vs < 500 ms target | [../web/README.md](../web/README.md) row `search.html` |
| 9 | Degraded-embedder honesty: HashEmbedder fallback **hides scores** instead of inventing them | [../web/README.md](../web/README.md) row `search.html` ("degraded HashEmbedder, scores hidden") — `[GAP: 2.1]` |
| 10 | Skills/recipes: atomic → composite; recipes Research v3 / Movies v5 / Notes fallback; explicit run order, multi-pass, test-on-sample; **dispatch engine is BUILD-NEW** | [../web/README.md](../web/README.md) row `skills-manager.html` — `[GAP: 4.1]` |
| 11 | Research docs carry sources + pass provenance, link the source post, export .md / Docs-Chain PDF; regeneration marks the old doc stale | [../web/README.md](../web/README.md) row `research-viewer.html` |
| 12 | Assets keep sha256, renditions (…-web), linked posts, Range/HLS streaming, re-download for broken links, sensitive-lock; **OCR tracked as a gap** | [../web/README.md](../web/README.md) row `assets-browser.html` — `[GAP: 2.6]` |
| 13 | Three-tier RBAC (Root / Account Admin / Standard) + append-only audit log | [../web/README.md](../web/README.md) row `accounts-admin.html` |
| 14 | Per-Account white-label (colors/logo/slogan) behind a server-enforced WCAG-AA gate (422 with measured ratio + passing suggestion); Helix attribution footer locked | [theming.md](../../theming.md) §3/§10; [../web/README.md](../web/README.md) row `settings.html` |
| 15 | Web portal is the primary surface; web/CSS + Angular design-system layer is production-usable; **web + CLI ship first** | [design-system.md](../../design-system.md) §7 web row `[OPERATOR]` — `[GAP: 8.1]` |
| 16 | Desktop = Tauri 2 wrapping the same Angular UI, no separate UI work; installers **not published** (icon formats .icns/.ico/hicolor are prepared export targets) | [design-system.md](../../design-system.md) §7; [brand-assets.md](../../brand-assets.md) §5 desktop table |
| 17 | Android/iOS = KMP/Compose, **in development**: `UI-Components-KMP` is a utilities-only scaffold, no widgets/CI/publish, foreign-branded palette | [platform-map.md](../../library/platform-map.md) §2 — `[GAP: 8.4]`; iOS host open: `[OPEN: THREADY-DES-LIB-02]` |
| 18 | HarmonyOS (ArkTS) / Aurora (Qt) via `helix_shims`, **in development**; `helix_design` verified as empty scaffold; layered HarmonyOS icon JSON prepared; Aurora density buckets unverified | [platform-map.md](../../library/platform-map.md) §2; [brand-assets.md](../../brand-assets.md) §5/§5.1 — `[GAP: 8.2/8.3/8.5]`, `[OPEN: THREADY-DES-05]` |
| 19 | TUI = Go, Bubble Tea + Lipgloss, **pattern verified in-house**; styled from the same tokens | [platform-map.md](../../library/platform-map.md) §2 (local `helix_track` clone read); [design-system.md](../../design-system.md) §7 |
| 20 | PWA install: manifest + maskable icons prepared | [brand-assets.md](../../brand-assets.md) §5/§5.1 (`manifest.webmanifest` + `<head>` block) |
| 21 | Android adaptive icon + Android 13+ monochrome layer; iOS single-size 1024 + iOS 18 dark/tinted variants; Play Store 512 icon | [brand-assets.md](../../brand-assets.md) §5/§5.1 |
| 22 | No third-party CDNs; fonts self-hosted (CSP/offline posture) | [design-system.md](../../design-system.md) §4; [DESIGN.md](../../opendesign/DESIGN.md) §3/§7 |
| 23 | Operated infrastructure: single Hetzner dedicated host, three fully-separated envs (dev/sta/prod), rootless Podman; secrets runtime-load-only, never logged | [../../../deployment/index.md](../../../deployment/index.md) §1 `[OPERATOR]` |
| 24 | Backups: daily full + hourly DB incrementals; RPO ≈ 1 h, RTO ≈ 4 h, documented restore runbook | [../../../deployment/index.md](../../../deployment/index.md) §1 (Q41/Q45) |
| 25 | Search backed by a pgvector semantic index | [../../../deployment/index.md](../../../deployment/index.md) §2 (database area: "PostgreSQL + pgvector schema"); service inventory names (Herald, Processing Engine, Semantic Search, Event Bus) ibid. |
| 26 | **Negative claim, deliberately made**: no store listing live, no installer published, self-host offering not documented → not claimed | This directory's scope decision (§1); absence verified against the deployment + design ground truth; routed to `[OPEN: THREADY-DES-MKT-01/-02]` |

## 6. Accessibility & theming contract

Same bar as the [web set](../web/README.md) §5:

- WCAG 2.2 AA with the shipped tokens; **brand green is never body text** — text/interactive
  emphasis uses `--accent` (6.03:1 light / 13.56:1 dark, both measured); `--brand`/`--brand-2`
  appear only in the hero lockup, header mark and launcher-icon assets.
- Status badges: "verified/available" states tint from `--success`; "in development" is a neutral
  outlined chip — semantic colors are state, never decoration; `--danger` is reserved and unused
  on these pages (no destructive actions).
- Skip-to-content link, `:focus-visible` via `--focus-ring` everywhere, `aria-pressed` theme
  toggle, native `<select>` language picker (en / ru / sr-Cyrl), `aria-current="page"` nav.
- Reduced motion disables all transitions; wide tables (availability matrix) scroll inside their
  own container — the page never scrolls horizontally.
- Both modes verified by construction: the identical token blocks and mechanisms as the 13-screen
  web set, including the pre-paint script that exists for this very surface (theming.md §2).

## 7. Open items

- `[OPEN: THREADY-DES-MKT-01]` **Final marketing IA / copy sign-off** — page set, information
  architecture and every headline/body string on the three pages are `[DEFAULT — adjustable]`
  until the operator signs them off; explicitly includes the decision on whether any
  **self-host** claim is ever made (not documented today, therefore not claimed anywhere).
- `[OPEN: THREADY-DES-MKT-02]` **SEO / analytics / legal pages scope** — meta/OG/sitemap/robots
  strategy for the SSG/prerendered Angular 22 site, analytics (or its deliberate absence),
  privacy policy, terms, imprint, and the legal footprint store listings will require. None of
  these pages are designed yet; nothing here pretends they are.
- Inherited, referenced in-page where relevant: `[GAP: 2.1]` HashEmbedder, `[GAP: 2.6]` OCR,
  `[GAP: 4.1]` dispatch engine, `[GAP: 5.1]` Max adapter, `[GAP: 8.1–8.6]` platform packages,
  `[OPEN: THREADY-DES-03]` heart color, `[OPEN: THREADY-DES-05]` Aurora buckets,
  `[OPEN: THREADY-DES-LIB-02]` iOS host path.
- Screenshot slots on `landing.html` await Docs Chain captures of the web/TUI artifacts
  (§11.4.65); until then they link to the living designs and are labelled as slots.

---

*Made with love ♥ by Helix Development.*
