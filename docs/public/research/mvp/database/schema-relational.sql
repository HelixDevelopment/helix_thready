-- =============================================================================
--  Helix Thready — Relational Schema (PostgreSQL 16, production)
--  Classification : PUBLIC
--  Location       : docs/public/research/mvp/database/schema-relational.sql
--  Status         : Draft — v0.1
--  Revision       : 1 (2026-07-21) — swarm (database)
--  Owner module   : digital.vasic.database  [IN-HOUSE: database]
--  Applied via    : digital.vasic.database/pkg/migration.Runner  (see migration-strategy.md)
--  Related        : ./schema-vector.sql  ./indexing.md  ./partitioning.md
--                   ./retention-archive.md  ./erd.md
--  Provenance     : final request §2.1 (data layer), §3 (workflow), §6 (users),
--                   §7 (assets), §14 (infra), Q11/Q12/Q30/Q40 (billing/retention/audit)
--
--  READ FIRST — design decisions encoded below:
--   * Development uses SQLite (modernc.org/sqlite, cgo-free); production uses
--     PostgreSQL (pgx/v5). This file is the PRODUCTION dialect. SQLite-dialect
--     notes are inline as "-- SQLITE:". Driver is selected by database.Config.Driver.
--   * Enum-like columns are TEXT + CHECK (not CREATE TYPE) so expand/contract
--     migrations can widen a domain without ALTER TYPE table rewrites.
--   * TIMESTAMPTZ everywhere; the server runs in UTC.
--   * UUID v4 surrogate PKs (gen_random_uuid, pgcrypto) + natural unique keys.
--   * REFERENTIAL INTEGRITY INTO PARTITIONED TABLES: PostgreSQL requires a FK
--     target to include the partition key. The firehose tables (posts, replies,
--     events, audit_log) are RANGE-partitioned on time, so their PK is
--     (id, <time>). We deliberately DO NOT declare DB-level foreign keys that
--     point INTO those tables; those references are enforced by the processing
--     engine's transactional single-claim + optional validation triggers, and
--     children denormalise the partition-key column (…_posted_at) for pruned
--     joins. Every OTHER table uses full DB-level FKs. This is a standard,
--     defensible pattern at the operator's Large scale (10k+ posts/day).
--   * The vector/embedding tables live in ./schema-vector.sql (pgvector).
-- =============================================================================

-- -----------------------------------------------------------------------------
-- 0. Extensions
-- -----------------------------------------------------------------------------
CREATE EXTENSION IF NOT EXISTS pgcrypto;   -- gen_random_uuid()
CREATE EXTENSION IF NOT EXISTS citext;     -- case-insensitive email / tag / slug
CREATE EXTENSION IF NOT EXISTS pg_trgm;    -- trigram indexes for LIKE / fuzzy search
CREATE EXTENSION IF NOT EXISTS btree_gin;  -- composite btree+gin indexes
-- vector extension is created by schema-vector.sql / vectordb.Client.Connect.
-- SQLITE: none of the above; SQLite dev uses text ids + FTS5 for search.

-- -----------------------------------------------------------------------------
-- 1. Shared trigger: maintain updated_at
-- -----------------------------------------------------------------------------
CREATE OR REPLACE FUNCTION thready_set_updated_at() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
  NEW.updated_at := now();
  RETURN NEW;
END;
$$;

-- =============================================================================
--  DOMAIN A — Tenancy & Identity (three-tier RBAC: root / account-admin / user)
--  Provenance: final request §6.1 (three-tier), §6.3 (auth), Q9/Q10; User Service
--              is BUILD-NEW on digital.vasic.auth + security/pkg/policy.
-- =============================================================================

