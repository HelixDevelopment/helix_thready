package processing

import (
	"context"
	"time"
)

// State is the overall outcome of Process for a post.
type State int

const (
	// StateRejected means the claim was rejected (a duplicate trigger); no work ran.
	StateRejected State = iota
	// StateCompleted means every resolved step succeeded.
	StateCompleted
	// StateFailed means a step was abandoned (dead) after exhausting retries.
	StateFailed
	// StateCanceled means the context was canceled mid-processing.
	StateCanceled
)

// String returns a readable name for the state.
func (s State) String() string {
	switch s {
	case StateRejected:
		return "rejected"
	case StateCompleted:
		return "completed"
	case StateFailed:
		return "failed"
	case StateCanceled:
		return "canceled"
	default:
		return "unknown"
	}
}

// Result is the outcome of Process for a single post.
type Result struct {
	// PostID echoes the processed post.
	PostID string
	// State is the overall outcome.
	State State
	// ProcessedOnce is true when THIS call won the claim and ran the steps; false
	// for a rejected duplicate. It is the exactly-once witness.
	ProcessedOnce bool
	// Steps are the per-step outcomes, in execution order.
	Steps []StepResult
	// Assets is the union of every succeeded step's artifacts, in order — the
	// aggregate produced by the post.
	Assets []string
}

// Processor is the reusable Processing-Engine core. It claims a post exactly once,
// resolves and orders the matching Skills by stage precedence, runs each with
// retry/backoff, emits lifecycle events, fires the completion callback, and reports
// the overall state. Every collaborator is an injected seam, so the Processor
// imports no sibling modules — the real modules plug in behind Claimer/SkillSet/
// Skill/EventEmitter/Callbacker. A single Processor is safe for concurrent Process
// calls: the shared Claimer serializes claims.
type Processor struct {
	claims   Claimer
	skills   SkillSet
	emitter  EventEmitter
	callback Callbacker
	retry    RetryPolicy
	failFast bool
	clock    func() time.Time
}

// Option configures a Processor.
type Option func(*Processor)

// WithEmitter injects the events seam. Nil is ignored.
func WithEmitter(e EventEmitter) Option {
	return func(p *Processor) {
		if e != nil {
			p.emitter = e
		}
	}
}

// WithCallbacker injects the completion-callback seam. Nil is ignored.
func WithCallbacker(c Callbacker) Option {
	return func(p *Processor) {
		if c != nil {
			p.callback = c
		}
	}
}

// WithRetry sets the per-step retry policy.
func WithRetry(rp RetryPolicy) Option {
	return func(p *Processor) { p.retry = rp }
}

// WithFailFast controls whether a dead step stops the remaining steps. When true
// (the default), because later stages consume earlier outputs, a dead step is
// fail-fast: remaining steps are skipped and the post is marked failed. When false,
// remaining steps still run (best-effort) and the post is marked failed if any step
// died.
func WithFailFast(on bool) Option {
	return func(p *Processor) { p.failFast = on }
}

// WithClock overrides the time source (useful in tests).
func WithClock(f func() time.Time) Option {
	return func(p *Processor) {
		if f != nil {
			p.clock = f
		}
	}
}

// NewProcessor builds a Processor over the given exactly-once Claimer and SkillSet
// resolver. The events and callback seams default to no-ops until injected; retry
// defaults to DefaultRetryPolicy and fail-fast is on. Nil claims/skills are
// defaulted to a fresh MemoryClaimer and an empty resolver so the zero-configured
// Processor is still safe to call.
func NewProcessor(claims Claimer, skills SkillSet, opts ...Option) *Processor {
	if claims == nil {
		claims = NewMemoryClaimer()
	}
	if skills == nil {
		skills = SkillSetFunc(func(Post) []Skill { return nil })
	}
	p := &Processor{
		claims:   claims,
		skills:   skills,
		emitter:  nopEmitter{},
		callback: nopCallbacker{},
		retry:    DefaultRetryPolicy(),
		failFast: true,
		clock:    func() time.Time { return time.Now().UTC() },
	}
	for _, o := range opts {
		o(p)
	}
	return p
}

// Process runs the full pipeline for one post:
//
//  1. Claim the post. If the claim is rejected (a duplicate trigger), emit
//     post.rejected and return immediately with State=StateRejected and
//     ProcessedOnce=false — a no-op. This is the exactly-once guarantee: the Skills
//     run only for the caller that wins the claim.
//  2. Resolve the matching Skills and order them by stage precedence.
//  3. Run each step with retry/backoff, emitting step events. On a dead step the
//     post is marked failed; with fail-fast (default) the remaining steps are
//     skipped.
//  4. Mark the post Done or Failed, emit the terminal post event, and fire the
//     completion callback carrying the final state.
//
// Process returns a nil error for a rejected duplicate, a completed post, or a
// failed post (a dead step is reported via Result.State, not the error). It returns
// the context error if the context is canceled mid-run, and a wrapped delivery
// error if the completion callback itself fails (Result.State still reports the
// true processing outcome).
func (p *Processor) Process(ctx context.Context, post Post) (Result, error) {
	res := Result{PostID: post.ID}

	// 1. Exactly-once claim.
	if !p.claims.Claim(post.ID) {
		p.emit(Event{Type: EventPostRejected, PostID: post.ID, Time: p.now()})
		res.State = StateRejected
		res.ProcessedOnce = false
		return res, nil
	}
	res.ProcessedOnce = true
	p.emit(Event{Type: EventPostClaimed, PostID: post.ID, Time: p.now()})

	// 2. Resolve + order by precedence (download → convert → analyze → research → reply).
	steps := OrderByPrecedence(p.skills.Resolve(post))

	// 3. Run each step with retry/backoff.
	failed := false
	var ctxErr error
	for _, s := range steps {
		sr := p.runStep(ctx, post, s)
		res.Steps = append(res.Steps, sr)
		if sr.Succeeded {
			res.Assets = append(res.Assets, sr.Artifacts...)
		}
		if sr.Dead {
			failed = true
			if isCtxErr(sr.Err) {
				// Cancellation is global, not step-local: always stop.
				ctxErr = ctx.Err()
				break
			}
			if p.failFast {
				break
			}
		}
	}

	// 4. Terminal state, event, and completion callback.
	switch {
	case ctxErr != nil:
		res.State = StateCanceled
		p.claims.MarkFailed(post.ID)
		p.emit(Event{Type: EventPostFailed, PostID: post.ID, Time: p.now()})
		// The context is dead; a completion callback would fail its own delivery,
		// so it is not fired here. The canceled state is surfaced via the error.
		return res, ctxErr
	case failed:
		res.State = StateFailed
		p.claims.MarkFailed(post.ID)
		p.emit(Event{Type: EventPostFailed, PostID: post.ID, Time: p.now()})
		return res, p.fireCallback(ctx, res, CompletionFailed)
	default:
		res.State = StateCompleted
		p.claims.MarkDone(post.ID)
		p.emit(Event{Type: EventPostCompleted, PostID: post.ID, Time: p.now()})
		return res, p.fireCallback(ctx, res, CompletionSucceeded)
	}
}

