<!--
  Title           : Helix Thready — Documentation Conventions
  Classification  : PUBLIC
  Location        : docs/public/research/mvp/CONVENTIONS.md
  Status          : Active — v1.0
  Revision        : 1 (2026-07-21)
  Author          : Helix Thready documentation swarm (orchestrated)
  Related         : ./index.md
-->

# Helix Thready — Documentation Conventions

**Every** Markdown file under `docs/public/research/mvp/` and `docs/private/research/mvp/`
MUST follow these conventions. They implement the Helix Constitution documentation rules
(`§11.4.65` md→HTML/PDF sync, `§11.4.61` metadata+ToC, `§11.4.44` revision headers,
`§11.4.212` README/index canonical entry) and the original request's diagram/branding rules.

## 1. Source of truth (do not re-decide)

The authoritative inputs — treat as read-only sources of truth, never contradict:

1. `docs/private/research/mvp/helix_thready_research_request_final.md` — the merged, answered
   request: the **technology decision matrix**, architecture, operator decisions, and Q1–Q45
   answers. **This is the primary source.**
2. `docs/private/research/mvp/helix_thready_research_request.md` — original request + Part II answers.
3. `docs/private/research/mvp/helix_thready_subsystem_gaps_and_improvements.md` — the per-subsystem
   gap register (P0/P1/P2). Every subsystem gap MUST be addressed by a design plan or an explicit
   tracked workable item in the relevant section.

**In-house first:** reuse the existing `vasic-digital` / `HelixDevelopment` / `milos85vasic`
submodules named in the decision matrix. Do not invent external tech where an in-house module
exists. When you need a module's real interface, read its source via `gh` (e.g.
`gh repo view vasic-digital/<repo>`), the local clones under
`/home/milos/Factory/projects/tools_and_research/`, or CodeGraph — do not guess.

## 2. File header (mandatory, top of every file)

```
<!--
  Title           : <human title>
  Classification  : PUBLIC | CLASSIFIED and CONFIDENTIAL
  Location        : <repo-relative path>
  Status          : Draft | Review | Active — v<major.minor>
  Revision        : <n> (YYYY-MM-DD)
  Author          : Helix Thready documentation swarm (<area>)
  Related         : <relative links to related docs>
-->
```

Immediately below the `# H1` title, include a **revision-history** table:

| Rev | Date | Author | Change |
|-----|------|--------|--------|
| 1 | 2026-07-21 | swarm (<area>) | Initial draft |

Then a **Table of Contents** (for files longer than ~2 screens).

## 3. Provenance tags (use inline, as in the source docs)

`[CONSTITUTION §x]` authoritative · `[IN-HOUSE: module]` reuse existing submodule ·
`[RESEARCH]` web/source research · `[OPERATOR]` operator decision ·
`[DEFAULT — adjustable]` proposed default · `[BUILD-NEW]` confirmed new-submodule gap ·
`[GAP: id]` addresses a gap-register item.

## 4. Diagrams (mandatory rule)

- Author diagrams as **Mermaid** in fenced ```mermaid blocks embedded in the Markdown.
- **Every diagram MUST be immediately followed by a multi-paragraph Markdown explanation**
  ("Explanation (for readers/models that cannot see the diagram): …") describing every node,
  edge, and flow in prose. A diagram without a prose explanation is incomplete.
- Also save the Mermaid source as a sibling `.mmd` file next to the doc (e.g.
  `diagrams/<name>.mmd`) so Docs Chain / a renderer can export PNG/SVG later. Note in the doc:
  `> Rendered PNG/SVG exported via Docs Chain (§11.4.65).`

## 5. Cross-references

Use **relative links** between docs: `[Event model](../architecture/event-model.md)`. Every
section's `index.md` links to its files AND to the sibling area indexes it depends on. Keep an
"Upstream/Downstream dependencies" note at the top of each area index.

## 6. Code & schema examples

- Include **code-level examples** (Go / TypeScript / SQL) illustrating key interfaces,
  algorithms, and contracts — real snippets or clearly-labelled pseudo-code.
- **Database:** schemas as PostgreSQL **DDL** (`CREATE TABLE …`), pgvector DDL for vectors,
  indexes, partitioning, and forward/rollback **migration** scripts.
- **API:** endpoints as **OpenAPI 3.1** (YAML), plus the WebSocket/SSE event contract.
- **Testing:** TDD **reproduce-first** skeletons (RED test first), covering the **15 mandated
  test types** `[CONSTITUTION §11.4.27]`.

## 7. Quality bar (no bluff)

- No skinny docs, no gaps, no danger zones left unaddressed. Where something is unresolved,
  mark it `[OPEN: …]` and add a tracked workable item — never paper over it.
- Distinguish **verified** facts from **assumptions**; do not claim a module "works" if the gap
  register flagged it a scaffold/stub — reference the gap and the design plan to close it.
- Enterprise-grade, implementation-ready, self-consistent with the decision matrix.

## 8. Naming, versioning, repos `[CONSTITUTION]`

Project-prefixed release tags `<PREFIX>-<ver>` (§11.4.151); commit + push to **all four
upstreams** (GitHub/GitLab/GitFlic/GitVerse, §2.1); no server-side CI (§11.4.156); decoupled
submodules (§11.4.28). Public docs live in the **main repo** (`docs/public/…`); sensitive
materials (credentials, access, gap register) live in the **private submodule** (`docs/private/…`).

---

*Made with love ♥ by Helix Development.*
