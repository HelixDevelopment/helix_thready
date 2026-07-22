# Helix Thready — Configuration Loader (`digital.vasic.threadyconfig`)

The typed, validated **configuration loader** for Helix Thready. It is the
single grounding point for the environment-variable reference documented in
`docs/public/research/mvp/user-guides/configuration.md` (Appendix A — the
Master environment-variable index of **162** variables). It parses the
documented `THREADY_*`, `HELIX_*`, `HERALD_*`, `CONTAINERS_REMOTE_*`,
`LETS_ENCRYPT_*`, `OTEL_*` and cloud-provider variables into a strongly-typed
`Config`, applies the documented defaults, validates format and
required-in-production constraints returning an **aggregated** error, and
**redacts** secrets from its string forms.

- **Module path:** `digital.vasic.threadyconfig`
- **Go:** 1.26, **stdlib only** (no third-party dependencies)
- **Standalone module:** not in the parent `implementation/go.work`, so run
  every Go command with **`GOWORK=off`**.

```
cd implementation/config
GOWORK=off go build ./... && GOWORK=off go vet ./... \
  && GOWORK=off gofmt -l . && GOWORK=off go test ./... -v -count=1
```

## Public API

```go
// Parse the process environment.
cfg, err := threadyconfig.LoadFromEnv()

// Or inject a getenv (the seam the whole test-suite uses).
cfg, err := threadyconfig.Load(func(k string) string { return lookup[k] })
if err != nil {
    // *MultiError — every problem at once, each naming its variable.
    log.Fatal(err)
}

// Safe to log: secrets are masked.
log.Printf("config: %s", cfg) // or cfg.Redacted()

// Parse a .env file into a raw map (feed into Load via a closure).
m, err := threadyconfig.ParseDotEnv(file)
```

| Symbol | Purpose |
|--------|---------|
| `Load(getenv func(string) string) (*Config, error)` | Parse → default → validate. Returns `nil, *MultiError` on any problem. `nil` getenv is treated as an empty environment. |
| `LoadFromEnv() (*Config, error)` | `Load(os.Getenv)`. |
| `ParseDotEnv(io.Reader) (map[string]string, error)` | `.env` parser: comments, blank lines, `export ` prefix, single/double quotes, `=`-in-value, inline comments, `\n`/`\t` escapes. |
| `Config.String() string` | Human-readable dump with **all secrets masked** — safe to log. |
| `Config.Redacted() *Config` | A copy with secret fields replaced by `***REDACTED***`; the receiver is not mutated. |
| `Config.Database.SQLitePath() string` | On-disk SQLite path parsed from the DSN when `Driver == "sqlite"`. |
| `MultiError` | Aggregated error; implements `error` + `Unwrap() []error` (works with `errors.Is/As`). |

## The `Config` shape

`Config` is grouped by subsystem. Each field carries a doc-comment naming its
source environment variable:

