-- =============================================================================
--  Helix Thready — Vector / Semantic Schema (PostgreSQL 16 + pgvector, cosine)
--  Classification : PUBLIC
--  Location       : docs/public/research/mvp/database/schema-vector.sql
--  Status         : Draft — v0.1
--  Revision       : 1 (2026-07-21) — swarm (database)
--  Owner module   : digital.vasic.vectordb (pgvector backend)  [IN-HOUSE: vectordb]
--  Embeddings     : digital.vasic.embeddings -> HelixLLM /v1/embeddings  [IN-HOUSE: embeddings]
--  Related        : ./schema-relational.sql  ./indexing.md  ./erd.md
--  Provenance     : final request §2.1.1 (relational<->semantic), Q1 (pgvector cosine),
--                   §15/§19.1 (Lumen re-impl), Q14 (<500 ms search SLO), Q15 (models).
--
--  VERIFIED against source (digital.vasic.vectordb/pkg/pgvector/client.go):
--   * A "collection" maps to a table named  <TablePrefix><collection>  (default
--     prefix "vectordb_"). Client.CreateCollection emits exactly:
--        CREATE TABLE IF NOT EXISTS <t> (
--          id TEXT PRIMARY KEY,
--          embedding vector(<dim>),
--          metadata JSONB DEFAULT '{}',
--          created_at TIMESTAMPTZ DEFAULT NOW(),
--          updated_at TIMESTAMPTZ DEFAULT NOW());
--   * Search uses cosine distance operator  <=>  (DistanceOperator default), and
--     returns score = 1 - distance (higher = closer). ORDER BY distance LIMIT topK.
--   * Upsert writes id, embedding::vector, metadata::jsonb, updated_at=NOW().
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

-- IVFFlat ALTERNATIVE (lower memory; needs ANALYZE-time lists tuning + a training
-- set). Use only if HNSW memory is a constraint. lists ~= sqrt(rows) as a start;
-- query with  SET ivfflat.probes = 10;  Keep ONE index family per table, not both.
-- CREATE INDEX idx_vec_posts_ivf ON vectordb_posts
--   USING ivfflat (embedding vector_cosine_ops) WITH (lists = 200);

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
-- [OPEN: vector-tenant-isolation] (ATM-DB-013) Metadata post-filtering after ANN can
-- under-fill topK for small tenants sharing a large index. Mitigations (choose per scale in
-- indexing.md): (a) over-fetch (LIMIT topK*4 then filter), (b) per-large-tenant
-- collections (table-per-tenant via TablePrefix), (c) migrate to Qdrant payload
-- filtering (register §3.1 hardening) behind the same VectorStore interface.
-- =============================================================================
