# EVIDENCE — Boba callback-normalization adapter

Module: `digital.vasic.bobaadapter`
Gap closed: **[GAP: 6.4]** — Boba-Base (`milos85vasic/Boba-Base`, a torrent
meta-search / download engine) **already** exposes callbacks (an SSE
`result_found` event stream and a hook-registration endpoint
`POST /api/v1/hooks`), but its callback shape is **bespoke**. This adapter does
**not** add callbacks to Boba; it **normalizes** Boba's existing events into the
one shared Helix Thready callback envelope
`{job_id, state, progress, result_ref, error, ts}` — **byte-identical to**
`implementation/metube_webhook`, which uses the same shared
`{job_id, state, progress, result_ref, error, ts}` shape. (`implementation/callback_task`
carries the same six fields but names its first one `task_id`, so its bytes
differ — a pre-existing sibling divergence, out of scope here.) It signs the
envelope with `X-Thready-Signature: sha256=<hex>` (HMAC-SHA256 over the exact raw
request body, event-bus contract §9) and fires it to a downstream sink.

Physical evidence — the exact commands and their real, unedited output.
Captured on host `linux/amd64`.

## Build/test discipline

A parent `implementation/go.work` exists that does **not** list this directory,
so **every** Go command is run with `GOWORK=off` (the module is standalone and
imports no siblings — standard library only).

## Command sequence

```
cd implementation/boba_adapter
GOWORK=off go build ./...
GOWORK=off go vet ./...
GOWORK=off gofmt -l .
GOWORK=off go test ./... -v -race -count=1
```

## go version

```
go version go1.26.4-X:nodwarf5 linux/amd64
```

## GOWORK=off go build ./...

```
(build OK — no output, exit 0)
```

## GOWORK=off go vet ./...

```
(vet OK — no output, exit 0)
```

## GOWORK=off gofmt -l .

```
(clean — no files listed, exit 0)
```

## GOWORK=off go test ./... -v -race -count=1

```
=== RUN   TestBridge_DownloadCompleteFiresExactlyOneSignedWebhook
--- PASS: TestBridge_DownloadCompleteFiresExactlyOneSignedWebhook (0.00s)
=== RUN   TestBridge_DownloadErrorFiresFailureWebhook
--- PASS: TestBridge_DownloadErrorFiresFailureWebhook (0.00s)
=== RUN   TestBridge_DedupSameResultIDFiresOnce
--- PASS: TestBridge_DedupSameResultIDFiresOnce (0.00s)
=== RUN   TestBridge_DistinctResultIDsFireIndependently
--- PASS: TestBridge_DistinctResultIDsFireIndependently (0.00s)
=== RUN   TestBridge_DeliveryFailureLeavesResultUnfiredForRetry
--- PASS: TestBridge_DeliveryFailureLeavesResultUnfiredForRetry (0.00s)
=== RUN   TestBridge_FullChainSSEToWebhook
--- PASS: TestBridge_FullChainSSEToWebhook (0.00s)
=== RUN   TestParseSSE_ResultFoundFrame
--- PASS: TestParseSSE_ResultFoundFrame (0.00s)
=== RUN   TestParseSSE_MultiDataLinesJoined
--- PASS: TestParseSSE_MultiDataLinesJoined (0.00s)
=== RUN   TestParseSSE_CommentFrameIsNoData
--- PASS: TestParseSSE_CommentFrameIsNoData (0.00s)
=== RUN   TestParseSSE_EventNameFallbackWhenBodyOmitsType
--- PASS: TestParseSSE_EventNameFallbackWhenBodyOmitsType (0.00s)
=== RUN   TestParseHookPayload_DownloadComplete
--- PASS: TestParseHookPayload_DownloadComplete (0.00s)
=== RUN   TestParseHookPayload_DownloadErrorStatusFallback
--- PASS: TestParseHookPayload_DownloadErrorStatusFallback (0.00s)
=== RUN   TestParseHookPayload_EmptyIsError
--- PASS: TestParseHookPayload_EmptyIsError (0.00s)
=== RUN   TestParseHookPayload_BadJSONIsError
--- PASS: TestParseHookPayload_BadJSONIsError (0.00s)
=== RUN   TestNormProgress_ClampAndPercent
--- PASS: TestNormProgress_ClampAndPercent (0.00s)
=== RUN   TestSSEReader_ParsesMultiFrameStream
--- PASS: TestSSEReader_ParsesMultiFrameStream (0.00s)
=== RUN   TestSSEReader_HandlerErrorStopsStream
--- PASS: TestSSEReader_HandlerErrorStopsStream (0.00s)
=== RUN   TestSSEReader_Non2xxIsError
--- PASS: TestSSEReader_Non2xxIsError (0.00s)
=== RUN   TestHTTPHookRegistrar_RegistersCallbackURL
--- PASS: TestHTTPHookRegistrar_RegistersCallbackURL (0.00s)
=== RUN   TestHTTPHookRegistrar_Non2xxIsError
--- PASS: TestHTTPHookRegistrar_Non2xxIsError (0.00s)
=== RUN   TestHTTPHookRegistrar_HookIDFallbackKey
--- PASS: TestHTTPHookRegistrar_HookIDFallbackKey (0.00s)
=== RUN   TestSignVerifyRoundTrip
--- PASS: TestSignVerifyRoundTrip (0.00s)
=== RUN   TestWebhookSink_SignsExactBodyReceiverRecomputes
--- PASS: TestWebhookSink_SignsExactBodyReceiverRecomputes (0.00s)
=== RUN   TestWebhookSink_RetryOn500Then200
--- PASS: TestWebhookSink_RetryOn500Then200 (0.00s)
=== RUN   TestWebhookSink_FailsAfterExhaustingRetries
--- PASS: TestWebhookSink_FailsAfterExhaustingRetries (0.00s)
PASS
ok  	digital.vasic.bobaadapter	1.032s
```

