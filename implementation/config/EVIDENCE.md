# EVIDENCE — Helix Thready Configuration Loader (`digital.vasic.threadyconfig`)

Physical, reproducible evidence for the typed configuration-loader module. No
bluff: every command below was run in this module directory and its real output
is pasted verbatim.

> This module is **not** listed in the parent `implementation/go.work`, so it is
> a standalone module and every command **must** be run with `GOWORK=off`.

Reproduce with:

```
cd implementation/config
GOWORK=off go build ./... \
  && GOWORK=off go vet ./... \
  && GOWORK=off gofmt -l . \
  && GOWORK=off go test ./... -v -count=1
```

## Environment

```
$ go version
go version go1.26.4-X:nodwarf5 linux/amd64
```

## `GOWORK=off go build ./...`

```
$ GOWORK=off go build ./...
(exit 0 — no output = success)
```

## `GOWORK=off go vet ./...`

```
$ GOWORK=off go vet ./...
(exit 0 — no output = success)
```

## `GOWORK=off gofmt -l .`

```
$ GOWORK=off gofmt -l .
(empty — every file is already gofmt-clean)
```

## `GOWORK=off go test ./... -v -count=1`

```
=== RUN   TestLoad_FullEnvParsesEveryFieldKind
--- PASS: TestLoad_FullEnvParsesEveryFieldKind (0.00s)
=== RUN   TestLoad_DefaultsAppliedOnEmptyEnv
--- PASS: TestLoad_DefaultsAppliedOnEmptyEnv (0.00s)
=== RUN   TestConfig_SQLitePath
--- PASS: TestConfig_SQLitePath (0.00s)
=== RUN   TestConfig_RedactionHidesSecrets
--- PASS: TestConfig_RedactionHidesSecrets (0.00s)
=== RUN   TestLoad_RoundTripDocumentedDevExample
--- PASS: TestLoad_RoundTripDocumentedDevExample (0.00s)
=== RUN   TestLoad_RoundTripDocumentedProdExample
--- PASS: TestLoad_RoundTripDocumentedProdExample (0.00s)
=== RUN   TestParseDotEnv_EdgeCases
--- PASS: TestParseDotEnv_EdgeCases (0.00s)
=== RUN   TestParseDotEnv_MissingEqualsIsError
--- PASS: TestParseDotEnv_MissingEqualsIsError (0.00s)
=== RUN   TestParseDotEnv_InvalidKeyIsError
--- PASS: TestParseDotEnv_InvalidKeyIsError (0.00s)
=== RUN   TestParseDotEnv_Empty
--- PASS: TestParseDotEnv_Empty (0.00s)
=== RUN   TestLoad_MissingRequiredInProductionAggregates
--- PASS: TestLoad_MissingRequiredInProductionAggregates (0.00s)
=== RUN   TestLoad_BadNumericIsNamedError
--- PASS: TestLoad_BadNumericIsNamedError (0.00s)
=== RUN   TestLoad_BadDurationIsNamedError
--- PASS: TestLoad_BadDurationIsNamedError (0.00s)
=== RUN   TestLoad_BadURLIsNamedError
--- PASS: TestLoad_BadURLIsNamedError (0.00s)
=== RUN   TestLoad_BadEnumIsNamedError
--- PASS: TestLoad_BadEnumIsNamedError (0.00s)
=== RUN   TestLoad_MultipleBadValuesAllAggregated
--- PASS: TestLoad_MultipleBadValuesAllAggregated (0.00s)
=== RUN   TestLoad_ConditionalBackendRequirements
--- PASS: TestLoad_ConditionalBackendRequirements (0.00s)
=== RUN   TestLoad_ShortJWTSecretRejected
--- PASS: TestLoad_ShortJWTSecretRejected (0.00s)
=== RUN   TestLoad_ShortEncryptionKeyRejected
--- PASS: TestLoad_ShortEncryptionKeyRejected (0.00s)
=== RUN   TestMultiError_UnwrapAndSingle
--- PASS: TestMultiError_UnwrapAndSingle (0.00s)
=== RUN   TestLoad_NilGetenvUsesDefaults
--- PASS: TestLoad_NilGetenvUsesDefaults (0.00s)
PASS
ok  	digital.vasic.threadyconfig	0.003s
```

**21 tests, 21 pass, 0 fail, 0 skip.** No test was deleted, skipped, or
weakened to reach green.

## Coverage

