<!--
  Title           : Helix Thready — Troubleshooting
  Classification  : PUBLIC
  Location        : docs/public/research/mvp/user-guides/troubleshooting.md
  Status          : Draft — v0.1 (zero-version)
  Revision        : 1 (2026-07-21)
  Author          : Helix Thready documentation swarm (user-guides)
  Related         : ./configuration.md, ./installation.md, ./end-user-manual.md, ./faq.md
-->

# Helix Thready — Troubleshooting

| Rev | Date | Author | Change |
|-----|------|--------|--------|
| 1 | 2026-07-21 | swarm (user-guides) | Initial symptom → cause → fix guide |

Symptom-first troubleshooting. Many entries reference **known scaffold traps** from the
[gap register](../../../../private/research/mvp/helix_thready_subsystem_gaps_and_improvements.md) — these
are not bugs in your setup, they are documented `[BUILD-NEW]`/stub statuses you should recognize.
Start with `thready doctor` ([cli-reference.md §5.8](./cli-reference.md#58-doctor--diagnostics)).

## Table of contents

1. [Post-processing triage (diagram)](#1-post-processing-triage-diagram)
2. [Service won't start](#2-service-wont-start)
3. [Can't sign in to a messenger](#3-cant-sign-in-to-a-messenger)
4. [Posts never get processed](#4-posts-never-get-processed)
5. [Semantic search returns irrelevant results](#5-semantic-search-returns-irrelevant-results)
6. [A download never completes](#6-a-download-never-completes)
7. [Comic/screenshot text isn't extracted](#7-comicscreenshot-text-isnt-extracted)
8. [Auth / token / RBAC errors](#8-auth--token--rbac-errors)
9. [Configuration changes not taking effect](#9-configuration-changes-not-taking-effect)
10. [Where the logs are](#10-where-the-logs-are)
11. [Open items](#11-open-items)

## 1. Post-processing triage (diagram)

```mermaid
flowchart TB
  START(["Post not processed / wrong result"]) --> Q1{Status?}
  Q1 -->|queued forever| PAUSED{Processing paused?}
  PAUSED -->|yes| RESUME["Resume: thready processing resume"]
  PAUSED -->|no| WORKERS["Check workers/queue: thready processing status; scale THREADY_WORKERS"]
  Q1 -->|failed| STEP{Which step failed?}
  STEP -->|download| DLGAP["MeTube poll-only [GAP:5] / DL-Manager missing [GAP:4]<br/>→ retry step; check 3rd-party URLs"]
  STEP -->|research/analyze| LLM["LLM/provider down → check HELIX_LLM_* + provider keys; circuit breaker"]
  STEP -->|reply| MSG["Messenger session invalid → thready messenger login telegram"]
  Q1 -->|processed but| WRONG{Symptom?}
  WRONG -->|search irrelevant| EMB["Hash embedder [GAP:1]<br/>set HELIX_EMBEDDING_PROVIDER=llama; restart"]
  WRONG -->|no OCR text| OCR["No OCR engine [GAP:2]<br/>OCR adapter [BUILD-NEW]; LLM-vision fallback only"]
  WRONG -->|missing category| TAGS["Tag in a reply? verify complete-post assembly; reprocess"]
  DLGAP --> DLQ{Still failing after 5 retries?}
  LLM --> DLQ
  MSG --> DLQ
  DLQ -->|yes| INSPECT["Inspect DLQ (Admin) + logs (ClickHouse); open issue"]
  DLQ -->|no| OK([Resolved])
```

> Rendered PNG/SVG exported via Docs Chain (§11.4.65). Source: [diagrams/troubleshoot-post.mmd](./diagrams/troubleshoot-post.mmd).

**Explanation (for readers/models that cannot see the diagram).** The triage starts from a post that
either never processed or produced a wrong result, and branches on its **status**. If it is *queued
forever*, first check whether processing is **paused** (resume it) and otherwise check the worker pool
and queue depth — a starved pool is fixed by raising `THREADY_WORKERS`. If it *failed*, branch on the
**step**: a `download` failure usually traces to the two documented download gaps (MeTube is poll-only
`[GAP:5]`, the generic Download Manager doesn't exist yet `[GAP:4]`) or bad 3rd-party URLs, and is
retried per-step; a `research`/`analyze` failure points at the LLM stack (check `HELIX_LLM_*`, provider
keys, and whether the circuit breaker tripped); a `reply` failure means the messenger session is
invalid (re-login). If it *processed but* looks wrong, branch on symptom: irrelevant **search** is the
classic hash-embedder trap `[GAP:1]` (set `HELIX_EMBEDDING_PROVIDER=llama` and restart); missing **OCR
text** is the no-OCR-engine gap `[GAP:2]` (LLM-vision fallback only until the adapter ships); and a
**missing category** usually means the hashtag lived in a reply and the complete-post assembly or a
reprocess is needed. All failing branches converge on a check: if it still fails after the 5-retry
ceiling it lands in the dead-letter queue for an Admin to inspect alongside the ClickHouse logs;
otherwise the retry resolves it.

## 2. Service won't start

| Symptom | Cause | Fix |
|---------|-------|-----|
| `ABORT: missing required var X` (exit 3) | Required env var absent | Add it to `.env`; run `thready config validate`. |
| `ABORT: non-semantic embedder blocked` | `HELIX_EMBEDDING_PROVIDER=hash` `[GAP: 1]` | Set `=llama` and point `THREADY_EMBEDDING_BASE_URL` at a running llama.cpp. |
| DB connection refused | Postgres/pgvector not up | `podman compose ... up -d postgres`; check `THREADY_DB_DSN`. |
| `pgvector extension not found` | Extension not installed | `CREATE EXTENSION vector;` in the DB. |
| TLS cert error on boot | Let's Encrypt not issued yet | Check `LETS_ENCRYPT_*`; see [deployment](../deployment/index.md). |

The service **fails loudly and lists** what's missing — read the first error line; it names the key.

## 3. Can't sign in to a messenger

`[GAP: 3]` **Know the status first.** Telegram works via the `gotd/td` MTProto user client (being
promoted from the `qaherald` harness). **Max is not implemented** — the adapter is `[BUILD-NEW]`.

| Symptom | Cause | Fix |
|---------|-------|-----|
| `messenger 'max' not implemented` | Max adapter is `[BUILD-NEW]` | Expected in the zero version; use Telegram. Track ATM — Max adapter. |
| Telegram login code never arrives | Wrong `HERALD_TELEGRAM_API_ID/HASH` or phone | Re-check credentials from my.telegram.org. |
| `2FA required` loop | Account has 2FA | Set `HERALD_TELEGRAM_2FA_PASSWORD`. |
| Session lost after restart | Session file not persisted | Set `HERALD_TELEGRAM_SESSION_PATH` to a writable, encrypted path. |
| Can't backfill channel history | Using Bot-API path (can't backfill) | Ensure the MTProto **user** client is active, not a bot token. |

## 4. Posts never get processed

| Symptom | Cause | Fix |
|---------|-------|-----|
| All posts `queued`, none `processing` | Processing paused | `thready processing status`; `thready processing resume`. |
| Slow drain under load | Worker pool too small | Raise `THREADY_WORKERS` (default 32). |
| A post `processing` for very long | Research-heavy soft timeout | `THREADY_POST_TIMEOUT` (default 30 m); long downloads are delegated and shouldn't hold a slot. |
| System replies being re-processed | Should never happen | Only organic posts are processed by design; if you see this, file it — the reply-exclusion invariant is broken. |
| Duplicate processing | Should never happen | Single-claim via Postgres lock guarantees exactly-once; report if observed. |

## 5. Semantic search returns irrelevant results

`[GAP: 1]` **The #1 gotcha.** HelixLLM's default `HashEmbedder` emits non-semantic pseudo-vectors;
search built on it returns garbage with only a startup warning.

**Fix:**
1. `thready doctor embeddings` → if it shows `provider=hash`, that's the cause.
2. Start a real llama.cpp embedding server; set `HELIX_EMBEDDING_PROVIDER=llama` and
   `THREADY_EMBEDDING_BASE_URL`/`THREADY_EMBEDDING_MODEL`.
3. Ensure `THREADY_EMBEDDING_DIM` matches the model (the RAG path historically hardcoded 768 — set it
   explicitly).
4. **Re-embed** existing content: `thready search reindex --all` (embeddings computed on the hash
   provider must be regenerated).
5. Restart; confirm the portal's "degraded" banner clears.

## 6. A download never completes

| Content | Cause | Status |
|---------|-------|--------|
| Video (`#Video`, `#Channel`) | MeTube is **poll-only** `[GAP: 5]` — no completion webhook | Thready polls `GET /api/postprocess/status`; completion notification lags. The download still finishes; the outbound webhook is `[BUILD-NEW]`. |
| Torrent (`#Torrent`) | Boba callback contract bespoke `[GAP: 6.4]` | Boba has SSE + `POST /api/v1/hooks`; if callbacks don't arrive, check `THREADY_BOBA_CALLBACK_URL` reachability. |
| Direct file URL (HTTP/FTP/…) | Download Manager **doesn't exist yet** `[GAP: 4]` | Direct-URL downloads are unavailable until the `[BUILD-NEW]` Download Manager ships. |
| Broken existing asset | Physical link broke | `thready asset reheal <id>` re-downloads. |

Use `thready post retry <id> --step download` to re-drive a stuck download step. Persistent failures
land in the DLQ.

## 7. Comic/screenshot text isn't extracted

`[GAP: 2]` VisionEngine has **no OCR engine**. Comic transcription and screenshot text extraction fall
back to **LLM-vision** (lower fidelity, non-deterministic, no bounding boxes) until the
Tesseract/PaddleOCR adapter `[BUILD-NEW]` lands. `THREADY_OCR_PROVIDER=none` today means "LLM-vision
only". This is expected in the zero version — not a misconfiguration.

## 8. Auth / token / RBAC errors

| Symptom | Cause | Fix |
|---------|-------|-----|
| `401 unauthorized` (exit 4) | Missing/expired token | `thready auth login`, or set `THREADY_TOKEN`. |
| `403 forbidden` (exit 77) | RBAC denies the action | You lack the role/scope; an Admin must grant it. Not a bug — server enforces. |
| Tokens rejected across services | HS256 single-service only `[GAP: 10]` | Move to `THREADY_JWT_SIGNING_ALG=RS256` with a shared public key. |
| MFA loop | TOTP not enrolled (Admin) | Complete TOTP enrolment; admin tiers are forced. |
| API key works but too broad | Over-scoped key | Mint a narrower key: `thready auth token --scopes read:search`. |

## 9. Configuration changes not taking effect

- **Runtime-editable settings** (directories, poll frequency, branding, retention) go
  client → REST → System and apply immediately, emitting `config.changed`. If not, check RBAC and the
  audit log.
- **Secrets are NOT hot-reloaded.** Rotating `THREADY_JWT_SECRET`, provider keys, DB DSN, etc. requires
  a **service restart**. This is deliberate (secrets are read once at boot).
- **`.env` precedence:** a value exported in your shell **overrides** the `.env` file. If a change to
  `.env` "does nothing", check for a stale `export` in `~/.bashrc`/`~/.zshrc`.

## 10. Where the logs are

- **Structured logs** — logrus (JSON in prod) shipped to **ClickHouse** via `observability`
  (`THREADY_CLICKHOUSE_DSN`). Query with the observability tooling; **not** ELK/Loki.
- **Traces** — OpenTelemetry (`OTEL_EXPORTER_OTLP_ENDPOINT`) → Jaeger/Zipkin.
- **Metrics** — Prometheus at `THREADY_METRICS_ADDR` → Grafana.
- **Audit** — `thready audit tail` / `audit query` (append-only).
- **Container logs** — `podman logs <container>` per stack.
- **Secrets are always redacted** in every sink.

## 11. Open items

- `[OPEN: trb-1]` `thready search reindex` depends on the semantic-search service `[GAP register §11]`;
  confirm the exact reindex command once implemented.
- `[OPEN: trb-2]` DLQ inspection UX (portal/CLI surface for stuck posts) to be finalized with the
  BackgroundTasks integration. Tracked: **ATM — DLQ inspection surface**.
- `[OPEN: trb-3]` Several fixes here reference `[BUILD-NEW]` services (Max, Download Manager, OCR,
  MeTube webhook); their operator-facing error messages finalize when those ship.

---

*Made with love ♥ by Helix Development.*
