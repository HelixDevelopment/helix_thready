<!--
  Title           : Helix Thready — Marketing Site SEO / Meta Specification
  Classification  : PUBLIC
  Location        : docs/public/research/mvp/design/screens/marketing/seo.md
  Status          : Draft — v0.1
  Revision        : 1 (2026-07-22)
  Author          : Helix Thready documentation swarm (design · marketing)
  Related         : ./README.md, ./landing.html, ./features.html, ./download.html,
                    ./privacy.html, ./terms.html, ./imprint.html,
                    ../../brand-assets.md, ../../design-system.md,
                    ../../assets/generate-raster.sh, ../../assets/icon-export-matrix.md,
                    ../../../deployment/index.md
-->

# Helix Thready — Marketing Site SEO / Meta Specification

| Rev | Date | Author | Change |
|-----|------|--------|--------|
| 1 | 2026-07-22 | swarm (design · marketing) | Initial SEO/meta spec under the active scoping of `[OPEN: THREADY-DES-MKT-02]`: per-page title/description/OG/Twitter values derived from real product copy; JSON-LD `SoftwareApplication` template restricted to doc-backed fields; sitemap.xml + robots.txt templates; canonical/hreflang notes for en / ru / sr-Cyrl |

## Table of contents

- [1. Scope and ground rules](#1-scope-and-ground-rules)
- [2. Canonical origin and URL model](#2-canonical-origin-and-url-model)
- [3. Per-page titles and descriptions](#3-per-page-titles-and-descriptions)
- [4. Open Graph and Twitter cards](#4-open-graph-and-twitter-cards)
- [5. JSON-LD — SoftwareApplication](#5-json-ld--softwareapplication)
- [6. sitemap.xml template](#6-sitemapxml-template)
- [7. robots.txt template](#7-robotstxt-template)
- [8. Canonical and hreflang notes (en / ru / sr-Cyrl)](#8-canonical-and-hreflang-notes-en--ru--sr-cyrl)
- [9. Operator placeholders and open ends](#9-operator-placeholders-and-open-ends)

## 1. Scope and ground rules

This document is the meta/SEO deliverable of the **active scoping of
`[OPEN: THREADY-DES-MKT-02]`** (see [README.md](./README.md) §7). It covers the six designed
marketing pages — [`landing`](./landing.html), [`features`](./features.html),
[`download`](./download.html), [`privacy`](./privacy.html), [`terms`](./terms.html),
[`imprint`](./imprint.html) — rendered by the **Angular 22 marketing app with SSR +
SSG/prerender** ([design-system.md](../../design-system.md) §7 note `[Q17]`), where every value
below lives as route-level metadata emitted at prerender time.

Ground rules, identical to the page set `[CONVENTIONS §7 — no bluff]`:

- **Every description derives from real product copy** already registered in
  [README.md](./README.md) §5 — no new product claim is minted here.
- **No invented assets**: every image slot references a file the
  [raster pipeline](../../assets/generate-raster.sh) actually emits, or the hand-authored
  master SVGs in [`../../assets/`](../../assets/).
- **No invented identities**: social handles, organization URLs and final locale URLs are
  `[OPERATOR]` slots, never guessed.
- Copy contract as everywhere on the marketing surface: calm, understated, no exclamation
  marks (DESIGN.md §1 voice and tone).

## 2. Canonical origin and URL model

The deployment ground truth names exactly one production domain
([deployment index](../../../deployment/index.md) §1, table "The three subdomains (from §8.2)"):

| Environment | Domain |
|-------------|--------|
| Development | `dev.thready.hxd3v.com` |
| Staging | `sta.thready.hxd3v.com` |
| Production | `thready.hxd3v.com` |

Throughout this spec, `{ORIGIN}` denotes the marketing-site origin.
**`[OPERATOR]`: confirm whether the marketing site is served from `https://thready.hxd3v.com`
itself or from a dedicated host** — the deployment docs name the production domain for the
system as a whole and do not separately place the marketing app.

Path model `[DEFAULT — adjustable]` (mirrors the designed page set one-to-one):

| Page | Path (en default) |
|------|-------------------|
| Landing | `/` |
| Features | `/features` |
| Download | `/download` |
| Privacy policy | `/legal/privacy` |
| Terms of service | `/legal/terms` |
| Imprint | `/legal/imprint` |

Locale URLs for ru and sr-Cyrl: **`[OPERATOR: final locale URLs]`**. The working default in the
templates below is a path prefix (`/ru/…`, `/sr/…`) `[DEFAULT — adjustable]`; see §8 for the
hreflang mechanics either way.

## 3. Per-page titles and descriptions

Titles are the `<title>` values already carried by the designed pages. Descriptions are
assembled only from copy on those pages (register rows cited). Targets: title ≤ 60 characters,
description ≤ 160 characters.

| Page | `<title>` | `meta description` | Provenance |
|------|-----------|--------------------|------------|
| Landing | `Thready — read your threads, smarter` | `Thready follows the channels you already read, processes every post through an accountable AI pipeline, and turns threads into searchable research.` | Tagline: brand-assets.md §8.1 (README §5 row 1); sentence: landing.html hero sub (rows 2/6/8) |
| Features | `Features — Thready` | `Channels with visible backfill, threads kept intact, an AI pipeline you can watch, and search that admits its limits — how Thready works today.` | landing.html feature cards / features.html card grid (rows 2, 4, 6, 8, 9) |
| Download | `Download — Thready` | `Where Thready runs: the web portal first, a desktop app wrapping the same UI, a terminal client, and mobile clients in development. No store listing is live yet.` | download.html statuses (rows 15–19, 26) |
| Privacy | `Privacy policy — Thready` | `How Thready handles account data, connected channel content and processing artifacts.` — **interim, structural**; `[OPERATOR — replace after counsel text lands]` | privacy.html §2 inventories (rows 3, 5, 6, 12, 13, 25) |
| Terms | `Terms of service — Thready` | `The agreement covering use of Thready.` — **interim, structural**; `[OPERATOR — replace after counsel text lands]` | terms.html structure |
| Imprint | `Imprint — Thready` | `Provider identification for Thready.` — **interim, structural**; `[OPERATOR — replace after counsel text lands]` | imprint.html structure |

Deployment gate: the three legal pages must not be deployed at all until their
`[OPERATOR — legal counsel text required]` placeholders are replaced (each page's design
contract strip states this), so their interim descriptions can never reach an index ahead of
counsel.

## 4. Open Graph and Twitter cards

### 4.1 Image slots — what actually exists

The [raster pipeline](../../assets/generate-raster.sh) (`generate-raster.sh`, honest-output
contract — it never writes placeholder PNGs) emits, per
[icon-export-matrix.md](../../assets/icon-export-matrix.md) and
[brand-assets.md](../../brand-assets.md) §5/§5.1:

- `raster/web/icon-192.png`, `raster/web/icon-512.png` — PWA icons (deployed as
  `/icons/icon-192.png`, `/icons/icon-512.png` per the `manifest.webmanifest` block in
  brand-assets.md §5.1);
- `raster/web/maskable-192.png`, `raster/web/maskable-512.png` — maskable variants;
- `raster/web/favicon.svg`, `favicon-16/32/48.png`, `favicon.ico` — favicon set;
- `raster/brand/logo-mark-128/256/512.png`, `raster/brand/logo-full-h64/h128.png`,
  `raster/brand/footer-slogan-h44.png` — brand lockup conveniences.

**There is no 1200×630 (1.91:1) social-card render in the pipeline today.** This spec does not
pretend one exists: the `og:image` slot uses the real 512×512 launcher icon
(`{ORIGIN}/icons/icon-512.png`) as the interim value, and the Twitter card type is `summary`
(square art), **not** `summary_large_image`, until a dedicated social card is added to
`generate-raster.sh` — tracked in §9.

### 4.2 Per-page template

One block per page; `{PATH}`, `{TITLE}`, `{DESCRIPTION}` come from §2/§3.

```html
<link rel="canonical" href="{ORIGIN}{PATH}">
<meta property="og:type" content="website">
<meta property="og:site_name" content="Thready">
<meta property="og:title" content="{TITLE}">
<meta property="og:description" content="{DESCRIPTION}">
<meta property="og:url" content="{ORIGIN}{PATH}">
<meta property="og:image" content="{ORIGIN}/icons/icon-512.png">
<meta property="og:image:width" content="512">
<meta property="og:image:height" content="512">
<meta property="og:image:alt" content="Thready launcher icon — double-spiral mark">
<meta name="twitter:card" content="summary">
<meta name="twitter:title" content="{TITLE}">
<meta name="twitter:description" content="{DESCRIPTION}">
<meta name="twitter:image" content="{ORIGIN}/icons/icon-512.png">
```

Notes:

- `og:locale` / `og:locale:alternate` require `language_TERRITORY` values; the mapping from
  en / ru / sr-Cyrl to territory-qualified locales is **`[OPERATOR]`** (part of the final
  locale decision, §8) and the tags are omitted until then rather than guessed.
- `twitter:site` / `twitter:creator`: **`[OPERATOR — social handles, if any exist]`** — no
  handle is documented anywhere in the ground truth, so none is invented.
- The favicon and manifest `<head>` block is already specified verbatim in
  [brand-assets.md](../../brand-assets.md) §5.1 and is not duplicated here.

## 5. JSON-LD — SoftwareApplication

Emitted on the landing page only `[DEFAULT — adjustable]`. Restricted to fields with a
documented source; everything else is deliberately omitted.

```html
<script type="application/ld+json">
{
  "@context": "https://schema.org",
  "@type": "SoftwareApplication",
  "name": "Thready",
  "slogan": "read your threads, smarter",
  "description": "Thready follows the channels you already read, processes every post through an accountable AI pipeline, and turns threads into searchable research.",
  "operatingSystem": "Web",
  "url": "{ORIGIN}/",
  "image": "{ORIGIN}/icons/icon-512.png",
  "applicationCategory": "[OPERATOR — category claim, not documented; choose or omit]",
  "author": {
    "@type": "Organization",
    "name": "Helix Development",
    "url": "[OPERATOR — organization URL, not documented]"
  }
}
</script>
```

Field provenance and deliberate omissions:

| Field | Status | Source / reason |
|-------|--------|-----------------|
| `name` | doc-backed | Product name throughout the design pack |
| `slogan` | doc-backed | brand-assets.md §8.1 tagline (README §5 row 1) |
| `description` | doc-backed | landing.html hero sub (README §5 rows 2/6/8) |
| `operatingSystem: "Web"` | doc-backed | Web is the primary and only production-usable surface today (README §5 row 15); scaffold clients are not listed as shipped platforms |
| `image` | doc-backed | Raster-pipeline output, §4.1 |
| `author.name` | doc-backed | Locked attribution "Made with ♥ by Helix Development" (brand-assets.md §8) |
| `applicationCategory` | `[OPERATOR]` | No category claim documented — supply or omit |
| `author.url` | `[OPERATOR]` | No organization site documented — supply or omit |
| `offers` | **omitted** | No public pricing exists (terms.html §8 fact panel); inventing a price or "free" claim is a bluff either way |
| `aggregateRating` / `review` | **omitted** | No ratings exist; never fabricated |
| `downloadUrl` / `installUrl` | **omitted** | No installer is published (README §5 row 26) |
| `softwareVersion` | **omitted** | MVP in active development; no published version |

## 6. sitemap.xml template

Six canonical URLs; each entry carries hreflang alternates for the three locales (values per
§8; ru/sr URLs are the `[DEFAULT — adjustable]` path prefixes until
**`[OPERATOR: final locale URLs]`** lands). `lastmod` is emitted by the SSG prerender build,
not hand-maintained.

```xml
<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9"
        xmlns:xhtml="http://www.w3.org/1999/xhtml">
  <url>
    <loc>{ORIGIN}/</loc>
    <lastmod>{BUILD_DATE}</lastmod>
    <xhtml:link rel="alternate" hreflang="en" href="{ORIGIN}/"/>
    <xhtml:link rel="alternate" hreflang="ru" href="{ORIGIN}/ru/"/>
    <xhtml:link rel="alternate" hreflang="sr-Cyrl" href="{ORIGIN}/sr/"/>
    <xhtml:link rel="alternate" hreflang="x-default" href="{ORIGIN}/"/>
  </url>
  <url>
    <loc>{ORIGIN}/features</loc>
    <lastmod>{BUILD_DATE}</lastmod>
    <xhtml:link rel="alternate" hreflang="en" href="{ORIGIN}/features"/>
    <xhtml:link rel="alternate" hreflang="ru" href="{ORIGIN}/ru/features"/>
    <xhtml:link rel="alternate" hreflang="sr-Cyrl" href="{ORIGIN}/sr/features"/>
    <xhtml:link rel="alternate" hreflang="x-default" href="{ORIGIN}/features"/>
  </url>
  <url>
    <loc>{ORIGIN}/download</loc>
    <lastmod>{BUILD_DATE}</lastmod>
    <xhtml:link rel="alternate" hreflang="en" href="{ORIGIN}/download"/>
    <xhtml:link rel="alternate" hreflang="ru" href="{ORIGIN}/ru/download"/>
    <xhtml:link rel="alternate" hreflang="sr-Cyrl" href="{ORIGIN}/sr/download"/>
    <xhtml:link rel="alternate" hreflang="x-default" href="{ORIGIN}/download"/>
  </url>
  <url>
    <loc>{ORIGIN}/legal/privacy</loc>
    <lastmod>{BUILD_DATE}</lastmod>
    <xhtml:link rel="alternate" hreflang="en" href="{ORIGIN}/legal/privacy"/>
    <xhtml:link rel="alternate" hreflang="ru" href="{ORIGIN}/ru/legal/privacy"/>
    <xhtml:link rel="alternate" hreflang="sr-Cyrl" href="{ORIGIN}/sr/legal/privacy"/>
    <xhtml:link rel="alternate" hreflang="x-default" href="{ORIGIN}/legal/privacy"/>
  </url>
  <url>
    <loc>{ORIGIN}/legal/terms</loc>
    <lastmod>{BUILD_DATE}</lastmod>
    <xhtml:link rel="alternate" hreflang="en" href="{ORIGIN}/legal/terms"/>
    <xhtml:link rel="alternate" hreflang="ru" href="{ORIGIN}/ru/legal/terms"/>
    <xhtml:link rel="alternate" hreflang="sr-Cyrl" href="{ORIGIN}/sr/legal/terms"/>
    <xhtml:link rel="alternate" hreflang="x-default" href="{ORIGIN}/legal/terms"/>
  </url>
  <url>
    <loc>{ORIGIN}/legal/imprint</loc>
    <lastmod>{BUILD_DATE}</lastmod>
    <xhtml:link rel="alternate" hreflang="en" href="{ORIGIN}/legal/imprint"/>
    <xhtml:link rel="alternate" hreflang="ru" href="{ORIGIN}/ru/legal/imprint"/>
    <xhtml:link rel="alternate" hreflang="sr-Cyrl" href="{ORIGIN}/sr/legal/imprint"/>
    <xhtml:link rel="alternate" hreflang="x-default" href="{ORIGIN}/legal/imprint"/>
  </url>
</urlset>
```

The three legal URLs enter the sitemap **only once counsel text has replaced every
placeholder** — before that the pages do not deploy at all (§3 deployment gate).

## 7. robots.txt template

Production marketing origin:

```text
User-agent: *
Allow: /

Sitemap: {ORIGIN}/sitemap.xml
```

Operational notes, grounded in the deployment ground truth
([deployment index](../../../deployment/index.md) §1):

- `dev.thready.hxd3v.com` and `sta.thready.hxd3v.com` are real, publicly-routable
  environment subdomains. They must not be indexed: serve them
  `User-agent: * / Disallow: /` **and** an `X-Robots-Tag: noindex` header
  `[DEFAULT — adjustable]` — robots.txt alone does not prevent indexing of discovered URLs.
- The signed-in product portal is an application, not a content site; whether any of it is
  crawlable is out of scope here and default-deny `[DEFAULT — adjustable]`.

## 8. Canonical and hreflang notes (en / ru / sr-Cyrl)

The locale set comes from the designed language picker (EN / RU / SR with values
`en` / `ru` / `sr-Cyrl`) present on every marketing page.

- **Canonical**: every locale variant is self-canonical (its `rel=canonical` points to its own
  URL, never to the en page) — the variants are translations, not duplicates.
- **hreflang codes**: `en`, `ru`, and `sr-Cyrl` — the Serbian tag is script-qualified BCP-47,
  matching the picker value exactly (Serbian in Cyrillic script). If a Latin-script Serbian
  variant is ever added it would be `sr-Latn`, a separate alternate — not claimed today.
- **x-default**: points to the en URL `[DEFAULT — adjustable]` — en is the authoring language
  of the entire design pack.
- **Reciprocity**: every page's alternate cluster lists all three locales plus x-default, and
  every variant emits the same cluster (hreflang is only honored when reciprocal). The sitemap
  entries in §6 mirror the same clusters.
- **URL shape**: the `/ru/…`, `/sr/…` path prefixes are a working default only —
  **`[OPERATOR: final locale URLs]`** decides prefix vs subdomain vs anything else. Whatever
  lands, the invariants above (self-canonical, reciprocal clusters, script-qualified Serbian
  tag) carry over unchanged.
- **HTML `lang`**: each variant sets `<html lang>` to its locale (`en`, `ru`, `sr-Cyrl`) —
  the designed artifacts carry `lang="en"` as the en originals.

## 9. Operator placeholders and open ends

Everything a human must supply before this spec is deployable, in one place:

| # | Slot | Where |
|---|------|-------|
| 1 | Marketing-site origin (`{ORIGIN}`) — confirm `https://thready.hxd3v.com` or a dedicated host | §2 |
| 2 | Final locale URLs for ru / sr-Cyrl (prefix vs subdomain vs other) | §2, §6, §8 |
| 3 | Final meta descriptions for privacy / terms / imprint after counsel text lands | §3 |
| 4 | `og:locale` territory mapping for the three locales | §4.2 |
| 5 | Social handles (`twitter:site` / `twitter:creator`), if any exist | §4.2 |
| 6 | JSON-LD `applicationCategory` and `author.url` — supply or omit | §5 |
| 7 | A dedicated 1200×630 social-card render added to `generate-raster.sh` (then upgrade `og:image` and the Twitter card type to `summary_large_image`) | §4.1 |
| 8 | Analytics decision — adoption or deliberate absence — still open under `[OPEN: THREADY-DES-MKT-02]`; it interacts with the privacy page's §2.4 placeholder | README.md §7 |

No JSON-LD, sitemap or robots artifact ships until slots 1–2 are decided; the legal URLs
additionally wait on slot 3 and the counsel placeholders in the pages themselves.

---

*Made with love ♥ by Helix Development.*
