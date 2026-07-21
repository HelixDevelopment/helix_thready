-- =============================================================================
--  Helix Thready — pgvector semantic-collection TEMPLATE
--  Classification : PUBLIC
--  Location       : docs/public/research/mvp/database/materials/templates/semantic-collection.sql
--  Status         : Draft — v0.1
--  Revision       : 1 (2026-07-22) — swarm (database, materials pack)
--  Owner module   : digital.vasic.vectordb (pgvector backend)   [IN-HOUSE: vectordb]
--  Embeddings     : digital.vasic.embeddings -> HelixLLM /v1/embeddings  [IN-HOUSE: embeddings]
--  Related        : ../../schema-vector.sql, ../../migrations/0006_vector_collections.sql,
--                   ../../indexing.md §5 (vector ANN), ../../erd.md §7 (relational<->vector)
--  Provenance     : Q1 (pgvector, cosine), Q15 (voyage-code-3 1024 / jina 768),
--                   Q14 (< 500 ms search SLO). ATM-DB-013 (tenant isolation).
--
--  PURPOSE
--    A reusable, PARAMETRIZED template to stand up ONE semantic collection exactly the
--    way digital.vasic.vectordb's pgvector adapter does, PLUS the ANN + metadata indexes
--    and the cosine query the adapter does NOT create. Render it per collection by
--    substituting the {{PLACEHOLDERS}} (sed / envsubst / a code generator — see the recipe
--    at the bottom). {{...}} is used (NOT psql :vars) on purpose: an identifier like
--    idx_vec_{{COLLECTION}}_hnsw substitutes unambiguously, whereas psql ':collection_hnsw'
--    would be read as a single variable name (underscore is a valid psql var char).
--
--  PARAMETERS
--    {{PREFIX}}       table-name prefix        — VERIFIED default "vectordb_" (DefaultConfig)
--    {{COLLECTION}}   logical collection name  — e.g. posts | replies | assets | generated
--    {{DIM}}          embedding dimension      — MODEL-DRIVEN (embeddings.Dimensions()):
--                                                voyage-code-3 = 1024, jina-v2-base-code = 768.
--                                                NEVER hardcode by policy; a collection pins
--                                                ONE model+dim (vector(N) is fixed width).
--    {{M}}            HNSW graph degree        — [DEFAULT — adjustable] 16
--    {{EFC}}          HNSW ef_construction     — [DEFAULT — adjustable] 64
--    Resulting table name = {{PREFIX}}{{COLLECTION}}  (e.g. vectordb_posts).
--
--  VERIFIED against source (vector_db/pkg/pgvector/client.go, read Pass 3):
--    * Client.CreateCollection emits EXACTLY the table shape in §1 and creates NO ANN
--      index — so §2/§3 (the indexes) are owned by us, not the adapter.
--    * DistanceOperator(metric): cosine -> <=> (vector_cosine_ops) [Thready default, Q1];
--      dot_product -> <#> (vector_ip_ops); euclidean -> <-> (vector_l2_ops). The ANN index
--      operator class MUST match the query operator; we query with <=>, so every index
--      here uses vector_cosine_ops.
--    * Client.Search emits `SELECT id, embedding::text, metadata::text, embedding <=> $1
--      AS distance FROM <t> ORDER BY distance LIMIT $2` with NO WHERE clause — it IGNORES
--      SearchQuery.Filter. Tenant isolation is therefore APP-SIDE (§4). [ATM-DB-013]
--    * Upsert serialises every metadata value as a JSON STRING (naive encoder), so the
--      tenant filter is string-typed: metadata @> jsonb_build_object('account_id',$2::text).
-- =============================================================================


-- ############################################################################
-- ##  TEMPLATE BODY (substitute {{PREFIX}} {{COLLECTION}} {{DIM}} {{M}} {{EFC}}) ##
-- ##  Everything below is idempotent (IF NOT EXISTS) — safe to re-render.       ##
-- ############################################################################

CREATE EXTENSION IF NOT EXISTS vector;   -- also ensured by vectordb.Client.Connect

-- ---------------------------------------------------------------------------
-- 1. Collection table — byte-for-byte the adapter's CreateCollection DDL.
--    id convention:  "<relational_uuid>"            (whole-row vectors), or
--                    "<relational_uuid>:<chunk>"    (chunked documents).
--    metadata ALWAYS carries the relational back-reference + tenant key:
--      { "source_id":"<uuid>", "kind":"post|reply|asset|generated",
--        "account_id":"<uuid>", "span":"<start-end>", "lang":"en" }
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS {{PREFIX}}{{COLLECTION}} (
  id         text PRIMARY KEY,
  embedding  vector({{DIM}}),
  metadata   jsonb DEFAULT '{}',
  created_at timestamptz DEFAULT now(),
  updated_at timestamptz DEFAULT now()
);

-- ---------------------------------------------------------------------------
-- 2a. HNSW index (PREFERRED for the Aggressive < 500 ms SLO, pgvector >= 0.5).
--     No training step, tolerant of incremental writes, best recall/latency at query
--     time. Query-time recall is per-session:  SET hnsw.ef_search = 40;  (raise for recall).
--     CONCURRENTLY so an online build never locks writes — hence it CANNOT run inside a
--     migration transaction (see ../../migrations/0007_secondary_indexes.sql, ATM-DB-002).
-- ---------------------------------------------------------------------------
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_vec_{{COLLECTION}}_hnsw
  ON {{PREFIX}}{{COLLECTION}} USING hnsw (embedding vector_cosine_ops)
  WITH (m = {{M}}, ef_construction = {{EFC}});

