-- =============================================================================
--  Helix Thready — Reference / lookup SEED data
--  Classification : PUBLIC
--  Location       : docs/public/research/mvp/database/materials/seed.sql
--  Status         : Draft — v0.1
--  Revision       : 1 (2026-07-22) — swarm (database, materials pack)
--  Related        : ../migrations/0001_init.sql (accounts/users/roles/permissions/
--                     memberships/messengers), ../migrations/0002_classification.sql
--                     (hashtags/categories/hashtag_categories),
--                     ../migrations/0003_assets.sql (skills), ../erd.md §3/§4/§5,
--                     ./migrations/0003_billing_metering.sql (plans, at the bottom).
--  Provenance     : final request §3.2.2 (content types), §3.3 (precedence),
--                   §3.5 (indirect determination), §6.1 (3-tier RBAC), §4.1 (messengers),
--                   HelixSkills §3.2.2 (one Skill per content type).  [OPERATOR]/[RESEARCH]
--
--  WHAT THIS IS
--    Idempotent reference/lookup rows for the tables created by the canonical migration
--    chain (../migrations/0001..0003). It seeds ONLY tenant-independent lookup data:
--    content-type categories + the hashtag taxonomy, the three SYSTEM RBAC roles and
--    their permission grants, the default Skill registry, and the messenger providers.
--    It creates NO tenant data (no accounts, no users, no memberships) — Root-admin
--    bootstrap is a DEPLOY concern (final request §22 Q7 "Initial account creation"),
--    not seed data.
--
--  HOW TO RUN
--    Apply AFTER the schema migrations, e.g.  psql "$DSN" -f seed.sql  (or feed through
--    digital.vasic.database as a data step). Every INSERT is guarded by ON CONFLICT
--    DO NOTHING against a UNIQUE/PK column, so re-running is a safe no-op (matches the
--    idempotency discipline in migration-strategy.md).
--
--  ANTI-BLUFF NOTES (CONVENTIONS §7)
--    * VERIFIED: the target column domains match the shipped CHECK constraints —
--        categories.precedence_class IN ('download','convert','analyze','research','reply')
--        roles.tier                 IN ('root','account_admin','user')
--        skills.kind                IN ('atomic','composite','umbrella')
--        messengers.kind            IN ('telegram','max')
--      (read at ../migrations/0002,0003,0001). Inserting an out-of-domain value would be
--      rejected by the DB — this seed stays inside those domains.
--    * ASSUMED [DEFAULT — adjustable]: the exact category/skill/permission CODES and
--      sort_order values below are proposed defaults derived from the request text; they
--      are data, not schema, so they can be edited without a migration.
--    * The `max` messenger is seeded as a REGISTERED provider only. Per the gap register
--      (§5, P0) the Max adapter is an EMPTY STUB — the row does NOT imply a working
--      integration; its `capabilities.status` is 'stub' so callers never assume otherwise.
-- =============================================================================

BEGIN;

-- =============================================================================
-- 1. CONTENT-TYPE CATEGORIES  (final request §3.2.2 / §3.3 precedence)
--    `precedence_class` drives deterministic multi-hashtag Skill ordering:
--       download > convert > analyze > research > reply   (§3.3)
--    `sort_order` is the secondary tiebreak WITHIN a class (lower = dispatched first,
--    so download-type Skills run before analysis-type Skills and research can consume
--    already-downloaded media).
-- =============================================================================
INSERT INTO categories (code, display_name, precedence_class, sort_order) VALUES
  -- ---- download class (media acquisition; run first) ----
  ('video',        'Video',                     'download', 10),
  ('torrent',      'Torrent / Magnet',          'download', 12),
  ('series',       'Serial / Series',           'download', 14),
  ('movie',        'Movie',                     'download', 16),
  ('documentary',  'Documentary',               'download', 18),
  ('concert',      'Concert',                   'download', 20),
  ('game',         'Game',                      'download', 22),
  ('software',     'Software',                  'download', 24),
  ('channel',      'Channel',                   'download', 26),
  ('playlist',     'Playlist',                  'download', 28),
  ('music',        'Music',                     'download', 30),
  ('book',         'Book',                      'download', 32),
  ('comic',        'Comic',                     'download', 34),
  ('netflix',      'Netflix',                   'download', 36),
  ('training',     'Training / Course',         'download', 38),
  ('to_download',  'To Download (action tag)',  'download', 40),
  -- ---- convert class (format / web renditions) ----
  ('to_convert',   'To Convert / Web rendition','convert',  50),
  -- ---- analyze class (OCR / vision extraction) ----
  ('ocr',          'OCR / Transcription',       'analyze',  60),
  ('vision',       'Vision / QR / Screenshot',  'analyze',  62),
  -- ---- research class (deep research + docs) ----
  ('research',     'Research',                  'research', 70),
  ('technology',   'Technology',                'research', 72),
  ('code_repo',    'Code Repository (git)',     'research', 74),
  -- ---- reply class (status reply; run last) ----
  ('status_reply', 'Status Reply',              'reply',    90)