## Pass/fail summary

| Metric            | Value        |
| ----------------- | ------------ |
| Tests run         | 25           |
| `--- PASS`        | 25           |
| `--- FAIL`        | 0            |
| Race detector     | enabled (`-race`), no data races reported |
| `go build`        | clean (exit 0) |
| `go vet`          | clean (exit 0) |
| `gofmt -l .`      | clean (no files listed) |

## What the tests prove (mapped to the contract)

- **Parse canned Boba SSE frames** — `TestParseSSE_ResultFoundFrame` (nested
  `result` object → `result_found`, non-terminal), `TestParseSSE_MultiDataLinesJoined`
  (multiple `data:` lines joined with `\n` per the SSE spec → `download_complete`,
  terminal, progress forced to 1.0), `TestParseSSE_CommentFrameIsNoData`
  (keep-alive comment frame → `errNoData`, skipped), and
  `TestParseSSE_EventNameFallbackWhenBodyOmitsType` (event kind taken from the
  SSE `event:` name when the JSON omits it; percent progress normalized).
- **Parse canned Boba hook payloads** — `TestParseHookPayload_DownloadComplete`
  (`save_path` result-ref spelling), `TestParseHookPayload_DownloadErrorStatusFallback`
  (kind derived from `status:"failed"`, error text from `message`, id from
  `infohash`), plus empty/bad-JSON rejection. `TestNormProgress_ClampAndPercent`
  pins the progress normalization (percent detection + clamp).
- **A download-complete fires exactly ONE standard HMAC webhook** —
  `TestBridge_DownloadCompleteFiresExactlyOneSignedWebhook`: a non-terminal
  `result_found` for the same id fires nothing; the terminal `download_complete`
  fires one POST to an httptest receiver that **independently recomputes** the
  HMAC-SHA256 over the raw body (`crypto/hmac` + `crypto/sha256`, with no
  reference to the package's own `Sign`) and matches it, and asserts the full
  envelope (`job_id`, `state=success`, `progress=1.0`, `result_ref`, empty
  `error`, fixed `ts`). `TestBridge_DownloadErrorFiresFailureWebhook` proves the
  `state=failure` path (error message carried, no `result_ref`).
- **Dedup (same id → one fire)** — `TestBridge_DedupSameResultIDFiresOnce`
  delivers the identical terminal event 5× and asserts the receiver got
  **exactly 1** request and `AlreadyFired` is true;
  `TestBridge_DistinctResultIDsFireIndependently` proves per-id dedup with two
  ids interleaved (`A,B,A,B` → 2 requests).
- **SSE reader parses a multi-frame stream from a live server** —
  `TestSSEReader_ParsesMultiFrameStream` stands up a real `httptest` SSE server
  emitting `result_found` + a comment keep-alive + `download_progress` +
  `download_complete`, and asserts the reader yields exactly the three real
  events (skipping the comment) with correct types/fields.
  `TestSSEReader_HandlerErrorStopsStream` and `TestSSEReader_Non2xxIsError` cover
  early-stop and error paths.
- **Retry 500-then-200** — `TestWebhookSink_RetryOn500Then200` proves the sink
  redelivers after a 500 and succeeds on the 200 (exactly 2 attempts);
  `TestWebhookSink_FailsAfterExhaustingRetries` proves it errors after
  exhausting retries; `TestBridge_DeliveryFailureLeavesResultUnfiredForRetry`
  proves a failed delivery does **not** mark the result fired, so a later
  delivery of the same event retries it.
- **Full chain** — `TestBridge_FullChainSSEToWebhook` wires a real `httptest`
  Boba SSE server → `SSEReader` → `Bridge.Consume` → `WebhookSink` → a real
  `httptest` receiver that independently recomputes the HMAC, proving only the
  terminal event fires and the standard envelope arrives signed.
- **Real hook registrar** — `TestHTTPHookRegistrar_RegistersCallbackURL` stands
  up a real server, asserts the `POST /api/v1/hooks` path, the registered
  callback URL and event filter in the body, and the returned hook id; the
  non-2xx and `hook_id` fallback-key cases are also covered.

## Honesty / inference note (NO BLUFF)

Boba's callbacks are **VERIFIED** to exist (SSE `result_found` +
`POST /api/v1/hooks`). The concrete JSON **field names** consumed by the wire
structs (`event`, `search_id`, `result{infohash,title,tracker,magnet,...}`,
`id`, `status`, `progress`, `path`/`save_path`/`file`, `error`/`message`) and
the hook request/response shapes are **[inferred]** from Boba's torrent
meta-search domain and are marked as such inline in `event.go` and `source.go`.
The parser is deliberately **lenient** (several accepted spellings per field), so
aligning it with Boba's exact keys is a one-line change per field and does not
touch the normalization / signing / delivery logic. The SSE endpoint path
(`DefaultSSEPath`) is [inferred] and overridable; the hooks path
(`DefaultHooksPath = /api/v1/hooks`) matches Boba's documented verb.

