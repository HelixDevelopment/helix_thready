-- =============================================================================
--  Migration 0001 — init (foundational core: tenancy + ingestion + processing)
--  Classification : PUBLIC
--  Location       : docs/public/research/mvp/database/migrations/0001_init.sql
--  Revision       : 1 (2026-07-21) — swarm (database)
--  Applied via    : digital.vasic.database/pkg/migration.Runner  [IN-HOUSE: database]
--  Loader         : see ../migration-strategy.md §"Loading .sql into migration.Migration"
--
--  FORMAT CONTRACT (parsed by the Go loader):
--   * Two sections delimited by the marker lines below.
--   * "-- +thready Up"   ... everything until "-- +thready Down" is Migration.Up
--   * "-- +thready Down" ... everything after is Migration.Down
--   * The runner executes Up inside ONE transaction (applyOne: Begin/Exec/COMMIT),
--     so NO "CREATE INDEX CONCURRENTLY" here (tables are empty at init anyway).
--   * Up has NO bind parameters -> pgx uses the simple protocol, which permits
--     multiple statements in a single Exec. Keep it that way for every migration.
--   * This migration is the reviewable "expand" for Phase-1 Foundation. Follow-on
--     migrations add the remaining domains (see migration-strategy.md §roadmap):
--       0002 classification · 0003 assets · 0004 billing · 0005 events/audit ·
--       0006 vector collections + ANN indexes · 0007 secondary indexes (CONCURRENTLY).
-- =============================================================================

-- +thready Up

CREATE EXTENSION IF NOT EXISTS pgcrypto;
CREATE EXTENSION IF NOT EXISTS citext;
CREATE EXTENSION IF NOT EXISTS pg_trgm;
CREATE EXTENSION IF NOT EXISTS btree_gin;

CREATE OR REPLACE FUNCTION thready_set_updated_at() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
  NEW.updated_at := now();
  RETURN NEW;
END;
$$;

-- ---- Tenancy & identity ----------------------------------------------------
CREATE TABLE accounts (
  id                     uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  slug                   citext NOT NULL UNIQUE,
  name                   text   NOT NULL,
  branding               jsonb  NOT NULL DEFAULT '{}'::jsonb,
  default_retention_days integer,
  status                 text   NOT NULL DEFAULT 'active'
                           CHECK (status IN ('active','suspended','deleted')),
  created_by             uuid,
  created_at             timestamptz NOT NULL DEFAULT now(),
  updated_at             timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT accounts_retention_nonneg
    CHECK (default_retention_days IS NULL OR default_retention_days >= 0)
);
CREATE TRIGGER trg_accounts_updated BEFORE UPDATE ON accounts
  FOR EACH ROW EXECUTE FUNCTION thready_set_updated_at();

