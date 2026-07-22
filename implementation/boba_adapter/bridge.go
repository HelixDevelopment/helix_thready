package bobaadapter

import (
	"context"
	"sync"
	"time"
)

// Bridge normalizes Boba's terminal download events into the shared Helix
// Thready callback Envelope and fires exactly one HMAC-signed webhook per
// distinct result. It is the join point: Boba's bespoke events go in (via
// Handle, or via Consume over an EventSource), and one standard signed callback
// per result id comes out to a downstream sink.
//
// Delivery is de-duplicated by a fired set keyed on the event's dedup key
// (ResultID, falling back to SearchID), so a result whose terminal event is
// re-delivered — an SSE re-emit, a hook retry from Boba, an at-least-once
// stream — never fires the downstream callback twice.
type Bridge struct {
	// Sink delivers completion envelopes (e.g. a WebhookSink). Required.
	Sink Notifier
	// Now stamps envelope timestamps; nil uses time.Now().UTC().
	Now func() time.Time

	mu    sync.Mutex
	fired map[string]bool
}

// NewBridge constructs a Bridge delivering to sink.
func NewBridge(sink Notifier) *Bridge {
	return &Bridge{
		Sink:  sink,
		fired: make(map[string]bool),
	}
}

func (b *Bridge) now() time.Time {
	if b.Now != nil {
		return b.Now()
	}
	return time.Now().UTC()
}

// dedupKey is the identity a terminal event is de-duplicated on: the stable
// result id, falling back to the search id if a download event carries no id.
func dedupKey(ev BobaEvent) string {
	return firstNonEmpty(ev.ResultID, ev.SearchID)
}

// AlreadyFired reports whether a completion webhook was already delivered for
// the given dedup key.
func (b *Bridge) AlreadyFired(key string) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.fired[key]
}

// Handle processes one normalized Boba event. Non-terminal events (result_found,
// download_started, download_progress) are ignored and return (false, nil). A
// terminal event (download_complete / download_error) that has not already fired
// is mapped to a standard Envelope, signed, and delivered via Sink; on success
// it is marked fired and Handle returns (true, nil). A delivery failure returns
// (false, err) and does NOT mark the key fired, so a later re-delivery of the
// same event retries it. An already-fired key returns (false, nil).
func (b *Bridge) Handle(ctx context.Context, ev BobaEvent) (bool, error) {
	if !ev.Type.Terminal() {
		return false, nil
	}
	key := dedupKey(ev)

	b.mu.Lock()
	if b.fired[key] {
		b.mu.Unlock()
		return false, nil
	}
	b.mu.Unlock()

	env := EnvelopeFor(ev, b.now())
	if err := b.Sink.Notify(ctx, env); err != nil {
		return false, err
	}

	b.mu.Lock()
	b.fired[key] = true
	b.mu.Unlock()
	return true, nil
}

// Consume streams events from src and feeds each through Handle until the source
// ends, the context is cancelled, or delivery fails. It wires an EventSource
// (e.g. an SSEReader) straight into the Bridge.
func (b *Bridge) Consume(ctx context.Context, src EventSource) error {
	return src.Stream(ctx, func(ev BobaEvent) error {
		_, err := b.Handle(ctx, ev)
		return err
	})
}
