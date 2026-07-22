package processing

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
)

// recorder is a shared, mutex-guarded log of ordered tokens (skill runs, event
// types, callback fires). It lets tests assert the exact interleaving/order of the
// seams under -race.
type recorder struct {
	mu  sync.Mutex
	seq []string
}

func (r *recorder) add(tok string) {
	r.mu.Lock()
	r.seq = append(r.seq, tok)
	r.mu.Unlock()
}

func (r *recorder) snapshot() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]string, len(r.seq))
	copy(out, r.seq)
	return out
}

// fakeSkill is a recording Skill with an injectable per-attempt behavior. It counts
// runs atomically (safe under concurrent Process) and records each run in the shared
// recorder so tests can assert precedence order.
type fakeSkill struct {
	name string
	kind Kind
	rec  *recorder
	runs atomic.Int64
	// behavior returns the result/error for the given 1-based attempt number.
	behavior func(attempt int) (StepResult, error)
}

func (s *fakeSkill) Name() string { return s.name }
func (s *fakeSkill) Kind() Kind   { return s.kind }

func (s *fakeSkill) Run(_ context.Context, _ Post) (StepResult, error) {
	n := int(s.runs.Add(1))
	if s.rec != nil {
		s.rec.add("run:" + s.name)
	}
	return s.behavior(n)
}

// succeeds builds a skill that succeeds on the first attempt, producing the given
// artifacts.
func succeeds(name string, kind Kind, rec *recorder, artifacts ...string) *fakeSkill {
	return &fakeSkill{
		name: name, kind: kind, rec: rec,
		behavior: func(int) (StepResult, error) {
			return StepResult{Output: "ok:" + name, Artifacts: artifacts}, nil
		},
	}
}

// failsThenSucceeds builds a skill that returns a transient error for the first
// failFor attempts, then succeeds.
func failsThenSucceeds(name string, kind Kind, rec *recorder, failFor int, artifacts ...string) *fakeSkill {
	return &fakeSkill{
		name: name, kind: kind, rec: rec,
		behavior: func(attempt int) (StepResult, error) {
			if attempt <= failFor {
				return StepResult{}, errors.New("transient boom")
			}
			return StepResult{Output: "ok:" + name, Artifacts: artifacts}, nil
		},
	}
}

// alwaysFails builds a skill whose every attempt returns a transient error.
func alwaysFails(name string, kind Kind, rec *recorder) *fakeSkill {
	return &fakeSkill{
		name: name, kind: kind, rec: rec,
		behavior: func(int) (StepResult, error) {
			return StepResult{}, errors.New("permanent-ish transient boom")
		},
	}
}

// alwaysPermanent builds a skill whose first attempt returns a Permanent error.
func alwaysPermanent(name string, kind Kind, rec *recorder) *fakeSkill {
	return &fakeSkill{
		name: name, kind: kind, rec: rec,
		behavior: func(int) (StepResult, error) {
			return StepResult{}, Permanent(errors.New("bad input"))
		},
	}
}

// cancelsThenTransient builds a skill that cancels the supplied context on its first
// run and returns a transient error, so the following backoff observes a canceled
// context — a deterministic exercise of the cancel-during-backoff branch.
func cancelsThenTransient(name string, kind Kind, rec *recorder, cancel context.CancelFunc) *fakeSkill {
	return &fakeSkill{
		name: name, kind: kind, rec: rec,
		behavior: func(int) (StepResult, error) {
			cancel()
			return StepResult{}, errors.New("transient during cancel")
		},
	}
}

// recordingEmitter captures every Event in order, guarded for concurrent use.
type recordingEmitter struct {
	mu     sync.Mutex
	events []Event
}

func (e *recordingEmitter) Emit(ev Event) {
	e.mu.Lock()
	e.events = append(e.events, ev)
	e.mu.Unlock()
}

// types returns the ordered dot-notation event type names.
func (e *recordingEmitter) types() []string {
	e.mu.Lock()
	defer e.mu.Unlock()
	out := make([]string, len(e.events))
	for i, ev := range e.events {
		out[i] = ev.Type.String()
	}
	return out
}

// recordingCallbacker captures the completion callbacks it receives.
type recordingCallbacker struct {
	mu    sync.Mutex
	calls []Completion
	// err, when set, is returned by every Notify (injectable delivery failure).
	err error
}

func (c *recordingCallbacker) Notify(_ context.Context, comp Completion) error {
	c.mu.Lock()
	c.calls = append(c.calls, comp)
	c.mu.Unlock()
	return c.err
}

func (c *recordingCallbacker) last() (Completion, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.calls) == 0 {
		return Completion{}, false
	}
	return c.calls[len(c.calls)-1], true
}

func (c *recordingCallbacker) count() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.calls)
}
