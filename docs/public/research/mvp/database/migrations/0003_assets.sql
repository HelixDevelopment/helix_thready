-- =============================================================================
--  Migration 0003 — processing artifacts & assets
--  Classification : PUBLIC
--  Location       : docs/public/research/mvp/database/migrations/0003_assets.sql
--  Revision       : 1 (2026-07-22) — swarm (database)
--  Applied via    : digital.vasic.database/pkg/migration.Runner  [IN-HOUSE: database]
--  Depends on     : 0001_init (accounts exists; posts/replies referenced softly)
--  Provenance     : final request §3.2 (skills/dispatch), §7 (assets), §7.3 (ordering),
--                   §3.6 (encrypted sensitive assets); [IN-HOUSE: helix_skills, storage].
--
--  NOTE ON ORDERING: this migration carries skills, skill_runs and generated_artifacts
--  in addition to assets/asset_links because asset_links has a REAL DB foreign key to
--  generated_artifacts, so the artifact tables must exist first. All are pure DDL in one
--  transaction. Secondary indexes are added later in 0007 (CONCURRENTLY).
-- =============================================================================

-- +thready Up

-- skills: relational mirror of the helix_skills Skill-Graph nodes Thready dispatches.
-- [GAP: helix_skills-4.1] helix_skills is a KNOWLEDGE DAG with no execution engine;
-- Thready's dispatch engine is BUILD-NEW and skill_runs (below) is its execution ledger.
CREATE TABLE skills (
  id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  skill_key  text NOT NULL UNIQUE,
  name       text NOT NULL,
  version    text NOT NULL DEFAULT '0.0.0',
  kind       text NOT NULL CHECK (kind IN ('atomic','composite','umbrella')),
  sort_order integer NOT NULL DEFAULT 100,
  is_enabled boolean NOT NULL DEFAULT true,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);
COMMENT ON COLUMN skills.skill_key  IS 'Skill-Graph node id (the SKILL.md name); stable dispatch key.';
COMMENT ON COLUMN skills.kind        IS 'Skill-Graph node kind: atomic | composite | umbrella (helix_skills taxonomy).';
COMMENT ON COLUMN skills.sort_order  IS 'Deterministic dispatch order: download-type Skills before analysis-type (§3.3).';
CREATE TRIGGER trg_skills_updated BEFORE UPDATE ON skills
  FOR EACH ROW EXECUTE FUNCTION thready_set_updated_at();

-- skill_runs: one row per (post, skill) execution attempt. Soft ref into partitioned posts.
CREATE TABLE skill_runs (
  id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  post_id        uuid NOT NULL,
  post_posted_at timestamptz NOT NULL,
  skill_id       uuid NOT NULL REFERENCES skills(id) ON DELETE RESTRICT,
  status         text NOT NULL DEFAULT 'pending'
                   CHECK (status IN ('pending','running','done','failed','skipped')),
  attempt        integer NOT NULL DEFAULT 1,
  started_at     timestamptz,
  finished_at    timestamptz,
  metrics        jsonb NOT NULL DEFAULT '{}'::jsonb,
  error          text,
  created_at     timestamptz NOT NULL DEFAULT now()
);
COMMENT ON TABLE  skill_runs         IS 'Execution ledger for the BUILD-NEW dispatch engine: one row per (post, skill) attempt (final request §3.3).';
COMMENT ON COLUMN skill_runs.metrics IS 'Per-run observability payload (duration_ms, tokens, bytes, tool calls) as JSONB.';

-- generated_artifacts: research docs / books / transcripts / summaries produced by Skills.
-- Embedded into vectordb_generated for semantic search over generated materials (§1.3).
CREATE TABLE generated_artifacts (
  id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  account_id     uuid NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
  post_id        uuid NOT NULL,
  post_posted_at timestamptz NOT NULL,
  kind           text NOT NULL CHECK (kind IN ('research_doc','book','transcript','summary','other')),
  title          text NOT NULL DEFAULT '',
  storage_key    text,
  lang           text NOT NULL DEFAULT 'en',
  created_at     timestamptz NOT NULL DEFAULT now(),
  updated_at     timestamptz NOT NULL DEFAULT now()
);
COMMENT ON COLUMN generated_artifacts.storage_key IS 'Asset Service / storage object key if materialised; NULL if inline-only. Clients never see raw paths (§7.1).';
CREATE TRIGGER trg_generated_artifacts_updated BEFORE UPDATE ON generated_artifacts
  FOR EACH ROW EXECUTE FUNCTION thready_set_updated_at();

