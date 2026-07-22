package callbacktask

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// Sentinel errors returned by the Registry.
var (
	// ErrNotFound is returned when a task id is unknown.
	ErrNotFound = errors.New("callbacktask: task not found")
	// ErrInvalidTransition is returned when a state transition is not allowed
	// from the task's current state.
	ErrInvalidTransition = errors.New("callbacktask: invalid state transition")
	// ErrInvalidProgress is returned when a progress value is outside 0.0..1.0.
	ErrInvalidProgress = errors.New("callbacktask: progress out of range")
)

// Notifier receives status Envelopes as tasks transition. A WebhookSink is the
// primary implementation, but any sink (event-bus publish, in-memory spy) works.
//
// A sink owns only its in-flight retry/back-off: WebhookSink retries transport
// errors and non-2xx responses with back-off, then returns an error once its
// attempts are exhausted. It does NOT persist or dead-letter that final failure.
// Capturing the final failure is the Registry's job: it does not discard the
// returned error — it records the undeliverable Envelope in a delivery
// dead-letter list (see Registry.DeliveryFailures) and, if configured, hands it
// to the delivery-error hook (see WithDeliveryErrorHook). So an exhausted
// delivery is observable, never silently lost. Delivery remains at-least-once.
type Notifier interface {
	Notify(ctx context.Context, env Envelope) error
}

// DeliveryFailure records a status Envelope whose delivery to the Notifier
// failed permanently — e.g. a WebhookSink that exhausted its retries. Recording
// it is the delivery-side dead-letter: it makes an undeliverable callback
// observable (via Registry.DeliveryFailures) instead of silently dropping it.
type DeliveryFailure struct {
	Envelope Envelope  // the status envelope that could not be delivered
	Err      string    // the sink's returned error, stringified
	At       time.Time // when the failure was recorded (Registry clock)
}

// Registry is the in-memory task store and state machine. It is safe for
// concurrent use by multiple goroutines.
type Registry struct {
	mu    sync.Mutex
	tasks map[string]*Task
	dead  []*Task // task dead-letter queue (work retries exhausted)

	// deliveryFailures is the delivery-side dead-letter: envelopes the Notifier
	// failed to deliver (e.g. a WebhookSink that exhausted its retries).
	deliveryFailures []DeliveryFailure

	maxAttempts     int
	backoff         func(attempt int) time.Duration
	clock           func() time.Time
	notifier        Notifier
	onDeliveryError func(ctx context.Context, env Envelope, err error)

	seq atomic.Uint64
}

// Option configures a Registry.
type Option func(*Registry)

// WithMaxAttempts sets the attempt ceiling before a retryable failure is
// dead-lettered. Must be >= 1; values < 1 are clamped to 1.
func WithMaxAttempts(n int) Option {
	return func(r *Registry) {
		if n < 1 {
			n = 1
		}
		r.maxAttempts = n
	}
}

// WithBackoff sets the back-off schedule used to compute NextRetryAt. attempt
// is 1-based (the attempt that just failed).
func WithBackoff(f func(attempt int) time.Duration) Option {
	return func(r *Registry) { r.backoff = f }
}

// WithClock overrides the time source (useful in tests).
func WithClock(f func() time.Time) Option {
	return func(r *Registry) { r.clock = f }
}

// WithNotifier attaches a sink that receives an Envelope on every real state
// transition.
//
// Delivery is SYNCHRONOUS: the sink's Notify runs inline on the transition call
// path (Submit/Start/Progress/Complete/Fail) after the Registry lock is
// released but before the transition method returns. A WebhookSink configured
// with retries + back-off therefore blocks the caller for the full retry
// schedule (worst case (MaxRetries+1) requests plus the sum of the back-offs).
// If a transition must not block on a slow or unreachable endpoint, wrap the
// sink in one that hands delivery to a background worker/queue and returns
// immediately.
func WithNotifier(n Notifier) Option {
	return func(r *Registry) { r.notifier = n }
}

// WithDeliveryErrorHook registers a callback invoked whenever a Notifier
// delivery fails permanently — e.g. a WebhookSink that exhausted its retries.
// Use it to route the undeliverable callback to an external dead-letter store,
// alerting, or a log. The Registry always also records the failure internally
// (see DeliveryFailures); this hook is an additional, optional observer. Like
// the Notifier itself, the hook runs synchronously on the transition call path,
// so it must not block for long.
func WithDeliveryErrorHook(f func(ctx context.Context, env Envelope, err error)) Option {
	return func(r *Registry) { r.onDeliveryError = f }
}

