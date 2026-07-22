# Telegram Adapter (`digital.vasic.telegramadapter`)

A Go adapter that reads **Telegram channel/group thread history** into Helix
Thready's [ThreadReader](../threadreader) ingestion seam.

It exists to close **[GAP: 5.1.1]** from
[`messenger-ingestion.md`](../../docs/public/research/mvp/architecture/messenger-ingestion.md):
Herald already contains a working `gotd/td` MTProto **user** client (in
`qaherald/internal/mtproto`) that can backfill channel history via
`messages.getHistory` / `messages.getReplies` — the Bot API cannot. The plan is
to promote that client to a first-class `ThreadReader.MessageSource`. This module
delivers the **offline, dependency-free half** of that promotion: the real,
tested mapping from Telegram messages to Thready `Post`s.

## What is real vs. what is a stub (no bluff)

| Part | Status | Where |
|------|--------|-------|
| `TGMessage → Post` mapping (`MapMessages`) | **REAL, stdlib-only, `go test` covered** | `map.go`, `map_test.go` |
| Live MTProto `Connect` / `Authenticate` / `FetchThreadHistory` | **[BUILD-NEW] honest stub** — returns `ErrNotImplemented` | `client.go` |

There is **no live Telegram read here** and none is claimed. This environment has
no `api_id`/`api_hash`, no login session and no account, so every live method
fails loudly rather than fake a connection. See [`EVIDENCE.md`](./EVIDENCE.md)
for the honest verdict and captured build/test output.

## Why an intermediate `TGMessage` (no `gotd/td` dependency)

Instead of `go get`-ting `gotd/td`, this module defines a local `TGMessage`
struct (plus `TGPeer`, `TGReplyTo`, `TGFwdFrom`, `TGMedia`) that **mirrors the
fields the vendored `gotd/td` `tg.Message` exposes** — id, peer, from-id, date,
message text, reply header (`reply_to_msg_id` / `reply_to_top_id` / `forum_topic`),
forward header, and media. Provenance is documented field-by-field in
`tgmessage.go`.

This keeps the module **stdlib-only and offline-testable**, and makes the "no
bluff" guarantee mechanical: there is no live client to accidentally exercise.
On promotion, Herald's real `tg.Message` values are copied into `TGMessage`
field-for-field and `MapMessages` is called **unchanged**.

## The ThreadReader + Herald / `gotd/td` seam

```
Telegram DC ──MTProto──▶ gotd/td telegram.Client        [BUILD-NEW, in Herald]
                              │  messages.getReplies / getHistory
                              ▼
                         []tg.Message
                              │  field-for-field copy (on promotion)
                              ▼
        ┌───────────────────────────────────────────────┐
        │ digital.vasic.telegramadapter (THIS MODULE)    │
        │                                                │
        │  []TGMessage ──MapMessages()──▶ []Post   ◀── REAL + tested
        │                                                │
        │  TelegramThreadReader implements MessageSource │
        │   Connect / Authenticate / FetchThreadHistory  ◀── [BUILD-NEW] stub
        │   FetchThread(ctx, threadID) ──┐               │
        └────────────────────────────────┼──────────────┘
                                          ▼
                    threadreader.MessageSource.FetchThread(ctx, string) ([]Post, error)
                                          ▼
                    threadreader.Assembler.Assemble(posts) → Thread (root + organic replies)
```

- `MapMessages([]TGMessage) ([]Post, error)` is the reusable core; it needs no
  client.
- `TelegramThreadReader` implements the local `MessageSource` seam
  (`Connect` / `Authenticate` / `FetchThreadHistory`) and additionally exposes
  `FetchThread(ctx, threadID)` so it also satisfies **`threadreader.MessageSource`**
  — wiring it into the assembler is a one-liner once a live transport exists.

### Mapping rules (`MapMessages`)

| `Post` field | Source | Notes |
|--------------|--------|-------|
| `ID` | `TGMessage.ID` | decimal string, 64-bit-exact |
| `ThreadID` | peer key `(+ "/" + topic root)` | forum-topic / `getReplies` grouping (`reply_to_top_id` / `forum_topic`) |
| `ParentID` | `ReplyTo.ReplyToMsgID` | immediate reply edge; `""` when absent → **root**. Handles reply-to-a-reply |
| `AuthorID` | `FromID`, else `Peer` | a channel **broadcast** is authored by the channel |
| `Text` | `TGMessage.Message` | **verbatim** — hashtag tokens preserved for downstream extraction |
| `TimestampUnix` | `TGMessage.Date` | already Unix seconds |
| `IsForwarded` | `FwdFrom != nil` | presence of a forward header |
| `Attachments` | `Media` | 0 or 1; document `mime_type` verbatim, else inferred; `SHA256` empty (MTProto carries none) |

## Run the tests

```
cd implementation/telegram_adapter
go build ./... && go vet ./... && gofmt -l . && go test ./... -v -race -count=1
```

Latest captured run: **12 test functions / 24 PASS, 0 fail**, `-race`, `-count=1`
— see [`EVIDENCE.md`](./EVIDENCE.md).

The tests feed a representative slice — channel root + human replies + a
reply-to-a-reply + a forwarded message + one with media — and assert the full
`[]Post` (ids, parent linkage, forwarded flag, attachments, verbatim text), plus
edge cases: missing reply-to (root), empty/nil, media-only, MIME inference,
forum-topic grouping, invalid (non-positive) id, and large-id precision. A
separate test asserts every live MTProto method returns `ErrNotImplemented`.

---

*Made with love ♥ by Helix Development.*
