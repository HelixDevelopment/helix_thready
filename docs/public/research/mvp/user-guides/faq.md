<!--
  Title           : Helix Thready — Frequently Asked Questions
  Classification  : PUBLIC
  Location        : docs/public/research/mvp/user-guides/faq.md
  Status          : Draft — v0.1 (zero-version)
  Revision        : 1 (2026-07-21)
  Author          : Helix Thready documentation swarm (user-guides)
  Related         : ./index.md, ./configuration.md, ./end-user-manual.md, ./troubleshooting.md
-->

# Helix Thready — Frequently Asked Questions

| Rev | Date | Author | Change |
|-----|------|--------|--------|
| 1 | 2026-07-21 | swarm (user-guides) | Initial FAQ |

Short answers with links to the authoritative guide. Answers are grounded in the technology decision
matrix and the [gap register](../../../../private/research/mvp/helix_thready_subsystem_gaps_and_improvements.md);
where a feature depends on a scaffold/`[BUILD-NEW]` module, the answer says so.

## Table of contents

1. [General](#1-general)
2. [Install & configuration](#2-install--configuration)
3. [Messengers & channels](#3-messengers--channels)
4. [Processing & categories](#4-processing--categories)
5. [Search](#5-search)
6. [Assets & downloads](#6-assets--downloads)
7. [Accounts, roles & security](#7-accounts-roles--security)
8. [Clients & SDKs](#8-clients--sdks)
9. [Data, backup & compliance](#9-data-backup--compliance)

## 1. General

**Q: What is Helix Thready?**
It connects to messaging platforms (Telegram, then Max, then more) with your private accounts, reads
whole threads, processes each post through AI recipes (Skills) selected by hashtags/content type,
generates research and downloadable assets, and makes everything searchable by meaning. See
[index.md](./index.md) and [end-user-manual.md](./end-user-manual.md).

**Q: Is this the shipped product?**
No — this is the **zero version** documentation, written before implementation. It documents the
system as decided in the decision matrix. Where a subsystem is a scaffold/stub, the guides say so.

**Q: Who makes it?** Helix Development. The footer slogan *"Made with love ♥ by Helix Development"*
appears on every surface and generated document, and persists even under per-account white-labeling.

## 2. Install & configuration

**Q: What do I need to run it?** Rootless Podman, Go 1.26.x (to build), PostgreSQL 16+ with pgvector,
and a llama.cpp/HelixLLM endpoint. See [installation.md §1](./installation.md#1-prerequisites).

**Q: Where do settings live?** In a gitignored `.env`, with values also readable from
`~/.bashrc`/`~/.zshrc` and `$HOME/api_keys.sh`; missing sources are skipped silently, never logged.
The full list is [configuration.md](./configuration.md).

**Q: I set a var in `.env` but nothing changed.** A shell `export` overrides `.env` (process-env
wins). Also, **secrets are not hot-reloaded** — restart after rotating them.
[troubleshooting.md §9](./troubleshooting.md#9-configuration-changes-not-taking-effect).

**Q: Docker?** No — **rootless Podman only** `[CONSTITUTION §11.4.76]`.

## 3. Messengers & channels

**Q: Which messengers work today?** **Telegram** (via the `gotd/td` MTProto user client, being
promoted from Herald's `qaherald` harness). **Max is not available yet** — the adapter is `[BUILD-NEW]`
`[GAP: 3]`. Plan around Telegram for the zero version.

**Q: Interactive or headless sign-in?** Both. Interactive prompts for the login code/2FA; headless
(`THREADY_MESSENGER_SIGNIN_MODE=noninteractive`) reads all `HERALD_TELEGRAM_*` from the environment.
[installation.md §7](./installation.md#7-messenger-sign-in--first-channel).

**Q: Does it read replies too?** Yes — Thready assembles the **complete post** = root + the full chain
of organic human replies (excluding the system's own replies). Hashtags added in a reply still count.
[end-user-manual.md §3](./end-user-manual.md#3-complete-posts-root--reply-chain).

**Q: Can I just add a channel and let it figure out the content?** Yes — auto-recognition detects the
thread's content type and how to process it; you can override per channel.

## 4. Processing & categories

**Q: How does it decide what to do with a post?** By its hashtags + content type, mapped to Skills.
Missing/weak tags trigger indirect determination (GitHub→research, torrent→download, …); still
unclassified → generic ingest + manual-review queue, never dropped.
[end-user-manual.md §5](./end-user-manual.md#5-hashtag-categories).

**Q: A post has several hashtags — which wins?** None — they're **additive**. All matching recipes run,
ordered `download > convert > analyze > research > reply`. A `#Research #Video` post both downloads the
video and does deep research.

**Q: Will the same post be processed twice under an event storm?** No. Each post is claimed exactly
once via a Postgres lock (single-claim idempotency) `[CONSTITUTION §11.4.176]`.

**Q: Can I re-process a post?** Yes — `thready post reprocess <id>` (full refresh) or
`thready post retry <id> --step <step>` (one step). Client → REST → System.

**Q: What do the status replies contain?** Success/failure, metrics, and asset references, posted from
the Robot or User account (`THREADY_REPLY_ACCOUNT`).

## 5. Search

**Q: How does search work?** Semantic (by meaning) over both original posts and generated materials,
via pgvector + a llama.cpp embedding model — the in-house "Lumen-style" capability. Target < 500 ms.
[end-user-manual.md §6](./end-user-manual.md#6-semantic-search).

**Q: My search results are irrelevant.** Almost always the HelixLLM **default hash embedder** `[GAP: 1]`.
Set `HELIX_EMBEDDING_PROVIDER=llama`, ensure `THREADY_EMBEDDING_DIM` matches the model, reindex, and
restart. [troubleshooting.md §5](./troubleshooting.md#5-semantic-search-returns-irrelevant-results).

**Q: Which vector DB?** pgvector (cosine), co-located in Postgres. Qdrant/Pinecone/Milvus sit behind
the same interface but are **unverified** `[GAP: 8]` — stay on pgvector.

## 6. Assets & downloads

**Q: Are download links direct file paths?** No — every asset link resolves through the **Asset
Service** (built on Catalogizer) with auth/RBAC. Raw original + a web-optimized `-web` rendition are
kept. [end-user-manual.md §7](./end-user-manual.md#7-media-assets-and-downloads).

**Q: A download never finishes.** Depends on the path: MeTube is **poll-only** `[GAP: 5]` (finishes but
notifies slowly), the generic Download Manager for direct URLs **doesn't exist yet** `[GAP: 4]`, and
Boba callbacks are being standardized. [troubleshooting.md §6](./troubleshooting.md#6-a-download-never-completes).

**Q: Comic/screenshot text isn't extracted.** VisionEngine has **no OCR engine** `[GAP: 2]`; it falls
back to LLM-vision until the Tesseract/PaddleOCR adapter `[BUILD-NEW]` lands.

**Q: A physical file link broke.** `thready asset reheal <id>` re-downloads it.

## 7. Accounts, roles & security

**Q: What are the roles?** Root Admin (owns everything, exactly one), Account Admin (owns one account),
Standard User (consumer). [root-admin-guide.md](./root-admin-guide.md),
[account-admin-guide.md](./account-admin-guide.md).

**Q: Can one person be in multiple accounts?** Yes — with different roles per account; a user can
create their own account and become its admin.

**Q: Is MFA required?** TOTP is **mandatory for Root and Account Admins**, optional for Standard Users
(`THREADY_MFA_REQUIRED_TIERS`).

**Q: How are passwords/tokens protected?** Argon2id password hashing (≥12, breach-checked); AES-256-GCM
at rest; TLS 1.3 in transit. JWT is HS256 today, moving to RS256/EdDSA for multi-service `[GAP: 10]`.

**Q: Can mobile clients be used with real accounts now?** **No** — `Security-KMP` mobile secure storage
is an **in-memory stub** `[GAP: 7]`; mobile is evaluation-only until Android Keystore / iOS Keychain
are implemented. [mobile-guide.md §3](./mobile-guide.md#3-security-status-read-before-you-ship).

## 8. Clients & SDKs

**Q: Which clients exist?** Web (primary), CLI, TUI, Desktop (Tauri), Mobile (native + KMP). Web + CLI
are first priority. All use the same REST `/v1` + event bus.

**Q: Is the CLI as capable as the Web?** Yes — "everything possible from the Web works from the CLI/TUI"
(headless, pipeline-friendly). [cli-reference.md](./cli-reference.md).

**Q: Which SDK language?** **Go is primary**; TS/Python/Java-Kotlin are High priority; others follow.
Generated from OpenAPI 3.1 + Protobuf. [sdk-quickstart.md](./sdk-quickstart.md).

**Q: Which languages is the UI available in?** The UI is fully localized in **English (`en`), Russian
(`ru`), and Serbian Cyrillic (`sr-Cyrl`)** via HelixTranslate + `digital.vasic.i18n` (Q35). Set the
default with `THREADY_DEFAULT_LOCALE`; users can switch in-app, with an explicit choice plus system
default. **Post content** is stored in its **original** language (not auto-translated) with on-demand
translation available; generated research is English-primary with on-demand translation.
[configuration.md §15](./configuration.md#15-retention-billing--localization).

## 9. Data, backup & compliance

**Q: How long is data kept?** Indefinitely by default, with per-account overrides (accounts may
shorten). [root-admin-guide.md §6](./root-admin-guide.md#6-global-retention--data-policy).

**Q: What's the backup policy?** Daily full + hourly DB incrementals; assets daily snapshot; RPO ≈ 1 h,
RTO ≈ 4 h with a documented restore runbook.
[root-admin-guide.md §10](./root-admin-guide.md#10-backup--disaster-recovery-runbook).

**Q: Is it GDPR-certified?** No — compliance is **minimal/internal** for the MVP, but the design is
GDPR-aware (erasure/export hooks exist). Certification is deferred `[OPEN]`.

**Q: Where are the logs / how do I monitor it?** logrus + ClickHouse (logs), OpenTelemetry (traces),
Prometheus + Grafana (metrics) — the in-house stack, **not** ELK/Loki/Datadog.
[troubleshooting.md §10](./troubleshooting.md#10-where-the-logs-are).

---

*Made with love ♥ by Helix Development.*