-- accounts = tenants. Root sets a global default retention; each account may
-- override (shorten) it. Branding holds white-label config (colors/logo/slogan).
CREATE TABLE accounts (
  id                     uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  slug                   citext NOT NULL UNIQUE,
  name                   text   NOT NULL,
  branding               jsonb  NOT NULL DEFAULT '{}'::jsonb,   -- {colors,logo_url,slogan}
  default_retention_days integer,                                -- NULL = keep indefinitely
  status                 text   NOT NULL DEFAULT 'active'
                           CHECK (status IN ('active','suspended','deleted')),
  created_by             uuid,                                   -- users.id (soft ref; bootstrap)
  created_at             timestamptz NOT NULL DEFAULT now(),
  updated_at             timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT accounts_retention_nonneg
    CHECK (default_retention_days IS NULL OR default_retention_days >= 0)
);
COMMENT ON COLUMN accounts.default_retention_days IS
  'NULL = keep indefinitely (system default). A positive value shortens retention for this account only (operator decision: keep-indefinitely + per-account overrides).';
CREATE TRIGGER trg_accounts_updated BEFORE UPDATE ON accounts
  FOR EACH ROW EXECUTE FUNCTION thready_set_updated_at();

-- users are global identities; a user may belong to many accounts via memberships.
-- Exactly one root admin exists (is_root); enforced by a partial unique index.
CREATE TABLE users (
  id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  email           citext NOT NULL UNIQUE,
  password_hash   text   NOT NULL,                 -- Argon2id via digital.vasic.security
  display_name    text   NOT NULL DEFAULT '',
  totp_secret_enc bytea,                           -- AES-256-GCM sealed (security/pkg/securestorage)
  mfa_enabled     boolean NOT NULL DEFAULT false,
  is_root         boolean NOT NULL DEFAULT false,
  status          text NOT NULL DEFAULT 'active'
                    CHECK (status IN ('active','invited','disabled','deleted')),
  last_login_at   timestamptz,
  created_at      timestamptz NOT NULL DEFAULT now(),
  updated_at      timestamptz NOT NULL DEFAULT now()
);
-- Only ONE root admin may exist (final request §6.1 "only one exists").
CREATE UNIQUE INDEX uq_users_single_root ON users ((is_root)) WHERE is_root;
CREATE TRIGGER trg_users_updated BEFORE UPDATE ON users
  FOR EACH ROW EXECUTE FUNCTION thready_set_updated_at();

-- roles: system roles (account_id NULL) + account-scoped custom roles.
CREATE TABLE roles (
  id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  account_id  uuid REFERENCES accounts(id) ON DELETE CASCADE,  -- NULL = system role
  name        text NOT NULL,
  tier        text NOT NULL CHECK (tier IN ('root','account_admin','user')),
  description text NOT NULL DEFAULT '',
  created_at  timestamptz NOT NULL DEFAULT now(),
  UNIQUE (account_id, name)
);

CREATE TABLE permissions (
  id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  code        text NOT NULL UNIQUE,   -- e.g. 'post.read', 'account.manage', 'billing.view'
  description text NOT NULL DEFAULT ''
);

CREATE TABLE role_permissions (
  role_id       uuid NOT NULL REFERENCES roles(id)       ON DELETE CASCADE,
  permission_id uuid NOT NULL REFERENCES permissions(id) ON DELETE CASCADE,
  PRIMARY KEY (role_id, permission_id)
);

-- memberships: user <-> account <-> role. A user may belong to multiple accounts.
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

-- =============================================================================
--  DOMAIN B — Messenger & Ingestion
--  Provenance: final request §1.2, §3.1; [IN-HOUSE: herald], Telegram gotd/td,
--              Max = BUILD-NEW adapter. "complete post = root + organic replies".
-- =============================================================================

-- messengers: platform registry (telegram, max; extensible).
CREATE TABLE messengers (
  id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  kind         text NOT NULL UNIQUE CHECK (kind IN ('telegram','max')),
  display_name text NOT NULL,
  capabilities jsonb NOT NULL DEFAULT '{}'::jsonb,  -- {forum_topics:true, reply_threads:true,...}
  created_at   timestamptz NOT NULL DEFAULT now()
);