// New builds a Registry. Defaults: maxAttempts=8 (matches the event_sink DDL
// ceiling), exponential back-off 100ms * 2^(attempt-1), wall-clock time.
func New(opts ...Option) *Registry {
	r := &Registry{
		tasks:       make(map[string]*Task),
		maxAttempts: 8,
		backoff:     defaultBackoff,
		clock:       func() time.Time { return time.Now().UTC() },
	}
	for _, o := range opts {
		o(r)
	}
	return r
}

func defaultBackoff(attempt int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	return 100 * time.Millisecond * (1 << (attempt - 1))
}

func (r *Registry) now() time.Time { return r.clock() }

func (r *Registry) newID() string {
	n := r.seq.Add(1)
	var b [8]byte
	// crypto/rand.Read never returns a short read; ignore err per stdlib contract.
	_, _ = rand.Read(b[:])
	return fmt.Sprintf("task-%08d-%s", n, hex.EncodeToString(b[:]))
}

// notify delivers env to the sink outside the Registry lock. It runs
// SYNCHRONOUSLY on the transition call path (see WithNotifier), so a sink that
// retries with back-off blocks the caller until it returns. A delivery error
// (e.g. a WebhookSink that exhausted its retries) is NOT swallowed: it is
// recorded in the delivery dead-letter list (see DeliveryFailures) and passed
// to the optional delivery-error hook, so an undeliverable callback is
// observable rather than silently lost.
func (r *Registry) notify(ctx context.Context, env Envelope) {
	if r.notifier == nil {
		return
	}
	if err := r.notifier.Notify(ctx, env); err != nil {
		r.recordDeliveryFailure(ctx, env, err)
	}
}

// recordDeliveryFailure captures a permanently-undeliverable Envelope: it
// appends it to the delivery dead-letter list and invokes the optional hook.
func (r *Registry) recordDeliveryFailure(ctx context.Context, env Envelope, err error) {
	r.mu.Lock()
	r.deliveryFailures = append(r.deliveryFailures, DeliveryFailure{
		Envelope: env,
		Err:      err.Error(),
		At:       r.now(),
	})
	r.mu.Unlock()

	if r.onDeliveryError != nil {
		r.onDeliveryError(ctx, env, err)
	}
}

