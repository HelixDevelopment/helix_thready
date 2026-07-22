# ThreadReader (`digital.vasic.threadreader`)

The messenger-agnostic **ThreadReader** abstraction for Helix Thready — the
`[GAP: 5.1.3]` closure from
[`messenger-ingestion.md`](../../docs/public/research/mvp/architecture/messenger-ingestion.md).

## Purpose

In the messengers Thready ingests (Telegram, Max, Slack…) **a "post" is a
composite**, not a single message. Hashtags are frequently added as a *reply* to a
link-only or text-only root post. Collapsing a thread to its first message would
silently drop the very hashtags and attachments that decide how it is processed.

ThreadReader assembles the **complete post**:

> **root post + the full chain of _organic_ replies (real humans), in order,
> excluding the system's own processing/status replies**, and extracts every
> hashtag across the whole chain.

It is deliberately pure (stdlib only, no I/O, no messenger SDK) so it is reusable,
deterministic and trivially testable. Concrete messenger clients plug in behind the
`MessageSource` seam.

## API

```go
// Models
type Attachment struct { ID, MIME, FileName, SHA256 string }

type Post struct {
    ID, ThreadID, ParentID, AuthorID, Text string
    TimestampUnix int64
    IsForwarded   bool
    Attachments   []Attachment
}

type Thread struct {
    Root    Post
    Replies []Post // organic only, chronological
}
func (t *Thread) Hashtags() []string // union across root + replies, deduped

// Seam over any messenger (Telegram gotd/td, Max OneMe, Slack, …)
type MessageSource interface {
    FetchThread(ctx context.Context, threadID string) ([]Post, error)
}

// Core assembly
type Assembler struct { /* ... */ }
func NewAssembler(systemAuthorIDs ...string) *Assembler
func (a *Assembler) Assemble(posts []Post) (*Thread, error)
func (a *Assembler) IsSystemAuthor(authorID string) bool

// Source + assembler wired together
type ThreadReader struct { /* ... */ }
func New(source MessageSource, systemAuthorIDs ...string) *ThreadReader
func (tr *ThreadReader) Read(ctx context.Context, threadID string) (*Thread, error)

// Hashtag extraction (the classification input)
func ExtractHashtags(text string) []string

// Errors
var ErrNoPosts     = errors.New("threadreader: no posts provided")
var ErrMissingRoot = errors.New("threadreader: no root post found in thread")
```

### Typical use

```go
tr := threadreader.New(myTelegramSource, "thready-bot") // bot IDs to exclude
thread, err := tr.Read(ctx, threadID)
if err != nil { /* ErrMissingRoot, ErrNoPosts, or a source error */ }

tags := thread.Hashtags()          // classification input
for _, r := range thread.Replies { // organic replies, chronological
    _ = r
}
```

## How the organic-vs-system rule works

`Assemble` is fully deterministic regardless of input order or duplicates:

1. **Dedupe by post `ID`.** Exact duplicates (a message delivered twice) collapse
   to one. The final order is imposed by step 4, so the result never depends on
   which duplicate arrived first.
2. **Identify the root** — a post whose `ParentID` is empty or points at itself.
   If several qualify, the earliest by `(timestamp, ID)` wins. If **none** qualify
   (only replies, whose parent is absent), `Assemble` returns `ErrMissingRoot` —
   deterministic, never a panic. Empty input returns `ErrNoPosts`.
3. **Exclude system authors.** Any reply whose `AuthorID` is in the configured
   system/bot set is dropped — these are Thready's own status replies. This mirrors
   the verified herald anti-echo-loop primitive `IsSelfEcho` (fed by
   `BotSelfIdentity` / `StampSender`): a reply from **this** bot is dropped, but a
   reply from a **different** author — a human, or another bot not in the set — is
   **kept** (multi-bot collaboration is real traffic).
4. **Order** the surviving organic replies by `TimestampUnix`, tie-broken by `ID`,
   so a shuffled input always yields the same chronological chain.

Forwarded messages are ordinary replies: their text, `IsForwarded` flag and
attachments are preserved untouched.

`Thread.Hashtags()` then unions `ExtractHashtags` across the root and every organic
reply — which is precisely why a **link-only root still classifies**: the tags live
on a reply. `ExtractHashtags` is unicode-aware (Cyrillic, CJK, accents, digits,
underscores), stops at adjacent punctuation, requires a word boundary before `#`
(so `foo#bar` and `example.com#frag` are ignored), and dedupes with case preserved.

## Run the tests

```
cd implementation/threadreader
go build ./... && go vet ./... && go test ./... -v -race -count=1
```

Requires **Go 1.26+**. Stdlib only — no external dependencies. Captured proof of a
green run is in [`EVIDENCE.md`](./EVIDENCE.md).

## Scope boundary (honest)

This is the **pure ThreadReader core**. Live messenger adapters — the Telegram
gotd/td MTProto reader (`[GAP: 5.1.1]`) and the Max OneMe port (`[GAP: 5.1.2]`,
`[OPEN: ING-1]`) — are separate BUILD-NEW work that implement `MessageSource` and
feed this core. The `MessageSource` used in the tests is an in-memory fake; the
assembly, filtering and hashtag logic it exercises is real and test-green.

---

*Made with love ♥ by Helix Development.*