No test is skipped, deleted, or faked; no command output was edited.

## Verdict

**READY.** The module compiles, vets clean, is gofmt-clean, and all **25** tests
pass under the race detector with no data races. Every contract requirement —
SSE + hook parsing, the shared `{job_id,state,progress,result_ref,error,ts}`
envelope, `X-Thready-Signature: sha256=<hex>` HMAC-SHA256 over the raw body,
per-result dedup, non-2xx retry, a real `net/http` SSE reader and hook
registrar, and a full end-to-end chain — is covered by a real assertion. The
only honestly-flagged caveat is that Boba's bespoke JSON field names are
inferred (lenient, single-line to correct) rather than confirmed against a live
Boba build.

## Review fixes

Two review findings addressed (docs-only + one test; no production code changed):

1. **Accuracy fix (docs).** The docs previously called the envelope "identical to
   `callback_task` **and** `metube_webhook`." Verified against the siblings:
   `metube_webhook/webhook.go` leads its `Envelope` with `json:"job_id"` and uses
   the `success`/`failure` state vocabulary — **byte-identical** to Boba's.
   `callback_task/task.go` leads its `Envelope` with `json:"task_id"` (and uses a
   `succeeded`/`failed` vocabulary), so its bytes **differ**. The wording in
   `event.go` (package doc), `webhook.go` (`SignatureHeader`, `CompletionState`,
   `Envelope` comments), `EVIDENCE.md` and `README.md` was corrected to state:
   byte-identical to `metube_webhook`; shares the
   `{job_id, state, progress, result_ref, error, ts}` shape; `callback_task`'s
   `task_id` is a pre-existing sibling divergence, out of scope here. **Module code
   is unchanged** — it already implements the `{job_id,...}` shape per spec.
2. **Coverage fix (test).** `EnvelopeFor` sets
   `result_ref = firstNonEmpty(ev.Path, ev.Magnet, ev.Torrent)`, but every prior
   test set `Path`, leaving the Magnet/Torrent fallback untested. Added
   `TestEnvelopeFor_ResultRefFallsBackMagnetThenTorrent` (table-driven, 3 subtests):
   Path wins when present; falls back to `Magnet` when `Path` is empty; falls back
   to `Torrent` when both `Path` and `Magnet` are empty. No existing test was
   weakened or removed.

Re-run — real, unedited output. `go version go1.26.4-X:nodwarf5 linux/amd64`.

### GOWORK=off go build ./...

```
(build OK — no output, exit 0)
```

### GOWORK=off go vet ./...

```
(vet OK — no output, exit 0)
```

### GOWORK=off gofmt -l .

```
(clean — no files listed, exit 0)
```

### GOWORK=off go test ./... -v -race -count=1

