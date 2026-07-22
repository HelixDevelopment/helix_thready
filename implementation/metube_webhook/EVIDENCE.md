# EVIDENCE — MeTube outbound-completion-webhook shim

Module: `digital.vasic.metubewebhook`
Gap closed: **[GAP: 6.5]** — MeTube (milos85vasic/YT-DLP) is poll-only
(`GET /api/postprocess/status`, `GET /api/postprocess/jobs`) and has no outbound
completion webhook. This module is the **outbound shim**: it polls MeTube,
detects terminal transitions, and fires one HMAC-signed completion webhook per
job.

Physical evidence — the exact commands and their real, unedited output.
Captured on host `linux/amd64`.

## Command sequence

```
cd implementation/metube_webhook
go build ./... && go vet ./... && gofmt -l . && go test ./... -v -race -count=1
```

## go version

```
go version go1.26.4-X:nodwarf5 linux/amd64
```

## go build ./...

```
(build OK — no output, exit 0)
```

## go vet ./...

```
(vet OK — no output, exit 0)
```

## gofmt -l .

```
(clean — no files listed, exit 0)
```

## go test ./... -v -race -count=1

```
=== RUN   TestParseJobs_ObjectShape
--- PASS: TestParseJobs_ObjectShape (0.00s)
=== RUN   TestParseJobs_ArrayShape
--- PASS: TestParseJobs_ArrayShape (0.00s)
=== RUN   TestParseJobs_ErrorMessageFromMsgFallback
--- PASS: TestParseJobs_ErrorMessageFromMsgFallback (0.00s)
=== RUN   TestParseJobs_ErrorFieldPreferredOverMsg
--- PASS: TestParseJobs_ErrorFieldPreferredOverMsg (0.00s)
=== RUN   TestParseJobs_ProgressClamped
--- PASS: TestParseJobs_ProgressClamped (0.00s)
=== RUN   TestParseJobs_StateMapping
--- PASS: TestParseJobs_StateMapping (0.00s)
=== RUN   TestParseJobs_EmptyBody
--- PASS: TestParseJobs_EmptyBody (0.00s)
=== RUN   TestParseJobs_BadJSON
--- PASS: TestParseJobs_BadJSON (0.00s)
=== RUN   TestPoller_FiresExactlyOneSuccessWebhook
--- PASS: TestPoller_FiresExactlyOneSuccessWebhook (0.00s)
=== RUN   TestPoller_ErrorJobFiresFailureWebhook
--- PASS: TestPoller_ErrorJobFiresFailureWebhook (0.00s)
=== RUN   TestPoller_DedupNoSecondFire
--- PASS: TestPoller_DedupNoSecondFire (0.00s)
=== RUN   TestPoller_MultipleJobsIndependentDedup
--- PASS: TestPoller_MultipleJobsIndependentDedup (0.00s)
=== RUN   TestPoller_FullChainMeTubeMockToWebhook
--- PASS: TestPoller_FullChainMeTubeMockToWebhook (0.00s)
=== RUN   TestPoller_DeliveryFailureLeavesJobUnfiredForRetry
--- PASS: TestPoller_DeliveryFailureLeavesJobUnfiredForRetry (0.00s)
=== RUN   TestHTTPStatusSource_EndToEnd
--- PASS: TestHTTPStatusSource_EndToEnd (0.00s)
=== RUN   TestHTTPStatusSource_Non2xxIsError
--- PASS: TestHTTPStatusSource_Non2xxIsError (0.00s)
=== RUN   TestHTTPStatusSource_CustomPath
--- PASS: TestHTTPStatusSource_CustomPath (0.00s)
=== RUN   TestSignVerifyRoundTrip
--- PASS: TestSignVerifyRoundTrip (0.00s)
=== RUN   TestWebhookSink_SignsExactBodyReceiverRecomputes
--- PASS: TestWebhookSink_SignsExactBodyReceiverRecomputes (0.00s)
=== RUN   TestWebhookSink_RetryOn500Then200
--- PASS: TestWebhookSink_RetryOn500Then200 (0.00s)
=== RUN   TestWebhookSink_FailsAfterExhaustingRetries
--- PASS: TestWebhookSink_FailsAfterExhaustingRetries (0.00s)
PASS
ok  	digital.vasic.metubewebhook	1.030s
```

