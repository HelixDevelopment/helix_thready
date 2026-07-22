# callbacktask — Helix Thready standardized Callback/Task module

`module digital.vasic.callbacktask` · Go 1.26 · **standard library only** · no external deps.

The canonical async-**task** contract for Helix Thready. `callback_task` is the
generic *task* envelope family, keyed on `{task_id, state: succeeded/failed}`.

> **Two envelope families, one HMAC scheme.** `callback_task` is the generic-task
> envelope (`{task_id, succeeded/failed}`). Download / job-completion sources
> (**MeTube** via `metube_webhook`, **Boba** via `boba_adapter`) instead emit the
> separate *job* envelope (`{job_id, success/failure}`, byte-identical between
> those two modules). Both families sign the exact raw body with the **same**
> `X-Thready-Signature` HMAC-SHA256 scheme, so a downstream receiver verifies
> signatures identically and branches on `job_id` vs `task_id` to tell them apart.
> There is deliberately **not** a single wire shape across every provider.

Realizes:
- `docs/public/research/mvp/development/build-new-subsystems.md` §2 — *Standardized
  callback/task module* (item `ATM-030`, gap register `§6.4`/`§6.5`).
- `docs/public/research/mvp/api/event-bus-contract.md` §9 — outbound webhook &
  `X-Thready-Signature` HMAC-SHA256 scheme.

## Purpose

Extract the common 3rd-party async mechanism — **accept task → run async → status
→ outbound (HMAC-signed) webhook on completion → error → retry with back-off →
dead-letter** — into a reusable, decoupled Go module. Delivery is **at-least-once**;
completion is **idempotent** so a redelivered/replayed update never double-completes.

## Model

### `Task`

| Field | Meaning |
|---|---|
| `ID` | Generated unique id (`task-<seq>-<rand>`). |
| `Type` | Job type (e.g. `download`). |
| `Payload` | Opaque JSON bytes. |
| `State` | `queued` · `running` · `succeeded` · `failed` · `retrying` · `dead`. |
| `Attempts` | Failed-attempt counter. |
| `Progress` | `0.0..1.0`. |
| `ResultRef` | Asset Service ref on success. |
| `Error` | Last error message. |
| `CreatedAt` / `UpdatedAt` / `NextRetryAt` | Timestamps (UTC). |

### State machine

```
queued ──Start──▶ running ──Progress──▶ running
                     │
                     ├── Complete ─────────▶ succeeded        (terminal, idempotent)
                     │
                     └── Fail(err):
                          retryable & attempts<max ─▶ retrying ──Start──▶ running …
                          retryable & attempts=max ─▶ dead      (+ dead-letter queue)
                          non-retryable            ─▶ failed    (terminal)
```

`succeeded`, `failed`, `dead` are terminal. Exhausting the retry ceiling
dead-letters the task (retrievable via `Registry.DeadLetters()`).

### Status envelope (stable wire shape)

The outbound envelope always carries the same fields (no `omitempty`), so the
JSON — and the HMAC over it — is deterministic:

```json
{ "task_id": "...", "state": "succeeded", "progress": 1.0,
  "result_ref": "asset:...", "error": "", "ts": "2026-07-22T00:00:00Z" }
```

## API

```go
r := callbacktask.New(
    callbacktask.WithMaxAttempts(8),                 // retry ceiling (default 8)
    callbacktask.WithBackoff(myBackoff),             // attempt→delay (default 100ms·2^(n-1))
    callbacktask.WithNotifier(sink),                 // fires an Envelope on every transition
    callbacktask.WithDeliveryErrorHook(onDropped),   // called when a delivery fails permanently
)

task, _ := r.Submit(ctx, "download", payload)        // → queued
r.Start(ctx, task.ID)                                 // → running
r.Progress(ctx, task.ID, 0.5)                         // running self-loop
r.Complete(ctx, task.ID, "asset:123")                 // → succeeded (idempotent)

// Failure paths:
r.Fail(ctx, id, callbacktask.JobError{Code:"5xx", Message:"upstream", Retryable:true})

env, _  := r.Status(id)          // current Envelope
snap, _ := r.Get(id)             // Task snapshot
dead    := r.DeadLetters()       // []Task whose WORK exhausted its retry ceiling
drops   := r.DeliveryFailures()  // []DeliveryFailure whose DELIVERY exhausted its retries
```

