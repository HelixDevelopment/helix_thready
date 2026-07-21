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
| 2 | 2026-07-22 | swarm (user-guides, Pass 3) | Depth pass: **corrected messenger env names to the VERIFIED Herald `HERALD_MTPROTO_*` / `HERALD_TGRAM_*` names** read from `vasic-digital/herald/.env.example`; added VERIFIED VisionEngine (`HELIX_VISION_*`, `HELIX_OLLAMA_*`, `HELIX_LLAMACPP_RPC_*`), LLMProvider (Cerebras/Fireworks/HuggingFace/Replicate/SambaNova/SiliconFlow) and additional `CONTAINERS_REMOTE_*` variables; added **Appendix A — Master environment-variable index** (name · purpose · default · scope · example); added **Appendix B — worked `.env` examples per environment**; split the §1 diagram explanation into multi-paragraph form; closed `[OPEN: cfg-messenger]`. |

This is the **complete, documented environment-variable reference** for Helix Thready, mandated by
the original request: *"All environment variables that system supports MUST be properly documented
in separate dedicated document(s)."* Every variable the system reads is listed here with type,
default, scope, provenance, and whether it is **VERIFIED** (a real in-house module variable read at
source) or an **ASSUMPTION** / `[DEFAULT — adjustable]` (a Thready-specific default proposed here).

