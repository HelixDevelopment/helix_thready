<!--
  Title           : Helix Thready — Implementation Phase Index
  Classification  : PUBLIC
  Location        : implementation/README.md
  Status          : Active — v1.2
  Revision        : 4 (2026-07-22)
  Author          : Helix Thready documentation swarm (implementation)
  Related         : ./QUALITY_GATE.md · ./sdk/CONFORMANCE.md · ../docs/public/research/mvp/index.md · ../docs/public/research/mvp/CONVENTIONS.md · ../docs/private/research/mvp/helix_thready_subsystem_gaps_and_improvements.md
-->

# Helix Thready — Implementation Phase Index

This directory stages the **first wave of buildable code** for Helix Thready: 17
self-contained, project-agnostic **Go modules** plus an `integration` capstone, each
under `implementation/<name>/` with module path `digital.vasic.<X>` (the capstone is
`thready.integration`), staged here as a monorepo. Each standalone module has its own
`go.mod` (standard-library only) and is intended to be **promoted later to its own
repository** under `vasic-digital` / `HelixDevelopment` per `[CONSTITUTION §11.4.28]`
(decoupled submodules). A committed `go.work` workspace (force-added — `go.work` is
normally gitignored) ties **fourteen** of the modules together for the end-to-end
`integration` capstone; the three newest modules (`boba_adapter`, `config`, `sdk_go`)
are not listed in the workspace and build standalone with `GOWORK=off`.

Every module ships a `README.md` (contract + API) and an `EVIDENCE.md` (verbatim,
reproducible `go build`/`go vet`/`gofmt`/`go test` capture with an honest verdict).
This index summarizes each and is grounded entirely in what those files state — the
REAL-vs-stub column below is deliberately conservative.

