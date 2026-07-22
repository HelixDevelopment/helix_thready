package metubewebhook

import (
	"context"
	"errors"
	"sync"
	"time"
)

// Poller bridges MeTube's poll-only status API to an outbound completion
// webhook. On each Poll it reads the current job set from Source, detects jobs
// that have reached a terminal state (finished / error), and fires exactly one
// completion Envelope per job via Notifier. Delivery is de-duplicated by a fired
// set keyed on job ID, so a job that stays terminal across many polls is never
// notified twice.
type Poller struct {
	// Source supplies current MeTube job statuses.
	Source StatusSource
	// Notifier delivers completion envelopes (e.g. a WebhookSink).
	Notifier Notifier
	// Now stamps envelope timestamps; nil uses time.Now.
	Now func() time.Time

	mu    sync.Mutex
	fired map[string]bool
}

// NewPoller constructs a Poller over source and notifier.
func NewPoller(source StatusSource, notifier Notifier) *Poller {
	return &Poller{
		Source:   source,
		Notifier: notifier,
		fired:    make(map[string]bool),
	}
}

func (p *Poller) now() time.Time {
	if p.Now != nil {
		return p.Now()
	}
	return time.Now().UTC()
}

// AlreadyFired reports whether a completion webhook was already delivered for
// jobID.
func (p *Poller) AlreadyFired(jobID string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.fired[jobID]
}

// Poll performs one poll cycle. It returns the number of completion webhooks
// fired during this cycle and any errors encountered (source read + per-job
// delivery failures joined). A job whose delivery fails is NOT marked fired, so
// a subsequent Poll retries it; a job whose delivery succeeds is marked fired
// and never delivered again.
func (p *Poller) Poll(ctx context.Context) (int, error) {
	jobs, err := p.Source.Jobs(ctx)
	if err != nil {
		return 0, err
	}

	fired := 0
	var errs []error
	for _, job := range jobs {
		if !job.State.Terminal() {
			continue
		}

		p.mu.Lock()
		if p.fired[job.ID] {
			p.mu.Unlock()
			continue
		}
		p.mu.Unlock()

		env := EnvelopeFor(job, p.now())
		if derr := p.Notifier.Notify(ctx, env); derr != nil {
			errs = append(errs, derr)
			continue
		}

		p.mu.Lock()
		p.fired[job.ID] = true
		p.mu.Unlock()
		fired++
	}

	return fired, errors.Join(errs...)
}

// Run polls on a fixed interval until ctx is cancelled, returning ctx's error.
// Per-cycle delivery errors are surfaced to onError (if non-nil) and do not stop
// the loop.
func (p *Poller) Run(ctx context.Context, interval time.Duration, onError func(error)) error {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if _, err := p.Poll(ctx); err != nil && onError != nil {
				onError(err)
			}
		}
	}
}
