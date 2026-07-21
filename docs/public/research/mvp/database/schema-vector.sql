-- =============================================================================
--  Helix Thready — Vector / Semantic Schema (PostgreSQL 16 + pgvector, cosine)
--  Classification : PUBLIC
--  Location       : docs/public/research/mvp/database/schema-vector.sql
--  Status         : Draft — v0.2
--  Revision       : 3 (2026-07-22) — critic (database, Pass 4): added the concrete
--                   sensitive-content redacted-embedding spec (backs [GAP: security-7.1]).
--                   Rev 2 (Pass 3): verified DistanceOperator mapping; VERIFIED that
--                   Client.Search ignores SearchQuery.Filter (tenant isolation is app-side);
--                   metadata-as-strings Upsert note; IVFFlat DDL.
--  Owner module   : digital.vasic.vectordb (pgvector backend)  [IN-HOUSE: vectordb]
--  Embeddings     : digital.vasic.embeddings -> HelixLLM /v1/embeddings  [IN-HOUSE: embeddings]
--  Related        : ./schema-relational.sql  ./indexing.md  ./erd.md
--  Provenance     : final request §2.1.1 (relational<->semantic), Q1 (pgvector cosine),
--                   §15/§19.1 (Lumen re-impl), Q14 (<500 ms search SLO), Q15 (models).
--
--  VERIFIED against source (digital.vasic.vectordb/pkg/pgvector/client.go, read Pass 3):
--   * A "collection" maps to a table named  <TablePrefix><collection>  (default
--     prefix "vectordb_", DefaultConfig()). Client.CreateCollection emits exactly:
--        CREATE TABLE IF NOT EXISTS <t> (
--          id TEXT PRIMARY KEY,
--          embedding vector(<dim>),
--          metadata JSONB DEFAULT '{}',
--          created_at TIMESTAMPTZ DEFAULT NOW(),
--          updated_at TIMESTAMPTZ DEFAULT NOW());          (client.go CreateCollection)
--     CollectionConfig.Validate rejects Dimension < 1 and an unknown Metric.
--   * Search emits:  SELECT id, embedding::text, metadata::text,
--        embedding <=> $1::vector AS distance FROM <t> ORDER BY distance LIMIT $2;
--     It returns []SearchResult populated with ID and Score = 1 - distance ONLY
--     (Vector and Metadata are selected but discarded). ORDER BY distance ASC =
--     closest first; LIMIT = TopK.                          (client.go Search)
--   * Upsert:  INSERT INTO <t>(id, embedding, metadata, updated_at)
--        VALUES ($1,$2::vector,$3::jsonb,NOW()) ON CONFLICT (id) DO UPDATE ...
--     Metadata is serialised NAIVELY: every value is stringified
--     (fmt.Sprintf("%q:%q", k, "%v"(val))) — so ALL metadata values land as JSON
--     STRINGS (no nested objects, no numeric/bool JSON). Our tenant filter is
--     therefore string-typed:  metadata @> jsonb_build_object('account_id', $2::text).
--
--  >>> VERIFIED CAVEAT — the adapter's Search has NO metadata/tenant filter <<<
--   SearchQuery has a Filter map[string]any field and SearchQuery.Validate accepts
--   it, but Client.Search NEVER references Filter in its SQL (verified: the WHERE
--   clause is absent). So per-tenant scoping CANNOT be delegated to the adapter as
--   shipped. Thready must scope results by ONE of: (a) over-fetch TopK*N then filter
--   metadata->>'account_id' app-side (baseline; needs idx_vec_*_meta GIN), (b)
--   per-large-tenant collections (table-per-tenant via TablePrefix), or (c) swap to
--   Qdrant payload filtering behind the same VectorStore interface. This is exactly
--   [OPEN: vector-tenant-isolation] / ATM-DB-013 — now source-confirmed, not assumed.
--
--  DistanceOperator(metric) mapping (VERIFIED, client.go):
--     cosine (default)  -> <=>   (vector_cosine_ops)     <-- Thready uses this (Q1)
--     dot_product       -> <#>   (vector_ip_ops)
--     euclidean (L2)    -> <->   (vector_l2_ops)
--   The ANN index operator class MUST match the query operator. We query with <=>,
--   so every HNSW/IVFFlat index below uses vector_cosine_ops.
--
--  >>> ANTI-BLUFF / GAP (register §3.1) <<<
--   Client.CreateCollection creates the TABLE ONLY — it does NOT create an ANN
--   index. Without an ivfflat/hnsw index, every Search is a sequential scan and
--   the < 500 ms SLO (Q14) is unmet at Large scale. Therefore the ANN indexes
--   below are OWNED BY THIS SCHEMA/MIGRATIONS, not by the adapter. See indexing.md
--   §"Vector ANN indexes" for tuning. [GAP: vectordb-3.1] [GAP: HelixLLM-1 embedder]
-- =============================================================================

