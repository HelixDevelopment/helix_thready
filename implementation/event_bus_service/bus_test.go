package eventbusservice

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

// recvTimeout waits up to d for one event on ch. ok is false on timeout or on a
// closed channel with no buffered value.
func recvTimeout(t *testing.T, ch <-chan Event, d time.Duration) (Event, bool) {
	t.Helper()
	select {
	case e, ok := <-ch:
		return e, ok
	case <-time.After(d):
		return Event{}, false
	}
}

// expectNone fails if any real event arrives on ch within d. A closed channel
// (ok == false) is treated as "no event".
func expectNone(t *testing.T, ch <-chan Event, d time.Duration) {
	t.Helper()
	select {
	case e, ok := <-ch:
		if ok {
			t.Fatalf("expected no event, but received %+v", e)
		}
	case <-time.After(d):
	}
}

func TestMatchSubject(t *testing.T) {
	cases := []struct {
		pattern, subject string
		want             bool
	}{
		{"", "anything.at.all", true},
		{">", "anything.at.all", true},
		{"post.received", "post.received", true},
		{"post.received", "post.processed", false},
		{"post.*", "post.received", true},
		{"post.*", "post.received.web", false}, // single-token wildcard
		{"post.>", "post.received", true},
		{"post.>", "post.received.web", true}, // multi-token tail
		{"post.>", "post", false},             // tail needs >=1 token
		{"*.received", "post.received", true},
		{"*.received", "asset.received", true},
		{"post.received", "post", false}, // token count must match
		{"post", "post.received", false},
	}
	for _, c := range cases {
		if got := MatchSubject(c.pattern, c.subject); got != c.want {
			t.Errorf("MatchSubject(%q, %q) = %v, want %v", c.pattern, c.subject, got, c.want)
		}
	}
}

func TestPublish_MatchingReceives_NonMatchingDoesNot(t *testing.T) {
	b := NewDefault()
	defer b.Close()

	matchSub := b.Subscribe(Filter{Subject: "post.received"})
	otherSub := b.Subscribe(Filter{Subject: "asset.stored"})

	if _, err := b.Publish(NewEvent("post.received", "post.received", "hello")); err != nil {
		t.Fatalf("publish: %v", err)
	}

	got, ok := recvTimeout(t, matchSub.C, time.Second)
	if !ok {
		t.Fatal("matching subscriber received nothing")
	}
	if got.Payload != "hello" {
		t.Fatalf("payload = %v, want hello", got.Payload)
	}
	expectNone(t, otherSub.C, 200*time.Millisecond)
}

func TestFilter_GlobAndMetadata(t *testing.T) {
	b := NewDefault()
	defer b.Close()

	// Glob subject + metadata predicate: post.* AND account_id=acc1.
	sub := b.Subscribe(Filter{
		Subject:  "post.*",
		Metadata: map[string]string{"account_id": "acc1"},
	})

	// Matches: subject glob ok, metadata ok.
	b.Publish(NewEvent("post.received", "post.received", "yes").WithMetadata("account_id", "acc1"))
	// Metadata mismatch.
	b.Publish(NewEvent("post.received", "post.received", "no-meta").WithMetadata("account_id", "acc2"))
	// Subject too deep for single-token glob.
	b.Publish(NewEvent("post.received.web", "post.received.web", "no-deep").WithMetadata("account_id", "acc1"))
	// Subject namespace mismatch.
	b.Publish(NewEvent("asset.stored", "asset.stored", "no-subj").WithMetadata("account_id", "acc1"))

	got, ok := recvTimeout(t, sub.C, time.Second)
	if !ok {
		t.Fatal("expected the matching event")
	}
	if got.Payload != "yes" {
		t.Fatalf("payload = %v, want yes", got.Payload)
	}
	expectNone(t, sub.C, 200*time.Millisecond)
}

func TestSticky_LateSubscriberGetsSnapshot_ThenInvalidateClears(t *testing.T) {
	b := NewDefault()
	defer b.Close()

	// Publish sticky BEFORE anyone subscribes.
	if _, err := b.PublishSticky(NewEvent("post.state.p1", "post.state", "running")); err != nil {
		t.Fatalf("publish sticky: %v", err)
	}

	// A late subscriber immediately receives the current sticky value.
	late := b.Subscribe(Filter{Subject: "post.state.p1"})
	got, ok := recvTimeout(t, late.C, time.Second)
	if !ok {
		t.Fatal("late subscriber did not receive the sticky snapshot")
	}
	if got.Payload != "running" {
		t.Fatalf("sticky payload = %v, want running", got.Payload)
	}

	// Invalidate clears it.
	b.Invalidate("post.state.p1")
	if _, ok := b.Sticky("post.state.p1"); ok {
		t.Fatal("sticky value still present after Invalidate")
	}

	// A subscriber joining AFTER invalidation gets nothing.
	later := b.Subscribe(Filter{Subject: "post.state.p1"})
	expectNone(t, later.C, 200*time.Millisecond)
}

