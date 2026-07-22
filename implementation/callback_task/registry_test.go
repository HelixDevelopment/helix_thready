package callbacktask

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

// spyNotifier is an in-memory Notifier that records every Envelope it receives.
type spyNotifier struct {
	mu   sync.Mutex
	envs []Envelope
}

func (s *spyNotifier) Notify(_ context.Context, env Envelope) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.envs = append(s.envs, env)
	return nil
}

func (s *spyNotifier) count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.envs)
}

func (s *spyNotifier) countState(st State) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := 0
	for _, e := range s.envs {
		if e.State == st {
			n++
		}
	}
	return n
}

func mustSubmit(t *testing.T, r *Registry) Task {
	t.Helper()
	task, err := r.Submit(context.Background(), "download", []byte(`{"url":"http://example/x"}`))
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	return task
}

func TestStateTransitions_HappyPath(t *testing.T) {
	ctx := context.Background()
	r := New()

	task := mustSubmit(t, r)
	if task.State != StateQueued {
		t.Fatalf("submit: want queued, got %s", task.State)
	}

	if _, err := r.Start(ctx, task.ID); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if _, err := r.Progress(ctx, task.ID, 0.5); err != nil {
		t.Fatalf("Progress: %v", err)
	}
	got, err := r.Complete(ctx, task.ID, "asset:done-123")
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if got.State != StateSucceeded {
		t.Fatalf("complete: want succeeded, got %s", got.State)
	}
	if got.Progress != 1.0 {
		t.Fatalf("complete: want progress 1.0, got %v", got.Progress)
	}
	if got.ResultRef != "asset:done-123" {
		t.Fatalf("complete: want result_ref, got %q", got.ResultRef)
	}

	env, err := r.Status(task.ID)
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if env.State != StateSucceeded || env.TaskID != task.ID {
		t.Fatalf("status mismatch: %+v", env)
	}
}

func TestInvalidTransitions(t *testing.T) {
	ctx := context.Background()
	r := New()
	task := mustSubmit(t, r)

	// Cannot complete a queued task (must be running).
	if _, err := r.Complete(ctx, task.ID, "asset:x"); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("complete-from-queued: want ErrInvalidTransition, got %v", err)
	}
	// Cannot progress a queued task.
	if _, err := r.Progress(ctx, task.ID, 0.3); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("progress-from-queued: want ErrInvalidTransition, got %v", err)
	}
	// Progress out of range.
	if _, err := r.Start(ctx, task.ID); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if _, err := r.Progress(ctx, task.ID, 1.5); !errors.Is(err, ErrInvalidProgress) {
		t.Fatalf("progress-oor: want ErrInvalidProgress, got %v", err)
	}
	// Unknown id.
	if _, err := r.Start(ctx, "task-does-not-exist"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("start-unknown: want ErrNotFound, got %v", err)
	}
}

func TestRetryThenSuccess(t *testing.T) {
	ctx := context.Background()
	r := New(WithMaxAttempts(3), WithBackoff(func(int) time.Duration { return time.Second }))

	task := mustSubmit(t, r)
	if _, err := r.Start(ctx, task.ID); err != nil {
		t.Fatalf("Start: %v", err)
	}

	// First attempt fails with a retryable error.
	got, err := r.Fail(ctx, task.ID, JobError{Code: "net", Message: "reset by peer", Retryable: true})
	if err != nil {
		t.Fatalf("Fail: %v", err)
	}
	if got.State != StateRetrying {
		t.Fatalf("fail-1: want retrying, got %s", got.State)
	}
	if got.Attempts != 1 {
		t.Fatalf("fail-1: want attempts 1, got %d", got.Attempts)
	}
	if got.NextRetryAt.IsZero() {
		t.Fatalf("fail-1: expected NextRetryAt to be scheduled")
	}

	// Worker retries: retrying -> running -> succeeded.
	if _, err := r.Start(ctx, task.ID); err != nil {
		t.Fatalf("retry Start: %v", err)
	}
	final, err := r.Complete(ctx, task.ID, "asset:after-retry")
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if final.State != StateSucceeded {
		t.Fatalf("final: want succeeded, got %s", final.State)
	}
	if final.Attempts != 1 {
		t.Fatalf("final: attempts should remain 1, got %d", final.Attempts)
	}
}

func TestExhaustionDeadAndDLQ(t *testing.T) {
	ctx := context.Background()
	r := New(WithMaxAttempts(3), WithBackoff(func(int) time.Duration { return 0 }))

	task := mustSubmit(t, r)

	// Fail 3 times (each attempt: start -> fail). The 3rd fail exhausts.
	var last Task
	for i := 1; i <= 3; i++ {
		if _, err := r.Start(ctx, task.ID); err != nil {
			t.Fatalf("attempt %d Start: %v", i, err)
		}
		var err error
		last, err = r.Fail(ctx, task.ID, JobError{Code: "5xx", Message: "upstream error", Retryable: true})
		if err != nil {
			t.Fatalf("attempt %d Fail: %v", i, err)
		}
	}

	if last.State != StateDead {
		t.Fatalf("exhaustion: want dead, got %s", last.State)
	}
	if last.Attempts != 3 {
		t.Fatalf("exhaustion: want attempts 3, got %d", last.Attempts)
	}

	dl := r.DeadLetters()
	if len(dl) != 1 {
		t.Fatalf("DLQ: want 1 dead-lettered task, got %d", len(dl))
	}
	if dl[0].ID != task.ID || dl[0].State != StateDead {
		t.Fatalf("DLQ: wrong task: %+v", dl[0])
	}

	// A dead task cannot be started again.
	if _, err := r.Start(ctx, task.ID); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("start-dead: want ErrInvalidTransition, got %v", err)
	}
}