-- messenger_accounts: the operator's connected accounts/sessions (per tenant).
-- Session material is sensitive -> stored AES-256-GCM sealed; never logged.
CREATE TABLE messenger_accounts (
  id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  account_id   uuid NOT NULL REFERENCES accounts(id)   ON DELETE CASCADE,
  messenger_id uuid NOT NULL REFERENCES messengers(id) ON DELETE RESTRICT,
  external_ref text NOT NULL,             -- phone / bot username (non-secret handle)
  session_enc  bytea,                     -- gotd/td session or Max session, sealed
  auth_state   text NOT NULL DEFAULT 'unauthenticated'
                 CHECK (auth_state IN ('unauthenticated','pending_code','pending_2fa','authenticated','revoked')),
  status       text NOT NULL DEFAULT 'active'
                 CHECK (status IN ('active','disabled')),
  created_at   timestamptz NOT NULL DEFAULT now(),
  updated_at   timestamptz NOT NULL DEFAULT now(),
  UNIQUE (account_id, messenger_id, external_ref)
);
CREATE TRIGGER trg_messenger_accounts_updated BEFORE UPDATE ON messenger_accounts
  FOR EACH ROW EXECUTE FUNCTION thready_set_updated_at();

-- channels: channels / groups / forums to read from.
CREATE TABLE channels (
  id                    uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  account_id            uuid NOT NULL REFERENCES accounts(id)           ON DELETE CASCADE,
  messenger_account_id  uuid NOT NULL REFERENCES messenger_accounts(id) ON DELETE CASCADE,
  external_channel_id   text NOT NULL,
  kind                  text NOT NULL CHECK (kind IN ('channel','group','forum')),
  title                 text NOT NULL DEFAULT '',
  access_hash_enc       bytea,                       -- Telegram access_hash, sealed
  poll_interval_seconds integer NOT NULL DEFAULT 300 CHECK (poll_interval_seconds > 0),
  retention_days        integer,                     -- optional per-channel override; NULL = inherit
  is_active             boolean NOT NULL DEFAULT true,
  last_polled_at        timestamptz,
  created_at            timestamptz NOT NULL DEFAULT now(),
  updated_at            timestamptz NOT NULL DEFAULT now(),
  UNIQUE (messenger_account_id, external_channel_id),
  CONSTRAINT channels_retention_nonneg
    CHECK (retention_days IS NULL OR retention_days >= 0)
);
CREATE TRIGGER trg_channels_updated BEFORE UPDATE ON channels
  FOR EACH ROW EXECUTE FUNCTION thready_set_updated_at();

-- threads: the envelope of a "complete post". 1:1 with a root post (soft ref to
-- avoid the posts<->threads circular FK; both created in one tx by the reader).
CREATE TABLE threads (
  id                  uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  account_id          uuid NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
  channel_id          uuid NOT NULL REFERENCES channels(id) ON DELETE CASCADE,
  root_post_id        uuid,                 -- posts.id (partitioned target -> no DB FK)
  root_post_posted_at timestamptz,          -- denormalised partition key for pruned joins
  external_topic_id   bigint,               -- forum topic id (channels.getForumTopics)
  title               text NOT NULL DEFAULT '',
  reply_count         integer NOT NULL DEFAULT 0,
  last_activity_at    timestamptz NOT NULL DEFAULT now(),
  created_at          timestamptz NOT NULL DEFAULT now(),
  updated_at          timestamptz NOT NULL DEFAULT now(),
  UNIQUE (channel_id, external_topic_id, root_post_id)
);
CREATE TRIGGER trg_threads_updated BEFORE UPDATE ON threads
  FOR EACH ROW EXECUTE FUNCTION thready_set_updated_at();

