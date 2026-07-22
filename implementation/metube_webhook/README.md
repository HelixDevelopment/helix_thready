# metube_webhook — MeTube outbound-completion-webhook shim

`digital.vasic.metubewebhook` · Go 1.26 · standard library only

## Purpose

MeTube (the `milos85vasic/YT-DLP` fork) tracks download / post-process jobs but
is **poll-only**: it exposes

- `GET /api/postprocess/status`
- `GET /api/postprocess/jobs`

and has **no outbound completion webhook**, so an external orchestrator cannot be
notified when a job finishes. This is the documented gap **[GAP: 6.5]**.

This module is the **shim** that closes it, without touching MeTube:

1. **Polls** MeTube's `/api/postprocess/jobs` on an interval.
2. **Detects transitions to a terminal state** — `finished → success`,
   `error → failure`.
3. For each **newly-terminal** job, **fires one** standardized, HMAC-signed
   completion webhook (HTTP POST) to a configured sink URL.
4. **De-duplicates**: a job is never notified twice (a `fired` set keyed on job
   ID); a job that stays terminal across many polls fires exactly once.
5. **Retries** delivery on transport errors / non-2xx responses with back-off.

> Scope: this is the **outbound shim only**. Adding a native completion webhook
> to MeTube upstream is a separate change.

## MeTube poll API (input)

`GET /api/postprocess/jobs` is expected to return the current job set as JSON.
Two shapes are accepted (object or bare array):

```json
{ "jobs": [
  { "id": "vid1", "status": "downloading", "percent": 42.5, "filename": "clip.mp4" },
  { "id": "vid2", "status": "finished",    "filename": "/downloads/vid2.mp4" },
  { "id": "vid3", "status": "error",       "msg": "ffmpeg exited 1" }
] }
```

Recognized `status` values → normalized `JobState`:
`pending`, `downloading`, `postprocessing`, `finished` (terminal, success),
`error` (terminal, failure). `percent` (0–100) is normalized to `progress`
(0.0–1.0); the error text is read from `error`, falling back to `msg`.

## Webhook envelope (output)

The outbound POST body matches the canonical `callback_task` completion shape
`{job_id, state, progress, result_ref, error, ts}`:

```json
{
  "job_id": "vid2",
  "state": "success",
  "progress": 1.0,
  "result_ref": "/downloads/vid2.mp4",
  "error": "",
  "ts": "2023-11-14T22:13:20Z"
}
```

`state` is `success` (from `finished`) or `failure` (from `error`). For a
failure, `error` carries the message and `result_ref` is empty.

### Signature

Each POST carries:

```
X-Thready-Signature: sha256=<hex>
Content-Type: application/json
```

`<hex>` is the lowercase HMAC-SHA256 digest of the **exact raw request body**
under the shared secret. A receiver verifies by recomputing the HMAC over the
bytes it received and constant-time comparing (see `Verify`).

## Package shape

| Type / func                     | Role                                                         |
| ------------------------------- | ------------------------------------------------------------ |
| `JobStatus`, `JobState`         | Normalized MeTube job snapshot + state vocabulary.           |
| `ParseJobs([]byte)`             | MeTube JSON → `[]JobStatus` (pure, offline-testable).        |
| `StatusSource` (interface)      | Seam the poller reads job statuses from.                     |
| `HTTPStatusSource`              | Real `net/http` GET of MeTube's `/api/postprocess/jobs`.     |
| `Envelope`, `EnvelopeFor`       | Outbound completion payload + builder.                       |
| `Sign` / `SignatureValue` / `Verify` | HMAC-SHA256 over the raw body.                          |
| `Notifier` (interface)          | Delivery seam.                                               |
| `WebhookSink`                   | HMAC-signed HTTP POST delivery with retry + back-off.        |
| `Poller`                        | Poll → detect terminal → dedup → fire; `Poll` and `Run`.     |

## Usage

```go
src := &metubewebhook.HTTPStatusSource{BaseURL: "http://metube:8081"}
sink := &metubewebhook.WebhookSink{
    URL:        "https://orchestrator/hooks/metube",
    Secret:     []byte(os.Getenv("THREADY_WEBHOOK_SECRET")),
    MaxRetries: 3,
}
p := metubewebhook.NewPoller(src, sink)

// Poll forever every 2s; per-cycle errors go to the callback.
_ = p.Run(ctx, 2*time.Second, func(err error) { log.Println("poll:", err) })
```

## Run the tests

```
cd implementation/metube_webhook
go build ./...
go vet ./...
gofmt -l .
go test ./... -v -race -count=1
```

See `EVIDENCE.md` for the captured, unedited output (21/21 tests pass under
`-race`).