```
$ GOWORK=off go test ./... -cover -count=1
ok  	digital.vasic.threadyconfig	0.003s	coverage: 89.6% of statements
```

## Race detector

```
$ GOWORK=off go test ./... -race -count=1
ok  	digital.vasic.threadyconfig	1.015s
```

## Documented-variable coverage (honest accounting)

Source of truth: `docs/public/research/mvp/user-guides/configuration.md`,
**Appendix A — Master environment-variable index**, which enumerates **162**
individual variables across four sub-tables (A.1 core/TLS/deploy — 29;
A.2 data/embeddings — 26; A.3 LLM/vision/messengers/downloads — 63;
A.4 bus/auth/assets/observability/billing/branding — 44).

**Coverage: 162 / 162 documented variables are read into the typed `Config`
(100% read-coverage).** Every variable in Appendix A is consumed by `Load`:

- Discrete typed fields for the structured settings (enums, ints, floats,
  bools, `time.Duration`, URL-validated strings) — this is the bulk of the 162.
- The 19 cloud-LLM credential vars (`ANTHROPIC_API_KEY` … `REPLICATE_API_TOKEN`)
  are captured into `Config.LLM.CloudProviderKeys` (secret map).
- The 3 vision-provider credential vars (`ASTICA/KIMI/STEPFUN_API_KEY`) are
  captured into `Config.Vision.ProviderKeys` (secret map).

**Depth of validation (honest — not every field is format-checked):**

- **Format-validated** (enum / numeric / float / duration / URL-shape): ~95
  variables (all the enums, all `*_TIMEOUT`/`*_TTL`/`*_INTERVAL`/`*_LIFETIME`
  durations, all counts/sizes/dims, all `*_URL`/`*_ENDPOINT`/`*_BASE_URL`
  fields, all `*_ENABLED`/`*_ON_*` booleans).
- **Required-in-production / conditional-required**: `THREADY_DB_DSN`,
  `THREADY_JWT_SECRET` (HS256) or `THREADY_JWT_PRIVATE_KEY_PATH` +
  `THREADY_JWT_PUBLIC_KEY_PATH` (RS256/EdDSA), `THREADY_ENCRYPTION_KEY`;
  plus backend-conditional: redis→`THREADY_CACHE_REDIS_URL`,
  nats→`THREADY_NATS_URL`, minio/s3→`THREADY_STORAGE_ENDPOINT`,
  qdrant→`THREADY_QDRANT_URL`, postgres→`THREADY_DB_DSN`. Secret-strength
  checks on the JWT HS256 secret and the AES encryption key (≥32 bytes).
- **String pass-through with documented defaults** (no deep format check, by
  design — free-form values): e.g. `THREADY_CORS_ORIGINS`, `THREADY_OCR_LANGS`,
  `THREADY_MFA_REQUIRED_TIERS`, `THREADY_*_CRON`, `THREADY_BRAND_*`,
  `HERALD_OPERATOR_IDS`, `THREADY_GAME_DEFAULT_PLATFORMS`,
  `THREADY_SOFTWARE_DEFAULT_OS`, session/key file paths, `FIREBASE_PROJECT_ID`.

**Deferred / not independently modelled (deliberate, disclosed):**

- Cron-expression *syntax* validation for `THREADY_BACKUP_FULL_CRON` /
  `THREADY_BACKUP_INCREMENTAL_CRON` — stored, not parsed.
- `${VAR}` interpolation inside `.env` values — `ParseDotEnv` returns values
  **verbatim**; shell/compose-style variable expansion is intentionally left to
  the caller/orchestrator (the documented Appendix B skeletons use `${DB_PW}` /
  `${AES_MASTER_KEY}` placeholders that a real shell expands before the process
  starts; the prod round-trip test injects concrete values to reflect that).
- The 22 credential vars live in secret maps rather than as 22 named struct
  fields; they are read and redacted, but not each individually enum/format
  checked (a key's *shape* is provider-specific and out of scope).

## Secret-redaction proof

`TestConfig_RedactionHidesSecrets` loads a config with a known Telegram bot
token, JWT secret, OpenAI key and a DSN password, then asserts each raw secret
string is **absent** from `Config.String()` while non-secret fields
(`Thready`, `0.0.0.0:8443`) are **present** and the `***REDACTED***` mask
appears. `TestLoad_RoundTripDocumentedProdExample` additionally asserts the DSN
password never surfaces in `String()`. Both pass (see run above).
