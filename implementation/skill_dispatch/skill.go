package skilldispatch

import (
	"context"
	"errors"
)

// Result is the output of a single Skill run. It is deliberately small — the
// dispatch engine only needs enough to record what a step produced; rich
// artifacts are referenced, not embedded.
type Result struct {
	// SkillName echoes the Skill that produced this result.
	SkillName string
	// Output is a short, human-readable summary (e.g. an asset ref or note).
	Output string
	// Artifacts are opaque references to things the Skill produced (asset ids,
	// doc ids, embedding ids). The engine passes them through untouched.
	Artifacts []string
}

// Skill is a runnable unit in the dispatch engine. It composes over a helix_skills
// knowledge unit: Match decides whether the Skill applies to a post, Kind places
// it in the stage precedence, and Run performs the (idempotent) work.
//
// Run MUST be safe to re-run: the engine retries transient failures, so a Skill
// that half-completed and is retried should converge to the same result rather
// than double-apply. Return a plain error for a transient failure (it will be
// retried) or wrap it with Permanent to signal a non-retryable failure.
type Skill interface {
	// Name is the Skill's stable identifier, used for ordering ties, events and
	// results (e.g. "video.download", "tech.research").
	Name() string

	// Kind is the pipeline stage this Skill belongs to (drives precedence order).
	Kind() Kind

	// Match reports whether this Skill applies to the given post. Matching is
	// additive across Skills — many Skills may match one post.
	Match(post Post) bool

	// Run executes the Skill for the post. A nil error means success; a non-nil
	// error is retried unless wrapped with Permanent.
	Run(ctx context.Context, post Post) (Result, error)
}

// permanentError marks an error as non-retryable. The engine treats every other
// error as transient (retryable) — a deliberate default: an unclassified failure
// is retried rather than silently dropped.
type permanentError struct{ err error }

// Permanent wraps err to signal the dispatch engine that the failure is terminal
// and must NOT be retried (e.g. a validation error, a 4xx, corrupt input).
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
