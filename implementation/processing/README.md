# processing — Helix Thready Processing Orchestrator (`digital.vasic.processing`)

The reusable **Processing-Engine core**: the shippable orchestrator that composes
the Thready pipeline modules **at runtime** to process one claimed post. Given a
post, it claims it exactly once, resolves and orders the matching Skills by the
deterministic stage precedence, runs each with retry/backoff, emits lifecycle
events, and fires a completion callback carrying the final state.

It realises the runtime behaviour of
`docs/public/research/mvp/architecture/processing-pipeline.md` (§1, §3–§5, §9).

## Purpose: composition by seams, not by import

The integration capstone (`implementation/integration`) already proved the real
modules compose **in a test**. This module is the orchestrator that composes them
**at runtime**, decoupled behind interfaces — so it imports **no** sibling modules
and depends only on the Go standard library. The real modules plug in through thin
adapters, and the shapes here deliberately mirror the committed contracts so those
adapters are near-trivial:

| Seam (interface) | Responsibility | Real module behind it |
|------------------|----------------|-----------------------|
| `Claimer` | Idempotent single-claim per post id (exactly-once) | `skill_dispatch.ClaimRegistry` / a Postgres claim registry |
| `SkillSet` | Resolve a post → the Skills that apply (hashtag/content-type → Skills) | `skill_dispatch.Registry.Resolve` |
| `Skill` (`StepRunner`) | A runnable step: `Name`/`Kind`/`Run(ctx, post) → StepResult` | `skill_dispatch.Skill` (real download/OCR/research skills) |
| `EventEmitter` | Receive per-step lifecycle events | an `event_bus_service` adapter (mirrors `skill_dispatch.StepEvent`) |
| `Callbacker` | Fire the completion callback | a `callback_task.WebhookSink` adapter |

The `Post` type mirrors `threadreader.Post` (id, threadID, hashtags, text,
attachments); `Completion` mirrors `callback_task.Envelope` field-for-field
(`task_id, state, progress, result_ref, error, ts`); `Kind`, the retry policy, the
claim states and the event set match `skill_dispatch`. Nothing here claims the real
modules are imported — it is standalone and unit-tested on its own, and the capstone
already proved the real modules satisfy such seams.

## Stage precedence

`OrderByPrecedence` sorts the resolved Skills into the fixed, deterministic order
(processing-pipeline.md §5), stably within a stage:

```
download  >  convert  >  analyze  >  research  >  reply
```

Later stages consume earlier outputs, so the order is imposed by the orchestrator
regardless of the order the `SkillSet` returns.

## Process flow

`Processor.Process(ctx, post) (Result, error)`:

1. **Claim** the post via `Claimer`. A rejected claim (duplicate trigger) emits
   `post.rejected` and returns `State=Rejected`, `ProcessedOnce=false` — a no-op.
   This is the exactly-once guarantee.
2. **Resolve + order** the Skills (`SkillSet.Resolve` then `OrderByPrecedence`).
3. **Run each step** with retry + exponential backoff up to `MaxAttempts`; a
   transient error is retried, a `Permanent(err)` is not. On exhaustion the step is
   **dead**. With fail-fast on (default) a dead step skips the remaining steps.
4. **Emit events** throughout: `post.claimed`, per step `step.started` /
   `step.succeeded` / `step.failed` (per attempt) / `step.dead`, then `post.completed`
   or `post.failed`.
5. **Fire the completion callback** (`Callbacker.Notify`) with a `Completion`
   envelope carrying the final state, progress, first artifact ref, and (on failure)
   the error.

Returns `Result{PostID, State, ProcessedOnce, Steps[], Assets[]}`. The error is nil
for rejected/completed/failed posts (a dead step is reported via `State`); it is the
context error if the context is canceled mid-run, or a wrapped delivery error if the
completion callback itself fails (the `State` still reports the true outcome).

## Usage sketch

```go
proc := processing.NewProcessor(
    processing.NewMemoryClaimer(),          // or an adapter over the real claim registry
    skillSet,                               // your SkillSet resolver
    processing.WithEmitter(busAdapter),     // bridge to event_bus_service
    processing.WithCallbacker(webhookSink), // bridge to callback_task
    processing.WithRetry(processing.RetryPolicy{MaxAttempts: 3, BaseDelay: 100 * time.Millisecond, MaxDelay: 30 * time.Second}),
    // processing.WithFailFast(false),      // opt out of fail-fast
)
res, err := proc.Process(ctx, post)
```

## Run the tests

```
cd implementation/processing
GOWORK=off go build ./... && GOWORK=off go vet ./... && GOWORK=off gofmt -l . && GOWORK=off go test ./... -v -race -count=1
```

`GOWORK=off` is required — this is a standalone module, deliberately not a member of
`implementation/go.work`. See `EVIDENCE.md` for captured build/vet/gofmt/test output
(17 tests, 100% pass under `-race`) and the required-scenario coverage map.

## Files

- `post.go` — `Post`, `Attachment`, hashtag matching (mirrors `threadreader.Post`).
- `kind.go` — `Kind` stages + precedence ordering constants.
- `skill.go` — `Skill`/`StepRunner` seam, `StepResult`, `Permanent`/`IsRetryable`.
- `resolve.go` — `SkillSet` seam, `SkillSetFunc`, `OrderByPrecedence`.
- `retry.go` — `RetryPolicy` (exponential backoff, overflow-safe).
- `event.go` — `EventType`, `Event`, `EventEmitter` seam, `EmitterFunc`.
- `callback.go` — `CompletionState`, `Completion` (mirrors `callback_task.Envelope`), `Callbacker` seam.
- `claim.go` — `ClaimState`, `Claimer` seam, `MemoryClaimer` reference implementation.
- `processor.go` — `Processor`, options, `State`, `Result`, `Process`.
- `*_test.go` — TDD suite with fake seams (recording call order + injectable failures).

---

*Made with love ♥ by Helix Development.*