**Wave-2 addendum ([§1.3](#13-wave-2-additions)).**
Since the initial 17-module wave, four kinds of work landed: two more Go modules
(`cli`, `processing`), a real `deployment_smoke` HTTP proof of the gateway, and **five
additional-language SDKs** (`sdk_py`, `sdk_ts`, `sdk_java`, `sdk_rs`, `sdk_rb`) — so the
gateway now has a client in **six languages** (Go + those five). Two consolidated,
freshly-recomputed evidence artifacts back the whole tree: **[`QUALITY_GATE.md`](./QUALITY_GATE.md)**
(a real gate re-run — **20 Go modules · 374 test functions · race-clean**) and
**[`sdk/CONFORMANCE.md`](./sdk/CONFORMANCE.md)** (all 6 SDK suites re-run — **117 non-Go
tests, 0 failures** — plus a source-read conformance matrix: **42/42** operation cells
and **24/24** behavior cells match the canonical `openapi.yaml`, verdict **CONSISTENT**).
A background security review also hardened `sdk_go` (credential-over-cleartext-http guard)
and `cli` (password read from `THREADY_PASSWORD`, off the process argv).

| Rev | Date | Author | Change |
|-----|------|--------|--------|
| 1 | 2026-07-22 | swarm (implementation) | Initial index over the 14-module implementation wave |
| 2 | 2026-07-22 | swarm (implementation) | Add `boba_adapter`, `config`, `sdk_go` (→ 17 standalone modules) + the `go.work` `integration` capstone; refresh the aggregate (329 test fns, 18 EVIDENCE.md, all stdlib-only) |
| 3 | 2026-07-22 | swarm (implementation) | Cross-cutting review corrections: `ocr_adapter` recorded **race-clean** (→ **17/17** standalone suites green under `-race`, no exception); `config` redaction now masks `THREADY_NATS_URL` + `OTEL_EXPORTER_OTLP_ENDPOINT` (+1 test → 22, aggregate 330) |
| 4 | 2026-07-22 | swarm (implementation) | **Wave-2 additions** — `cli` + `processing` Go modules, `deployment_smoke` (real 12/12 HTTP proof of `rest_gateway`), and **5 polyglot SDKs** (`sdk_py`/`sdk_ts`/`sdk_java`/`sdk_rs`/`sdk_rb`); a background security review fixed a **credential-over-cleartext-http guard** in `sdk_go` + **password-off-argv** in `cli` (see [§1.3](#13-wave-2-additions)). Consolidated real evidence in [`QUALITY_GATE.md`](./QUALITY_GATE.md) (**20 Go modules, 374 tests, race-clean**) and [`sdk/CONFORMANCE.md`](./sdk/CONFORMANCE.md) (**6 SDKs, matrix CONSISTENT**) |

## Table of contents

- [1. The modules (17 standalone + integration capstone)](#1-the-modules-17-standalone--integration-capstone)
  - [1.1 Summary table](#11-summary-table)
  - [1.2 Per-module REAL vs honest-stub detail](#12-per-module-real-vs-honest-stub-detail)
  - [1.3 Wave-2 additions (cli, processing, deployment_smoke, polyglot SDKs)](#13-wave-2-additions)
- [2. Status summary](#2-status-summary)
- [3. What's real vs pending](#3-whats-real-vs-pending)
- [4. How to run](#4-how-to-run)
- [5. Promotion plan](#5-promotion-plan)
- [6. Upstream status](#6-upstream-status)

Provenance tags used below: `[CONSTITUTION §x]` authoritative · `[BUILD-NEW]`
confirmed new-submodule gap · `[GAP: id]` addresses a gap-register item ·
`[OPERATOR]` operator decision · `[OPEN: id]` tracked-open item.

---

## 1. The modules (17 standalone + integration capstone)

### 1.1 Summary table

Rows 1–17 are the standalone, independently-promotable modules; the final row is the
`go.work` **integration capstone** that composes the first fourteen of them.

| # | Module · path | Purpose (one line) | Closes | Tests · `-race` | REAL vs honest `[BUILD-NEW]` stub |
|---|---------------|--------------------|--------|-----------------|-----------------------------------|
| 1 | **download_manager** · `digital.vasic.downloadmanager` | Multi-protocol download manager: job queue, worker pool, retry, progress/completion callbacks | `[GAP: 6.3]` | 14 · ✅ clean | **REAL:** HTTP(S) segmented + resumable + SHA-256 download; Manager (queue/state/retry/pause-resume). **STUB:** FTP/SMB/NFS/WebDav → `ErrNotImplemented` |
| 2 | **callback_task** · `digital.vasic.callbacktask` | One canonical async-job contract (accept→run→status→HMAC webhook→retry→dead-letter) | `[GAP: 6.4]`/`[GAP: 6.5]` (ATM-030) | 16 · ✅ clean | **REAL:** task state machine, retry/backoff, work + delivery dead-lettering, HMAC-SHA256 webhook. **Pending:** non-blocking async delivery wrapper is the caller's (documented) |
| 3 | **threadreader** · `digital.vasic.threadreader` | Messenger-agnostic thread assembly: root + organic replies, hashtag union | `[GAP: 5.1.3]` | 11 fn / 29 cases · ✅ clean | **REAL:** assembly/system-filter/hashtag pure core. **Pending:** live Telegram/Max sources (`MessageSource` is an in-memory fake here) |
| 4 | **ocr_adapter** · `digital.vasic.ocr` | OCR engine for VisionEngine via the real `tesseract` CLI (hybrid seam) | `[GAP: 2.6]` | 13 · ✅ clean | **REAL:** `TesseractProvider` (real tesseract 5.3.0 shell-out). **STUB:** `LLMVisionProvider` `[BUILD-NEW]` interface only, never faked |
| 5 | **user_service** · `digital.vasic.userservice` | Identity/access core: passwords, JWT, API keys, RBAC, TOTP | `[BUILD-NEW: User Service]` | 36 · ✅ clean | **REAL:** PBKDF2, JWT HS256/RS256 (rotation/revocation/alg-pin), API keys, RBAC, TOTP (RFC 6238/4226 vectors). **Pending:** REST handlers, JWKS, durable DB (in-memory stores), OAuth2 linking; Argon2id→stdlib PBKDF2 |
| 6 | **event_bus_service** · `digital.vasic.eventbusservice` | In-process pub/sub: filters, sticky+invalidate, durable replay-from-cursor | `[BUILD-NEW: Event Bus]` | 13 · ✅ clean | **REAL:** pub/sub, glob+metadata filters, sticky, durable replay, async ordering, metrics. **Pending:** NATS JetStream durable transport (adapter seam); no retention/eviction window |
| 7 | **semantic_search** · `digital.vasic.semsearch` | Lumen-style core: chunk→embed→cosine-KNN→boost→floor→merge→rank | `[GAP: 2.1]` | 19 · ✅ clean | **REAL:** Go-AST+Markdown chunker, deterministic feature-hashing embedder, in-memory cosine-KNN, scoring engine. **Pending:** `OpenAICompatEmbedder`→llama.cpp/HelixLLM `/v1/embeddings` (built, not integration-tested); pgvector store |
| 8 | **skill_dispatch** · `digital.vasic.skilldispatch` | Skill execution engine: resolve→order→run with idempotent claim + retry + events | `[GAP: 4.1]` | 23 fn / 27 cases · ✅ clean | **REAL:** registry/resolve, stage precedence, exactly-once single-claim, retry/backoff, event stream. **Pending:** Postgres/BackgroundTasks durable claim + real event bus (modeled in-memory); concrete Skills plug in |
| 9 | **metering** · `digital.vasic.metering` | Usage metering, quota (atomic reserve), integer-cent billing | `[OPERATOR]` Q11 (subscription+metered) | 19 · ✅ clean | **REAL:** Recorder aggregation/windowing, QuotaPolicy check-and-reserve, Plan/Biller (no float money). **Pending:** durable persistence (real Postgres) behind in-memory stores |
| 10 | **asset_service** · `digital.vasic.assetservice` | Store/secure/serve bytes: content-addressed, encrypted-at-rest, RBAC, Range | `[BUILD-NEW: Asset Service]` | 34 · ✅ clean | **REAL:** ContentStore (sha256/integrity/dedup), AES-256-GCM EncryptedStore, Local/HTTP source, Range/206 handler, RBAC resolver, web renditions. **STUB:** FTP/SMB/NFS/WebDAV → `ErrNotImplemented`. **Out of core:** HLS/DASH transcoder `[OPEN: ASSET-2]`, durable asset-index SoR |
| 11 | **max_adapter** · `digital.vasic.maxadapter` | Max/OneMe messenger adapter: history payload → `[]Post` | `[GAP: 5.1.2]` | 10 fn / 19 cases · ✅ clean | **REAL:** OneMe opcode-49 history → `[]Post` mapping (`ParseHistory`). **STUB:** live WebSocket connect/auth/fetch → `ErrNotImplemented`; protocol reverse-engineered, unverified on-wire |
| 12 | **telegram_adapter** · `digital.vasic.telegramadapter` | Telegram thread-history adapter: `TGMessage` → `[]Post` | `[GAP: 5.1.1]` | 12 fn / 24 cases · ✅ clean | **REAL:** `MapMessages` (`TGMessage`→`Post`, reply/forward/media/forum-topic). **STUB:** live gotd/td MTProto connect/auth/fetch → `ErrNotImplemented` (promote Herald's `qaherald/internal/mtproto`) |
| 13 | **metube_webhook** · `digital.vasic.metubewebhook` | Outbound completion-webhook shim over MeTube's poll-only API | `[GAP: 6.5]` | 21 · ✅ clean | **REAL:** ParseJobs, real HTTP poll source, terminal-transition detect, dedup, HMAC-signed retrying WebhookSink, Poller. **Pending:** native webhook in MeTube upstream (separate change); live edge = a running MeTube |
| 14 | **rest_gateway** · `digital.vasic.restgateway` | `/v1` HTTP surface: routing, JWT, RBAC, idempotency, SSE, error envelope | `[BUILD-NEW]` (api/`openapi.yaml`) | 19 · ✅ clean | **REAL:** router, JWT HS256 (alg-pin), RBAC floor+scopes, Idempotency-Key, SSE, error envelope — over **in-memory** Services. **Pending:** wire Services to sibling modules (`go.work` step); prod JWT is RS256/EdDSA via JWKS; TOTP seeded; rate-limit out of scope |
| 15 | **boba_adapter** · `digital.vasic.bobaadapter` | Normalize Boba's bespoke SSE/hook callbacks → the one shared HMAC completion envelope | `[GAP: 6.4]` | 26 fn / 28 cases · ✅ clean | **REAL:** `ParseSSE`/`ParseHookPayload` → normalized `BobaEvent`; shared `{job_id,state,progress,result_ref,error,ts}` envelope **byte-identical to `metube_webhook`**; `X-Thready-Signature` HMAC-SHA256 over raw body; per-result dedup (terminal-only); real `net/http` `SSEReader` + `HTTPHookRegistrar`; `WebhookSink` retry/back-off; full SSE→webhook chain. **Caveat:** Boba's JSON **field names** are `[inferred]` (lenient parser, one-line per field to correct), unverified on a live Boba build |
| 16 | **config** · `digital.vasic.threadyconfig` | Typed, validated loader over the documented env-var reference; secret redaction | `[BUILD-NEW]` (configuration.md App. A) | 22 · ✅ clean · 89.7% cov | **REAL:** typed `Config` reading **162/162** documented vars (100% read; ~95 format-validated), aggregated `*MultiError`, env-sensitive defaults, required-in-prod + backend-conditional checks, secret redaction, `ParseDotEnv`. **Deferred (disclosed):** no `${VAR}` interpolation, cron *syntax* unparsed, 22 credential vars held in secret maps (read+redacted, not each format-checked) |
| 17 | **sdk_go** · `digital.vasic.threadysdk` | Typed, stdlib-only Go client for the gateway's `/v1` control API | `[BUILD-NEW]` (api/`openapi.yaml`) | 18 · ✅ clean · 78.5% cov | **REAL:** typed `/v1` client (login, channels, threads, posts, reprocess, search, skills, SSE `SubscribeEvents`), auth injection (`Bearer` / `X-API-Key`), typed `*APIError` mapping, `Idempotency-Key` on unsafe POSTs, transparent idempotent-GET retries. **Scope:** tested against an `httptest` **contract-mock** of `/v1` (the correct SDK unit-test strategy), not a live gateway |
| — | **integration** · `thready.integration` | `go.work` capstone: end-to-end pipeline composing the 14 workspace modules | capstone (`go.work`) | 4 · ✅ **GREEN** `-race` | **REAL:** composes the **actual** fourteen modules (imported via `../go.work`, not re-stubbed) into the real Thready pipeline — ingest→assemble→dispatch (idempotent claim)→download+store→OCR→index/search→events→HMAC callback→metering. `TestThreadyPipelineEndToEnd` + 3 compose tests. **No committed module required a change; zero cross-module defects** |

> Test counts are the final headline number from each `EVIDENCE.md`. Where a suite
> uses table subtests, both the top-level function count and the total case count
> are shown (`fn / cases`). ✅ = suite green under Go's race detector (`-race`).

### 1.2 Per-module REAL vs honest-stub detail

1. **download_manager** `[GAP: 6.3]` — Real, race-clean HTTP(S) fetcher (probe →
   parallel `Range` GETs reassembled via `WriteAt` → per-segment resume from
   `<dest>.dlstate` → SHA-256 verify → atomic rename) and a real `Manager` (bounded
   worker pool, state machine, exponential backoff + full jitter, progress/completion
   callbacks, pause/resume). A fixed Enqueue/Shutdown TOCTOU race is guarded by two
   regression tests (survives `-count=20`). FTP/SMB/NFS/WebDav are honest
   `ErrNotImplemented` stubs — the documented reuse seam for `digital.vasic.filesystem`.

2. **callback_task** `[GAP: 6.4]`/`[GAP: 6.5]` (ATM-030) — Real task state machine
   (`queued→running→succeeded|retrying|failed|dead`), idempotent completion, retry to a
   ceiling then dead-letter, and an HMAC-SHA256 (`X-Thready-Signature`) webhook sink;
   tests use a real `httptest` receiver that **independently** recomputes the HMAC.
   Delivery is synchronous by design (documented); exhausted deliveries are captured in
   `DeliveryFailures()`, never swallowed.

3. **threadreader** `[GAP: 5.1.3]` — Real, deterministic, stdlib-pure assembler: dedupe
   by ID, root detection with tie-break, drop the system/bot's own replies (keep other
   humans/bots), chronological ordering, unicode-aware hashtag union across the chain.
   The live edges are separate modules (#11, #12) that implement `MessageSource`; the
   in-repo source is an in-memory fake.

4. **ocr_adapter** `[GAP: 2.6]` — Real OCR by shelling out to the installed
   `tesseract 5.3.0` (FullText + TSV word regions). The `Hybrid` orchestrator's
   `LLMVisionProvider` second pass is a declared interface only — `[BUILD-NEW]`,
   deliberately unimplemented, invoked only if a genuine secondary is supplied (none
   ships). It drives the external `tesseract` binary, but that is orthogonal to Go's
   race detector: the suite is **race-clean**, 13/13 green under `-race` (see
   `ocr_adapter/EVIDENCE.md` §8).

5. **user_service** `[BUILD-NEW]` — Real crypto core: PBKDF2-HMAC-SHA256 passwords,
   JWT HS256+RS256 with rotation/revocation and algorithm pinning (rejects `alg:none`
   and HS/RS confusion), scoped/masked API keys, three-tier tenant-isolated RBAC, and
   RFC 6238/4226 TOTP validated against **published known-answer vectors**. A
   refresh-token replay race is fixed with an atomic compare-and-revoke. Not yet a wired
   HTTP service (no REST/JWKS/DB/OAuth2); Argon2id is substituted by the stdlib PBKDF2.

6. **event_bus_service** `[BUILD-NEW]` — Real in-process engine: NATS-style glob +
   metadata filters, sticky last-value per subject with explicit invalidation, durable
   subscribers that replay from a `Seq` cursor (at-least-once; consumers dedupe on
   `Event.ID`), FIFO async publish, metrics. NATS JetStream is the out-of-scope durable
   transport this core deliberately mirrors; the whole log is retained for process
   lifetime (no eviction window).

7. **semantic_search** `[GAP: 2.1]` — Real chunk→embed→cosine-KNN→boost→floor→merge→rank
   pipeline proven with a genuine (not opaque) feature-hashing embedder, including a
   captured ranking where the source-boost/test-penalty actually flips the order. The
   production embedder (`OpenAICompatEmbedder` → llama.cpp/HelixLLM `/v1/embeddings`,
   `HELIX_EMBEDDING_PROVIDER=llama`) is implemented as a real HTTP client but **not**
   integration-tested (no live server); a pgvector `VectorIndex` is the production store
   behind the same seam.

8. **skill_dispatch** `[GAP: 4.1]` — Real execution layer over the HelixSkills DAG:
   additive hashtag/content-type resolution, deterministic stage precedence
   (download→convert→analyze→research→reply), exactly-once single-claim proven under
   `-race` with atomic call counts, per-step retry/backoff then dead-letter, ordered
   event stream. In-memory `ClaimRegistry`/`EventSink` model the production
   Postgres/BackgroundTasks + event-bus substrates; concrete Skills (Download/OCR/LLM)
   plug in.

9. **metering** `[OPERATOR]` (Q11, subscription + metered) — Real Recorder
   (per-account/metric buckets, half-open period windowing), QuotaPolicy with an atomic
   check-and-reserve proven never to overshoot under 200–300 concurrent goroutines, and
   deterministic integer-cent billing (base + per-metric block overage, no float touches
   money) verified against an exact worked invoice. No live stub — the whole core is
   real; production would back the in-memory stores with durable storage.

10. **asset_service** `[BUILD-NEW]` — Real content-addressed store (sha256 id,
    integrity-verify-on-read, dedup), AES-256-GCM encryption at rest with genuine
    wrong-key/tamper rejection, `LocalSource`/`HTTPSource`, `http.ServeContent`
    Range/206 serving, deny-by-default RBAC resolver that never leaks a filesystem path,
    and `…-web` renditions. FTP/SMB/NFS/WebDAV are `ErrNotImplemented` stubs; HLS/DASH
    transcoding is explicitly out of core scope `[OPEN: ASSET-2]`; the asset index is
    in-memory.

11. **max_adapter** `[GAP: 5.1.2]` — Real, offline-tested OneMe opcode-49 history →
    `[]Post` mapper (numeric-or-string 64-bit-exact ids, reply/forward linkage,
    epoch-ms→Unix-s, `_type`-discriminated attachments; three envelope shapes). The live
    WebSocket connect/auth/fetch is a `[BUILD-NEW]` `ErrNotImplemented` stub — no Max
    account exists here and the protocol is reverse-engineered and unverified on-wire
    (`PROTOCOL.md` marks CONFIRMED vs INFERRED).

12. **telegram_adapter** `[GAP: 5.1.1]` — Real `MapMessages` (`TGMessage`→`Post`: ids,
    channel-as-author for broadcasts, verbatim hashtag text, reply-to-reply parent
    linkage, forward flag, media→attachment with MIME inference, forum-topic/`getReplies`
    grouping). A local `TGMessage` mirrors gotd/td's `tg.Message` so the mapper is
    stdlib-only and offline-testable; live MTProto connect/auth/fetch is an
    `ErrNotImplemented` stub pending promotion of Herald's `qaherald/internal/mtproto`
    client (no `api_id`/`api_hash`/session here).

13. **metube_webhook** `[GAP: 6.5]` — Real shim that polls MeTube's poll-only
    `/api/postprocess/jobs`, detects terminal transitions (`finished→success`,
    `error→failure`), de-duplicates per job id, and fires exactly one HMAC-signed
    completion webhook (canonical `callback_task` envelope) with retry/back-off; a real
    `httptest` MeTube-mock exercises the full chain. Adding a native webhook to MeTube
    upstream is a separate change.

14. **rest_gateway** `[BUILD-NEW]` — Real `/v1` HTTP surface end-to-end via `httptest`:
    request-id → access log → panic recovery → JWT bearer (HS256, algorithm-pinned) →
    RBAC role floor → scope enforcement → Idempotency-Key, plus a consistent JSON error
    envelope and a real SSE round-trip. The injected `Services` are honest in-memory
    implementations; wiring them to the sibling domain modules is the later `go.work`
    step. Production JWT is RS256/EdDSA-via-JWKS (HS256 here for self-testability), TOTP
    is a seeded fixed code, and rate limiting lives in a separate edge module.

15. **boba_adapter** `[GAP: 6.4]` — Boba-Base (`milos85vasic/Boba-Base`) *already*
    emits callbacks (an SSE `result_found` stream + a `POST /api/v1/hooks` registration),
    but its shape is bespoke. This module does **not** add callbacks to Boba — it
    **normalizes** its existing terminal events into the one shared Helix Thready
    completion envelope `{job_id,state,progress,result_ref,error,ts}` (**byte-identical to
    `metube_webhook`**), signs it with `X-Thready-Signature: sha256=<hex>` (HMAC-SHA256
    over the exact raw body), dedups per result id, and fires it via a retrying
    `WebhookSink`. Real `net/http` `SSEReader` + `HTTPHookRegistrar`; a full
    `httptest` SSE→webhook chain whose receiver **independently recomputes** the HMAC.
    Honest caveat: Boba's callbacks are **verified to exist** but their concrete JSON
    **field names** are `[inferred]` from its torrent-search domain (the parser is
    lenient — several accepted spellings per field — so aligning to Boba's exact keys is
    a one-line change that never touches normalize/sign/deliver).

16. **config** `[BUILD-NEW]` — The typed, validated configuration loader; the single
    grounding point for `configuration.md` Appendix A (the master env-var index of
    **162** variables). `Load(getenv)` reads **162/162** documented vars into a
    subsystem-grouped `Config`, applies the documented (environment-sensitive) defaults,
    and returns **every** problem at once as a single `*MultiError` (~95 vars are
    format-validated: enums, ints/floats, durations, URL-shapes, bools; production and
    backend-conditional requirements enforced; JWT-secret / AES-key strength checked).
    `Config.String()`/`Redacted()` **mask every secret** (tokens, keys, peppers, the
    encryption key, credential-bearing DSNs/URLs, and the cloud/vision key maps).
    Honest scope: `ParseDotEnv` returns values verbatim (no `${VAR}` interpolation), cron
    *syntax* is stored-not-parsed, and the 22 credential vars live in secret maps (read +
    redacted, not each individually format-checked). 22/22 tests, 89.7% coverage,
    race-clean.

17. **sdk_go** `[BUILD-NEW]` — A typed, stdlib-only Go client (`package thready`) for the
    gateway's `/v1` control API. Typed request/response DTOs, auth injection
    (`Authorization: Bearer …` after `Login`/access token, or `X-API-Key`, mutually
    exclusive), a canonical `{"error":{…}}`→`*APIError` mapping recovered via
    `errors.As`, an auto/overridable `Idempotency-Key` on unsafe POSTs, transparent
    capped-backoff retries for idempotent GETs (POSTs never retried), and an SSE
    `SubscribeEvents`. Correct-for-an-SDK test strategy: exercised against a
    `net/http/httptest` **contract-mock** of `/v1` (asserting the exact method/path/
    headers/body it sends and the typed value it decodes), **not** the live gateway.
    18/18 race-enabled tests, 78.5% coverage.

18. **integration** (`thready.integration`, capstone) — The proof that the fourteen
    workspace modules **compose into the real end-to-end Thready pipeline** using the
    *actual* modules (imported via the parent `../go.work`, `replace`-pinned to local
    paths so the graph also resolves offline), not re-stubbed copies.
    `TestThreadyPipelineEndToEnd` drives a realistic thread through real crypto/OCR/HTTP/
    cosine/events/metering: `telegram_adapter`→`threadreader` (assemble + hashtags) →
    `skill_dispatch` (precedence order + idempotent single-claim: a duplicate `Process`
    runs each Skill exactly once) → `download_manager` (sha256-verified fetch) →
    `asset_service` (content-addressed, tamper-caught) → `ocr_adapter` (real `tesseract`)
    → `semantic_search` (real cosine retrieves the OCR chunk) → `event_bus_service`
    (9 events replayed in order) → `callback_task` (HMAC independently recomputed) →
    `metering` (invoice line asserted). Plus three compose tests
    (`max_adapter`→assembler, `metube_webhook` completion, `rest_gateway` login
    round-trip). **No committed module required a change**; the only friction was a
    documented toolchain note (`go build ./...` at the non-module workspace root). 4/4
    PASS under `-race -count=1`, stable under `-count=3`.

---

### 1.3 Wave-2 additions

Landed after the initial 17-module wave — same discipline (TDD, independent anti-bluff
review, own `EVIDENCE.md`, committed + pushed to GitHub/GitLab/GitVerse). All numbers
below are re-verified in [`QUALITY_GATE.md`](./QUALITY_GATE.md) (Go) and
[`sdk/CONFORMANCE.md`](./sdk/CONFORMANCE.md) (SDKs).

**Two more Go modules** (in the race-clean `QUALITY_GATE.md` sweep):

| Module · path | Purpose | Tests · `-race` | REAL vs stub |
|---|---|---|---|
| **processing** · `digital.vasic.processing` | Shippable Processing-Engine orchestrator: claim (exactly-once) → resolve+order by precedence → run each w/ retry+backoff → dead-step on exhaustion → per-step events → completion callback | 17 · ✅ | **REAL:** full orchestrator over injected seams (Claimer/SkillSet/Skill/EventEmitter/Callbacker) whose shapes match the committed contracts (Post/Kind/RetryPolicy/`callback_task.Envelope`); real atomic `MemoryClaimer` exactly-once proven under 64 concurrent goroutines. Imports **no** siblings — composes at the seam. |
| **cli** · `digital.vasic.threadycli` | Headless `/v1` CLI over `sdk_go` (login, channels, posts, reprocess, search, skills) | 25 · ✅ | **REAL:** flag parsing, command dispatch, JSON/table output, exit codes, over the real `sdk_go` client (`replace ../sdk_go`). Password read from `THREADY_PASSWORD` (off argv); `--password` warns. |

**`deployment_smoke`** (not a Go module) — `smoke.sh` builds the `rest_gateway` binary,
serves `/v1` on a free port, and asserts **real HTTP**: health `200` · unauth `401` ·
login `200`+token · authed `200` — **12/12 checks PASS**, corroborated by the server's
structured JSON access log. A rootless-Podman `Containerfile` + `podman_smoke.sh` are
provided; that container leg is honestly **DEFERRED** (offline base-image pull) — never faked.

**`server`** (`thready.server`, a workspace-composition module like `integration`) — the
runnable assembly that serves the gateway's `/v1` over the **real domain modules** instead
of in-memory stubs: `AuthService`→`user_service` (real PBKDF2 + RFC 6238 TOTP),
`SearchService`→`semantic_search` (real cosine-KNN), `SkillService`→`skill_dispatch` (real
registry/precedence), `EventService`→`event_bus_service` (real pub/sub); `Channels`/`Accounts`
stay honest in-memory CRUD (no domain module). `cmd/thready-server` runs it on `$PORT` with
graceful shutdown. **4 e2e tests green under `-race`** prove genuine behavior end-to-end: a
real-PBKDF2 login (wrong password *and* wrong TOTP → 401 through the real verifiers), real
cosine ranking (a vector-DB query ranks `vectordb.md` top, a disjoint telegram query ranks
`telegram.md` top — a negative control), and real skill precedence order. Honest note: because
the gateway's coded-error type is unexported, a *missing*-post reprocess surfaces as 500 (not
404) — the tested real paths are unaffected (disclosed in `server/EVIDENCE.md`). This is the
concrete realization of the "wire the gateway to the real modules" next-step named above.

**Six-language SDK set** — one `/v1` client per language, each self-contained with its
own native test runner. The [conformance matrix](./sdk/CONFORMANCE.md) proves all six
issue the **same** method+path for every operation (**42/42** cells) and share the same
four behaviors (**24/24**): bearer-wins auth, `Idempotency-Key` on unsafe POSTs, idempotent-GET
retry on 503/429, and the credential-over-cleartext-http guard.

| SDK · dir | Runtime | Dependencies | Tests (real) |
|---|---|---|---|
| **Go** · `sdk_go` | Go 1.26 | stdlib | 20 · `go test -race` ✅ |
| **Python** · `sdk_py` | Python 3.13 | stdlib (urllib/json) | 29 · `unittest` ✅ |
| **TypeScript** · `sdk_ts` | Node 24 | Node built-ins (no npm) | 24 · `node:test` ✅ |
| **Java** · `sdk_java` | JDK 21 | stdlib (no jars / build tool) | 20 · JUnit-free runner ✅ |
| **Rust** · `sdk_rs` | rustc 1.96 | `std` only (no cargo/crates) | 17 · `rustc --test` ✅ |
| **Ruby** · `sdk_rb` | Ruby 3.3 | core stdlib (no gems) | 27 (88 asserts) · stdlib harness ✅ |

Kotlin / Swift / .NET / Dart SDKs are **deferred, not stubbed**: their toolchains are not
installable in this offline environment, and shipping a client that cannot be compiled-and-tested
here would be exactly the bluff the mandate forbids. They land when their toolchains are available.

---

## 2. Status summary

- **Total modules:** **19 standalone** Go modules (`digital.vasic.<X>`, `go 1.26`) **plus
  the `integration` capstone and the `server` assembly** — **21 Go `go.mod` in all** — alongside
  **5 additional-language SDKs** (`sdk_py`/`sdk_ts`/`sdk_java`/`sdk_rs`/`sdk_rb`) and the
  `deployment_smoke` HTTP proof. (Wave-1 = rows 1–17 above; the wave-2 Go modules — `processing`,
  `cli`, and the real-module-backed `server` — and the polyglot SDKs are in [§1.3](#13-wave-2-additions).)
- **Total tests:** **378 Go test functions across the 21 Go modules, 0 failures, all
  race-clean** — the 20-module sweep in [`QUALITY_GATE.md`](./QUALITY_GATE.md) (374) plus the
  `server` assembly's 4 e2e (verified separately, race-green) — **plus 117 tests across the 5
  non-Go SDKs** (Python 29 · TS 24 · Java 20 · Rust 17 · Ruby 27, all green in their native
  runners — [`sdk/CONFORMANCE.md`](./sdk/CONFORMANCE.md)), for **495 automated tests total**,
  plus `deployment_smoke`'s **12/12** real-HTTP checks. Several Go suites layer table subtests
  on top (e.g. `boba_adapter` 26 fn / 28 cases).
- **All stdlib-only:** every standalone `go.mod` has **no `require` block** — zero
  third-party Go dependencies. The only `require` anywhere is `integration/go.mod`, and
  its requires are the **in-house sibling modules** it composes (pinned to local paths via
  `replace`, so the graph resolves offline — still no third-party). External *binaries*
  are used only where unavoidable (`ocr_adapter` shells out to `tesseract`; its tests also
  use ImageMagick to synthesize images).
- **Green under `-race`:** **19 of 19** standalone Go module suites (the wave-1 17 +
  `processing` + `cli`) run and pass under Go's race detector with `-count=1`, and the
  **`integration` capstone is GREEN under `-race`** (stable under `-count=3`); the whole
  set was re-swept together in [`QUALITY_GATE.md`](./QUALITY_GATE.md). This **includes
  `ocr_adapter`**: although it drives the
  external `tesseract` process, that is orthogonal to Go's race detector, and its suite is
  race-clean (13/13 under `-race`; captured in `ocr_adapter/EVIDENCE.md` §8). Several
  modules (download_manager, user_service, event_bus_service) additionally survive
  `-count=5..20` stability reruns.
- **`go vet` / `gofmt`:** clean across all modules (each `EVIDENCE.md` captures it).
- **EVIDENCE:** **18 `EVIDENCE.md`** files — one per standalone module plus the capstone —
  each pasting verbatim, reproducible `build`/`vet`/`gofmt`/`test` output with an honest
  verdict.
- **`go.work`:** a committed workspace (force-added, since `go.work` is normally
  gitignored) lists **fourteen** modules + `./integration` and ties them together for the
  capstone. The three newest modules (`boba_adapter`, `config`, `sdk_go`) are **not** in
  the workspace and build standalone with `GOWORK=off`.
- **Committed + pushed:** all modules are committed and pushed to **GitHub, GitLab,
  and GitVerse**. GitFlic is blocked — see [§6](#6-upstream-status).

## 3. What's real vs pending

The **offline, deterministic cores are real and tested**. The **live edges** are honest
`[BUILD-NEW]` seams (interface + `ErrNotImplemented`, or a built-but-not-integration-tested
client) that are pending real credentials/services and are never faked:

- **Telegram gotd/td MTProto reads** — `telegram_adapter` live client (`[GAP: 5.1.1]`);
  needs `api_id`/`api_hash` + a login session; lands by promoting Herald's
  `qaherald/internal/mtproto`.
- **Max OneMe WebSocket** — `max_adapter` live client (`[GAP: 5.1.2]`); needs a Max
  account; protocol reverse-engineered and unverified on-wire.
- **llama.cpp / HelixLLM embeddings** — `semantic_search` `OpenAICompatEmbedder`
  (`[GAP: 2.1]`); built as a real HTTP client but needs a running HelixLLM with
  `HELIX_EMBEDDING_PROVIDER=llama` (a real embedding GGUF, not the default `HashEmbedder`).
- **pgvector** — the production `VectorIndex` behind `semantic_search` (in-memory
  cosine-KNN today).
- **Real Postgres** — durable backing for `metering` (Recorder/QuotaPolicy),
  `skill_dispatch` (durable claim), and `user_service` (token/API-key stores), all
  in-memory today.
- **Real NATS (JetStream)** — the durable transport `event_bus_service` mirrors as an
  adapter seam.
- **FTP / SMB / NFS / WebDav** — the `ErrNotImplemented` file-source stubs in
  `download_manager` and `asset_service`, the reuse seam for `digital.vasic.filesystem`.
- **A running MeTube** — the live poll target for `metube_webhook` (the shim and its
  HTTP source are real and tested against a mock).
- **Boba's exact callback keys** — `boba_adapter` already parses, normalizes, signs and
  delivers Boba's callbacks, but Boba's concrete JSON **field names** are `[inferred]` (a
  lenient parser accepts several spellings per field); confirming them against a live Boba
  build is a one-line-per-field change that never touches the normalize/sign/deliver logic.
- **A live gateway for the SDK** — `sdk_go` is verified against an `httptest`
  contract-mock of `/v1` (the correct SDK unit-test strategy), not the running
  `rest_gateway` process; `config` has no pending live edge (its core is fully real).
- **Service wiring** — `rest_gateway` serves over in-memory `Services`; the committed
  `go.work` + `integration` capstone now demonstrate the real modules composing end to
  end, but wiring the gateway's own injected `Services` to the sibling domain modules in
  production remains the next step.

Also deliberately out of scope for this wave (documented, not claimed): OCR LLM-vision
second pass, Asset Service HLS/DASH transcoder `[OPEN: ASSET-2]`, User Service
REST/JWKS/OAuth2, and edge rate limiting.

## 4. How to run

Each module is self-contained — run its suite from its own directory:

```bash
cd implementation/<module>
go build ./... && go vet ./... && gofmt -l . && go test ./... -race -count=1
```

Example:

```bash
cd implementation/asset_service && go test ./... -race -count=1
```

**`GOWORK=off` for the three newest modules.** `boba_adapter`, `config` and `sdk_go` are
**not** listed in the committed `go.work`, so every Go command for them must disable the
workspace:

```bash
cd implementation/config && GOWORK=off go test ./... -race -count=1
```

**ocr_adapter tooling** — it drives the `tesseract` binary, so its tests require
`tesseract` and, for the synthesized test images, ImageMagick `convert` on `PATH`.
Driving an external process is orthogonal to Go's race detector, so the suite is
race-clean and `-race` applies as for every other module:

```bash
cd implementation/ocr_adapter && go test ./... -race -count=1
```

**go.work integration workspace (committed).** A `go.work` workspace **is now committed**
(force-added — `go.work` is normally gitignored) listing the fourteen composable modules
+ `./integration`; it ties them together so the `integration` capstone imports the *real*
modules. Run the capstone from the workspace root:

```bash
cd implementation            # the go.work workspace root
go work sync
cd integration && go test ./... -race -count=1
```

`integration/go.mod` `replace`-pins its fourteen siblings to local paths, so the module
graph also resolves offline. This is the concrete mechanism that will later wire
`rest_gateway`'s injected `Services` interfaces to the real domain modules
(`user_service`, `threadreader`, `semantic_search`, `skill_dispatch`, `asset_service`,
`event_bus_service`, `metering`) instead of the in-memory implementations (see
`integration/EVIDENCE.md`).

## 5. Promotion plan

Per `[CONSTITUTION §11.4.28]` (decoupled submodules) and `[CONSTITUTION §2.1]` (four
upstreams), each module graduates from this monorepo staging area to **its own
repository** under `vasic-digital` / `HelixDevelopment`:

- One repo per `digital.vasic.<X>` module, carrying its `README.md` + `EVIDENCE.md`.
- **`upstreams` recipes** wire each new repo to all four remotes (GitHub / GitLab /
  GitFlic / GitVerse) so a single push fans out, matching the org's mirror-everywhere
  policy (`§2.1`).
- Project-prefixed release tags (`§11.4.151`) and no server-side CI (`§11.4.156`) apply
  as for every Helix repo.
- The **decoupled / project-agnostic contract is preserved**: each of the **17
  standalone** modules imports no in-house peers today (only documented reuse *seams*), so
  promotion is a move, not a rewrite. Consumers depend on the promoted module path, and
  the `go.work` workspace stitches them during integration and local development. The
  `integration` capstone is the workspace-only glue that composes real siblings (its
  `go.mod` `require`s them via local `replace`); it is not itself promoted.

## 6. Upstream status

- **GitHub, GitLab, GitVerse** — ✅ committed and pushed; the implementation wave is
  present on all three (`origin` fans out to them).
- **GitFlic** — ⛔ **blocked**. A peer-committed design asset,
  `docs/public/research/mvp/design/exports/design-book.pdf` (**≈56 MB / 55.88 MiB**, the
  ~132 MiB pack), **exceeds GitFlic's server-side push size limit**, so the push to
  `git@gitflic.ru:helixdevelopment/helix_thready.git` is rejected. This is **operator
  action needed** — resolve on the GitFlic side (raise the limit / LFS / history
  surgery for that blob); it does not affect the code, which is intact on the other three
  upstreams.

---

*Made with love ♥ by Helix Development.*