> **Naming convention (VERIFIED pattern, Thready names are ASSUMPTION).** In-house modules use a
> module prefix: `HELIX_VISION_*` / `HELIX_OLLAMA_*` / `HELIX_LLAMACPP_RPC_*` (VisionEngine — VERIFIED
> in `helix_track/vision_engine/.env.example`), `HERALD_MTPROTO_*` / `HERALD_TGRAM_*` / `HERALD_MAX_*`
> (messengers — VERIFIED in `vasic-digital/herald/.env.example` and `herald/docs/guides/messengers/`),
> `CONTAINERS_REMOTE_*` (deployment — VERIFIED in `helix_track/containers/.env.example`), and bare
> provider keys such as `ANTHROPIC_API_KEY`, `OPENAI_API_KEY`, `GOOGLE_API_KEY`, `DEEPSEEK_API_KEY`
> (VERIFIED in `vasic-digital/LLMProvider/.env.example`). Thready's own service variables use the
> **`THREADY_*`** prefix. The `THREADY_*` names are this document's proposal `[DEFAULT — adjustable]`;
> the reused `HELIX_*`/`HERALD_*`/`CONTAINERS_*`/provider names are the actual module variables Thready
> inherits and **must not rename** — the exact spellings below were read from each module's committed
> `.env.example` at source, not guessed.

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
19. [Appendix A — Master environment-variable index](#appendix-a--master-environment-variable-index)
20. [Appendix B — Worked `.env` examples per environment](#appendix-b--worked-env-examples-per-environment)

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
resolves configuration through an ordered chain of four optional sources, walking them top to bottom.
Each source is skipped without error if it is absent — a missing `.env`, a bare shell, no
`api_keys.sh`, and no secrets mount are all valid states that never abort the boot and never emit a
log line about the absence. This is deliberate: the resolver is designed so the same binary boots
identically on a developer laptop with a single `.env` and on a hardened production host where every
secret arrives from a mounted vault, with no code path that treats "source not present" as an error.

The first source is a **`.env` file** in the working directory (or the explicit path in
`THREADY_ENV_FILE`), parsed as simple `KEY=VALUE` lines. The second source is the **process
environment** — any variable already exported by the operator's `~/.bashrc`/`~/.zshrc`, a Kubernetes
ConfigMap, or a CI secret store. The third source is **`$HOME/api_keys.sh`**, sourced for secrets
(LLM provider keys, messenger tokens) — the operator's shared key vault described in the final
request's Appendix B. The fourth source, used in production, is the **private-repo secrets mount**,
which contributes sealed values that never touch the public tree.

The four sources are then **merged with a fixed precedence**: a value already present in the process
environment wins over the same key in `.env`, which in turn wins over the compiled default. This is
the identical 12-factor order the sibling Herald module documents as VERIFIED (`herald` spec V3 §3.3:
explicit flag > shell export > `.env` fallback > compiled default), so an operator can keep team
defaults in a committed-shaped `.env.example`-derived `.env` while overriding any single value per host
via a shell export (`THREADY_LOG_LEVEL=debug ./thready serve`) — the file never clobbers the export.

Finally the service **validates** that every *required* variable for its role is present. If any are
missing it **fails loudly**, printing the missing keys by name, and refuses to start half-configured;
if a blocked value is present (for example `HELIX_EMBEDDING_PROVIDER=hash` in a search context, §8.1)
it aborts with a fix-it message rather than warning-and-continuing. At no point are secret *values*
printed — `*_PASSWORD`/`*_TOKEN`/`*_API_KEY`/`*_SECRET`/DSNs are redacted in every sink. This
"SKIP-if-missing, fail-loud-if-required, never-log-secrets" behaviour is the Constitution's
secrets-hygiene rule `[CONSTITUTION §11.4.10]` applied uniformly across every Thready service.

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

# ── Telegram (VERIFIED Herald MTProto user-client names) ──
# Fill from api_keys.sh in real use. These read the FULL thread history.
HERALD_MTPROTO_APP_ID=
HERALD_MTPROTO_APP_HASH=
HERALD_MTPROTO_PHONE=
# HERALD_MTPROTO_PASSWORD=            # only if the account has cloud 2FA
# HERALD_MTPROTO_SESSION_FILE=~/.config/herald/mtproto.session
# ── Telegram Bot API (status replies; cannot backfill history) ──
# HERALD_TGRAM_BOT_TOKEN=
# HERALD_TGRAM_CHAT_ID=
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
| `CONTAINERS_REMOTE_DEFAULT_SSH_KEY` | path | unset | `[IN-HOUSE: containers]` (VERIFIED) | Private key for deploy-target SSH (else the agent). |
| `CONTAINERS_REMOTE_CONNECT_TIMEOUT` | duration | `10s` | `[IN-HOUSE: containers]` (VERIFIED) | SSH connect timeout to a deploy target. |
| `CONTAINERS_REMOTE_COMMAND_TIMEOUT` | duration | `5m` | `[IN-HOUSE: containers]` (VERIFIED) | Per-remote-command timeout. |
| `CONTAINERS_REMOTE_SSH_CONTROL_MASTER` | bool | `true` | `[IN-HOUSE: containers]` (VERIFIED) | Re-use one SSH control connection (`ControlMaster`). |
| `CONTAINERS_REMOTE_SSH_CONTROL_PERSIST` | duration | `60s` | `[IN-HOUSE: containers]` (VERIFIED) | How long the multiplexed SSH master lingers. |
| `CONTAINERS_REMOTE_SSH_MAX_CONNECTIONS` | int | `10` | `[IN-HOUSE: containers]` (VERIFIED) | Concurrent SSH connections per target. |
| `CONTAINERS_REMOTE_SCHEDULER` | enum `roundrobin\|leastloaded` | `roundrobin` | `[IN-HOUSE: containers]` (VERIFIED) | Placement across multiple remote hosts. |
| `CONTAINERS_REMOTE_VOLUME_TYPE` | enum `bind\|volume` | `volume` | `[IN-HOUSE: containers]` (VERIFIED) | Default mount type for remote deploys. |

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
| `GROQ_API_KEY`, `MISTRAL_API_KEY`, `QWEN_API_KEY`, `OPENROUTER_API_KEY`, `COHERE_API_KEY`, `TOGETHER_API_KEY`, `XAI_API_KEY`, `PERPLEXITY_API_KEY`, `NVIDIA_API_KEY` | secret | VERIFIED | Additional `LLMProvider` adapters; set as needed. |
| `CEREBRAS_API_KEY`, `FIREWORKS_API_KEY`, `HUGGINGFACE_API_KEY`, `SAMBANOVA_API_KEY`, `SILICONFLOW_API_KEY` | secret | VERIFIED (`LLMProvider/.env.example`) | Further VERIFIED provider adapters. |
| `REPLICATE_API_TOKEN` | secret | VERIFIED (`LLMProvider/.env.example`) | Replicate uses a `*_TOKEN`, not `*_API_KEY`, spelling — do not rename. |
| `THREADY_LLM_MAX_RETRIES` | int (`5`) | `[DEFAULT — adjustable]` | Provider retry ceiling. |
| `THREADY_LLM_CIRCUIT_BREAKER` | bool (`true`) | `[IN-HOUSE: LLMProvider]` | Circuit breaker for flapping providers. |

### 8.3 Vision & OCR `[GAP: 2]`

`HELIX_VISION_*` are **VERIFIED** in `vision_engine/.env.example`. VisionEngine has **no OCR engine**
(P0 gap) — OCR is a `[BUILD-NEW]` Tesseract/PaddleOCR adapter. Until it lands, `#Comic`/`#Screenshot`
OCR falls back to LLM-vision transcription only (lower fidelity, non-deterministic).

| Variable | Type | Default | Provenance | Notes |
|----------|------|---------|------------|-------|
| `HELIX_VISION_PROVIDER` | enum `auto\|openai\|anthropic\|gemini\|qwen\|astica\|nvidia\|kimi\|stepfun\|ollama` | `auto` | `[IN-HOUSE: VisionEngine]` (VERIFIED) | Probes configured providers. Enum widened to the VERIFIED set in `vision_engine/.env.example`. |
| `HELIX_VISION_TIMEOUT` | int (sec) | `60` | VERIFIED | Vision call timeout. |
| `HELIX_VISION_MAX_IMAGE_SIZE` | int (px) | `4096` | VERIFIED | Downscale ceiling. |
| `HELIX_VISION_OPENCV_ENABLED` | bool | `true` | VERIFIED (`vision_engine/.env.example`) | Enable the OpenCV pre-analysis pass (SSIM/near-duplicate detection). |
| `HELIX_VISION_SSIM_THRESHOLD` | float `0..1` | `0.95` | VERIFIED | Structural-similarity cutoff for treating two frames/images as duplicates. |
| `HELIX_VISION_HOSTS` | csv | unset | VERIFIED | Vision worker hosts to distribute calls across (e.g. `thinker.local,amber.local`). |
| `HELIX_VISION_USER` | string | unset | VERIFIED | SSH user for the remote vision hosts. |
| `HELIX_OLLAMA_URL` | url | `http://localhost:11434` | VERIFIED | Local Ollama endpoint when `HELIX_VISION_PROVIDER=ollama`. |
| `HELIX_OLLAMA_MODEL` | string | `minicpm-v:8b` | VERIFIED | Local multimodal model for Ollama vision. |
| `HELIX_LLAMACPP_RPC_ENABLED` | bool | `false` | VERIFIED | Use a llama.cpp RPC worker pool for local vision. |
| `HELIX_LLAMACPP_RPC_WORKERS` | csv host:port | unset | VERIFIED | RPC worker addresses (only if RPC enabled). |
| `HELIX_LLAMACPP_RPC_MODEL` | path | `~/models/vision-model.gguf` | VERIFIED | GGUF model for the RPC vision path. |
| `ASTICA_API_KEY` | secret | unset | VERIFIED (`vision_engine/.env.example`) | API key for the `astica` vision provider. |
| `KIMI_API_KEY`, `STEPFUN_API_KEY` | secret | unset | VERIFIED | Keys for the `kimi` / `stepfun` vision providers. |
| `THREADY_OCR_PROVIDER` | enum `tesseract\|paddleocr\|none` | `none` | `[BUILD-NEW]` | Real OCR pending the adapter; `none` = LLM-vision only. |
| `THREADY_OCR_LANGS` | csv | `eng,rus` | `[DEFAULT — adjustable]` | Tesseract language packs. Add `srp` for Serbian-Cyrillic content — `sr-Cyrl` is a Full-priority UI locale (§15), so deployments serving Serbian scans should set `eng,rus,srp`. |

> **VERIFIED note on OpenCV vs OCR.** VisionEngine's OpenCV pass (`HELIX_VISION_OPENCV_ENABLED`,
> `HELIX_VISION_SSIM_THRESHOLD`) is real and does image-similarity / near-duplicate work — but it is
> **not** text recognition. The `[GAP: 2]` "no OCR engine" status stands: OpenCV de-duplicates frames,
> it does not transcribe `#Comic`/`#Screenshot` text. Do not mistake OpenCV-enabled for OCR-capable.

## 9. Messengers (Herald) `[GAP: 3]`

**VERIFIED status (read from `vasic-digital/herald` source).** Herald exposes **two distinct Telegram
transports**, and Thready uses both for different jobs:

- The **MTProto user client** (`HERALD_MTPROTO_*`) signs in as a real Telegram **user account** and is
  the only transport that can **read full thread history / backfill** a channel. It exists in the code
  today but lives inside Herald's `qaherald` QA harness and is being **promoted to a first-class
  channel** for Thready (`[BUILD-NEW]` P0). This is the transport Thready's ingest depends on.
- The **Bot-API path** (`HERALD_TGRAM_*`) is a `@BotFather` bot used for **outbound status replies** and
  an optional inbound long-poll. It **cannot backfill channel history** — Telegram's bot-privacy
  boundary blocks a bot from reading other members' history — so it is a *reply/notify* transport, not
  a *reader* (VERIFIED: `herald/docs/research/telegram-bot-to-bot-constraint.md`).

**Max is an empty stub.** Only reserved env vars exist (`HERALD_MAX_BOT_TOKEN`, `HERALD_MAX_CHAT_ID`);
Herald's own `docs/guides/messengers/MAX.md` marks Max **`PLANNED` (HRD-NNN)** with "(not yet
implemented)". The adapter (Bot API + Go port of the OneMe user-WebSocket) is `[BUILD-NEW]` P0. Do not
expect Max reading before that lands. Sign-in flows:
[installation.md §7](./installation.md#7-messenger-sign-in--first-channel).

**Telegram — MTProto user client (reads history):**

| Variable | Type | Default | Provenance | Notes |
|----------|------|---------|------------|-------|
| `HERALD_MTPROTO_APP_ID` | secret (int) | unset | `[IN-HOUSE: herald]` (VERIFIED — `.env.example`) | `app_id` from my.telegram.org/apps (~5–8 digits). |
| `HERALD_MTPROTO_APP_HASH` | secret (32-hex) | unset | `[IN-HOUSE: herald]` (VERIFIED) | Paired `app_hash` from my.telegram.org/apps. |
| `HERALD_MTPROTO_PHONE` | string (E.164) | unset | `[IN-HOUSE: herald]` (VERIFIED) | Phone of the reading **user** account; first run triggers an SMS/app login code. |
| `HERALD_MTPROTO_PASSWORD` | secret | unset | `[IN-HOUSE: herald]` (VERIFIED) | Cloud 2FA password; only prompted if Telegram returns `SESSION_PASSWORD_NEEDED`. |
| `HERALD_MTPROTO_SESSION_FILE` | path | `~/.config/herald/mtproto.session` | `[IN-HOUSE: herald]` (VERIFIED) | Persisted MTProto session (avoids re-login). Store on an encrypted path. |

**Telegram — Bot API (status replies / notify):**

| Variable | Type | Default | Provenance | Notes |
|----------|------|---------|------------|-------|
| `HERALD_TGRAM_BOT_TOKEN` | secret | unset | `[IN-HOUSE: herald]` (VERIFIED) | `@BotFather` token (`NNNNNNNNNN:35-char`). **Cannot backfill history.** |
| `HERALD_TGRAM_CHAT_ID` | string | unset | `[IN-HOUSE: herald]` (VERIFIED) | Target chat: private DM positive, group negative, supergroup `-100<id>`. |
| `HERALD_TGRAM_LIVE_INBOUND` | bool | `false` | `[IN-HOUSE: herald]` (VERIFIED) | Enable the Bot-API inbound long-poll listener. |
| `HERALD_OPERATOR_IDS` | csv (int) | unset | `[IN-HOUSE: herald]` (VERIFIED) | Telegram user IDs allowed to issue operator commands. |

**Max (reserved — PLANNED, not implemented) & Thready messenger orchestration:**

| Variable | Type | Default | Provenance | Notes |
|----------|------|---------|------------|-------|
| `HERALD_MAX_BOT_TOKEN` | secret | unset | `[IN-HOUSE: herald]` (VERIFIED reserved) | Max Bot-API token — **adapter PLANNED, not built** (`herald/docs/guides/messengers/MAX.md`). |
| `HERALD_MAX_CHAT_ID` | string | unset | `[IN-HOUSE: herald]` (VERIFIED reserved) | Reserved; no code consumes it yet. |
| `THREADY_MESSENGER_SIGNIN_MODE` | enum `interactive\|noninteractive` | `interactive` | `[OPERATOR]` | Non-interactive reads all creds from env for headless deploy. |
| `THREADY_POLL_INTERVAL` | duration | `5m` | `[DEFAULT — adjustable]` | Scheduled wake frequency; event triggers supplement polling. |
| `THREADY_REPLY_ACCOUNT` | enum `robot\|user` | `robot` | `[DEFAULT — adjustable]` | Which account posts status replies (`robot` → Bot API, `user` → MTProto). |

> **Migration note (from Rev 1).** Earlier drafts used `HERALD_TELEGRAM_API_ID` / `_API_HASH` /
> `_PHONE` / `_2FA_PASSWORD` / `_SESSION_PATH`. Those names were an ASSUMPTION and are **wrong** — the
> VERIFIED Herald spellings are the `HERALD_MTPROTO_*` names above. If you copied an early `.env`,
> rename those five keys.

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

**Closed in Rev 2 (Pass 3):**

- ✅ `[CLOSED: cfg-messenger]` Messenger env-var names were an ASSUMPTION. **Verified at source** in
  `vasic-digital/herald/.env.example` (+ `docs/guides/messengers/`): the real names are
  `HERALD_MTPROTO_APP_ID/APP_HASH/PHONE/PASSWORD/SESSION_FILE` (user client),
  `HERALD_TGRAM_BOT_TOKEN/CHAT_ID/LIVE_INBOUND` (Bot API), `HERALD_OPERATOR_IDS`, and the reserved
  `HERALD_MAX_BOT_TOKEN/CHAT_ID`. §9 now carries the VERIFIED names.
- ✅ `[CLOSED: cfg-vision]` VisionEngine variables verified in `helix_track/vision_engine/.env.example`
  (`HELIX_VISION_OPENCV_ENABLED`, `HELIX_VISION_SSIM_THRESHOLD`, `HELIX_OLLAMA_*`,
  `HELIX_LLAMACPP_RPC_*`, `ASTICA_API_KEY`, provider enum widened) — §8.3.
- ✅ `[CLOSED: cfg-containers]` Additional `CONTAINERS_REMOTE_*` verified in
  `helix_track/containers/.env.example` — §5.

## Appendix A — Master environment-variable index

This is the single **one-row-per-variable** consolidated index the original request mandates
(*"All environment variables … MUST be properly documented"*). Columns are **Name · Purpose · Default
· Scope · Example**. *Scope* names which component reads the variable and in which environment it
applies (`all` = every service/env unless noted). Provenance/verification detail for each row lives in
the categorized section cross-referenced by the section number in the Name cell's group heading above;
`(V)` marks a VERIFIED module variable, `(A)` an ASSUMPTION/`THREADY_*` proposal, `(D)` a
`[DEFAULT — adjustable]`.

### A.1 Core runtime, environments, TLS

| Name | Purpose | Default | Scope | Example |
|------|---------|---------|-------|---------|
| `THREADY_ENV` (A) | Select env profile & safety rails | `development` | all services | `THREADY_ENV=production` |
| `THREADY_ENV_FILE` (A) | Explicit `.env` path | `./.env` | all services | `THREADY_ENV_FILE=/etc/thready/prod.env` |
| `THREADY_HTTP_ADDR` (A) | REST + WS/SSE bind | `0.0.0.0:8443` | API server | `THREADY_HTTP_ADDR=0.0.0.0:8443` |
| `THREADY_HTTP3_ENABLED` (V) | Enable HTTP/3 (QUIC) | `true` | API server | `THREADY_HTTP3_ENABLED=true` |
| `THREADY_HTTP_COMPRESSION` (D) | Response compression | `br,gzip` | API server | `THREADY_HTTP_COMPRESSION=br,gzip` |
| `THREADY_LOG_LEVEL` (V) | logrus level | `info` | all services | `THREADY_LOG_LEVEL=debug` |
| `THREADY_LOG_FORMAT` (V) | Log format | `json` (`text` dev) | all services | `THREADY_LOG_FORMAT=json` |
| `THREADY_REQUEST_TIMEOUT` (D) | Per-request server timeout | `30s` | API server | `THREADY_REQUEST_TIMEOUT=30s` |
| `THREADY_RATE_LIMIT_RPS` (V) | Per-identity rate cap | `100` | API server | `THREADY_RATE_LIMIT_RPS=100` |
| `THREADY_CORS_ORIGINS` (V) | Allowed browser origins | *(none)* | API server | `THREADY_CORS_ORIGINS=https://thready.hxd3v.com` |
| `THREADY_PORT_PREFIX` (V) | Deterministic dynamic-port base | unset | container stack | `THREADY_PORT_PREFIX=24` |
| `THREADY_PUBLIC_DOMAIN` (op) | Base domain | `thready.hxd3v.com` | all services | `THREADY_PUBLIC_DOMAIN=thready.hxd3v.com` |
| `THREADY_PUBLIC_BASE_URL` (A) | Absolute link base | `https://thready.hxd3v.com` | API/content | `THREADY_PUBLIC_BASE_URL=https://thready.hxd3v.com` |
| `LETS_ENCRYPT_EMAIL` (V) | ACME account email | unset | proxy/prod | `LETS_ENCRYPT_EMAIL=ops@hxd3v.com` |
| `LETS_ENCRYPT_CHALLENGE` (V) | ACME challenge type | `http-01` | proxy/prod | `LETS_ENCRYPT_CHALLENGE=dns-01` |
| `THREADY_TLS_MIN_VERSION` (D) | Minimum TLS | `1.3` | API/proxy | `THREADY_TLS_MIN_VERSION=1.3` |
| `CONTAINERS_REMOTE_ENABLED` (V) | Remote container distribution | `false` | deploy | `CONTAINERS_REMOTE_ENABLED=true` |
| `CONTAINERS_REMOTE_DEFAULT_SSH_USER` (V) | Deploy SSH user | `deploy` | deploy | `CONTAINERS_REMOTE_DEFAULT_SSH_USER=thready` |
| `CONTAINERS_REMOTE_DEFAULT_RUNTIME` (V) | Runtime | `podman` | deploy | `CONTAINERS_REMOTE_DEFAULT_RUNTIME=podman` |
| `CONTAINERS_REMOTE_DEFAULT_SSH_KEY` (V) | Deploy SSH key | unset | deploy | `CONTAINERS_REMOTE_DEFAULT_SSH_KEY=~/.ssh/thready_deploy` |
| `CONTAINERS_REMOTE_PORT_RANGE_START`/`_END` (V) | Tunnel/port range | `20000`/`30000` | deploy | `CONTAINERS_REMOTE_PORT_RANGE_START=20000` |
| `CONTAINERS_REMOTE_CONNECT_TIMEOUT` (V) | SSH connect timeout | `10s` | deploy | `CONTAINERS_REMOTE_CONNECT_TIMEOUT=10s` |
| `CONTAINERS_REMOTE_COMMAND_TIMEOUT` (V) | Remote command timeout | `5m` | deploy | `CONTAINERS_REMOTE_COMMAND_TIMEOUT=5m` |
| `CONTAINERS_REMOTE_SSH_CONTROL_MASTER` (V) | Reuse SSH master | `true` | deploy | `CONTAINERS_REMOTE_SSH_CONTROL_MASTER=true` |
| `CONTAINERS_REMOTE_SSH_CONTROL_PERSIST` (V) | SSH master linger | `60s` | deploy | `CONTAINERS_REMOTE_SSH_CONTROL_PERSIST=60s` |
| `CONTAINERS_REMOTE_SSH_MAX_CONNECTIONS` (V) | SSH concurrency/target | `10` | deploy | `CONTAINERS_REMOTE_SSH_MAX_CONNECTIONS=10` |
| `CONTAINERS_REMOTE_SCHEDULER` (V) | Multi-host placement | `roundrobin` | deploy | `CONTAINERS_REMOTE_SCHEDULER=leastloaded` |
| `CONTAINERS_REMOTE_VOLUME_TYPE` (V) | Default mount type | `volume` | deploy | `CONTAINERS_REMOTE_VOLUME_TYPE=volume` |

### A.2 Data, datastores, embeddings

| Name | Purpose | Default | Scope | Example |
|------|---------|---------|-------|---------|
| `THREADY_DB_DRIVER` (V) | Relational backend | `sqlite`/`postgres` | all services | `THREADY_DB_DRIVER=postgres` |
| `THREADY_DB_DSN` (V) | DB connection string | *(see quick-start)* | all services | `THREADY_DB_DSN=postgres://thready:***@db:5432/thready` |
| `THREADY_DB_MAX_OPEN_CONNS` (D) | pgx pool size | `32` | all services | `THREADY_DB_MAX_OPEN_CONNS=64` |
| `THREADY_DB_MAX_IDLE_CONNS` (D) | Idle pool | `8` | all services | `THREADY_DB_MAX_IDLE_CONNS=8` |
| `THREADY_DB_CONN_MAX_LIFETIME` (D) | Connection recycle | `30m` | all services | `THREADY_DB_CONN_MAX_LIFETIME=30m` |
| `THREADY_DB_MIGRATE_ON_BOOT` (V) | Auto-migrate | `true` dev/`false` prod | API/migrator | `THREADY_DB_MIGRATE_ON_BOOT=false` |
| `THREADY_DB_PARTITIONING` (gap) | Time-partition `posts` | `true` prod | DB | `THREADY_DB_PARTITIONING=true` |
| `THREADY_VECTOR_BACKEND` (V) | Vector store | `pgvector` | search/workers | `THREADY_VECTOR_BACKEND=pgvector` |
| `THREADY_VECTOR_DSN` (V) | Vector DB DSN | *(shares Postgres)* | search/workers | `THREADY_VECTOR_DSN=postgres://thready:***@db:5432/thready` |
| `THREADY_VECTOR_METRIC` (V) | Distance metric | `cosine` | search | `THREADY_VECTOR_METRIC=cosine` |
| `THREADY_EMBEDDING_DIM` (gap) | Embedding dimension | `1024` | search/workers | `THREADY_EMBEDDING_DIM=1024` |
| `THREADY_VECTOR_INDEX` (D) | ANN index | `hnsw` | search | `THREADY_VECTOR_INDEX=hnsw` |
| `THREADY_QDRANT_URL` (gap) | Qdrant endpoint (unverified) | unset | search | `THREADY_QDRANT_URL=http://qdrant:6333` |
| `THREADY_CACHE_BACKEND` (V) | Cache tier | `memory`/`redis` | all services | `THREADY_CACHE_BACKEND=redis` |
| `THREADY_CACHE_REDIS_URL` (V) | Redis URL | unset | all services | `THREADY_CACHE_REDIS_URL=redis://cache:6379/0` |
| `THREADY_CACHE_TTL` (D) | Entry TTL | `10m` | all services | `THREADY_CACHE_TTL=10m` |
| `THREADY_STORAGE_BACKEND` (V) | Asset store | `filesystem`/`minio` | asset service | `THREADY_STORAGE_BACKEND=minio` |
| `THREADY_STORAGE_ENDPOINT` (V) | MinIO endpoint | unset | asset service | `THREADY_STORAGE_ENDPOINT=https://minio:9000` |
| `THREADY_STORAGE_BUCKET` (A) | Object bucket | `thready-assets` | asset service | `THREADY_STORAGE_BUCKET=thready-assets` |
| `THREADY_STORAGE_ACCESS_KEY` (V) | Storage key (secret) | unset | asset service | `THREADY_STORAGE_ACCESS_KEY=***` |
| `THREADY_STORAGE_SECRET_KEY` (V) | Storage secret | unset | asset service | `THREADY_STORAGE_SECRET_KEY=***` |
| `THREADY_STORAGE_SIGNED_URL_TTL` (gap) | Signed-URL lifetime | `15m` | asset service | `THREADY_STORAGE_SIGNED_URL_TTL=15m` |
| `HELIX_EMBEDDING_PROVIDER` (V) | **Embedder — must be real** | `llama` (enforced) | search/workers | `HELIX_EMBEDDING_PROVIDER=llama` |
| `THREADY_EMBEDDING_BASE_URL` (V) | Embedding endpoint | `http://localhost:8080/v1` | search/workers | `THREADY_EMBEDDING_BASE_URL=http://llm:8080/v1` |
| `THREADY_EMBEDDING_MODEL` (R) | Embedding model | `jina-embeddings-v2-base-code` | search/workers | `THREADY_EMBEDDING_MODEL=jina-embeddings-v2-base-code` |
| `THREADY_EMBEDDING_API_KEY` (V) | Embedding key | unset | search/workers | `THREADY_EMBEDDING_API_KEY=***` |

### A.3 LLM, vision, messengers, downloads

| Name | Purpose | Default | Scope | Example |
|------|---------|---------|-------|---------|
| `HELIX_LLM_BASE_URL` (V) | Local llama.cpp URL | unset | workers | `HELIX_LLM_BASE_URL=http://llm:8080` |
| `HELIX_LLM_MODEL` (V) | Reasoning model | unset | workers | `HELIX_LLM_MODEL=Llama-3.1-70B-Instruct-Q4_K_M` |
| `HELIX_LLM_CODE_MODEL` (V) | Code model | unset | workers | `HELIX_LLM_CODE_MODEL=Qwen2.5-Coder` |
| `ANTHROPIC_API_KEY` / `OPENAI_API_KEY` / `GOOGLE_API_KEY` / `DEEPSEEK_API_KEY` (V) | Cloud LLM fallbacks | unset | workers | `OPENAI_API_KEY=sk-***` |
| `GROQ_/MISTRAL_/QWEN_/OPENROUTER_/COHERE_/TOGETHER_/XAI_/PERPLEXITY_/NVIDIA_API_KEY` (V) | More LLM adapters | unset | workers | `GROQ_API_KEY=***` |
| `CEREBRAS_/FIREWORKS_/HUGGINGFACE_/SAMBANOVA_/SILICONFLOW_API_KEY` (V) | More LLM adapters | unset | workers | `FIREWORKS_API_KEY=***` |
| `REPLICATE_API_TOKEN` (V) | Replicate adapter (note `_TOKEN`) | unset | workers | `REPLICATE_API_TOKEN=r8_***` |
| `THREADY_LLM_MAX_RETRIES` (D) | Provider retry ceiling | `5` | workers | `THREADY_LLM_MAX_RETRIES=5` |
| `THREADY_LLM_CIRCUIT_BREAKER` (V) | Break flapping providers | `true` | workers | `THREADY_LLM_CIRCUIT_BREAKER=true` |
| `HELIX_VISION_PROVIDER` (V) | Vision provider select | `auto` | vision workers | `HELIX_VISION_PROVIDER=auto` |
| `HELIX_VISION_TIMEOUT` (V) | Vision timeout (s) | `60` | vision workers | `HELIX_VISION_TIMEOUT=60` |
| `HELIX_VISION_MAX_IMAGE_SIZE` (V) | Downscale ceiling (px) | `4096` | vision workers | `HELIX_VISION_MAX_IMAGE_SIZE=4096` |
| `HELIX_VISION_OPENCV_ENABLED` (V) | OpenCV pre-pass | `true` | vision workers | `HELIX_VISION_OPENCV_ENABLED=true` |
| `HELIX_VISION_SSIM_THRESHOLD` (V) | Duplicate cutoff | `0.95` | vision workers | `HELIX_VISION_SSIM_THRESHOLD=0.95` |
| `HELIX_VISION_HOSTS` / `HELIX_VISION_USER` (V) | Remote vision fleet | unset | vision workers | `HELIX_VISION_HOSTS=thinker.local,amber.local` |
| `HELIX_OLLAMA_URL` / `HELIX_OLLAMA_MODEL` (V) | Local Ollama vision | `…:11434` / `minicpm-v:8b` | vision workers | `HELIX_OLLAMA_MODEL=minicpm-v:8b` |
| `HELIX_LLAMACPP_RPC_ENABLED`/`_WORKERS`/`_MODEL` (V) | llama.cpp RPC vision | `false` / unset / `~/models/…` | vision workers | `HELIX_LLAMACPP_RPC_ENABLED=false` |
| `ASTICA_API_KEY` / `KIMI_API_KEY` / `STEPFUN_API_KEY` (V) | Vision provider keys | unset | vision workers | `ASTICA_API_KEY=***` |
| `THREADY_OCR_PROVIDER` (BN) | OCR engine (pending) | `none` | vision workers | `THREADY_OCR_PROVIDER=none` |
| `THREADY_OCR_LANGS` (D) | OCR language packs | `eng,rus` | vision workers | `THREADY_OCR_LANGS=eng,rus,srp` |
| `HERALD_MTPROTO_APP_ID` (V) | Telegram user app_id | unset | ingest | `HERALD_MTPROTO_APP_ID=1234567` |
| `HERALD_MTPROTO_APP_HASH` (V) | Telegram user app_hash | unset | ingest | `HERALD_MTPROTO_APP_HASH=abcd…32hex` |
| `HERALD_MTPROTO_PHONE` (V) | Reading account phone | unset | ingest | `HERALD_MTPROTO_PHONE=+15551234567` |
| `HERALD_MTPROTO_PASSWORD` (V) | Cloud 2FA password | unset | ingest | `HERALD_MTPROTO_PASSWORD=***` |
| `HERALD_MTPROTO_SESSION_FILE` (V) | Session path | `~/.config/herald/mtproto.session` | ingest | `HERALD_MTPROTO_SESSION_FILE=/secure/mtproto.session` |
| `HERALD_TGRAM_BOT_TOKEN` (V) | Bot-API reply token | unset | reply | `HERALD_TGRAM_BOT_TOKEN=123:AA***` |
| `HERALD_TGRAM_CHAT_ID` (V) | Bot-API target chat | unset | reply | `HERALD_TGRAM_CHAT_ID=-1001234567890` |
| `HERALD_TGRAM_LIVE_INBOUND` (V) | Bot inbound long-poll | `false` | reply | `HERALD_TGRAM_LIVE_INBOUND=true` |
| `HERALD_OPERATOR_IDS` (V) | Operator user allowlist | unset | reply | `HERALD_OPERATOR_IDS=111,222` |
| `HERALD_MAX_BOT_TOKEN` / `HERALD_MAX_CHAT_ID` (V, reserved) | Max (PLANNED) | unset | *(none yet)* | `HERALD_MAX_BOT_TOKEN=` |
| `THREADY_MESSENGER_SIGNIN_MODE` (op) | Interactive vs headless | `interactive` | ingest | `THREADY_MESSENGER_SIGNIN_MODE=noninteractive` |
| `THREADY_POLL_INTERVAL` (D) | Poll frequency | `5m` | ingest | `THREADY_POLL_INTERVAL=2m` |
| `THREADY_REPLY_ACCOUNT` (D) | Who posts replies | `robot` | reply | `THREADY_REPLY_ACCOUNT=robot` |
| `THREADY_BOBA_URL` (V) | Torrent service | unset | workers | `THREADY_BOBA_URL=http://boba:8000` |
| `THREADY_BOBA_CALLBACK_URL` (gap) | Boba completion callback | derived | workers | `THREADY_BOBA_CALLBACK_URL=https://thready.hxd3v.com/v1/hooks/boba` |
| `THREADY_METUBE_URL` (V) | Video downloader | unset | workers | `THREADY_METUBE_URL=http://metube:8081` |
| `THREADY_METUBE_WEBHOOK_URL` (BN) | MeTube webhook (pending) | derived | workers | `THREADY_METUBE_WEBHOOK_URL=https://thready.hxd3v.com/v1/hooks/metube` |
| `THREADY_DOWNLOAD_MANAGER_URL` (BN) | Generic DL manager (pending) | unset | workers | `THREADY_DOWNLOAD_MANAGER_URL=http://dlm:8082` |
| `THREADY_DOWNLOAD_CONCURRENCY` (D) | Parallel downloads | `4` | workers | `THREADY_DOWNLOAD_CONCURRENCY=4` |
| `THREADY_GAME_DEFAULT_PLATFORMS` (D) | `#Game` targets | `PC-Windows,PS4,Android` | workers | `THREADY_GAME_DEFAULT_PLATFORMS=PC-Windows,PS4` |
| `THREADY_SOFTWARE_DEFAULT_OS` (D) | `#Software` OSes | `Windows,Linux,macOS` | workers | `THREADY_SOFTWARE_DEFAULT_OS=Windows,Linux` |

### A.4 Event bus, processing, auth, assets, observability, retention, branding

| Name | Purpose | Default | Scope | Example |
|------|---------|---------|-------|---------|
| `THREADY_EVENTBUS_BACKEND` (V) | Event transport | `inprocess`/`nats` | all services | `THREADY_EVENTBUS_BACKEND=nats` |
| `THREADY_NATS_URL` (V) | NATS URL | unset | all services | `THREADY_NATS_URL=nats://nats:4222` |
| `THREADY_NATS_STREAM` (A) | JetStream stream | `thready` | all services | `THREADY_NATS_STREAM=thready` |
| `THREADY_WORKERS` (D) | Worker-pool size | `32` | workers | `THREADY_WORKERS=64` |
| `THREADY_RETRY_MAX` (D) | Max retries/post | `5` | workers | `THREADY_RETRY_MAX=5` |
| `THREADY_RETRY_BASE` (D) | Back-off base | `2s` | workers | `THREADY_RETRY_BASE=2s` |
| `THREADY_RETRY_FACTOR` (D) | Back-off multiplier | `2.0` | workers | `THREADY_RETRY_FACTOR=2.0` |
| `THREADY_RETRY_CAP` (D) | Back-off ceiling | `5m` | workers | `THREADY_RETRY_CAP=5m` |
| `THREADY_POST_TIMEOUT` (D) | Per-post soft budget | `30m` | workers | `THREADY_POST_TIMEOUT=30m` |
| `THREADY_SKILL_CONCURRENCY` (D) | Per-Skill concurrency | `8` | workers | `THREADY_SKILL_CONCURRENCY=8` |
| `THREADY_JWT_SIGNING_ALG` (V) | JWT algorithm | `HS256`→`RS256` | auth/all | `THREADY_JWT_SIGNING_ALG=RS256` |
| `THREADY_JWT_SECRET` (V) | HS256 secret | unset | auth/all | `THREADY_JWT_SECRET=***≥32B` |
| `THREADY_JWT_PRIVATE_KEY_PATH`/`_PUBLIC_KEY_PATH` (V) | RS256/EdDSA PEMs | unset | auth/all | `THREADY_JWT_PRIVATE_KEY_PATH=/secure/jwt.pem` |
| `THREADY_ACCESS_TOKEN_TTL` (D) | Access token life | `15m` | auth | `THREADY_ACCESS_TOKEN_TTL=15m` |
| `THREADY_REFRESH_TOKEN_TTL` (D) | Refresh token life | `168h` | auth | `THREADY_REFRESH_TOKEN_TTL=168h` |
| `THREADY_IDLE_TIMEOUT` (D) | Web idle logout | `30m` | API/web | `THREADY_IDLE_TIMEOUT=30m` |
| `THREADY_MFA_REQUIRED_TIERS` (D) | Tiers forced to MFA | `root,account_admin` | auth | `THREADY_MFA_REQUIRED_TIERS=root,account_admin` |
| `THREADY_PASSWORD_MIN_LEN` (D) | Min password length | `12` | auth | `THREADY_PASSWORD_MIN_LEN=14` |
| `THREADY_ARGON2_MEMORY_KIB` (V) | Argon2id memory | `65536` | auth | `THREADY_ARGON2_MEMORY_KIB=65536` |
| `THREADY_API_KEY_HASH_PEPPER` (V) | API-key pepper | unset | auth | `THREADY_API_KEY_HASH_PEPPER=***` |
| `THREADY_ENCRYPTION_KEY` (V) | AES-256-GCM master | unset | security/all | `THREADY_ENCRYPTION_KEY=***32B` |
| `THREADY_ASSET_SERVICE_URL` (BN) | Asset Service (pending decouple) | unset | clients/workers | `THREADY_ASSET_SERVICE_URL=http://assets:8090` |
| `THREADY_MEDIA_DIR` (op) | Media root | `./data/media` | asset service | `THREADY_MEDIA_DIR=/data/media` |
| `THREADY_WEB_RENDITION_SUFFIX` (D) | `-web` rendition suffix | `-web` | asset service | `THREADY_WEB_RENDITION_SUFFIX=-web` |
| `THREADY_ENCRYPTED_ASSET_DIR` (V) | Sealed asset dir | `./data/secure` | asset service | `THREADY_ENCRYPTED_ASSET_DIR=/data/secure` |
| `THREADY_ASSET_DEDUP` (gap) | Content-hash dedup | `true` | asset service | `THREADY_ASSET_DEDUP=true` |
| `OTEL_EXPORTER_OTLP_ENDPOINT` (V) | OTLP collector | unset | all services | `OTEL_EXPORTER_OTLP_ENDPOINT=http://otel:4317` |
| `THREADY_METRICS_ADDR` (V) | Prometheus endpoint | `0.0.0.0:9090` | all services | `THREADY_METRICS_ADDR=0.0.0.0:9090` |
| `THREADY_CLICKHOUSE_DSN` (V) | Log/analytics sink | unset | all services | `THREADY_CLICKHOUSE_DSN=clickhouse://ch:9000/thready` |
| `THREADY_AUDIT_RETENTION` (D) | Audit retention | `8760h` | audit | `THREADY_AUDIT_RETENTION=8760h` |
| `THREADY_BACKUP_FULL_CRON` (op) | Full backup schedule | `0 3 * * *` | backup | `THREADY_BACKUP_FULL_CRON=0 3 * * *` |
| `THREADY_BACKUP_INCREMENTAL_CRON` (op) | Incremental schedule | `0 * * * *` | backup | `THREADY_BACKUP_INCREMENTAL_CRON=0 * * * *` |
| `FIREBASE_PROJECT_ID` (V) | Crashlytics/Distribution | unset | mobile | `FIREBASE_PROJECT_ID=thready-prod` |
| `THREADY_RETENTION_DEFAULT` (op) | Global retention | `indefinite` | all services | `THREADY_RETENTION_DEFAULT=indefinite` |
| `THREADY_BILLING_MODE` (op) | Billing model | `subscription+metered` | billing | `THREADY_BILLING_MODE=subscription+metered` |
| `THREADY_METERING_FLUSH` (D) | Meter flush interval | `1m` | billing | `THREADY_METERING_FLUSH=1m` |
| `THREADY_DEFAULT_LOCALE` (D) | UI locale default | `en` | web/mobile | `THREADY_DEFAULT_LOCALE=sr-Cyrl` |
| `THREADY_TRANSLATE_URL` (V) | HelixTranslate URL | unset | web/workers | `THREADY_TRANSLATE_URL=http://translate:8095` |
| `THREADY_BRAND_NAME` (op) | System brand name | `Thready` | web/content | `THREADY_BRAND_NAME=Thready` |
| `THREADY_BRAND_PRIMARY_COLOR` (V) | Brand base color | `#B6E376` | web/content | `THREADY_BRAND_PRIMARY_COLOR=#B6E376` |
| `THREADY_BRAND_LOGO_PATH` (op) | Default logo | `./assets/Logo.png` | web/content | `THREADY_BRAND_LOGO_PATH=./assets/Logo.png` |
| `THREADY_BRAND_SLOGAN` (op) | Footer slogan | `Made with love ♥ by Helix Development` | web/content | *(as default)* |
| `THREADY_THEME_DEFAULT` (V) | Default theme | `system` | web/mobile | `THREADY_THEME_DEFAULT=system` |

> *(op)* = `[OPERATOR]`, *(R)* = `[RESEARCH]`, *(gap)* = tied to a gap-register item, *(BN)* =
> `[BUILD-NEW]` (service not shipped; variable reserved for when it lands).

## Appendix B — Worked `.env` examples per environment

Three complete, copy-pasteable skeletons — **development**, **staging**, **production** — showing the
same variables at the settings each environment actually uses. Secrets are placeholders; never commit
real values (`chmod 600 .env`).

### B.1 Development (SQLite, in-process bus, local llama.cpp)

```dotenv
THREADY_ENV=development
THREADY_HTTP_ADDR=0.0.0.0:8443
THREADY_LOG_LEVEL=debug
THREADY_LOG_FORMAT=text
THREADY_DB_DRIVER=sqlite
THREADY_DB_DSN=file:./data/thready.db?_pragma=busy_timeout(5000)
THREADY_DB_MIGRATE_ON_BOOT=true
THREADY_VECTOR_BACKEND=pgvector
THREADY_VECTOR_DSN=postgres://thready:thready@localhost:5432/thready?sslmode=disable
THREADY_EMBEDDING_DIM=1024
HELIX_EMBEDDING_PROVIDER=llama
THREADY_EMBEDDING_BASE_URL=http://localhost:8080/v1
THREADY_EMBEDDING_MODEL=jina-embeddings-v2-base-code
THREADY_EVENTBUS_BACKEND=inprocess
THREADY_CACHE_BACKEND=memory
THREADY_STORAGE_BACKEND=filesystem
THREADY_MEDIA_DIR=./data/media
THREADY_JWT_SIGNING_ALG=HS256
THREADY_JWT_SECRET=dev-only-change-me-32-bytes-minimum
HERALD_MTPROTO_APP_ID=
HERALD_MTPROTO_APP_HASH=
HERALD_MTPROTO_PHONE=
```

### B.2 Staging (Postgres, NATS, MinIO — mirrors prod, smaller)

```dotenv
THREADY_ENV=staging
THREADY_HTTP_ADDR=0.0.0.0:8443
THREADY_LOG_LEVEL=info
THREADY_LOG_FORMAT=json
THREADY_PUBLIC_DOMAIN=sta.thready.hxd3v.com
THREADY_DB_DRIVER=postgres
THREADY_DB_DSN=postgres://thready:${DB_PW}@sta-db:5432/thready?sslmode=require
THREADY_DB_MIGRATE_ON_BOOT=false
THREADY_VECTOR_BACKEND=pgvector
THREADY_VECTOR_DSN=postgres://thready:${DB_PW}@sta-db:5432/thready?sslmode=require
THREADY_EMBEDDING_DIM=1024
HELIX_EMBEDDING_PROVIDER=llama
THREADY_EMBEDDING_BASE_URL=http://llm:8080/v1
THREADY_EVENTBUS_BACKEND=nats
THREADY_NATS_URL=nats://sta-nats:4222
THREADY_CACHE_BACKEND=redis
THREADY_CACHE_REDIS_URL=redis://sta-cache:6379/0
THREADY_STORAGE_BACKEND=minio
THREADY_STORAGE_ENDPOINT=https://sta-minio:9000
THREADY_STORAGE_BUCKET=thready-assets-sta
THREADY_JWT_SIGNING_ALG=RS256
THREADY_JWT_PRIVATE_KEY_PATH=/secure/jwt-sta.pem
THREADY_JWT_PUBLIC_KEY_PATH=/secure/jwt-sta.pub.pem
THREADY_MFA_REQUIRED_TIERS=root,account_admin
OTEL_EXPORTER_OTLP_ENDPOINT=http://sta-otel:4317
THREADY_CLICKHOUSE_DSN=clickhouse://sta-ch:9000/thready
HERALD_MTPROTO_SESSION_FILE=/secure/mtproto-sta.session
THREADY_MESSENGER_SIGNIN_MODE=noninteractive
```

### B.3 Production (hardened; RS256, backups, partitioning)

```dotenv
THREADY_ENV=production
THREADY_HTTP_ADDR=0.0.0.0:8443
THREADY_HTTP3_ENABLED=true
THREADY_LOG_LEVEL=info
THREADY_LOG_FORMAT=json
THREADY_PUBLIC_DOMAIN=thready.hxd3v.com
THREADY_TLS_MIN_VERSION=1.3
LETS_ENCRYPT_EMAIL=ops@hxd3v.com
THREADY_DB_DRIVER=postgres
THREADY_DB_DSN=postgres://thready:${DB_PW}@db:5432/thready?sslmode=require
THREADY_DB_MIGRATE_ON_BOOT=false
THREADY_DB_MAX_OPEN_CONNS=64
THREADY_DB_PARTITIONING=true
THREADY_VECTOR_BACKEND=pgvector
THREADY_VECTOR_DSN=postgres://thready:${DB_PW}@db:5432/thready?sslmode=require
THREADY_EMBEDDING_DIM=1024
THREADY_VECTOR_INDEX=hnsw
HELIX_EMBEDDING_PROVIDER=llama
THREADY_EMBEDDING_BASE_URL=http://llm:8080/v1
THREADY_EVENTBUS_BACKEND=nats
THREADY_NATS_URL=nats://nats:4222
THREADY_WORKERS=64
THREADY_CACHE_BACKEND=redis
THREADY_CACHE_REDIS_URL=redis://cache:6379/0
THREADY_STORAGE_BACKEND=minio
THREADY_STORAGE_ENDPOINT=https://minio:9000
THREADY_STORAGE_BUCKET=thready-assets
THREADY_JWT_SIGNING_ALG=RS256
THREADY_JWT_PRIVATE_KEY_PATH=/secure/jwt.pem
THREADY_JWT_PUBLIC_KEY_PATH=/secure/jwt.pub.pem
THREADY_ACCESS_TOKEN_TTL=15m
THREADY_REFRESH_TOKEN_TTL=168h
THREADY_MFA_REQUIRED_TIERS=root,account_admin
THREADY_ENCRYPTION_KEY=${AES_MASTER_KEY}
THREADY_ENCRYPTED_ASSET_DIR=/data/secure
OTEL_EXPORTER_OTLP_ENDPOINT=http://otel:4317
THREADY_METRICS_ADDR=0.0.0.0:9090
THREADY_CLICKHOUSE_DSN=clickhouse://ch:9000/thready
THREADY_AUDIT_RETENTION=8760h
THREADY_BACKUP_FULL_CRON=0 3 * * *
THREADY_BACKUP_INCREMENTAL_CRON=0 * * * *
THREADY_RETENTION_DEFAULT=indefinite
THREADY_BILLING_MODE=subscription+metered
THREADY_DEFAULT_LOCALE=en
THREADY_MESSENGER_SIGNIN_MODE=noninteractive
HERALD_MTPROTO_SESSION_FILE=/secure/mtproto.session
```

---

*Made with love ♥ by Helix Development.*