CREATE TABLE users (
  id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  email           citext NOT NULL UNIQUE,
  password_hash   text   NOT NULL,
  display_name    text   NOT NULL DEFAULT '',
  totp_secret_enc bytea,
  mfa_enabled     boolean NOT NULL DEFAULT false,
  is_root         boolean NOT NULL DEFAULT false,
  status          text NOT NULL DEFAULT 'active'
                    CHECK (status IN ('active','invited','disabled','deleted')),
  last_login_at   timestamptz,
  created_at      timestamptz NOT NULL DEFAULT now(),
  updated_at      timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX uq_users_single_root ON users ((is_root)) WHERE is_root;
CREATE TRIGGER trg_users_updated BEFORE UPDATE ON users
  FOR EACH ROW EXECUTE FUNCTION thready_set_updated_at();

CREATE TABLE roles (
  id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  account_id  uuid REFERENCES accounts(id) ON DELETE CASCADE,
  name        text NOT NULL,
  tier        text NOT NULL CHECK (tier IN ('root','account_admin','user')),
  description text NOT NULL DEFAULT '',
  created_at  timestamptz NOT NULL DEFAULT now(),
  UNIQUE (account_id, name)
);

CREATE TABLE permissions (
  id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  code        text NOT NULL UNIQUE,
  description text NOT NULL DEFAULT ''
);

CREATE TABLE role_permissions (
  role_id       uuid NOT NULL REFERENCES roles(id)       ON DELETE CASCADE,
  permission_id uuid NOT NULL REFERENCES permissions(id) ON DELETE CASCADE,
  PRIMARY KEY (role_id, permission_id)
);

CREATE TABLE memberships (
  id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id    uuid NOT NULL REFERENCES users(id)    ON DELETE CASCADE,
  account_id uuid NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
  role_id    uuid NOT NULL REFERENCES roles(id)    ON DELETE RESTRICT,
  status     text NOT NULL DEFAULT 'active'
               CHECK (status IN ('active','invited','revoked')),
  invited_by uuid REFERENCES users(id) ON DELETE SET NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (user_id, account_id)
);
CREATE TRIGGER trg_memberships_updated BEFORE UPDATE ON memberships
  FOR EACH ROW EXECUTE FUNCTION thready_set_updated_at();

-- ---- Messenger & ingestion -------------------------------------------------
CREATE TABLE messengers (
  id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  kind         text NOT NULL UNIQUE CHECK (kind IN ('telegram','max')),
  display_name text NOT NULL,
  capabilities jsonb NOT NULL DEFAULT '{}'::jsonb,
  created_at   timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE messenger_accounts (
  id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  account_id   uuid NOT NULL REFERENCES accounts(id)   ON DELETE CASCADE,
  messenger_id uuid NOT NULL REFERENCES messengers(id) ON DELETE RESTRICT,
  external_ref text NOT NULL,
  session_enc  bytea,
  auth_state   text NOT NULL DEFAULT 'unauthenticated'
                 CHECK (auth_state IN ('unauthenticated','pending_code','pending_2fa','authenticated','revoked')),
  status       text NOT NULL DEFAULT 'active' CHECK (status IN ('active','disabled')),
  created_at   timestamptz NOT NULL DEFAULT now(),
  updated_at   timestamptz NOT NULL DEFAULT now(),
  UNIQUE (account_id, messenger_id, external_ref)
);
CREATE TRIGGER trg_messenger_accounts_updated BEFORE UPDATE ON messenger_accounts
  FOR EACH ROW EXECUTE FUNCTION thready_set_updated_at();

CREATE TABLE channels (
  id                    uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  account_id            uuid NOT NULL REFERENCES accounts(id)           ON DELETE CASCADE,
  messenger_account_id  uuid NOT NULL REFERENCES messenger_accounts(id) ON DELETE CASCADE,
  external_channel_id   text NOT NULL,
  kind                  text NOT NULL CHECK (kind IN ('channel','group','forum')),
  title                 text NOT NULL DEFAULT '',
  access_hash_enc       bytea,
  poll_interval_seconds integer NOT NULL DEFAULT 300 CHECK (poll_interval_seconds > 0),
  retention_days        integer,
  is_active             boolean NOT NULL DEFAULT true,
  last_polled_at        timestamptz,
  created_at            timestamptz NOT NULL DEFAULT now(),
  updated_at            timestamptz NOT NULL DEFAULT now(),
  UNIQUE (messenger_account_id, external_channel_id),
  CONSTRAINT channels_retention_nonneg CHECK (retention_days IS NULL OR retention_days >= 0)
);
CREATE TRIGGER trg_channels_updated BEFORE UPDATE ON channels
  FOR EACH ROW EXECUTE FUNCTION thready_set_updated_at();

CREATE TABLE threads (
  id                  uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  account_id          uuid NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
  channel_id          uuid NOT NULL REFERENCES channels(id) ON DELETE CASCADE,
  root_post_id        uuid,
  root_post_posted_at timestamptz,
  external_topic_id   bigint,
  title               text NOT NULL DEFAULT '',
  reply_count         integer NOT NULL DEFAULT 0,
  last_activity_at    timestamptz NOT NULL DEFAULT now(),
  created_at          timestamptz NOT NULL DEFAULT now(),
  updated_at          timestamptz NOT NULL DEFAULT now(),
  UNIQUE (channel_id, external_topic_id, root_post_id)
);
CREATE TRIGGER trg_threads_updated BEFORE UPDATE ON threads
  FOR EACH ROW EXECUTE FUNCTION thready_set_updated_at();

-- Firehose: RANGE-partitioned on posted_at. PK includes partition key.
CREATE TABLE posts (
  id                  uuid NOT NULL DEFAULT gen_random_uuid(),
  account_id          uuid NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
  channel_id          uuid NOT NULL REFERENCES channels(id) ON DELETE CASCADE,
  thread_id           uuid REFERENCES threads(id) ON DELETE SET NULL,
  external_message_id bigint NOT NULL,
  author_ref          text NOT NULL DEFAULT '',
  raw_text            text NOT NULL DEFAULT '',
  lang                text,
  is_system_reply     boolean NOT NULL DEFAULT false,
  content_hash        bytea NOT NULL,
  posted_at           timestamptz NOT NULL,
  ingested_at         timestamptz NOT NULL DEFAULT now(),
  updated_at          timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (id, posted_at),
  UNIQUE (channel_id, external_message_id, posted_at)
) PARTITION BY RANGE (posted_at);
CREATE TABLE posts_2026_07 PARTITION OF posts FOR VALUES FROM ('2026-07-01') TO ('2026-08-01');
CREATE TABLE posts_2026_08 PARTITION OF posts FOR VALUES FROM ('2026-08-01') TO ('2026-09-01');
CREATE TABLE posts_default  PARTITION OF posts DEFAULT;

CREATE TABLE replies (
  id                    uuid NOT NULL DEFAULT gen_random_uuid(),
  account_id            uuid NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
  thread_id             uuid NOT NULL REFERENCES threads(id) ON DELETE CASCADE,
  parent_post_id        uuid NOT NULL,
  parent_post_posted_at timestamptz,
  parent_reply_id       uuid,
  external_message_id   bigint NOT NULL,
  author_ref            text NOT NULL DEFAULT '',
  raw_text              text NOT NULL DEFAULT '',
  is_system_reply       boolean NOT NULL DEFAULT false,
  posted_at             timestamptz NOT NULL,
  ingested_at           timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (id, posted_at),
  UNIQUE (thread_id, external_message_id, posted_at)
) PARTITION BY RANGE (posted_at);
CREATE TABLE replies_2026_07 PARTITION OF replies FOR VALUES FROM ('2026-07-01') TO ('2026-08-01');
CREATE TABLE replies_2026_08 PARTITION OF replies FOR VALUES FROM ('2026-08-01') TO ('2026-09-01');
CREATE TABLE replies_default  PARTITION OF replies DEFAULT;

-- ---- Processing core -------------------------------------------------------
CREATE TABLE processing_state (
  post_id        uuid PRIMARY KEY,
  post_posted_at timestamptz NOT NULL,
  account_id     uuid NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
  status         text NOT NULL DEFAULT 'pending'
                   CHECK (status IN ('pending','claimed','running','done','failed','skipped')),
  attempts       integer NOT NULL DEFAULT 0,
  max_attempts   integer NOT NULL DEFAULT 5,
  claimed_by     text,
  claimed_at     timestamptz,
  visible_at     timestamptz NOT NULL DEFAULT now(),
  precedence     text,
  result         jsonb,
  last_error     text,
  created_at     timestamptz NOT NULL DEFAULT now(),
  updated_at     timestamptz NOT NULL DEFAULT now()
);
CREATE TRIGGER trg_processing_state_updated BEFORE UPDATE ON processing_state
  FOR EACH ROW EXECUTE FUNCTION thready_set_updated_at();

-- The hot claim index (partial): claimable rows only. See indexing.md.
CREATE INDEX idx_processing_claimable
  ON processing_state (visible_at)
  WHERE status = 'pending';

-- +thready Down

DROP TABLE IF EXISTS processing_state CASCADE;
DROP TABLE IF EXISTS replies CASCADE;
DROP TABLE IF EXISTS posts CASCADE;
DROP TABLE IF EXISTS threads CASCADE;
DROP TABLE IF EXISTS channels CASCADE;
DROP TABLE IF EXISTS messenger_accounts CASCADE;
DROP TABLE IF EXISTS messengers CASCADE;
DROP TABLE IF EXISTS memberships CASCADE;
DROP TABLE IF EXISTS role_permissions CASCADE;
DROP TABLE IF EXISTS permissions CASCADE;
DROP TABLE IF EXISTS roles CASCADE;
DROP TABLE IF EXISTS users CASCADE;
DROP TABLE IF EXISTS accounts CASCADE;
DROP FUNCTION IF EXISTS thready_set_updated_at();
-- Extensions are intentionally NOT dropped (may be shared by other schemas).