-- posts: ROOT posts — the primary processed unit. Firehose table, RANGE-partitioned
-- on posted_at. PK includes the partition key. See partitioning.md for maintenance.
CREATE TABLE posts (
  id                 uuid NOT NULL DEFAULT gen_random_uuid(),
  account_id         uuid NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
  channel_id         uuid NOT NULL REFERENCES channels(id) ON DELETE CASCADE,
  thread_id          uuid REFERENCES threads(id) ON DELETE SET NULL,
  external_message_id bigint NOT NULL,
  author_ref         text NOT NULL DEFAULT '',
  raw_text           text NOT NULL DEFAULT '',
  lang               text,                          -- BCP-47; NULL until detected
  is_system_reply    boolean NOT NULL DEFAULT false,-- true = our own status reply; NEVER processed
  content_hash       bytea NOT NULL,                -- sha256(normalised body) for dedup/idempotency
  posted_at          timestamptz NOT NULL,          -- PARTITION KEY
  ingested_at        timestamptz NOT NULL DEFAULT now(),
  updated_at         timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (id, posted_at),
  UNIQUE (channel_id, external_message_id, posted_at)
) PARTITION BY RANGE (posted_at);
COMMENT ON COLUMN posts.is_system_reply IS
  'Processing SKIPS the system''s own replies — only organic human posts are processed (final request §3.2.3).';
-- Initial + default partitions (pg_partman/maintenance job manages the rolling set).
CREATE TABLE posts_2026_07 PARTITION OF posts
  FOR VALUES FROM ('2026-07-01') TO ('2026-08-01');
CREATE TABLE posts_2026_08 PARTITION OF posts
  FOR VALUES FROM ('2026-08-01') TO ('2026-09-01');
CREATE TABLE posts_default  PARTITION OF posts DEFAULT;

-- replies: organic replies within a thread. Firehose table, RANGE-partitioned.
-- Hashtags are frequently added as a reply to a link-only root -> replies carry tags too.
CREATE TABLE replies (
  id                  uuid NOT NULL DEFAULT gen_random_uuid(),
  account_id          uuid NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
  thread_id           uuid NOT NULL REFERENCES threads(id) ON DELETE CASCADE,
  parent_post_id      uuid NOT NULL,                 -- posts.id (partitioned -> no DB FK)
  parent_post_posted_at timestamptz,                 -- denormalised for pruned joins
  parent_reply_id     uuid,                          -- self-ref within reply chain (soft)
  external_message_id bigint NOT NULL,
  author_ref          text NOT NULL DEFAULT '',
  raw_text            text NOT NULL DEFAULT '',
  is_system_reply     boolean NOT NULL DEFAULT false,
  posted_at           timestamptz NOT NULL,          -- PARTITION KEY
  ingested_at         timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (id, posted_at),
  UNIQUE (thread_id, external_message_id, posted_at)
) PARTITION BY RANGE (posted_at);
CREATE TABLE replies_2026_07 PARTITION OF replies
  FOR VALUES FROM ('2026-07-01') TO ('2026-08-01');
CREATE TABLE replies_2026_08 PARTITION OF replies
  FOR VALUES FROM ('2026-08-01') TO ('2026-09-01');
CREATE TABLE replies_default  PARTITION OF replies DEFAULT;

-- =============================================================================
--  DOMAIN C — Classification (hashtags, categories/content-types)
--  Provenance: final request §3.2.2 (content types), §3.3 (additive categories),
--              §3.5 (indirect determination). Precedence: download>convert>analyze>research>reply.
-- =============================================================================

CREATE TABLE hashtags (
  id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tag        citext NOT NULL UNIQUE,        -- stored without '#'
  created_at timestamptz NOT NULL DEFAULT now()
);

