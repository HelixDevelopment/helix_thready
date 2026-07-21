-- =============================================================================
--  Migration 0007 — secondary indexes, FTS & vector ANN (NON-TRANSACTIONAL path)
--  Classification : PUBLIC
--  Location       : docs/public/research/mvp/database/migrations/0007_secondary_indexes.sql
--  Revision       : 1 (2026-07-22) — swarm (database)
--  Provenance     : indexing.md §3 (relational), §4 (FTS), §5 (vector ANN).
--
--  >>> DO NOT APPLY THIS THROUGH THE TRANSACTIONAL runner.Apply PATH <<<
--   CREATE INDEX CONCURRENTLY / DROP INDEX CONCURRENTLY cannot run inside a transaction
--   block, and applyOne wraps Up in Begin/Commit. This migration is applied by the
--   dedicated NON-TRANSACTIONAL deploy step (migration-strategy.md §8.2 / ATM-DB-002):
--   each statement executes on its own autocommit connection, and the schema_migrations
--   row (version 7) is written manually after all statements succeed.
--
--  PARTITIONED-PARENT INDEXES: `CREATE INDEX CONCURRENTLY` is NOT allowed on a partitioned
--  parent. The statements below use plain `CREATE INDEX` on the parents (posts, replies,
--  events, audit_log) — instant at bootstrap because the partitions are empty. For an
--  ONLINE build on an already-populated deployment, replace each with the ON ONLY parent +
--  per-partition `CREATE INDEX CONCURRENTLY` + `ALTER INDEX … ATTACH PARTITION` recipe in
--  indexing.md §7 (zero-downtime). Non-partitioned tables use CONCURRENTLY directly.
-- =============================================================================

-- +thready Up

-- ---- Ingestion read paths (posts/replies are PARTITIONED parents) ----------
CREATE INDEX IF NOT EXISTS idx_posts_channel_time      ON posts   (channel_id, posted_at DESC);  -- Q1 poll: newest per channel
CREATE INDEX IF NOT EXISTS idx_posts_account_time      ON posts   (account_id, posted_at DESC);  -- Q7 dashboards/exports
CREATE INDEX IF NOT EXISTS idx_replies_thread_time     ON replies (thread_id,  posted_at ASC);   -- Q2 thread assembly (chronological)
CREATE INDEX IF NOT EXISTS idx_replies_parent          ON replies (parent_post_id);              -- reply -> parent post (soft ref)

-- ---- Threads / channels (non-partitioned -> CONCURRENTLY) -------------------
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_threads_channel_activity ON threads  (channel_id, last_activity_at DESC);
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_channels_due_poll        ON channels (last_polled_at) WHERE is_active;  -- partial: only pollable channels

-- ---- Classification --------------------------------------------------------
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_post_hashtags_hashtag    ON post_hashtags   (hashtag_id);   -- Q6 posts by tag (reverse)
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_reply_hashtags_hashtag   ON reply_hashtags  (hashtag_id);
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_post_categories_category ON post_categories (category_id);  -- posts of a category

-- ---- Processing ------------------------------------------------------------
-- The hot claim index idx_processing_claimable is created in 0001 (partial, WHERE status='pending').
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_processing_status ON processing_state (status, updated_at);   -- observability: stuck/failed rows
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_skill_runs_post   ON skill_runs       (post_id, skill_id);    -- runs of a post

-- ---- Assets ----------------------------------------------------------------
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_assets_parent        ON assets      (parent_asset_id)        WHERE parent_asset_id IS NOT NULL;  -- renditions of a raw asset
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_asset_links_post     ON asset_links (post_id)                WHERE post_id               IS NOT NULL;
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_asset_links_reply    ON asset_links (reply_id)               WHERE reply_id              IS NOT NULL;
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_asset_links_artifact ON asset_links (generated_artifact_id) WHERE generated_artifact_id IS NOT NULL;
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_asset_links_asset    ON asset_links (asset_id);

-- ---- Identity / RBAC hot paths (partial: active memberships only) ----------
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_memberships_account ON memberships (account_id) WHERE status = 'active';
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_memberships_user    ON memberships (user_id)    WHERE status = 'active';

-- ---- Billing / metering ----------------------------------------------------
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_usage_account_metric ON usage_records (account_id, metric, window_start DESC);
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_usage_unbilled       ON usage_records (account_id) WHERE NOT billed;   -- open reconciliation set
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_subscriptions_account ON subscriptions (account_id, status);

