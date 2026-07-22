<!--
  Title           : Helix Thready — Implementation Phase Index
  Classification  : PUBLIC
  Location        : implementation/README.md
  Status          : Active — v1.0
  Revision        : 1 (2026-07-22)
  Author          : Helix Thready documentation swarm (implementation)
  Related         : ../docs/public/research/mvp/index.md · ../docs/public/research/mvp/CONVENTIONS.md · ../docs/private/research/mvp/helix_thready_subsystem_gaps_and_improvements.md
-->

# Helix Thready — Implementation Phase Index

This directory stages the **first wave of buildable code** for Helix Thready: 14
self-contained, project-agnostic **Go modules**, each under `implementation/<name>/`
with module path `digital.vasic.<X>`, staged here as a monorepo. Each module is a
standalone Go module (its own `go.mod`, standard-library only) intended to be
**promoted later to its own repository** under `vasic-digital` / `HelixDevelopment`
per `[CONSTITUTION §11.4.28]` (decoupled submodules).

Every module ships a `README.md` (contract + API) and an `EVIDENCE.md` (verbatim,
reproducible `go build`/`go vet`/`gofmt`/`go test` capture with an honest verdict).
This index summarizes each and is grounded entirely in what those files state — the
REAL-vs-stub column below is deliberately conservative.

| Rev | Date | Author | Change |
|-----|------|--------|--------|
| 1 | 2026-07-22 | swarm (implementation) | Initial index over the 14-module implementation wave |

## Table of contents

- [1. The 14 modules](#1-the-14-modules)
  - [1.1 Summary table](#11-summary-table)
  - [1.2 Per-module REAL vs honest-stub detail](#12-per-module-real-vs-honest-stub-detail)
- [2. Status summary](#2-status-summary)
- [3. What's real vs pending](#3-whats-real-vs-pending)
- [4. How to run](#4-how-to-run)
- [5. Promotion plan](#5-promotion-plan)
- [6. Upstream status](#6-upstream-status)

Provenance tags used below: `[CONSTITUTION §x]` authoritative · `[BUILD-NEW]`
confirmed new-submodule gap · `[GAP: id]` addresses a gap-register item ·
`[OPERATOR]` operator decision · `[OPEN: id]` tracked-open item.

---

## 1. The 14 modules

### 1.1 Summary table

| # | Module · path | Purpose (one line) | Closes | Tests · `-race` | REAL vs honest `[BUILD-NEW]` stub |
|---|---------------|--------------------|--------|-----------------|-----------------------------------|
| 1 | **download_manager** · `digital.vasic.downloadmanager` | Multi-protocol download manager: job queue, worker pool, retry, progress/completion callbacks | `[GAP: 6.3]` | 14 · ✅ clean | **REAL:** HTTP(S) segmented + resumable + SHA-256 download; Manager (queue/state/retry/pause-resume). **STUB:** FTP/SMB/NFS/WebDav → `ErrNotImplemented` |
| 2 | **callback_task** · `digital.vasic.callbacktask` | One canonical async-job contract (accept→run→status→HMAC webhook→retry→dead-letter) | `[GAP: 6.4]`/`[GAP: 6.5]` (ATM-030) | 16 · ✅ clean | **REAL:** task state machine, retry/backoff, work + delivery dead-lettering, HMAC-SHA256 webhook. **Pending:** non-blocking async delivery wrapper is the caller's (documented) |
| 3 | **threadreader** · `digital.vasic.threadreader` | Messenger-agnostic thread assembly: root + organic replies, hashtag union | `[GAP: 5.1.3]` | 11 fn / 29 cases · ✅ clean | **REAL:** assembly/system-filter/hashtag pure core. **Pending:** live Telegram/Max sources (`MessageSource` is an in-memory fake here) |
| 4 | **ocr_adapter** · `digital.vasic.ocr` | OCR engine for VisionEngine via the real `tesseract` CLI (hybrid seam) | `[GAP: 2.6]` | 13 · ⚠️ **no `-race`** | **REAL:** `TesseractProvider` (real tesseract 5.3.0 shell-out). **STUB:** `LLMVisionProvider` `[BUILD-NEW]` interface only, never faked |
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

> Test counts are the final headline number from each `EVIDENCE.md`. Where a suite
> uses table subtests, both the top-level function count and the total case count
> are shown (`fn / cases`). ✅ = suite green under Go's race detector (`-race`);
> ⚠️ = see the ocr_adapter note in [§2](#2-status-summary).

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
   ships). **Note:** this is the one suite whose `EVIDENCE.md` runs `go test -count=1`
   **without** `-race` (it drives an external binary); 13/13 green.

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

---

## 2. Status summary

- **Total modules:** 14, each an independent Go module (`digital.vasic.<X>`, `go 1.26`).
- **Total tests:** **260 top-level test functions** across the suite (**~303 cases**
  counting table subtests), 0 failures. Per module: 14, 16, 11, 13, 36, 13, 19, 23, 19,
  34, 10, 12, 21, 19.
- **All stdlib-only:** every `go.mod` has **no `require` block** — zero third-party Go
  dependencies. External *binaries* are used only where unavoidable (`ocr_adapter`
  shells out to `tesseract`; its tests also use ImageMagick to synthesize images).
- **Green under `-race`:** **13 of 14** module suites run and pass under Go's race
  detector with `-count=1`. **Honest exception:** `ocr_adapter` runs `go test -count=1`
  **without** `-race` because it drives the external `tesseract` process — 13/13 green,
  but not under the race detector. Every other module is race-clean, and several
  (download_manager, user_service, event_bus_service) additionally survive
  `-count=5..20` stability reruns.
- **`go vet` / `gofmt`:** clean across all modules (each `EVIDENCE.md` captures it).
- **Committed + pushed:** all 14 modules are committed and pushed to **GitHub, GitLab,
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
- **Service wiring** — `rest_gateway` serves over in-memory `Services`; connecting them
  to the sibling domain modules is the `go.work` integration step below.

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

**ocr_adapter exception** — omit `-race` (it drives the `tesseract` binary; requires
`tesseract` and, for the test images, ImageMagick `convert` on `PATH`):

```bash
cd implementation/ocr_adapter && go test ./... -count=1
```

**go.work integration workspace.** The modules are decoupled by design (separate
`go.mod` each), so there is **no** committed `go.work` yet. A `go.work` workspace is the
planned mechanism that ties them together for cross-module integration — most concretely,
to wire `rest_gateway`'s injected `Services` interfaces to the real domain modules
(`user_service`, `threadreader`, `semantic_search`, `skill_dispatch`, `asset_service`,
`event_bus_service`, `metering`) instead of the in-memory implementations. That
integration step is the next wave (see `rest_gateway/EVIDENCE.md`).

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
- The **decoupled / project-agnostic contract is preserved**: modules import no in-house
  peers today (only documented reuse *seams*), so promotion is a move, not a rewrite.
  Consumers depend on the promoted module path, and the `go.work` workspace stitches them
  during integration and local development.

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
