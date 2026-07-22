package skilldispatch

import (
	"sync"
	"sync/atomic"
	"testing"
)

func TestClaim_FirstWinsSecondRejected(t *testing.T) {
	c := NewClaimRegistry()
	if !c.Claim("post-1") {
		t.Fatal("first claim should win")
	}
	if c.Claim("post-1") {
		t.Fatal("second claim of same id must be rejected")
	}
	if got := c.State("post-1"); got != ClaimProcessing {
		t.Fatalf("state = %v, want processing", got)
	}
}

func TestClaim_MarkDoneAndFailedRejectReclaim(t *testing.T) {
	c := NewClaimRegistry()
	c.Claim("done")
	c.MarkDone("done")
	if got := c.State("done"); got != ClaimDone {
		t.Fatalf("state = %v, want done", got)
	}
	if c.Claim("done") {
		t.Fatal("a done post must not be re-claimable under duplicate triggers")
	}

	c.Claim("failed")
	c.MarkFailed("failed")
	if got := c.State("failed"); got != ClaimFailed {
		t.Fatalf("state = %v, want failed", got)
	}
	if c.Claim("failed") {
		t.Fatal("a failed post must not be re-claimable under duplicate triggers")
	}
}

func TestClaim_ReleaseAllowsReclaim(t *testing.T) {
	c := NewClaimRegistry()
	c.Claim("p")
	c.Release("p")
	if got := c.State("p"); got != ClaimUnclaimed {
		t.Fatalf("state after release = %v, want unclaimed", got)
	}
	if !c.Claim("p") {
		t.Fatal("released post should be claimable again (explicit reprocess)")
	}
}

func TestClaim_MarkOnUnclaimedIsNoop(t *testing.T) {
	c := NewClaimRegistry()
	c.MarkDone("ghost")
	if got := c.State("ghost"); got != ClaimUnclaimed {
		t.Fatalf("marking an unclaimed id changed state to %v, want unclaimed", got)
	}
}

// Under many concurrent Claims for the same id, exactly one wins. This is the
// core of the exactly-once guarantee; -race must stay clean.
func TestClaim_ConcurrentSingleWinner(t *testing.T) {
	c := NewClaimRegistry()
	const goroutines = 200

	var wins atomic.Int64
	var wg sync.WaitGroup
	start := make(chan struct{})
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			<-start
			if c.Claim("hot") {
				wins.Add(1)
			}
		}()
	}
	close(start)
	wg.Wait()

	if wins.Load() != 1 {
		t.Fatalf("concurrent claims produced %d winners, want exactly 1", wins.Load())
	}
}
