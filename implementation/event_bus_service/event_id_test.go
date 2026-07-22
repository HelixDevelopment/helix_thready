package eventbusservice

import (
	"fmt"
	"testing"
	"time"
)

// These tests close the coverage gap for the spec bullet
// "durable replay ... idempotent via event ID". They assert the actual
// Event.ID guarantees the contract relies on: the bus assigns a non-empty ID
// to every publish, IDs are unique across many publishes, the ID is STABLE
// across durable replay-from-cursor, and a consumer that dedupes on Event.ID
// processes each event exactly once even under overlapping (replay + live)
// redelivery. (helpers recvTimeout / expectNone live in bus_test.go.)

// TestEventID_AssignedNonEmpty_AndUnique proves that both Publish and
// PublishSticky stamp a non-empty Event.ID and that IDs do not collide across a
// large number of publishes.
func TestEventID_AssignedNonEmpty_AndUnique(t *testing.T) {
	b := NewDefault()
	defer b.Close()

	const n = 500
	ids := make(map[string]bool, n)
	for i := 0; i < n; i++ {
		var (
			e   Event
			err error
		)
		// Exercise both publish paths; the ID assignment is shared but this
		// guards against a regression on either entry point.
		ev := NewEvent(fmt.Sprintf("subj.%d", i%7), "t", i)
		if i%2 == 0 {
			e, err = b.Publish(ev)
		} else {
			e, err = b.PublishSticky(ev)
		}
		if err != nil {
			t.Fatalf("publish %d: %v", i, err)
		}
		if e.ID == "" {
			t.Fatalf("publish %d returned an empty Event.ID", i)
		}
		if ids[e.ID] {
			t.Fatalf("duplicate Event.ID %q at publish %d", e.ID, i)
		}
		ids[e.ID] = true
	}
	if len(ids) != n {
		t.Fatalf("unique IDs = %d, want %d", len(ids), n)
	}
}

// TestEventID_StableAcrossDurableReplay proves that the ID assigned to an event
// at publish time is the exact ID a durable subscriber sees when it replays that
// event from a cursor — both for a full replay and for a reconnect-from-cursor
// resume. Without stability, ID-based dedup on the consumer would be useless.
func TestEventID_StableAcrossDurableReplay(t *testing.T) {
	b := NewDefault()
	defer b.Close()

	// Publish a batch while nobody is subscribed; capture the assigned IDs.
	const n1 = 6
	want1 := make([]string, 0, n1)
	for i := 0; i < n1; i++ {
		e, err := b.Publish(NewEvent("s", "t", i))
		if err != nil {
			t.Fatalf("publish %d: %v", i, err)
		}
		if e.ID == "" {
			t.Fatalf("publish %d returned an empty Event.ID", i)
		}
		want1 = append(want1, e.ID)
	}

	// Durable replay from the start must return the SAME IDs, in publish order.
	sub := b.SubscribeDurable(Filter{Subject: "s"}, 0)
	var cursor int64
	for i, want := range want1 {
		got, ok := recvTimeout(t, sub.C, time.Second)
		if !ok {
			t.Fatalf("durable replay missing event %d", i)
		}
		if got.ID != want {
			t.Fatalf("replayed event %d ID = %q, want %q (ID not stable across replay)", i, got.ID, want)
		}
		cursor = got.Seq
	}
	b.Unsubscribe(sub)

	// More events arrive during the outage; capture their IDs too.
	const n2 = 4
	want2 := make([]string, 0, n2)
	for i := 0; i < n2; i++ {
		e, err := b.Publish(NewEvent("s", "t", 100+i))
		if err != nil {
			t.Fatalf("outage publish %d: %v", i, err)
		}
		want2 = append(want2, e.ID)
	}

	// Reconnect at the cursor: only the missed events replay, with identical IDs.
	sub2 := b.SubscribeDurable(Filter{Subject: "s"}, cursor)
	for i, want := range want2 {
		got, ok := recvTimeout(t, sub2.C, time.Second)
		if !ok {
			t.Fatalf("durable resume missing event %d", i)
		}
		if got.ID != want {
			t.Fatalf("resumed event %d ID = %q, want %q (ID not stable across resume)", i, got.ID, want)
		}
	}
	expectNone(t, sub2.C, 200*time.Millisecond)
}

