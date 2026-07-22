<!--
  Title           : Helix Thready — Mobile Screen Designs (Catalogue)
  Classification  : PUBLIC
  Location        : docs/public/research/mvp/design/screens/mobile/README.md
  Status          : Draft — v0.1
  Revision        : 2 (2026-07-22)
  Author          : Helix Thready documentation swarm (design · screens)
  Related         : ../../wireframes.md, ../../design-system.md, ../../theming.md,
                    ../../ux-flows.md, ../../../CONVENTIONS.md
-->

# Helix Thready — Mobile Screen Designs

| Rev | Date | Author | Change |
|-----|------|--------|--------|
| 1 | 2026-07-22 | swarm (design · screens) | Initial catalogue: 7 device-framed screen artifacts (390×844), Android/iOS chrome toggle, HarmonyOS/Aurora port notes, light+dark |
| 2 | 2026-07-22 | swarm (design · screens) | +4 artifacts closing the §6 IA gaps: Channels tab root, Add-Channel bottom sheet, Assets grid, full-screen Media viewer; §6 coverage reconciled; minted `THREADY-DES-SCR-MOB-01/-02/-03` |

## 1. What this directory is

Device-framed, **self-contained HTML artifacts** (no CDNs, no external assets — 11 screens) rendering
the Thready mobile screens at **390×844 in a rounded phone bezel**, populated with the exact realistic
fixture data used across the design docs (channels `#ml-papers`/`#films`/`Max: dev-notes`, the
diffusion-paper thread, the download-63% pipeline, `alice@acme.io`/`bob@…` RBAC rows, 128 channels /
1,204 posts / 17 processing / 42.1 TB stats). They are the **mid-fidelity realization** of the
mobile wireframes ([wireframes.md §6](../../wireframes.md#6-mobile-wireframes)); the low-fi
structural contract stays authoritative, and high-fi Figma work remains in
[prototypes.md](../../prototypes.md) `[OPEN: THREADY-DES-09]`.

Every artifact:

- **Inlines the Thready tokens verbatim** from [design-system.md §3.1/§3.2](../../design-system.md#3-token-architecture)
  (`#B6E376` brand, `#446E12`/`#ABDDC9`/`#B7EBD6` accents, `#020817`/`#F8FAFC` fg,
  `#475569`/`#94A3B8` neutrals; Space Grotesk / Hanken Grotesk / JetBrains Mono **with system
  fallbacks only** — fonts are not embedded, matching the no-CDN/self-hosting posture).
- Supports **light + dark** via the exact shipped mechanism: `@media (prefers-color-scheme: dark)`
  default + explicit `:root[data-theme="dark"]` override ([theming.md §2](../../theming.md#2-light--dark-resolution)),
  with a Light/System/Dark control on the page.
- Carries an **Android (Material 3 / Compose) ↔ iOS (SwiftUI) chrome toggle** switching status bar
  (punch-hole vs. Dynamic Island), app-bar/back pattern (← + predictive back vs. `‹ Title` + edge
  swipe), navigation bar (M3 80dp pill-indicator bar vs. 49pt tab bar + home indicator), and
  gesture area.
- Documents **HarmonyOS (ArkTS)** and **Aurora OS (Qt Quick)** as **native ports of the same
  layouts** in a per-screen notes panel — both clients are `helix_shims` **skeletons**
  `[GAP: 8.5]`; the notes never claim they are build-ready.

## 2. Catalogue

| File | Screen | Ground truth | Status |
|------|--------|--------------|--------|
| [home-feed.html](./home-feed.html) | Home tab — live activity + processing status + recent threads | wireframes §6 (Home), §3.3 (data), ux-flows §3 (events) | grounded |
| [channels.html](./channels.html) | Channels tab root — subscribed-channels list, per-channel state chips (healthy / syncing / ⚠ auth / paused), M3 FAB vs. iOS nav-bar add action | wireframes §6 (`T2 → CHLIST`), §3.4 (row contract), §6.2 (gestures) | grounded; unread pills `[OPEN: THREADY-DES-SCR-MOB-01]` |
| [add-channel.html](./add-channel.html) | Add-Channel bottom sheet — source picker, account/sign-in, link + Resolve with hint/error/processing states, step gating | wireframes §6 (`CHLIST → CHADD`), §3.4 (wizard + validation table), §1.3; ux-flows §2/§2.1 | grounded; Max `[GAP: 5.1]`; WhatsApp `[OPEN: THREADY-DES-SCR-MOB-03]` |
| [channel-threads.html](./channel-threads.html) | Channel detail — thread list (complete posts) | wireframes §3.5 + §6, §6.2 gestures | grounded |
| [post-detail.html](./post-detail.html) | Post detail — tags, replies, pipeline, assets, Reprocess | wireframes §3.6 + §6; ux-flows §3/§3.1 | grounded |
| [search.html](./search.html) | Search — semantic/keyword/hybrid, scope, scored results | wireframes §3.7 + §6; ux-flows §4 | grounded |
| [assets-grid.html](./assets-grid.html) | Assets tab root — masonry media grid with type badges, filter chips, empty-state variant | wireframes §6 (`T4 → ASSETS`), §3.8 (library vocabulary), §1.1 (empty), §6.2 | grounded |
| [media-viewer.html](./media-viewer.html) | Full-screen media player / asset viewer — video / image-zoom / audio-waveform / doc variants + pipeline-provenance overlay | wireframes §6 (`POST → PLAYER`, `ASSETS → PLAYER`), §3.8 (stream Range/HLS), §3.6, §6.2 (zoom) | grounded; share/save-to-device `[OPEN: THREADY-DES-SCR-MOB-02]` |
| [notifications.html](./notifications.html) | Notifications centre over Event Bus events | **derived** — events are ground truth (ux-flows §2/§3); the surface is not in the §6 tab map | `[OPEN: THREADY-DES-15]` |
| [account.html](./account.html) | Account — profile, multi-Account switcher, RBAC admin, sign out | wireframes §6 (More) + §3.10; ux-flows §5 | grounded |
| [settings.html](./settings.html) | Settings — theme/language, effective branding, messengers, security | wireframes §6 + §3.11; theming §2/§8 | grounded |

**Coverage vs. the §6 IA (reconciled, Rev 2).** Every node of the wireframes §6 navigation map now has
an artifact: Home → [home-feed](./home-feed.html); Channels → [channels](./channels.html) →
[add-channel](./add-channel.html) sheet / [channel-threads](./channel-threads.html) →
[post-detail](./post-detail.html) → [media-viewer](./media-viewer.html); Search →
[search](./search.html); Assets → [assets-grid](./assets-grid.html) → media-viewer; More →
[account](./account.html) / [settings](./settings.html). Nothing promised by §6 is missing on disk.
The one surface **beyond** §6 remains [notifications.html](./notifications.html), a derived proposal
`[OPEN: THREADY-DES-15]`.

## 3. Conventions the artifacts follow

- **Never fake:** every stub/scaffold dependency is flagged on-screen or in the notes panel —
  Max adapter `[GAP: 5.1]`, dispatch engine `[GAP: 4.1]`, MeTube webhook `[GAP: 6.5]`, OCR
  `[GAP: 2.6]`, HashEmbedder `[GAP: 2.1]` (search shows the mandated degraded banner),
  `Security-KMP` secure-storage stub `[GAP: 7.3]` and `UI-Components-KMP` `[GAP: 8.4]` (both
  **mobile release gates**, wireframes §6.2).
- **Interaction states** follow the uniform legend of wireframes §1.1 (default / loading /
  skeleton / empty / error / disabled / success); artifacts render the *default* state and
  annotate the others inline where a screen has a notable one (offline freeze, degraded search,
  409 “already running”).
- **Errors are `--danger`, never `--accent`;** indirect (AI-derived) tags render dashed vs. solid
  direct hashtags; the Helix Development attribution is locked where it appears.
- Bezel/status-bar cosmetics (clock values `14:02`/`9:41`, battery glyphs, island/punch-hole
  dimensions) are illustrative chrome, `[DEFAULT — adjustable]` — not token-governed.

## 4. Open items

- `[OPEN: THREADY-DES-15]` — **Notifications centre**: confirm whether mobile gets a dedicated
  notifications surface (and push delivery preferences) or stays with the Home live feed only;
  the wireframes §6 IA does not include one. Until decided, [notifications.html](./notifications.html)
  is a derived proposal over the verified event contract.
- `[OPEN: THREADY-DES-04]` — Cyrillic subsets (ru / sr-Cyrl) of the three faces; affects every
  screen’s text rendering.
- `[OPEN: THREADY-DES-06]` — whether Account Admins may edit their own branding; settings.html
  keeps branding read-only on mobile until resolved.
- `[OPEN: THREADY-DES-09]` — high-fidelity Figma frames refine these artifacts in
  [prototypes.md](../../prototypes.md).
- `[OPEN: THREADY-DES-SCR-MOB-01]` — **unread "N new" pills** on the Channels list rows
  ([channels.html](./channels.html)) are not in the §6/§3.4 ground truth; they are a
  `[DEFAULT — adjustable]` proposal derived from `post.received` events since last visit. Confirm
  whether mobile shows unread counts at all, and the reset rule (on open vs. on scroll-past).
- `[OPEN: THREADY-DES-SCR-MOB-02]` — **media viewer ⇪ Share / ⭳ Save-to-device**
  ([media-viewer.html](./media-viewer.html)): no ground-truth export path exists — §3.8's
  "re-download" is a server-side re-fetch for broken links, and the desktop precedent
  (`[OPEN: THREADY-DES-16]`, [desktop README §3.4](../desktop/README.md)) already left local-media
  paths undecided. Both actions are specified disabled until product resolves the export path.
- `[OPEN: THREADY-DES-SCR-MOB-03]` — **WhatsApp as a third Add-Channel source**
  ([add-channel.html](./add-channel.html)): requested, but absent from the verified source set
  (wireframes §3.4: Telegram/Max; ux-flows §2 env contract `TG_*`/`MAX_*`). Rendered as a disabled
  tile until the decision matrix adds an adapter; never shown as selectable.

---

*Made with love ♥ by Helix Development.*
