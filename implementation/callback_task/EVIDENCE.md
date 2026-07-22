# EVIDENCE — Standardized Callback/Task module (`digital.vasic.callbacktask`)

Physical, reproducible evidence. Captured by running the exact command sequence
below and pasting the real, unedited output. No output was fabricated or hand-edited.

## Environment

```
captured_at: 2026-07-22T12:18:41Z
go_version:  go version go1.26.4-X:nodwarf5 linux/amd64
module:      module digital.vasic.callbacktask
uname:       Linux 6.12.41-6.12-alt1 x86_64
```

## Command

```bash
cd implementation/callback_task && go build ./... && go vet ./... && go test ./... -v -race -count=1
```

## Output

```text
$ go build ./...
(exit 0 — no output means success)

$ go vet ./...
(exit 0 — no output means success)

$ go test ./... -v -race -count=1
=== RUN   TestStateTransitions_HappyPath
--- PASS: TestStateTransitions_HappyPath (0.00s)
=== RUN   TestInvalidTransitions
--- PASS: TestInvalidTransitions (0.00s)
=== RUN   TestRetryThenSuccess
--- PASS: TestRetryThenSuccess (0.00s)
=== RUN   TestExhaustionDeadAndDLQ
--- PASS: TestExhaustionDeadAndDLQ (0.00s)
=== RUN   TestNonRetryableFails
--- PASS: TestNonRetryableFails (0.00s)
=== RUN   TestIdempotentDoubleComplete
--- PASS: TestIdempotentDoubleComplete (0.00s)
=== RUN   TestConcurrentSubmitRaceClean
--- PASS: TestConcurrentSubmitRaceClean (0.00s)
=== RUN   TestWebhookFiresWithValidHMAC
--- PASS: TestWebhookFiresWithValidHMAC (0.00s)
=== RUN   TestWebhookRejectedOnTamperedSecret
--- PASS: TestWebhookRejectedOnTamperedSecret (0.00s)
=== RUN   TestWebhookRetryOn500Then200
--- PASS: TestWebhookRetryOn500Then200 (0.00s)
=== RUN   TestWebhookExhaustsRetries
--- PASS: TestWebhookExhaustsRetries (0.00s)
=== RUN   TestRegistryWebhookIntegration
--- PASS: TestRegistryWebhookIntegration (0.00s)
PASS
ok  	digital.vasic.callbacktask	1.024s
```

## Summary

- Tests run: 12
- Passed:    12
- Failed:    0
- Race detector: enabled (`-race`), clean (no DATA RACE reports)
- `go build`: exit 0 · `go vet`: exit 0 · `go test`: ok

### Requirement coverage

| Required behavior | Test | Result |
|---|---|---|
| State transitions (queued→running→progress→succeeded) | `TestStateTransitions_HappyPath` | PASS |
| Invalid transitions / bad progress / unknown id rejected | `TestInvalidTransitions` | PASS |
| Retry then success (retryable fail → retrying → running → succeeded) | `TestRetryThenSuccess` | PASS |
| Exhaustion → dead + dead-letter queue | `TestExhaustionDeadAndDLQ` | PASS |
| Non-retryable error → terminal failed (not DLQ) | `TestNonRetryableFails` | PASS |
| Idempotent double-complete (no-op, single notification) | `TestIdempotentDoubleComplete` | PASS |
| Concurrent submission is race-clean | `TestConcurrentSubmitRaceClean` | PASS |
| Webhook fires with valid HMAC the receiver recomputes & matches | `TestWebhookFiresWithValidHMAC` | PASS |
| Receiver rejects a wrong-secret HMAC; sink surfaces the error | `TestWebhookRejectedOnTamperedSecret` | PASS |
| Webhook retry on receiver 500 then 200 (HMAC valid each attempt) | `TestWebhookRetryOn500Then200` | PASS |
| Webhook delivery exhausts retries and errors | `TestWebhookExhaustsRetries` | PASS |
| End-to-end: Registry+WebhookSink delivers succeeded envelope, HMAC-verified | `TestRegistryWebhookIntegration` | PASS |

## Verdict

**READY.** The module compiles (`go build` exit 0), passes `go vet` (exit 0), and all
12 tests pass under the race detector (`-race`) with a clean run. The HMAC
signatures are independently recomputed by real `net/http/httptest` receivers (using
`crypto/hmac`+`crypto/sha256` directly, not the module’s own helper), so the signing
scheme is verified end-to-end, not asserted against itself. stdlib-only; no external
dependencies.

---

## Fix pass (review follow-up: findings A / B / C)

Post-review changes closing the reviewer's Important findings. No prior test was
weakened or deleted; the independent-HMAC test oracle is unchanged.

### What changed

- **A — anti-bluff / accuracy (delivery dead-lettering).** The `Notifier` doc
  comment previously claimed the sink does "dead-lettering of delivery failures",
  which `WebhookSink` never did — and `Registry.notify` discarded the returned
  error (`_ = r.notifier.Notify(...)`), so an exhausted webhook was silently lost.
  Fixed by (1) correcting the doc to state exactly what a sink does (in-flight
  retry/back-off only; no persistent DLQ) and (2) implementing real delivery
  dead-lettering: `Registry.notify` now captures the error, records the
  undeliverable envelope in `DeliveryFailures() []DeliveryFailure`, and invokes an
  optional `WithDeliveryErrorHook`. New test `TestRegistryCapturesExhaustedDelivery`
  drives a task to `succeeded` against an always-500 `httptest` server and proves
  the exhausted `succeeded` delivery is captured (list + hook), not swallowed.
