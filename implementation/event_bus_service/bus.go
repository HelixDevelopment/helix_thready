package eventbusservice

import (
	"errors"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

// TypeStickyInvalidated is the Type of the synthetic notification the bus fans
// out to live subscribers when a sticky value is invalidated. It is a live-only
// signal — it is not written to the durable log and is not retained.
const TypeStickyInvalidated = "eventbus.sticky.invalidated"

// ErrClosed is returned by publish operations after the bus has been closed.
var ErrClosed = errors.New("eventbusservice: bus is closed")

// Config tunes a Bus.
type Config struct {
	// BufferSize is the per-live-subscriber channel buffer. Live events that
	// overflow are dropped (counted in Metrics.Dropped). Ignored by durable
	// subscribers, which never drop.
	BufferSize int
	// AsyncBufferSize bounds the PublishAsync hand-off queue.
	AsyncBufferSize int
}

// Metrics is a snapshot of the bus counters.
type Metrics struct {
	// Published counts real events accepted by Publish/PublishSticky/PublishAsync
	// (not sticky-invalidation notifications).
	Published int64
	// Delivered counts individual successful deliveries into subscriber channels
	// (one publish to N subscribers is N deliveries).
	Delivered int64
	// Dropped counts live deliveries skipped because a subscriber buffer was full.
	Dropped int64
}

// Bus is a typed in-process publish/subscribe bus with sticky events and durable
// replay. The zero value is not usable; construct one with New or NewDefault.
type Bus struct {
	cfg Config

	mu     sync.Mutex
	subs   map[uint64]*Subscription
	nextID uint64
	seq    int64
	log    []Event          // ordered durable journal (the "stream")
	sticky map[string]Event // per-subject last value (the compacted store)
	closed bool

	// async dispatcher
	asyncCh      chan Event
	asyncCond    *sync.Cond // on mu; broadcast when asyncPending hits 0
	asyncPending int
	asyncWG      sync.WaitGroup
	stop         chan struct{}

	// counters (atomic)
	published int64
	delivered int64
	dropped   int64
}

// NewDefault constructs a Bus with sensible defaults (BufferSize 64,
// AsyncBufferSize 256).
func NewDefault() *Bus {
	return New(Config{BufferSize: 64, AsyncBufferSize: 256})
}

// New constructs a Bus with the given config, filling in defaults for any
// non-positive field, and starts the async dispatcher.
func New(cfg Config) *Bus {
	if cfg.BufferSize <= 0 {
		cfg.BufferSize = 64
	}
	if cfg.AsyncBufferSize <= 0 {
		cfg.AsyncBufferSize = 256
	}
	b := &Bus{
		cfg:     cfg,
		subs:    make(map[uint64]*Subscription),
		sticky:  make(map[string]Event),
		asyncCh: make(chan Event, cfg.AsyncBufferSize),
		stop:    make(chan struct{}),
	}
	b.asyncCond = sync.NewCond(&b.mu)
	b.asyncWG.Add(1)
	go b.dispatch()
	return b
}

// Publish delivers a one-time event to every currently-subscribed matching
// subscriber and appends it to the durable log for later replay. It returns the
// event enriched with its assigned ID and Seq.
func (b *Bus) Publish(e Event) (Event, error) {
	return b.publish(e, false)
}

// PublishSticky publishes e as a one-time event (like Publish) AND retains it as
// the last value for its Subject, so a subscriber that joins later immediately
// receives this snapshot before any live event. The retained value is replaced
// by the next sticky publish for the same Subject or cleared by Invalidate.
func (b *Bus) PublishSticky(e Event) (Event, error) {
	return b.publish(e, true)
}

func (b *Bus) publish(e Event, sticky bool) (Event, error) {
	b.mu.Lock()
	if b.closed {
		b.mu.Unlock()
		return e, ErrClosed
	}
	b.seq++
	e.Seq = b.seq
	if e.ID == "" {
		e.ID = newID()
	}
	if e.TimestampUnix == 0 {
		e.TimestampUnix = time.Now().Unix()
	}
	b.log = append(b.log, e)
	if sticky {
		b.sticky[e.Subject] = e
	}
	targets := make([]*Subscription, 0, len(b.subs))
	for _, s := range b.subs {
		if s.Filter.Matches(e) {
			targets = append(targets, s)
		}
	}
	b.mu.Unlock()

	atomic.AddInt64(&b.published, 1)
	for _, s := range targets {
		s.deliver(e)
	}
	return e, nil
}

// PublishAsync enqueues e for publication by the bus's single dispatcher
// goroutine and returns immediately. Ordering across PublishAsync calls is
// preserved (FIFO). Use Drain to wait until all queued async publishes have been
// processed.
func (b *Bus) PublishAsync(e Event) error {
	b.mu.Lock()
	if b.closed {
		b.mu.Unlock()
		return ErrClosed
	}
	b.asyncPending++
	b.mu.Unlock()

	select {
	case b.asyncCh <- e:
		return nil
	case <-b.stop:
		b.mu.Lock()
		b.asyncPending--
		if b.asyncPending == 0 {
			b.asyncCond.Broadcast()
		}
		b.mu.Unlock()
		return ErrClosed
	}
}

func (b *Bus) dispatch() {
	defer b.asyncWG.Done()
	handle := func(e Event) {
		b.publish(e, false)
		b.mu.Lock()
		b.asyncPending--
		if b.asyncPending == 0 {
			b.asyncCond.Broadcast()
		}
		b.mu.Unlock()
	}
	for {
		select {
		case e := <-b.asyncCh:
			handle(e)
		case <-b.stop:
			for {
				select {
				case e := <-b.asyncCh:
					handle(e)
				default:
					return
				}
			}
		}
	}
}

// Drain blocks until every event handed to PublishAsync so far has been
// published.
func (b *Bus) Drain() {
	b.mu.Lock()
	for b.asyncPending > 0 {
		b.asyncCond.Wait()
	}
	b.mu.Unlock()
}

// Subscribe returns a live subscription for events matching filter. If a sticky
// value is currently retained for a subject the filter matches, it is delivered
// first (snapshot-before-live ordering), then live events follow.
func (b *Bus) Subscribe(filter Filter) *Subscription {
	return b.subscribe(filter, false, 0, false)
}

// SubscribeAll is Subscribe with a match-everything filter.
func (b *Bus) SubscribeAll() *Subscription {
	return b.subscribe(Filter{}, false, 0, false)
}

// SubscribeDurable returns a durable subscription: it first replays every logged
// event matching filter with Seq > afterSeq (pass 0 to replay the whole retained
// log), then delivers live events, all in publish order and without dropping.
// This is the reconnect path — a consumer resumes from the highest Seq it has
// already processed. Consumers must dedupe on Event.ID to stay idempotent under
// at-least-once redelivery.
func (b *Bus) SubscribeDurable(filter Filter, afterSeq int64) *Subscription {
	return b.subscribe(filter, true, afterSeq, false)
}

func (b *Bus) subscribe(filter Filter, durable bool, afterSeq int64, _ bool) *Subscription {
	b.mu.Lock()
	b.nextID++
	s := &Subscription{
		ID:      b.nextID,
		Filter:  filter,
		durable: durable,
		bus:     b,
	}
	if durable {
		s.out = make(chan Event)
		s.done = make(chan struct{})
		s.cond = sync.NewCond(&s.mu)
		// Replay backlog: every logged event past the cursor that matches.
		var backlog []Event
		for _, e := range b.log {
			if e.Seq > afterSeq && filter.Matches(e) {
				backlog = append(backlog, e)
			}
		}
		s.seedLocked(backlog)
	} else {
		s.out = make(chan Event, b.cfg.BufferSize)
		// Deliver current sticky snapshot(s) first, in publish order.
		var snaps []Event
		for _, e := range b.sticky {
			if filter.Matches(e) {
				snaps = append(snaps, e)
			}
		}
		sort.Slice(snaps, func(i, j int) bool { return snaps[i].Seq < snaps[j].Seq })
		s.seedLocked(snaps)
	}
	s.C = s.out
	b.subs[s.ID] = s
	b.mu.Unlock()

	if durable {
		go s.pump()
	}
	return s
}

// Unsubscribe cancels sub: it receives no further events and its channel C is
// closed once outstanding buffered events are drained.
func (b *Bus) Unsubscribe(sub *Subscription) {
	if sub == nil {
		return
	}
	b.mu.Lock()
	delete(b.subs, sub.ID)
	b.mu.Unlock()
	sub.shutdown()
}

// Invalidate clears the retained sticky value for subject (a subscriber that
// joins afterwards receives no snapshot for it) and fans out a live
// TypeStickyInvalidated notification to currently-connected matching subscribers
// so they can drop any stale value they are showing. The notification is not
// logged or retained.
func (b *Bus) Invalidate(subject string) {
	b.mu.Lock()
	if b.closed {
		b.mu.Unlock()
		return
	}
	_, had := b.sticky[subject]
	delete(b.sticky, subject)
	n := Event{
		ID:            newID(),
		Type:          TypeStickyInvalidated,
		Subject:       subject,
		TimestampUnix: time.Now().Unix(),
	}
	var targets []*Subscription
	if had {
		for _, s := range b.subs {
			if s.Filter.Matches(n) {
				targets = append(targets, s)
			}
		}
	}
	b.mu.Unlock()

	for _, s := range targets {
		s.deliver(n)
	}
}

// Sticky returns the retained last value for subject, if any.
func (b *Bus) Sticky(subject string) (Event, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	e, ok := b.sticky[subject]
	return e, ok
}

// Metrics returns a snapshot of the bus counters.
func (b *Bus) Metrics() Metrics {
	return Metrics{
		Published: atomic.LoadInt64(&b.published),
		Delivered: atomic.LoadInt64(&b.delivered),
		Dropped:   atomic.LoadInt64(&b.dropped),
	}
}

// Close stops accepting new publishes, drains any queued async publishes, stops
// the dispatcher, and cancels every subscription (closing their channels). It is
// idempotent.
func (b *Bus) Close() {
	b.mu.Lock()
	if b.closed {
		b.mu.Unlock()
		return
	}
	b.mu.Unlock()

	// Drain queued async work while the bus is still open so nothing is lost.
	b.Drain()

	b.mu.Lock()
	b.closed = true
	subs := make([]*Subscription, 0, len(b.subs))
	for _, s := range b.subs {
		subs = append(subs, s)
	}
	b.subs = make(map[uint64]*Subscription)
	b.mu.Unlock()

	close(b.stop)
	b.asyncWG.Wait()

	for _, s := range subs {
		s.shutdown()
	}
}
