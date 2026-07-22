# Processing Orchestrator — Build & Test Evidence

Physical proof that the `digital.vasic.processing` Go module compiles, vets clean,
is gofmt-clean, and passes its full test suite with the race detector enabled. No
bluff: every assertion checks a real observable (recorded skill run order, atomic
run counts, captured lifecycle events, captured completion envelopes, claim states)
produced by fake seams that record their calls and inject real failures.

- **What it is:** the reusable Processing-Engine core — the shippable orchestrator
  that composes the pipeline modules **at runtime** for one claimed post (claim →
  resolve+order → run with retry/backoff → emit events → completion callback).
- **Composition by seams, not by import:** the module imports **no** sibling
  modules. Every collaborator is an injected interface — `Claimer`, `SkillSet`,
  `Skill`/`StepRunner`, `EventEmitter`, `Callbacker` — that the real modules satisfy
  via thin adapters. The shapes deliberately mirror the committed contracts so the
  adapters are near-trivial: `Post` ← `threadreader.Post`; `Kind`/precedence +
  claim + retry + event set ← `skill_dispatch`; `Completion` ← `callback_task.Envelope`
  (field-for-field: task_id/state/progress/result_ref/error/ts). The integration
  capstone (`implementation/integration`) already proved the real modules satisfy
  such seams in a composed test; this module is the runtime orchestrator behind them.
- **Module path:** `digital.vasic.processing`
- **Go directive:** `go 1.26`
- **Go toolchain:** `go version go1.26.4-X:nodwarf5 linux/amd64`
- **Dependencies:** standard library only (no external modules, no sibling modules).
- **Date captured:** 2026-07-22

## Commands

```
cd implementation/processing && GOWORK=off go build ./... && GOWORK=off go vet ./... && GOWORK=off gofmt -l . && GOWORK=off go test ./... -v -race -count=1
```

`GOWORK=off` is required: this is a standalone module and is deliberately NOT a
member of `implementation/go.work`.

## Captured output (verbatim)

```text
$ go version
go version go1.26.4-X:nodwarf5 linux/amd64

$ GOWORK=off go build ./...
(build ok, no output)

$ GOWORK=off go vet ./...
(vet ok, no output)

$ GOWORK=off gofmt -l .
(no files need formatting — empty output)

$ GOWORK=off go test ./... -v -race -count=1
=== RUN   TestProcess_MultiCategory_PrecedenceOrder_EventsAndCallback
--- PASS: TestProcess_MultiCategory_PrecedenceOrder_EventsAndCallback (0.00s)
=== RUN   TestProcess_Idempotency_Sequential
--- PASS: TestProcess_Idempotency_Sequential (0.00s)
=== RUN   TestProcess_Idempotency_Concurrent
--- PASS: TestProcess_Idempotency_Concurrent (0.00s)
=== RUN   TestProcess_Retry_TransientThenSucceed
--- PASS: TestProcess_Retry_TransientThenSucceed (0.00s)
=== RUN   TestProcess_AlwaysFail_Dead_FailedPost
--- PASS: TestProcess_AlwaysFail_Dead_FailedPost (0.00s)
=== RUN   TestProcess_PermanentError_NoRetry
--- PASS: TestProcess_PermanentError_NoRetry (0.00s)
=== RUN   TestProcess_FailFast_SkipsLaterStages
--- PASS: TestProcess_FailFast_SkipsLaterStages (0.00s)
=== RUN   TestProcess_FailFastOff_RunsRemaining
--- PASS: TestProcess_FailFastOff_RunsRemaining (0.00s)
=== RUN   TestProcess_ContextCancellation
--- PASS: TestProcess_ContextCancellation (0.00s)
=== RUN   TestProcess_RejectedDuplicate_FiresOnlyRejectedEvent
--- PASS: TestProcess_RejectedDuplicate_FiresOnlyRejectedEvent (0.00s)
=== RUN   TestProcess_CallbackDeliveryError_Surfaced
--- PASS: TestProcess_CallbackDeliveryError_Surfaced (0.00s)
=== RUN   TestOrderByPrecedence_SortsAndIsStable
--- PASS: TestOrderByPrecedence_SortsAndIsStable (0.00s)
=== RUN   TestMemoryClaimer_SingleWinnerConcurrent
--- PASS: TestMemoryClaimer_SingleWinnerConcurrent (0.00s)
=== RUN   TestMemoryClaimer_Lifecycle
--- PASS: TestMemoryClaimer_Lifecycle (0.00s)
=== RUN   TestPost_Hashtags
--- PASS: TestPost_Hashtags (0.00s)
=== RUN   TestKind_StringAndValid
--- PASS: TestKind_StringAndValid (0.00s)
=== RUN   TestRetryPolicy_Delay
--- PASS: TestRetryPolicy_Delay (0.00s)
PASS
ok  	digital.vasic.processing	1.025s
```

