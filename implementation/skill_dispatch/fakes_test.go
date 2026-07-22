package skilldispatch

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
)

// recorder captures the order in which fake Skills execute, safely under
// concurrency. It is the ground truth for the execution-order and call-count
// assertions.
type recorder struct {
	mu    sync.Mutex
	order []string
}

func (r *recorder) record(name string) {
	r.mu.Lock()
	r.order = append(r.order, name)
	r.mu.Unlock()
}

func (r *recorder) snapshot() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]string, len(r.order))
	copy(out, r.order)
	return out
}

// fakeSkill is a fully controllable Skill for the tests. It records every Run in
// a shared recorder, counts its calls atomically (so concurrent Process calls are
// observable), and can inject failures: failFirst transient failures then success,
// always-transient failure, or a single Permanent (non-retryable) failure.
type fakeSkill struct {
	name string
	kind Kind
	tags []string // Match succeeds if the post carries any of these hashtags

	rec   *recorder
	calls atomic.Int64

	failFirst int  // the first N Run calls return a transient error
	always    bool // every Run returns a transient error
	permanent bool // the first Run returns a Permanent (non-retryable) error
}

func (f *fakeSkill) Name() string { return f.name }
func (f *fakeSkill) Kind() Kind   { return f.kind }

func (f *fakeSkill) Match(p Post) bool { return p.HasAnyHashtag(f.tags...) }

func (f *fakeSkill) Calls() int { return int(f.calls.Load()) }

func (f *fakeSkill) Run(ctx context.Context, p Post) (Result, error) {
	n := int(f.calls.Add(1))
	if f.rec != nil {
		f.rec.record(f.name)
	}
	switch {
	case f.always:
		return Result{}, fmt.Errorf("%s: always-fail attempt %d", f.name, n)
	case f.permanent && n == 1:
		return Result{}, Permanent(fmt.Errorf("%s: permanent failure", f.name))
	case n <= f.failFirst:
		return Result{}, fmt.Errorf("%s: transient fail %d", f.name, n)
	default:
		return Result{SkillName: f.name, Output: "ok:" + p.ID}, nil
	}
}

// spySink records every emitted event in order for assertion.
type spySink struct {
	mu     sync.Mutex
	events []StepEvent
}

func (s *spySink) Emit(e StepEvent) {
	s.mu.Lock()
	s.events = append(s.events, e)
	s.mu.Unlock()
}

func (s *spySink) snapshot() []StepEvent {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]StepEvent, len(s.events))
	copy(out, s.events)
	return out
}

// count returns how many events of the given type were emitted.
func (s *spySink) count(t EventType) int {
	n := 0
	for _, e := range s.snapshot() {
		if e.Type == t {
			n++
		}
	}
	return n
}

// pair is a compact (event type, skill name) projection for order assertions.
type pair struct {
	typ  EventType
	name string
}

func (s *spySink) pairs() []pair {
	evs := s.snapshot()
	out := make([]pair, len(evs))
	for i, e := range evs {
		out[i] = pair{typ: e.Type, name: e.SkillName}
	}
	return out
}