CREATE EXTENSION IF NOT EXISTS vector;   -- also ensured by vectordb.Client.Connect

-- -----------------------------------------------------------------------------
-- Dimension policy  [GAP: HelixLLM-1 / embeddings-2.7]
--   The embedding dimension is MODEL-DRIVEN and MUST be discovered at deploy time
--   from the provider's Dimensions() (digital.vasic.embeddings exposes it per
--   provider), NOT hardcoded. Confirmed dims:
--     * voyage-code-3                : 1024 (default; supports 256/512/1024/2048)
--     * jina-embeddings-v2-base-code :  768
--   A single deployment pins ONE model+dim for a collection; mixing dims in one
--   table is impossible (vector(N) is fixed). Re-embedding to change dim is a
--   rebuild migration (create vectordb_posts_v2, backfill, swap). The examples
--   below use 1024 (voyage-code-3). DO NOT default to HelixLLM's HashEmbedder —
--   set HELIX_EMBEDDING_PROVIDER=llama or the vectors are non-semantic noise.
-- -----------------------------------------------------------------------------

-- =========================== Collection tables ==============================
-- id convention:  "<relational_uuid>" for whole-row vectors, or
--                 "<relational_uuid>:<chunk_index>" for chunked docs.
-- metadata (JSONB) ALWAYS carries the back-reference to the relational store:
--   { "source_id":"<uuid>", "kind":"post|reply|asset|generated",
--     "account_id":"<uuid>", "span":"<start-end>", "lang":"en" }
-- Search returns ids -> the API hydrates full rows from the relational store
-- (final request §2.1.1: "vectors reference rows; search returns ids").

-- Source posts (original messages).
CREATE TABLE IF NOT EXISTS vectordb_posts (
  id         text PRIMARY KEY,
  embedding  vector(1024),
  metadata   jsonb DEFAULT '{}',
  created_at timestamptz DEFAULT now(),
  updated_at timestamptz DEFAULT now()
);

-- Organic replies (tags & context frequently live here).
CREATE TABLE IF NOT EXISTS vectordb_replies (
  id         text PRIMARY KEY,
  embedding  vector(1024),
  metadata   jsonb DEFAULT '{}',
  created_at timestamptz DEFAULT now(),
  updated_at timestamptz DEFAULT now()
);

-- Assets (OCR transcripts, captions, doc/book text extractions, comic transcription).
CREATE TABLE IF NOT EXISTS vectordb_assets (
  id         text PRIMARY KEY,
  embedding  vector(1024),
  metadata   jsonb DEFAULT '{}',
  created_at timestamptz DEFAULT now(),
  updated_at timestamptz DEFAULT now()
);

-- Generated materials (research docs, books, summaries) — the "both posts AND
-- generated materials are indexed" requirement (final request §1.3, §19.1).
CREATE TABLE IF NOT EXISTS vectordb_generated (
  id         text PRIMARY KEY,
  embedding  vector(1024),
  metadata   jsonb DEFAULT '{}',
  created_at timestamptz DEFAULT now(),
  updated_at timestamptz DEFAULT now()
);

