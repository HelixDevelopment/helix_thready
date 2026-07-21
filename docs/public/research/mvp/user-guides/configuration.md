<!--
  Title           : Helix Thready — Configuration & Environment-Variable Reference
  Classification  : PUBLIC
  Location        : docs/public/research/mvp/user-guides/configuration.md
  Status          : Draft — v0.1 (zero-version)
  Revision        : 1 (2026-07-21)
  Author          : Helix Thready documentation swarm (user-guides)
  Related         : ./installation.md, ./root-admin-guide.md, ./troubleshooting.md,
                    ../deployment/index.md, ../api/index.md
-->

# Helix Thready — Configuration & Environment-Variable Reference

| Rev | Date | Author | Change |
|-----|------|--------|--------|
| 1 | 2026-07-21 | swarm (user-guides) | Complete `.env` reference for the zero version |

This is the **complete, documented environment-variable reference** for Helix Thready, mandated by
the original request: *"All environment variables that system supports MUST be properly documented
in separate dedicated document(s)."* Every variable the system reads is listed here with type,
default, scope, provenance, and whether it is **VERIFIED** (a real in-house module variable read at
source) or an **ASSUMPTION** / `[DEFAULT — adjustable]` (a Thready-specific default proposed here).

> **Naming convention (VERIFIED pattern, Thready names are ASSUMPTION).** In-house modules use a
> module prefix: `HELIX_*` (HelixLLM/VisionEngine — VERIFIED in `vision_engine/.env.example` etc.),
> `HERALD_*` (messengers — VERIFIED reserved names in the gap register), `CONTAINERS_*`
> (deployment — VERIFIED in `containers/.env.example`), and bare provider keys such as
> `ANTHROPIC_API_KEY`, `OPENAI_API_KEY`, `GOOGLE_API_KEY`, `DEEPSEEK_API_KEY` (VERIFIED in
> `llm_provider/.env.example`). Thready's own service variables use the **`THREADY_*`** prefix.
> The `THREADY_*` names are this document's proposal `[DEFAULT — adjustable]`; the reused
> `HELIX_*`/`HERALD_*`/`CONTAINERS_*`/provider names are the actual module variables Thready
> inherits and must not rename.

## Table of contents