-- assets: content-hash-deduped blobs stored via the Asset Service (Catalogizer + storage).
CREATE TABLE assets (
  id               uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  account_id       uuid NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
  parent_asset_id  uuid REFERENCES assets(id) ON DELETE SET NULL,
  content_hash     bytea NOT NULL,
  kind             text NOT NULL CHECK (kind IN ('video','audio','image','document','book','comic','other')),
  mime             text NOT NULL DEFAULT 'application/octet-stream',
  size_bytes       bigint NOT NULL DEFAULT 0 CHECK (size_bytes >= 0),
  storage_backend  text NOT NULL DEFAULT 'minio' CHECK (storage_backend IN ('minio','s3','local')),
  storage_key      text NOT NULL,
  is_web_rendition boolean NOT NULL DEFAULT false,
  is_encrypted     boolean NOT NULL DEFAULT false,
  sensitivity      text NOT NULL DEFAULT 'internal'
                     CHECK (sensitivity IN ('public','internal','sensitive')),
  created_at       timestamptz NOT NULL DEFAULT now(),
  updated_at       timestamptz NOT NULL DEFAULT now(),
  UNIQUE (account_id, content_hash)
);
COMMENT ON COLUMN assets.parent_asset_id IS 'Raw original -> …-web rendition link (self-reference). NULL for a raw/original asset.';
COMMENT ON COLUMN assets.content_hash    IS 'sha256(bytes) for per-tenant dedup + integrity. UNIQUE(account_id, content_hash).';
COMMENT ON COLUMN assets.storage_key     IS 'Opaque object key; the schema stores NO signed URL (backend-agnostic; see retention-archive.md §5, ATM-DB-033).';
COMMENT ON COLUMN assets.is_encrypted    IS 'True for the specially-encrypted asset directory (credit cards/contracts/QR/screenshots, §3.6). Key material lives in security, never in the DB.';
CREATE TRIGGER trg_assets_updated BEFORE UPDATE ON assets
  FOR EACH ROW EXECUTE FUNCTION thready_set_updated_at();

-- asset_links: attach an asset to EXACTLY ONE subject (post | reply | generated_artifact).
CREATE TABLE asset_links (
  id                    uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  asset_id              uuid NOT NULL REFERENCES assets(id) ON DELETE CASCADE,
  post_id               uuid,
  post_posted_at        timestamptz,
  reply_id              uuid,
  reply_posted_at       timestamptz,
  generated_artifact_id uuid REFERENCES generated_artifacts(id) ON DELETE CASCADE,
  role                  text NOT NULL DEFAULT 'source'
                          CHECK (role IN ('source','generated','rendition')),
  ordering_index        integer,
  created_at            timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT asset_links_one_subject CHECK (
    (CASE WHEN post_id               IS NOT NULL THEN 1 ELSE 0 END) +
    (CASE WHEN reply_id              IS NOT NULL THEN 1 ELSE 0 END) +
    (CASE WHEN generated_artifact_id IS NOT NULL THEN 1 ELSE 0 END) = 1
  )
);
COMMENT ON COLUMN asset_links.ordering_index IS 'Preserves numeric-prefix series/playlist order (§7.3); NULL if unordered.';
COMMENT ON CONSTRAINT asset_links_one_subject ON asset_links IS
  'Exactly one of {post_id, reply_id, generated_artifact_id} must be set — an asset attaches to a single subject.';

-- +thready Down

DROP TABLE IF EXISTS asset_links         CASCADE;
DROP TABLE IF EXISTS assets              CASCADE;
DROP TABLE IF EXISTS generated_artifacts CASCADE;
DROP TABLE IF EXISTS skill_runs          CASCADE;
DROP TABLE IF EXISTS skills              CASCADE;
