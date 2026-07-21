-- =============================================================================
--  Migration 0006 — vector collections (pgvector tables)
--  Classification : PUBLIC
--  Location       : docs/public/research/mvp/database/migrations/0006_vector_collections.sql
--  Revision       : 1 (2026-07-22) — swarm (database)
--  Applied via    : digital.vasic.database/pkg/migration.Runner  [IN-HOUSE: database]
--  Owner module   : digital.vasic.vectordb (pgvector backend)     [IN-HOUSE: vectordb]
--  Provenance     : final request §2.1.1 (relational<->semantic), Q1 (pgvector cosine),
--                   Q15 (voyage-code-3 1024 / jina 768). See schema-vector.sql.
--
--  VERIFIED (vector_db/pkg/pgvector/client.go): CreateCollection emits exactly the table
--  shape below (id TEXT PK, embedding vector(N), metadata JSONB, created_at, updated_at)
--  and creates NO ANN index. This migration therefore matches the adapter's DDL so the
--  tables are identical whether created by the adapter or by the runner (both IF NOT EXISTS,
--  idempotent). The ANN + metadata GIN indexes are OWNED BY 0007 (they are what makes the
--  < 500 ms SLO reachable — the adapter never creates them). [GAP: vectordb-3.1]
--
--  DIMENSION: the vector(N) width is MODEL-DRIVEN (embeddings.Dimensions()), NOT hardcoded
--  by policy. These literals use 1024 (voyage-code-3). For jina-embeddings-v2-base-code use
--  768. A single deployment pins ONE model+dim per collection; changing dim is a rebuild
--  migration (create *_v2, backfill, swap). NEVER default to HelixLLM's HashEmbedder
--  (set HELIX_EMBEDDING_PROVIDER=llama) or the vectors are non-semantic noise. [GAP: HelixLLM-1]
-- =============================================================================

-- +thready Up

CREATE EXTENSION IF NOT EXISTS vector;   -- also ensured by vectordb.Client.Connect

CREATE TABLE IF NOT EXISTS vectordb_posts (
  id         text PRIMARY KEY,
  embedding  vector(1024),
  metadata   jsonb DEFAULT '{}',
  created_at timestamptz DEFAULT now(),
  updated_at timestamptz DEFAULT now()
);

CREATE TABLE IF NOT EXISTS vectordb_replies (
  id         text PRIMARY KEY,
  embedding  vector(1024),
  metadata   jsonb DEFAULT '{}',
  created_at timestamptz DEFAULT now(),
  updated_at timestamptz DEFAULT now()
);

CREATE TABLE IF NOT EXISTS vectordb_assets (
  id         text PRIMARY KEY,
  embedding  vector(1024),
  metadata   jsonb DEFAULT '{}',
  created_at timestamptz DEFAULT now(),
  updated_at timestamptz DEFAULT now()
);

CREATE TABLE IF NOT EXISTS vectordb_generated (
  id         text PRIMARY KEY,
  embedding  vector(1024),
  metadata   jsonb DEFAULT '{}',
  created_at timestamptz DEFAULT now(),
  updated_at timestamptz DEFAULT now()
);

-- +thready Down

DROP TABLE IF EXISTS vectordb_generated CASCADE;
DROP TABLE IF EXISTS vectordb_assets    CASCADE;
DROP TABLE IF EXISTS vectordb_replies   CASCADE;
DROP TABLE IF EXISTS vectordb_posts     CASCADE;
-- The vector extension is intentionally NOT dropped (may be shared).
