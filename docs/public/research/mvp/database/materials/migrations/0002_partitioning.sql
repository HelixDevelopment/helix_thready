-- =============================================================================
--  Materials migration 0002 — partition maintenance (SQL layer)
--  Classification : PUBLIC
--  Location       : docs/public/research/mvp/database/materials/migrations/0002_partitioning.sql
--  Revision       : 1 (2026-07-22) — swarm (database, materials pack)
--  Applied via    : digital.vasic.database/pkg/migration.Runner  [IN-HOUSE: database]
--  Depends on     : ../../migrations/0001_init.sql (posts & replies RANGE-partition parents,
--                     thready_set_updated_at()). This pack sits ON the 0001 baseline.
--  Provenance     : partitioning.md §4/§5 (create-ahead + detach), §7 (autovacuum/fillfactor),
--                   [GAP: database-3.2] (no partition helpers in the module today).
--
--  >>> MATERIALS-PACK NUMBERING CAVEAT (anti-bluff) <<<
--   This is an ILLUSTRATIVE expand migration in the materials pack, versioned on top of the
--   0001 baseline. The CANONICAL chain (../../migrations/) already uses versions 0002–0007,
--   so this file must NOT be loaded into the SAME schema_migrations table as the canonical
--   0002_classification. When folded upstream it must be RENUMBERED (e.g. 0008). It is
--   provided as a runnable reference for the SQL partition-maintenance layer that the design
--   docs describe only as a Go helper (partitioning.md §5 MonthlyRange).
--
--  WHAT IT ADDS  (pure EXPAND — additive; no contract, nothing is removed/rewritten):
--   1. thready_ensure_month_partition(parent, month_start) — idempotent: creates ONE monthly
--      partition if missing and stamps the aggressive-autovacuum + high-fillfactor profile.
--   2. thready_partition_maintenance(premake) — the scheduled-job entry point (uses now()):
--      ensures current + `premake` months ahead for posts and replies. NOT called from this
--      migration (migrations must be deterministic); it is invoked by digital.vasic.background
--      on a monthly cadence (partitioning.md §5).
--   3. A DETERMINISTIC forward window of monthly partitions (literal months) for posts and
--      replies, and (re-)stamps the storage profile on the 0001 bootstrap partitions.
--
--  TRANSACTIONAL: yes. All statements are plain DDL / function DDL / a deterministic DO loop
--  with NO bind parameters and NO CONCURRENTLY, so the runner's single applyOne transaction
--  (pgx simple protocol, multi-statement) handles it. Contrast ../../migrations/0007 (ATM-DB-002).
-- =============================================================================

-- +thready Up

-- 1. Idempotent single-partition ensure. Creates <parent>_<YYYY>_<MM> for the half-open
--    range [month_start, month_start+1mo) and stamps the firehose storage profile.
CREATE OR REPLACE FUNCTION thready_ensure_month_partition(parent_table text, month_start date)
RETURNS text
LANGUAGE plpgsql AS $fn$
DECLARE
  part_name  text;
  next_start date := (month_start + interval '1 month')::date;
BEGIN
  IF month_start <> date_trunc('month', month_start)::date THEN
    RAISE EXCEPTION 'month_start % must be the first day of a month (half-open UTC ranges)', month_start;
  END IF;
  part_name := format('%s_%s', parent_table, to_char(month_start, 'YYYY_MM'));
  -- Metadata-only; does not lock existing partitions. IF NOT EXISTS => safe re-run.
  EXECUTE format(
    'CREATE TABLE IF NOT EXISTS %I PARTITION OF %I FOR VALUES FROM (%L) TO (%L)',
    part_name, parent_table, month_start::text, next_start::text);
  -- Append-mostly firehose: aggressive autovacuum + high fillfactor (partitioning.md §7).
  EXECUTE format(
    'ALTER TABLE %I SET ('
      || 'autovacuum_vacuum_scale_factor = 0.02, '
      || 'autovacuum_analyze_scale_factor = 0.02, '
      || 'autovacuum_vacuum_cost_delay = 2, '
      || 'fillfactor = 95)',
    part_name);
  RETURN part_name;
END;
$fn$;
COMMENT ON FUNCTION thready_ensure_month_partition(text, date) IS
  'Idempotent create-ahead of one monthly partition + firehose storage profile. SQL analogue of partition.MonthlyRange.EnsureAhead (partitioning.md §5).';