ON CONFLICT (code) DO NOTHING;

-- =============================================================================
-- 2. HASHTAG TAXONOMY  (§3.2.1 tags stored WITHOUT '#', case-insensitive citext)
--    Canonical tags plus common singular/plural variants that collapse to one row.
-- =============================================================================
INSERT INTO hashtags (tag) VALUES
  ('research'), ('technology'),
  ('video'), ('videos'),
  ('torrent'), ('magnet'),
  ('series'), ('serial'),
  ('movie'), ('movies'),
  ('documentary'),
  ('concert'), ('concerts'),
  ('game'), ('games'),
  ('software'),
  ('channel'),
  ('playlist'),
  ('music'),
  ('book'), ('books'),
  ('comic'), ('comics'),
  ('netflix'),
  ('training'),
  ('todownload'), ('toconvert'), ('todo'),
  ('github'), ('git'),
  ('qr'), ('screenshot')
ON CONFLICT (tag) DO NOTHING;

-- =============================================================================
-- 3. INDIRECT-DETERMINATION MAP  hashtag_categories  (final request §3.5)
--    A tag implies one or more content categories. Realises the worked examples:
--      torrent  -> Torrent + ToDownload
--      github   -> Code Repository + deep Research + Technology
--      comic    -> Comic + OCR transcription
--    Resolved by joining seeded tags to seeded categories (both keyed by natural code).
-- =============================================================================
INSERT INTO hashtag_categories (hashtag_id, category_id)
SELECT h.id, c.id
FROM (VALUES
    ('research',    'research'),
    ('technology',  'technology'), ('technology', 'research'),
    ('video',       'video'),      ('videos',     'video'),
    ('torrent',     'torrent'),    ('torrent',    'to_download'),
    ('magnet',      'torrent'),    ('magnet',     'to_download'),
    ('series',      'series'),     ('serial',     'series'),
    ('movie',       'movie'),      ('movies',     'movie'),
    ('documentary', 'documentary'),
    ('concert',     'concert'),    ('concerts',   'concert'),
    ('game',        'game'),       ('games',      'game'),
    ('software',    'software'),
    ('channel',     'channel'),
    ('playlist',    'playlist'),
    ('music',       'music'),
    ('book',        'book'),       ('books',      'book'),
    ('comic',       'comic'),      ('comic',      'ocr'),
    ('comics',      'comic'),      ('comics',     'ocr'),
    ('netflix',     'netflix'),
    ('training',    'training'),   ('training',   'research'),
    ('todownload',  'to_download'),
    ('toconvert',   'to_convert'),
    ('github',      'code_repo'),  ('github',     'research'), ('github', 'technology'),
    ('git',         'code_repo'),  ('git',        'research'),
    ('qr',          'vision'),
    ('screenshot',  'vision'),     ('screenshot', 'ocr')
  ) AS m(tag, code)
JOIN hashtags   h ON h.tag  = m.tag     -- citext = text: case-insensitive match
JOIN categories c ON c.code = m.code
ON CONFLICT DO NOTHING;

-- =============================================================================
-- 4. RBAC — SYSTEM ROLES + PERMISSIONS + GRANTS  (final request §6.1 three tiers)
--    System roles have account_id = NULL. Fixed UUIDs make the seed idempotent by PK
--    (independent of the roles UNIQUE(account_id,name) NULL-distinct quirk) and let the
--    role_permissions grants below reference roles without a name lookup.
-- =============================================================================
INSERT INTO roles (id, account_id, name, tier, description) VALUES
  ('11111111-1111-1111-1111-111111111111', NULL, 'root',          'root',
     'Tier 1 Root Admin: full system control over all accounts; only one exists.'),
  ('22222222-2222-2222-2222-222222222222', NULL, 'account_admin', 'account_admin',
     'Tier 2 Account Admin: full control of their own account and its users.'),
  ('33333333-3333-3333-3333-333333333333', NULL, 'user',          'user',
     'Tier 3 Standard User: consumer access to assigned accounts.')
ON CONFLICT (id) DO NOTHING;

