-- =============================================================================
--  Materials migration 0003 — billing & metering (self-contained expand)
--  Classification : PUBLIC
--  Location       : docs/public/research/mvp/database/materials/migrations/0003_billing_metering.sql
--  Revision       : 1 (2026-07-22) — swarm (database, materials pack)
--  Applied via    : digital.vasic.database/pkg/migration.Runner  [IN-HOUSE: database]
--  Depends on     : ../../migrations/0001_init.sql (accounts, thready_set_updated_at()).
--  Provenance     : operator Q11 (subscription + metered, from day one), final request §6.2,
--                   erd.md §6 (billing domain), indexing.md (usage/subscription indexes).
--
--  >>> RELATIONSHIP TO THE CANONICAL CHAIN (anti-bluff) <<<
--   The canonical ../../migrations/0004_billing.sql owns the four base billing tables
--   (plans, subscriptions, usage_records, invoices) and remains the SOURCE OF TRUTH.
--   THIS materials migration is a SELF-CONTAINED expand on the 0001 baseline that (a)
--   defines those same four base tables (identical shapes, so a DB stood up either way is
--   schema-compatible) AND (b) adds the METERING-AUTOMATION layer the canonical 0004 does
--   NOT ship: usage_rollups, an append-only billing_events ledger, and the record_usage /
--   add_usage / rollup_usage functions.
--     => It is an ALTERNATIVE to canonical 0004+0007 billing objects. Do NOT apply BOTH
--        this file and canonical 0004 to the same schema (duplicate table definitions).
--        Materials-pack numbering caveat also applies: renumber before folding upstream
--        (the canonical chain already uses 0002–0007).
--
--  EXPAND-CONTRACT STYLE: this is the EXPAND phase — every object is additive and
--  backward-compatible (new tables, new nullable-defaulted columns, new indexes, new
--  functions); N-1 services keep working. Nothing is dropped or rewritten. A later CONTRACT
--  migration (illustrated, commented, at the bottom) is where a domain would be tightened.
--
--  TRANSACTIONAL: yes. All DDL, indexes on NEW empty tables (plain CREATE INDEX is instant),
--  and functions — no CONCURRENTLY, no bind params — so the runner's single applyOne
--  transaction handles it.
-- =============================================================================

-- +thready Up

-- ---- Base billing tables (shapes identical to canonical 0004) ---------------
-- plans: sellable subscription tiers with a JSONB limits object.
CREATE TABLE plans (
  id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  code         text NOT NULL UNIQUE,
  display_name text NOT NULL,
  limits       jsonb NOT NULL DEFAULT '{}'::jsonb,
  price_cents  bigint NOT NULL DEFAULT 0 CHECK (price_cents >= 0),
  currency     text NOT NULL DEFAULT 'EUR',
  interval     text NOT NULL DEFAULT 'month' CHECK (interval IN ('month','year')),
  is_active    boolean NOT NULL DEFAULT true,
  created_at   timestamptz NOT NULL DEFAULT now()
);
COMMENT ON COLUMN plans.limits IS 'Per-plan ceilings as JSONB, e.g. {"channels":100,"posts_per_day":10000,"seats":100} (Large-scale caps, Q2).';

-- subscriptions: account <-> plan billing binding with a period + status lifecycle.
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

-- usage_records: metered windows. UNIQUE(account_id, metric, window_start) => idempotent metering.
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

-- invoices: a period charge = plan fee + metered usage.
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

-- Base secondary indexes (new empty tables => plain CREATE INDEX is instant & tx-safe).
CREATE INDEX idx_usage_account_metric  ON usage_records (account_id, metric, window_start DESC);
CREATE INDEX idx_usage_unbilled        ON usage_records (account_id) WHERE NOT billed;
CREATE INDEX idx_subscriptions_account ON subscriptions (account_id, status);
CREATE INDEX idx_invoices_account      ON invoices      (account_id, issued_at DESC);

-- ---- Metering-automation layer (NET-NEW vs canonical 0004) ------------------

