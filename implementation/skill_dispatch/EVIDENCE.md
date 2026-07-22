# Skill-Dispatch Engine — Build & Test Evidence

Physical proof that the `digital.vasic.skilldispatch` Go module compiles, vets
clean, is gofmt-clean, and passes its full test suite with the race detector
enabled. No bluff: every assertion checks a real observable (recorded run order,
atomic call counts, captured events, claim states) produced by fake Skills that
record their calls and inject real failures.

- **Gap addressed:** `[GAP: 4.1]` — HelixSkills stores Skills as knowledge units
  (a DAG) but has **no execution engine**. This module is that missing
  dispatch/execution layer (processing-pipeline.md §3–§5).
- **Module path:** `digital.vasic.skilldispatch`
- **Go directive:** `go 1.26`
- **Go toolchain:** `go version go1.26.4-X:nodwarf5 linux/amd64`
- **Dependencies:** standard library only (no external modules).
- **Date captured:** 2026-07-22

## Commands

```
cd implementation/skill_dispatch && go build ./... && go vet ./... && gofmt -l . && go test ./... -v -race -count=1
```

## Captured output (verbatim)

```text
$ go version
go version go1.26.4-X:nodwarf5 linux/amd64

$ go build ./...
(build ok, no output)

$ go vet ./...
(vet ok, no output)

$ gofmt -l .
(no files need formatting — empty output)

$ go test ./... -v -race -count=1
=== RUN   TestClaim_FirstWinsSecondRejected
--- PASS: TestClaim_FirstWinsSecondRejected (0.00s)
=== RUN   TestClaim_MarkDoneAndFailedRejectReclaim
--- PASS: TestClaim_MarkDoneAndFailedRejectReclaim (0.00s)
=== RUN   TestClaim_ReleaseAllowsReclaim
--- PASS: TestClaim_ReleaseAllowsReclaim (0.00s)
=== RUN   TestClaim_MarkOnUnclaimedIsNoop
--- PASS: TestClaim_MarkOnUnclaimedIsNoop (0.00s)
=== RUN   TestClaim_ConcurrentSingleWinner
--- PASS: TestClaim_ConcurrentSingleWinner (0.00s)
=== RUN   TestDispatch_MultiCategoryRunsBoth
--- PASS: TestDispatch_MultiCategoryRunsBoth (0.00s)
=== RUN   TestDispatch_ExecutionOrderMatchesPrecedence
--- PASS: TestDispatch_ExecutionOrderMatchesPrecedence (0.00s)
=== RUN   TestDispatch_Idempotency_Sequential
--- PASS: TestDispatch_Idempotency_Sequential (0.00s)
=== RUN   TestDispatch_Idempotency_Concurrent
--- PASS: TestDispatch_Idempotency_Concurrent (0.00s)
=== RUN   TestDispatch_Retry_TransientThenSucceed
--- PASS: TestDispatch_Retry_TransientThenSucceed (0.00s)
=== RUN   TestDispatch_Retry_AlwaysFail_Dead
--- PASS: TestDispatch_Retry_AlwaysFail_Dead (0.00s)
=== RUN   TestDispatch_PermanentError_NoRetry
--- PASS: TestDispatch_PermanentError_NoRetry (0.00s)
=== RUN   TestDispatch_Events_InOrder
--- PASS: TestDispatch_Events_InOrder (0.00s)
=== RUN   TestDispatch_Events_RejectedDuplicate
--- PASS: TestDispatch_Events_RejectedDuplicate (0.00s)
=== RUN   TestDispatch_DeadStep_SkipsLaterStages
--- PASS: TestDispatch_DeadStep_SkipsLaterStages (0.00s)
=== RUN   TestDispatch_ContextCancellation
--- PASS: TestDispatch_ContextCancellation (0.02s)
=== RUN   TestOrderByPrecedence_StageOrder
--- PASS: TestOrderByPrecedence_StageOrder (0.00s)
=== RUN   TestOrderByPrecedence_DownloadBeforeResearch
--- PASS: TestOrderByPrecedence_DownloadBeforeResearch (0.00s)
=== RUN   TestOrderByPrecedence_StableWithinKind
--- PASS: TestOrderByPrecedence_StableWithinKind (0.00s)
=== RUN   TestOrderByPrecedence_DoesNotMutateInput
--- PASS: TestOrderByPrecedence_DoesNotMutateInput (0.00s)
=== RUN   TestRegistry_Resolve_ByHashtag
=== RUN   TestRegistry_Resolve_ByHashtag/video_only
=== RUN   TestRegistry_Resolve_ByHashtag/research_only
=== RUN   TestRegistry_Resolve_ByHashtag/both
=== RUN   TestRegistry_Resolve_ByHashtag/none
--- PASS: TestRegistry_Resolve_ByHashtag (0.00s)
    --- PASS: TestRegistry_Resolve_ByHashtag/video_only (0.00s)
    --- PASS: TestRegistry_Resolve_ByHashtag/research_only (0.00s)
    --- PASS: TestRegistry_Resolve_ByHashtag/both (0.00s)
    --- PASS: TestRegistry_Resolve_ByHashtag/none (0.00s)
=== RUN   TestRegistry_Resolve_HashtagNormalization
--- PASS: TestRegistry_Resolve_HashtagNormalization (0.00s)
=== RUN   TestRegistry_Len
--- PASS: TestRegistry_Len (0.00s)
PASS
ok  	digital.vasic.skilldispatch	1.161s
```