-- Fine-grained capability codes (resource.action). Enforced by security/pkg/policy at
-- the API layer reading role_permissions (final request §6.3). [GAP: auth-7.2] the RBAC
-- tables back the User Service; JWT signing (RS256/EdDSA) is an API/security concern.
INSERT INTO permissions (code, description) VALUES
  ('system.admin',       'Cross-account system administration (Root only).'),
  ('account.create',     'Create a new account (becomes its admin).'),
  ('account.read',       'View account settings and branding.'),
  ('account.update',     'Edit account settings, branding, retention override.'),
  ('account.delete',     'Delete/suspend an account.'),
  ('user.invite',        'Invite a user to an account.'),
  ('user.read',          'View users within an account.'),
  ('user.update',        'Edit user roles/status within an account.'),
  ('user.remove',        'Revoke a user membership.'),
  ('role.manage',        'Create/edit account-scoped roles and permission bundles.'),
  ('channel.create',     'Register a channel/group/forum to read.'),
  ('channel.read',       'View channels and poll configuration.'),
  ('channel.update',     'Edit channel poll cadence / retention / active flag.'),
  ('channel.delete',     'Remove a channel.'),
  ('post.read',          'Read ingested posts, threads and replies.'),
  ('post.reprocess',     'Trigger explicit re-processing/refresh of a post.'),
  ('search.query',       'Run semantic / full-text search.'),
  ('asset.read',         'View asset metadata.'),
  ('asset.download',     'Download / stream asset content via the Asset Service.'),
  ('skill.read',         'View the Skill registry and run history.'),
  ('skill.manage',       'Enable/disable Skills and edit dispatch order.'),
  ('event.subscribe',    'Open WS/SSE/webhook event subscriptions.'),
  ('billing.view',       'View plans, subscription, usage and invoices.'),
  ('billing.manage',     'Change plan / manage subscription and payment.'),
  ('audit.read',         'Read the account audit log.')
ON CONFLICT (code) DO NOTHING;

-- Grant: ROOT gets EVERY permission (full system control, §6.1 tier 1).
INSERT INTO role_permissions (role_id, permission_id)
SELECT '11111111-1111-1111-1111-111111111111'::uuid, p.id
FROM permissions p
ON CONFLICT DO NOTHING;

-- Grant: ACCOUNT_ADMIN gets everything WITHIN their account (no system.admin,
-- no account.create/delete which are Root-scoped bootstrap/teardown, §6.1 tier 2).
INSERT INTO role_permissions (role_id, permission_id)
SELECT '22222222-2222-2222-2222-222222222222'::uuid, p.id
FROM permissions p
WHERE p.code IN (
  'account.read','account.update',
  'user.invite','user.read','user.update','user.remove','role.manage',
  'channel.create','channel.read','channel.update','channel.delete',
  'post.read','post.reprocess','search.query',
  'asset.read','asset.download',
  'skill.read','skill.manage',
  'event.subscribe',
  'billing.view','billing.manage',
  'audit.read'
)
ON CONFLICT DO NOTHING;

-- Grant: STANDARD USER gets consumer/read access only (§6.1 tier 3).
INSERT INTO role_permissions (role_id, permission_id)
SELECT '33333333-3333-3333-3333-333333333333'::uuid, p.id
FROM permissions p
WHERE p.code IN (
  'account.read',
  'channel.read',
  'post.read','post.reprocess','search.query',
  'asset.read','asset.download',
  'skill.read',
  'event.subscribe',
  'billing.view'
)
ON CONFLICT DO NOTHING;

