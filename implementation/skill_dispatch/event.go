package skilldispatch

import "time"

// EventType classifies a StepEvent emitted during Process.
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
	// EventPostFailed is emitted once when a post finished with a dead step.
	EventPostFailed
)

// String returns a readable name for the event type.
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

// StepEvent is one observable moment in Process. Post-level events carry only a
// PostID; step-level events additionally carry the Skill's name and kind, the
// 1-based attempt number, and (for failures) the error string.
type StepEvent struct {
	Type      EventType
	PostID    string
	SkillName string
	Kind      Kind
	Attempt   int
	Err       string
	Time      time.Time
}

// EventSink receives every StepEvent the Dispatcher emits, in order. Implement it
// to bridge to an event bus, metrics, or a log. Emit must not panic; it is called
// synchronously on the Process call path.
type EventSink interface {
	Emit(event StepEvent)
}

// nopSink is the default sink used when none is injected; it discards events.
type nopSink struct{}

func (nopSink) Emit(StepEvent) {}