-- usage_rollups: monthly per-(account,metric) aggregate of the raw usage windows. The
-- reconciliation surface the invoicing job reads. UNIQUE => idempotent rollup upsert.
CREATE TABLE usage_rollups (
  id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  account_id   uuid NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
  metric       text NOT NULL
                 CHECK (metric IN ('posts_processed','assets_bytes','llm_tokens','searches')),
  period_month date NOT NULL,                       -- first day of the billing month (UTC)
  quantity     numeric(20,4) NOT NULL DEFAULT 0 CHECK (quantity >= 0),
  invoiced     boolean NOT NULL DEFAULT false,
  updated_at   timestamptz NOT NULL DEFAULT now(),
  UNIQUE (account_id, metric, period_month),
  CONSTRAINT usage_rollups_month_is_first CHECK (period_month = date_trunc('month', period_month)::date)
);
CREATE INDEX idx_usage_rollups_open ON usage_rollups (account_id) WHERE NOT invoiced;
CREATE TRIGGER trg_usage_rollups_updated BEFORE UPDATE ON usage_rollups
  FOR EACH ROW EXECUTE FUNCTION thready_set_updated_at();
COMMENT ON TABLE usage_rollups IS 'Monthly aggregate of usage_records per (account,metric). Idempotent rollup target; invoiced flag drives reconciliation.';