-- categories = the documented content types (Video, Torrent, Research, Comic, …).
CREATE TABLE categories (
  id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  code            text NOT NULL UNIQUE,     -- 'video','torrent','research','comic',...
  display_name    text NOT NULL,
  precedence_class text NOT NULL
                    CHECK (precedence_class IN ('download','convert','analyze','research','reply')),
  sort_order      integer NOT NULL DEFAULT 100,   -- deterministic Skill ordering
  created_at      timestamptz NOT NULL DEFAULT now()
);
COMMENT ON COLUMN categories.precedence_class IS
  'Deterministic multi-hashtag precedence (final request §3.3): download > convert > analyze > research > reply.';

-- hashtag -> category mapping (many-to-many; a tag may imply several types).
CREATE TABLE hashtag_categories (
  hashtag_id  uuid NOT NULL REFERENCES hashtags(id)   ON DELETE CASCADE,
  category_id uuid NOT NULL REFERENCES categories(id) ON DELETE CASCADE,
  PRIMARY KEY (hashtag_id, category_id)
);

-- post <-> hashtag (denormalised post_posted_at for pruned joins; no DB FK to posts).
CREATE TABLE post_hashtags (
  post_id         uuid NOT NULL,
  post_posted_at  timestamptz NOT NULL,
  hashtag_id      uuid NOT NULL REFERENCES hashtags(id) ON DELETE CASCADE,
  source          text NOT NULL DEFAULT 'explicit'
                    CHECK (source IN ('explicit','indirect','ai_inferred')),
  PRIMARY KEY (post_id, hashtag_id)
);
COMMENT ON COLUMN post_hashtags.source IS
  'explicit = literal #tag; indirect = derived from link type (§3.5); ai_inferred = LLM fallback (§3.2.1).';

-- reply <-> hashtag (tags often live on replies).
CREATE TABLE reply_hashtags (
  reply_id        uuid NOT NULL,
  reply_posted_at timestamptz NOT NULL,
  hashtag_id      uuid NOT NULL REFERENCES hashtags(id) ON DELETE CASCADE,
  source          text NOT NULL DEFAULT 'explicit'
                    CHECK (source IN ('explicit','indirect','ai_inferred')),
  PRIMARY KEY (reply_id, hashtag_id)
);

-- post <-> category (additive classification; a post may be many categories).
CREATE TABLE post_categories (
  post_id        uuid NOT NULL,
  post_posted_at timestamptz NOT NULL,
  category_id    uuid NOT NULL REFERENCES categories(id) ON DELETE CASCADE,
  confidence     real NOT NULL DEFAULT 1.0 CHECK (confidence >= 0 AND confidence <= 1),
  PRIMARY KEY (post_id, category_id)
);

-- =============================================================================
--  DOMAIN D — Processing, Skills, Generated artifacts
--  Provenance: final request §3.2 (pipeline), §3.3 (idempotency/retry),
--              [IN-HOUSE: background] (Postgres task queue), [IN-HOUSE: helix_skills].
-- =============================================================================

-- processing_state: 1:1 with a post. Backs the idempotent single-claim (§3.3).
-- Standalone PK on post_id (uuid) — practically unique; app enforces the post ref.
CREATE TABLE processing_state (
  post_id        uuid PRIMARY KEY,
  post_posted_at timestamptz NOT NULL,
  account_id     uuid NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
  status         text NOT NULL DEFAULT 'pending'
                   CHECK (status IN ('pending','claimed','running','done','failed','skipped')),
  attempts       integer NOT NULL DEFAULT 0,
  max_attempts   integer NOT NULL DEFAULT 5,        -- §3.3 default retry ceiling
  claimed_by     text,                              -- worker id holding the claim
  claimed_at     timestamptz,
  visible_at     timestamptz NOT NULL DEFAULT now(),-- backoff gate: not claimable until now>=visible_at
  precedence     text,                              -- resolved highest precedence_class
  result         jsonb,
  last_error     text,
  created_at     timestamptz NOT NULL DEFAULT now(),
  updated_at     timestamptz NOT NULL DEFAULT now()
);
COMMENT ON TABLE processing_state IS
  'Idempotent single-claim per post (final request §3.3). A worker claims with SELECT ... FOR UPDATE SKIP LOCKED on (status=pending AND visible_at<=now()); exactly-once even under a post.received event storm.';