// Submit registers a new task in StateQueued and returns a snapshot.
func (r *Registry) Submit(ctx context.Context, taskType string, payload []byte) (Task, error) {
	now := r.now()
	t := &Task{
		ID:        r.newID(),
		Type:      taskType,
		State:     StateQueued,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if len(payload) > 0 {
		t.Payload = append(json.RawMessage(nil), payload...)
	}

	r.mu.Lock()
	r.tasks[t.ID] = t
	snap := *t
	r.mu.Unlock()

	r.notify(ctx, snap.Envelope())
	return snap, nil
}

// Start moves a task from queued (or retrying) to running.
func (r *Registry) Start(ctx context.Context, id string) (Task, error) {
	return r.transition(ctx, id, func(t *Task) error {
		if t.State != StateQueued && t.State != StateRetrying {
			return fmt.Errorf("%w: start from %s", ErrInvalidTransition, t.State)
		}
		t.State = StateRunning
		t.NextRetryAt = time.Time{}
		return nil
	})
}

// Progress records a progress update (0.0..1.0) on a running task.
func (r *Registry) Progress(ctx context.Context, id string, p float64) (Task, error) {
	return r.transition(ctx, id, func(t *Task) error {
		if p < 0 || p > 1 {
			return fmt.Errorf("%w: %v", ErrInvalidProgress, p)
		}
		if t.State != StateRunning {
			return fmt.Errorf("%w: progress from %s", ErrInvalidTransition, t.State)
		}
		t.Progress = p
		return nil
	})
}

// Complete moves a running task to succeeded. It is idempotent: completing an
// already-succeeded task is a no-op that returns the current snapshot and does
// NOT re-fire the notifier (so at-least-once delivery never double-completes).
func (r *Registry) Complete(ctx context.Context, id, resultRef string) (Task, error) {
	r.mu.Lock()
	t, ok := r.tasks[id]
	if !ok {
		r.mu.Unlock()
		return Task{}, ErrNotFound
	}
	if t.State == StateSucceeded {
		// Idempotent no-op — already done.
		snap := *t
		r.mu.Unlock()
		return snap, nil
	}
	if t.State != StateRunning {
		snap := *t
		r.mu.Unlock()
		return snap, fmt.Errorf("%w: complete from %s", ErrInvalidTransition, t.State)
	}
	t.State = StateSucceeded
	t.Progress = 1.0
	t.ResultRef = resultRef
	t.Error = ""
	t.UpdatedAt = r.now()
	snap := *t
	r.mu.Unlock()

	r.notify(ctx, snap.Envelope())
	return snap, nil
}

// Fail records a failed attempt on a running task. Semantics:
//   - retryable, attempts remaining      -> retrying (NextRetryAt = now + back-off)
//   - retryable, attempts exhausted       -> dead + dead-letter queue
//   - non-retryable                        -> failed (terminal)
func (r *Registry) Fail(ctx context.Context, id string, jobErr JobError) (Task, error) {
	r.mu.Lock()
	t, ok := r.tasks[id]
	if !ok {
		r.mu.Unlock()
		return Task{}, ErrNotFound
	}
	if t.State != StateRunning {
		snap := *t
		r.mu.Unlock()
		return snap, fmt.Errorf("%w: fail from %s", ErrInvalidTransition, t.State)
	}

	t.Attempts++
	t.UpdatedAt = r.now()
	if jobErr.Message != "" {
		t.Error = jobErr.Message
	} else {
		t.Error = jobErr.Code
	}

	switch {
	case jobErr.Retryable && t.Attempts < r.maxAttempts:
		t.State = StateRetrying
		t.NextRetryAt = r.now().Add(r.backoff(t.Attempts))
	case jobErr.Retryable:
		// Retries exhausted -> dead-letter.
		t.State = StateDead
		t.NextRetryAt = time.Time{}
		r.dead = append(r.dead, t)
	default:
		// Non-retryable terminal failure.
		t.State = StateFailed
		t.NextRetryAt = time.Time{}
	}

	snap := *t
	r.mu.Unlock()

	r.notify(ctx, snap.Envelope())
	return snap, nil
}

// transition is the shared lock/mutate/snapshot/notify path for the simple
// transitions (Start, Progress).
func (r *Registry) transition(ctx context.Context, id string, mutate func(*Task) error) (Task, error) {
	r.mu.Lock()
	t, ok := r.tasks[id]
	if !ok {
		r.mu.Unlock()
		return Task{}, ErrNotFound
	}
	if err := mutate(t); err != nil {
		snap := *t
		r.mu.Unlock()
		return snap, err
	}
	t.UpdatedAt = r.now()
	snap := *t
	r.mu.Unlock()

	r.notify(ctx, snap.Envelope())
	return snap, nil
}

// Status returns the current status Envelope for a task.
func (r *Registry) Status(id string) (Envelope, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	t, ok := r.tasks[id]
	if !ok {
		return Envelope{}, ErrNotFound
	}
	return t.Envelope(), nil
}

// Get returns a snapshot copy of a task.
func (r *Registry) Get(id string) (Task, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	t, ok := r.tasks[id]
	if !ok {
		return Task{}, ErrNotFound
	}
	return *t, nil
}

// DeadLetters returns snapshot copies of all dead-lettered tasks, in the order
// they were dead-lettered.
func (r *Registry) DeadLetters() []Task {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]Task, len(r.dead))
	for i, t := range r.dead {
		out[i] = *t
	}
	return out
}

// DeliveryFailures returns snapshot copies of every callback the Notifier
// failed to deliver (e.g. a WebhookSink that exhausted its retries), in the
// order they were recorded. This is the delivery-side dead-letter view, and is
// distinct from DeadLetters(): DeadLetters tracks tasks whose *work* exhausted
// the retry ceiling, whereas DeliveryFailures tracks status callbacks whose
// *delivery* to the sink failed.
func (r *Registry) DeliveryFailures() []DeliveryFailure {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]DeliveryFailure, len(r.deliveryFailures))
	copy(out, r.deliveryFailures)
	return out
}

// Len reports the number of tasks currently tracked.
func (r *Registry) Len() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.tasks)
}