```
=== RUN   TestBridge_DownloadCompleteFiresExactlyOneSignedWebhook
--- PASS: TestBridge_DownloadCompleteFiresExactlyOneSignedWebhook (0.00s)
=== RUN   TestBridge_DownloadErrorFiresFailureWebhook
--- PASS: TestBridge_DownloadErrorFiresFailureWebhook (0.00s)
=== RUN   TestBridge_DedupSameResultIDFiresOnce
--- PASS: TestBridge_DedupSameResultIDFiresOnce (0.00s)
=== RUN   TestBridge_DistinctResultIDsFireIndependently
--- PASS: TestBridge_DistinctResultIDsFireIndependently (0.00s)
=== RUN   TestBridge_DeliveryFailureLeavesResultUnfiredForRetry
--- PASS: TestBridge_DeliveryFailureLeavesResultUnfiredForRetry (0.00s)
=== RUN   TestBridge_FullChainSSEToWebhook
--- PASS: TestBridge_FullChainSSEToWebhook (0.00s)
=== RUN   TestParseSSE_ResultFoundFrame
--- PASS: TestParseSSE_ResultFoundFrame (0.00s)
=== RUN   TestParseSSE_MultiDataLinesJoined
--- PASS: TestParseSSE_MultiDataLinesJoined (0.00s)
=== RUN   TestParseSSE_CommentFrameIsNoData
--- PASS: TestParseSSE_CommentFrameIsNoData (0.00s)
=== RUN   TestParseSSE_EventNameFallbackWhenBodyOmitsType
--- PASS: TestParseSSE_EventNameFallbackWhenBodyOmitsType (0.00s)
=== RUN   TestParseHookPayload_DownloadComplete
--- PASS: TestParseHookPayload_DownloadComplete (0.00s)
=== RUN   TestParseHookPayload_DownloadErrorStatusFallback
--- PASS: TestParseHookPayload_DownloadErrorStatusFallback (0.00s)
=== RUN   TestParseHookPayload_EmptyIsError
--- PASS: TestParseHookPayload_EmptyIsError (0.00s)
=== RUN   TestParseHookPayload_BadJSONIsError
--- PASS: TestParseHookPayload_BadJSONIsError (0.00s)
=== RUN   TestNormProgress_ClampAndPercent
--- PASS: TestNormProgress_ClampAndPercent (0.00s)
=== RUN   TestSSEReader_ParsesMultiFrameStream
--- PASS: TestSSEReader_ParsesMultiFrameStream (0.00s)
=== RUN   TestSSEReader_HandlerErrorStopsStream
--- PASS: TestSSEReader_HandlerErrorStopsStream (0.00s)
=== RUN   TestSSEReader_Non2xxIsError
--- PASS: TestSSEReader_Non2xxIsError (0.00s)
=== RUN   TestHTTPHookRegistrar_RegistersCallbackURL
--- PASS: TestHTTPHookRegistrar_RegistersCallbackURL (0.00s)
=== RUN   TestHTTPHookRegistrar_Non2xxIsError
--- PASS: TestHTTPHookRegistrar_Non2xxIsError (0.00s)
=== RUN   TestHTTPHookRegistrar_HookIDFallbackKey
--- PASS: TestHTTPHookRegistrar_HookIDFallbackKey (0.00s)
=== RUN   TestSignVerifyRoundTrip
--- PASS: TestSignVerifyRoundTrip (0.00s)
=== RUN   TestEnvelopeFor_ResultRefFallsBackMagnetThenTorrent
=== RUN   TestEnvelopeFor_ResultRefFallsBackMagnetThenTorrent/path_wins_when_present
=== RUN   TestEnvelopeFor_ResultRefFallsBackMagnetThenTorrent/falls_back_to_magnet_when_path_is_empty
=== RUN   TestEnvelopeFor_ResultRefFallsBackMagnetThenTorrent/falls_back_to_torrent_when_path_and_magnet_are_empty
--- PASS: TestEnvelopeFor_ResultRefFallsBackMagnetThenTorrent (0.00s)
    --- PASS: TestEnvelopeFor_ResultRefFallsBackMagnetThenTorrent/path_wins_when_present (0.00s)
    --- PASS: TestEnvelopeFor_ResultRefFallsBackMagnetThenTorrent/falls_back_to_magnet_when_path_is_empty (0.00s)
    --- PASS: TestEnvelopeFor_ResultRefFallsBackMagnetThenTorrent/falls_back_to_torrent_when_path_and_magnet_are_empty (0.00s)
=== RUN   TestWebhookSink_SignsExactBodyReceiverRecomputes
--- PASS: TestWebhookSink_SignsExactBodyReceiverRecomputes (0.00s)
=== RUN   TestWebhookSink_RetryOn500Then200
--- PASS: TestWebhookSink_RetryOn500Then200 (0.00s)
=== RUN   TestWebhookSink_FailsAfterExhaustingRetries
--- PASS: TestWebhookSink_FailsAfterExhaustingRetries (0.00s)
PASS
ok  	digital.vasic.bobaadapter	1.031s
```

**Result:** 26 test functions (28 including subtests), all `--- PASS`, 0 `--- FAIL`,
race detector enabled with no data races; build / vet / gofmt all clean.
**Verdict: READY** (unchanged).