-- =============================================================================
-- 5. DEFAULT SKILL REGISTRY  (final request §3.2.2 — one recipe per content type)
--    Mirror of the helix_skills Skill-Graph nodes Thready dispatches. sort_order encodes
--    "download-type Skills before analysis-type Skills" (§3.3): pre-classify < download <
--    convert < analyze < research < post-processing.
--    [GAP: helix_skills-4.1] helix_skills is a KNOWLEDGE DAG with no execution engine;
--    Thready's dispatch engine is BUILD-NEW and skill_runs is its execution ledger — a
--    seeded row here is a dispatchable NODE, NOT a claim that the engine is implemented.
-- =============================================================================
INSERT INTO skills (skill_key, name, version, kind, sort_order, is_enabled) VALUES
  -- umbrella entry point
  ('process.post',           'Post Processing Umbrella',       '1.0.0', 'umbrella',  5,  true),
  -- pre-processing (classification + thread assembly)
  ('classify.hashtags',      'Hashtag Classification',         '1.0.0', 'atomic',    8,  true),
  ('classify.ai-fallback',   'AI Classification Fallback',     '1.0.0', 'atomic',    9,  true),
  ('thread.assemble',        'Thread Context Assembly',        '1.0.0', 'atomic',    10, true),
  -- download-type recipes
  ('video.download',         'Video Download + Web Rendition', '1.0.0', 'composite', 20, true),
  ('torrent.fetch',          'Torrent / Magnet Fetch (Boba)',  '1.0.0', 'composite', 22, true),
  ('series.fetch',           'Series All-Seasons Fetch',       '1.0.0', 'composite', 24, true),
  ('movie.fetch',            'Movie Fetch',                    '1.0.0', 'composite', 26, true),
  ('concert.fetch',          'Concert Media Fetch',            '1.0.0', 'composite', 28, true),
  ('game.seek',              'Game Seek + Download',           '1.0.0', 'composite', 30, true),
  ('software.seek',          'Software Seek + Download',       '1.0.0', 'composite', 32, true),
  ('channel.download',       'Full Channel Download',          '1.0.0', 'composite', 34, true),
  ('playlist.download',      'Ordered Playlist Download',      '1.0.0', 'composite', 36, true),
  ('music.fetch',            'Music Fetch (MP3/FLAC/OPUS)',    '1.0.0', 'composite', 38, true),
  ('book.bibliography',      'Book Bibliography Download',     '1.0.0', 'composite', 40, true),
  ('netflix.find',           'Netflix Locate + Fetch',         '1.0.0', 'composite', 42, true),
  ('training.download',      'Course / Training Download',     '1.0.0', 'composite', 44, true),
  -- convert
  ('convert.web-rendition',  'Web Rendition Conversion',       '1.0.0', 'atomic',    50, true),
  -- analyze (OCR / vision)
  ('comic.ocr-transcribe',   'Comic OCR Full Transcription',   '1.0.0', 'composite', 55, true),
  ('asset.ocr',              'Asset OCR',                      '1.0.0', 'atomic',    56, true),
  ('vision.extract',         'Vision Meaning Extraction',      '1.0.0', 'atomic',    57, true),
  ('qr.decode',              'QR Decode + Metadata',           '1.0.0', 'atomic',    58, true),
  -- research
  ('research.deep',          'Multi-Pass Deep Web Research',   '1.0.0', 'composite', 70, true),
  ('technology.research',    'Technology Deep Research',       '1.0.0', 'composite', 72, true),
  ('coderepo.research',      'Code Repository Research',       '1.0.0', 'composite', 74, true),
  -- post-processing
  ('embed.index',            'Embed + Semantic Index',         '1.0.0', 'atomic',    80, true),
  ('artifact.generate',      'Generate Documentation/Book',    '1.0.0', 'atomic',    82, true),
  ('status.reply',           'Post Status Reply',              '1.0.0', 'atomic',    90, true)
ON CONFLICT (skill_key) DO NOTHING;

-- =============================================================================
-- 6. MESSENGER PROVIDERS  (final request §4.1 / §3.1)
--    `capabilities` advertises forum/reply-thread support + transport. Anti-bluff:
--    telegram is LIVE (herald gotd/td MTProto reader); max is a REGISTERED but STUB
--    provider (gap register §5, P0 — no Go client exists yet). `status` makes that
--    explicit so nothing treats the Max row as a working integration.
-- =============================================================================
INSERT INTO messengers (kind, display_name, capabilities) VALUES
  ('telegram', 'Telegram',
     '{"transport":"mtproto","library":"gotd/td","forum_topics":true,"reply_threads":true,"status":"live"}'::jsonb),
  ('max', 'Max',
     '{"transport":"bot+oneme","library":"build-new","forum_topics":false,"reply_threads":true,"status":"stub"}'::jsonb)
ON CONFLICT (kind) DO NOTHING;

COMMIT;

-- =============================================================================
-- 7. OPTIONAL — DEFAULT BILLING PLANS  [DEFAULT — adjustable]  (operator Q11)
--    Reference plan tiers so a demo/dev DB can exercise subscription + metered billing
--    from day one. Requires the `plans` table (canonical ../migrations/0004_billing.sql
--    OR this pack's ./migrations/0003_billing_metering.sql). Guarded so seed.sql still
--    runs cleanly if `plans` is absent (skips this block instead of erroring).
--    `limits` mirrors the Large-scale ceilings in erd.md §6 / Q2.
-- =============================================================================
DO $$
BEGIN
  IF to_regclass('public.plans') IS NOT NULL THEN
    INSERT INTO plans (code, display_name, limits, price_cents, currency, interval, is_active) VALUES
      ('free', 'Free',
         '{"channels":3,"posts_per_day":200,"seats":2,"assets_gb":5}'::jsonb,
         0,     'EUR', 'month', true),
      ('pro', 'Pro',
         '{"channels":25,"posts_per_day":2000,"seats":10,"assets_gb":500}'::jsonb,
         4900,  'EUR', 'month', true),
      ('scale', 'Scale',
         '{"channels":100,"posts_per_day":10000,"seats":100,"assets_gb":51200}'::jsonb,
         49900, 'EUR', 'month', true)
    ON CONFLICT (code) DO NOTHING;
  ELSE
    RAISE NOTICE 'skip: plans table absent — apply billing migration first to seed default plans';
  END IF;
END $$;

-- =============================================================================
-- Made with love ♥ by Helix Development.
-- =============================================================================
