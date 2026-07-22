package processing

import "sync"

// ClaimState is the lifecycle state of a claimed post.
type ClaimState int

const (
	// ClaimUnclaimed means no claim exists for the post id (the zero/absent state).
	ClaimUnclaimed ClaimState = iota
	// ClaimProcessing means the post has been claimed and is being processed.
	ClaimProcessing
	// ClaimDone means the post finished processing with all steps succeeded.
	ClaimDone
	// ClaimFailed means the post finished processing with at least one dead step
	// (or was canceled mid-run).
	ClaimFailed
)

// String returns a readable name for the claim state.
func (s ClaimState) String() string {
	switch s {
	case ClaimUnclaimed:
		return "unclaimed"
	case ClaimProcessing:
		return "processing"
	case ClaimDone:
		return "done"
	case ClaimFailed:
		return "failed"
	default:
		return "unknown"
	}
}

// Claimer provides idempotent single-claim per post id — the exactly-once seam.
// Claim must succeed exactly once per post: the first caller to Claim a given id
// wins; every later Claim for that id (Processing, Done or Failed) is rejected.
// This is what makes the pipeline safe under duplicate "new post" triggers. The
// real skill_dispatch.ClaimRegistry (and a Postgres-backed registry) satisfy it.
type Claimer interface {
	// Claim attempts to claim postID, returning true only for the first caller.
	Claim(postID string) bool
	// MarkDone records a successful completion.
	MarkDone(postID string)
	// MarkFailed records a terminal failure; the post stays claimed so duplicate
	// triggers will not reprocess it.
	MarkFailed(postID string)
	// State returns the current claim state (ClaimUnclaimed if absent).
	State(postID string) ClaimState
}

// MemoryClaimer is the default in-memory Claimer: an atomic single-claim registry
// guarded by one mutex, so concurrent Claims for the same id are resolved
// atomically — exactly one returns true. It is the reference implementation of the
// exactly-once seam and is safe for concurrent use.
type MemoryClaimer struct {
	mu     sync.Mutex
	states map[string]ClaimState
}

// NewMemoryClaimer returns an empty MemoryClaimer.
func NewMemoryClaimer() *MemoryClaimer {
	return &MemoryClaimer{states: make(map[string]ClaimState)}
}

// Claim moves postID to Processing and returns true only if it was previously
// unclaimed; otherwise it returns false and leaves the existing state untouched.
func (c *MemoryClaimer) Claim(postID string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, exists := c.states[postID]; exists {
		return false
	}
	c.states[postID] = ClaimProcessing
	return true
}

// MarkDone records a successful completion. It is a no-op if postID was never
// claimed.
func (c *MemoryClaimer) MarkDone(postID string) { c.transition(postID, ClaimDone) }

// MarkFailed records a terminal failure. It is a no-op if postID was never claimed.
func (c *MemoryClaimer) MarkFailed(postID string) { c.transition(postID, ClaimFailed) }

// Release removes any claim for postID, allowing it to be claimed again. It backs
// the explicit reprocess/refresh trigger; it is NOT part of the normal duplicate-
// trigger path.
func (c *MemoryClaimer) Release(postID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.states, postID)
}

// State returns the current claim state for postID (ClaimUnclaimed if absent).
func (c *MemoryClaimer) State(postID string) ClaimState {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.states[postID]
}

func (c *MemoryClaimer) transition(postID string, to ClaimState) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, exists := c.states[postID]; !exists {
		return
	}
	c.states[postID] = to
}
