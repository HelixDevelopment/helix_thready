<!--
  Title           : Helix Thready — Development & Orchestration (Area Index)
  Classification  : PUBLIC
  Location        : docs/public/research/mvp/development/index.md
  Status          : Review — v0.2
  Revision        : 2 (2026-07-21)
  Author          : Helix Thready documentation swarm (development)
  Related         : ./workable-items.md, ./agent-orchestration.md, ./submodule-map.md,
                    ./coding-standards.md, ./contribution-guidelines.md, ./build-new-subsystems.md,
                    ../index.md, ../CONVENTIONS.md
-->

# Helix Thready — Development & Orchestration (Area Index)

| Rev | Date | Author | Change |
|-----|------|--------|--------|
| 1 | 2026-07-21 | swarm (development) | Initial draft — index, dependency map, area file links |
| 2 | 2026-07-21 | swarm (development, review) | Review pass — tracked the pending `architecture/`+`database/` `index.md` cross-links as `ATM-072` |
| 3 | 2026-07-21 | orchestrator (integration) | Integration pass — `architecture/index.md` + `database/index.md` now present and the §3 cross-links resolve; `ATM-072` closed |

This is the canonical entry point for the **Development & Orchestration** area of the Helix
Thready MVP. It defines *how the system is built*: the granular workable-item backlog, the
agent-fleet orchestration model, the map of reused in-house submodules, the coding standards,
the contribution rules, and the design plans for the confirmed new subsystems.

All files here follow **[../CONVENTIONS.md](../CONVENTIONS.md)** exactly and never contradict
the authoritative sources of truth (see below). Provenance tags used throughout:
`[CONSTITUTION §x]`, `[IN-HOUSE: module]`, `[RESEARCH]`, `[OPERATOR]`,
`[DEFAULT — adjustable]`, `[BUILD-NEW]`, `[GAP: id]`, `[OPEN: …]`.

## Table of Contents