-- ============================ ANN indexes (cosine) ==========================
-- HNSW (pgvector >= 0.5) is preferred for the Aggressive < 500 ms SLO: better
-- recall/latency at query time, no training step, tolerant of incremental writes.
-- Operator class MUST match the query operator <=>  ->  vector_cosine_ops.
--
-- HNSW build params (tune per indexing.md):
--   m = 16 (graph degree), ef_construction = 64 (build quality).
-- Query-time recall is set per-session:  SET hnsw.ef_search = 40;  (raise for recall).
CREATE INDEX IF NOT EXISTS idx_vec_posts_hnsw
  ON vectordb_posts     USING hnsw (embedding vector_cosine_ops) WITH (m = 16, ef_construction = 64);
CREATE INDEX IF NOT EXISTS idx_vec_replies_hnsw
  ON vectordb_replies   USING hnsw (embedding vector_cosine_ops) WITH (m = 16, ef_construction = 64);
CREATE INDEX IF NOT EXISTS idx_vec_assets_hnsw
  ON vectordb_assets    USING hnsw (embedding vector_cosine_ops) WITH (m = 16, ef_construction = 64);
CREATE INDEX IF NOT EXISTS idx_vec_generated_hnsw
  ON vectordb_generated USING hnsw (embedding vector_cosine_ops) WITH (m = 16, ef_construction = 64);

-- IVFFlat ALTERNATIVE (lower memory; needs a training set present before build, and
-- lists tuning). Use ONLY if HNSW memory is a hard constraint. Rule of thumb:
--   lists ≈ sqrt(rows) for < 1M rows, or rows/1000 for >= 1M rows;
--   query-time  SET ivfflat.probes = sqrt(lists)  (raise for recall, costs latency).
-- IVFFlat must be built AFTER data is loaded (it clusters existing vectors); newly
-- inserted rows land in the nearest existing list, so periodically REINDEX after large
-- backfills. Keep ONE index family per table (HNSW or IVFFlat), never both.
-- CREATE INDEX idx_vec_posts_ivf     ON vectordb_posts     USING ivfflat (embedding vector_cosine_ops) WITH (lists = 200);
-- CREATE INDEX idx_vec_replies_ivf   ON vectordb_replies   USING ivfflat (embedding vector_cosine_ops) WITH (lists = 200);
-- CREATE INDEX idx_vec_assets_ivf    ON vectordb_assets    USING ivfflat (embedding vector_cosine_ops) WITH (lists = 100);
-- CREATE INDEX idx_vec_generated_ivf ON vectordb_generated USING ivfflat (embedding vector_cosine_ops) WITH (lists = 100);

-- ===================== Metadata filter (tenant + source) ====================
-- Semantic queries are tenant-scoped (WHERE metadata->>'account_id' = $acct) and
-- often kind-scoped. A GIN index on metadata supports containment (@>) pushdown;
-- pair it with the ANN scan (pgvector post-filters after ANN, so over-fetch topK).
CREATE INDEX IF NOT EXISTS idx_vec_posts_meta
  ON vectordb_posts     USING gin (metadata jsonb_path_ops);
CREATE INDEX IF NOT EXISTS idx_vec_replies_meta
  ON vectordb_replies   USING gin (metadata jsonb_path_ops);
CREATE INDEX IF NOT EXISTS idx_vec_assets_meta
  ON vectordb_assets    USING gin (metadata jsonb_path_ops);
CREATE INDEX IF NOT EXISTS idx_vec_generated_meta
  ON vectordb_generated USING gin (metadata jsonb_path_ops);

