<!--
  Title           : Helix Thready — Database Area (Index)
  Classification  : PUBLIC
  Location        : docs/public/research/mvp/database/index.md
  Status          : Draft — v0.1
  Revision        : 1 (2026-07-21)
  Author          : Helix Thready documentation swarm (database)
  Related         : ../index.md ../CONVENTIONS.md
                    ../architecture/index.md ../api/index.md ../deployment/index.md
-->

# Helix Thready — Database Area (Index)

| Rev | Date | Author | Change |
|-----|------|--------|--------|
| 1 | 2026-07-21 | swarm (database) | Initial database pack: ERD, relational + vector DDL, indexing, partitioning, retention/archive, migrations |
| 2 | 2026-07-21 | reviewer (database) | Review pass: registered `ATM-DB-033` (MinIO signed-URL parity); diagram-consistency fixes across the pack |
| 3 | 2026-07-22 | swarm (database, Pass 3) | Depth pass: shipped runnable migrations `0002`–`0007`; full column-level data dictionary in the relational DDL; per-index rationale matrix; HASH/LIST partition DDL; new `constraints-and-integrity.md`; source-confirmed that the pgvector adapter's `Search` emits no tenant filter (strengthens `ATM-DB-013`) |
| 4 | 2026-07-22 | critic (database, Pass 4) | Completeness pass: added §8 the consolidated **15-test-type coverage matrix** (every mandated type mapped to a concrete DB test bank or explicitly marked delegated/N-A with rationale — closes the CONVENTIONS §6 test-coverage requirement for the data layer); backed the previously-asserted `security-7.1` "encrypted-yet-searchable" claim with a concrete redacted-embedding spec in `schema-vector.sql` |

This is the canonical entry point for the Helix Thready **Database** area. It specifies the
complete, implementation-ready data layer: the entity model, PostgreSQL + pgvector DDL,
indexing, partitioning/scaling, retention/archive, and the migration strategy — all grounded
in the authoritative research (final request §2.1, §3, §6, §7, §14, Q1–Q45) and the in-house
`digital.vasic.database` / `digital.vasic.vectordb` / `digital.vasic.embeddings` modules.

## Table of Contents

