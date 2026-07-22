package downloadmanager

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
)

// TestEnqueueShutdownNoPanic is the regression guard for the Enqueue/Shutdown
// TOCTOU race.
//
// Root cause (pre-fix): Enqueue checked m.stopped under m.mu, released m.mu, and
// only THEN sent on m.queue. If Shutdown ran in that window it set stopped and
// close(m.queue) under m.mu, so the subsequent `m.queue <- j` panicked with
// "send on closed channel" and crashed the process (no recover on that path).
//
// This test drives many concurrent Enqueue callers against a concurrent Shutdown
// under stress. Each Enqueue call runs the panic-prone send in the caller's own
// goroutine, so a per-goroutine recover deterministically captures the panic as
// the failure signal. Pre-fix this fails (a send lands on the closed channel);
// post-fix the send happens under the same lock as the stopped check, so Enqueue
// either enqueues cleanly or returns an error, but never panics.
func TestEnqueueShutdownNoPanic(t *testing.T) {
	const (
		attempts  = 300
		enqueuers = 64
	)

	for attempt := 0; attempt < attempts; attempt++ {
		m := New(Config{Workers: 4})
		m.Start()

		var (
			wg          sync.WaitGroup
			panicked    atomic.Bool
			panicVal    atomic.Value
			enqueuedOK  atomic.Int64
			rejectedErr atomic.Int64
			start       = make(chan struct{})
		)

		for i := 0; i < enqueuers; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				defer func() {
					if r := recover(); r != nil {
						panicked.Store(true)
						panicVal.Store(r)
					}
				}()
				<-start
				// Spin enqueuing until the manager rejects us (stopped). Each
				// iteration re-runs the check-then-send sequence, maximising the
				// chance that Shutdown's close() lands inside the window.
				for {
					// ftp:// resolves to the honest stub fetcher: no network, so
					// workers drain the queue near-instantly and the buffer never
					// fills. Keeps the test about the Enqueue/Shutdown race only.
					_, err := m.Enqueue(TaskSpec{URL: "ftp://example.invalid/f", DestPath: "/dev/null"})
					if err != nil {
						rejectedErr.Add(1)
						return
					}
					enqueuedOK.Add(1)
				}
			}()
		}

		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_ = m.Shutdown(context.Background())
		}()

		close(start)
		wg.Wait()

		if panicked.Load() {
			t.Fatalf("attempt %d: Enqueue panicked racing with Shutdown: %v", attempt, panicVal.Load())
		}
	}
}

// TestEnqueueAfterShutdownReturnsError is the deterministic companion: once
// Shutdown has completed, Enqueue must reject with a clear error and never panic.
func TestEnqueueAfterShutdownReturnsError(t *testing.T) {
	m := New(Config{Workers: 2})
	m.Start()

	if err := m.Shutdown(context.Background()); err != nil {
		t.Fatalf("shutdown: %v", err)
	}

	var (
		id       string
		err      error
		panicked bool
	)
	func() {
		defer func() {
			if r := recover(); r != nil {
				panicked = true
			}
		}()
		id, err = m.Enqueue(TaskSpec{URL: "ftp://example.invalid/f", DestPath: "/dev/null"})
	}()

	if panicked {
		t.Fatal("Enqueue after Shutdown panicked; want a returned error")
	}
	if err == nil {
		t.Fatalf("Enqueue after Shutdown returned id=%q, nil error; want a stopped error", id)
	}
}
