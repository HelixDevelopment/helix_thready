<!--
  Title           : Helix Thready — Event Model (EventBus / NATS JetStream)
  Classification  : PUBLIC
  Location        : docs/public/research/mvp/architecture/event-model.md
  Status          : Draft — v0.1
  Revision        : 1 (2026-07-21)
  Author          : Helix Thready documentation swarm (System Architecture)
  Related         : ./system-overview.md, ./concurrency-and-idempotency.md,
                    ./data-flow.md, ./messenger-ingestion.md, ./processing-pipeline.md
-->

# Helix Thready — Event Model

| Rev | Date | Author | Change |
|-----|------|--------|--------|
| 1 | 2026-07-21 | swarm (System Architecture) | Initial draft — transport, event catalog, sticky/one-time, replay |
| 2 | 2026-07-21 | swarm (review pass) | Add OpenAPI 3.1 SSE subscription contract (§8) per CONVENTIONS §6 |

## Table of Contents

1. [Transport & guarantees](#1-transport--guarantees)
2. [The `event.Event` envelope (verified interface)](#2-the-eventevent-envelope-verified-interface)
3. [Subject naming & namespacing](#3-subject-naming--namespacing)
4. [One-time vs sticky events + invalidation](#4-one-time-vs-sticky-events--invalidation)
5. [Event lifecycle diagram](#5-event-lifecycle-diagram)
6. [Durable replay for disconnected clients](#6-durable-replay-for-disconnected-clients)
7. [Event catalog (MVP)](#7-event-catalog-mvp)
8. [Client subscription contract](#8-client-subscription-contract)
9. [Gap-register coverage](#9-gap-register-coverage)
10. [TDD reproduce-first skeletons](#10-tdd-reproduce-first-skeletons)
11. [Open items](#11-open-items)

---

## 1. Transport & guarantees

The event bus is `digital.vasic.eventbus` `[IN-HOUSE: eventbus]`. It ships **two mirrored
transports** behind one event model:

- `pkg/bus` — an in-process channel bus for intra-process fan-out (buffered subscriber
  channels, publish timeout, dead-subscriber cleanup). **VERIFIED** at source: `bus.New(config)`,
  `Config{BufferSize, PublishTimeout, CleanupInterval, MaxSubscribers}`.
- `pkg/nats` — a **NATS JetStream** adapter that mirrors the same pub/sub surface but persists
  every published event in a JetStream stream. **VERIFIED**: `nats.New(ctx, Config{URL,
  StreamName,…})` calls `ensureStream()` and exposes `Publish(e *event.Event) error`,
  `Subscribe(...)`, `Close()`. The default stream name is `EVENTBUS`.

At Thready's **Large scale** `[OPERATOR]`, **JetStream is the primary transport**; the
in-process bus is used only for tight intra-service fan-out. Delivery guarantee is
**at-least-once** with idempotent consumers `[research_request_final §3.4]`. Kafka/RabbitMQ
(`digital.vasic.messaging`) remain available for firehose-scale inter-service streams but are
**not** the Thready event bus (that decision is explicit: "Not Redis, not Kafka" in §2.1.3).

> **Anti-bluff note.** At-least-once means duplicate deliveries *will* happen (redeliver on
> consumer restart, network partition). Correctness therefore depends on the idempotent
> single-claim in [concurrency-and-idempotency.md](./concurrency-and-idempotency.md), not on
> the bus. The event model is only safe *because* consumers dedupe.

## 2. The `event.Event` envelope (verified interface)

The envelope is fixed by `eventbus/pkg/event` — **read at source**, reproduced verbatim:

```go
// digital.vasic.eventbus/pkg/event
package event

type Type string // dot-notation topic, e.g. "post.received"

type Event struct {
    ID        string
    Type      Type
    Source    string
    Payload   interface{}
    Timestamp time.Time
    TraceID   string
    Metadata  map[string]string
}

func New(eventType Type, source string, payload interface{}) *Event // sets ID, TraceID, ts
func (e *Event) WithTraceID(traceID string) *Event
func (e *Event) WithMetadata(key, value string) *Event

type Handler func(*Event)
type Subscription struct { ID string; Types []Type; Channel <-chan *Event; /*…*/ }
```

Thready adds **no new envelope fields**; it uses `Metadata` for `account_id`, `messenger`,
`post_id`, and `idempotency_key`, and reuses `TraceID` to stitch a post's whole processing
story across services (the same `TraceID` flows from `post.received` through every
`post.processing.*` to `post.processed`). Payloads are typed Go structs serialized to JSON by
the JetStream adapter (`Publish` marshals to JSON — VERIFIED).

```go
// Thready publishes a post.received event on ingestion (illustrative composition
// over the verified event.New signature).
evt := event.New("post.received", "ingestion", PostReceivedPayload{
    PostID:    p.ID,
    Account:   p.AccountID,
    Messenger: "telegram",
    RootMsgID: p.RootMessageID,
    Hashtags:  p.Hashtags,
}).WithMetadata("account_id", p.AccountID).
   WithMetadata("idempotency_key", p.ID)
if err := jetstreamBus.Publish(evt); err != nil { /* retry with backoff */ }
```

## 3. Subject naming & namespacing

The JetStream adapter derives a subject from the event `Type` (VERIFIED: `subjectFor(t)` +
`sanitizeType`). Thready's convention layers three namespaces so one JetStream cluster can host
all three environments and enforce tenant isolation:

```
thready.<env>.<account_id>.<event.type>
        │      │            └── dot-notation event type, e.g. post.processed
        │      └── account UUID (or "system" for global events)
        └── dev | sta | prod
```

Example subjects: `thready.prod.9c1e….post.received`, `thready.prod.system.channel.health`.
Consumers filter by subject wildcards, e.g. a per-account client subscribes to
`thready.prod.9c1e….>` (all events for its account) while the Processing service consumes
`thready.prod.*.post.received` across accounts. This directly resolves
`[OPEN: OVERVIEW-2]` — env isolation is achievable with subject prefixes on a shared cluster.

## 4. One-time vs sticky events + invalidation

The original request mandates **both** one-time events and **sticky** events with explicit
invalidation `[research_request_final §2.1.3, §3.4]`.

- **One-time events** — fire, are delivered, and are done (`post.received`,
  `post.processing.progress`, `post.processed`, `asset.stored`). A late subscriber does **not**
  see a past one-time event except via JetStream durable replay of the retained window.
- **Sticky events** — retain a **last value per entity** so a newly connected client can read
  current state without a REST round-trip. Implemented as a **compacted JetStream subject** (a
  `WorkQueue`/`Limits` stream with per-subject last-value retention keyed by entity id) plus an
  L1 last-value cache (`digital.vasic.cache`). The canonical sticky event is `post.state`
  (keyed `…post.state.<post_id>`) carrying the current `{state, progress, last_error,
  asset_refs}`.
- **Invalidation** — a sticky value is invalidated on state change or TTL. On an explicit
  reprocess (`client → REST /v1/posts/{id}/reprocess → System`), the Processing service
  publishes `post.state.invalidate` for that `post_id`, clears the last-value cache entry, and
  the next `post.state` re-establishes the sticky value. TTL is a defensive backstop
  (`[DEFAULT — adjustable]` 24 h) so a crashed producer cannot pin a stale sticky value forever.

```go
// Sticky publish: last-value semantics keyed by entity id.
func (p *Publisher) PublishState(ctx context.Context, s PostState) error {
    subj := fmt.Sprintf("thready.%s.%s.post.state.%s", p.env, s.AccountID, s.PostID)
    p.lastValue.Set(subj, s, 24*time.Hour)      // L1 sticky cache
    return p.js.PublishMsg(ctx, subj, mustJSON(s)) // compacted subject retains last
}

func (p *Publisher) Invalidate(ctx context.Context, accountID, postID string) error {
    subj := fmt.Sprintf("thready.%s.%s.post.state.%s", p.env, accountID, postID)
    p.lastValue.Delete(subj)
    return p.js.PublishMsg(ctx, subj+".invalidate", nil)
}
```

## 5. Event lifecycle diagram

```mermaid
sequenceDiagram
  participant HR as Herald ThreadReader
  participant BUS as EventBus (NATS JetStream)
  participant BG as BackgroundTasks
  participant PROC as Skill Dispatch
  participant EBS as Event Bus Service
  participant CL as Client (WS/SSE)

  HR->>BUS: Publish post.received (JetStream, persisted)
  BUS->>BG: durable pull consumer (at-least-once)
  BG->>BG: single-claim (row lock) — dedupe
  BG->>PROC: hand claimed post
  PROC->>BUS: Publish post.processing.progress (one-time)
  PROC->>BUS: Publish post.state (STICKY, last-value)
  PROC->>BUS: Publish post.processed (one-time)
  Note over BUS: sticky subject retains last post.state per post_id
  CL->>EBS: Subscribe(account, types)
  EBS->>BUS: durable consumer per client
  BUS-->>EBS: replay missed + live events
  EBS-->>CL: fan-out over WebSocket/SSE
  CL->>EBS: reconnect after drop
  EBS->>BUS: resume durable consumer (replay backlog)
  PROC->>BUS: Publish post.state.invalidate (on reprocess)
```

> Rendered PNG/SVG exported via Docs Chain (§11.4.65). Source: `diagrams/event-flow.mmd`.

**Explanation (for readers/models that cannot see the diagram).** The sequence starts when the
Herald ThreadReader publishes `post.received` to JetStream, where it is persisted. A durable
pull consumer inside BackgroundTasks receives it (at-least-once, so possibly more than once).
Before doing any work, BackgroundTasks performs the **single-claim** (a Postgres row lock) that
deduplicates redeliveries — this is the linchpin that makes at-least-once safe. Once claimed,
the Skill Dispatch engine runs and emits a stream of events: repeated one-time
`post.processing.progress` events for live UI, a **sticky** `post.state` event that JetStream
retains as the last value for that `post_id`, and finally a one-time `post.processed`. On the
client side, a client subscribes through the thin Event Bus Service, which opens a **durable
consumer** per client; JetStream replays any events the client missed and then streams live
ones, fanned out over WebSocket or SSE. If the client drops and reconnects, the durable
consumer resumes and replays the backlog — no events are lost. Finally, when someone triggers a
reprocess, the dispatcher publishes `post.state.invalidate`, clearing the sticky value so the
next `post.state` reflects the fresh run.

## 6. Durable replay for disconnected clients

Each client subscription is backed by a **named durable JetStream consumer**
(`durable = "client-<client_id>"`) so its acknowledgement cursor survives disconnects. On
reconnect the Event Bus Service resumes the same durable consumer and JetStream redelivers
unacknowledged events from the retained window. For long outages beyond the retention window,
clients **reconcile via REST snapshots** (`GET /v1/posts?updated_since=…`) — the event stream
is the fast path, REST is the source-of-truth backstop. This dual path is mandated by
`§3.4` ("durable JetStream consumers replay missed events on reconnect; clients also reconcile
via REST snapshots").

## 7. Event catalog (MVP)

The original request mandates a dedicated document enumerating **every** system event, when it
fires, and how to subscribe. This is the architectural seed of that catalog (the exhaustive
per-field version lives in the API pack).

| Event `Type` | Kind | Producer | Fires when | Key payload |
|--------------|------|----------|-----------|-------------|
| `post.received` | one-time | Ingestion | Root+organic thread assembled & persisted | post_id, account, messenger, hashtags |
| `post.claimed` | one-time | BackgroundTasks | Post claimed for processing (dedupe won) | post_id, worker_id |
| `post.processing.progress` | one-time | Skill Dispatch | Each Skill step reports progress | post_id, step, percent, message |
| `post.state` | **sticky** | Skill Dispatch | State transition (queued→running→done) | post_id, state, progress, asset_refs, last_error |
| `post.state.invalidate` | one-time | Skill Dispatch | Reprocess/refresh requested | post_id |
| `post.processed` | one-time | Skill Dispatch | All Skills done, status reply posted | post_id, metrics, asset_refs |
| `post.failed` | one-time | Skill Dispatch | Whole-post processing failed after retries | post_id, error, retriable |
| `skill.step.retried` | one-time | Skill Dispatch | A step re-enqueued after backoff | post_id, step, attempt |
| `asset.download.progress` | one-time | Download Mgr/Boba/MeTube | Delegated download progress callback | job_id, percent |
| `asset.stored` | one-time | Asset Service | Asset (raw or `…-web`) committed to storage | asset_id, post_id, kind, checksum |
| `index.updated` | one-time | Semantic-search | Embeddings written for a post/artifact | source_id, kind, vector_count |
| `channel.health` | **sticky** | Ingestion | Channel reachability/lag changes | channel_id, reachable, lag_s |
| `account.updated` | **sticky** | User Service | Account/branding/policy change | account_id, version |

## 8. Client subscription contract

Clients never touch NATS directly. They subscribe through the **Event Bus Service** over
WebSocket (bidirectional) or SSE (one-way), authenticated by the same JWT/API-key as REST and
scoped by RBAC so a client only receives events for accounts it can see. The wire contract
(OpenAPI/AsyncAPI detail in the API pack):

```
WS  /v1/events?types=post.state,post.processed&account=<id>   (Authorization: Bearer …)
SSE GET /v1/events/stream?types=post.processing.progress&post_id=<id>
```

On connect, sticky events are delivered **first** (current last-value snapshot) then live
events follow — so a UI paints current state immediately without a separate REST fetch.

The architecture-level contract as **OpenAPI 3.1** `[CONVENTIONS §6]` (the full per-event
AsyncAPI channel doc — including the bidirectional WebSocket frames, which OpenAPI does not model
natively — lives in the [api/](../api/index.md) pack):

```yaml
openapi: 3.1.0
info: { title: Helix Thready — Event stream (architecture excerpt), version: "1.0.0" }
paths:
  /v1/events/stream:
    get:
      operationId: subscribeEventStream
      summary: Server-Sent Events stream of Thready events (one-way).
      security: [ { bearerAuth: [] } ]     # same JWT/API-key as REST; RBAC-scoped by account
      parameters:
        - { name: types, in: query, required: true,
            description: Comma-separated event.Type filter (e.g. post.state,post.processed),
            schema: { type: string } }
        - { name: account, in: query, required: false,
            schema: { type: string, format: uuid } }
        - { name: post_id, in: query, required: false,
            schema: { type: string, format: uuid } }
        - { name: Last-Event-ID, in: header, required: false,
            description: Resume cursor; maps to the durable JetStream consumer ack floor,
            schema: { type: string } }
      responses:
        "200":
          description: text/event-stream; sticky snapshot first, then live events.
          content:
            text/event-stream:
              schema: { $ref: "#/components/schemas/EventEnvelope" }
        "401": { description: Missing/invalid credentials }
        "403": { description: RBAC denies the requested account/types }
components:
  securitySchemes:
    bearerAuth: { type: http, scheme: bearer, bearerFormat: JWT }
  schemas:
    EventEnvelope:               # mirrors eventbus/pkg/event.Event (§2)
      type: object
      required: [id, type, source, timestamp]
      properties:
        id:        { type: string }
        type:      { type: string, examples: [post.state, post.processed] }
        source:    { type: string }
        payload:   {}            # typed per event.Type; see §7 catalog
        timestamp: { type: string, format: date-time }
        traceId:   { type: string }
        metadata:  { type: object, additionalProperties: { type: string } }
```

## 9. Gap-register coverage

- `[GAP: 2.9]` `session_orchestrator` (the atomic claim registry) is DESIGN-ONLY. Thready does
  **not** depend on it; the equivalent guarantee is provided by BackgroundTasks' Postgres claim
  (see [concurrency-and-idempotency.md](./concurrency-and-idempotency.md)). The event model is
  designed to be correct under at-least-once *without* a separate claim registry.
- `[GAP: 6.6]` The standardized callback/task module feeds `asset.download.progress` /
  `asset.stored`; its schema is defined in [asset-and-download.md](./asset-and-download.md).
- The Event Bus Service itself is `[BUILD-NEW]` (gap register §11) — a thin wrapper; the heavy
  lifting (JetStream, durability) is the VERIFIED `eventbus/pkg/nats`.

## 10. TDD reproduce-first skeletons

Per `[CONSTITUTION §11.4.27/43]`, every behavior starts RED. Representative skeletons:

```go
// RED: at-least-once must NOT cause double processing.
func TestPostReceived_DuplicateDelivery_ProcessesOnce(t *testing.T) {
    bus := newTestJetStream(t)
    evt := event.New("post.received", "test", PostReceivedPayload{PostID: "p1"})
    _ = bus.Publish(evt)
    _ = bus.Publish(evt) // simulate JetStream redelivery
    got := drainClaims(t, bus, 2*time.Second)
    require.Equal(t, 1, got.ProcessedCount("p1")) // FAILS until single-claim wired
}

// RED: sticky post.state must be delivered to a late subscriber before live events.
func TestStickyState_LateSubscriber_GetsSnapshotFirst(t *testing.T) {
    pub := newPublisher(t)
    _ = pub.PublishState(ctx, PostState{PostID: "p1", State: "running", Progress: 42})
    sub := subscribe(t, "thready.test.*.post.state.p1")
    first := <-sub.Channel
    require.Equal(t, "running", first.Payload.(PostState).State) // snapshot first
}

// RED: invalidate must clear the sticky value.
func TestInvalidate_ClearsStickyValue(t *testing.T) {
    pub := newPublisher(t)
    _ = pub.PublishState(ctx, PostState{PostID: "p1", State: "done"})
    _ = pub.Invalidate(ctx, "acct", "p1")
    require.False(t, pub.lastValue.Has("thready.test.acct.post.state.p1"))
}
```

The 15 mandated test types apply: unit (above), integration (real JetStream container), chaos
(kill the consumer mid-stream, assert replay), stress (10k `post.received`/min), security (a
client cannot subscribe to another account's subject).

## 11. Open items

- `[OPEN: EVT-1]` JetStream retention window for the durable-replay backstop is
  `[DEFAULT — adjustable]` 7 days / 50 GB per stream; final sizing depends on the deployment
  pack's disk budget. Tracked as a workable item against the deployment area.
- `[OPEN: EVT-2]` Whether `post.state` sticky uses a dedicated compacted stream vs a KV bucket
  (`nats.KeyValue`) is an implementation choice; both satisfy last-value+invalidate. Decision
  deferred to the Event Bus Service build (`[BUILD-NEW]`).

---

*Made with love ♥ by Helix Development.*
