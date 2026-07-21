<!--
  Title           : Helix Thready — REST /v1 Endpoint Reference
  Classification  : PUBLIC
  Location        : docs/public/research/mvp/api/rest-endpoints.md
  Status          : Draft — v0.1
  Revision        : 1 (2026-07-21)
  Author          : Helix Thready documentation swarm (API & SDKs)
  Related         : ./openapi.yaml, ./authn-authz.md, ./event-bus-contract.md,
                    ./error-model.md, ./versioning.md, ./sdk-strategy.md, ../database/index.md
-->

# Helix Thready — REST /v1 Endpoint Reference

| Rev | Date | Author | Change |
|-----|------|--------|--------|
| 1 | 2026-07-21 | swarm (API & SDKs) | Initial draft: resource groups, conventions, maturity |
| 2 | 2026-07-21 | swarm (API & SDKs) | Added p95 SLO; documented `x-thready-maturity` values (now real in `openapi.yaml`); expanded idempotency scope; fixed shell-quoting in the example; linked contract-tests.md |

The authoritative machine contract is [openapi.yaml](./openapi.yaml). This document
explains **behaviour** the OpenAPI cannot fully express: resource-group semantics,
pagination, idempotency, filtering, rate limits, and — critically — the **maturity of the
backing subsystem** behind each group (so no group is presented as GA when the gap register
says its backing module is a stub/scaffold).

## Table of Contents