func TestNonRetryableFails(t *testing.T) {
	ctx := context.Background()
	r := New(WithMaxAttempts(5))
	task := mustSubmit(t, r)
	if _, err := r.Start(ctx, task.ID); err != nil {
		t.Fatalf("Start: %v", err)
	}
	got, err := r.Fail(ctx, task.ID, JobError{Code: "bad_input", Message: "unsupported url", Retryable: false})
	if err != nil {
		t.Fatalf("Fail: %v", err)
	}
	if got.State != StateFailed {
		t.Fatalf("non-retryable: want failed, got %s", got.State)
	}
	// Non-retryable failure is not dead-lettered.
	if n := len(r.DeadLetters()); n != 0 {
		t.Fatalf("non-retryable: DLQ should be empty, got %d", n)
	}
}

func TestIdempotentDoubleComplete(t *testing.T) {
	ctx := context.Background()
	spy := &spyNotifier{}
	r := New(WithNotifier(spy))

	task := mustSubmit(t, r) // notify: queued
	if _, err := r.Start(ctx, task.ID); err != nil {
		t.Fatalf("Start: %v", err)
	} // notify: running

	first, err := r.Complete(ctx, task.ID, "asset:final")
	if err != nil {
		t.Fatalf("Complete #1: %v", err)
	} // notify: succeeded
	if first.State != StateSucceeded {
		t.Fatalf("complete #1: want succeeded, got %s", first.State)
	}

	countBefore := spy.count()

	// Completing an already-done task is a no-op: no error, no new notification.
	second, err := r.Complete(ctx, task.ID, "asset:should-be-ignored")
	if err != nil {
		t.Fatalf("Complete #2 (idempotent): %v", err)
	}
	if second.State != StateSucceeded {
		t.Fatalf("complete #2: want succeeded, got %s", second.State)
	}
	if second.ResultRef != "asset:final" {
		t.Fatalf("complete #2 must not overwrite result_ref, got %q", second.ResultRef)
	}
	if spy.count() != countBefore {
		t.Fatalf("idempotent double-complete fired a duplicate notification: %d -> %d", countBefore, spy.count())
	}
	if n := spy.countState(StateSucceeded); n != 1 {
		t.Fatalf("succeeded must be delivered exactly once, got %d", n)
	}
}

func TestDefaultBackoffArithmetic(t *testing.T) {
	// defaultBackoff(attempt) = 100ms * 2^(attempt-1), attempt clamped to >= 1.
	cases := []struct {
		attempt int
		want    time.Duration
	}{
		{-3, 100 * time.Millisecond}, // clamped to attempt 1
		{0, 100 * time.Millisecond},  // clamped to attempt 1
		{1, 100 * time.Millisecond},
		{2, 200 * time.Millisecond},
		{3, 400 * time.Millisecond},
		{4, 800 * time.Millisecond},
		{5, 1600 * time.Millisecond},
	}
	for _, c := range cases {
		if got := defaultBackoff(c.attempt); got != c.want {
			t.Fatalf("defaultBackoff(%d) = %v, want %v", c.attempt, got, c.want)
		}
	}
}

func TestStateTerminal(t *testing.T) {
	for _, s := range []State{StateSucceeded, StateFailed, StateDead} {
		if !s.Terminal() {
			t.Fatalf("State %q should be terminal", s)
		}
	}
	for _, s := range []State{StateQueued, StateRunning, StateRetrying} {
		if s.Terminal() {
			t.Fatalf("State %q should NOT be terminal", s)
		}
	}
}

func TestConcurrentSubmitRaceClean(t *testing.T) {
	ctx := context.Background()
	r := New()

	const n = 300
	var wg sync.WaitGroup
	ids := make(chan string, n)

	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			task, err := r.Submit(ctx, "download", []byte(`{"url":"http://example/y"}`))
			if err != nil {
				t.Errorf("Submit: %v", err)
				return
			}
			ids <- task.ID
			// Exercise concurrent read paths against the writers too.
			_, _ = r.Status(task.ID)
			_ = r.DeadLetters()
			_ = r.Len()
		}()
	}
	wg.Wait()
	close(ids)

	seen := make(map[string]bool, n)
	for id := range ids {
		if seen[id] {
			t.Fatalf("duplicate task id generated: %s", id)
		}
		seen[id] = true
	}
	if len(seen) != n {
		t.Fatalf("want %d unique tasks, got %d", n, len(seen))
	}
	if r.Len() != n {
		t.Fatalf("registry Len: want %d, got %d", n, r.Len())
	}
}