> **Two distinct dead-letters.** `DeadLetters()` holds tasks whose *work* exhausted
> the retry ceiling. `DeliveryFailures()` holds status callbacks whose *delivery* to
> the sink failed permanently (see below). They are independent.

`Registry` is safe for concurrent use. Query/return values are **snapshots**
(no live pointers escape), so callers cannot race the store.

### Outbound webhook (`WebhookSink` implements `Notifier`)

```go
sink := &callbacktask.WebhookSink{
    URL:        "https://orchestrator/v1/processing/callbacks/metube",
    Secret:     hmacSecret,           // per-sink key
    MaxRetries: 8,                     // total attempts = MaxRetries+1
    Backoff:    myBackoff,             // delay before each retry (default exponential)
}
err := sink.Notify(ctx, env)          // POST + sign; retries non-2xx / transport errors
```

Wire it into the `Registry` via `WithNotifier(sink)` for end-to-end
transition → signed webhook delivery.

#### Delivery is synchronous (blocking)

`notify` runs **inline on the transition call path** (`Submit` / `Start` /
`Progress` / `Complete` / `Fail`): the sink's `Notify` is invoked after the
Registry lock is released but **before the transition method returns**. A
`WebhookSink` configured with retries + back-off therefore **blocks the caller**
for the whole retry schedule — worst case `(MaxRetries+1)` HTTP requests plus the
sum of the back-offs. If a state transition must not block on a slow or
unreachable endpoint, wrap the sink in one that enqueues delivery to a background
worker and returns immediately (this module does not do that for you).

#### Delivery dead-lettering (exhausted deliveries are not lost)

`WebhookSink` owns only its **in-flight** retry/back-off; it has **no persistent
dead-letter store** and, once its attempts are exhausted, simply returns an error.
The `Registry` does **not** discard that error. It records the undeliverable
`Envelope` in a delivery dead-letter list, retrievable via
`Registry.DeliveryFailures() []DeliveryFailure`, and — if you registered one —
passes it to the `WithDeliveryErrorHook` callback for external routing/alerting.
So a webhook that exhausts its retries (including a `succeeded` completion) is
**observable**, never silently swallowed.

```go
r := callbacktask.New(
    callbacktask.WithNotifier(sink),
    callbacktask.WithDeliveryErrorHook(func(ctx context.Context, env callbacktask.Envelope, err error) {
        // route to an external DLQ / alert / log — runs synchronously, keep it quick
        log.Printf("callback delivery failed: task=%s state=%s err=%v", env.TaskID, env.State, err)
    }),
)
// ... after transitions:
for _, d := range r.DeliveryFailures() {
    // d.Envelope (what couldn't be delivered), d.Err (why), d.At (when)
}
```

## HMAC scheme

- Header: **`X-Thready-Signature`**.
- Value: **`sha256=<hex>`** where `<hex>` is the lowercase HMAC-SHA256 digest of
  the **raw request body** keyed by the per-sink secret.
- The receiver recomputes the HMAC over the exact received bytes and constant-time
  compares (`hmac.Equal`). Mismatch ⇒ reject (401); the sink treats non-2xx as a
  retryable delivery failure.

Helpers: `callbacktask.Sign(secret, body) string`,
`callbacktask.SignatureValue(secret, body) string` (adds the `sha256=` prefix),
`callbacktask.Verify(secret, body, headerValue) bool`.

## Run the tests

```bash
cd implementation/callback_task
go build ./...
go vet ./...
gofmt -l .        # no output == formatted
go test ./... -v -race -count=1
```

Tests use `net/http/httptest` as a **real** webhook receiver that **independently**
recomputes the HMAC (via `crypto/hmac`+`crypto/sha256` directly, not this module's
helper) and matches it against the header. See [`EVIDENCE.md`](./EVIDENCE.md) for
captured, unedited `go version` + build/vet/test output and the verdict.