- [1. Authoritative sources (read-only)](#1-authoritative-sources-read-only)
- [2. Area files](#2-area-files)
- [3. Upstream / Downstream dependencies](#3-upstream--downstream-dependencies)
- [4. How this area maps onto the four phases](#4-how-this-area-maps-onto-the-four-phases)
- [5. Verified vs assumed — reading discipline](#5-verified-vs-assumed--reading-discipline)
- [6. Open items tracked in this area](#6-open-items-tracked-in-this-area)

## 1. Authoritative sources (read-only)

- **Final answered request** — `../../../../private/research/mvp/helix_thready_research_request_final.md`
  (the technology decision matrix §0.2, architecture §2, processing workflow §3, methodology §5,
  operator decisions §0.1, Q1–Q45 §18). **Primary source.**
- **Subsystem gaps & improvements** — `../../../../private/research/mvp/helix_thready_subsystem_gaps_and_improvements.md`
  (per-subsystem P0/P1/P2 gap register). Every gap relevant to this area is addressed by a design
  plan or a tracked `ATM-NNN` item tagged `[GAP: …]`.
- **Original request + Part II answers** — `../../../../private/research/mvp/helix_thready_research_request.md`.
- **Conventions** — [../CONVENTIONS.md](../CONVENTIONS.md).

## 2. Area files

| File | Purpose |
|------|---------|
| [workable-items.md](./workable-items.md) | The granular `ATM-NNN` backlog — every item decoupled, agent-implementable, mapped to the four phases (§5.1.2), with acceptance criteria, dependencies, test-type coverage and `[GAP: …]` tags |
| [agent-orchestration.md](./agent-orchestration.md) | The dev-fleet plan: native-alias-first `[§11.4.196/198]`, subagent-driven `[§11.4.20/70]`, automatic multi-track ruler `[§11.4.187]`, exactly-once claim registry `[§11.4.176]`, git-worktree isolation, Fable @ xhigh review `[§11.4.209]` |
| [submodule-map.md](./submodule-map.md) | Every reused `vasic-digital` / `HelixDevelopment` / `milos85vasic` submodule, its role, import path and **maturity** from the gap register (PRODUCTION / FOUNDATION / SCAFFOLD / DESIGN-ONLY / BUILD-NEW) |
| [coding-standards.md](./coding-standards.md) | Go 1.26 standards, TDD reproduce-first `[§11.4.43]`, design patterns, SOLID, decoupling `[§11.4.28]`, concurrency, error handling, anti-bluff |
| [contribution-guidelines.md](./contribution-guidelines.md) | commit-all wrapper + git-hook gates `[§11.4.75]`, all-upstreams push `[§2.1]`, project-prefixed tags `[§11.4.151]`, no server CI `[§11.4.156]`, workable-items DB discipline `[§11.4.93/95]` |
| [build-new-subsystems.md](./build-new-subsystems.md) | Scoped design plans for every `[BUILD-NEW]` gap: Download Manager, Max adapter, OCR adapter, User Service, Asset Service, standardized callback/task module (plus Event Bus service, ThreadReader, Semantic-search service) |

## 3. Upstream / Downstream dependencies

**Upstream (this area consumes):**

- **architecture** — the component boundaries, event model and concurrency model that the
  workable items decompose into decoupled tasks. See [../architecture/index.md](../architecture/index.md).
- **api** — the OpenAPI/Protobuf contracts the SDK-codegen and REST-skeleton items target. See
  [../api/index.md](../api/index.md).
- **database** — the schema/migration definitions the DB-wiring items implement. See
  [../database/index.md](../database/index.md).
- The **decision matrix** and **gap register** (authoritative sources §1) constrain every choice.

**Downstream (this area produces / feeds):**

- **testing** — every `ATM-NNN` names its mandated test types; the testing area expands them into
  the 15-type banks. See [../testing/index.md](../testing/index.md).
- **deployment** — the contribution + orchestration rules feed the release/deploy pipeline. See
  [../deployment/index.md](../deployment/index.md).
- **design** / **user-guides** — Phase-3 client items feed those areas. See
  [../design/index.md](../design/index.md), [../user-guides/index.md](../user-guides/index.md).

```mermaid
flowchart LR
  ARCH[architecture] --> DEV[development]
  API[api] --> DEV
  DB[database] --> DEV
  GAP[(gap register + decision matrix)] --> DEV
  DEV --> TST[testing]
  DEV --> DEP[deployment]
  DEV --> DES[design]
  DEV --> UG[user-guides]
  subgraph DEV_files[development area files]
    WI[workable-items] --- AO[agent-orchestration]
    SM[submodule-map] --- CS[coding-standards]
    CG[contribution-guidelines] --- BN[build-new-subsystems]
  end
  DEV --- DEV_files
```

**Explanation (for readers/models that cannot see the diagram).** The development area sits
downstream of the architecture, API and database areas plus the two authoritative planning
inputs (the decision matrix and the private gap register): those define *what* to build, while
this area defines *how* and *in what order*. The development area then feeds four downstream
areas — testing (which expands each item's declared test types into full banks), deployment
(which consumes the contribution and release rules), and design plus user-guides (which consume
the Phase-3 client-application items). Internally the area is six coupled files: the workable-item
backlog and the agent-orchestration plan are the operational core; the submodule map, coding
standards, contribution guidelines and build-new-subsystem design plans are the reference
material every item is implemented against.

> Rendered PNG/SVG exported via Docs Chain (§11.4.65). Source: [diagrams/dev-area-deps.mmd](./diagrams/dev-area-deps.mmd).

## 4. How this area maps onto the four phases

The final request §5.1.2 defines four sequential phases. The workable-item backlog is organized
by phase and sub-phase; the orchestration model runs multiple phases' non-contending items in
parallel tracks where dependencies allow.

```mermaid
flowchart TB
  subgraph P1[Phase 1 — Foundation]
    P11[1.1 Infrastructure]
    P12[1.2 Core Services]
    P13[1.3 Integration]
  end
  subgraph P2[Phase 2 — Processing Engine]
    P21[2.1 Acquisition]
    P22[2.2 Pipeline]
    P23[2.3 Skills]
    P24[2.4 Semantic Search]
  end
  subgraph P3[Phase 3 — Client Applications]
    P31[3.1 Web]
    P32[3.2 Desktop]
    P33[3.3 Mobile]
    P34[3.4 CLI/TUI]
    P35[3.5 Design System]
  end
  subgraph P4[Phase 4 — Testing & Deployment]
    P41[4.1–4.5 Test types]
    P46[4.6 Production Deploy]
  end
  P1 --> P2 --> P3 --> P4
  XC[Cross-cutting: anti-bluff, decoupling audit, SDK codegen] -.-> P1 & P2 & P3 & P4
```

**Explanation (for readers/models that cannot see the diagram).** Phase 1 (Foundation) stands up
infrastructure, the core services (User Service, Event Bus service, Asset Service), and the
messenger/database/API integration. Phase 2 (Processing Engine) builds acquisition, the
processing pipeline, Skills dispatch and semantic search on top of that foundation. Phase 3
(Client Applications) delivers the Web portal, Desktop, Mobile, CLI/TUI and design-system
surfaces. Phase 4 (Testing & Deployment) runs the mandated test types and the production
deployment. A cross-cutting band of items — the anti-bluff sweep, the decoupling audit and SDK
codegen — spans every phase because those disciplines apply continuously rather than at one
milestone. The phases are drawn sequentially, but the orchestration model (see
[agent-orchestration.md](./agent-orchestration.md)) runs non-contending items from different
phases concurrently across isolated git-worktree tracks whenever their dependencies are already
satisfied.

> Rendered PNG/SVG exported via Docs Chain (§11.4.65). Source: [diagrams/dev-phase-map.mmd](./diagrams/dev-phase-map.mmd).

## 5. Verified vs assumed — reading discipline

Per CONVENTIONS §7 and the gap register's anti-bluff caveat, this area distinguishes:

- **VERIFIED** — read at source (the decision matrix, the gap register marked `VERIFIED`, or a
  module inspected in the local clones under `/home/milos/Factory/projects/tools_and_research/`).
- **ASSUMPTION / `[DEFAULT — adjustable]`** — a proposed engineering default the operator may
  override; never presented as a settled fact.
- A module the gap register flagged `SCAFFOLD` / `DESIGN-ONLY` / `BUILD-NEW` is **never** described
  as "working"; the relevant `[GAP: …]` and the plan to close it are cited instead.

## 6. Open items tracked in this area

Items that could not be fully resolved from the sources available are marked `[OPEN: …]` and
carried as tracked workable items. The consolidated list lives in
[workable-items.md §8 (Open items register)](./workable-items.md#8-open-items-register); the
headline open items are:

- `[OPEN: constitution-anchor-verify]` — the local Constitution submodule copy tops out at
  §11.4.192; the exact normative text of **§11.4.196 / §11.4.198 / §11.4.209** was taken from the
  final request's descriptions, not read at source. Re-verify against the canonical constitution
  before relying on the precise wording (tracked as `ATM-066`).
- `[OPEN: max-oneme-go-port]` — the Max OneMe user-WebSocket reference implementations are Python;
  a Go port is unproven and needs a research spike (tracked as `ATM-018` / build-new plan).
- `[OPEN: agentpool-contract]` — `digital.vasic.llmorchestrator`'s `AgentPool` capability-matching
  contract is not locally cloned; re-verify at source (tracked as `ATM-067`).
- ~~`[OPEN: sibling-area-index-missing]`~~ **CLOSED (integration pass, 2026-07-21).** The upstream
  cross-links in §3 to `../architecture/index.md` and `../database/index.md` now resolve: both areas
  publish their canonical `index.md` `[§11.4.212]`, matching every other sibling area. The integration
  orchestrator verified all cross-area references across the eight areas resolve to real files
  (`ATM-072` closed — see [../INTEGRATION_REPORT.md](../INTEGRATION_REPORT.md)).

---

*Made with love ♥ by Helix Development.*