CREATE TRIGGER trg_processing_state_updated BEFORE UPDATE ON processing_state
  FOR EACH ROW EXECUTE FUNCTION thready_set_updated_at();

-- skills: relational mirror of the helix_skills Skill-Graph nodes Thready dispatches.
CREATE TABLE skills (
  id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  skill_key  text NOT NULL UNIQUE,          -- Skill-Graph node id (SKILL.md name)
  name       text NOT NULL,
  version    text NOT NULL DEFAULT '0.0.0',
  kind       text NOT NULL CHECK (kind IN ('atomic','composite','umbrella')),
  sort_order integer NOT NULL DEFAULT 100,  -- download-type before analysis-type (§3.3)
  is_enabled boolean NOT NULL DEFAULT true,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE TRIGGER trg_skills_updated BEFORE UPDATE ON skills
  FOR EACH ROW EXECUTE FUNCTION thready_set_updated_at();

-- skill_runs: one row per (post, skill) execution attempt.
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

-- generated_artifacts: research docs / books / transcripts / summaries produced by Skills.
-- Indexed semantically (see schema-vector.sql: vectordb_generated).
CREATE TABLE generated_artifacts (
  id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  account_id     uuid NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
  post_id        uuid NOT NULL,
  post_posted_at timestamptz NOT NULL,
  kind           text NOT NULL CHECK (kind IN ('research_doc','book','transcript','summary','other')),
  title          text NOT NULL DEFAULT '',
  storage_key    text,                       -- Asset Service / storage key if materialised
  lang           text NOT NULL DEFAULT 'en',
  created_at     timestamptz NOT NULL DEFAULT now(),
  updated_at     timestamptz NOT NULL DEFAULT now()
);
CREATE TRIGGER trg_generated_artifacts_updated BEFORE UPDATE ON generated_artifacts
  FOR EACH ROW EXECUTE FUNCTION thready_set_updated_at();

-- =============================================================================
--  DOMAIN E — Assets & links
--  Provenance: final request §7 (assets), [IN-HOUSE: Catalogizer + storage];
--              raw preserved + …-web rendition (parent_asset_id); content-hash dedup.
-- =============================================================================

CREATE TABLE assets (
  id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  account_id      uuid NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
  parent_asset_id uuid REFERENCES assets(id) ON DELETE SET NULL,  -- raw -> …-web rendition
  content_hash    bytea NOT NULL,             -- sha256 for dedup + integrity
  kind            text NOT NULL CHECK (kind IN ('video','audio','image','document','book','comic','other')),
  mime            text NOT NULL DEFAULT 'application/octet-stream',
  size_bytes      bigint NOT NULL DEFAULT 0 CHECK (size_bytes >= 0),
  storage_backend text NOT NULL DEFAULT 'minio' CHECK (storage_backend IN ('minio','s3','local')),
  storage_key     text NOT NULL,              -- object key; clients NEVER see raw paths
  is_web_rendition boolean NOT NULL DEFAULT false,
  is_encrypted    boolean NOT NULL DEFAULT false, -- specially-encrypted asset dir (§3.6)
  sensitivity     text NOT NULL DEFAULT 'internal'
                    CHECK (sensitivity IN ('public','internal','sensitive')),
  created_at      timestamptz NOT NULL DEFAULT now(),
  updated_at      timestamptz NOT NULL DEFAULT now(),
  UNIQUE (account_id, content_hash)           -- per-tenant content-hash dedup
);
CREATE TRIGGER trg_assets_updated BEFORE UPDATE ON assets
  FOR EACH ROW EXECUTE FUNCTION thready_set_updated_at();

-- asset_links: attach an asset to exactly one subject (post | reply | generated_artifact).
-- ordering_index preserves series/playlist order (numeric prefixes, §7.3).
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
  -- exactly one subject must be set
  CONSTRAINT asset_links_one_subject CHECK (
    (CASE WHEN post_id               IS NOT NULL THEN 1 ELSE 0 END) +
    (CASE WHEN reply_id              IS NOT NULL THEN 1 ELSE 0 END) +
    (CASE WHEN generated_artifact_id IS NOT NULL THEN 1 ELSE 0 END) = 1
  )
);