-- 2b. IVFFlat ALTERNATIVE (lower memory; ONLY if HNSW memory is a hard constraint).
--     Must be built AFTER data is loaded (it clusters existing vectors); newly inserted
--     rows fall into the nearest existing list, so REINDEX periodically after big backfills.
--     Keep ONE index family per table (HNSW *or* IVFFlat, never both). Tuning rule of thumb:
--       lists ~= sqrt(rows) for < 1M rows, or rows/1000 for >= 1M rows;
--       query-time  SET ivfflat.probes = sqrt(lists)  (raise for recall, costs latency).
--     Uncomment to use INSTEAD OF 2a:
-- CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_vec_{{COLLECTION}}_ivf
--   ON {{PREFIX}}{{COLLECTION}} USING ivfflat (embedding vector_cosine_ops) WITH (lists = 200);

-- ---------------------------------------------------------------------------
-- 3. Metadata GIN — tenant/kind containment (@>) pushdown. REQUIRED for the app-side
--    tenant filter in §4 (the adapter emits no WHERE, so we filter metadata ourselves).
-- ---------------------------------------------------------------------------
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_vec_{{COLLECTION}}_meta
  ON {{PREFIX}}{{COLLECTION}} USING gin (metadata jsonb_path_ops);


-- ############################################################################
-- ##  4. COSINE QUERY TEMPLATES (parametrized)                              ##
-- ############################################################################
--
-- $1 = query embedding (vector({{DIM}}))   $2 = tenant account_id (text)   $3 = TopK
--
-- 4a. TENANT-SCOPED query (Thready's search service issues THIS directly — NOT via the
--     adapter's filterless Client.Search). GIN metadata filter + ANN order-by cosine:
--
--     SET hnsw.ef_search = 40;
--     SELECT id,
--            metadata->>'source_id' AS source_id,
--            1 - (embedding <=> $1::vector) AS score      -- cosine similarity in [0,1]
--     FROM   {{PREFIX}}{{COLLECTION}}
--     WHERE  metadata @> jsonb_build_object('account_id', $2::text)
--     ORDER  BY embedding <=> $1::vector                  -- ascending distance = closest first
--     LIMIT  $3;
--
-- 4b. OVER-FETCH pattern that stays INSIDE the shipped adapter's filterless Search
--     (fetch TopK * over_fetch unfiltered, then drop non-tenant ids app-side, then hydrate):
--
--     SELECT id, 1 - (embedding <=> $1::vector) AS score
--     FROM   {{PREFIX}}{{COLLECTION}}
--     ORDER  BY embedding <=> $1::vector
--     LIMIT  ($3 * 4);        -- TopK * 4; application keeps only this tenant's ids
--
-- [ATM-DB-013 / OPEN: vector-tenant-isolation] Metadata post-filtering after ANN can
-- UNDER-FILL TopK for a small tenant sharing a large index. Choose per scale (indexing.md
-- §5): (a) 4a filtered query, (b) 4b over-fetch, (c) per-large-tenant collection (render
-- this template with {{COLLECTION}} = posts_<account>), or (d) Qdrant payload filter behind
-- the same VectorStore interface. Search returns IDS ONLY; the API hydrates full rows from
-- the relational system of record and re-checks RBAC before releasing content.
--
-- =============================================================================


-- ############################################################################
-- ##  CONCRETE RENDERED EXAMPLE — vectordb_posts @ 1024 (voyage-code-3)      ##
-- ##  This is what the template above expands to; kept as an executable      ##
-- ##  reference (commented so rendering the template does not double-create).##
-- ############################################################################
-- CREATE EXTENSION IF NOT EXISTS vector;
-- CREATE TABLE IF NOT EXISTS vectordb_posts (
--   id         text PRIMARY KEY,
--   embedding  vector(1024),
--   metadata   jsonb DEFAULT '{}',
--   created_at timestamptz DEFAULT now(),
--   updated_at timestamptz DEFAULT now()
-- );
-- CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_vec_posts_hnsw
--   ON vectordb_posts USING hnsw (embedding vector_cosine_ops) WITH (m = 16, ef_construction = 64);
-- CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_vec_posts_meta
--   ON vectordb_posts USING gin (metadata jsonb_path_ops);


-- ############################################################################
-- ##  RENDERING RECIPE                                                      ##
-- ############################################################################
-- Render the four Thready collections with sed (universally available, unambiguous). Note
-- the HNSW/meta index CREATEs use CONCURRENTLY, so they must run OUTSIDE a transaction
-- (psql autocommit is fine); the table CREATE (§1) can run in a migration transaction.
-- DIMENSION MUST come from embeddings.Dimensions() at deploy time — do NOT hardcode; and
-- NEVER default HelixLLM to its HashEmbedder (set HELIX_EMBEDDING_PROVIDER=llama) or the
-- vectors are non-semantic noise. [GAP: HelixLLM-1] [GAP: vectordb-3.1]
--
--   render() {   # $1=collection $2=dim
--     sed -e "s/{{PREFIX}}/vectordb_/g" -e "s/{{COLLECTION}}/$1/g" \
--         -e "s/{{DIM}}/$2/g" -e "s/{{M}}/16/g" -e "s/{{EFC}}/64/g" \
--         templates/semantic-collection.sql
--   }
--   for c in posts replies assets generated; do
--     render "$c" 1024 | psql "$DSN"      # 1024 = voyage-code-3; use 768 for jina
--   done
--
-- envsubst variant: rename the {{X}} tokens to ${X} first, then
--   PREFIX=vectordb_ COLLECTION=posts DIM=1024 M=16 EFC=64 envsubst < template | psql "$DSN"
-- A Go/text-template generator substitutes the same five parameters.
-- =============================================================================
-- Made with love ♥ by Helix Development.
-- =============================================================================