- **B — documented the blocking constraint.** `notify` runs synchronously on the
  transition call path, so a retrying `WebhookSink` blocks the state-machine call.
  Documented in the `WithNotifier`/`notify` godoc and a new README section
  ("Delivery is synchronous (blocking)"). Not restructured to async.
- **C — coverage.** Added `TestDefaultBackoffArithmetic` (asserts
  `defaultBackoff` = 100ms·2^(n-1) to concrete values, incl. the `attempt<1`
  clamp), `TestStateTerminal` (terminal vs non-terminal states), and
  `TestVerifyRoundTrip` (`Verify` accepts prefixed + bare digests, rejects tampered
  body / wrong secret).

### Environment

```
captured_at: 2026-07-22T12:30:18Z
go_version:  go1.26.4-X:nodwarf5 linux/amd64
module:      module digital.vasic.callbacktask
uname:       Linux 6.12.41-6.12-alt1 x86_64
```

### Command

```bash
cd implementation/callback_task && go build ./... && go vet ./... && gofmt -l . && go test ./... -v -race -count=1
```

### Output

```text
$ go build ./...
(exit 0 — no output means success)

$ go vet ./...
(exit 0 — no output means success)

$ gofmt -l .
(no output — all files formatted)

$ go test ./... -v -race -count=1
=== RUN   TestStateTransitions_HappyPath
--- PASS: TestStateTransitions_HappyPath (0.00s)
=== RUN   TestInvalidTransitions
--- PASS: TestInvalidTransitions (0.00s)
=== RUN   TestRetryThenSuccess
--- PASS: TestRetryThenSuccess (0.00s)
=== RUN   TestExhaustionDeadAndDLQ
--- PASS: TestExhaustionDeadAndDLQ (0.00s)
=== RUN   TestNonRetryableFails
--- PASS: TestNonRetryableFails (0.00s)
=== RUN   TestIdempotentDoubleComplete
--- PASS: TestIdempotentDoubleComplete (0.00s)
=== RUN   TestDefaultBackoffArithmetic
--- PASS: TestDefaultBackoffArithmetic (0.00s)
=== RUN   TestStateTerminal
--- PASS: TestStateTerminal (0.00s)
=== RUN   TestConcurrentSubmitRaceClean
--- PASS: TestConcurrentSubmitRaceClean (0.00s)
=== RUN   TestWebhookFiresWithValidHMAC
--- PASS: TestWebhookFiresWithValidHMAC (0.00s)
=== RUN   TestWebhookRejectedOnTamperedSecret
--- PASS: TestWebhookRejectedOnTamperedSecret (0.00s)
=== RUN   TestWebhookRetryOn500Then200
--- PASS: TestWebhookRetryOn500Then200 (0.00s)
=== RUN   TestWebhookExhaustsRetries
--- PASS: TestWebhookExhaustsRetries (0.00s)
=== RUN   TestRegistryCapturesExhaustedDelivery
--- PASS: TestRegistryCapturesExhaustedDelivery (0.00s)
=== RUN   TestVerifyRoundTrip
--- PASS: TestVerifyRoundTrip (0.00s)
=== RUN   TestRegistryWebhookIntegration
--- PASS: TestRegistryWebhookIntegration (0.00s)
PASS
ok  	digital.vasic.callbacktask	1.025s
```

### Summary (fix pass)

- Tests run: 16 (was 12; +4: delivery-DLQ capture, defaultBackoff, State.Terminal, Verify)
- Passed:    16
- Failed:    0
- Race detector: enabled (`-race`), clean (no DATA RACE reports)
- `go build`: exit 0 · `go vet`: exit 0 · `gofmt -l .`: clean · `go test`: ok

### Verdict (fix pass)

**READY.** Findings A/B/C are closed. The `Notifier` doc no longer overclaims: it
states exactly what a sink does, and `Registry.notify` no longer discards the
delivery error — an exhausted webhook (including a `succeeded` completion) is now
recorded in `DeliveryFailures()` and surfaced to `WithDeliveryErrorHook`, proven by
`TestRegistryCapturesExhaustedDelivery`. The synchronous/blocking delivery
constraint is documented in godoc + README. All 16 tests pass under `-race`;
stdlib-only; no external dependencies.

## Doc-accuracy note (cross-cutting review, docs only)

Softened an overclaim: the package godoc (task.go) and README no longer promise
"a single completion shape regardless of provider". Reality: `callback_task` is the
generic **task** envelope (`{task_id, succeeded/failed}`); download/job-completion
sources (`metube_webhook`, `boba_adapter`) emit the *job* envelope
(`{job_id, success/failure}`) with the **same** `X-Thready-Signature` HMAC scheme —
a downstream receiver branches on `job_id` vs `task_id`. **Comments/README only — no
production logic changed.** Gate re-run after the edit (`GOWORK=off`):
`go build`/`go vet`/`gofmt -l .` clean, `go test ./... -race -count=1` →
`ok digital.vasic.callbacktask 1.028s` (16/16).
