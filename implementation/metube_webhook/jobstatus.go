// Package metubewebhook is the Helix Thready MeTube outbound-completion-webhook
// shim.
//
// It closes gap register [GAP: 6.5]: MeTube (milos85vasic/YT-DLP) tracks
// download / post-process jobs but is POLL-ONLY — it exposes
// GET /api/postprocess/status and GET /api/postprocess/jobs and has no outbound
// completion webhook, so an external orchestrator cannot be notified when a job
// finishes. This module is the shim: it polls MeTube's postprocess job list,
// detects transitions to a terminal state (finished -> success, error ->
// failure), and fires a single standardized, HMAC-signed completion webhook per
// job to a configured sink URL.
//
// The outbound envelope matches the canonical callback_task completion shape
// {job_id, state, progress, result_ref, error, ts} and is delivered with the
// "X-Thready-Signature: sha256=<hex>" header (HMAC-SHA256 over the exact raw
// request body), mirroring the event-bus contract §9 signing scheme.
//
// Scope note: this is the OUTBOUND shim only. It sits beside MeTube and requires
// no MeTube change. Adding a native completion webhook to MeTube itself upstream
// is a separate change.
//
// The module is self-contained and depends only on the Go standard library.
package metubewebhook

// JobState mirrors the status vocabulary of a MeTube postprocess job.
type JobState string

const (
	// StatePending: queued, not yet started.
	StatePending JobState = "pending"
	// StateDownloading: media transfer in progress.
	StateDownloading JobState = "downloading"
	// StatePostprocessing: download complete, post-processing (mux/convert) running.
	StatePostprocessing JobState = "postprocessing"
	// StateFinished: terminal success; ResultPath is set.
	StateFinished JobState = "finished"
	// StateError: terminal failure; Error is set.
	StateError JobState = "error"
)

// Terminal reports whether s is a terminal state (no further transitions).
func (s JobState) Terminal() bool {
	switch s {
	case StateFinished, StateError:
		return true
	default:
		return false
	}
}

// Valid reports whether s is one of the recognized MeTube states.
func (s JobState) Valid() bool {
	switch s {
	case StatePending, StateDownloading, StatePostprocessing, StateFinished, StateError:
		return true
	default:
		return false
	}
}

// JobStatus is the normalized snapshot of a single MeTube postprocess job,
// mapped from MeTube's on-the-wire status shape by the parser.
type JobStatus struct {
	// ID is MeTube's job identifier (stable across polls).
	ID string
	// State is the current lifecycle state.
	State JobState
	// Progress is normalized to 0.0..1.0 (MeTube reports percent 0..100).
	Progress float64
	// ResultPath is the output filename / result path once finished.
	ResultPath string
	// Error is the failure message when State == StateError.
	Error string
}