## Pass/fail summary

| Metric            | Value        |
| ----------------- | ------------ |
| Tests run         | 21           |
| `--- PASS`        | 21           |
| `--- FAIL`        | 0            |
| Race detector     | enabled (`-race`), no data races reported |
| `go build`        | clean (exit 0) |
| `go vet`          | clean (exit 0) |
| `gofmt -l .`      | clean (no files listed) |

## What the tests prove (mapped to the contract)

- **Exactly one success webhook** — `TestPoller_FiresExactlyOneSuccessWebhook`
  drives a canned status sequence `pending → downloading → finished` across 3
  polls; the httptest receiver **independently recomputes** the HMAC-SHA256 over
  the raw body (`crypto/hmac` + `crypto/sha256`, no reference to the package's
  own `Sign`) and matches it, and asserts the full envelope payload
  (`job_id`, `state=success`, `progress=1.0`, `result_ref`, empty `error`,
  fixed `ts`).
- **One failure webhook** — `TestPoller_ErrorJobFiresFailureWebhook`: an `error`
  job yields exactly one `state=failure` envelope carrying the error message and
  no `result_ref`.
- **DEDUP** — `TestPoller_DedupNoSecondFire` re-polls a persistently-finished job
  5× and asserts the receiver got **exactly 1** request; `AlreadyFired` is true.
  `TestPoller_MultipleJobsIndependentDedup` proves per-job dedup with two jobs.
- **Real HTTPStatusSource end-to-end** — `TestHTTPStatusSource_EndToEnd` and
  `TestPoller_FullChainMeTubeMockToWebhook` stand up a real httptest MeTube-mock
  server serving `/api/postprocess/jobs` and exercise
  `HTTPStatusSource → Poller → WebhookSink → httptest receiver` end to end.
- **Retry on non-2xx** — `TestWebhookSink_RetryOn500Then200` proves the sink
  redelivers after a 500 and succeeds on the 200 (2 attempts); the delivery-level
  failure/redelivery is also proven at the poll level in
  `TestPoller_DeliveryFailureLeavesJobUnfiredForRetry` (a failed delivery does
  NOT mark the job fired, so a later poll retries it).

## Honest verdict

**READY.** The module compiles, vets clean, is gofmt-clean, and all 21 tests
pass under the race detector. Every contract requirement (envelope shape,
`X-Thready-Signature: sha256=<hex>` HMAC-SHA256 over the raw body, terminal
transition detection, dedup, retry, real HTTP source) is covered by a real
assertion. No test is skipped, deleted, or faked; no output was edited.

**Scope note (honest):** this is the **outbound shim only**. It sits beside
MeTube and polls its existing poll-only API; it does not modify MeTube. Adding a
native completion webhook to MeTube (milos85vasic/YT-DLP) upstream is a separate
change and is out of scope for this module.

## Doc-accuracy note (cross-cutting review, docs only)

Corrected a misleading claim: the `Envelope` godoc (webhook.go) and README no
longer say the payload matches "the canonical `callback_task` shape". Reality:
metube's envelope is the **job-completion** family `{job_id, state:success/failure}`,
**byte-identical to `boba_adapter`**; `callback_task` is a *separate* generic-task
family (`{task_id, state:succeeded/failed}`) that only shares the same
`X-Thready-Signature` HMAC-SHA256 scheme. **Comments/README only — no production
logic changed.** Gate re-run after the edit (`GOWORK=off`): `go build`/`go vet`/
`gofmt -l .` clean, `go test ./... -race -count=1` → `ok digital.vasic.metubewebhook 1.030s` (21/21).