-- billing_events: APPEND-ONLY lifecycle ledger (defence-in-depth via the reject trigger
-- below, mirroring the audit_log append-only discipline, constraints-and-integrity.md §6).
CREATE TABLE billing_events (
  id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  account_id      uuid NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
  subscription_id uuid,                              -- soft ref (event survives sub deletion)
  type            text NOT NULL CHECK (type IN (
                    'subscription.created','subscription.updated','subscription.canceled',
                    'usage.metered','invoice.issued','invoice.paid','invoice.void',
                    'payment.received','payment.failed')),
  amount_cents    bigint CHECK (amount_cents IS NULL OR amount_cents >= 0),
  currency        text NOT NULL DEFAULT 'EUR',
  metadata        jsonb NOT NULL DEFAULT '{}'::jsonb,
  created_at      timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_billing_events_account ON billing_events (account_id, created_at DESC);

-- Append-only guard: reject UPDATE/DELETE on the ledger (least-privilege + this trigger).
CREATE OR REPLACE FUNCTION thready_reject_mutation() RETURNS trigger
LANGUAGE plpgsql AS $fn$
BEGIN
  RAISE EXCEPTION 'table % is append-only: % is not permitted', TG_TABLE_NAME, TG_OP;
END;
$fn$;
CREATE TRIGGER trg_billing_events_append_only
  BEFORE UPDATE OR DELETE ON billing_events
  FOR EACH ROW EXECUTE FUNCTION thready_reject_mutation();

-- record_usage: IDEMPOTENT ABSOLUTE-SET metering. The caller passes the CURRENT running
-- total for (account,metric,window_start); re-flushing the same window is a no-op change
-- (matches the "repeated meter flushes idempotent" contract of usage_records' UNIQUE key).
CREATE OR REPLACE FUNCTION record_usage(
  p_account uuid, p_metric text, p_quantity numeric,
  p_window_start timestamptz, p_window_end timestamptz)
RETURNS uuid
LANGUAGE plpgsql AS $fn$
DECLARE rec_id uuid;
BEGIN
  INSERT INTO usage_records (account_id, metric, quantity, window_start, window_end)
  VALUES (p_account, p_metric, p_quantity, p_window_start, p_window_end)
  ON CONFLICT (account_id, metric, window_start) DO UPDATE
    SET quantity   = EXCLUDED.quantity,                         -- absolute set => idempotent
        window_end = GREATEST(usage_records.window_end, EXCLUDED.window_end)
  RETURNING id INTO rec_id;
  RETURN rec_id;
END;
$fn$;
COMMENT ON FUNCTION record_usage(uuid, text, numeric, timestamptz, timestamptz) IS
  'Idempotent absolute-set metering upsert. Pass the window running total; safe to retry.';

-- add_usage: ACCUMULATE deltas into a window. Use for a delta pipeline; NOT retry-idempotent
-- (the caller must dedupe redelivered deltas). Provided as the explicit alternative to
-- record_usage so callers pick semantics deliberately.
CREATE OR REPLACE FUNCTION add_usage(
  p_account uuid, p_metric text, p_delta numeric,
  p_window_start timestamptz, p_window_end timestamptz)
RETURNS uuid
LANGUAGE plpgsql AS $fn$
DECLARE rec_id uuid;
BEGIN
  INSERT INTO usage_records (account_id, metric, quantity, window_start, window_end)
  VALUES (p_account, p_metric, p_delta, p_window_start, p_window_end)
  ON CONFLICT (account_id, metric, window_start) DO UPDATE
    SET quantity   = usage_records.quantity + EXCLUDED.quantity,  -- accumulate
        window_end = GREATEST(usage_records.window_end, EXCLUDED.window_end)
  RETURNING id INTO rec_id;
  RETURN rec_id;
END;
$fn$;
COMMENT ON FUNCTION add_usage(uuid, text, numeric, timestamptz, timestamptz) IS
  'Accumulating metering upsert for delta pipelines. NOT retry-idempotent — caller dedupes.';

-- rollup_usage: fold all raw usage windows for (account, month) into usage_rollups.
-- Idempotent absolute set: recomputes the month total from usage_records every call.
CREATE OR REPLACE FUNCTION rollup_usage(p_account uuid, p_month date)
RETURNS integer
LANGUAGE plpgsql AS $fn$
DECLARE
  month_start date := date_trunc('month', p_month)::date;
  month_end   date := (date_trunc('month', p_month) + interval '1 month')::date;
  n           integer;
BEGIN
  INSERT INTO usage_rollups (account_id, metric, period_month, quantity)
  SELECT u.account_id, u.metric, month_start, COALESCE(SUM(u.quantity), 0)
  FROM   usage_records u
  WHERE  u.account_id   = p_account
    AND  u.window_start >= month_start
    AND  u.window_start <  month_end
  GROUP BY u.account_id, u.metric
  ON CONFLICT (account_id, metric, period_month) DO UPDATE
    SET quantity = EXCLUDED.quantity;                           -- recompute => idempotent
  GET DIAGNOSTICS n = ROW_COUNT;
  RETURN n;
END;
$fn$;
COMMENT ON FUNCTION rollup_usage(uuid, date) IS
  'Idempotent monthly rollup of usage_records into usage_rollups for one account. Returns rows upserted.';

-- Convenience view: open (unbilled) raw usage per account+metric — the reconciliation set.
CREATE VIEW unbilled_usage AS
  SELECT account_id, metric, COUNT(*) AS windows, SUM(quantity) AS total_quantity,
         MIN(window_start) AS since
  FROM   usage_records
  WHERE  NOT billed
  GROUP  BY account_id, metric;

-- +thready Down

-- Full reverse (self-contained migration => drop everything it created, deepest first).
DROP VIEW     IF EXISTS unbilled_usage;
DROP FUNCTION IF EXISTS rollup_usage(uuid, date);
DROP FUNCTION IF EXISTS add_usage(uuid, text, numeric, timestamptz, timestamptz);
DROP FUNCTION IF EXISTS record_usage(uuid, text, numeric, timestamptz, timestamptz);
DROP TABLE    IF EXISTS billing_events CASCADE;          -- drops its append-only trigger
DROP FUNCTION IF EXISTS thready_reject_mutation();
DROP TABLE    IF EXISTS usage_rollups  CASCADE;
DROP TABLE    IF EXISTS invoices       CASCADE;
DROP TABLE    IF EXISTS usage_records  CASCADE;
DROP TABLE    IF EXISTS subscriptions  CASCADE;
DROP TABLE    IF EXISTS plans          CASCADE;

-- =============================================================================
--  ILLUSTRATIVE CONTRACT phase (NOT part of this migration — shown to complete the
--  expand->migrate->contract discipline, migration-strategy.md §5). Once every service
--  reads usage_rollups and nothing reads raw windows for billing, a later migration could
--  tighten the model, e.g. mark billed windows immutable or drop a superseded column:
--
--    -- 000N_contract_usage.sql
--    -- +thready Up
--    ALTER TABLE usage_records ADD CONSTRAINT usage_billed_frozen
--      CHECK (NOT billed OR quantity >= 0);        -- example post-backfill tightening
--    -- +thready Down
--    ALTER TABLE usage_records DROP CONSTRAINT usage_billed_frozen;
-- =============================================================================