-- ---- Events / audit (PARTITIONED parents -> plain CREATE INDEX) -------------
CREATE INDEX IF NOT EXISTS idx_events_account_time ON events    (account_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_events_sticky       ON events    (entity_id,  created_at DESC) WHERE scope = 'sticky' AND NOT invalidated;
CREATE INDEX IF NOT EXISTS idx_audit_account_time  ON audit_log (account_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_audit_actor         ON audit_log (actor_user_id, created_at DESC);

-- ---- Full-text search (generated tsvector; english config) -----------------
-- ADD COLUMN on a partitioned parent propagates to partitions; instant at bootstrap.
-- [OPEN: fts-multilang] (ATM-DB-011) per-language config; semantic search is primary meanwhile.
ALTER TABLE posts
  ADD COLUMN IF NOT EXISTS body_fts tsvector
  GENERATED ALWAYS AS (to_tsvector('english', coalesce(raw_text, ''))) STORED;
CREATE INDEX IF NOT EXISTS idx_posts_fts ON posts USING gin (body_fts);          -- partitioned parent
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_hashtags_trgm      ON hashtags USING gin (tag   gin_trgm_ops);  -- fuzzy tag search
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_threads_title_trgm ON threads  USING gin (title gin_trgm_ops);  -- fuzzy title search

-- ---- Vector ANN + metadata filter (pgvector; cosine op class) --------------
-- The adapter creates NEITHER of these; they are owned here. [GAP: vectordb-3.1]
-- HNSW cosine op class MUST match the query operator <=>  ->  vector_cosine_ops.
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_vec_posts_hnsw     ON vectordb_posts     USING hnsw (embedding vector_cosine_ops) WITH (m = 16, ef_construction = 64);
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_vec_replies_hnsw   ON vectordb_replies   USING hnsw (embedding vector_cosine_ops) WITH (m = 16, ef_construction = 64);
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_vec_assets_hnsw    ON vectordb_assets    USING hnsw (embedding vector_cosine_ops) WITH (m = 16, ef_construction = 64);
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_vec_generated_hnsw ON vectordb_generated USING hnsw (embedding vector_cosine_ops) WITH (m = 16, ef_construction = 64);
-- GIN on metadata for the tenant/kind filter (pgvector post-filters after ANN -> over-fetch topK).
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_vec_posts_meta     ON vectordb_posts     USING gin (metadata jsonb_path_ops);
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_vec_replies_meta   ON vectordb_replies   USING gin (metadata jsonb_path_ops);
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_vec_assets_meta    ON vectordb_assets    USING gin (metadata jsonb_path_ops);
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_vec_generated_meta ON vectordb_generated USING gin (metadata jsonb_path_ops);

-- +thready Down

DROP INDEX IF EXISTS idx_vec_generated_meta;
DROP INDEX IF EXISTS idx_vec_assets_meta;
DROP INDEX IF EXISTS idx_vec_replies_meta;
DROP INDEX IF EXISTS idx_vec_posts_meta;
DROP INDEX IF EXISTS idx_vec_generated_hnsw;
DROP INDEX IF EXISTS idx_vec_assets_hnsw;
DROP INDEX IF EXISTS idx_vec_replies_hnsw;
DROP INDEX IF EXISTS idx_vec_posts_hnsw;
DROP INDEX IF EXISTS idx_threads_title_trgm;
DROP INDEX IF EXISTS idx_hashtags_trgm;
DROP INDEX IF EXISTS idx_posts_fts;
ALTER TABLE posts DROP COLUMN IF EXISTS body_fts;
DROP INDEX IF EXISTS idx_audit_actor;
DROP INDEX IF EXISTS idx_audit_account_time;
DROP INDEX IF EXISTS idx_events_sticky;
DROP INDEX IF EXISTS idx_events_account_time;
DROP INDEX IF EXISTS idx_subscriptions_account;
DROP INDEX IF EXISTS idx_usage_unbilled;
DROP INDEX IF EXISTS idx_usage_account_metric;
DROP INDEX IF EXISTS idx_memberships_user;
DROP INDEX IF EXISTS idx_memberships_account;
DROP INDEX IF EXISTS idx_asset_links_asset;
DROP INDEX IF EXISTS idx_asset_links_artifact;
DROP INDEX IF EXISTS idx_asset_links_reply;
DROP INDEX IF EXISTS idx_asset_links_post;
DROP INDEX IF EXISTS idx_assets_parent;
DROP INDEX IF EXISTS idx_skill_runs_post;
DROP INDEX IF EXISTS idx_processing_status;
DROP INDEX IF EXISTS idx_post_categories_category;
DROP INDEX IF EXISTS idx_reply_hashtags_hashtag;
DROP INDEX IF EXISTS idx_post_hashtags_hashtag;
DROP INDEX IF EXISTS idx_channels_due_poll;
DROP INDEX IF EXISTS idx_threads_channel_activity;
DROP INDEX IF EXISTS idx_replies_parent;
DROP INDEX IF EXISTS idx_replies_thread_time;
DROP INDEX IF EXISTS idx_posts_account_time;
DROP INDEX IF EXISTS idx_posts_channel_time;
