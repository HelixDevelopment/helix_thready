package processing

import (
	"context"
	"errors"
)

// StepResult records one Skill step. A Skill's Run populates the produced fields
// (Output, Artifacts); the Processor owns and fills the execution fields
// (SkillName, Kind, Attempts, Succeeded, Dead, Err). The same struct is returned
// by Run and recorded in Result.Steps.
type StepResult struct {
	// SkillName echoes the Skill that produced this result (engine-owned).
	SkillName string
	// Kind is the stage this step belongs to (engine-owned).
	Kind Kind
	// Output is a short, human-readable summary (skill-produced), e.g. an asset ref.
	Output string
	// Artifacts are opaque references the Skill produced (asset ids, doc ids,
	// embedding ids); the engine passes them through untouched and unions them into
	// Result.Assets (skill-produced).
	Artifacts []string
	// Attempts is the number of Run attempts made, 1..MaxAttempts (engine-owned).
	Attempts int
	// Succeeded is true if the step ultimately succeeded (engine-owned).
	Succeeded bool
	// Dead is true if the step was abandoned after retries/permanent error
	// (engine-owned).
	Dead bool
	// Err is the final error string on failure (engine-owned).
	Err string
}

// Skill is a runnable step in the pipeline — the StepRunner seam. It composes over
// a helix_skills knowledge unit: Kind places it in the stage precedence and Run
// performs the (idempotent) work. A SkillSet decides which Skills apply to a post,
// so Skill itself carries no Match — resolution is the SkillSet's job.
//
// Run MUST be safe to re-run: the engine retries transient failures, so a Skill
// that half-completed and is retried should converge to the same result rather
// than double-apply. Return a plain error for a transient failure (it will be
// retried) or wrap it with Permanent to signal a non-retryable failure. Run should
// populate only Output and Artifacts on the returned StepResult; the engine owns
// the rest.
type Skill interface {
	// Name is the Skill's stable identifier, used for ordering ties, events and
	// results (e.g. "video.download", "tech.research").
	Name() string
	// Kind is the pipeline stage this Skill belongs to (drives precedence order).
	Kind() Kind
	// Run executes the Skill for the post. A nil error means success; a non-nil
	// error is retried unless wrapped with Permanent.
	Run(ctx context.Context, post Post) (StepResult, error)
}

// StepRunner is an alias for Skill — the "runnable step" seam under its other name.
type StepRunner = Skill

// permanentError marks an error as non-retryable. The engine treats every other
// error as transient (retryable) — a deliberate default: an unclassified failure
// is retried rather than silently dropped.
type permanentError struct{ err error }

// Permanent wraps err to signal the engine that the failure is terminal and must
// NOT be retried (e.g. a validation error, a 4xx, corrupt input).
func Permanent(err error) error {
	if err == nil {
		return nil
	}
	return &permanentError{err: err}
}

func (e *permanentError) Error() string { return e.err.Error() }
func (e *permanentError) Unwrap() error { return e.err }

// IsRetryable reports whether err should be retried. Every error is retryable
// unless it (or something it wraps) is a Permanent error.
func IsRetryable(err error) bool {
	if err == nil {
		return false
	}
	var p *permanentError
	return !errors.As(err, &p)
}