-- =============================================================================
--  DOMAIN F — Events & subscriptions (real-time)
--  Provenance: final request §3.4 (one-time vs sticky), [IN-HOUSE: eventbus/NATS JetStream].
--  The DB table is the durable AUDIT/replay catalog; JetStream is the live transport.
-- =============================================================================

CREATE TABLE events (
  id          uuid NOT NULL DEFAULT gen_random_uuid(),
  account_id  uuid NOT NULL,                          -- accounts.id (soft ref; high volume)
  type        text NOT NULL,                          -- 'post.received','processing.done',...
  scope       text NOT NULL DEFAULT 'one_time'
                CHECK (scope IN ('one_time','sticky')),
  entity_id   uuid,                                   -- sticky last-value key
  payload     jsonb NOT NULL DEFAULT '{}'::jsonb,
  invalidated boolean NOT NULL DEFAULT false,         -- sticky invalidation (§3.4)
  created_at  timestamptz NOT NULL,                   -- PARTITION KEY
  PRIMARY KEY (id, created_at)
) PARTITION BY RANGE (created_at);
CREATE TABLE events_2026_07 PARTITION OF events
  FOR VALUES FROM ('2026-07-01') TO ('2026-08-01');
CREATE TABLE events_default  PARTITION OF events DEFAULT;

-- event_subscriptions: client-facing subscription registrations (ws/sse/webhook).
CREATE TABLE event_subscriptions (
  id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  account_id uuid NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
  user_id    uuid REFERENCES users(id) ON DELETE CASCADE,
  pattern    text NOT NULL,                            -- subject filter, e.g. 'post.*'
  transport  text NOT NULL CHECK (transport IN ('ws','sse','webhook')),
  endpoint   text,                                     -- webhook URL (transport=webhook)
  is_active  boolean NOT NULL DEFAULT true,
  created_at timestamptz NOT NULL DEFAULT now()
);

-- =============================================================================
--  DOMAIN G — Billing & metering (subscription + metered, from day one)
--  Provenance: operator decision (Q11); final request §6.2.
-- =============================================================================

