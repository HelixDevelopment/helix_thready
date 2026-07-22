package skilldispatch

import (
	"context"
	"time"
)

// PostState is the overall outcome of Process for a post.
type PostState int

const (
	// PostRejected means the claim was rejected (a duplicate trigger); no work ran.
	PostRejected PostState = iota
	// PostCompleted means every resolved step succeeded.
	PostCompleted
	// PostFailed means a step was abandoned (dead) after exhausting retries.
	PostFailed
	// PostCanceled means the context was canceled mid-processing.
	PostCanceled
)

// String returns a readable name for the post state.
func (s PostState) String() string {
	switch s {
	case PostRejected:
		return "rejected"
	case PostCompleted:
		return "completed"
	case PostFailed:
		return "failed"
	case PostCanceled:
		return "canceled"
	default:
		return "unknown"
	}
}

// StepResult records the outcome of running one Skill step.
type StepResult struct {
	SkillName string
	Kind      Kind
	Attempts  int    // number of Run attempts made (1..MaxAttempts)
	Succeeded bool   // true if the step ultimately succeeded
	Dead      bool   // true if the step was abandoned after retries/permanent error
	Result    Result // the Skill's result on success
	Err       string // the final error on failure
}

// PostResult is the outcome of Process for a single post.
type PostResult struct {
	PostID  string
	Claimed bool         // false when the claim was rejected (duplicate)
	State   PostState    // overall outcome
	Steps   []StepResult // per-step outcomes, in execution order
}

// Dispatcher is the execution engine. It claims a post, resolves and orders the
// matching Skills, runs each with retry/backoff, emits step events, and tracks
// the overall post state. It composes over the Skill-Graph (knowledge/ordering)
// and adds only execution. A single Dispatcher is safe for concurrent Process
// calls: the shared ClaimRegistry serializes claims.
type Dispatcher struct {
	registry *Registry
	claims   *ClaimRegistry
	sink     EventSink
	retry    RetryPolicy
	clock    func() time.Time
}

// Option configures a Dispatcher.
type Option func(*Dispatcher)

// WithEventSink injects the sink that receives every step event.
func WithEventSink(s EventSink) Option {
	return func(d *Dispatcher) {
		if s != nil {
			d.sink = s
		}
	}
}

// WithRetry sets the retry policy.
func WithRetry(p RetryPolicy) Option {
	return func(d *Dispatcher) { d.retry = p }
}

// WithClaimRegistry injects a shared ClaimRegistry (e.g. one shared across
// Dispatchers). By default each Dispatcher owns a fresh one.
func WithClaimRegistry(c *ClaimRegistry) Option {
	return func(d *Dispatcher) {
		if c != nil {
			d.claims = c
		}
	}
}

// WithClock overrides the time source (useful in tests).
func WithClock(f func() time.Time) Option {
	return func(d *Dispatcher) {
		if f != nil {
			d.clock = f
		}
	}
}

// NewDispatcher builds a Dispatcher over the given Registry.
func NewDispatcher(reg *Registry, opts ...Option) *Dispatcher {
	d := &Dispatcher{
		registry: reg,
		claims:   NewClaimRegistry(),
		sink:     nopSink{},
		retry:    DefaultRetryPolicy(),
		clock:    func() time.Time { return time.Now().UTC() },
	}
	for _, o := range opts {
		o(d)
	}
	return d
}

// Claims exposes the ClaimRegistry so callers can inspect post state or perform
// an explicit reprocess (Release).
func (d *Dispatcher) Claims() *ClaimRegistry { return d.claims }

