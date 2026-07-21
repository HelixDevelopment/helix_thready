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
| [`migrations/0001_init.sql`](./migrations/0001_init.sql) | Runnable initial migration (Up/Down) — foundational core schema |
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
| `[GAP: security-7.1]` encrypted-yet-searchable sensitive data (P2) | [erd.md §4/§5](./erd.md#4-domain-b--messenger--ingestion), [retention-archive.md §6](./retention-archive.md#6-gdpr-aware-erasure--export) | Sealed `bytea` columns (`session_enc`, `access_hash_enc`, `totp_secret_enc`); `sensitivity`/`is_encrypted` on assets; embeddings over redacted form (§3.6) |

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
| `ATM-DB-013` | `vector-tenant-isolation` | Metadata post-filter after ANN can under-fill topK for small tenants sharing a large index | Over-fetch (topK×4) + GIN metadata filter; per-large-tenant collections; Qdrant payload-filter fallback ([schema-vector.sql](./schema-vector.sql), [indexing.md §5](./indexing.md#5-vector-ann-indexes-pgvector)) |
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

*Made with love ♥ by Helix Development.*
