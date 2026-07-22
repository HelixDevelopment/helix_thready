# Max Adapter (`digital.vasic.maxadapter`)

A Go client skeleton for the **Max** messenger (max.ru, by VK; internal codename
**OneMe**), built for Helix Thready's messenger-agnostic ingestion seam.

The honest split this package commits to:

| Half | Status | Where |
|------|--------|-------|
| OneMe history payload → `[]Post` mapping | **REAL, offline, tested** | `parse.go`, `wire.go`, `parse_test.go` |
| Live WebSocket connect / auth / fetch | **[BUILD-NEW] stub** (`ErrNotImplemented`) | `client.go`, `client_test.go` |

There is **no faked network traffic and no claim that live reads work** — this
environment has no Max account and the internal protocol is reverse-engineered
and unverified on-wire. See `PROTOCOL.md` for the research (CONFIRMED vs INFERRED,
with source URLs) and `EVIDENCE.md` for the captured build/test output.

## Purpose

Max, like Telegram, treats a "post" as a **thread**: a root message plus replies
(hashtags and attachments frequently arrive as replies). Thready's `ThreadReader`
assembles that complete post. This adapter's job is to turn Max's on-wire chat
history into the normalized `Post` records the assembler consumes.

The substantive, order-independent, testable core of any such adapter is the
**payload → Post mapper**. That is what is real here.

## The ThreadReader integration seam

`threadreader.MessageSource` (in `../threadreader/source.go`) is:

```go
type MessageSource interface {
    FetchThread(ctx context.Context, threadID string) ([]Post, error)
}
```

`OneMeClient` is shaped to satisfy it:

- `MaxClient` interface: `Connect`, `Authenticate(Credentials{Phone,Token})`,
  `FetchThreadHistory(ctx, threadID) ([]Post, error)`.
- `OneMeClient.FetchThread(ctx, threadID)` delegates to `FetchThreadHistory`, so
  it matches `MessageSource.FetchThread` exactly.

This package is **stdlib-only** and deliberately does **not** import the
`threadreader` module (they are separate Go modules). `Post`/`Attachment` mirror
`threadreader.Post`/`threadreader.Attachment` field-for-field, so the bridge is a
trivial struct copy in whichever module wires them together, e.g.:

```go
func toThreadReader(p maxadapter.Post) threadreader.Post {
    tr := threadreader.Post{
        ID: p.ID, ThreadID: p.ThreadID, ParentID: p.ParentID,
        AuthorID: p.AuthorID, Text: p.Text,
        TimestampUnix: p.TimestampUnix, IsForwarded: p.IsForwarded,
    }
    for _, a := range p.Attachments {
        tr.Attachments = append(tr.Attachments, threadreader.Attachment(a))
    }
    return tr
}
```

Once a live transport exists, `OneMeClient.FetchThreadHistory` calls
`ParseHistory` on the opcode-49 response and the client drops straight into the
`MessageSource` slot.

## What `ParseHistory` does

`ParseHistory([]byte) ([]Post, error)` accepts the OneMe opcode-49 history
response in any of three shapes (full WebSocket frame, bare `payload`, or bare
message array) and maps each message:

- `id` → `ID` (numeric **or** string; large 64-bit ids kept digit-exact)
- enclosing `chatId` → `ThreadID`
- reply parent → `ParentID` (`link.messageId` when `link.type=="REPLY"`, else
  `prevMessageId`)
- `sender` → `AuthorID`, `text` → `Text`
- `time` (epoch ms) → `TimestampUnix` (Unix seconds)
- `link.type=="FORWARD"` → `IsForwarded` (cross-chat; does not set `ParentID`)
- `attaches[]` (`_type`-discriminated) → `Attachments` (id, filename, inferred
  MIME; `SHA256` empty — OneMe sends no hash)

Malformed JSON is a real error; missing optional fields degrade to zero values.

## Files

- `models.go` — `Post`, `Attachment`, `Credentials`, `MaxClient`, `ErrNotImplemented`.
- `parse.go` — `ParseHistory` and the mapping logic (the real core).
- `wire.go` — on-wire structs + `flexID` (precise number-or-string id decoding).
- `client.go` — `OneMeClient`, the `[BUILD-NEW]` live stub implementing `MaxClient`.
- `parse_test.go` / `client_test.go` — offline TDD tests.
- `testdata/history_frame.json` — the representative canned opcode-49 payload.
- `PROTOCOL.md` — protocol research with citations and CONFIRMED/INFERRED marks.
- `EVIDENCE.md` — captured build/vet/gofmt/test output.

## Run the tests

```
cd implementation/max_adapter
go build ./... && go vet ./... && gofmt -l . && go test ./... -v -race -count=1
```

Expected: build/vet clean, `gofmt -l` prints nothing, all tests `PASS`
(see `EVIDENCE.md` for the recorded run).
