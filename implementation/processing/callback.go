package processing

import (
	"context"
	"time"
)

// CompletionState is the terminal state carried by the completion callback. The
// string values mirror callback_task.State ("succeeded", "failed") so a
// callback_task adapter maps them without translation.
type CompletionState string

const (
	// CompletionSucceeded: every resolved step succeeded.
	CompletionSucceeded CompletionState = "succeeded"
	// CompletionFailed: a step was abandoned (dead) after exhausting retries.
	CompletionFailed CompletionState = "failed"
	// CompletionCanceled: the context was canceled mid-processing.
	CompletionCanceled CompletionState = "canceled"
)

// Completion is the outbound completion-callback envelope. It mirrors
// callback_task.Envelope field-for-field ({task_id, state, progress, result_ref,
// error, ts}) so a WebhookSink adapter is a straight copy — the Processing Engine
// consumes a single completion shape regardless of the underlying provider.
type Completion struct {
	// TaskID is the post id (the task the callback reports on).
	TaskID string
	// State is the terminal outcome.
	State CompletionState
	// Progress is the fraction of resolved steps that succeeded (0.0..1.0).
	Progress float64
	// ResultRef references the primary produced artifact, if any (e.g. "asset:…").
	ResultRef string
	// Error is the final error string on a failed completion (empty on success).
	Error string
	// TS is the completion time.
	TS time.Time
}

// Callbacker fires the completion callback when a post reaches a terminal state —
// the callback seam. The real callback_task.WebhookSink (HMAC-signed outbound
// webhook) satisfies it via a thin adapter. Notify SHOULD honor ctx cancellation.
type Callbacker interface {
	Notify(ctx context.Context, c Completion) error
}

// CallbackerFunc adapts a plain function to the Callbacker seam.
type CallbackerFunc func(ctx context.Context, c Completion) error

// Notify calls the underlying function.
func (f CallbackerFunc) Notify(ctx context.Context, c Completion) error { return f(ctx, c) }

// nopCallbacker is the default callbacker used when none is injected; it does
// nothing and reports success.
type nopCallbacker struct{}

func (nopCallbacker) Notify(context.Context, Completion) error { return nil }
