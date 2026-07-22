# Telegram Adapter — Build Evidence (physical, no bluff)

Real captured output of building, vetting, formatting and testing the
`digital.vasic.telegramadapter` module. Nothing here is hand-edited output.

## Environment

```
$ go version
go version go1.26.4-X:nodwarf5 linux/amd64
```

- Module path: `digital.vasic.telegramadapter`
- `go.mod`: `go 1.26`
- Dependencies: **stdlib only** (`context`, `errors`, `fmt`, `mime`,
  `path/filepath`, `strconv`, `strings`; tests add `reflect`, `strings`,
  `testing`, `context`, `errors`). No third-party modules, **no `go get gotd/td`**,
  no `require` block. The gotd/td `tg.Message` shape is mirrored by the local
  `TGMessage` intermediate type, which keeps the mapper offline-testable.

## Command

```
cd implementation/telegram_adapter && go build ./... && go vet ./... && gofmt -l . && go test ./... -v -race -count=1
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
=== RUN   TestTelegramThreadReader_LiveCallsAreHonestStubs
--- PASS: TestTelegramThreadReader_LiveCallsAreHonestStubs (0.00s)
=== RUN   TestParseThreadID
=== RUN   TestParseThreadID/channel:1001
=== RUN   TestParseThreadID/channel:1001/100
=== RUN   TestParseThreadID/chat:42/7
=== RUN   TestParseThreadID/user:99
=== RUN   TestParseThreadID/channel:1234567890123456789/55
--- PASS: TestParseThreadID (0.00s)
    --- PASS: TestParseThreadID/channel:1001 (0.00s)
    --- PASS: TestParseThreadID/channel:1001/100 (0.00s)
    --- PASS: TestParseThreadID/chat:42/7 (0.00s)
    --- PASS: TestParseThreadID/user:99 (0.00s)
    --- PASS: TestParseThreadID/channel:1234567890123456789/55 (0.00s)
=== RUN   TestMapMessages_RepresentativeThread
--- PASS: TestMapMessages_RepresentativeThread (0.00s)
=== RUN   TestMapMessages_HashtagTextPreservedVerbatim
--- PASS: TestMapMessages_HashtagTextPreservedVerbatim (0.00s)
=== RUN   TestMapMessages_RootDetection
--- PASS: TestMapMessages_RootDetection (0.00s)
=== RUN   TestMapMessages_Empty
=== RUN   TestMapMessages_Empty/nil
=== RUN   TestMapMessages_Empty/empty
--- PASS: TestMapMessages_Empty (0.00s)
    --- PASS: TestMapMessages_Empty/nil (0.00s)
    --- PASS: TestMapMessages_Empty/empty (0.00s)
=== RUN   TestMapMessages_MediaOnly
--- PASS: TestMapMessages_MediaOnly (0.00s)
=== RUN   TestMapMessages_MIMEInference
=== RUN   TestMapMessages_MIMEInference/photo_is_jpeg
=== RUN   TestMapMessages_MIMEInference/document_mime_verbatim
=== RUN   TestMapMessages_MIMEInference/document_infers_from_ext
=== RUN   TestMapMessages_MIMEInference/document_octet-stream_fallback
=== RUN   TestMapMessages_MIMEInference/document_no_name_no_mime
--- PASS: TestMapMessages_MIMEInference (0.01s)
    --- PASS: TestMapMessages_MIMEInference/photo_is_jpeg (0.00s)
    --- PASS: TestMapMessages_MIMEInference/document_mime_verbatim (0.00s)
    --- PASS: TestMapMessages_MIMEInference/document_infers_from_ext (0.01s)
    --- PASS: TestMapMessages_MIMEInference/document_octet-stream_fallback (0.00s)
    --- PASS: TestMapMessages_MIMEInference/document_no_name_no_mime (0.00s)
=== RUN   TestMapMessages_ForumTopicGrouping
--- PASS: TestMapMessages_ForumTopicGrouping (0.00s)
=== RUN   TestMapMessages_InvalidID
--- PASS: TestMapMessages_InvalidID (0.00s)
=== RUN   TestMapMessages_LargeIDPrecision
--- PASS: TestMapMessages_LargeIDPrecision (0.00s)
=== RUN   TestMapMessages_ChannelPostAuthorIsChannel
--- PASS: TestMapMessages_ChannelPostAuthorIsChannel (0.00s)
PASS
ok  	digital.vasic.telegramadapter	1.023s
```

**Totals:** 12 test functions, 12 sub-tests → 24 `PASS`, 0 fail, race detector
enabled (`-race`), no cache (`-count=1`). Package result `ok`.

## Honest verdict

- **REAL + tested:** the `TGMessage → Post` mapping (`MapMessages`, `map.go`). It
  maps message ids (64-bit-exact), author (sender, or the channel itself for a
  broadcast), verbatim hashtag-bearing text, reply parent linkage from the reply
  header's `reply_to_msg_id` (including reply-to-a-reply chains), the forwarded
  flag from the presence of a forward header, `Date` → Unix-seconds timestamps,
  and media → attachments (document `mime_type` verbatim, else inferred). It also
  derives forum-topic / `getReplies` thread grouping into `ThreadID` from
  `reply_to_top_id` / the `forum_topic` flag, with `GroupByThread` bucketing the
  result. Proven against a representative root+replies+forward+media slice plus
  edge cases: missing reply-to (root), empty/nil input, media-only, MIME
  inference, forum-topic grouping, invalid (non-positive) id, and large-id
  precision.
- **[BUILD-NEW], NOT done:** the live gotd/td MTProto USER client — `Connect` /
  `Authenticate` / `FetchThreadHistory` (and the `FetchThread` bridge) in
  `client.go`. Every live method returns `ErrNotImplemented`, and
  `TestTelegramThreadReader_LiveCallsAreHonestStubs` asserts it. This environment
  has **no api_id/api_hash, no login session and no Telegram account**; the
  on-wire MTProto session is **unverified**. No network call is faked and no live
  channel read is claimed to work. The live reads land when Herald's
  `qaherald/internal/mtproto` gotd/td client is promoted to a first-class
  `MessageSource` (§3 / [GAP: 5.1.1] of
  `docs/public/research/mvp/architecture/messenger-ingestion.md`); the intended
  `messages.getReplies` / `messages.getHistory` call shape is documented in the
  `FetchThreadHistory` doc comment, and it terminates in the already-tested
  `MapMessages`.