1. [How configuration is resolved](#1-how-configuration-is-resolved)
2. [The `.env` file and secret hygiene](#2-the-env-file-and-secret-hygiene)
3. [Quick-start minimal `.env`](#3-quick-start-minimal-env)
4. [Core runtime & server](#4-core-runtime--server)
5. [Environments, domains & TLS](#5-environments-domains--tls)
6. [Relational database](#6-relational-database)
7. [Datastores (vector, cache, object storage)](#7-datastores)
8. [Embeddings, LLM & Vision](#8-embeddings-llm--vision)
9. [Messengers (Herald)](#9-messengers-herald)
10. [Event bus, background jobs & processing](#10-event-bus-background-jobs--processing)
11. [Authentication & security](#11-authentication--security)
12. [Downloads & 3rd-party systems](#12-downloads--3rd-party-systems)
13. [Assets & media directories](#13-assets--media-directories)
14. [Observability, logging & backup](#14-observability-logging--backup)
15. [Retention, billing & localization](#15-retention-billing--localization)
16. [White-labeling & branding](#16-white-labeling--branding)
17. [Precedence, validation & change management](#17-precedence-validation--change-management)
18. [Open items](#18-open-items)

## 1. How configuration is resolved

Per the original request, configuration comes from *"env. variables in .env file or obtaining them
from .bashrc or .zshrc from host home directory"*, and credentials additionally from
`$HOME/api_keys.sh` (Appendix B of the final request) and the private repo. Missing sources are
**skipped silently** and never logged `[CONSTITUTION §11.4.10]`.

```mermaid
flowchart TB
  START([Process start]) --> ENVFILE{".env in CWD<br/>or THREADY_ENV_FILE?"}
  ENVFILE -->|found| LOADENV[Load .env<br/>KEY=VALUE lines]
  ENVFILE -->|missing| SKIP1[SKIP silently]
  LOADENV --> SHELL
  SKIP1 --> SHELL
  SHELL{"~/.bashrc / ~/.zshrc<br/>exported vars?"}
  SHELL -->|exported| LOADSHELL[Inherit from process env]
  SHELL -->|none| SKIP2[SKIP silently]
  LOADSHELL --> KEYS
  SKIP2 --> KEYS
  KEYS{"$HOME/api_keys.sh?"}
  KEYS -->|present| SOURCEKEYS[Source secrets<br/>API keys / tokens]
  KEYS -->|missing| SKIP3[SKIP — never error]
  SOURCEKEYS --> PRIV
  SKIP3 --> PRIV
  PRIV{"Private repo<br/>secrets mount?"}
  PRIV -->|mounted| LOADPRIV[Load sealed values]
  PRIV -->|none| SKIP4[SKIP]
  LOADPRIV --> MERGE
  SKIP4 --> MERGE
  MERGE[Merge with precedence<br/>process-env wins over .env] --> VALIDATE{Required<br/>vars present?}
  VALIDATE -->|yes| RUN([Service runs])
  VALIDATE -->|no| FAILLOUD[Fail loudly<br/>list missing keys]
```

> Rendered PNG/SVG exported via Docs Chain (§11.4.65). Source: [diagrams/config-resolution.mmd](./diagrams/config-resolution.mmd).

**Explanation (for readers/models that cannot see the diagram).** At start-up each Thready service
resolves configuration through an ordered chain of four sources, each of which is optional and
skipped without error if absent. First it looks for a `.env` file in the working directory (or the
path in `THREADY_ENV_FILE`) and loads simple `KEY=VALUE` lines. Second it inherits any variables
already exported into the process environment from the operator's `~/.bashrc`/`~/.zshrc`. Third, if
`$HOME/api_keys.sh` exists it is sourced for secrets (LLM provider keys, messenger tokens) — this is
the operator's shared key vault described in the final request. Fourth, in production the private-repo
secrets mount contributes sealed values. The four sources are then **merged with precedence**: a
value already present in the process environment wins over the same key in `.env`, so an operator can
override a file value at launch (`THREADY_LOG_LEVEL=debug ./thready serve`). Finally the service
**validates** that all *required* variables (for its role) are present; if any are missing it **fails
loudly** and prints the missing keys — it never starts half-configured and never prints secret
values. This "SKIP-if-missing, fail-loud-if-required, never-log-secrets" behaviour is the
Constitution's secrets-hygiene rule `[CONSTITUTION §11.4.10]` applied uniformly across services.

## 2. The `.env` file and secret hygiene

- The `.env` file and any `secrets`/`api_keys.sh` **must be gitignored** and never committed to a
  public repo `[CONSTITUTION §11.4.10]`. Thready ships a committed **`.env.example`** template
  (placeholders only) and you copy it to `.env`.
- File permissions: `chmod 600 .env` and `chmod 700` on secret directories (VERIFIED requirement,
  final request §14.4 / Q39).
- Secrets are **runtime-load-only**; if a key leaks, rotate it and re-run the leak-audit hook.
- Sensitive values (`*_PASSWORD`, `*_TOKEN`, `*_API_KEY`, `*_SECRET`, DSNs) are **redacted in all
  logs and in the config-dump endpoint**. `[GAP: 7 / security]` The searchable-but-sealed
  representation for "encrypted yet semantically searchable credentials" is a `[BUILD-NEW]` item on
  `security/pkg/securestorage`; until it lands, credential *content* extracted from posts is stored
  encrypted but not yet semantically indexed (see [end-user-manual.md](./end-user-manual.md#9-sensitive-content)).

## 3. Quick-start minimal `.env`

The smallest `.env` that boots a **local development** stack (SQLite + in-process bus + local
llama.cpp embeddings). Copy, then fill the messenger credentials to actually read a channel.

```dotenv
# ── Core ──────────────────────────────────────────────
THREADY_ENV=development
THREADY_HTTP_ADDR=0.0.0.0:8443
THREADY_LOG_LEVEL=info

# ── Relational DB (dev = SQLite, cgo-free) ────────────
THREADY_DB_DRIVER=sqlite
THREADY_DB_DSN=file:./data/thready.db?_pragma=busy_timeout(5000)

# ── Vector DB (dev = pgvector on a local Postgres) ────
THREADY_VECTOR_BACKEND=pgvector
THREADY_VECTOR_DSN=postgres://thready:thready@localhost:5432/thready?sslmode=disable
THREADY_EMBEDDING_DIM=1024

# ── Embeddings: MUST be a real semantic provider ──────
# [GAP: 1] Do NOT leave HelixLLM on its default hash embedder — it returns garbage relevance.
HELIX_EMBEDDING_PROVIDER=llama
THREADY_EMBEDDING_BASE_URL=http://localhost:8080/v1
THREADY_EMBEDDING_MODEL=jina-embeddings-v2-base-code

# ── Event bus (dev = in-process) ──────────────────────
THREADY_EVENTBUS_BACKEND=inprocess

# ── Auth (dev secret; use RS256 keypair in prod) ──────
THREADY_JWT_SIGNING_ALG=HS256
THREADY_JWT_SECRET=change-me-32-bytes-minimum-dev-only

# ── Telegram (fill from api_keys.sh in real use) ──────
HERALD_TELEGRAM_API_ID=
HERALD_TELEGRAM_API_HASH=
HERALD_TELEGRAM_PHONE=
```

## 4. Core runtime & server

| Variable | Type | Default | Provenance | Notes |
|----------|------|---------|------------|-------|
| `THREADY_ENV` | enum `development\|staging\|production` | `development` | ASSUMPTION | Selects env-specific defaults; drives log format & safety rails. |
| `THREADY_ENV_FILE` | path | `./.env` | ASSUMPTION | Explicit `.env` location (overrides CWD lookup). |
| `THREADY_HTTP_ADDR` | host:port | `0.0.0.0:8443` | ASSUMPTION | REST `/v1` + WS/SSE bind address. |
| `THREADY_HTTP3_ENABLED` | bool | `true` | `[IN-HOUSE: http3]` | Enables HTTP/3 (QUIC) via `vasic-digital/http3`; falls back to HTTP/2. |
| `THREADY_HTTP_COMPRESSION` | csv | `br,gzip` | `[DEFAULT — adjustable]` | Response compression (Brotli/gzip). |
| `THREADY_LOG_LEVEL` | enum `debug\|info\|warn\|error` | `info` | `[IN-HOUSE: observability]` | logrus level. |
| `THREADY_LOG_FORMAT` | enum `json\|text` | `json` (`text` in dev) | `[IN-HOUSE: observability]` | Structured logging. |
| `THREADY_REQUEST_TIMEOUT` | duration | `30s` | `[DEFAULT — adjustable]` | Per-request server timeout. |
| `THREADY_RATE_LIMIT_RPS` | int | `100` | `[IN-HOUSE: ratelimiter]` | Per-identity request rate cap. |
| `THREADY_CORS_ORIGINS` | csv | `` (none) | `[IN-HOUSE: middleware]` | Allowed browser origins (portal domains). |
| `THREADY_PORT_PREFIX` | int (≤65535 budget) | unset | `[IN-HOUSE: port_prefix]` | Deterministic dynamic-port base for the container stack. |

## 5. Environments, domains & TLS

Three fully separated environments behind subdomains on one Hetzner host (final request §8.2).

| Variable | Type | Default | Provenance | Notes |
|----------|------|---------|------------|-------|
| `THREADY_PUBLIC_DOMAIN` | host | `thready.hxd3v.com` | `[OPERATOR]` | Base domain. Dev/staging derive `dev.`/`sta.` prefixes. |
| `THREADY_PUBLIC_BASE_URL` | url | `https://thready.hxd3v.com` | ASSUMPTION | Absolute base for links in generated content & emails. |
| `LETS_ENCRYPT_EMAIL` | email | unset | `[IN-HOUSE: lets_encrypt]` | ACME account email. |
| `LETS_ENCRYPT_CHALLENGE` | enum `http-01\|dns-01` | `http-01` | `[IN-HOUSE: lets_encrypt]` | ACME challenge type (acme.sh). |
| `THREADY_TLS_MIN_VERSION` | enum | `1.3` | `[DEFAULT — adjustable]` | Minimum TLS version (§Q38). |
| `CONTAINERS_REMOTE_ENABLED` | bool | `false` | `[IN-HOUSE: containers]` (VERIFIED) | Enable remote container distribution. |
| `CONTAINERS_REMOTE_DEFAULT_SSH_USER` | string | `deploy` | `[IN-HOUSE: containers]` (VERIFIED) | SSH user for deploy targets (Thready uses `thready`). |
| `CONTAINERS_REMOTE_DEFAULT_RUNTIME` | enum `docker\|podman` | `podman` | `[CONSTITUTION §11.4.76]` | **Rootless Podman only** for Thready. |
| `CONTAINERS_REMOTE_PORT_RANGE_START` / `_END` | int | `20000` / `30000` | `[IN-HOUSE: containers]` (VERIFIED) | Tunnel/port range. |

## 6. Relational database

`digital.vasic.database` selects the backend by `Driver` (VERIFIED: SQLite dev / Postgres prod,
migrations via `pkg/migration.Runner`).

| Variable | Type | Default | Provenance | Notes |
|----------|------|---------|------------|-------|
| `THREADY_DB_DRIVER` | enum `sqlite\|postgres` | `sqlite` (dev) / `postgres` (prod) | `[IN-HOUSE: database]` | Maps to `Config.Driver`. |
| `THREADY_DB_DSN` | dsn | see quick-start | `[IN-HOUSE: database]` | SQLite: `file:...`; Postgres: `postgres://user:pass@host:5432/db`. |
| `THREADY_DB_MAX_OPEN_CONNS` | int | `32` | `[DEFAULT — adjustable]` | pgx pool size (tune for Large scale). |
| `THREADY_DB_MAX_IDLE_CONNS` | int | `8` | `[DEFAULT — adjustable]` | Idle pool. |
| `THREADY_DB_CONN_MAX_LIFETIME` | duration | `30m` | `[DEFAULT — adjustable]` | Connection recycle. |
| `THREADY_DB_MIGRATE_ON_BOOT` | bool | `true` (dev) / `false` (prod) | `[IN-HOUSE: database]` | Auto-run `migration.Runner.Apply` at start (prod runs migrations via deploy step). |
| `THREADY_DB_PARTITIONING` | bool | `true` (prod) | `[GAP: 3.2]` | Time-partitioned `posts` for 10k+/day. Partitioning helpers are a `P1` improvement on `database` — until merged this is best-effort. |

## 7. Datastores

### 7.1 Vector DB (semantic search) `[GAP: 8]`

Only the **pgvector** backend is production-wired (VERIFIED); Qdrant/Pinecone/Milvus adapters exist
behind the same `VectorStore` interface but are **unverified end-to-end** — a config-only swap is
planned, not guaranteed.

| Variable | Type | Default | Provenance | Notes |
|----------|------|---------|------------|-------|
| `THREADY_VECTOR_BACKEND` | enum `pgvector\|qdrant\|pinecone\|milvus` | `pgvector` | `[IN-HOUSE: vectordb]` | Only `pgvector` is VERIFIED. Others `[OPEN]`. |
| `THREADY_VECTOR_DSN` | dsn | shares Postgres | `[IN-HOUSE: vectordb]` | pgvector co-locates in the relational Postgres. |
| `THREADY_VECTOR_METRIC` | enum `cosine\|l2\|ip` | `cosine` | `[IN-HOUSE: vectordb]` | Cosine `<=>` per decision matrix. |
| `THREADY_EMBEDDING_DIM` | int | `1024` | `[GAP: 1]` | Must match the embedding model's true dimension. RAG path historically hardcoded 768 — set this explicitly. |
| `THREADY_VECTOR_INDEX` | enum `hnsw\|ivfflat` | `hnsw` | `[DEFAULT — adjustable]` | ANN index for the < 500 ms search SLO. |
| `THREADY_QDRANT_URL` | url | unset | `[GAP: 8]` | Only used if backend=`qdrant`; hardening tracked as `[OPEN]`. |

### 7.2 Cache

| Variable | Type | Default | Provenance | Notes |
|----------|------|---------|------------|-------|
| `THREADY_CACHE_BACKEND` | enum `memory\|redis\|postgres` | `memory` (dev) / `redis` (prod) | `[IN-HOUSE: cache]` | L1/L2 tiers supported. |
| `THREADY_CACHE_REDIS_URL` | url | unset | `[IN-HOUSE: cache]` | `redis://host:6379/0`. |
| `THREADY_CACHE_TTL` | duration | `10m` | `[DEFAULT — adjustable]` | Default entry TTL. |

### 7.3 Object storage (assets)

`digital.vasic.storage` (MinIO/S3) is the asset tier for the 50 TB+ scale.

| Variable | Type | Default | Provenance | Notes |
|----------|------|---------|------------|-------|
| `THREADY_STORAGE_BACKEND` | enum `filesystem\|minio\|s3` | `filesystem` (dev) / `minio` (prod) | `[IN-HOUSE: storage]` | |
| `THREADY_STORAGE_ENDPOINT` | url | unset | `[IN-HOUSE: storage]` | MinIO endpoint. |
| `THREADY_STORAGE_BUCKET` | string | `thready-assets` | ASSUMPTION | Object bucket. |
| `THREADY_STORAGE_ACCESS_KEY` | secret | unset | `[IN-HOUSE: storage]` | Redacted in logs. |
| `THREADY_STORAGE_SECRET_KEY` | secret | unset | `[IN-HOUSE: storage]` | Redacted in logs. |
| `THREADY_STORAGE_SIGNED_URL_TTL` | duration | `15m` | `[GAP: 3.2]` | MinIO signed-URL parity with CloudFront signing is a `P1` verification item. |

## 8. Embeddings, LLM & Vision

### 8.1 Embeddings `[GAP: 1]` (P0 trap — read this)

**VERIFIED danger zone.** HelixLLM's *default* local embedder is a non-semantic `HashEmbedder`
stub that emits deterministic pseudo-vectors with only a startup WARNING; any semantic search built
on the default silently returns garbage. **You must set `HELIX_EMBEDDING_PROVIDER=llama`** (real
llama.cpp `/embedding`) for every semantic workload. Thready is configured to **fail loudly**, not
warn, if the hash embedder is selected in a search/RAG context.

| Variable | Type | Default | Provenance | Notes |
|----------|------|---------|------------|-------|
| `HELIX_EMBEDDING_PROVIDER` | enum `llama\|openai\|hash` | **`llama`** (enforced) | `[IN-HOUSE: HelixLLM]` (VERIFIED) | `hash` is blocked for search; setting it aborts start-up. |
| `THREADY_EMBEDDING_BASE_URL` | url | `http://localhost:8080/v1` | `[IN-HOUSE: embeddings]` | OpenAI-compatible endpoint (HelixLLM `/v1/embeddings`). |
| `THREADY_EMBEDDING_MODEL` | string | `jina-embeddings-v2-base-code` | `[RESEARCH]` | Code-tuned; alt `voyage-code-3`. |
| `THREADY_EMBEDDING_API_KEY` | secret | unset | `[IN-HOUSE: embeddings]` | Only if the endpoint requires it. |

### 8.2 LLM providers

Bare provider-key names are **VERIFIED** from `llm_provider/.env.example`. Set only the ones you use;
missing keys are skipped.

| Variable | Type | Provenance | Notes |
|----------|------|------------|-------|
| `HELIX_LLM_BASE_URL` | url | `[IN-HOUSE: HelixLLM]` | Local llama.cpp server (OpenAI+Anthropic APIs). |
| `HELIX_LLM_MODEL` | string | `[IN-HOUSE]` | e.g. `Llama-3.1-70B-Instruct-Q4_K_M` (research/reasoning). |
| `HELIX_LLM_CODE_MODEL` | string | `[IN-HOUSE]` | e.g. `Qwen2.5-Coder`. |
| `ANTHROPIC_API_KEY` | secret | VERIFIED | Cloud fallback `claude-sonnet-4`. |
| `OPENAI_API_KEY` | secret | VERIFIED | Fallback / vision `gpt-4o`. |
| `GOOGLE_API_KEY` | secret | VERIFIED | Fallback `gemini-2.5-pro`. |
| `DEEPSEEK_API_KEY` | secret | VERIFIED | Fallback `deepseek-v3`. |
| `GROQ_API_KEY`, `MISTRAL_API_KEY`, `QWEN_API_KEY`, `OPENROUTER_API_KEY`, `COHERE_API_KEY`, `TOGETHER_API_KEY`, `XAI_API_KEY`, `PERPLEXITY_API_KEY`, `NVIDIA_API_KEY` | secret | VERIFIED | Additional `llmprovider` adapters (40+); set as needed. |
| `THREADY_LLM_MAX_RETRIES` | int (`5`) | `[DEFAULT — adjustable]` | Provider retry ceiling. |
| `THREADY_LLM_CIRCUIT_BREAKER` | bool (`true`) | `[IN-HOUSE: LLMProvider]` | Circuit breaker for flapping providers. |

### 8.3 Vision & OCR `[GAP: 2]`

`HELIX_VISION_*` are **VERIFIED** in `vision_engine/.env.example`. VisionEngine has **no OCR engine**
(P0 gap) — OCR is a `[BUILD-NEW]` Tesseract/PaddleOCR adapter. Until it lands, `#Comic`/`#Screenshot`
OCR falls back to LLM-vision transcription only (lower fidelity, non-deterministic).

| Variable | Type | Default | Provenance | Notes |
|----------|------|---------|------------|-------|
| `HELIX_VISION_PROVIDER` | enum `auto\|openai\|anthropic\|gemini\|qwen\|ollama` | `auto` | `[IN-HOUSE: VisionEngine]` (VERIFIED) | Probes configured providers. |
| `HELIX_VISION_TIMEOUT` | int (sec) | `60` | VERIFIED | Vision call timeout. |
| `HELIX_VISION_MAX_IMAGE_SIZE` | int (px) | `4096` | VERIFIED | Downscale ceiling. |
| `THREADY_OCR_PROVIDER` | enum `tesseract\|paddleocr\|none` | `none` | `[BUILD-NEW]` | Real OCR pending the adapter; `none` = LLM-vision only. |
| `THREADY_OCR_LANGS` | csv | `eng,rus` | `[DEFAULT — adjustable]` | Tesseract language packs (Cyrillic for `ru`/`sr`). |

## 9. Messengers (Herald) `[GAP: 3]`

**VERIFIED status.** Telegram thread-reading works via the `gotd/td` MTProto **user** client, which
today lives inside Herald's `qaherald` QA harness and is being **promoted to a first-class channel**
(`[BUILD-NEW]` P0). The Bot-API adapter cannot backfill channel history. **Max is an empty stub** —
only reserved env vars exist (`HERALD_MAX_BOT_TOKEN`, `HERALD_MAX_CHAT_ID`); the adapter (Bot API +
Go port of the OneMe user-WebSocket) is `[BUILD-NEW]` P0. Do not expect Max reading before that
lands. Sign-in flows: [installation.md §7](./installation.md#7-messenger-sign-in-first-channel).

| Variable | Type | Provenance | Notes |
|----------|------|------------|-------|
| `HERALD_TELEGRAM_API_ID` | secret (int) | `[IN-HOUSE: herald]` | From my.telegram.org. |
| `HERALD_TELEGRAM_API_HASH` | secret | `[IN-HOUSE: herald]` | Paired with API ID. |
| `HERALD_TELEGRAM_PHONE` | string | `[IN-HOUSE: herald]` | User account phone (E.164). |
| `HERALD_TELEGRAM_2FA_PASSWORD` | secret | `[IN-HOUSE: herald]` | Only if the account has 2FA. |
| `HERALD_TELEGRAM_SESSION_PATH` | path | `[IN-HOUSE: herald]` | Persisted MTProto session (avoids re-login). Store encrypted. |
| `HERALD_MAX_BOT_TOKEN` | secret | `[IN-HOUSE: herald]` (VERIFIED reserved) | Max Bot API token — **adapter not built yet**. |
| `HERALD_MAX_CHAT_ID` | string | `[IN-HOUSE: herald]` (VERIFIED reserved) | Reserved; no code consumes it yet. |
| `THREADY_MESSENGER_SIGNIN_MODE` | enum `interactive\|noninteractive` | `[OPERATOR]` | Non-interactive reads all creds from env for headless deploy. |
| `THREADY_POLL_INTERVAL` | duration (`5m`) | `[DEFAULT — adjustable]` | Scheduled wake frequency; event triggers supplement polling. |
| `THREADY_REPLY_ACCOUNT` | enum `robot\|user` | `robot` | `[DEFAULT — adjustable]` | Which account posts status replies. |

## 10. Event bus, background jobs & processing

| Variable | Type | Default | Provenance | Notes |
|----------|------|---------|------------|-------|
| `THREADY_EVENTBUS_BACKEND` | enum `inprocess\|nats` | `inprocess` (dev) / `nats` (prod) | `[IN-HOUSE: eventbus]` | NATS JetStream is the Large-scale transport. |
| `THREADY_NATS_URL` | url | unset | `[IN-HOUSE: eventbus]` | `nats://host:4222`. |
| `THREADY_NATS_STREAM` | string | `thready` | ASSUMPTION | JetStream stream name. |
| `THREADY_WORKERS` | int | `32` | `[DEFAULT — adjustable]` (Q4) | BackgroundTasks worker-pool size. |
| `THREADY_RETRY_MAX` | int | `5` | `[DEFAULT — adjustable]` (§3.3) | Max retries per step/post. |
| `THREADY_RETRY_BASE` | duration | `2s` | `[DEFAULT — adjustable]` | Exponential back-off base. |
| `THREADY_RETRY_FACTOR` | float | `2.0` | `[DEFAULT — adjustable]` | Back-off multiplier. |
| `THREADY_RETRY_CAP` | duration | `5m` | `[DEFAULT — adjustable]` | Back-off ceiling. |
| `THREADY_POST_TIMEOUT` | duration | `30m` | `[DEFAULT — adjustable]` | Per-post soft budget (research-heavy). |
| `THREADY_SKILL_CONCURRENCY` | int | `8` | `[DEFAULT — adjustable]` | Per-Skill concurrency cap. |

> **Idempotency (VERIFIED design).** Each post is claimed exactly once via a Postgres row/advisory
> lock in the BackgroundTasks queue, so a "new post" event storm never double-processes a post
> `[CONSTITUTION §11.4.176]`. No env var disables this.

## 11. Authentication & security `[GAP: 10]`

**VERIFIED:** `digital.vasic.auth` defaults JWT to **HMAC-SHA256**, which is fine single-service but
multi-service verification wants asymmetric keys. Thready plans **RS256/EdDSA + JWKS rotation**
(`P1`). Set `THREADY_JWT_SIGNING_ALG=RS256` once the keypair is provisioned.

| Variable | Type | Default | Provenance | Notes |
|----------|------|---------|------------|-------|
| `THREADY_JWT_SIGNING_ALG` | enum `HS256\|RS256\|EdDSA` | `HS256` (dev) → `RS256` (prod target) | `[IN-HOUSE: auth]` `[GAP: 10]` | |
| `THREADY_JWT_SECRET` | secret | unset | `[IN-HOUSE: auth]` | HS256 only; ≥32 bytes. |
| `THREADY_JWT_PRIVATE_KEY_PATH` / `_PUBLIC_KEY_PATH` | path | unset | `[IN-HOUSE: auth]` | RS256/EdDSA PEMs. |
| `THREADY_ACCESS_TOKEN_TTL` | duration | `15m` | `[DEFAULT — adjustable]` (Q9) | Access token lifetime. |
| `THREADY_REFRESH_TOKEN_TTL` | duration | `168h` (7 d) | `[DEFAULT — adjustable]` | Refresh token lifetime. |
| `THREADY_IDLE_TIMEOUT` | duration | `30m` | `[DEFAULT — adjustable]` | Web idle logout. |
| `THREADY_MFA_REQUIRED_TIERS` | csv | `root,account_admin` | `[DEFAULT — adjustable]` (Q9) | TOTP mandatory for admin tiers, optional for users. |
| `THREADY_PASSWORD_MIN_LEN` | int | `12` | `[DEFAULT — adjustable]` | Argon2id-hashed; breach-list checked. |
| `THREADY_ARGON2_MEMORY_KIB` | int | `65536` | `[IN-HOUSE: security]` | Argon2id KDF memory. |
| `THREADY_API_KEY_HASH_PEPPER` | secret | unset | `[IN-HOUSE: auth]` | Server-side pepper for API-key hashing. |
| `THREADY_ENCRYPTION_KEY` | secret (32 B) | unset | `[IN-HOUSE: security]` | AES-256-GCM master key for sealed storage. Redacted. |

## 12. Downloads & 3rd-party systems `[GAP: 4] [GAP: 5]`

| Variable | Type | Default | Provenance | Notes |
|----------|------|---------|------------|-------|
| `THREADY_BOBA_URL` | url | unset | `[IN-HOUSE: Boba]` | Torrent search/download (SSE + `POST /api/v1/hooks`). |
| `THREADY_BOBA_CALLBACK_URL` | url | derived | `[GAP: 6.4]` | Where Boba posts completion; contract being standardized. |
| `THREADY_METUBE_URL` | url | unset | `[IN-HOUSE: MeTube]` | Video/streaming download. |
| `THREADY_METUBE_WEBHOOK_URL` | url | derived | `[BUILD-NEW]` `[GAP: 5]` | MeTube is **poll-only today**; outbound webhook is being added. Until then Thready polls `GET /api/postprocess/status`. |
| `THREADY_DOWNLOAD_MANAGER_URL` | url | unset | `[BUILD-NEW]` `[GAP: 4]` | Generic multi-protocol Download Manager **does not exist yet**; HTTP(S) source + queue/resume semantics are being built. |
| `THREADY_DOWNLOAD_CONCURRENCY` | int | `4` | `[DEFAULT — adjustable]` | Parallel download jobs. |
| `THREADY_GAME_DEFAULT_PLATFORMS` | csv | `PC-Windows,PS4,Android` | `[DEFAULT — adjustable]` | `#Game` default targets (§3.2.2). |
| `THREADY_SOFTWARE_DEFAULT_OS` | csv | `Windows,Linux,macOS` | `[DEFAULT — adjustable]` | `#Software` default OSes. |

## 13. Assets & media directories

Directories are configurable and modifiable at runtime via the management API (client → REST → System).

| Variable | Type | Default | Provenance | Notes |
|----------|------|---------|------------|-------|
| `THREADY_ASSET_SERVICE_URL` | url | unset | `[BUILD-NEW]` (Catalogizer) `[GAP: 9]` | Asset Service. Decoupling from Catalogizer **and** the HLS/DASH transcoder integration are `P1` — Catalogizer's `Streaming` submodule is a WebSocket hub, not media/transcode streaming. Zero version serves raw + one `-web` rendition over HTTP Range. Client links resolve **through** this service, never direct paths. |
| `THREADY_MEDIA_DIR` | path | `./data/media` | `[OPERATOR-editable]` | Root for downloaded media. |
| `THREADY_WEB_RENDITION_SUFFIX` | string | `-web` | `[DEFAULT — adjustable]` (Q36) | Suffix before extension for web-optimized renditions. |
| `THREADY_ENCRYPTED_ASSET_DIR` | path | `./data/secure` | `[IN-HOUSE: security]` | AES-256-GCM directory for cards/contracts/QR/screenshots. |
| `THREADY_ASSET_DEDUP` | bool | `true` | `[GAP: 6.1]` | Content-hash dedup + integrity checksums. |

## 14. Observability, logging & backup

`digital.vasic.observability` = OpenTelemetry + Prometheus + logrus + ClickHouse (VERIFIED; **not**
ELK/Loki/Datadog).

| Variable | Type | Default | Provenance | Notes |
|----------|------|---------|------------|-------|
| `OTEL_EXPORTER_OTLP_ENDPOINT` | url | unset | `[IN-HOUSE: observability]` | OTLP collector (Jaeger/Zipkin behind it). |
| `THREADY_METRICS_ADDR` | host:port | `0.0.0.0:9090` | `[IN-HOUSE: observability]` | Prometheus scrape endpoint. |
| `THREADY_CLICKHOUSE_DSN` | dsn | unset | `[IN-HOUSE: observability]` | Log/analytics sink. |
| `THREADY_AUDIT_RETENTION` | duration | `8760h` (1 y) | `[DEFAULT — adjustable]` (Q40) | Append-only audit-log retention. |
| `THREADY_BACKUP_FULL_CRON` | cron | `0 3 * * *` | `[OPERATOR]` (Q41) | Daily full backup. |
| `THREADY_BACKUP_INCREMENTAL_CRON` | cron | `0 * * * *` | `[OPERATOR]` | Hourly DB incrementals (RPO ≈ 1 h). |
| `FIREBASE_PROJECT_ID` | string | unset | `[CONSTITUTION §11.4.47]` | Crashlytics/Analytics/Distribution (mobile). |

## 15. Retention, billing & localization

| Variable | Type | Default | Provenance | Notes |
|----------|------|---------|------------|-------|
| `THREADY_RETENTION_DEFAULT` | duration \| `indefinite` | `indefinite` | `[OPERATOR]` (Q12) | Global default; Accounts may shorten. |
| `THREADY_BILLING_MODE` | enum `subscription+metered` | `subscription+metered` | `[OPERATOR]` (Q11) | Metering on from day one. |
| `THREADY_METERING_FLUSH` | duration | `1m` | `[DEFAULT — adjustable]` | Usage-event flush interval. |
| `THREADY_DEFAULT_LOCALE` | enum `en\|ru\|sr-Cyrl` | `en` | `[DEFAULT — adjustable]` (Q35) | UI locale default. |
| `THREADY_TRANSLATE_URL` | url | unset | `[IN-HOUSE: HelixTranslate]` | On-demand translation service. |

## 16. White-labeling & branding

Per-account branding; new accounts default to Thready/Helix Development branding (final request §8.3).

| Variable | Type | Default | Provenance | Notes |
|----------|------|---------|------------|-------|
| `THREADY_BRAND_NAME` | string | `Thready` | `[OPERATOR-editable]` | System-default product name. |
| `THREADY_BRAND_PRIMARY_COLOR` | hex | `#B6E376` | `[IN-HOUSE: design_system]` | Helix-green base; per-account override in DB. |
| `THREADY_BRAND_LOGO_PATH` | path | `./assets/Logo.png` | `[OPERATOR-editable]` | Default logo (theme derived from it). |
| `THREADY_BRAND_SLOGAN` | string | `Made with love ♥ by Helix Development` | `[OPERATOR]` | Footer slogan (heart glyph); Helix attribution persists even under white-label. |
| `THREADY_THEME_DEFAULT` | enum `system\|light\|dark` | `system` | `[CONSTITUTION §11.4.162]` | Light + dark mandatory. |

> **Note.** Per-account branding overrides (colors/logo/slogan for a specific Account) are stored in
> the **database**, not env vars — they are edited by the Root Admin via the API/portal
> ([root-admin-guide.md §5](./root-admin-guide.md#5-white-label-branding)). Env vars set only the
> *system default* applied to new Accounts.

## 17. Precedence, validation & change management

- **Precedence:** process-env (shell export) > `.env` file > compiled default. Sourced secrets
  (`api_keys.sh`) and the private-repo mount populate process-env before the file is read.
- **Validation:** the service validates its required set at boot and **fails loudly** listing
  missing keys; use `thready config validate` ([cli-reference.md](./cli-reference.md#57-config-commands))
  to check a `.env` without starting the service.
- **Runtime changes:** directory/frequency/branding settings changeable at runtime go
  **client → REST API → System** (final request §21.4) and are RBAC-gated; they persist to the DB
  and re-emit a `config.changed` event. Secrets are **not** hot-reloaded — rotating a key requires a
  service restart (documented in [troubleshooting.md](./troubleshooting.md#9-configuration-changes-not-taking-effect)).
- **Docs sync:** `.env.example` is kept in lockstep with this reference via Docs Chain
  `[CONSTITUTION §11.4.65]`; a CI-equivalent local hook diffs the two.

## 18. Open items

- `[OPEN: cfg-1]` Final `THREADY_*` variable names are this document's proposal; ratify against the
  actual Thready service `config` package once it exists. Tracked workable item: **ATM — reconcile
  `THREADY_*` names with implemented config struct tags**.
- `[OPEN: cfg-2]` `THREADY_VECTOR_BACKEND=qdrant|pinecone|milvus` are unverified `[GAP: 8]`; keep on
  `pgvector` until the Qdrant backend is integration-tested. Workable item: **ATM — harden Qdrant
  backend to pgvector parity**.
- `[OPEN: cfg-3]` `THREADY_METUBE_WEBHOOK_URL` / `THREADY_DOWNLOAD_MANAGER_URL` reference
  `[BUILD-NEW]` services; their exact request/response contract is finalized in the callback-module
  spec. Workable item: **ATM — publish standardized callback schema**.
- `[OPEN: cfg-4]` Searchable-but-sealed credential representation not yet implemented `[GAP: 7]`;
  credential content is encrypted but not semantically indexed. Workable item: **ATM — redacted-token
  embedding over `securestorage`**.

---

*Made with love ♥ by Helix Development.*
