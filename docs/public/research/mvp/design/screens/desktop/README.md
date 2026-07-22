<!--
  Title           : Helix Thready — Desktop Screen Designs (Tauri) & Differences Catalogue
  Classification  : PUBLIC
  Location        : docs/public/research/mvp/design/screens/desktop/README.md
  Status          : Draft — v0.1
  Revision        : 1 (2026-07-22)
  Author          : Helix Thready documentation swarm (design · screens)
  Related         : ./desktop-shell.html, ../../wireframes.md, ../../design-system.md,
                    ../../theming.md, ../../../CONVENTIONS.md
-->

# Helix Thready — Desktop (Tauri 2) Screen Designs

| Rev | Date | Author | Change |
|-----|------|--------|--------|
| 1 | 2026-07-22 | swarm (design · screens) | Initial: Tauri shell artifact + desktop-differences catalogue (shortcuts, offline cache, notifications, file-drop, tray, menus) |

## Table of contents

- [1. Position](#1-position)
- [2. Catalogue](#2-catalogue)
- [3. Desktop-differences catalogue](#3-desktop-differences-catalogue)
- [4. Open items](#4-open-items)

## 1. Position

The desktop client is **Tauri 2 wrapping the Angular 19 product portal** — "no separate token
work; OS-specific chrome overrides only" `[VERIFIED — design-system.md §7 platform table;
wireframes.md §1]`. There are therefore **no separate desktop screen designs**: every screen is the
responsive web portal ([wireframes.md §3](../../wireframes.md#3-web-portal--screen-wireframes),
realized in `screens/web/`). What *is* desktop-specific is the shell around it — title bar, menus,
tray, native notifications, shortcuts-as-accelerators, offline behavior, drag-and-drop — and that
is what this directory designs. Whether desktop needs **anything** beyond the wrapped web UI
(tray, native notifications) is explicitly `[OPEN: THREADY-DES-08]`; everything in that scope
below is a proposal (`[DEFAULT — adjustable]`), never presented as decided.

## 2. Catalogue

| File | Content | Ground truth | Status |
|------|---------|--------------|--------|
| [desktop-shell.html](./desktop-shell.html) | Self-contained artifact: Tauri window with per-OS custom title bar (macOS traffic lights + global menu bar / Windows caption buttons / Linux CSD), native menu map, wrapped app shell + Dashboard, tray popover mock, native notification mock, file-drop overlay; light+dark via `[data-theme="dark"]` | design-system §7, wireframes §3.1/§3.3/§1.2, theming §2 | shell grounded; tray/notifications `[OPEN: THREADY-DES-08]` |

## 3. Desktop-differences catalogue

Everything the desktop build adds/changes relative to the same web app running in a browser tab.

### 3.1 Keyboard shortcuts

The web keyboard model is **verified ground truth** (wireframes §1.2) and works unchanged; desktop
promotes it to native menu accelerators so shortcuts appear in menus and work when focus is in
webview chrome:

| Binding | Action | Source |
|---------|--------|--------|
| `/` or `Ctrl/⌘+K` | focus global search | wireframes §1.2 (verified web model) |
| `g` then `d/c/s/a/k` | go to Dashboard/Channels/Search/Assets/Skills | wireframes §1.2 |
| `Esc`, `?`, `[`/`]`, `Tab`, `Enter`/`Space` | overlays / help / nav collapse / focus / activate | wireframes §1.2 |
| `Ctrl/⌘+N` | New Channel (Add-Channel wizard) | `[DEFAULT — adjustable]` desktop accelerator |
| `Ctrl/⌘+W` | close window → hide to tray | `[DEFAULT — adjustable]`, tray under `[OPEN: THREADY-DES-08]` |
| `Ctrl/⌘+ +/−/0` | zoom in/out/reset (webview zoom) | `[DEFAULT — adjustable]` |
| Global (system-wide) show/hide shortcut | none by default | deliberately omitted — not grounded |

Rule: desktop MUST NOT rebind any verified web binding; accelerators are additive.

### 3.2 Offline cache

The web app already specifies connectivity states for mobile (banner *live/reconnecting/offline*,
cached snapshot reads "stale · updated 3m ago", queued optimistic mutations, WS→polling fallback —
wireframes §6.2). Desktop adopts the **same contract** `[DEFAULT — adjustable]`:

- Last-snapshot cache for Dashboard/Channels/Search results renders when offline; footer shows
  `offline cache: fresh | stale (3m) | offline`.
- Mutations queue and flush on reconnect with the same conflict-resolution toast.
- The Tauri shell adds nothing beyond persistence of the webview cache — no separate sync engine
  is designed (or claimed).

### 3.3 Notifications

- In-focus: the standard in-app toast (`thready-toast`) — identical to web.
- Hidden/unfocused: native OS notification on `processing.completed` / `processing.failed`
  (the verified Event Bus contract, ux-flows §3); click focuses the window on the Post detail.
  Never both at once. **Scope is `[OPEN: THREADY-DES-08]`** — the wireframes explicitly leave
  "native tray/notifications for processing completion" undecided.

### 3.4 File-drop

Two distinct cases, deliberately separated for honesty:

1. **Text/URL drop** (grounded): a dropped string matching `t.me/…` or `max.ru/join/…` opens the
   **Add-Channel wizard at step 3 with the link pre-filled** — exactly the paste+Resolve step that
   already exists (wireframes §3.4). Non-matching text is rejected with the same `422` message the
   Resolve step uses.
2. **Media-file drop** (`[OPEN: THREADY-DES-16]`): there is **no ground-truth ingest path for
   local files** — posts and assets originate from messenger channels, and the Assets library has
   no upload flow in the wireframes. Until product decides whether local-file ingest exists at
   all, the drop overlay names the gap and does nothing else. Never fake an upload UI.

### 3.5 Tray `[OPEN: THREADY-DES-08]` `[DEFAULT — adjustable]`

Close hides to tray; background WS/SSE subscriptions continue; tray badge = active processing
count (danger tint on failure); popover: live queue summary, Open, Pause polling, Quit. See the
mock in [desktop-shell.html](./desktop-shell.html).

### 3.6 Native menus & title bar

Per-OS chrome only (macOS global menu bar + traffic lights; Windows/Linux in-window menu row +
caption buttons); the menu **map** mirrors the web IA and verified shortcuts (table in
[desktop-shell.html](./desktop-shell.html)); menu structure itself `[DEFAULT — adjustable]`.
Window title carries the active Account (`Thready — Acme`).

### 3.7 Theming

Identical to web: shared `ThemeService` (`thready-theme` storage, system default) + the
per-Account white-label `:root[data-account=…]` injection (theming §2/§6). The View ▸ Appearance
menu drives the same three-state toggle. A pre-paint head script avoids the wrong-theme flash on
the webview (theming §2).

## 4. Open items

- `[OPEN: THREADY-DES-08]` — confirm desktop scope beyond the wrapped web UI: tray, native
  notifications, close-to-tray. All such designs above are proposals pending this decision.
- `[OPEN: THREADY-DES-16]` — local media-file ingest (file-drop case 2): decide whether a local
  upload path into the Asset Service exists at all; no UI is designed until it does.

---

*Made with love ♥ by Helix Development.*
