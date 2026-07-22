# Download Manager — Build & Test Evidence

Physical proof that the `digital.vasic.downloadmanager` Go module compiles, vets
clean, and passes its full test suite with the race detector enabled. No mocks
for the wire: every HTTP test serves real bytes over `net/http/httptest` with
genuine Range support.

- **Gap addressed:** `[GAP: 6.3]` (Download Manager — generic multi-protocol
  download; HTTP + queue/resume/segmentation/progress/retry/callback).
- **Module path:** `digital.vasic.downloadmanager`
- **Go directive:** `go 1.26`
- **Go toolchain:** `go version go1.26.4-X:nodwarf5 linux/amd64`
- **Dependencies:** standard library only (no external modules).
- **Date captured:** 2026-07-22

## Commands

```
cd implementation/download_manager && go build ./... && go vet ./... && go test ./... -v -race -count=1
```

## Captured output (verbatim)

```text
$ go version
go version go1.26.4-X:nodwarf5 linux/amd64

$ go build ./...
(build ok, no output)

$ go vet ./...
(vet ok, no output)

$ go test ./... -v -race -count=1
=== RUN   TestFullDownloadSHA256
--- PASS: TestFullDownloadSHA256 (0.00s)
=== RUN   TestSegmentedDownloadByteIdentical
--- PASS: TestSegmentedDownloadByteIdentical (0.00s)
=== RUN   TestResumeAfterInterruption
--- PASS: TestResumeAfterInterruption (0.08s)
=== RUN   TestChecksumMismatchIsPermanent
--- PASS: TestChecksumMismatchIsPermanent (0.00s)
=== RUN   TestNoRangeServerFallback
--- PASS: TestNoRangeServerFallback (0.00s)
=== RUN   TestStubFetcherNotImplemented
--- PASS: TestStubFetcherNotImplemented (0.00s)
=== RUN   TestRetryThenSucceed
--- PASS: TestRetryThenSucceed (0.01s)
=== RUN   TestProgressCallbackMonotonic
--- PASS: TestProgressCallbackMonotonic (0.04s)
=== RUN   TestCompletionCallbackFiresOnce
--- PASS: TestCompletionCallbackFiresOnce (0.02s)
=== RUN   TestFailurePastMaxRetriesDead
--- PASS: TestFailurePastMaxRetriesDead (0.00s)
=== RUN   TestPermanentErrorFailedState
--- PASS: TestPermanentErrorFailedState (0.00s)
=== RUN   TestPauseResume
--- PASS: TestPauseResume (0.12s)
PASS
ok  	digital.vasic.downloadmanager	1.293s
```

## Pass/fail summary

```
ok  	digital.vasic.downloadmanager	1.293s   (12/12 tests PASS, -race, -count=1)
```

## Required-scenario coverage map

| # | Required behavior | Test |
|---|-------------------|------|
| 1 | Full download + sha256 matches source | `TestFullDownloadSHA256` |
| 2 | Segmented/ranged download reassembles byte-identical | `TestSegmentedDownloadByteIdentical` |
| 3 | Resume after simulated interruption completes correctly | `TestResumeAfterInterruption` |
| 4 | Retry: 500 twice then 200 → succeeds within max retries | `TestRetryThenSucceed` |
| 5 | Progress callback fires with monotonic bytes | `TestProgressCallbackMonotonic` |
| 6 | Completion callback fires once with final state | `TestCompletionCallbackFiresOnce` |
| 7 | Failure past max-retries → dead state + callback | `TestFailurePastMaxRetriesDead` |

Additional honest-coverage tests: `TestChecksumMismatchIsPermanent` (integrity
failure is non-retryable), `TestNoRangeServerFallback` (single-stream when the
server ignores Range), `TestStubFetcherNotImplemented` and
`TestPermanentErrorFailedState` (FTP/SMB/NFS/WebDav stubs fail honestly with
`ErrNotImplemented`, no retries), and `TestPauseResume` (pause mid-transfer,
resume from persisted state to a verified checksum).

## Reproduce

```
cd implementation/download_manager
go build ./... && go vet ./... && go test ./... -v -race -count=1
```

Stability: the timing-sensitive tests (`Resume`, `Pause`, `Progress`, `Retry`,
`Dead`) were additionally run 5× under `-race`; all passed each run.

## Verdict

**READY.** The HTTP(S) fetcher and the Manager (queue, worker pool, state
machine, retry with exponential backoff + full jitter, progress + completion
callbacks) are real and fully exercised against a live in-process HTTP server.
FTP/SMB/NFS/WebDav are honest interface stubs that return `ErrNotImplemented` —
the documented reuse points for `digital.vasic.filesystem`, not yet wired in.

## Race fix (Enqueue/Shutdown TOCTOU)