## Pass/fail summary

```
ok  digital.vasic.processing   1.025s   (17 tests, 100% PASS, -race, -count=1)
```

Stability: the full suite (including the concurrency and cancellation tests) was
additionally run 3× more under `-race`; all passed each run.

```
race run 1: PASS
race run 2: PASS
race run 3: PASS
```

## Required-scenario coverage map

| # | Required behavior (task TDD) | Test |
|---|------------------------------|------|
| 1 | `#Video #Research` post → skills run in precedence order (download-kind before analyze- before research-kind) with events + callback fired | `TestProcess_MultiCategory_PrecedenceOrder_EventsAndCallback` |
| 2 | Idempotency (sequential): 2nd Process is a claim-rejected no-op; skill runs exactly once; one completion | `TestProcess_Idempotency_Sequential` |
| 3 | Idempotency (concurrent): same post processed at once → one claim winner, each skill runs exactly once | `TestProcess_Idempotency_Concurrent`, `TestMemoryClaimer_SingleWinnerConcurrent` |
| 4 | Retry: transient-fail ×2 then success → runs 3× and post completes | `TestProcess_Retry_TransientThenSucceed` |
| 5 | Always-failing step → dead + failure events + overall failed | `TestProcess_AlwaysFail_Dead_FailedPost` |
| 6 | Event stream asserted in order | `TestProcess_MultiCategory_…` (full stream), `TestProcess_Retry_…`, `TestProcess_AlwaysFail_…`, `TestProcess_PermanentError_NoRetry` |
| 7 | Callback envelope carries the right final state | `TestProcess_MultiCategory_…` (succeeded), `TestProcess_AlwaysFail_…` (failed) |

Additional honest-coverage tests: `TestProcess_PermanentError_NoRetry` (a
`Permanent`-wrapped error runs once and dies — not retried),
`TestProcess_FailFast_SkipsLaterStages` (default fail-fast skips the later stage
that would consume a dead step's output), `TestProcess_FailFastOff_RunsRemaining`
(the fail-fast option off runs remaining steps but still fails the post),
`TestProcess_ContextCancellation` (cancellation during backoff yields a canceled
post and returns the ctx error; claim marked failed),
`TestProcess_RejectedDuplicate_FiresOnlyRejectedEvent` (a duplicate fires exactly
one `post.rejected` and no completion callback),
`TestProcess_CallbackDeliveryError_Surfaced` (a completion-callback delivery failure
is surfaced via the returned error while `State` still reports Completed),
`TestOrderByPrecedence_SortsAndIsStable` (stage ordering, stable within a kind,
non-mutating), and the claim-registry and Post/Kind/RetryPolicy unit tests.

## Reproduce

```
cd implementation/processing
GOWORK=off go build ./... && GOWORK=off go vet ./... && GOWORK=off gofmt -l . && GOWORK=off go test ./... -v -race -count=1
```

## Verdict

**READY.** The Processing orchestrator is real and fully exercised: exactly-once
single-claim (proven with atomic run-count assertions under `-race`, sequential AND
concurrent), deterministic stage ordering (download → convert → analyze → research →
reply, imposed by the orderer regardless of resolve order, stable within a stage),
per-step retry with exponential backoff to a ceiling then dead-lettering (transient
vs `Permanent` distinguished), an in-order start/success/failure lifecycle event
stream, a fail-fast option, honored context cancellation, and a completion callback
whose envelope carries the correct final state (succeeded/failed) — mirroring
`callback_task.Envelope`. It composes the pipeline modules purely through interface
seams and imports no siblings; the real modules plug in via thin adapters, which the
integration capstone already proved satisfy such seams. Standard library only.