// runStep runs one Skill with retry/backoff, emitting a started event, one failed
// event per failed attempt, and a terminal succeeded or dead event.
func (p *Processor) runStep(ctx context.Context, post Post, s Skill) StepResult {
	sr := StepResult{SkillName: s.Name(), Kind: s.Kind()}
	p.emit(Event{
		Type: EventStepStarted, PostID: post.ID, SkillName: s.Name(), Kind: s.Kind(),
		Attempt: 0, Time: p.now(),
	})

	max := p.retry.attempts()
	var lastErr error
	for attempt := 1; attempt <= max; attempt++ {
		// Honor cancellation before spending an attempt.
		if err := ctx.Err(); err != nil {
			lastErr = err
			p.emit(Event{
				Type: EventStepFailed, PostID: post.ID, SkillName: s.Name(), Kind: s.Kind(),
				Attempt: attempt, Err: err.Error(), Time: p.now(),
			})
			break
		}

		out, err := s.Run(ctx, post)
		sr.Attempts = attempt
		if err == nil {
			sr.Succeeded = true
			sr.Output = out.Output
			sr.Artifacts = out.Artifacts
			p.emit(Event{
				Type: EventStepSucceeded, PostID: post.ID, SkillName: s.Name(), Kind: s.Kind(),
				Attempt: attempt, Time: p.now(),
			})
			return sr
		}

		lastErr = err
		p.emit(Event{
			Type: EventStepFailed, PostID: post.ID, SkillName: s.Name(), Kind: s.Kind(),
			Attempt: attempt, Err: err.Error(), Time: p.now(),
		})

		if !IsRetryable(err) {
			break
		}
		if attempt < max {
			if !p.backoff(ctx, attempt+1) {
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
	p.emit(Event{
		Type: EventStepDead, PostID: post.ID, SkillName: s.Name(), Kind: s.Kind(),
		Attempt: sr.Attempts, Err: sr.Err, Time: p.now(),
	})
	return sr
}

// fireCallback builds the completion envelope for a terminal post and delivers it
// through the Callbacker. A delivery error is wrapped and returned so the caller
// can retry delivery; the post's true State is unaffected.
func (p *Processor) fireCallback(ctx context.Context, res Result, state CompletionState) error {
	c := Completion{
		TaskID:    res.PostID,
		State:     state,
		Progress:  progressOf(res.Steps),
		ResultRef: firstAsset(res.Assets),
		TS:        p.now(),
	}
	if state == CompletionFailed {
		c.Error = firstDeadErr(res.Steps)
	}
	if err := p.callback.Notify(ctx, c); err != nil {
		return &callbackError{err: err}
	}
	return nil
}

// backoff waits for the delay before the given 1-based attempt, returning false if
// the context is canceled first.
func (p *Processor) backoff(ctx context.Context, nextAttempt int) bool {
	delay := p.retry.Delay(nextAttempt)
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

func (p *Processor) emit(e Event) {
	if e.Time.IsZero() {
		e.Time = p.now()
	}
	p.emitter.Emit(e)
}

func (p *Processor) now() time.Time { return p.clock() }

// callbackError wraps a completion-callback delivery failure so callers can tell it
// apart from a processing error via errors.As.
type callbackError struct{ err error }

func (e *callbackError) Error() string { return "completion callback: " + e.err.Error() }
func (e *callbackError) Unwrap() error { return e.err }

// isCtxErr reports whether an error string is one of the context errors.
func isCtxErr(s string) bool {
	return s == context.Canceled.Error() || s == context.DeadlineExceeded.Error()
}

// progressOf returns the fraction of steps that succeeded (1.0 when there are no
// steps — a trivially complete post).
func progressOf(steps []StepResult) float64 {
	if len(steps) == 0 {
		return 1.0
	}
	ok := 0
	for _, s := range steps {
		if s.Succeeded {
			ok++
		}
	}
	return float64(ok) / float64(len(steps))
}

// firstAsset returns the first artifact reference, or "".
func firstAsset(assets []string) string {
	if len(assets) == 0 {
		return ""
	}
	return assets[0]
}

// firstDeadErr returns the error string of the first dead step, or "".
func firstDeadErr(steps []StepResult) string {
	for _, s := range steps {
		if s.Dead {
			return s.Err
		}
	}
	return ""
}
