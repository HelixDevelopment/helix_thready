// Package callbacktask is the Helix Thready standardized Callback/Task module.
//
// It defines one canonical async-job contract (accept task -> run async ->
// status -> outbound HMAC-signed webhook on completion -> error -> retry with
// back-off -> dead-letter) applied uniformly across the 3rd-party integrations
// (Boba, MeTube, Download Manager) so the Processing Engine consumes a single
// completion shape regardless of the underlying provider.
//
// It realizes the contract described in
// docs/public/research/mvp/development/build-new-subsystems.md §2
// ("Standardized callback/task module", item ATM-030, gaps §6.4/§6.5) and the
// outbound-webhook / HMAC scheme in
// docs/public/research/mvp/api/event-bus-contract.md §9.
//
// The module is self-contained and depends only on the Go standard library.
package callbacktask

import (
	"encoding/json"
	"time"
)

// State is the lifecycle state of a Task.
type State string

const (
	// StateQueued: submitted, not yet picked up by a worker.
	StateQueued State = "queued"
	// StateRunning: a worker is actively processing the task.
	StateRunning State = "running"
	// StateSucceeded: terminal success; ResultRef is set.
	StateSucceeded State = "succeeded"
	// StateFailed: terminal failure that will not be retried (non-retryable error).
	StateFailed State = "failed"
	// StateRetrying: a retryable failure with attempts remaining; scheduled for
	// re-run after NextRetryAt (exponential back-off).
	StateRetrying State = "retrying"
	// StateDead: retries exhausted (attempts reached the ceiling) -> dead-letter.
	StateDead State = "dead"
)

// Terminal reports whether s is a terminal state (no further transitions).
func (s State) Terminal() bool {
	switch s {
	case StateSucceeded, StateFailed, StateDead:
		return true
	default:
		return false
	}
}

// Task is a single unit of asynchronous work.
type Task struct {
	ID          string          `json:"id"`
	Type        string          `json:"type"`
	Payload     json.RawMessage `json:"payload,omitempty"`
	State       State           `json:"state"`
	Attempts    int             `json:"attempts"`
	Progress    float64         `json:"progress"` // 0.0..1.0
	ResultRef   string          `json:"result_ref,omitempty"`
	Error       string          `json:"error,omitempty"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
	NextRetryAt time.Time       `json:"next_retry_at,omitempty"`
}

// JobError is the error carried by Registry.Fail. Retryable is the decision
// point: a retryable error re-enters the retry loop (subject to the attempt
// ceiling), a non-retryable one terminates the task in StateFailed.
type JobError struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	Retryable bool   `json:"retryable"`
}

// Envelope is the stable outbound status envelope, matching the canonical
// schema {task_id, state, progress, result_ref, error, ts}. Field presence is
// stable (no omitempty) so the JSON shape — and therefore the HMAC signature
// computed over it — is deterministic.
type Envelope struct {
	TaskID    string    `json:"task_id"`
	State     State     `json:"state"`
	Progress  float64   `json:"progress"`
	ResultRef string    `json:"result_ref"`
	Error     string    `json:"error"`
	TS        time.Time `json:"ts"`
}

// Envelope renders the current status of the task as an Envelope.
func (t *Task) Envelope() Envelope {
	return Envelope{
		TaskID:    t.ID,
		State:     t.State,
		Progress:  t.Progress,
		ResultRef: t.ResultRef,
		Error:     t.Error,
		TS:        t.UpdatedAt,
	}
}
