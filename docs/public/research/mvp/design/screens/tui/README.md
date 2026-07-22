<!--
  Title           : Helix Thready — TUI Screen Designs (Catalogue)
  Classification  : PUBLIC
  Location        : docs/public/research/mvp/design/screens/tui/README.md
  Status          : Draft — v0.2
  Revision        : 2 (2026-07-22)
  Author          : Helix Thready documentation swarm (design · screens)
  Related         : ./tui-screens.html, ./lipgloss-theme.md, ../../wireframes.md (§5),
                    ../../design-system.md (§7), ../../../CONVENTIONS.md
-->

# Helix Thready — TUI Screen Designs

| Rev | Date | Author | Change |
|-----|------|--------|--------|
| 1 | 2026-07-22 | swarm (design · screens) | Initial catalogue: styled terminal mockups (5 screens) + Lipgloss theme mapping |
| 2 | 2026-07-22 | swarm (design · screens) | Gap closure vs. wireframes §5: +5 screens (Login, Channel detail, Add-Channel form, Assets, Skills) + in-artifact screen index; minted `THREADY-DES-TUI-01/-02` |

## 1. What this directory is

The Thready TUI is **Bubble Tea + Cobra + Lipgloss**, structured after the **verified in-house
pattern** `helix_track/llms_verifier/llm-verifier/tui` (app model → screen models →
Lipgloss-styled views + notifications/live pane) `[VERIFIED — wireframes.md §5]`. This directory
holds the styled visual realization of the TUI wireframes:

