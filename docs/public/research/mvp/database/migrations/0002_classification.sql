-- =============================================================================
--  Migration 0002 — classification (hashtags, content-type categories, joins)
--  Classification : PUBLIC
--  Location       : docs/public/research/mvp/database/migrations/0002_classification.sql
--  Revision       : 1 (2026-07-22) — swarm (database)
--  Applied via    : digital.vasic.database/pkg/migration.Runner  [IN-HOUSE: database]
--  Depends on     : 0001_init (accounts/channels/posts/replies must already exist)
--  Provenance     : final request §3.2.2 (content types), §3.3 (additive categories +
--                   deterministic precedence), §3.5 (indirect determination).
--
--  FORMAT CONTRACT (parsed by the Go loader — see migration-strategy.md §3):
--   * "-- +thready Up"   .. everything until "-- +thready Down" is Migration.Up
--   * "-- +thready Down" .. everything after is Migration.Down
--   * Up runs inside ONE transaction (applyOne: Begin/Exec/COMMIT) — all pure DDL,
--     no CONCURRENTLY, no bind parameters (pgx simple protocol; multi-statement OK).
--   * Secondary indexes for these tables are created later in 0007 (CONCURRENTLY).
-- =============================================================================

-- +thready Up

-- Normalised tag registry. Stored WITHOUT the leading '#'. citext => the tag is
-- matched case-insensitively so '#Torrent' and '#torrent' collapse to one row.
CREATE TABLE hashtags (
  id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tag        citext NOT NULL UNIQUE,
  created_at timestamptz NOT NULL DEFAULT now()
);
COMMENT ON TABLE  hashtags     IS 'Normalised hashtag registry (stored without ''#''; case-insensitive via citext).';
COMMENT ON COLUMN hashtags.tag IS 'Canonical tag text, no leading ''#''. UNIQUE + citext => case-insensitive dedup.';

-- categories = the documented content types (Video, Torrent, Research, Comic, …).
-- precedence_class + sort_order drive deterministic multi-hashtag Skill ordering.
CREATE TABLE categories (
  id               uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  code             text NOT NULL UNIQUE,
  display_name     text NOT NULL,
  precedence_class text NOT NULL
                     CHECK (precedence_class IN ('download','convert','analyze','research','reply')),
  sort_order       integer NOT NULL DEFAULT 100,
  created_at       timestamptz NOT NULL DEFAULT now()
);
COMMENT ON COLUMN categories.precedence_class IS
  'Deterministic multi-hashtag precedence (final request §3.3): download > convert > analyze > research > reply.';
COMMENT ON COLUMN categories.sort_order IS
  'Secondary deterministic ordering within a precedence_class (download-type Skills before analysis-type).';

-- hashtag -> category mapping (M:N; a tag may imply several content types).
CREATE TABLE hashtag_categories (
  hashtag_id  uuid NOT NULL REFERENCES hashtags(id)   ON DELETE CASCADE,
  category_id uuid NOT NULL REFERENCES categories(id) ON DELETE CASCADE,
  PRIMARY KEY (hashtag_id, category_id)
);
COMMENT ON TABLE hashtag_categories IS
  'Indirect-determination map (final request §3.5): a tag (e.g. #torrent) implies content categories (Torrent + ToDownload).';

-- post <-> hashtag. Soft ref into partitioned posts: carries post_posted_at, no DB FK.
CREATE TABLE post_hashtags (
  post_id        uuid NOT NULL,
  post_posted_at timestamptz NOT NULL,
  hashtag_id     uuid NOT NULL REFERENCES hashtags(id) ON DELETE CASCADE,
  source         text NOT NULL DEFAULT 'explicit'
                   CHECK (source IN ('explicit','indirect','ai_inferred')),
  PRIMARY KEY (post_id, hashtag_id)
);
COMMENT ON COLUMN post_hashtags.post_posted_at IS
  'Denormalised copy of posts.posted_at (partition key) so joins back to posts prune partitions. No DB FK into the partitioned parent (see erd.md §9).';
COMMENT ON COLUMN post_hashtags.source IS
  'explicit = literal #tag; indirect = derived from link type (§3.5); ai_inferred = LLM fallback (§3.2.1).';

-- reply <-> hashtag (tags frequently live on replies to a link-only root).
CREATE TABLE reply_hashtags (
  reply_id        uuid NOT NULL,
  reply_posted_at timestamptz NOT NULL,
  hashtag_id      uuid NOT NULL REFERENCES hashtags(id) ON DELETE CASCADE,
  source          text NOT NULL DEFAULT 'explicit'
                    CHECK (source IN ('explicit','indirect','ai_inferred')),
  PRIMARY KEY (reply_id, hashtag_id)
);
COMMENT ON COLUMN reply_hashtags.reply_posted_at IS
  'Denormalised copy of replies.posted_at (partition key) for pruned joins; no DB FK into partitioned replies.';

-- post <-> category (ADDITIVE classification — a post may be many categories).
CREATE TABLE post_categories (
  post_id        uuid NOT NULL,
  post_posted_at timestamptz NOT NULL,
  category_id    uuid NOT NULL REFERENCES categories(id) ON DELETE CASCADE,
  confidence     real NOT NULL DEFAULT 1.0 CHECK (confidence >= 0 AND confidence <= 1),
  PRIMARY KEY (post_id, category_id)
);
COMMENT ON TABLE  post_categories            IS 'Additive content-type classification (final request §3.2.2): a post may be Video AND Research AND ToDownload.';
COMMENT ON COLUMN post_categories.confidence IS 'Classifier confidence [0,1]; 1.0 for explicit-tag-derived, < 1.0 for ai_inferred.';

-- +thready Down

DROP TABLE IF EXISTS post_categories    CASCADE;
DROP TABLE IF EXISTS reply_hashtags     CASCADE;
DROP TABLE IF EXISTS post_hashtags      CASCADE;
DROP TABLE IF EXISTS hashtag_categories CASCADE;
DROP TABLE IF EXISTS categories         CASCADE;
DROP TABLE IF EXISTS hashtags           CASCADE;