-- ============================ Reference query ==============================
-- Example: top-10 posts semantically near $1 (a 1024-dim query vector) for tenant
-- $2, hydrated back to the relational store by the API layer. Mirrors the adapter's
-- Search SQL (client.go) plus the tenant metadata filter.
--
--   SET hnsw.ef_search = 40;
--   SELECT id,
--          metadata->>'source_id' AS source_id,
--          1 - (embedding <=> $1::vector) AS score
--   FROM   vectordb_posts
--   WHERE  metadata @> jsonb_build_object('account_id', $2::text)
--   ORDER  BY embedding <=> $1::vector
--   LIMIT  10;
--
-- NOTE: the WHERE tenant filter above is what the SHIPPED adapter does NOT emit (see the
-- VERIFIED CAVEAT in the header). Thready's search service therefore issues this filtered
-- SQL directly (not via Client.Search) OR over-fetches through the adapter and filters in
-- Go. Over-fetch pattern that stays inside the adapter's filterless Search:
--
--   -- fetch TopK*4 unfiltered, then keep only this tenant's ids, then hydrate:
--   SELECT id, 1 - (embedding <=> $1::vector) AS score
--   FROM   vectordb_posts
--   ORDER  BY embedding <=> $1::vector
--   LIMIT  40;                    -- TopK(10) * over_fetch(4); app drops non-tenant ids
--
-- [OPEN: vector-tenant-isolation] (ATM-DB-013) Metadata post-filtering after ANN can
-- under-fill topK for small tenants sharing a large index. Mitigations (choose per scale in
-- indexing.md): (a) over-fetch (LIMIT topK*4 then filter), (b) per-large-tenant
-- collections (table-per-tenant via TablePrefix), (c) migrate to Qdrant payload
-- filtering (register §3.1 hardening) behind the same VectorStore interface.
-- =============================================================================

-- =====================================================================================
-- Sensitive content: "encrypted yet semantically searchable"  [GAP: security-7.1 (P2)]
-- =====================================================================================
--  The gap register (§7.1) asks for a "searchable-but-sealed representation" so specially
--  encrypted assets (credit cards, contracts, QR, screenshots — final request §3.6; relational
--  assets.is_encrypted=true, sensitivity='sensitive') can still be found by semantic search
--  WITHOUT exposing the plaintext. The raw bytes stay sealed in the Asset Service
--  (AES-256-GCM, key material in digital.vasic.security — NEVER in the DB). This is the
--  concrete data-layer mechanism (the claim in index.md §5 is backed here, not asserted):
--
--   1. NEVER embed raw sensitive plaintext. The embedding pipeline first produces a REDACTED
--      / TOKENIZED derivation via security/pkg/pii (PAN -> "<credit_card>", emails/SSNs/keys
--      -> typed placeholders), keeping only non-identifying semantic context (document class,
--      layout terms, non-PII surrounding text). Only that redacted form is embedded.
--   2. The vector row's metadata is minimal and typed-not-valued:
--        { "source_id":"<uuid>", "kind":"asset", "account_id":"<uuid>",
--          "sensitivity":"sensitive", "redacted":true }
--      No plaintext, no PAN fragments, no filename that leaks content land in metadata.
--   3. Search returns ids only; the API hydrates the sealed asset and re-checks RBAC +
--      sensitivity before releasing ANY plaintext, so a vector match cannot leak the secret.
--   4. Erasure (retention-archive.md §6) destroys the relational row + sealed bytes + this
--      vector row in one unit of work; the redacted embedding is not independently sensitive
--      but is deleted with its source so search never surfaces an orphan.
--
--  Trade-off (honest): a redacted embedding is LESS precise than one over full plaintext —
--  intentionally. For 'sensitive' content, discoverability of the *kind/context* is the goal,
--  not exact-content recall. Deployments needing exact recall over sealed content must use a
--  searchable-encryption scheme, which is OUT OF SCOPE for the MVP data layer (tracked with
--  the security area, not papered over here). [GAP: security-7.1] [OPERATOR compliance = minimal]
-- =====================================================================================
