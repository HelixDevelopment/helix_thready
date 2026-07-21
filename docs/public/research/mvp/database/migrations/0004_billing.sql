-- =============================================================================
--  Migration 0004 — billing & metering (subscription + metered, from day one)
--  Classification : PUBLIC
--  Location       : docs/public/research/mvp/database/migrations/0004_billing.sql
--  Revision       : 1 (2026-07-22) — swarm (database)
--  Applied via    : digital.vasic.database/pkg/migration.Runner  [IN-HOUSE: database]
--  Depends on     : 0001_init (accounts exists)
--  Provenance     : operator decision Q11 (subscription + metered); final request §6.2.
--
--  All pure DDL in one transaction. Secondary indexes are added later in 0007.
-- =============================================================================

-- +thready Up

-- plans: the sellable subscription tiers with a JSONB limits object.
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
COMMENT ON COLUMN plans.limits      IS 'Per-plan ceilings as JSONB, e.g. {"channels":100,"posts_per_day":10000,"seats":100} — matches the Large-scale caps (Q2).';
COMMENT ON COLUMN plans.price_cents IS 'Minor-unit integer price (avoids float rounding). 0 = free tier.';

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
COMMENT ON COLUMN subscriptions.status IS 'Lifecycle: trialing -> active -> past_due -> canceled. plan_id uses ON DELETE RESTRICT so an in-use plan cannot be deleted.';
CREATE TRIGGER trg_subscriptions_updated BEFORE UPDATE ON subscriptions
  FOR EACH ROW EXECUTE FUNCTION thready_set_updated_at();

-- usage_records: metered usage windows. UNIQUE(account_id, metric, window_start) => idempotent metering.
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
COMMENT ON TABLE  usage_records          IS 'Metered usage per (account, metric, window). UNIQUE key makes repeated meter flushes idempotent (upsert-on-conflict).';
COMMENT ON COLUMN usage_records.quantity IS 'numeric(20,4): exact accumulation (bytes/tokens/counts) without float drift.';
COMMENT ON COLUMN usage_records.billed   IS 'Reconciliation flag: set true once rolled into an invoice; index idx_usage_unbilled tracks the open set.';

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
COMMENT ON COLUMN invoices.subscription_id IS 'ON DELETE SET NULL: an invoice survives subscription deletion (financial record must persist).';

-- +thready Down

DROP TABLE IF EXISTS invoices      CASCADE;
DROP TABLE IF EXISTS usage_records CASCADE;
DROP TABLE IF EXISTS subscriptions CASCADE;
DROP TABLE IF EXISTS plans         CASCADE;
