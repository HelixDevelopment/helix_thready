<!--
  Title           : Helix Thready — Coding Standards
  Classification  : PUBLIC
  Location        : docs/public/research/mvp/development/coding-standards.md
  Status          : Review — v0.2
  Revision        : 2 (2026-07-21)
  Author          : Helix Thready documentation swarm (development)
  Related         : ./index.md, ./contribution-guidelines.md, ./workable-items.md,
                    ../testing/index.md
-->

# Helix Thready — Coding Standards

| Rev | Date | Author | Change |
|-----|------|--------|--------|
| 1 | 2026-07-21 | swarm (development) | Initial draft — Go 1.26, TDD, patterns, SOLID, decoupling, concurrency |
| 2 | 2026-07-21 | swarm (development, review) | Review pass — clarified operator precedence in the anti-bluff embedder guard |

These are the engineering standards every `ATM-NNN` implementation must meet. They implement the
final request §5.2 development principles and the Constitution's TDD `[§11.4.43]`, decoupling
`[§11.4.28]`, and anti-bluff `[§11.4.27]` mandates.

## Table of Contents

- [1. Language & toolchain](#1-language--toolchain)
- [2. TDD — reproduce-first `[§11.4.43]`](#2-tdd--reproduce-first-114-43)
- [3. Design patterns](#3-design-patterns)
- [4. SOLID & core principles](#4-solid--core-principles)
- [5. Decoupling `[§11.4.28]`](#5-decoupling-114-28)
- [6. Concurrency](#6-concurrency)
- [7. Error handling & observability](#7-error-handling--observability)
- [8. Anti-bluff discipline `[§11.4.27]`](#8-anti-bluff-discipline-114-27)
- [9. Naming, layout, formatting](#9-naming-layout-formatting)
- [10. Non-Go surfaces](#10-non-go-surfaces)

## 1. Language & toolchain

- **Primary language: Go 1.26.x** `[CONSTITUTION silent → project pins latest stable]`. The
  Constitution does not pin a Go version; the project pins the latest stable, matching org tooling
  (1.25/1.26). `go.mod` declares `go 1.26`.
- **Formatting/vetting (mandatory):** `gofmt -s`, `go vet`, `staticcheck`, `golangci-lint` (with
  `errcheck`, `govet`, `staticcheck`, `ineffassign`, `revive`). CI-equivalent enforcement runs in
  the local git hooks `[§11.4.75/156]` — there is no server CI.
- **Race + mutation:** `go test -race` on every package; `go-mutesting` mutation testing plus
  paired-mutation anti-bluff gates (§8).
- **No cgo unless required:** dev uses cgo-free `modernc.org/sqlite`; cgo is allowed only where a
  capability demands it (e.g. `gosseract`/Tesseract in the OCR adapter `ATM-033`), gated behind a
  build tag and documented.

## 2. TDD — reproduce-first `[§11.4.43]`

Every change is **reproduce-first**: write the failing (RED) test that reproduces the requirement
or bug **before** any fix; the same test asserts the fix (GREEN); then extend to all cases. Iron
Law `[§11.4.102]`: **no fix without root-cause investigation first** — no guess-and-retry, no
"probably flaky" reruns without captured forensic evidence.

```mermaid
flowchart LR
  REQ[Requirement / bug] --> RED[Write failing RED test\nreproduces exactly]
  RED --> RC[Root-cause investigation\n§11.4.102 Iron Law]
  RC --> IMPL[Implement minimal fix]
  IMPL --> GREEN[Same test -> GREEN]
  GREEN --> EXT[Extend to all cases\n+ 15 test types where applicable]
  EXT --> REV[Fable @ xhigh review §11.4.209]
```

**Explanation (for readers/models that cannot see the diagram).** A requirement or bug first becomes
a RED test that reproduces it precisely — never a fix attempt. Only after a completed root-cause
investigation (the §11.4.102 Iron Law forbids fixing before understanding) does the minimal
implementation follow, turning the same test GREEN. The work then extends to all edge cases and adds
the applicable mandated test types, and finally passes the independent Fable review. The RED test is
the contract: it must fail for the right reason before it is allowed to pass.

> Rendered PNG/SVG exported via Docs Chain (§11.4.65). Source: [diagrams/tdd-reproduce-first.mmd](./diagrams/tdd-reproduce-first.mmd).

```go
// reproduce-first: the RED test is written and MUST fail before the dispatcher exists.
func TestDispatcher_MultiHashtag_DownloadBeforeResearch(t *testing.T) {
    post := Post{Hashtags: []string{"#Video", "#Research"}}
    got := NewDispatcher(testGraph).Plan(post)          // Plan() not yet implemented -> RED
    want := []SkillStep{{Skill: "video-download", Order: 10}, {Skill: "deep-research", Order: 40}}
    if !reflect.DeepEqual(got, want) {
        t.Fatalf("precedence violated: download must precede research\n got=%v\nwant=%v", got, want)
    }
}
```

## 3. Design patterns

The Constitution does not mandate specific GoF patterns; they are adopted as **project engineering
standards** per final request §5.2. Standard patterns and where they apply:

| Pattern | Thready usage |
|---------|---------------|
| **Adapter** | `OCRProvider` (Tesseract/PaddleOCR), messenger channels (Telegram/Max), download sources (HTTP/FTP/SMB) |
| **Factory / Factory-of-Factories** | `VectorStore` (pgvector/Qdrant), embedding-provider selection, download-source construction |
| **Strategy** | Retry/back-off policies, transcode profiles, embedding-model choice |
| **Circuit-Breaker** | Guarding flapping external systems (present in `LLMProvider`/`filesystem`/`lets_encrypt`) |
| **Observer** | Event Bus subscriptions; processing progress events |
| **Facade** | The SDK surface over the REST/Protobuf core; the Asset Service over storage+filesystem |
| **Mediator** | The Skill-dispatch engine coordinating download/convert/analyze/research steps |
| **Proxy** | Asset links that resolve through the Asset Service, never direct file paths |

```go
// Adapter + Factory: a first-class OCRProvider seam (closes VisionEngine's no-OCR gap, ATM-033).
type OCRProvider interface {
    Recognize(ctx context.Context, img image.Image) (OCRResult, error) // per-word boxes + text
    Name() string
}
type OCRResult struct{ Text string; Words []WordBox }
type WordBox struct{ Text string; Rect image.Rectangle; Conf float64 }

func NewOCRProvider(cfg OCRConfig) (OCRProvider, error) { // Factory selects the concrete adapter
    switch cfg.Engine {
    case "tesseract": return newTesseractAdapter(cfg)     // cgo gosseract or subprocess
    case "paddle":    return newPaddleAdapter(cfg)         // hard/multilingual scans
    default:          return nil, fmt.Errorf("ocr: unknown engine %q", cfg.Engine)
    }
}
```

## 4. SOLID & core principles

- **S**ingle-Responsibility — one reason to change per type; the dispatch engine dispatches, it does
  not embed nor transcode.
- **O**pen-Closed — extend via new adapters/strategies, not by editing existing switch bodies;
  register new providers through the factory.
- **L**iskov — every `OCRProvider`/`VectorStore`/`Channel` implementation is substitutable; contract
  tests assert this (`ATM-064`).
- **I**nterface-Segregation — small, purpose-built interfaces (`Recognize`, `OpenSeekable`), not
  god-interfaces.
- **D**ependency-Inversion — depend on interfaces, inject concretes via config (`[§11.4.28]`).
- Plus **DRY**, **KISS**, heavy decoupling and reuse. Prefer composition over inheritance.

## 5. Decoupling `[§11.4.28]`

VERIFIED at source (Constitution §11.4.28 / CONST-051). Rules for every Thready module:

- **Project-not-aware + config-injected.** A module never hardcodes Thready specifics; it takes a
  `Config` and dependencies via constructor injection.
- **Generic interfaces + abstract factories** for variants; no hard coupling to a concrete backend.
- **Dependency layout** `<root>/<name>/` or `<root>/submodules/<name>/`; **no nested own-org
  submodule chains** (depth-1 exception only for constitution-anchored engines with `helix-deps.yaml`).
- New capabilities become their **own repo** (see [build-new-subsystems.md](./build-new-subsystems.md)),
  never a vendored copy.

```go
// Dependency inversion + config injection: the dispatcher depends on interfaces it does not construct.
type Dispatcher struct {
    graph   SkillGraph      // read-only knowledge source (helix_skills)
    queue   TaskQueue       // digital.vasic.background
    bus     EventPublisher  // digital.vasic.eventbus
}
func NewDispatcher(cfg Config, graph SkillGraph, queue TaskQueue, bus EventPublisher) *Dispatcher {
    return &Dispatcher{graph: graph, queue: queue, bus: bus} // nothing project-specific hardcoded
}
```

## 6. Concurrency

Final request §5.2: "heavy use of atomic, non-blocking operations, correct data structures and
synchronization." Standards:

- Prefer **channels** and `context.Context` for cancellation/deadlines; every exported blocking call
  takes a `ctx`.
- Use `sync/atomic` and `sync.Once`/`sync.Map` where a mutex would be a bottleneck; document the
  invariant each atomic protects.
- **Idempotent single-claim** for the per-post processor (`ATM-023`) via Postgres advisory locks —
  the runtime analogue of the §11.4.176 exactly-once claim.
- Bound concurrency: the BackgroundTasks worker pool (`[DEFAULT — adjustable]` 32 workers), per-Skill
  concurrency caps, download concurrency delegated to Boba/MeTube/Download-Manager pools (Q4).
- Always `go test -race`; a data race is a release blocker.

```go
// Non-blocking exactly-once claim: TryAcquire returns false if another worker already holds the post.
func (p *Processor) TryClaim(ctx context.Context, postID int64) (bool, error) {
    // pg_try_advisory_xact_lock is non-blocking: it returns immediately, no goroutine parks.
    var ok bool
    err := p.db.QueryRowContext(ctx, `SELECT pg_try_advisory_xact_lock($1)`, postID).Scan(&ok)
    return ok, err // ok==false => another worker owns this post; skip without blocking
}
```

## 7. Error handling & observability

- **Wrap, don't swallow:** `fmt.Errorf("dispatch %d: %w", id, err)`; never discard an error without a
  logged reason (`errcheck` enforces).
- **Typed sentinels** for control flow (`errors.Is`/`errors.As`); no string-matching on messages.
- **Structured logging** via `digital.vasic.observability` (logrus + correlation ids); OTel spans on
  every cross-service call; Prometheus metrics on queues, latencies, retries.
- **Honest SKIP** `[§11.4.3/52]`: when a capability is genuinely unavailable (e.g. `pandoc` missing
  for Docs Chain, credentials absent), SKIP with a logged reason — never fake success.
- **Never log secrets** `[§11.4.10]`; credentials are runtime-load-only and redacted in logs.

## 8. Anti-bluff discipline `[§11.4.27]`

A green test must prove **real behavior**, not a stub. This is the single most important standard for
Thready because the gap register flags several dependencies as scaffolds:

- **No mocks beyond unit tests.** Integration/e2e/system/security/performance/etc. exercise the real
  system `[§11.4.27]`. Mocks/stubs/TODOs are unit-only.
- **Paired-mutation gates** `[CONST-035]`: for any module the gap register flags SCAFFOLD/DESIGN-ONLY
  (HelixLLM embedder, TOON, session_orchestrator, Security-KMP), add a gate that mutates the
  implementation and asserts the test **FAILs**, then restores and asserts it **PASSes** — proving
  the test detects real behavior. `ATM-058` runs this sweep before reliance.
- **Never claim a stub works.** If `HELIX_EMBEDDING_PROVIDER` is the `HashEmbedder`, the code **fails
  loudly** in a RAG/search context (`ATM-040`), it does not warn-and-continue.

```go
// Anti-bluff guard: refuse the non-semantic hash embedder in any semantic context (closes GAP 2.1).
func RequireSemanticEmbedder(cfg EmbedConfig) error {
    // Explicit parens: reject the hash provider outright, OR a "local" provider with no model loaded
    // (the HashEmbedder trap). Parenthesized so precedence is unmistakable, not relying on && > ||.
    if cfg.Provider == "hash" || (cfg.Provider == "local" && cfg.Model == "") {
        return fmt.Errorf("refusing HashEmbedder in semantic context: set HELIX_EMBEDDING_PROVIDER=llama")
    }
    return nil
}
```

## 9. Naming, layout, formatting

- **Lowercase snake_case** for repo/dir/file names `[§11.4.29 / CONST-052]`; Go identifiers follow
  standard Go casing.
- Package layout: `cmd/` (entrypoints), `pkg/` (public), `internal/` (private), `test/` (fixtures);
  one responsibility per package.
- Every exported symbol has a doc comment; every package has a `doc.go` or package comment.
- Every `.md` gets HTML/PDF/DOCX siblings via Docs Chain `[§11.4.65]`.

## 10. Non-Go surfaces

| Surface | Standard |
|---------|----------|
| TypeScript/Angular | Angular 19 (product) / 22 (marketing) on `design_system`; Jasmine+Karma unit, Cypress e2e (`cypress-axe`), Prettier+ESLint |
| Rust (Tauri) | `cargo fmt`, `cargo clippy -D warnings`, `cargo test` |
| Kotlin/KMP | ktlint/detekt; Kotest/kotlin-test; JUnit + Compose instrumented tests |
| Swift | swiftformat/swiftlint; XCTest |
| SQL | PostgreSQL DDL; migrations up/down via `migration.Runner`; expand-contract |
| YAML/Proto | OpenAPI 3.1 + Protobuf via the `helix_proto` pattern (`buf`, `openapi-generator`) |

---

*Made with love ♥ by Helix Development.*