-- 2. Scheduled-job entry point (NON-deterministic via now(); NOT run by this migration).
--    Ensures current + `premake` months exist for every firehose parent. Returns the
--    partition names it touched. Wrap the call in pg_advisory_xact_lock in the job so it
--    never races a concurrent migration (migration-strategy.md §7).
CREATE OR REPLACE FUNCTION thready_partition_maintenance(premake int DEFAULT 3)
RETURNS SETOF text
LANGUAGE plpgsql AS $fn$
DECLARE
  parents text[] := ARRAY['posts','replies'];
  parent  text;
  i       int;
  m       date;
BEGIN
  IF premake < 0 THEN
    RAISE EXCEPTION 'premake must be >= 0';
  END IF;
  FOREACH parent IN ARRAY parents LOOP
    FOR i IN 0..premake LOOP
      m := (date_trunc('month', now()) + (i || ' month')::interval)::date;
      RETURN NEXT thready_ensure_month_partition(parent, m);
    END LOOP;
  END LOOP;
END;
$fn$;
COMMENT ON FUNCTION thready_partition_maintenance(int) IS
  'Monthly create-ahead job entry point for posts+replies. Idempotent; call under an advisory lock. detach/archive of aged partitions is the retention job (retention-archive.md §4).';

-- 3. Deterministic forward window (literal months => reproducible for the pre-tag
--    structural-diff gate). 0001 shipped posts/replies_2026_07 and _2026_08; this ensures
--    2026_07 .. 2027_02 exist for BOTH parents and (re-)stamps the storage profile on all of
--    them (the ensure function is idempotent, so 07/08 are just re-stamped, not re-created).
DO $do$
DECLARE
  parents text[] := ARRAY['posts','replies'];
  months  date[] := ARRAY[
    DATE '2026-07-01', DATE '2026-08-01', DATE '2026-09-01', DATE '2026-10-01',
    DATE '2026-11-01', DATE '2026-12-01', DATE '2027-01-01', DATE '2027-02-01'];
  parent text;
  m      date;
BEGIN
  FOREACH parent IN ARRAY parents LOOP
    FOREACH m IN ARRAY months LOOP
      PERFORM thready_ensure_month_partition(parent, m);
    END LOOP;
  END LOOP;
END;
$do$;

-- OPTIONAL HARDENING (NOT applied by default) — composite FK into a partitioned parent,
-- the DB-enforced alternative to app-enforced integrity (partitioning.md §8, ATM-DB-004).
-- Requires the child to carry the partition key (processing_state.post_posted_at exists).
-- Uncomment per-environment if you prefer DB-enforced integrity over write throughput:
--   ALTER TABLE processing_state
--     ADD CONSTRAINT fk_processing_post
--     FOREIGN KEY (post_id, post_posted_at) REFERENCES posts (id, posted_at) ON DELETE CASCADE;

-- +thready Down

-- Reverse §3's forward window: drop the create-ahead partitions THIS migration added
-- (2026_09 .. 2027_02) for both parents. The 0001 baseline partitions (2026_07/2026_08) are
-- left in place — they belong to 0001 — but their storage profile is reset to defaults.
-- NOTE (migration-strategy.md §6): dropping a POPULATED partition loses its rows; on a live
-- deployment recover from PITR backup. These create-ahead partitions are normally empty.
DO $do$
DECLARE
  parents text[] := ARRAY['posts','replies'];
  drop_months date[] := ARRAY[
    DATE '2026-09-01', DATE '2026-10-01', DATE '2026-11-01', DATE '2026-12-01',
    DATE '2027-01-01', DATE '2027-02-01'];
  keep_months date[] := ARRAY[DATE '2026-07-01', DATE '2026-08-01'];
  parent text;
  m      date;
  nm     text;
BEGIN
  FOREACH parent IN ARRAY parents LOOP
    FOREACH m IN ARRAY drop_months LOOP
      nm := format('%s_%s', parent, to_char(m, 'YYYY_MM'));
      EXECUTE format('DROP TABLE IF EXISTS %I', nm);
    END LOOP;
    FOREACH m IN ARRAY keep_months LOOP
      nm := format('%s_%s', parent, to_char(m, 'YYYY_MM'));
      EXECUTE format(
        'ALTER TABLE %I RESET ('
          || 'autovacuum_vacuum_scale_factor, autovacuum_analyze_scale_factor, '
          || 'autovacuum_vacuum_cost_delay, fillfactor)', nm);
    END LOOP;
  END LOOP;
END;
$do$;

DROP FUNCTION IF EXISTS thready_partition_maintenance(int);
DROP FUNCTION IF EXISTS thready_ensure_month_partition(text, date);
