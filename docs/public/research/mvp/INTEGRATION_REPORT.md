<!--
  Title           : Helix Thready — MVP Documentation Integration Report
  Classification  : PUBLIC
  Location        : docs/public/research/mvp/INTEGRATION_REPORT.md
  Status          : Active — v1.0
  Revision        : 1 (2026-07-21)
  Author          : Helix Thready documentation swarm (integration orchestrator)
  Related         : ./index.md, ./CONVENTIONS.md,
                    ./architecture/index.md, ./api/index.md, ./database/index.md,
                    ./deployment/index.md, ./development/index.md, ./testing/index.md,
                    ./design/index.md, ./user-guides/index.md
-->

# Helix Thready — MVP Documentation Integration Report

| Rev | Date | Author | Change |
|-----|------|--------|--------|
| 1 | 2026-07-21 | orchestrator (integration) | Initial integration report — tree, per-area file counts, cross-reference verification, open-item & gap register roll-up |

This report records the **integration pass** over the eight MVP documentation areas under
`docs/public/research/mvp/`. It verifies that every cross-area reference resolves, that each area
index carries an Upstream/Downstream note consistent with the [master dependency map](./index.md#cross-area-dependency-map),
and it rolls up the remaining `[OPEN: …]` items and gap-register coverage across all areas. All
content follows [CONVENTIONS.md](./CONVENTIONS.md).

## Table of contents

1. [Summary](#1-summary)
2. [Documentation tree](#2-documentation-tree)
3. [Per-area file counts](#3-per-area-file-counts)
4. [Cross-references resolved](#4-cross-references-resolved)
5. [Upstream/Downstream reconciliation with the master map](#5-upstreamdownstream-reconciliation-with-the-master-map)
6. [Remaining open items across all areas](#6-remaining-open-items-across-all-areas)
7. [Gap-register coverage](#7-gap-register-coverage)
8. [Changes applied in this pass](#8-changes-applied-in-this-pass)

---

## 1. Summary

- **8 areas**, **74 Markdown docs**, **84 Mermaid `.mmd` diagram sources**, and **4 machine
  artifacts** (`api/openapi.yaml`, `database/schema-relational.sql`, `database/schema-vector.sql`,
  `database/migrations/0001_init.sql`) — **162 files** total (incl. the root `index.md`,
  `CONVENTIONS.md`, and this report is the 163rd once written).
- **Cross-references: PASS.** 736 file-targeted Markdown links checked; **0 broken**. All 110
  cross-parent links resolve to real files (the lone regex hit was an inline-code *example* of the
  link syntax in `CONVENTIONS.md §5`, not a rendered link).
- **Status tracker updated** in [index.md](./index.md): Pass 1 (draft) + Pass 2 (review) +
  Integrated marked done for all eight areas; Pass 3 (optional polish) left open.
- **Every area index carries an Upstream/Downstream note** and is directionally consistent with the
  master dependency map (see [§5](#5-upstreamdownstream-reconciliation-with-the-master-map)).
- **132 unique `[OPEN: …]` markers** and **82 `ATM-*` workable-item ids** are tracked; **178
  `[GAP: …]` tags** address the P0/P1/P2 register — none papered over.

## 2. Documentation tree

```
docs/public/research/mvp/
├── index.md                     (master index & roadmap — status tracker)
├── CONVENTIONS.md               (documentation conventions)
├── INTEGRATION_REPORT.md        (this file)
├── architecture/
│   ├── index.md
│   ├── system-overview.md · component-catalog.md · data-flow.md · event-model.md
│   ├── concurrency-and-idempotency.md · service-discovery.md · security-model.md
│   ├── messenger-ingestion.md · processing-pipeline.md · semantic-search.md · asset-and-download.md
│   └── diagrams/                (15 .mmd)
├── api/
│   ├── index.md
│   ├── openapi.yaml             (OpenAPI 3.1 — the REST /v1 contract)
│   ├── rest-endpoints.md · event-bus-contract.md · authn-authz.md · error-model.md
│   ├── versioning.md · sdk-strategy.md · contract-tests.md
│   └── diagrams/                (7 .mmd)
├── database/
│   ├── index.md · erd.md · indexing.md · partitioning.md · retention-archive.md · migration-strategy.md
│   ├── schema-relational.sql · schema-vector.sql
│   ├── migrations/0001_init.sql
│   └── diagrams/                (10 .mmd)
├── deployment/
│   ├── index.md
│   ├── container-topology.md · podman-compose.md · environments.md · tls-lets-encrypt.md
│   ├── deploy-and-rollback.md · backup-dr.md · service-discovery-ports.md
│   ├── hetzner-provisioning.md · secrets-and-config.md
│   └── diagrams/                (10 .mmd)
├── development/
│   ├── index.md · workable-items.md · agent-orchestration.md · submodule-map.md
│   ├── coding-standards.md · contribution-guidelines.md · build-new-subsystems.md
│   └── diagrams/                (9 .mmd)
├── testing/
│   ├── index.md · test-strategy.md · test-types.md · tdd-skeletons.md · helixqa-banks.md
│   ├── challenges-scenarios.md · static-analysis.md · performance-and-chaos.md
│   └── diagrams/                (8 .mmd)
├── design/
│   ├── index.md · design-system.md · brand-assets.md · theming.md · wireframes.md
│   ├── ux-flows.md · component-library.md · prototypes.md
│   └── diagrams/                (13 .mmd)
└── user-guides/
    ├── index.md · installation.md · configuration.md · root-admin-guide.md · account-admin-guide.md
    ├── end-user-manual.md · cli-reference.md · tui-usage.md · web-portal-guide.md · mobile-guide.md
    ├── sdk-quickstart.md · faq.md · troubleshooting.md
    └── diagrams/                (12 .mmd)
```

## 3. Per-area file counts

| Area | Markdown | `.mmd` diagrams | Other | Area total |
|------|:---:|:---:|:---:|:---:|
| architecture | 12 | 15 | 0 | 27 |
| api | 8 | 7 | 1 (`openapi.yaml`) | 16 |
| database | 6 | 10 | 3 (2× `.sql` + 1 migration) | 19 |
| deployment | 10 | 10 | 0 | 20 |
| development | 7 | 9 | 0 | 16 |
| testing | 8 | 8 | 0 | 16 |
| design | 8 | 13 | 0 | 21 |
| user-guides | 13 | 12 | 0 | 25 |
| **root** (`index.md`, `CONVENTIONS.md`) | 2 | 0 | 0 | 2 |
| **Total** | **74** | **84** | **4** | **162** |

Every embedded Mermaid diagram has a sibling `.mmd` source under the area's `diagrams/` folder per
[CONVENTIONS.md §4](./CONVENTIONS.md); the `.mmd` count (84) exceeds the doc count because several
docs embed multiple diagrams.

## 4. Cross-references resolved

A full link sweep parsed every Markdown link in all 74 docs and resolved the file portion of each
target relative to its source file.

| Metric | Result |
|--------|--------|
| File-targeted Markdown links checked | 736 |
| Cross-parent (`../…`) links | 110 |
| **Broken links** | **0** |
| Cross-area file-specific links (non-`index.md`) | `design/theming.md → ../database/erd.md` (×2) — resolves |

- Every area `index.md` links to its own files **and** to the sibling area indexes it depends on
  ([CONVENTIONS.md §5](./CONVENTIONS.md)); all eight sibling `index.md` targets exist and resolve.
- The private authoritative sources are referenced with correct relative depth: `../../../private/…`
  from the root `index.md`/`CONVENTIONS.md`, and `../../../../private/…` from the area
  subdirectories (both verified against the `docs/private/research/mvp/` location).
- The single link-checker hit (`CONVENTIONS.md → ../architecture/event-model.md`) is an inline-code
  **example** of the relative-link convention (wrapped in backticks, never rendered), not an actual
  reference. No fix required.

## 5. Upstream/Downstream reconciliation with the master map

The [master dependency map](./index.md#cross-area-dependency-map) draws these contract edges
(A → B ≡ "A feeds B", A upstream): `architecture → {api, database, deployment, testing}`,
`database → {api, testing}`, `api → {user-guides, design, testing}`, `design → user-guides`, and
`development → (underpins every area)`.

All eight area indexes carry an explicit **Upstream/Downstream** note, and every one is directionally
consistent with the map:

| Area | Upstream note lists | Downstream note lists | Consistent with master map |
|------|---------------------|-----------------------|:---:|
| architecture | authoritative sources, in-house engines (root of the DAG) | api, database, deployment, testing | ✔ |
| api | architecture, database, in-house modules | design, user-guides, testing | ✔ |
| database | architecture, research, in-house modules | api, deployment, testing, development | ✔ |
| deployment | architecture, database, api | development, testing | ✔ |
| development | architecture, api, database, decision-matrix + gap register | testing, deployment, design, user-guides | ✔ (process/underpinning axis) |
| testing | architecture, api, database | deployment, development | ✔ |
| design | architecture, api, `design_system`/OpenDesign, sources | user-guides, testing, impl repos | ✔ |
| user-guides | architecture, api, database, deployment, design | testing, docs-chain, operators | ✔ |

**Note on `development`.** The master map renders `development → (all)` because development is the
cross-cutting **process/underpinning** area — it defines the workable items and standards that
*produce* each section. The development index additionally lists architecture/api/database as
*upstream* because it **consumes their contracts** to author those items. These are two axes
(process vs. contract) of the same relationship, not a contradiction; the map's own prose calls
development the layer that "underpins everything." Its bidirectional edges with testing and
deployment (dev feeds the build backlog; those areas feed requirements back) are intentional and
documented in each index.

## 6. Remaining open items across all areas

**132 unique `[OPEN: …]` markers** remain tracked (each tied to an `ATM-*`/plan, none papered over).
The headline register per area index:

| Area | Index-level open items (headline) |
|------|-----------------------------------|
| architecture | `OVERVIEW-1/2`, `CAT-1/2`, `EVT-1/2`, `CONC-1/2`, `DISC-1/2`, `SEC-1/2/3`, `ING-1/2/3`, `PROC-1/2/3/4`, `SEM-1/2/3`, `ASSET-1/2/3`, `DF-1/2/3` (source-verification of FLAGGED interfaces) |
| api | `api-1` (proto files produced with code), `api-2` (rate-limit tiers pending billing), `api-3` (Zig hand-written SDK) |
| database | `ATM-DB-001/002/004/011/012/013/021/031/032/033` (pg placeholders, CONCURRENTLY, partition FK, multi-lang FTS, pgvector-vs-Qdrant benchmark, vector tenant isolation, GDPR cold erasure, MinIO signed-URL parity, sub-partitioning) |
| deployment | `dns-provider`, `host-sizing`, `secondary-store`, `buildnew-images` |
| development | `constitution-anchor-verify` (`ATM-066`), `max-oneme-go-port` (`ATM-018`), `agentpool-contract` (`ATM-067`); **`sibling-area-index-missing` (`ATM-072`) — CLOSED this pass** |
| testing | `canonical-helixqa-repo`, `helixstream-scope`, `docs-chain-tooling`, `mobile-device-farm` |
| design | `THREADY-DES-01` (Logo eyedrop), `THREADY-DES-02` (PenPot/Lottie bridge), `THREADY-DES-03` (heart glyph color), `THREADY-DES-14` (branding storage+contract reconciliation with database/api) |
| user-guides | per-guide open items + the `docs_chain` honest-SKIP note (`[GAP: 19]`); endpoint/entity names track the api/database areas |

Cross-area open items worth tracking to closure at the program level:

- **`THREADY-DES-14`** (design ↔ database ↔ api): the `account_branding` normalized table vs the
  canonical `accounts.branding` JSONB, and the expanded `setBranding` contract vs `api/openapi.yaml`
  — must converge on the canonical schema/contract; design MUST NOT diverge.
- **`ATM-DB-033`** (database ↔ deployment): MinIO signed-URL parity — a storage/deployment concern
  the schema defers to via an opaque `storage_key`.
- **`docs-chain-tooling`** (testing ↔ deployment ↔ user-guides): `pandoc`/`weasyprint` provisioning
  gates md→HTML/PDF/PNG sibling generation; the Markdown remains the source of truth meanwhile.

## 7. Gap-register coverage

**178 `[GAP: …]` tags** across the tree map the P0/P1/P2 items from
`helix_thready_subsystem_gaps_and_improvements.md` onto owning documents. The P0 traps are each
owned by a design plan or `[BUILD-NEW]` item — never claimed to "work":

| P0 gap | Owning area(s) |
|--------|----------------|
| `#1 / 2.1` HelixLLM `HashEmbedder` non-semantic default | architecture (semantic-search), api, database, deployment, user-guides |
| `#2 / 2.6` VisionEngine has no OCR engine | architecture (processing-pipeline), user-guides |
| `#6 / 4.1` helix_skills has no execution engine | architecture (processing-pipeline), api, database, user-guides |
| `#3 / 5.1` Herald Telegram in QA harness; Max empty stub | architecture (messenger-ingestion), api, user-guides |
| `6.2 / #4` filesystem: no HTTP source / download semantics | architecture (asset-and-download), user-guides |
| `6.3` Download Manager does not exist | architecture (asset-and-download), development (build-new), user-guides |
| `6.5 / #5` MeTube poll-only, no outbound webhook | architecture (asset-and-download), api, user-guides |
| `7.3` Security-KMP mobile secure storage is a stub | architecture (security-model), design, user-guides |

P1/P2 items (JWT asymmetric keys, partitioning helpers, LLMsVerifier port `:7061` vs `:8080`, Boba
callback contract, standardized callback/task module, searchable-sealed credentials, VisualRegression
CI, docs_chain pandoc/weasyprint) are each addressed in the owning area's gap table with a
`[GAP: …]` tag and a tracked plan. The BUILD-NEW subsystem set (Asset/Download/User/Max/OCR/webhook/
callback/EventBus service/ThreadReader/Semantic-search) is declared as placeholders in
`deployment/container-topology.md` and designed in `development/build-new-subsystems.md`.

## 8. Changes applied in this pass

1. **[index.md](./index.md)** — status tracker: Pass 1, Pass 2 and Integrated marked done (☑) for
   all eight areas; added a rev-2 row and a link to this report.
2. **[development/index.md](./development/index.md)** — closed `[OPEN: sibling-area-index-missing]`
   / `ATM-072`: `architecture/index.md` and `database/index.md` are present and the §3 cross-links
   resolve; added a rev-3 row.
3. **[user-guides/index.md](./user-guides/index.md)** — updated the §4 cross-area caveat: all
   upstream sibling indexes are committed and every link resolves; added a rev-2 row.
4. **INTEGRATION_REPORT.md** — this file (new).

No broken links required fixing; no Upstream/Downstream note required a directional correction. The
edits above reconcile the two now-stale "sibling index not yet committed / link may 404" notices
that predated area integration.

---

*Made with love ♥ by Helix Development.*