## Pass/fail summary

```
ok  digital.vasic.skilldispatch   1.161s   (23 tests / 27 cases incl. subtests, 100% PASS, -race, -count=1)
```

Stability: the full suite (including the concurrency and context-cancellation
tests) was additionally run 3× more under `-race`; all passed each run.

```
race run 1: PASS
race run 2: PASS
race run 3: PASS
```

## Required-scenario coverage map

| # | Required behavior (task TDD) | Test |
|---|------------------------------|------|
| 1 | `#Video` → download skill; `#Research` → research skill; both → both | `TestRegistry_Resolve_ByHashtag`, `TestDispatch_MultiCategoryRunsBoth` |
| 2 | Execution ORDER matches precedence: research+download runs download BEFORE research (recorded order) | `TestDispatch_ExecutionOrderMatchesPrecedence`, `TestOrderByPrecedence_DownloadBeforeResearch` |
| 3 | Idempotency (sequential): 2nd Process is a claim-rejected no-op; skill runs exactly once | `TestDispatch_Idempotency_Sequential` |
| 4 | Idempotency (concurrent): same post processed at once → each skill runs exactly once, one winner | `TestDispatch_Idempotency_Concurrent`, `TestClaim_ConcurrentSingleWinner` |
| 5 | Retry: transient-fail ×2 then success → runs 3× and post completes | `TestDispatch_Retry_TransientThenSucceed` |
| 6 | Retry: always-fail → retried to max, step dead + failure events emitted | `TestDispatch_Retry_AlwaysFail_Dead` |
| 7 | Events: sink received expected start/success/failure events in order | `TestDispatch_Events_InOrder` |

Additional honest-coverage tests: `TestDispatch_PermanentError_NoRetry` (a
`Permanent`-wrapped error is not retried — runs once, dies), `TestDispatch_Events_RejectedDuplicate`
(a duplicate trigger emits exactly one `post.rejected` event and nothing else),
`TestDispatch_DeadStep_SkipsLaterStages` (fail-fast: a dead download skips the
research stage that would consume its output), `TestDispatch_ContextCancellation`
(cancellation mid-backoff stops retries and yields a canceled post),
`TestOrderByPrecedence_StageOrder` / `StableWithinKind` / `DoesNotMutateInput`
(full stage ordering, stable within a kind, non-mutating), and the claim-registry
unit tests (`FirstWinsSecondRejected`, `MarkDoneAndFailedRejectReclaim`,
`ReleaseAllowsReclaim`, `MarkOnUnclaimedIsNoop`).

## Reproduce

```
cd implementation/skill_dispatch
go build ./... && go vet ./... && gofmt -l . && go test ./... -v -race -count=1
```

## Verdict

**READY.** The dispatch engine is real and fully exercised: hashtag/content-type
→ Skill resolution (additive categories), deterministic stage ordering
(download → convert → analyze → research → reply, stable within a stage),
idempotent single-claim (exactly-once under duplicate and concurrent triggers,
proven with atomic call-count assertions under `-race`), per-step retry with
exponential backoff to a ceiling then dead-lettering, and a start/success/failure
event stream asserted in order. No external dependencies; standard library only.
The engine is the execution layer composed over the helix_skills Skill-Graph — it
does not claim helix_skills executes work; it adds the execution that GAP 4.1 says
is missing.