// TestEventID_IdempotentDedupAcrossReplayAndLive demonstrates the actual
// idempotency guarantee: a consumer that keys a set on Event.ID sees each event
// exactly once even when the transport redelivers events (here modelled by a
// reconnect from an earlier cursor that replays an overlapping tail) and even as
// live events continue to flow. The raw delivery count deliberately exceeds the
// distinct event count, proving the overlap is real and that stable IDs are what
// make the consumer idempotent.
func TestEventID_IdempotentDedupAcrossReplayAndLive(t *testing.T) {
	b := NewDefault()
	defer b.Close()

	// Phase 1: a backlog published while nobody is subscribed.
	const backlog = 10
	publishedIDs := make([]string, 0, backlog)
	for i := 0; i < backlog; i++ {
		e, err := b.Publish(NewEvent("s", "t", i))
		if err != nil {
			t.Fatalf("backlog publish %d: %v", i, err)
		}
		publishedIDs = append(publishedIDs, e.ID)
	}

	// The consumer's idempotency store: Event.ID -> number of times delivered.
	seen := make(map[string]int)
	rawDeliveries := 0
	drainInto := func(sub *Subscription, count int) {
		for i := 0; i < count; i++ {
			got, ok := recvTimeout(t, sub.C, time.Second)
			if !ok {
				t.Fatalf("expected %d events from subscription, missing #%d", count, i)
			}
			if got.ID == "" {
				t.Fatalf("delivered event with empty ID: %+v", got)
			}
			seen[got.ID]++
			rawDeliveries++
		}
	}

	// First connection: replay the whole backlog from cursor 0.
	subA := b.SubscribeDurable(Filter{Subject: "s"}, 0)
	drainInto(subA, backlog)
	b.Unsubscribe(subA)

	// Reconnect from an EARLIER cursor than fully processed (a consumer that
	// lost some progress) → the tail with Seq > reconnectCursor replays AGAIN,
	// overlapping events already in `seen`.
	const reconnectCursor = 4
	overlap := backlog - reconnectCursor // events with Seq 5..10
	subB := b.SubscribeDurable(Filter{Subject: "s"}, int64(reconnectCursor))

	// Drain the overlapping replayed tail (redelivery of already-seen IDs).
	drainInto(subB, overlap)

	// Now live events flow to the same durable subscriber, interleaving the
	// "reconnect" story with fresh traffic.
	const live = 5
	liveIDs := make([]string, 0, live)
	for i := 0; i < live; i++ {
		e, err := b.Publish(NewEvent("s", "t", 100+i))
		if err != nil {
			t.Fatalf("live publish %d: %v", i, err)
		}
		liveIDs = append(liveIDs, e.ID)
	}
	drainInto(subB, live)

	// The overlap must actually have happened: raw deliveries exceed distinct.
	distinct := backlog + live
	if rawDeliveries <= distinct {
		t.Fatalf("expected redelivery overlap, got raw=%d distinct=%d", rawDeliveries, distinct)
	}
	if rawDeliveries != backlog+overlap+live {
		t.Fatalf("raw deliveries = %d, want %d", rawDeliveries, backlog+overlap+live)
	}

	// Keying on Event.ID collapses redelivery to exactly-once: every distinct
	// published/live event appears, and nothing extra.
	if len(seen) != distinct {
		t.Fatalf("distinct IDs seen = %d, want %d", len(seen), distinct)
	}
	for i, id := range publishedIDs {
		if seen[id] == 0 {
			t.Fatalf("backlog event %d (ID %q) never delivered", i, id)
		}
	}
	for i, id := range liveIDs {
		if seen[id] == 0 {
			t.Fatalf("live event %d (ID %q) never delivered", i, id)
		}
	}

	// Exactly the overlapping tail should have been delivered more than once —
	// concrete proof that stable IDs are what make the dedup idempotent.
	redelivered := 0
	for _, c := range seen {
		if c > 1 {
			redelivered++
		}
	}
	if redelivered != overlap {
		t.Fatalf("IDs delivered more than once = %d, want %d (the overlapping tail)", redelivered, overlap)
	}
}