```
Core          THREADY_ENV, _ENV_FILE, _LOG_LEVEL, _LOG_FORMAT, _PUBLIC_DOMAIN, _PUBLIC_BASE_URL
Deployment    THREADY_HTTP_ADDR, _HTTP3_ENABLED, _REQUEST_TIMEOUT, _RATE_LIMIT_RPS, _TLS_MIN_VERSION,
              LETS_ENCRYPT_*, + Remote{} (all CONTAINERS_REMOTE_*)
Database      THREADY_DB_DRIVER, _DSN, _MAX_OPEN_CONNS, _MAX_IDLE_CONNS, _CONN_MAX_LIFETIME,
              _MIGRATE_ON_BOOT, _PARTITIONING     (+ SQLitePath())
Vector        THREADY_VECTOR_BACKEND, _DSN, _METRIC, _INDEX, THREADY_QDRANT_URL
Embeddings    HELIX_EMBEDDING_PROVIDER, THREADY_EMBEDDING_BASE_URL, _MODEL, _DIM, _API_KEY
LLM           HELIX_LLM_BASE_URL, _MODEL, _CODE_MODEL, THREADY_LLM_MAX_RETRIES, _CIRCUIT_BREAKER,
              CloudProviderKeys{} (19 cloud keys: ANTHROPIC/OPENAI/…/REPLICATE)
Vision        HELIX_VISION_*, HELIX_OLLAMA_*, HELIX_LLAMACPP_RPC_*, ProviderKeys{} (ASTICA/KIMI/STEPFUN)
OCR           THREADY_OCR_PROVIDER, _LANGS
Cache         THREADY_CACHE_BACKEND, _REDIS_URL, _TTL
Storage       THREADY_STORAGE_*, THREADY_MEDIA_DIR, _WEB_RENDITION_SUFFIX, _ENCRYPTED_ASSET_DIR,
              _ASSET_DEDUP, _ASSET_SERVICE_URL
Messengers    Telegram{HERALD_MTPROTO_*, HERALD_TGRAM_*}, Max{HERALD_MAX_*},
              THREADY_MESSENGER_SIGNIN_MODE, _POLL_INTERVAL, _REPLY_ACCOUNT, HERALD_OPERATOR_IDS
Downloads     THREADY_BOBA_*, THREADY_METUBE_*, THREADY_DOWNLOAD_*, game/software defaults
EventBus      THREADY_EVENTBUS_BACKEND, THREADY_NATS_URL, _NATS_STREAM
Workers       THREADY_WORKERS, THREADY_RETRY_*, THREADY_POST_TIMEOUT, _SKILL_CONCURRENCY
Auth          THREADY_JWT_*, _ACCESS_TOKEN_TTL, _REFRESH_TOKEN_TTL, _IDLE_TIMEOUT, _MFA_REQUIRED_TIERS,
              _PASSWORD_MIN_LEN, _ARGON2_MEMORY_KIB, _API_KEY_HASH_PEPPER, _ENCRYPTION_KEY
Observability OTEL_EXPORTER_OTLP_ENDPOINT, THREADY_METRICS_ADDR, _CLICKHOUSE_DSN, _AUDIT_RETENTION,
              _BACKUP_*_CRON, FIREBASE_PROJECT_ID
Billing       THREADY_BILLING_MODE, _METERING_FLUSH, _RETENTION_DEFAULT
Branding      THREADY_DEFAULT_LOCALE, THREADY_TRANSLATE_URL, THREADY_BRAND_*, THREADY_THEME_DEFAULT
```

## Behaviour

**Defaults** follow Appendix A verbatim, including the environment-sensitive
ones: `THREADY_LOG_FORMAT` defaults to `text` in `development` and `json`
otherwise; `THREADY_DB_MIGRATE_ON_BOOT` defaults `true` outside production and
`false` in production; `THREADY_DB_PARTITIONING` defaults `true` only in
production.

**Validation** is aggregated: `Load` collects *every* format error (bad
integer / float / duration / URL / enum, each naming its variable) **and**
every required-field / cross-field problem, then returns them together as a
single `*MultiError`. In `production` the loader requires `THREADY_DB_DSN`, a
JWT credential matching the signing algorithm, and `THREADY_ENCRYPTION_KEY`;
backend selectors pull in their dependencies in every environment
(redis→URL, nats→URL, minio/s3→endpoint, qdrant→URL, postgres→DSN).

**Secret redaction** covers tokens, keys, passwords, peppers, the encryption
key, and credential-bearing DSNs/URLs (DB/vector/clickhouse/redis) plus the
cloud-provider and vision-provider key maps. `Config.String()` and
`Config.Redacted()` never emit a raw secret.

## Scope & honesty

`ParseDotEnv` returns values **verbatim** — it does **not** perform `${VAR}`
interpolation; the documented Appendix B skeletons use shell placeholders that
the environment expands before the process starts. Cron expressions and
free-form list/path values are stored but not deep-format-checked. See
`EVIDENCE.md` for the full documented-variable coverage accounting (162/162
read; ~95 format-validated) and the verbatim `build / vet / gofmt / test`
output.
