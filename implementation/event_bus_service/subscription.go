package eventbusservice

import (
	"sync"
	"sync/atomic"
)

// Subscription is a live handle returned by Subscribe/SubscribeAll/
// SubscribeDurable. Consumers read events from C until the subscription is
// cancelled with Bus.Unsubscribe (or the bus is closed), at which point C is
// closed.
//
// Two delivery disciplines share this type:
//
//   - Live (non-durable) subscriptions use a bounded buffered channel. When a
//     slow consumer lets the buffer fill, further live events for it are dropped
//     and counted in Metrics.Dropped — best-effort fan-out.
//   - Durable subscriptions are backed by an unbounded internal queue drained by
//     a dedicated pump goroutine with blocking sends, so they never drop:
//     at-least-once is provided by the queue plus replay-from-cursor over the
//     bus append log.
type Subscription struct {
	// ID is the bus-unique subscription id.
	ID uint64
	// Filter is the selector this subscription was created with.
	Filter Filter
	// C is the receive side consumers read events from.
	C <-chan Event

	bus     *Bus
	out     chan Event
	durable bool

	mu     sync.Mutex
	cond   *sync.Cond // durable only: signalled on enqueue/close
	queue  []Event    // durable only: pending events in publish order
	closed bool
	done   chan struct{} // durable only: closed to unblock a stuck pump send

	closeOutOnce  sync.Once
	closeDoneOnce sync.Once
}

// deliver routes an event to the subscription using its discipline.
func (s *Subscription) deliver(e Event) {
	if s.durable {
		s.enqueue(e)
		return
	}
	s.trySend(e)
}

// trySend is the live (non-durable) path: non-blocking send, drop on overflow.
func (s *Subscription) trySend(e Event) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return
	}
	select {
	case s.out <- e:
		atomic.AddInt64(&s.bus.delivered, 1)
	default:
		atomic.AddInt64(&s.bus.dropped, 1)
	}
}

// enqueue is the durable path: append to the internal queue and wake the pump.
func (s *Subscription) enqueue(e Event) {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return
	}
	s.queue = append(s.queue, e)
	s.cond.Signal()
	s.mu.Unlock()
}

// seedLocked appends events to a not-yet-registered subscription's backlog. It
// is called by the bus while holding the bus lock and before the subscription
// is visible to publishers, so no synchronisation with concurrent delivery is
// required here; the subsequent goroutine start / mutex acquisition establishes
// the happens-before edge.
func (s *Subscription) seedLocked(events []Event) {
	if s.durable {
		s.queue = append(s.queue, events...)
		return
	}
	for _, e := range events {
		select {
		case s.out <- e:
			atomic.AddInt64(&s.bus.delivered, 1)
		default:
			atomic.AddInt64(&s.bus.dropped, 1)
		}
	}
}

// pump drains the durable queue to the consumer in FIFO order with blocking
// sends (backpressure, never drop). It closes C when the subscription ends.
func (s *Subscription) pump() {
	for {
		s.mu.Lock()
		for len(s.queue) == 0 && !s.closed {
			s.cond.Wait()
		}
		if len(s.queue) == 0 && s.closed {
			s.mu.Unlock()
			s.closeOut()
			return
		}
		e := s.queue[0]
		s.queue = s.queue[1:]
		s.mu.Unlock()

		select {
		case s.out <- e:
			atomic.AddInt64(&s.bus.delivered, 1)
		case <-s.done:
			s.closeOut()
			return
		}
	}
}

// shutdown cancels the subscription and arranges for C to be closed.
func (s *Subscription) shutdown() {
	if s.durable {
		s.mu.Lock()
		s.closed = true
		s.cond.Signal()
		s.mu.Unlock()
		s.closeDoneOnce.Do(func() { close(s.done) })
		return
	}
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return
	}
	s.closed = true
	close(s.out)
	s.mu.Unlock()
}

func (s *Subscription) closeOut() {
	s.closeOutOnce.Do(func() { close(s.out) })
}