// Process runs the full pipeline for one post:
//
//  1. Claim the post. If the claim is rejected (duplicate trigger), emit
//     post.rejected and return immediately with State=PostRejected — a no-op.
//     This is the exactly-once guarantee: the Skills run only for the caller
//     that wins the claim.
//  2. Resolve the matching Skills and order them by stage precedence.
//  3. Run each step with retry/backoff, emitting step events. A dead step is
//     fail-fast: because later stages consume earlier outputs, remaining steps
//     are skipped and the post is marked failed.
//  4. Mark the post Done or Failed and emit the terminal post event.
//
// Process never returns an error for a rejected claim or a dead step — those are
// reported through PostResult.State. It returns the context error only if the
// context is canceled mid-run.
func (d *Dispatcher) Process(ctx context.Context, post Post) (PostResult, error) {
	res := PostResult{PostID: post.ID}

	if !d.claims.Claim(post.ID) {
		d.emit(StepEvent{Type: EventPostRejected, PostID: post.ID, Time: d.now()})
		res.State = PostRejected
		res.Claimed = false
		return res, nil
	}
	res.Claimed = true
	d.emit(StepEvent{Type: EventPostClaimed, PostID: post.ID, Time: d.now()})

	steps := OrderByPrecedence(d.registry.Resolve(post))

	failed := false
	var ctxErr error
	for _, s := range steps {
		sr := d.runStep(ctx, post, s)
		res.Steps = append(res.Steps, sr)
		if sr.Dead {
			failed = true
			if sr.Err == context.Canceled.Error() || sr.Err == context.DeadlineExceeded.Error() {
				ctxErr = ctx.Err()
			}
			// Fail-fast: later stages consume earlier outputs, so stop here.
			break
		}
	}

	switch {
	case ctxErr != nil:
		res.State = PostCanceled
		d.claims.MarkFailed(post.ID)
		d.emit(StepEvent{Type: EventPostFailed, PostID: post.ID, Time: d.now()})
		return res, ctxErr
	case failed:
		res.State = PostFailed
		d.claims.MarkFailed(post.ID)
		d.emit(StepEvent{Type: EventPostFailed, PostID: post.ID, Time: d.now()})
	default:
		res.State = PostCompleted
		d.claims.MarkDone(post.ID)
		d.emit(StepEvent{Type: EventPostCompleted, PostID: post.ID, Time: d.now()})
	}
	return res, nil
}

// runStep runs one Skill with retry/backoff, emitting a started event, one failed
// event per failed attempt, and a terminal succeeded or dead event.
func (d *Dispatcher) runStep(ctx context.Context, post Post, s Skill) StepResult {
	sr := StepResult{SkillName: s.Name(), Kind: s.Kind()}
	d.emit(StepEvent{
		Type: EventStepStarted, PostID: post.ID, SkillName: s.Name(), Kind: s.Kind(),
		Attempt: 0, Time: d.now(),
	})

	max := d.retry.attempts()
	var lastErr error
	for attempt := 1; attempt <= max; attempt++ {
		// Honor cancellation before spending an attempt.
		if err := ctx.Err(); err != nil {
			lastErr = err
			sr.Attempts = attempt - 1
			d.emit(StepEvent{
				Type: EventStepFailed, PostID: post.ID, SkillName: s.Name(), Kind: s.Kind(),
				Attempt: attempt, Err: err.Error(), Time: d.now(),
			})
			break
		}

		result, err := s.Run(ctx, post)
		sr.Attempts = attempt
		if err == nil {
			sr.Succeeded = true
			sr.Result = result
			d.emit(StepEvent{
				Type: EventStepSucceeded, PostID: post.ID, SkillName: s.Name(), Kind: s.Kind(),
				Attempt: attempt, Time: d.now(),
			})
			return sr
		}

		lastErr = err
		d.emit(StepEvent{
			Type: EventStepFailed, PostID: post.ID, SkillName: s.Name(), Kind: s.Kind(),
			Attempt: attempt, Err: err.Error(), Time: d.now(),
		})

		if !IsRetryable(err) {
			break
		}
		if attempt < max {
			if !d.backoff(ctx, attempt+1) {
				// Context canceled during backoff.
				lastErr = ctx.Err()
				break
			}
		}
	}

	sr.Dead = true
	if lastErr != nil {
		sr.Err = lastErr.Error()
	}
	d.emit(StepEvent{
		Type: EventStepDead, PostID: post.ID, SkillName: s.Name(), Kind: s.Kind(),
		Attempt: sr.Attempts, Err: sr.Err, Time: d.now(),
	})
	return sr
}

// backoff waits for the delay before the given 1-based attempt, returning false
// if the context is canceled first.
func (d *Dispatcher) backoff(ctx context.Context, nextAttempt int) bool {
	delay := d.retry.Delay(nextAttempt)
	if delay <= 0 {
		return ctx.Err() == nil
	}
	t := time.NewTimer(delay)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}

func (d *Dispatcher) emit(e StepEvent) {
	if e.Time.IsZero() {
		e.Time = d.now()
	}
	d.sink.Emit(e)
}

func (d *Dispatcher) now() time.Time { return d.clock() }
