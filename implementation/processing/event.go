package processing

import "time"

// EventType classifies a lifecycle Event emitted during Process. The set and their
// String() names mirror skill_dispatch/event_bus semantics so an EventEmitter
// adapter maps them straight onto the bus subjects ("post.claimed", "step.started",
// …).
type EventType int

const (
	// EventPostClaimed is emitted once when a post is successfully claimed.
	EventPostClaimed EventType = iota
	// EventPostRejected is emitted when Process is called for an already-claimed
	// post (a duplicate trigger) and does no work.
	EventPostRejected
	// EventStepStarted is emitted once when a Skill step begins (before attempt 1).
	EventStepStarted
	// EventStepSucceeded is emitted when a Skill step succeeds.
	EventStepSucceeded
	// EventStepFailed is emitted once per failed attempt (transient or the final
	// attempt of a Permanent/exhausted failure).
	EventStepFailed
	// EventStepDead is emitted when a Skill step is abandoned after retries are
	// exhausted or a Permanent error was returned.
	EventStepDead
	// EventPostCompleted is emitted once when every step of a post succeeded.
	EventPostCompleted
	// EventPostFailed is emitted once when a post finished with a dead step (or was
	// canceled mid-run).
	EventPostFailed
)

// String returns a readable, dot-notation name for the event type.
func (t EventType) String() string {
	switch t {
	case EventPostClaimed:
		return "post.claimed"
	case EventPostRejected:
		return "post.rejected"
	case EventStepStarted:
		return "step.started"
	case EventStepSucceeded:
		return "step.succeeded"
	case EventStepFailed:
		return "step.failed"
	case EventStepDead:
		return "step.dead"
	case EventPostCompleted:
		return "post.completed"
	case EventPostFailed:
		return "post.failed"
	default:
		return "unknown"
	}
}

// Event is one observable moment in Process. Post-level events carry only a PostID;
// step-level events additionally carry the Skill's name and kind, the 1-based
// attempt number, and (for failures) the error string.
type Event struct {
	Type      EventType
	PostID    string
	SkillName string
	Kind      Kind
	Attempt   int
	Err       string
	Time      time.Time
}

// EventEmitter receives every Event the Processor emits, in order — the events
// seam. Implement it to bridge to the event bus, metrics, or a log. Emit must not
// panic; it is called synchronously on the Process call path.
type EventEmitter interface {
	Emit(event Event)
}

// EmitterFunc adapts a plain function to the EventEmitter seam.
type EmitterFunc func(event Event)

// Emit calls the underlying function.
func (f EmitterFunc) Emit(event Event) { f(event) }

// nopEmitter is the default emitter used when none is injected; it discards events.
type nopEmitter struct{}

func (nopEmitter) Emit(Event) {}
