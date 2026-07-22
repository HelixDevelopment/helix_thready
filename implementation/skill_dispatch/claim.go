package skilldispatch

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
	// ClaimFailed means the post finished processing with at least one dead step.
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

// ClaimRegistry provides idempotent single-claim per post id — the exactly-once
// guarantee. Claim succeeds exactly once per post: the first caller to Claim a
// given id wins and the post moves to Processing; every later Claim for that id
// (whether it is Processing, Done or Failed) is rejected. This is what makes the
// pipeline safe under duplicate "new post" triggers.
//
// All operations are serialized by a single mutex, so concurrent Claims for the
// same id are resolved atomically — exactly one returns true.
type ClaimRegistry struct {
	mu     sync.Mutex
	states map[string]ClaimState
}

// NewClaimRegistry returns an empty ClaimRegistry.
func NewClaimRegistry() *ClaimRegistry {
	return &ClaimRegistry{states: make(map[string]ClaimState)}
}

// Claim attempts to claim postID. It returns true and moves the post to
// Processing only if the post was previously unclaimed; otherwise it returns
// false and leaves the existing state untouched. It is safe for concurrent use:
// under duplicate triggers exactly one caller receives true.
func (c *ClaimRegistry) Claim(postID string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, exists := c.states[postID]; exists {
		return false
	}
	c.states[postID] = ClaimProcessing
	return true
}

// MarkDone records a successful completion. It is a no-op if the post was never
// claimed.
func (c *ClaimRegistry) MarkDone(postID string) {
	c.transition(postID, ClaimDone)
}

// MarkFailed records a terminal failure (a dead step). The post stays claimed in
// the Failed state, so duplicate triggers will not reprocess it — reprocessing a
// failed post is an explicit operation via Release.
func (c *ClaimRegistry) MarkFailed(postID string) {
	c.transition(postID, ClaimFailed)
}

// Release removes any claim for postID, allowing it to be claimed again. It backs
// the explicit reprocess/refresh trigger; it is NOT part of the normal duplicate
// -trigger path.
func (c *ClaimRegistry) Release(postID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.states, postID)
}

// State returns the current claim state for postID (ClaimUnclaimed if absent).
func (c *ClaimRegistry) State(postID string) ClaimState {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.states[postID]
}

func (c *ClaimRegistry) transition(postID string, to ClaimState) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, exists := c.states[postID]; !exists {
		return
	}
	c.states[postID] = to
}