**Root cause:** `Manager.Enqueue` checked `m.stopped` under `m.mu`, released
`m.mu`, and only *then* did `m.queue <- j`. A concurrent `Shutdown` — which sets
`stopped` and `close(m.queue)` under `m.mu` — could land in that window, so the
post-unlock send hit a closed channel and panicked ("send on closed channel"),
crashing the process. (`Resume` was already correct: it sends while still
holding `m.mu`.)

**Fix (minimal):** move `m.queue <- j` *above* `m.mu.Unlock()` in `Enqueue`, so
the `stopped` check and the send are atomic with respect to `Shutdown`'s
`close(m.queue)` — mirroring `Resume`. Workers drain `m.queue` via `range`
without ever taking `m.mu`, so holding the lock across the buffered send cannot
deadlock. `Enqueue` after `Shutdown` returns `"downloadmanager: manager stopped"`
and never panics.

**Regression guard:** `TestEnqueueShutdownNoPanic` (64 concurrent enqueuers vs a
concurrent `Shutdown`, 300 attempts, per-goroutine `recover` asserts no panic)
and `TestEnqueueAfterShutdownReturnsError` (deterministic: post-shutdown enqueue
returns an error, no panic), in `manager_race_test.go`.

### Pre-fix reproduction (verbatim tail — captured before the fix)

The reproducer fired on the very first attempt, tripping both the race detector
(concurrent `chansend` at `manager.go:222` vs `closechan` at `manager.go:306`)
and the recovered panic:

```text
$ go test -run 'TestEnqueueShutdownNoPanic|TestEnqueueAfterShutdownReturnsError' -race -count=1 -v
==================
WARNING: DATA RACE
Read at 0x00c000109510 by goroutine 28:
  runtime.chansend()
      /usr/lib/golang/src/runtime/chan.go:176 +0x0
  digital%2evasic%2edownloadmanager.(*Manager).Enqueue()
      .../download_manager/manager.go:222 +0x5c4
  ...TestEnqueueShutdownNoPanic.func1()
      .../download_manager/manager_race_test.go:61 +0x218

Previous write at 0x00c000109510 by goroutine 78:
  runtime.closechan()
      /usr/lib/golang/src/runtime/chan.go:414 +0x0
  digital%2evasic%2edownloadmanager.(*Manager).Shutdown()
      .../download_manager/manager.go:306 +0xae
  ...TestEnqueueShutdownNoPanic.func2()
      .../download_manager/manager_race_test.go:75 +0x9e
==================
    manager_race_test.go:82: attempt 0: Enqueue panicked racing with Shutdown: send on closed channel
    testing.go:1712: race detected during execution of test
--- FAIL: TestEnqueueShutdownNoPanic (0.07s)
=== RUN   TestEnqueueAfterShutdownReturnsError
--- PASS: TestEnqueueAfterShutdownReturnsError (0.00s)
FAIL
exit status 1
FAIL	digital.vasic.downloadmanager	0.089s
```

### Post-fix verification (verbatim)

```text
$ go build ./... && go vet ./...
BUILD+VET OK

$ go test -run 'TestEnqueueShutdownNoPanic|TestEnqueueAfterShutdownReturnsError' -race -count=20
PASS
ok  	digital.vasic.downloadmanager	278.374s

$ go test ./... -race -count=1 -v
--- PASS: TestFullDownloadSHA256 (0.01s)
--- PASS: TestSegmentedDownloadByteIdentical (0.01s)
--- PASS: TestResumeAfterInterruption (0.08s)
--- PASS: TestChecksumMismatchIsPermanent (0.00s)
--- PASS: TestNoRangeServerFallback (0.00s)
--- PASS: TestStubFetcherNotImplemented (0.00s)
--- PASS: TestEnqueueShutdownNoPanic (22.44s)
--- PASS: TestEnqueueAfterShutdownReturnsError (0.00s)
--- PASS: TestRetryThenSucceed (0.05s)
--- PASS: TestProgressCallbackMonotonic (0.04s)
--- PASS: TestCompletionCallbackFiresOnce (0.02s)
--- PASS: TestFailurePastMaxRetriesDead (0.00s)
--- PASS: TestPermanentErrorFailedState (0.00s)
--- PASS: TestPauseResume (0.10s)
PASS
ok  	digital.vasic.downloadmanager	23.781s

$ go test -run 'TestPauseResume|TestProgressCallbackMonotonic|TestRetryThenSucceed|TestFailurePastMaxRetriesDead|TestResumeAfterInterruption' -race -count=5
PASS
ok  	digital.vasic.downloadmanager	2.496s
```

**Result:** 14/14 tests PASS under `-race` (12 original + 2 new); the
regression guard survives `-count=20`; timing-sensitive subset green at
`-count=5`. No `-race` warnings, no panics.
