# Skill-Dispatch Engine (`digital.vasic.skilldispatch`)

The Helix Thready **Skill-dispatch / execution engine**. It closes **`[GAP: 4.1]`**
from `docs/public/research/mvp/architecture/processing-pipeline.md`: HelixSkills
stores Skills as versioned **knowledge units** in a DAG but has **no execution
engine**. This module is that missing layer — it turns "recipe per hashtag" into
actually-running work.

Given a claimed post, it:

1. **Resolves** the matching Skill(s) from a hashtag/content-type registry
   (categories are *additive* — a post can match many Skills at once).
2. **Orders** them by the documented stage precedence.
3. **Runs** each Skill with **idempotent single-claim** (exactly-once per post,
   even under duplicate/concurrent triggers), **retry with exponential backoff**
   on transient failure (then marks that step *dead*), and **emits an event** per
   step start / success / failure.

Composition over helix_skills: the Skill-Graph remains the knowledge/ordering
source; this package adds only the execution layer. **Standard library only.**

## Precedence order

Skills run in a strict, deterministic stage order so that later stages consume
earlier outputs (processing-pipeline.md §5):

```
download  >  convert  >  analyze  >  research  >  reply
```

This ordering *is* the integer ordering of the `Kind` constants, and
`OrderByPrecedence` is a **stable** sort — Skills of the same `Kind` keep their
registration order. Ordering is decided by the orderer, **not** by registration
order across kinds: a research Skill registered before a download Skill still runs
after it.

## Idempotency guarantee

`ClaimRegistry` provides **exactly-once** processing per post ID:

- `Claim(postID)` returns `true` for the **first** caller only; it atomically
  moves the post to `Processing` under a mutex.
- Every later `Claim` for that ID — whether it is `Processing`, `Done`, or
  `Failed` — returns `false`. `Dispatcher.Process` treats a rejected claim as a
  **no-op** (`State = PostRejected`, no Skills run).
- A post that dies (a dead step) is marked `Failed` and **stays claimed**, so a
  duplicate trigger will not reprocess it. Reprocessing is an *explicit* op via
  `ClaimRegistry.Release`.

Under concurrent duplicate triggers, exactly one `Process` wins the claim and runs
each Skill exactly once. This is verified under `-race` with atomic call counts.

## API

```go
// A runnable unit composed over a helix_skills knowledge unit.
type Skill interface {
    Name() string                                        // stable id, e.g. "video.download"
    Kind() Kind                                          // pipeline stage (drives order)
    Match(post Post) bool                                // does this Skill apply?
    Run(ctx context.Context, post Post) (Result, error)  // do the (idempotent) work
}

// Register Skills; Resolve returns every Skill whose Match is true (additive).
reg := skilldispatch.NewRegistry()
reg.Register(downloadSkill, researchSkill)

// Build the engine. Options: WithRetry, WithEventSink, WithClaimRegistry, WithClock.
d := skilldispatch.NewDispatcher(reg,
    skilldispatch.WithRetry(skilldispatch.RetryPolicy{MaxAttempts: 3, BaseDelay: 100 * time.Millisecond, MaxDelay: 30 * time.Second}),
    skilldispatch.WithEventSink(mySink),
)

// Claim → resolve → order → run each step with retry/backoff, emitting events.
res, err := d.Process(ctx, skilldispatch.Post{
    ID:       "post-123",
    Hashtags: []string{"Research", "Video", "ToDownload"},
    Links:    []string{"https://youtu.be/x", "https://github.com/o/r"},
})
// res.State ∈ {PostCompleted, PostFailed, PostRejected, PostCanceled}
// res.Steps holds per-step attempts/outcome in execution order.
```

### Types

| Type | Role |
|------|------|
| `Post` | Unit of work: `ID` (claim key), `Hashtags`, `ContentType`, `Text`, `Links`. `HasHashtag`/`HasAnyHashtag` are case-insensitive and tolerate a leading `#`. |
| `Skill` | `Name` / `Kind` / `Match` / `Run`. Return a plain error for a **transient** failure (retried) or wrap it with `Permanent(err)` for a **non-retryable** one. |
| `Kind` | Stage: `KindDownload`, `KindConvert`, `KindAnalyze`, `KindResearch`, `KindReply`. Their order is the precedence. |
| `Registry` | `Register(...)`, `Resolve(post) []Skill` (registration order), `Len()`. |
| `OrderByPrecedence([]Skill) []Skill` | Stable sort into stage order; does not mutate input. |
| `ClaimRegistry` | `Claim`, `MarkDone`, `MarkFailed`, `Release`, `State`. The exactly-once guarantee. |
| `RetryPolicy` | `MaxAttempts`, `BaseDelay`, `MaxDelay`. `Delay(attempt)` is `BaseDelay·2^(attempt-2)` capped at `MaxDelay`, overflow-safe. |
| `EventSink` | `Emit(StepEvent)`. Receives every event in order (synchronous on the Process path). |
| `StepEvent` / `EventType` | `post.claimed`, `step.started`, `step.succeeded`, `step.failed`, `step.dead`, `post.completed`, `post.failed`, `post.rejected`. |
| `Dispatcher` | `Process(ctx, post) (PostResult, error)`, `Claims() *ClaimRegistry`. |

### Event stream

For a two-stage happy path the sink receives, in order:

```
post.claimed → step.started(dl) → step.succeeded(dl)
             → step.started(rs) → step.succeeded(rs) → post.completed
```

Each failed attempt emits a `step.failed` (with the 1-based `Attempt`); when
retries are exhausted (or a `Permanent` error is returned) the step emits a
terminal `step.dead` and the post ends `post.failed`. Because later stages consume
earlier outputs, a dead step is **fail-fast**: remaining stages are skipped.

### Retry semantics

- A Skill's `Run` error is **retryable by default** (an unclassified failure is
  retried, not dropped). Wrap with `Permanent(err)` to opt out.
- Backoff is exponential: `Delay(2) = BaseDelay`, `Delay(3) = 2·BaseDelay`, …,
  capped at `MaxDelay`. Backoff waits respect context cancellation.
- A step is attempted up to `MaxAttempts` times; the ceiling then dead-letters it.

## Run the tests

```
cd implementation/skill_dispatch
go build ./... && go vet ./... && gofmt -l . && go test ./... -v -race -count=1
```

See [`EVIDENCE.md`](./EVIDENCE.md) for captured build/vet/gofmt/test output and the
scenario coverage map.

## Scope / honesty

This is a self-contained, in-memory engine with **no external dependencies**. It
is the *execution* layer described by processing-pipeline.md §3; it does **not**
claim helix_skills executes work today, and it does not itself perform downloads,
OCR, or LLM research — those are supplied by concrete `Skill` implementations
(Download Manager, OCR adapter, research Skills) that plug into this engine. The
BackgroundTasks/Postgres durable claim and the event bus are the production
substrates this in-memory `ClaimRegistry` and `EventSink` model.

---

*Made with love ♥ by Helix Development.*