1. [Conventions](#1-conventions)
2. [Resource groups](#2-resource-groups)
   - [2.1 Auth](#21-auth) · [2.2 Accounts](#22-accounts) · [2.3 Users](#23-users)
   - [2.4 Messengers & Channels](#24-messengers--channels) · [2.5 Posts & Threads](#25-posts--threads)
   - [2.6 Processing](#26-processing) · [2.7 Assets & Downloads](#27-assets--downloads)
   - [2.8 Search](#28-search) · [2.9 Skills](#29-skills) · [2.10 Billing](#210-billing)
   - [2.11 Events](#211-events) · [2.12 Ops](#212-ops)
3. [Backing-service maturity (no-bluff)](#3-backing-service-maturity-no-bluff)
4. [Gaps addressed & open items](#4-gaps-addressed--open-items)

The surface topology (edge → middleware → services) is diagrammed in the
[area index](./index.md#surface-at-a-glance) — see [diagrams/api-surface.mmd](./diagrams/api-surface.mmd).

## 1. Conventions

- **Base URL** — `https://{env}.thready.hxd3v.com/v1` (prod host has no env prefix).
- **Transport** — HTTP/3 (QUIC) + HTTP/2 fallback (`vasic-digital/http3`); Brotli/gzip.
- **AuthN/AuthZ** — every route (except `security: []` ones) needs a JWT or scoped API key;
  role floor via `x-required-roles`; see [authn-authz.md](./authn-authz.md).
- **Content type** — `application/json` (UTF-8) except asset content streams.
- **Timestamps** — RFC 3339 UTC; **ids** — UUIDv4 strings unless noted.
- **Pagination** — cursor-based. List responses are `{ "data": [...], "meta": {
  "next_cursor": "…"|null, "total_estimate": N|null } }`. Pass `?cursor=&limit=` (limit
  1–200, default 50). Exact totals are avoided at Large scale (10k+ posts/day) — only an
  estimate is returned.
- **Filtering** — documented per group via query params (e.g. `channel_id`, `hashtag`,
  `status`). `root` may pass `account_id` to cross tenants; other roles are implicitly
  scoped to their account.
- **Idempotency** — unsafe POSTs accept `Idempotency-Key: <uuid>` (24 h window); see
  [error-model.md](./error-model.md) §5. In `openapi.yaml` the header is wired on every
  unsafe/async POST: `createApiKey`, `registerChannel`, `syncChannel`, `triggerProcessing`,
  `triggerReprocessing`, `redownloadAsset`, `registerSkill`, and `ingestCallback`.
- **Rate limits** — `RateLimit-*` headers on every response; 429 + `Retry-After` on breach.
- **SLO** — Aggressive posture (Q14): REST **p95 < 150 ms** for synchronous operations
  (excluding `202` async triggers and asset content streaming); **semantic search
  < 500 ms p95**. These are asserted by the performance tests in
  [contract-tests.md](./contract-tests.md) §performance.
- **Maturity** — operations whose backing subsystem is not yet GA carry an
  `x-thready-maturity` vendor extension in `openapi.yaml` — one of `ga` | `foundation` |
  `build_new` | `design` — mirroring §3 below. A non-`ga` value means the contract is stable
  but the implementation may `503` until the referenced gap closes.
- **Errors** — the single envelope from [error-model.md](./error-model.md). Per that doc,
  `401`, `429` and `500` may be returned by any authenticated operation (middleware chain)
  and are not re-listed per operation.
- **Async** — long operations return `202 Accepted` with a job resource and emit events;
  clients watch the event stream ([event-bus-contract.md](./event-bus-contract.md)) rather
  than blocking.

Example — an authenticated, paginated, filtered list (URL quoted so the shell does not
split on `&` or background the command):

```bash
curl -s "https://thready.hxd3v.com/v1/posts?channel_id=${CID}&status=failed&limit=100" \
  -H "Authorization: Bearer ${ACCESS}" | jq '.data[].id, .meta.next_cursor'
```

## 2. Resource groups

### 2.1 Auth

`/auth/login`, `/auth/refresh`, `/auth/logout`, `/auth/me`, `/auth/mfa/totp/{enroll,verify}`,
`/auth/oauth2/authorize`, `/api-keys` (list/create), `/api-keys/{keyId}` (revoke),
`/.well-known/jwks.json`. Fully specified in [authn-authz.md](./authn-authz.md). `login`,
`refresh`, and `jwks.json` are unauthenticated; everything else needs at least `user`.
API-key creation requires the key's scopes to be a subset of the caller's (403 otherwise).

### 2.2 Accounts

`GET/POST /accounts`, `GET/PATCH/DELETE /accounts/{accountId}`, `PUT
/accounts/{accountId}/branding`.

- `POST /accounts` — any authenticated user may create an account and becomes its
  `account_admin` (final request §6.1). `GET /accounts` returns all accounts for `root`,
  else the caller's memberships.
- `PATCH` — `account_admin`+; may set `retention_days` (per-account override; null =
  inherit global "keep indefinitely"). `DELETE` — `root` only.
- **Branding** — `PUT …/branding` sets white-label colors/logo/slogan; defaults to
  Thready/Helix Development (§8.3). Helix Development attribution persists in footers.

### 2.3 Users

`GET/POST /accounts/{accountId}/users` (list/invite), `GET/PATCH /users/{userId}`.

- Invite is `account_admin`+; a duplicate invite is `409 already_exists`. A user may belong
  to multiple accounts with different roles (`Membership[]`).
- `GET /users/{id}` — self, or an admin within the same tenant; cross-tenant reads are
  `403` (unless `root`). `PATCH` — `account_admin`+ within the tenant (e.g. change a user's
  role, disable).

### 2.4 Messengers & Channels

`GET /messengers`; `GET/POST /channels`, `GET/DELETE /channels/{channelId}`,
`POST /channels/{channelId}/sync`.

- `GET /messengers` returns each platform's capabilities **and maturity** (see §3): Telegram
  (`foundation` — MTProto user-client reader promotion pending), Max (`build_new`).
- `POST /channels` registers a channel/group by invite link or handle (`account_admin`+);
  the fixtures in the final request Appendix A are valid `external_ref`s.
- `POST /channels/{id}/sync` triggers an incremental poll or a full `backfill`. **Backfill
  depends on the MTProto reader (Telegram) / the Max adapter (both BUILD/BUILD-NEW)** — until
  those land it returns `x-thready-maturity: build_new` semantics and may `503 unavailable`.
  Sync is async → `202`; watch `channel.synced` (sticky).

### 2.5 Posts & Threads

`GET /posts` (filter: `channel_id`, `hashtag`, `status`, `account_id`), `GET /posts/{id}`,
`GET /posts/{id}/thread`.

- A **post** carries body, hashtags, derived `categories` (`ContentType[]`), extracted
  `links`, `asset_links`, and its `processing` state.
- `GET /posts/{id}/thread` returns the **complete post** = root + the full organic reply
  chain, **excluding Thready's own system replies** (`author.is_system == true` are never
  returned as processable). This is the core "assemble full thread context" requirement
  (§3.2.1) — tags are frequently added as a reply to a link-only root.
- Content categories are the 17 `ContentType` enum values (video, torrent, series, movie,
  research, documentary, concert, game, software, channel, playlist, music, book, comic,
  netflix, training, technology) — all built in parallel (Q31).

### 2.6 Processing

`POST /posts/{id}/process`, `POST /posts/{id}/reprocess`, `GET /posts/{id}/processing`,
`GET /processing/jobs`, `POST /processing/callbacks/{provider}`.

- `POST /posts/{id}/process` enqueues onto the BackgroundTasks queue with an **idempotent
  single-claim** (Postgres row/advisory lock), so a post is processed **exactly once** under
  a `post.received` event storm (§3.3). Returns `202` + `ProcessingJob`; a concurrent claim
  returns `409 conflict`.
- `reprocess` (`account_admin`+) forces a fresh run even if already `succeeded` — the
  "client → REST API → System" refresh trigger (§3.2.3).
- **Precedence** — a post runs **every** matching Skill, ordered deterministically:
  `download > convert > analyze > research > reply` (§3.3). Exposed as `ProcessingJob.precedence`.
- `POST /processing/callbacks/{provider}` — inbound 3rd-party completion callbacks (HMAC
  auth, not JWT); specified in [event-bus-contract.md](./event-bus-contract.md) §9.
- **Maturity** — the **Skill-dispatch engine is BUILD-NEW** atop `helix_skills` (which is a
  knowledge DAG, not a run engine — `[GAP: 4.1/#6]`). The endpoints are the target contract;
  they are backed by the new engine, not by `helix_skills` alone.

### 2.7 Assets & Downloads

`GET /assets`, `GET /assets/{id}`, `GET /assets/{id}/content`, `POST /assets/{id}/redownload`,
`GET /posts/{id}/assets`, `GET /downloads`.

- Client links are **never raw file paths** — `GET /assets/{id}/content` resolves through the
  Asset Service to a **signed, Range-capable** URL (`OpenSeekable`), returning `200`/`206`
  (Range) or `302` to a short-lived signed URL (§7.1). Sensitive assets are gated + encrypted.
- Assets keep the **raw original** plus web renditions (`…-web` suffix; HLS/DASH ladders,
  §7.3/Q36); `renditions[]` lists them. Dedup + integrity via `content_hash`.
- `POST /assets/{id}/redownload` (`account_admin`+) re-fetches a `broken` physical asset via
  the Download Manager → `202` + `DownloadJob`.
- `GET /posts/{id}/assets` returns `AssetLink[]` with `order_index` preserving
  series/playlist watch order (numeric prefixes).
- **Maturity** — Asset Service is decoupled **from Catalogizer** (`P1`, mature base); the
  **Download Manager is BUILD-NEW** (`P0`); Boba (torrents) + MeTube (video) are FOUNDATION
  and integrate via the standardized callback (§2.6). `filesystem` lacks an HTTP source
  today (`[GAP: 6.2]`).

### 2.8 Search

`POST /search`.

- Semantic / keyword / **hybrid** search over **posts and generated materials** (and
  optionally assets). Returns ranked `{source_id, kind, score, span, snippet}`; `source_id`
  is the **relational PK** the vector references (vectors store reference-only metadata;
  §2.1.1). SLO **< 500 ms p95** (Aggressive).
- Backed by the in-house "Lumen-style" **Semantic-search service** (`embeddings` +
  `vectordb`/pgvector cosine) — `[GAP: New#Semantic-search service]` (`P0` BUILD-NEW).
- **Danger zone, called out** — `[GAP: #1 HelixLLM]` HelixLLM's **default local embedder is
  a non-semantic `HashEmbedder` stub**; semantic search built on it silently returns garbage.
  The service **must** run with `HELIX_EMBEDDING_PROVIDER=llama` (real llama.cpp embeddings)
  and **fail loudly** (`503 unavailable`) if the hash embedder is active in a search context.
  `SearchResult.embedder` echoes the active provider so callers can verify.

### 2.9 Skills

`GET/POST /skills`, `GET /skills/{id}`.

- Skills are **knowledge units** in the Skill-Graph DAG (`atomic → composite → umbrella`)
  with typed edges (`requires`/`extends`/`composes`/`recommends`/`related_to`/
  `alternative_to`) and a `sort_order` for dispatch precedence; `binds_content_types`
  wires a Skill to the `ContentType`s that trigger it.
- `POST /skills` (`account_admin`+, scope `skills:write`) registers a Skill + edges.
- **Maturity** — `helix_skills` is FOUNDATION/MVP with an inconsistent file format and an
  open findings backlog (`[GAP: 4.1]`); these endpoints expose the graph, while **execution**
  is the separate BUILD-NEW dispatch engine (§2.6).

### 2.10 Billing

`GET /plans`, `GET/PUT /accounts/{id}/subscription`, `GET /accounts/{id}/usage`.

- **Subscription + metered** from day one (Q11/`[OPERATOR]`). `PUT …/subscription`
  (`account_admin`+) changes plan; `GET …/usage` returns metered `UsageRecord[]`
  (`posts_processed`, `assets_bytes`, `search_queries`, `llm_tokens`, `storage_bytes`).
- Per-plan rate-limit tiers feed the edge limiter (`[OPEN: api-2]`).

### 2.11 Events

`GET /events` (catalog), `GET /events/{entityType}/{entityId}/sticky` (last-value snapshot).
The live streams (`/v1/events/ws`, `/v1/events/stream`) and outbound/inbound callbacks are
specified in [event-bus-contract.md](./event-bus-contract.md).

### 2.12 Ops

`GET /healthz`, `GET /readyz` (unauthenticated, unversioned in spirit — see
[versioning.md](./versioning.md) §2; `/metrics` is Prometheus-scraped, not in the product
contract). Backed by `observability/pkg/health`.

## 3. Backing-service maturity (no-bluff)

Per the gap register — **do not present a group as GA when its backing module is a
stub/scaffold**. `x-thready-maturity` (one of `ga` | `foundation` | `build_new` | `design`)
is annotated on every affected operation in `openapi.yaml` — 15 operations carry it (verify
with `grep -cE '^\s+x-thready-maturity:' openapi.yaml`). The negative-control test in
[contract-tests.md](./contract-tests.md) §security fails the build if any operation whose
backing row below is non-GA is missing the annotation, so the two cannot drift.

| Group | Backing module(s) | Status | Gap | Contract implication |
|-------|-------------------|--------|-----|----------------------|
| Auth / Users | `digital.vasic.auth` (+ User Service) | PRODUCTION + BUILD-NEW | `#10, 7.2, New` | Contract GA; needs RS256/JWKS + RBAC layer built (see authn-authz). |
| Channels (Telegram) | Herald MTProto reader | FOUNDATION | `#3, 5.1` | Live read OK; **backfill pending** reader promotion. |
| Channels (Max) | Max adapter | **BUILD-NEW** | `#3, 5.1` | Endpoints exist; **503 until adapter built** (Bot API + OneMe WS port). |
| Processing | Skill-dispatch engine | **BUILD-NEW** | `#6, 4.1` | `helix_skills` is a knowledge DAG; execution engine is new. |
| Assets (serve) | Catalogizer → Asset Service | PRODUCTION (decouple P1) | `#9, 6.1` | Serve GA; decouple + HLS/DASH transcoder pending. |
| Downloads | Download Manager | **BUILD-NEW** | `#4, 6.3` | Generic multi-protocol manager is new (P0). |
| Downloads (video) | MeTube | FOUNDATION | `#5, 6.5` | Poll-only today; internal bridge until webhook lands. |
| Search | Semantic-search svc + embeddings | **BUILD-NEW** + trap | `#1, New` | **Must use real llama.cpp embedder**; fail loudly on hash stub. |
| Skills (read) | `helix_skills` | FOUNDATION | `#6, 4.1` | Graph read OK; format/backlog caveats. |
| Events | Event Bus service | **BUILD-NEW** | `#5, New` | Client-facing wrapper over eventbus/JetStream is new. |
| Billing | metering/subscription | design | `#12` | Schema fixed; provider integration in deployment pack. |

## 4. Gaps addressed & open items

- `[GAP: #3/5.1]` Herald Telegram backfill + Max adapter maturity surfaced honestly — §2.4, §3.
- `[GAP: #6/4.1]` processing endpoints backed by a BUILD-NEW Skill-dispatch engine — §2.6, §3.
- `[GAP: #1]` search embedder trap (fail loudly on HashEmbedder) — §2.8.
- `[GAP: New]` Semantic-search, Download Manager, Event Bus, User Service as BUILD-NEW — §3.
- `[GAP: #4/6.2/6.3/6.5]` download/callback contract + maturity — §2.6, §2.7.
- **TDD** — reproduce-first RED skeletons for every behaviour in this doc (pagination,
  idempotency single-claim, tenant isolation, maturity annotation, search fail-loud) are in
  [contract-tests.md](./contract-tests.md), mapped to the 15 mandated test types
  `[CONSTITUTION §11.4.27]`.
- `[OPEN: rest-1]` The full per-endpoint request/response examples set is expanded in the
  SDK quickstarts (Docs Chain) once the servers exist; this doc + `openapi.yaml` fix the
  contract.
- `[OPEN: rest-2]` `/metrics` exposure format (Prometheus text vs OTLP) is set in the
  deployment pack; it is out of the product `/v1` contract.

---

*Made with love ♥ by Helix Development.*