- **[tui-screens.html](./tui-screens.html)** — a self-contained artifact (no CDNs) rendering ten
  80-column terminal mockups in monospace with an **ANSI-16 palette mapped from the brand
  tokens**: exact box-drawing alignment, Bubble Tea composition labels
  (**list + viewport + statusbar** per screen, plus textinput/choice/spinner/progress where a
  screen is a form), the verified keybinding rail (wireframes §5.1), and realistic Thready data
  (#ml-papers / #films / Max auth-warning, download 63 %, scores 0.94/0.91/0.88). The full §5
  state machine is now covered (Login through Skills); an in-artifact index links every screen.
  Page chrome supports light + dark via `[data-theme="dark"]`; the terminal surface stays on the
  dark palette — "the TUI defaults to the terminal's dark surface" (design-system §7).
- **[lipgloss-theme.md](./lipgloss-theme.md)** — the normative brand-token → `lipgloss.Color`
  mapping (truecolor → ANSI-256 → ANSI-16), role styles re-binding the verified llm-verifier
  style roles to tokens, `CompleteColor`/`AdaptiveColor` options — every value marked
  **VERIFIED** vs **ASSUMED**.

## 2. Catalogue

| Screen (in tui-screens.html) | Bubble Tea composition | Ground truth | Status |
|------------------------------|------------------------|--------------|--------|
| 1 · Dashboard | key-hint rail + live viewport (WS/SSE) + threads list + statusbar | wireframes §5 layout, §5.1 keys | grounded |
| 2 · Channels | list (inverse-accent selection) + statusbar | wireframes §3.4 data re-laid; §5 nav state machine | grounded |
| 3 · Thread view (Post detail) | root header + pipeline/replies viewports + statusbar | wireframes §5.3; ux-flows §3.1 (409 idempotent retry) | grounded |
| 4 · Search | textinput + mode/scope + scored list + statusbar | wireframes §5.3; ux-flows §4 (degraded-embedder banner) | grounded |
| 5 · Processing queue | job list with progress cells + statusbar | **derived** — Dashboard queue pane (§3.3) + processing-event contract; the §5 wireframes fold the queue into the Dashboard | `[DEFAULT — adjustable]` |
| 6 · Login | textinput ×2 (endpoint/token) + auth-mode choice + spinner + statusbar | wireframes §5 (`Login → Dashboard`); token/config from CLI §4.1 (`THREADY_ENDPOINT`/`THREADY_TOKEN`, keychain-first); pending/failure machine mirrors ux-flows §2.1 | grounded; device-code option `[OPEN: THREADY-DES-TUI-02]` |
| 7 · Channel detail | list (threads, ● unread) + viewport (scroll rule) + statusbar | wireframes §5.3 + §3.5 (thread list re-laid); header state chip per §3.4; Max ⚠ auth-warning variant | grounded; Max adapter `[GAP: 5.1]` |
| 8 · Add-Channel form | horizontal choice (platform) + textinput ×2 + progress + statusbar | wireframes §3.4 five-step wizard as a TUI prompt sequence; ux-flows §2 journey + status-code table; instructive inline validation (§1.3) | grounded; Max path `[GAP: 5.1]` |
| 9 · Assets | list (type/name/size/source table, inverse-accent selection) + preview viewport + statusbar | wireframes §3.8 re-laid; Asset-Service-only links (§7.1); Selected style per lipgloss-theme §4 | grounded; inline preview `[OPEN: THREADY-DES-TUI-01]` |
| 10 · Skills | list (`[x]`/`[ ]` toggle) + detail viewport + statusbar | wireframes §3.9 + the web skills-manager vocabulary (atomic → composite → umbrella; Research v3 / Movies v5 / Notes v1) | grounded; engine `[GAP: 4.1]`, OCR `[GAP: 2.6]` |

## 3. Conventions the artifacts follow

- **Keys are the verified map** (wireframes §5.1: `ctrl+c/q`, `1–4`/`F1–F4`, `h/l`, `tab`,
  `j/k`, `enter/space`, `f`/`/`, `r/R`, `esc`, `?`), with the Thready domain keys (`c/a/k/n/p`)
  layered non-collidingly; every key is echoed in the statusbar/help rail.
- **Honesty is rendered, not footnoted:** the Max channel shows its ⚠ auth stub state
  `[GAP: 5.1]`; Search renders the mandated HashEmbedder degradation banner with scores hidden
  `[GAP: 2.1]`; the failed queue job shows the max-5-retries dead-letter and the idempotent
  `[r]etry` (second press ⇒ `409 already running`, ux-flows §3.1).
- **Palette provenance:** all hex values are the verified `thready.css` dark tokens; all ANSI-16
  index picks are ASSUMED nearest-color mappings pending terminal verification
  (`[OPEN: THREADY-DES-17]`, see lipgloss-theme.md §5/§8).
- The footer ♥ is `U+2665` tinted with the Lipgloss `Accent` (brand-assets §8); the
  Helix Development attribution line is locked.

## 4. Open items

- `[OPEN: THREADY-DES-17]` — ANSI degradation decisions (pinned `CompleteColor` vs automatic;
  border glyph color; optional light-terminal adaptive palette) — details in
  [lipgloss-theme.md §8](./lipgloss-theme.md#8-open-items).
- `[OPEN: THREADY-DES-TUI-01]` — **inline asset preview** (screen 9): terminal image/video
  rendering is protocol-dependent (kitty graphics / iTerm2 inline images / sixel) and absent on
  plain terminals; decide whether the preview pane renders pixels where the protocol allows or
  stays metadata + open/stream everywhere. The mockup ships the honest metadata-only stub.
- `[OPEN: THREADY-DES-TUI-02]` — **TUI sign-in method** (screen 6): ground truth pins the
  scoped-token path (CLI §4.1 `THREADY_TOKEN`/keychain); the device-code frame is a proposed
  interactive option mirroring the ux-flows §2.1 pending/failure machine — confirm with the
  product whether it ships or the TUI stays token-only.
- The dedicated **Processing queue** screen is a derived composition (`[DEFAULT — adjustable]`);
  confirm with the product whether it ships as its own screen or stays folded into the Dashboard.
- The palette **generator** (tokens → Go) is a tracked workable item of the design-system area
  (THREADY-DES-DS-01 / token-bridge, design-system §9); nothing here claims it exists yet.

---

*Made with love ♥ by Helix Development.*
