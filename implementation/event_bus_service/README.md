# Event Bus service core — `digital.vasic.eventbusservice`

The client-facing core of the Helix Thready **Event Bus service**: a typed,
in-process publish/subscribe bus with subscription filters, **sticky**
last-value events with explicit invalidation, and **durable** subscribers that
replay missed events from a cursor for at-least-once delivery to disconnected
consumers.

- Module path: `digital.vasic.eventbusservice`
- Go: **1.26**, **standard library only** (no third-party deps)
- Contract source: [`docs/public/research/mvp/architecture/event-model.md`](../../docs/public/research/mvp/architecture/event-model.md),
  [`docs/public/research/mvp/api/event-bus-contract.md`](../../docs/public/research/mvp/api/event-bus-contract.md)

## Purpose & scope

This is the **in-process engine**. The NATS JetStream durable transport the
architecture describes is an out-of-scope **adapter seam** — this core
deliberately models the same guarantees so a JetStream adapter can wrap it
without changing the client-facing surface:

| Architecture concept (`event-model.md`) | Modeled here as |
|-----------------------------------------|-----------------|
| `EVENTBUS` ordered, durable JetStream stream | the ordered in-memory append **log** (`Bus.log`) |
| per-subject compacted / KV **sticky** store | the per-subject last-value map (`Bus.sticky`) |
| named durable consumer with ack floor / replay | `SubscribeDurable(filter, afterSeq)` replay-from-cursor |
| at-least-once + idempotent consumers on `event.ID` | durable queue + dedupe on `Event.ID` |
| ephemeral live fan-out (`pkg/bus`) | live `Subscribe` buffered channel (drops under backpressure) |

## The event envelope

```go
type Event struct {
    ID            string            // assigned by the bus if empty; dedupe key
    Seq           int64             // bus-assigned global order; durable-replay cursor
    Type          string            // dot-notation type, e.g. "post.received"
    Subject       string            // routing key subscribers filter on
    Payload       any               // opaque body
    TimestampUnix int64             // publish time (Unix seconds); assigned if zero
    Metadata      map[string]string // tenant/routing attributes, e.g. account_id
}
```

`Type/Subject/Payload/TimestampUnix/Metadata` are caller-set; `ID` and `Seq` are
assigned by the bus at publish time (`ID` for idempotent consumers, `Seq` as the
durable resume cursor).

## API

```go
b := eventbusservice.NewDefault()        // or New(Config{BufferSize, AsyncBufferSize})
defer b.Close()

// Publish (one-time). Returns the enriched event (ID + Seq assigned).
ev, err := b.Publish(eventbusservice.NewEvent("post.received", "post.received", payload).
    WithMetadata("account_id", "acc1"))

// Publish asynchronously (FIFO order preserved by a single dispatcher).
_ = b.PublishAsync(eventbusservice.NewEvent("post.received", "post.received", payload))
b.Drain()                                 // wait for all async publishes to complete

// Subscribe with a filter (glob subject + metadata).
sub := b.Subscribe(eventbusservice.Filter{
    Subject:  "post.*",
    Metadata: map[string]string{"account_id": "acc1"},
})
for ev := range sub.C { /* handle */ }

subAll := b.SubscribeAll()                // match everything

b.Unsubscribe(sub)                        // stops delivery; closes sub.C
```

### Filters — glob subject + metadata

`Filter.Subject` is a **NATS-style token glob** (tokens split on `.`):

| Pattern | Matches | Does **not** match |
|---------|---------|--------------------|
| `""` or `>` | everything | — |
| `post.received` | `post.received` | `post.processed`, `post` |
| `post.*` | `post.received` | `post.received.web` (single-token `*`) |
| `post.>` | `post.received`, `post.received.web` | `post` (tail needs ≥1 token) |
| `*.received` | `post.received`, `asset.received` | `received` |

`Filter.Metadata` entries must **all** be present and equal on the event. A zero
`Filter{}` matches every event.

## Sticky events + invalidation

A **sticky** event retains its last value **per subject** so a client that
connects late immediately learns current state without a REST round-trip —
snapshot-before-live ordering, exactly as `event-model.md` §4/§4.1 mandates.

```go
// Retain last value for the subject AND deliver live.
b.PublishSticky(eventbusservice.NewEvent("post.state.p1", "post.state", state))

// A subscriber joining now receives the sticky snapshot FIRST, then live events.
late := b.Subscribe(eventbusservice.Filter{Subject: "post.state.p1"})
snapshot := <-late.C                       // == last retained value

// Explicit invalidation clears the retained value and notifies connected clients
// with a live-only TypeStickyInvalidated event (not logged, not retained).
b.Invalidate("post.state.p1")
// A subscriber joining AFTER Invalidate receives no snapshot.
```

- The retained value is replaced by the next `PublishSticky` for the same
  subject, or cleared by `Invalidate(subject)`.
- `Invalidate` fans out a `TypeStickyInvalidated` notification
  (`Type == "eventbus.sticky.invalidated"`, `Subject == subject`) to
  currently-connected matching subscribers so they can drop stale UI state.
- `Sticky(subject) (Event, bool)` reads the current retained value (backs a
  `GET …/sticky` snapshot endpoint).

## Durable subscribers (replay + at-least-once)

Every published event is appended to an ordered log. A **durable** subscription
replays the log from a cursor, then streams live — the reconnect path for a
disconnected client:

```go
sub := b.SubscribeDurable(filter, 0)       // 0 = replay whole retained log
var cursor int64
for ev := range sub.C {
    if alreadyProcessed(ev.ID) { continue } // idempotent: dedupe on ID
    handle(ev)
    cursor = ev.Seq                          // remember progress
}
// ... after a disconnect (gap) ...
sub2 := b.SubscribeDurable(filter, cursor)  // replays only events with Seq > cursor
```

Durable subscribers are backed by an unbounded internal queue drained with
blocking sends — they **never drop** (backpressure instead). At-least-once is
provided by the log + replay-from-cursor; consumers dedupe on `Event.ID` to stay
idempotent under redelivery.

Live (non-durable) subscribers use a bounded buffer and **drop** when a slow
consumer lets it fill (counted in `Metrics.Dropped`) — best-effort fan-out.

## Metrics

```go
m := b.Metrics()   // Published, Delivered, Dropped (int64 snapshot)
```

- `Published` — real events accepted by `Publish`/`PublishSticky`/`PublishAsync`.
- `Delivered` — individual successful deliveries into subscriber channels.
- `Dropped` — live deliveries skipped because a subscriber buffer was full.

## Running the tests

```bash
cd implementation/event_bus_service
go build ./...
go vet ./...
gofmt -l .            # no output = clean
go test ./... -v -race -count=1
```

Real captured output and the pass/fail verdict live in [`EVIDENCE.md`](./EVIDENCE.md).

## Files

| File | Contents |
|------|----------|
| `event.go` | `Event`, `Filter`, `MatchSubject` glob, ID generation |
| `bus.go` | `Bus`, `Config`, `Metrics`, publish/subscribe/sticky/durable/invalidate/async/close |
| `subscription.go` | `Subscription` — live (drop) vs durable (queue + pump) delivery disciplines |
| `bus_test.go` | TDD suite (10 tests, `-race`) |

---

*Made with love ♥ by Helix Development.*
