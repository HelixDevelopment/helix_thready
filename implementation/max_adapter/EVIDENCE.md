# Max Adapter — Build Evidence (physical, no bluff)

Real captured output of building, vetting, formatting and testing the
`digital.vasic.maxadapter` module. Nothing here is hand-edited output.

## Environment

```
$ go version
go version go1.26.4-X:nodwarf5 linux/amd64
```

- Module path: `digital.vasic.maxadapter`
- `go.mod`: `go 1.26`
- Dependencies: **stdlib only** (`bytes`, `context`, `encoding/json`, `errors`,
  `fmt`, `mime`, `path/filepath`, `strconv`, `strings`; tests add `os`,
  `path/filepath`, `reflect`, `testing`). No third-party modules, no `require`
  block.

## Command

```
cd implementation/max_adapter && go build ./... && go vet ./... && gofmt -l . && go test ./... -v -race -count=1
```

## Result — build, vet, gofmt

```
$ go build ./...
BUILD_OK (exit 0)

$ go vet ./...
VET_OK (exit 0)

$ gofmt -l .
(no output — all files formatted)
```

All three exited 0; `gofmt -l` printed nothing (every file already formatted).

## Result — test (`go test ./... -v -race -count=1`)

```
=== RUN   TestOneMeClient_LiveCallsAreHonestStubs
--- PASS: TestOneMeClient_LiveCallsAreHonestStubs (0.00s)
=== RUN   TestNewOneMeClient_Defaults
--- PASS: TestNewOneMeClient_Defaults (0.00s)
=== RUN   TestParseHistory_RepresentativeFrame
--- PASS: TestParseHistory_RepresentativeFrame (0.01s)
=== RUN   TestParseHistory_ReplyLinkage
=== RUN   TestParseHistory_ReplyLinkage/reply_via_link.messageId
=== RUN   TestParseHistory_ReplyLinkage/reply_via_prevMessageId
=== RUN   TestParseHistory_ReplyLinkage/forward_sets_flag,_not_parent
=== RUN   TestParseHistory_ReplyLinkage/explicit_forwarded_flag
--- PASS: TestParseHistory_ReplyLinkage (0.00s)
    --- PASS: TestParseHistory_ReplyLinkage/reply_via_link.messageId (0.00s)
    --- PASS: TestParseHistory_ReplyLinkage/reply_via_prevMessageId (0.00s)
    --- PASS: TestParseHistory_ReplyLinkage/forward_sets_flag,_not_parent (0.00s)
    --- PASS: TestParseHistory_ReplyLinkage/explicit_forwarded_flag (0.00s)
=== RUN   TestParseHistory_AcceptedShapes
=== RUN   TestParseHistory_AcceptedShapes/frame
=== RUN   TestParseHistory_AcceptedShapes/payload
=== RUN   TestParseHistory_AcceptedShapes/array
--- PASS: TestParseHistory_AcceptedShapes (0.00s)
    --- PASS: TestParseHistory_AcceptedShapes/frame (0.00s)
    --- PASS: TestParseHistory_AcceptedShapes/payload (0.00s)
    --- PASS: TestParseHistory_AcceptedShapes/array (0.00s)
=== RUN   TestParseHistory_MissingFields
--- PASS: TestParseHistory_MissingFields (0.00s)
=== RUN   TestParseHistory_EmptyHistory
=== RUN   TestParseHistory_EmptyHistory/empty_payload_messages
=== RUN   TestParseHistory_EmptyHistory/empty_bare_array
--- PASS: TestParseHistory_EmptyHistory (0.00s)
    --- PASS: TestParseHistory_EmptyHistory/empty_payload_messages (0.00s)
    --- PASS: TestParseHistory_EmptyHistory/empty_bare_array (0.00s)
=== RUN   TestParseHistory_Attachments
--- PASS: TestParseHistory_Attachments (0.00s)
=== RUN   TestParseHistory_LargeIDPrecision
--- PASS: TestParseHistory_LargeIDPrecision (0.00s)
=== RUN   TestParseHistory_InvalidJSON
--- PASS: TestParseHistory_InvalidJSON (0.00s)
PASS
ok  	digital.vasic.maxadapter	1.021s
```

**Totals:** 10 test functions, 9 sub-tests → 19 `PASS`, 0 fail, race detector
enabled (`-race`), no cache (`-count=1`). Package result `ok`.

## Honest verdict

- **REAL + tested:** the OneMe history JSON → `Post` mapping (`ParseHistory`,
  `wire.go`). It maps ids (numeric or string, 64-bit-exact), author, text,
  reply parent linkage (both `link.messageId` and `prevMessageId` encodings),
  the forwarded flag (`link.type == "FORWARD"`), epoch-ms → Unix-seconds
  timestamps, and `_type`-discriminated attachments — proven against a
  research-shaped canned payload plus edge cases (missing fields, forward,
  empty history, large-id precision, malformed JSON, three envelope shapes).
- **[BUILD-NEW], NOT done:** the live WebSocket connect / handshake / phone+token
  auth / opcode-49 send-receive (`client.go`). Every live method returns
  `ErrNotImplemented` and `TestOneMeClient_LiveCallsAreHonestStubs` asserts it.
  No live Max account exists in this environment; the on-wire session is
  **unverified**. No network call is faked and no live read is claimed to work.
- Provenance of every mapped field (CONFIRMED vs INFERRED, with source URLs) is
  in `PROTOCOL.md`.
