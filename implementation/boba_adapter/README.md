# boba_adapter — Boba callback-normalization adapter

`digital.vasic.bobaadapter` · Go 1.26 · standard library only

## Purpose

Boba-Base (the `milos85vasic/Boba-Base` torrent meta-search / download engine)
**already** emits callbacks:

- an **SSE** event stream (`result_found`, download progress/completion frames),
- a **hook registration** endpoint, `POST /api/v1/hooks`, that pushes events to a
  registered URL.

But Boba's callback shape is **bespoke**. This is the documented gap
**[GAP: 6.4]**. This module does **not** add callbacks to Boba — it
**normalizes** Boba's existing events into the **one shared Helix Thready
callback envelope** `{job_id, state, progress, result_ref, error, ts}`
(**byte-identical to** `metube_webhook`, which uses the same shared
`{job_id, state, progress, result_ref, error, ts}` shape — `callback_task`
carries the same six fields but names its first one `task_id`, a pre-existing
sibling divergence out of scope here), signs it with
`X-Thready-Signature: sha256=<hex>` (HMAC-SHA256 over the exact raw body), and
fires it to a downstream sink. Result: Boba, MeTube and the Download Manager all
speak **one** callback contract.

```
Boba SSE / hook  ──► ParseSSE / ParseHookPayload ──► BobaEvent ──► Bridge
                                                                     │  (dedup per result id,
                                                                     │   terminal events only)
                                                                     ▼
                                                        Envelope {job_id,state,…}
                                                                     │  HMAC-SHA256(raw body)
                                                                     ▼
                                              POST + X-Thready-Signature ──► downstream sink
```

> **Inference note (honest):** Boba's callbacks are **VERIFIED** to exist. The
> concrete JSON **field names** below are **[inferred]** from Boba's torrent
> meta-search domain (and marked so inline). The parser is lenient — several
> accepted spellings per field — so matching Boba's exact keys is a one-line
> change per field that does not touch the normalization / signing / delivery
> logic.

## Boba events (input)

### SSE frame

```
event: result_found
data: {"search_id":"s1","query":"ubuntu",
       "result":{"infohash":"HASH1","title":"Ubuntu 24.04","tracker":"lt","magnet":"magnet:?...","seeders":42}}

event: download_complete
data: {"id":"HASH1","status":"complete","path":"/downloads/ubuntu.iso"}
```

`ParseSSE([]byte)` parses **one** frame (multiple `data:` lines are joined with
`\n` per the SSE spec; comment / keep-alive frames yield an internal skip). The
`SSEReader` splits a live stream into frames on blank lines and parses each.

### Hook payload

```json
{ "event": "download_complete", "id": "HASH1", "status": "completed",
  "progress": 1.0, "save_path": "/downloads/ubuntu.iso" }
```

`ParseHookPayload([]byte)` maps a single hook POST body. Recognized event kinds
(from `event`/`type`, falling back to `status`):

| normalized `BobaEventType` | terminal | fires a callback |
| -------------------------- | :------: | :--------------: |
| `result_found`             |    no    |        no        |
| `download_started`         |    no    |        no        |
| `download_progress`        |    no    |        no        |
| `download_complete`        |   yes    |  yes → success   |
| `download_error`           |   yes    |  yes → failure   |

`progress` is normalized to `0.0–1.0` (a value `>1` is treated as a percent and
divided by 100, then clamped). The result id is Boba's explicit `id`, falling
back to the torrent `infohash`; it is used as the callback `job_id` **and** the
dedup key.

## Callback envelope (output)

The outbound POST body is byte-identical to `metube_webhook` and uses the shared
`{job_id, state, progress, result_ref, error, ts}` completion shape:

```json
{
  "job_id": "HASH1",
  "state": "success",
  "progress": 1.0,
  "result_ref": "/downloads/ubuntu.iso",
  "error": "",
  "ts": "2023-11-14T22:13:20Z"
}
```

`state` is `success` (from `download_complete`) or `failure` (from
`download_error`). For a success, `result_ref` is the completed download's
reference — the local path, falling back to the magnet / `.torrent` reference.
For a failure, `error` carries the message and `result_ref` is empty.

### Signature

Each POST carries:

```
X-Thready-Signature: sha256=<hex>
Content-Type: application/json
```

`<hex>` is the lowercase HMAC-SHA256 digest of the **exact raw request body**
under the shared secret. A receiver verifies by recomputing the HMAC over the
bytes it received and constant-time comparing (`Verify`).

## Package shape

| Type / func                              | Role                                                                    |
| ---------------------------------------- | ----------------------------------------------------------------------- |
| `BobaEvent`, `BobaEventType`             | Normalized, provider-neutral Boba event + kind vocabulary.              |
| `ParseSSE([]byte)`                       | One Boba SSE frame → `BobaEvent` (pure, offline-testable).              |
| `ParseHookPayload([]byte)`               | One Boba hook body → `BobaEvent` (pure, offline-testable).             |
| `EventSource` (interface)                | Streaming seam the Bridge consumes.                                     |
| `SSEReader`                              | Real `net/http` SSE reader (parses `data:` frames from a live stream).  |
| `HTTPHookRegistrar`                      | Real `POST /api/v1/hooks` callback-URL registration.                    |
| `Envelope`, `EnvelopeFor`                | Shared outbound completion payload + builder.                           |
| `Sign` / `SignatureValue` / `Verify`     | HMAC-SHA256 over the raw body.                                          |
| `Notifier` (interface)                   | Delivery seam.                                                          |
| `WebhookSink`                            | HMAC-signed HTTP POST delivery with retry + back-off.                   |
| `Bridge`                                 | Terminal-event → envelope → sign → fire; per-id dedup; `Handle`/`Consume`. |

## Usage

```go
// 1. Register the downstream callback URL with Boba (optional; if you consume
//    Boba's SSE stream directly you can skip this).
reg := &bobaadapter.HTTPHookRegistrar{BaseURL: "http://boba:8080"}
_, _ = reg.Register(ctx, "https://thready/hooks/boba", "download_complete", "download_error")

// 2. Bridge: normalize Boba terminal events → one signed standard callback.
sink := &bobaadapter.WebhookSink{
    URL:        "https://orchestrator/hooks/boba",
    Secret:     []byte(os.Getenv("THREADY_WEBHOOK_SECRET")),
    MaxRetries: 3,
}
b := bobaadapter.NewBridge(sink)

// 2a. Drive it from Boba's SSE stream end to end…
src := &bobaadapter.SSEReader{BaseURL: "http://boba:8080", Query: "q=ubuntu"}
_ = b.Consume(ctx, src)

// 2b. …or feed hook payloads received on your own endpoint:
ev, err := bobaadapter.ParseHookPayload(rawHookBody)
if err == nil {
    _, _ = b.Handle(ctx, ev) // fires exactly once per distinct result id
}
```

## Run the tests

```
cd implementation/boba_adapter
GOWORK=off go build ./...
GOWORK=off go vet ./...
GOWORK=off gofmt -l .
GOWORK=off go test ./... -v -race -count=1
```

> **`GOWORK=off` is required.** The parent `implementation/go.work` does not list
> this directory; the module is standalone and imports no siblings (stdlib
> only), so every Go command must disable the workspace.

See `EVIDENCE.md` for the captured, unedited output (**25/25** tests pass under
`-race`).