func TestDurable_ReplayMissedAfterGap(t *testing.T) {
	b := NewDefault()
	defer b.Close()

	// Three events published while nobody is subscribed.
	b.Publish(NewEvent("s", "t", 1))
	b.Publish(NewEvent("s", "t", 2))
	e3, _ := b.Publish(NewEvent("s", "t", 3))

	// Durable subscribe from the beginning replays all three, in order.
	sub := b.SubscribeDurable(Filter{Subject: "s"}, 0)
	for _, want := range []int{1, 2, 3} {
		got, ok := recvTimeout(t, sub.C, time.Second)
		if !ok {
			t.Fatalf("durable replay missing event %d", want)
		}
		if got.Payload.(int) != want {
			t.Fatalf("replay payload = %v, want %d", got.Payload, want)
		}
	}

	// Consumer records its cursor and disconnects (the gap).
	cursor := e3.Seq
	b.Unsubscribe(sub)

	// Two more events arrive during the outage.
	b.Publish(NewEvent("s", "t", 4))
	b.Publish(NewEvent("s", "t", 5))

	// Reconnect from the cursor: only the missed events replay.
	sub2 := b.SubscribeDurable(Filter{Subject: "s"}, cursor)
	for _, want := range []int{4, 5} {
		got, ok := recvTimeout(t, sub2.C, time.Second)
		if !ok {
			t.Fatalf("durable resume missing event %d", want)
		}
		if got.Payload.(int) != want {
			t.Fatalf("resume payload = %v, want %d", got.Payload, want)
		}
	}
	expectNone(t, sub2.C, 200*time.Millisecond)
}

func TestConcurrentPublishers_RaceClean(t *testing.T) {
	b := NewDefault()
	defer b.Close()

	// A durable match-all subscriber must receive every event (at-least-once,
	// no drops) even under concurrent publishers.
	sub := b.SubscribeDurable(Filter{}, 0)

	const goroutines = 8
	const perG = 100
	total := goroutines * perG

	var wg sync.WaitGroup
	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for i := 0; i < perG; i++ {
				b.Publish(NewEvent("s", "t", fmt.Sprintf("%d-%d", g, i)))
			}
		}(g)
	}
	wg.Wait()

	seen := make(map[string]bool, total)
	for i := 0; i < total; i++ {
		got, ok := recvTimeout(t, sub.C, 2*time.Second)
		if !ok {
			t.Fatalf("durable subscriber only received %d of %d events", i, total)
		}
		id := got.Payload.(string)
		if seen[id] {
			t.Fatalf("duplicate delivery of %s", id)
		}
		seen[id] = true
	}
	if got := b.Metrics().Published; got != int64(total) {
		t.Fatalf("Metrics().Published = %d, want %d", got, total)
	}
}

func TestUnsubscribe_StopsDelivery(t *testing.T) {
	b := NewDefault()
	defer b.Close()

	sub := b.Subscribe(Filter{Subject: "s"})
	b.Publish(NewEvent("s", "t", "a"))

	got, ok := recvTimeout(t, sub.C, time.Second)
	if !ok || got.Payload != "a" {
		t.Fatalf("before unsubscribe: got %+v ok=%v, want payload a", got, ok)
	}

	b.Unsubscribe(sub)
	b.Publish(NewEvent("s", "t", "b"))

	// No "b" should arrive; the channel is closed (ok == false).
	if e, ok := recvTimeout(t, sub.C, 300*time.Millisecond); ok {
		t.Fatalf("received %+v after unsubscribe, want none", e)
	}
}

func TestPublishAsync_OrderingAtLeastOnce(t *testing.T) {
	b := NewDefault()
	defer b.Close()

	sub := b.SubscribeDurable(Filter{Subject: "s"}, 0)

	const n = 50
	for i := 0; i < n; i++ {
		if err := b.PublishAsync(NewEvent("s", "t", i)); err != nil {
			t.Fatalf("PublishAsync(%d): %v", i, err)
		}
	}
	b.Drain()

	// FIFO ordering preserved by the single dispatcher; every event delivered.
	for i := 0; i < n; i++ {
		got, ok := recvTimeout(t, sub.C, time.Second)
		if !ok {
			t.Fatalf("async event %d not delivered", i)
		}
		if got.Payload.(int) != i {
			t.Fatalf("async out of order: got %v, want %d", got.Payload, i)
		}
	}
	if got := b.Metrics().Published; got < int64(n) {
		t.Fatalf("Metrics().Published = %d, want >= %d", got, n)
	}
}

func TestMetrics_PublishedDeliveredDropped(t *testing.T) {
	// Tiny buffer + a subscriber that never reads => live overflow drops.
	b := New(Config{BufferSize: 2, AsyncBufferSize: 8})
	defer b.Close()

	_ = b.Subscribe(Filter{Subject: "s"}) // never drained

	const n = 20
	for i := 0; i < n; i++ {
		b.Publish(NewEvent("s", "t", i))
	}

	m := b.Metrics()
	if m.Published != int64(n) {
		t.Fatalf("Published = %d, want %d", m.Published, n)
	}
	if m.Delivered+m.Dropped != int64(n) {
		t.Fatalf("Delivered(%d)+Dropped(%d) = %d, want %d", m.Delivered, m.Dropped, m.Delivered+m.Dropped, n)
	}
	if m.Dropped == 0 {
		t.Fatal("expected some drops with a full unread buffer")
	}
}

func TestInvalidate_NotifiesLiveSubscriber(t *testing.T) {
	b := NewDefault()
	defer b.Close()

	b.PublishSticky(NewEvent("post.state.p1", "post.state", "done"))

	sub := b.Subscribe(Filter{Subject: "post.state.p1"})
	// First the sticky snapshot.
	if got, ok := recvTimeout(t, sub.C, time.Second); !ok || got.Payload != "done" {
		t.Fatalf("snapshot: got %+v ok=%v", got, ok)
	}

	b.Invalidate("post.state.p1")
	got, ok := recvTimeout(t, sub.C, time.Second)
	if !ok {
		t.Fatal("connected subscriber did not receive invalidation notification")
	}
	if got.Type != TypeStickyInvalidated || got.Subject != "post.state.p1" {
		t.Fatalf("notification = %+v, want type %s subject post.state.p1", got, TypeStickyInvalidated)
	}
}