1. [Upstream / Downstream dependencies](#1-upstream--downstream-dependencies)
2. [Documents in this area](#2-documents-in-this-area)
3. [Decision snapshot (from the matrix)](#3-decision-snapshot-from-the-matrix)
4. [Provenance & verification summary](#4-provenance--verification-summary)
5. [Gaps addressed](#5-gaps-addressed)
6. [Open items](#6-open-items)
7. [Entity coverage checklist](#7-entity-coverage-checklist)
8. [Database test-type coverage (15 mandated types)](#8-database-test-type-coverage-15-mandated-types)

---

## 1. Upstream / Downstream dependencies

**Upstream (this area consumes):**

- **Architecture** ([`../architecture/`](../architecture/index.md)) — component boundaries,
  the relational↔semantic split, event model, and concurrency model that the schema realises.
- **Authoritative research** — `helix_thready_research_request_final.md` (decision matrix,
  §2.1 data layer, §3 workflow, §6 users, §7 assets, Q1/Q2/Q11/Q12/Q14/Q30/Q40/Q41),
  `helix_thready_subsystem_gaps_and_improvements.md` (P0/P1/P2 gap register).
- **In-house modules** — `digital.vasic.database` (`pkg/migration.Runner`, `Config`,
  drivers), `digital.vasic.vectordb` (pgvector `Client`), `digital.vasic.embeddings`
  (`Dimensions()` per provider), `digital.vasic.background` (claim queue),
  `digital.vasic.storage` (cold tier), `digital.vasic.security` (column encryption).

**Downstream (these areas consume this one):**

- **API** ([`../api/`](../api/index.md)) — OpenAPI resources map 1:1 to these entities;
  `/v1/search` uses the vector reference model; pagination uses the time-partition keys.
- **Deployment** ([`../deployment/`](../deployment/index.md)) — runs migrations at deploy,
  provisions read replicas, backup/DR (RPO 1 h / RTO 4 h), MinIO/S3 cold tier.
- **Testing** ([`../testing/`](../testing/index.md)) — DB test bank: migration apply/rollback
  on real Postgres, `EXPLAIN` index assertions, search p95 < 500 ms, archive chaos tests.
- **Development** ([`../development/`](../development/index.md)) — the `ATM-DB-*` workable
  items tracked below.

---

## 2. Documents in this area

| Document | Purpose |
|----------|---------|
| [`erd.md`](./erd.md) | Full entity-relationship model: master ERD + 5 domain ERDs (Mermaid + prose) + entity dictionary + referential-integrity strategy |
| [`schema-relational.sql`](./schema-relational.sql) | Production PostgreSQL DDL for every entity (extensions, tables, constraints, triggers, partition parents) |
| [`schema-vector.sql`](./schema-vector.sql) | pgvector DDL: collections referencing relational PKs, cosine ANN indexes, dimension policy |
| [`indexing.md`](./indexing.md) | Index catalogue (btree/GIN/trigram/FTS/partial/ANN), query-path map, SLO tuning, hot claim index |
| [`partitioning.md`](./partitioning.md) | Time-partitioned firehose tables, read replicas, maintenance job, pooling & pgvector co-location tuning |
| [`retention-archive.md`](./retention-archive.md) | Keep-indefinitely + per-account overrides, archive lifecycle, GDPR-aware erasure/export |
| [`migration-strategy.md`](./migration-strategy.md) | `migration.Runner` contract, expand-contract, rollback, advisory lock, verified caveats, roadmap |
| [`constraints-and-integrity.md`](./constraints-and-integrity.md) | Enforcement-layer model, CHECK-domain catalogue, FK on-delete matrix, append-only audit + soft-ref validation triggers (closes `ATM-DB-004`) |
| [`migrations/0001_init.sql`](./migrations/0001_init.sql) | Runnable migration (Up/Down): tenancy + ingestion + processing core |
| [`migrations/0002_classification.sql`](./migrations/0002_classification.sql) | Runnable migration: hashtags, categories, join tables |
| [`migrations/0003_assets.sql`](./migrations/0003_assets.sql) | Runnable migration: skills, skill_runs, generated_artifacts, assets, asset_links |
| [`migrations/0004_billing.sql`](./migrations/0004_billing.sql) | Runnable migration: plans, subscriptions, usage_records, invoices |
| [`migrations/0005_events_audit.sql`](./migrations/0005_events_audit.sql) | Runnable migration: events (partitioned), event_subscriptions, audit_log (partitioned), archived_partitions |
| [`migrations/0006_vector_collections.sql`](./migrations/0006_vector_collections.sql) | Runnable migration: `vectordb_*` collection tables (adapter-parity DDL) |
| [`migrations/0007_secondary_indexes.sql`](./migrations/0007_secondary_indexes.sql) | Runnable migration (non-transactional path): all secondary + FTS + vector ANN indexes |
| [`diagrams/`](./diagrams/) | Mermaid `.mmd` sources (siblings of every embedded diagram) |

> Rendered PNG/SVG exported via Docs Chain (§11.4.65). Every diagram in this area has a
> sibling `.mmd` in [`diagrams/`](./diagrams/) and an immediate multi-paragraph prose
> explanation per [CONVENTIONS.md §4](../CONVENTIONS.md).

---

## 3. Decision snapshot (from the matrix)

| Concern | Decision | Provenance |
|---------|----------|------------|
| Relational DB | `digital.vasic.database` — SQLite dev / PostgreSQL prod; `pkg/migration.Runner` | `[IN-HOUSE: database]` |
| Vector DB | `digital.vasic.vectordb` pgvector backend, **cosine** `<=>`; Qdrant swap by config | `[IN-HOUSE: vectordb]` Q1 |
| Embeddings | `digital.vasic.embeddings` → HelixLLM `/v1/embeddings`; `voyage-code-3` (1024) / `jina-embeddings-v2-base-code` (768) | `[IN-HOUSE]` Q15 |
| Scale | Large / multi-tenant; 10k+ posts/day, 50 TB+ assets → partition + replicas from day one | `[OPERATOR]` Q2 |
| Retention | Keep indefinitely + per-account overrides | `[OPERATOR]` Q12 |
| Billing | Subscription + metered from day one | `[OPERATOR]` Q11 |
| SLO | Search < 500 ms, API p95 < 150 ms | `[OPERATOR]` Q14 |
| Backup/DR | Daily full + hourly DB incrementals; RPO ≈ 1 h, RTO ≈ 4 h | `[OPERATOR]` Q41/Q45 |
| Migrations | `pkg/migration.Runner` (up/down), expand-contract, tested rollback | `[IN-HOUSE]` Q30 |

---

## 4. Provenance & verification summary

Following the anti-bluff quality bar ([CONVENTIONS.md §7](../CONVENTIONS.md)), claims are
tagged VERIFIED (read at source) vs ASSUMPTION.

**VERIFIED (read at module source under `/home/milos/Factory/projects/tools_and_research/helix_code/submodules/`):**

- `migration.Migration{Version,Name,Up,Down}`, `NewRunner(db, "schema_migrations")`,
  `Init/Applied/Apply/Rollback/RollbackWith`; each migration applied in one transaction;
  `Rollback` is inert and instructs `RollbackWith` (`database/pkg/migration/migration.go`).
- The runner's tracking-table writes use `?` placeholders; the pgx transport
  (`postgres.Client.Exec` / `pgTx.Exec`) does **not** rewrite them — a real Postgres-path
  caveat (`ATM-DB-001`, see [migration-strategy.md §8](./migration-strategy.md#8-verified-caveats--required-fixes-anti-bluff)).
- pgvector `Client.CreateCollection` creates the table with `id TEXT PK, embedding
  vector(N), metadata JSONB, created_at, updated_at` and **no ANN index**; `Search` uses
  cosine `<=>` returning `score = 1 - distance` (`vector_db/pkg/pgvector/client.go`). ANN
  indexes are therefore owned by our migrations.
- **(Pass 3, source-confirmed)** `Client.Search` emits `SELECT id, … , embedding <=> $1
  ORDER BY distance LIMIT $2` with **no `WHERE` clause** — `SearchQuery.Filter` is accepted by
  `Validate()` but **never used** in the adapter SQL, and only `id`+`score` are returned. So
  tenant isolation cannot be delegated to the shipped adapter; it is app-side over-fetch,
  per-tenant collections, or a Qdrant swap (`ATM-DB-013`, now verified not assumed).
- **(Pass 3)** `DistanceOperator(metric)` maps `cosine→<=>` / `dot_product→<#>` /
  `euclidean→<->`; `CollectionConfig.Validate` rejects `Dimension < 1`; `Upsert` serialises all
  metadata values as JSON **strings** (naive encoder) — the tenant filter is string-typed
  (`vector_db/pkg/pgvector/client.go`, `pkg/client/client.go`).
- `database.Config` exposes `Driver` + pooling knobs; `digital.vasic.database` has **no**
  partition/retention/archive package (`database/pkg/` set inspected).
- `embeddings` providers expose `Dimensions()` (jina v2 = 768; voyage-code-3 = 1024) — the
  dimension is model-driven, not hardcoded.

**ASSUMPTION / DEFAULT (adjustable, flagged inline):** monthly partition granularity; HNSW
`m=16 / ef_construction=64 / ef_search=40` starting points; retry defaults (5, 2 s, ×2, 5 min
cap); audit retention 1 year; plan/limit shapes; the `-- +thready Up/Down` migration file
format.

---

## 5. Gaps addressed

Every database-relevant item from `helix_thready_subsystem_gaps_and_improvements.md` is
addressed with a design plan or a tracked workable item.

| Gap-register item (priority) | Addressed in | How |
|------------------------------|--------------|-----|
| `[GAP: vectordb-3.1]` pgvector-only; Qdrant unverified; ANN tuning for < 500 ms (P1) | [schema-vector.sql](./schema-vector.sql), [indexing.md §5](./indexing.md#5-vector-ann-indexes-pgvector) | ANN indexes owned by migrations (adapter creates none); HNSW tuning table; Qdrant swap behind `VectorStore` interface; benchmark `ATM-DB-012` |
| `[GAP: database-3.2]` no partitioning/retention helpers; pooling + pgvector co-location tuning (P1) | [partitioning.md](./partitioning.md), [retention-archive.md](./retention-archive.md) | Native RANGE partitions + `pkg/partition` helper proposal; pooling & co-location §7; retention resolution + archive job |
| `[GAP: HelixLLM-1]` default `HashEmbedder` is non-semantic; RAG 768 hardcode (P0) | [schema-vector.sql](./schema-vector.sql) dimension policy | Dimension discovered via `Dimensions()` (no hardcode); explicit "set `HELIX_EMBEDDING_PROVIDER=llama`, never HashEmbedder" warning |
| `[GAP: helix_skills-4.1]` no execution engine (P0) | [erd.md §5](./erd.md#5-domain-c--processing-skills--assets) | `skills` mirror + `skill_runs` execution ledger backing the BUILD-NEW dispatch engine |
| `[GAP: auth-7.2]` HMAC→RS256/EdDSA + RBAC (P1) | [erd.md §3](./erd.md#3-domain-a--tenancy--identity), [schema-relational.sql](./schema-relational.sql) | Full RBAC tables (roles/permissions/role_permissions/memberships) back the User Service; signing handled in API/security area |
| `[GAP: session_orchestrator-2.9]` design-only claim registry (P1) | [indexing.md §6](./indexing.md#6-the-hot-claim-index-idempotent-processing), [migration-strategy.md §7](./migration-strategy.md#7-concurrency-advisory-lock) | Postgres `FOR UPDATE SKIP LOCKED` partial-index claim + advisory lock replace the unimplemented module for Thready's per-post claim |
| `[GAP: security-7.1]` encrypted-yet-searchable sensitive data (P2) | [schema-vector.sql](./schema-vector.sql) (sensitive-content spec), [erd.md §4/§5](./erd.md#4-domain-b--messenger--ingestion), [retention-archive.md §6](./retention-archive.md#6-gdpr-aware-erasure--export) | Sealed `bytea` columns (`session_enc`, `access_hash_enc`, `totp_secret_enc`); `sensitivity`/`is_encrypted` on assets; **concrete redacted/tokenized-embedding mechanism** now specified in `schema-vector.sql` (redact via `security/pkg/pii` → embed only the non-identifying form; typed-not-valued metadata; ids-only search + RBAC re-check on hydrate); exact-recall searchable-encryption explicitly out-of-MVP-scope |

---

## 6. Open items

Tracked as `ATM-DB-*` workable items; none is papered over.

| ID | `[OPEN: …]` | Summary | Plan |
|----|-------------|---------|------|
| `ATM-DB-001` | `migration-runner-pg-placeholders` | Runner tracking writes use `?`; pgx needs `$n` | Dialect decorator or upstream fix; guarded by a real-Postgres apply/rollback test ([migration-strategy.md §8.1](./migration-strategy.md#8-verified-caveats--required-fixes-anti-bluff)) |
| `ATM-DB-002` | `migration-concurrently` | `CREATE INDEX CONCURRENTLY` can't run in runner tx | Non-transactional path for `0007_secondary_indexes` |
| `ATM-DB-004` | `db-partition-fk` | App- vs DB-enforced FK into partitioned tables | Decide per env; optional composite FK for `processing_state` |
| `ATM-DB-011` | `fts-multilang` | Per-language FTS (en/ru/sr-Cyrl) | Lang-driven expression index; semantic search is primary meanwhile |
| `ATM-DB-012` | (benchmark) | pgvector vs Qdrant against 500 ms SLO | Scaling/benchmark test bank |
| `ATM-DB-013` | `vector-tenant-isolation` | **Source-confirmed:** the pgvector adapter's `Search` emits no `WHERE`/`Filter`; metadata post-filter after ANN can under-fill topK for small tenants sharing a large index | Over-fetch (topK×4) + GIN metadata filter (issued by our own SQL, not `Client.Search`); per-large-tenant collections; Qdrant payload-filter fallback ([schema-vector.sql](./schema-vector.sql) header, [indexing.md §5](./indexing.md#5-vector-ann-indexes-pgvector)) |
| `ATM-DB-031` | keep-archived-vectors-searchable | Per-account flag to retain vectors of archived content | Retention policy flag |
| `ATM-DB-032` | `gdpr-cold-erasure` | Erasure/anonymisation of cold-tier dumps | Depends on archive format choice |
| `ATM-DB-033` | `minio-signed-url-parity` | `storage` signed URLs are CloudFront/AWS-specific; verify MinIO parity | Storage/deployment concern (schema stores opaque `storage_key`); [retention-archive.md §5](./retention-archive.md#5-vector--asset-retention) |
| `ATM-DB-021` | account sub-partitioning | HASH sub-partition for whale tenants | Not MVP; escalation path |

---

## 7. Entity coverage checklist

All entities mandated by the area scope are modelled (see [erd.md §8](./erd.md#8-entity-dictionary)):

`messengers` ✓ · `accounts` ✓ · `channels`/groups ✓ · `posts` ✓ · `threads` ✓ ·
`replies` ✓ · `hashtags` ✓ · `categories` ✓ · `assets` ✓ · `asset_links` ✓ ·
`processing_state` ✓ · `skills` ✓ · `users` ✓ · `roles` ✓ · `permissions` ✓ ·
`memberships` ✓ · `events` ✓ · `subscriptions` ✓ · billing/metering (`plans`,
`usage_records`, `invoices`) ✓ · `audit_log` ✓. Supporting: `messenger_accounts`,
`post_hashtags`, `reply_hashtags`, `post_categories`, `hashtag_categories`, `skill_runs`,
`generated_artifacts`, `event_subscriptions`, `archived_partitions`, and the four
`vectordb_*` collections ✓.

---

## 8. Database test-type coverage (15 mandated types)

[CONVENTIONS.md §6](../CONVENTIONS.md) requires the pack to cover the **15 mandated test
types** `[CONSTITUTION §11.4.27]` (final request §9.1): *unit, integration, e2e,
full-automation, security, DDoS, scaling, chaos, stress, performance, benchmarking, UI, UX,
Challenges, HelixQA*. The individual docs ship the RED-first skeletons; this matrix is the
**single auditable map** of every type to where the data layer exercises it. Per the anti-bluff
rule, mocks/stubs are allowed **only in unit tests** — every other row runs against a **real**
Postgres 16 + pgvector container (SKIP-OK if no engine, `CONST-035`). Types with no data-layer
surface are marked **N-A (delegated)** with the owning area and the DB-side facet they feed —
they are *not* silently claimed as covered.

| # | Test type | Data-layer coverage | Where |
|---|-----------|---------------------|-------|
| 1 | **Unit** | Migration `splitMarkers` loader parse; retention `effective_retention` resolution; `asset_links_one_subject` CHECK arithmetic; CHECK-domain widening logic. Mocks allowed here only. | [migration-strategy.md §3](./migration-strategy.md#3-loading-sql-files-into-migrationmigration), [retention-archive.md §2](./retention-archive.md#2-retention-resolution-model) |
| 2 | **Integration** | Full migration `Apply`/`RollbackWith` on **real** Postgres *and* SQLite; pgvector `Upsert`+`Search` adapter contract; FK cascade + soft-ref `CONSTRAINT TRIGGER` accept/reject. | [migration-strategy.md §10](./migration-strategy.md#10-ci-less-enforcement--tdd), [constraints-and-integrity.md §8](./constraints-and-integrity.md#8-verification--open-items) |
| 3 | **e2e** | Ingest → classify → claim → embed → `/v1/search` hydrate over a live DB; asserts vectors reference rows and search returns ids that hydrate. DB supplies deterministic seed + partition fixtures; the full flow lives in [`../testing/`](../testing/index.md). | [erd.md §7](./erd.md#7-relational--vector-reference-model) |
| 4 | **Full-automation** | Seeded multi-month corpus replayed through the whole pipeline against a real DB, unattended; partition create-ahead + age-out run on the maintenance cadence. DB supplies the reproducible seed + `MonthlyRange` job. | [partitioning.md §5](./partitioning.md#5-partition-maintenance-create-ahead--detach-old) |
| 5 | **Security** | `audit_log` append-only trigger rejects UPDATE/DELETE (defence-in-depth over least-privilege grants); GDPR erasure reaches relational + vector + cold-tier; sealed `bytea` (`session_enc`/`access_hash_enc`/`totp_secret_enc`) never logged; tenant-isolation over-fetch filter correctness. | [constraints-and-integrity.md §6](./constraints-and-integrity.md#6-append-only-audit-log-enforcement), [retention-archive.md §6](./retention-archive.md#6-gdpr-aware-erasure--export), [schema-vector.sql](./schema-vector.sql) |
| 6 | **DDoS** | Data-layer facet: a `post.received` **claim storm** processes exactly-once via `FOR UPDATE SKIP LOCKED` (no thundering-herd double-work); bounded PgBouncer/pgx pool caps connection-exhaustion blast radius. **N-A (delegated)** for network-edge DDoS → [`../api/`](../api/index.md) / [`../deployment/`](../deployment/index.md). | [indexing.md §6](./indexing.md#6-the-hot-claim-index-idempotent-processing), [partitioning.md §7](./partitioning.md#7-connection-pooling--pgvector-co-location-tuning) |
| 7 | **Scaling** | ≥10k posts/day ingestion with search p95 < 500 ms **concurrently**; month-boundary inserts land in the right partition and prune in `EXPLAIN`; ANN over a seeded 1e6-vector fixture. | [partitioning.md §9](./partitioning.md#9-gaps-verification--open-items), [indexing.md §8](./indexing.md#8-verification--tdd) |
| 8 | **Chaos** | Crash mid-archive never drops un-copied data (archive-before-drop invariant); maintenance-job-skipped → rows fall into `*_default` and the alert fires; strongly-consistent reads never routed to a lagging replica. | [retention-archive.md §4/§8](./retention-archive.md#4-archive-pipeline-detach--cold--drop), [partitioning.md §3/§9](./partitioning.md#3-topology-diagram) |
| 9 | **Stress** | Sustained firehose insert with per-partition aggressive autovacuum (`scale_factor=0.02`) keeping bloat bounded; the partial claim index stays O(1)-ish under a deep pending backlog. | [partitioning.md §7](./partitioning.md#7-connection-pooling--pgvector-co-location-tuning), [indexing.md §6](./indexing.md#6-the-hot-claim-index-idempotent-processing) |
| 10 | **Performance** | Read SLO p95 < 150 ms: `EXPLAIN (ANALYZE, BUFFERS)` asserts each hot path is Index/Index-Only Scan, never Seq Scan; the claim query uses `idx_processing_claimable`. | [indexing.md §8](./indexing.md#8-verification--tdd) |
| 11 | **Benchmarking** | pgvector vs **Qdrant** against the 500 ms search SLO (`ATM-DB-012`); HNSW `ef_search` recall/latency curve; write-amplification budget per firehose insert. | [indexing.md §5/§8](./indexing.md#5-vector-ann-indexes-pgvector) |
| 12 | **UI** | **N-A (delegated)** — the data layer has no UI surface. DB owns no `.ds-*`/screen assets → [`../design/`](../design/index.md), client areas. | — |
| 13 | **UX** | **N-A (delegated)** — no direct UX surface; the DB's Aggressive latency SLOs (Q14: search < 500 ms, API p95 < 150 ms) are the **budget** UX tests spend against → client/API areas. | [index §3](#3-decision-snapshot-from-the-matrix) |
| 14 | **Challenges** | `vasic-digital/challenges` scenario bank — DB scenarios: retention-override resolution (channel > account > global), partition age-out + re-attach, tenant-isolation ANN under-fill (`ATM-DB-013`), CHECK-domain expand widening. | register §9.3, [constraints-and-integrity.md §2](./constraints-and-integrity.md#2-check-domain-catalogue-enum-like-columns) |
| 15 | **HelixQA** | `HelixDevelopment/helix_qa` YAML banks with **mandatory runtime evidence** — migration apply/rollback transcripts + `EXPLAIN` output + partition-listing captured as evidence artifacts (the org anti-bluff rule; a green SQLite-only run does not satisfy the Postgres-path caveats `ATM-DB-001/002`). | [migration-strategy.md §8/§10](./migration-strategy.md#8-verified-caveats--required-fixes-anti-bluff) |

**Anti-bluff note.** Rows 6/12/13 are honestly scoped: DDoS at the network edge, and all UI/UX,
are owned by the API/deployment/client areas — the database area covers only the facets a data
layer actually owns and names the delegate. The 13 covered types each cite a runnable skeleton;
none is asserted without a concrete test location. The Postgres-path caveats (`ATM-DB-001`
placeholder rewrite, `ATM-DB-002` `CONCURRENTLY`) are the paired-mutation gates that make a
green run *prove real behaviour* rather than a SQLite-only false pass.

---

*Made with love ♥ by Helix Development.*
