# digital.vasic.downloadmanager

A generic, project-agnostic multi-protocol **Download Manager** for Helix Thready
(gap register `[GAP: 6.3]`). It provides a job queue with a bounded worker pool,
per-job lifecycle state, progress reporting, retry with exponential backoff +
jitter, and a completion-callback hook. Protocol handling is pluggable behind a
`Fetcher` interface: a real HTTP(S) fetcher (segmented + resumable) ships today;
FTP/SMB/NFS/WebDav are honest interface stubs.

- **Module:** `digital.vasic.downloadmanager`
- **Go:** 1.26, **standard library only** (no external dependencies).
- **Self-contained:** does not import any in-house modules. The FTP/SMB/NFS/WebDav
  stubs are the documented reuse points for `digital.vasic.filesystem`; the HTTP
  fetcher is the reuse point for `vasic-digital/http3` — neither is imported here.

## What is real vs. stub

| Capability | Status |
|------------|--------|
| HTTP(S) download | **REAL** — `HTTPFetcher` |
| Ranged / segmented parallel download | **REAL** — N parallel `Range` GETs reassembled via `WriteAt` |
| Resume from an interrupted partial download | **REAL** — per-segment offsets persisted to `<dest>.dlstate` |
| SHA-256 checksum verification | **REAL** — verified before the atomic rename to the final path |
| Job queue, worker pool, state machine | **REAL** — `Manager` |
| Retry (exponential backoff + full jitter + max) | **REAL** |
| Progress + completion callbacks | **REAL** |
| Pause / Resume | **REAL** |
| FTP / SMB / NFS / WebDav | **STUB** — `NewStubFetcher`, returns `ErrNotImplemented` (never faked) |

## Public API

```go
// --- Manager ---
m := downloadmanager.New(downloadmanager.Config{
    Workers:     4,
    MaxRetries:  3,
    BaseBackoff: 200 * time.Millisecond,
    MaxBackoff:  10 * time.Second,
    Registry:    downloadmanager.DefaultRegistry(), // http/https + ftp/smb/nfs/webdav stubs
    OnProgress:  func(u downloadmanager.JobUpdate) { /* monotonic bytes */ },
    OnComplete:  func(u downloadmanager.JobUpdate) { /* fires once, terminal state */ },
})
m.Start()
defer m.Shutdown(context.Background())

id, err := m.Enqueue(downloadmanager.TaskSpec{
    URL:            "https://host/big.bin",
    DestPath:       "/tmp/big.bin",
    Segments:       8,
    ExpectedSHA256: "…hex…", // optional integrity check
})

update, err := m.Wait(ctx, id)   // blocks until terminal (succeeded/failed/dead)
snap, ok := m.Status(id)         // non-blocking snapshot
_ = m.Pause(id)                  // suspend; on-disk partial data is preserved
_ = m.Resume(id)                 // continue from where it stopped
```

### Job states (`State`)

`queued → running` then one of:
- `succeeded` — completed and (if requested) checksum-verified. **Terminal.**
- `retrying` — a transient failure; waiting on backoff before the next attempt.
- `failed` — a **permanent** error (unsupported scheme, HTTP 4xx, checksum
  mismatch, `ErrNotImplemented`); not retried. **Terminal.**
- `dead` — retryable failures exhausted the retry budget. **Terminal.**
- `paused` — suspended by `Pause`; resumable via `Resume`.

`OnComplete` fires exactly once, on any terminal state.

### Fetcher interface & registry

```go
type Fetcher interface {
    Schemes() []string
    Fetch(ctx context.Context, req FetchRequest) (FetchResult, error)
}

reg := downloadmanager.NewRegistry()
reg.Register(downloadmanager.NewHTTPFetcher())          // http, https
reg.Register(downloadmanager.NewStubFetcher("ftp"))     // ErrNotImplemented
f, ok := reg.Fetcher("https")
```

Retryability is expressed through the error: wrap with `downloadmanager.Permanent(err)`
for non-retryable failures; `downloadmanager.IsPermanent(err)` reports it, and
`ErrNotImplemented` is always permanent.

## How resume works

Each job downloads to `<dest>.part` (preallocated for ranged transfers) and
records per-segment committed offsets in `<dest>.dlstate`. On interruption the
state is flushed; a subsequent `Fetch`/`Resume` reloads it, validates it against
the server (size + `ETag`), and re-requests only the missing byte ranges. On
success the checksum is verified and `<dest>.part` is atomically renamed to
`<dest>`. Servers that do not support `Range` fall back to a single stream.

## Running the tests

```
cd implementation/download_manager
go build ./... && go vet ./... && go test ./... -v -race -count=1
```

All HTTP tests serve **real bytes** via `net/http/httptest` with genuine Range
support (not response mocks). See `EVIDENCE.md` for captured output. Current
result: **12/12 PASS** under `-race`.

## Files

| File | Contents |
|------|----------|
| `errors.go` | `ErrNotImplemented`, `PermanentError`, `Permanent`, `IsPermanent` |
| `fetcher.go` | `Fetcher`, `Registry`, `FetchRequest`/`FetchResult`, stub fetcher, `DefaultRegistry` |
| `http_fetcher.go` | `HTTPFetcher` — probe, segmented parallel Range download, resume, checksum |
| `manager.go` | `Manager`, `Config`, `TaskSpec`, `JobUpdate`, state machine, retry, callbacks, pause/resume |
| `*_test.go` | Real-server tests (`testutil_test.go` provides the Range server) |
| `EVIDENCE.md` | Captured build/vet/test output |