CREATE TABLE plans (
  id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  code        text NOT NULL UNIQUE,        -- 'free','pro','enterprise'
  display_name text NOT NULL,
  limits      jsonb NOT NULL DEFAULT '{}'::jsonb,  -- {channels:100, posts_per_day:10000,...}
  price_cents bigint NOT NULL DEFAULT 0 CHECK (price_cents >= 0),
  currency    text NOT NULL DEFAULT 'EUR',
  interval    text NOT NULL DEFAULT 'month' CHECK (interval IN ('month','year')),
  is_active   boolean NOT NULL DEFAULT true,
  created_at  timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE subscriptions (
  id                   uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  account_id           uuid NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
  plan_id              uuid NOT NULL REFERENCES plans(id)    ON DELETE RESTRICT,
  status               text NOT NULL DEFAULT 'trialing'
                         CHECK (status IN ('trialing','active','past_due','canceled')),
  current_period_start timestamptz NOT NULL DEFAULT now(),
  current_period_end   timestamptz NOT NULL,
  cancel_at_period_end boolean NOT NULL DEFAULT false,
  created_at           timestamptz NOT NULL DEFAULT now(),
  updated_at           timestamptz NOT NULL DEFAULT now()
);
CREATE TRIGGER trg_subscriptions_updated BEFORE UPDATE ON subscriptions
  FOR EACH ROW EXECUTE FUNCTION thready_set_updated_at();

-- usage_records: metered usage windows (posts processed, asset bytes, llm tokens, searches).
CREATE TABLE usage_records (
  id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  account_id   uuid NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
  metric       text NOT NULL
                 CHECK (metric IN ('posts_processed','assets_bytes','llm_tokens','searches')),
  quantity     numeric(20,4) NOT NULL DEFAULT 0 CHECK (quantity >= 0),
  window_start timestamptz NOT NULL,
  window_end   timestamptz NOT NULL,
  billed       boolean NOT NULL DEFAULT false,
  created_at   timestamptz NOT NULL DEFAULT now(),
  UNIQUE (account_id, metric, window_start)
);

CREATE TABLE invoices (
  id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  account_id      uuid NOT NULL REFERENCES accounts(id)      ON DELETE CASCADE,
  subscription_id uuid REFERENCES subscriptions(id)          ON DELETE SET NULL,
  amount_cents    bigint NOT NULL DEFAULT 0 CHECK (amount_cents >= 0),
  currency        text NOT NULL DEFAULT 'EUR',
  status          text NOT NULL DEFAULT 'open' CHECK (status IN ('open','paid','void')),
  issued_at       timestamptz NOT NULL DEFAULT now(),
  period_start    timestamptz NOT NULL,
  period_end      timestamptz NOT NULL
);

-- =============================================================================
--  DOMAIN H — Audit log (append-only, partitioned)
--  Provenance: final request §6.3, §14.4, Q40. All admin/user actions logged.
-- =============================================================================

CREATE TABLE audit_log (
  id            uuid NOT NULL DEFAULT gen_random_uuid(),
  account_id    uuid,                                  -- soft ref (append-only, never cascade)
  actor_user_id uuid,                                  -- soft ref
  action        text NOT NULL,                         -- 'account.update','user.invite',...
  target_type   text NOT NULL DEFAULT '',
  target_id     uuid,
  ip            inet,
  metadata      jsonb NOT NULL DEFAULT '{}'::jsonb,
  created_at    timestamptz NOT NULL,                  -- PARTITION KEY
  PRIMARY KEY (id, created_at)
) PARTITION BY RANGE (created_at);
COMMENT ON TABLE audit_log IS
  'Append-only. No UPDATE/DELETE in normal operation (enforce via role privileges + a rule/trigger). Retention default 1 year (Q40, adjustable).';
CREATE TABLE audit_log_2026_07 PARTITION OF audit_log
  FOR VALUES FROM ('2026-07-01') TO ('2026-08-01');
CREATE TABLE audit_log_default PARTITION OF audit_log DEFAULT;

-- =============================================================================
--  DOMAIN I — Archive catalog (bookkeeping for detached/cold partitions)
--  Provenance: retention-archive.md; digital.vasic.storage cold tier.
-- =============================================================================

CREATE TABLE archived_partitions (
  id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  source_table   text NOT NULL,                        -- 'posts','replies','events','audit_log'
  partition_name text NOT NULL,
  range_start    timestamptz NOT NULL,
  range_end      timestamptz NOT NULL,
  row_count      bigint,
  storage_backend text NOT NULL DEFAULT 'minio',
  storage_key    text NOT NULL,                        -- object key of the Parquet/SQL dump
  checksum       bytea,
  archived_at    timestamptz NOT NULL DEFAULT now(),
  dropped_at     timestamptz,                          -- set when the live partition is DROPped
  UNIQUE (source_table, partition_name)
);

-- =============================================================================
--  End of relational schema. Structural indexes are declared here as PK/UNIQUE;
--  the FULL secondary-index + FTS + ANN strategy lives in ./indexing.md and is
--  applied by later migrations. Vector tables live in ./schema-vector.sql.
-- =============================================================================
