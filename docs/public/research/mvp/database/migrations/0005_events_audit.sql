-- =============================================================================
--  Migration 0005 — events, subscriptions & audit log (partitioned firehoses)
--  Classification : PUBLIC
--  Location       : docs/public/research/mvp/database/migrations/0005_events_audit.sql
--  Revision       : 1 (2026-07-22) — swarm (database)
--  Applied via    : digital.vasic.database/pkg/migration.Runner  [IN-HOUSE: database]
--  Depends on     : 0001_init (accounts, users exist)
--  Provenance     : final request §3.4 (one-time vs sticky), §6.3/§14.4/Q40 (audit),
--                   [IN-HOUSE: eventbus/NATS JetStream = live transport; DB = replay catalog].
--
--  events and audit_log are RANGE-partitioned on created_at (their PK is (id, created_at)).
--  Partition maintenance (create-ahead of new months) is the scheduled job in
--  partitioning.md — NOT a migration. Only the parents + bootstrap partitions live here.
-- =============================================================================

-- +thready Up

-- events: durable AUDIT/replay catalog for the real-time system. Live transport is
-- NATS JetStream (digital.vasic.eventbus); this table is the queryable mirror.
CREATE TABLE events (
  id          uuid NOT NULL DEFAULT gen_random_uuid(),
  account_id  uuid NOT NULL,                              -- soft ref (high volume; no cascade)
  type        text NOT NULL,
  scope       text NOT NULL DEFAULT 'one_time'
                CHECK (scope IN ('one_time','sticky')),
  entity_id   uuid,
  payload     jsonb NOT NULL DEFAULT '{}'::jsonb,
  invalidated boolean NOT NULL DEFAULT false,
  created_at  timestamptz NOT NULL,                       -- PARTITION KEY
  PRIMARY KEY (id, created_at)
) PARTITION BY RANGE (created_at);
COMMENT ON TABLE  events             IS 'Durable event replay catalog (final request §3.4). RANGE-partitioned on created_at; short retention window.';
COMMENT ON COLUMN events.scope       IS 'one_time = fire-and-consume; sticky = last-value retained per entity_id (invalidated flips on state change/TTL).';
COMMENT ON COLUMN events.account_id  IS 'Soft reference to accounts.id (no DB FK: firehose volume; a replayed event must survive account deletion).';
CREATE TABLE events_2026_07 PARTITION OF events
  FOR VALUES FROM ('2026-07-01') TO ('2026-08-01');
CREATE TABLE events_2026_08 PARTITION OF events
  FOR VALUES FROM ('2026-08-01') TO ('2026-09-01');
CREATE TABLE events_default  PARTITION OF events DEFAULT;

-- event_subscriptions: client-facing subscription registrations (bounded per tenant -> real FKs).
CREATE TABLE event_subscriptions (
  id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  account_id uuid NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
  user_id    uuid REFERENCES users(id) ON DELETE CASCADE,
  pattern    text NOT NULL,
  transport  text NOT NULL CHECK (transport IN ('ws','sse','webhook')),
  endpoint   text,
  is_active  boolean NOT NULL DEFAULT true,
  created_at timestamptz NOT NULL DEFAULT now()
);
COMMENT ON COLUMN event_subscriptions.pattern   IS 'Subject filter, e.g. ''post.*'' or ''processing.done'' (matches the eventbus subject space).';
COMMENT ON COLUMN event_subscriptions.endpoint  IS 'Outbound webhook URL; required when transport = ''webhook'', NULL for ws/sse.';

-- audit_log: append-only action trail. RANGE-partitioned; default 1-year retention (Q40).
CREATE TABLE audit_log (
  id            uuid NOT NULL DEFAULT gen_random_uuid(),
  account_id    uuid,                                     -- soft ref
  actor_user_id uuid,                                     -- soft ref
  action        text NOT NULL,
  target_type   text NOT NULL DEFAULT '',
  target_id     uuid,
  ip            inet,
  metadata      jsonb NOT NULL DEFAULT '{}'::jsonb,
  created_at    timestamptz NOT NULL,                     -- PARTITION KEY
  PRIMARY KEY (id, created_at)
) PARTITION BY RANGE (created_at);
COMMENT ON TABLE  audit_log            IS 'Append-only. No UPDATE/DELETE in normal operation (enforce via role privileges + optional rule/trigger). Retention default 1 year (Q40, adjustable).';
COMMENT ON COLUMN audit_log.ip         IS 'Source IP of the actor (inet type); NULL for system-initiated actions.';
CREATE TABLE audit_log_2026_07 PARTITION OF audit_log
  FOR VALUES FROM ('2026-07-01') TO ('2026-08-01');
CREATE TABLE audit_log_2026_08 PARTITION OF audit_log
  FOR VALUES FROM ('2026-08-01') TO ('2026-09-01');
CREATE TABLE audit_log_default PARTITION OF audit_log DEFAULT;

-- archived_partitions: bookkeeping for detached/cold partitions (Domain I). Unpartitioned.
-- Backs the retention/archive pipeline (retention-archive.md §4): archive-before-drop +
-- restorable via the recorded storage_key + checksum.
CREATE TABLE archived_partitions (
  id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  source_table    text NOT NULL,
  partition_name  text NOT NULL,
  range_start     timestamptz NOT NULL,
  range_end       timestamptz NOT NULL,
  row_count       bigint,
  storage_backend text NOT NULL DEFAULT 'minio',
  storage_key     text NOT NULL,
  checksum        bytea,
  archived_at     timestamptz NOT NULL DEFAULT now(),
  dropped_at      timestamptz,
  UNIQUE (source_table, partition_name)
);
COMMENT ON TABLE  archived_partitions            IS 'Catalog of partitions archived to cold storage; dropped_at is set when the live partition is DROPped (archive-before-drop).';
COMMENT ON COLUMN archived_partitions.checksum   IS 'Checksum of the cold-storage dump, verified before DROP and on re-attach for audits/legal hold.';

-- +thready Down

DROP TABLE IF EXISTS archived_partitions CASCADE;
DROP TABLE IF EXISTS audit_log           CASCADE;  -- drops child partitions with it
DROP TABLE IF EXISTS event_subscriptions CASCADE;
DROP TABLE IF EXISTS events              CASCADE;
