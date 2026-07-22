package processing

import (
	"sync"
	"sync/atomic"
	"testing"
)

// TestOrderByPrecedence_SortsAndIsStable proves the orderer imposes
// download<convert<analyze<research<reply regardless of input order, keeps input
// order within a kind (stable), and does not mutate its input.
func TestOrderByPrecedence_SortsAndIsStable(t *testing.T) {
	rec := &recorder{}
	reply := succeeds("reply", KindReply, rec)
	research := succeeds("research", KindResearch, rec)
	analyzeA := succeeds("analyze.a", KindAnalyze, rec)
	analyzeB := succeeds("analyze.b", KindAnalyze, rec)
	download := succeeds("download", KindDownload, rec)

	// Scrambled input; two analyze skills registered A-before-B.
	in := []Skill{reply, research, analyzeA, analyzeB, download}
	got := OrderByPrecedence(in)

	wantNames := []string{"download", "analyze.a", "analyze.b", "research", "reply"}
	gotNames := make([]string, len(got))
	for i, s := range got {
		gotNames[i] = s.Name()
	}
	eq(t, "ordered names", gotNames, wantNames)

	// Input slice must be untouched (same order as constructed).
	if in[0].Name() != "reply" || in[4].Name() != "download" {
		t.Fatalf("input slice was mutated: %v", func() []string {
			ns := make([]string, len(in))
			for i, s := range in {
				ns[i] = s.Name()
			}
			return ns
		}())
	}
}

// TestMemoryClaimer_SingleWinnerConcurrent proves the claim registry yields exactly
// one winner under concurrent claims for the same id.
func TestMemoryClaimer_SingleWinnerConcurrent(t *testing.T) {
	c := NewMemoryClaimer()
	const n = 64
	var wins atomic.Int64
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			if c.Claim("same-id") {
				wins.Add(1)
			}
		}()
	}
	wg.Wait()
	if wins.Load() != 1 {
		t.Fatalf("claim winners = %d, want exactly 1", wins.Load())
	}
	if c.State("same-id") != ClaimProcessing {
		t.Fatalf("state = %v, want processing", c.State("same-id"))
	}
}

// TestMemoryClaimer_Lifecycle proves the done/failed transitions and that marking an
// unclaimed id is a no-op.
func TestMemoryClaimer_Lifecycle(t *testing.T) {
	c := NewMemoryClaimer()

	if got := c.State("x"); got != ClaimUnclaimed {
		t.Fatalf("initial state = %v, want unclaimed", got)
	}
	c.MarkDone("x") // no-op: never claimed
	if got := c.State("x"); got != ClaimUnclaimed {
		t.Fatalf("MarkDone on unclaimed changed state to %v", got)
	}

	if !c.Claim("x") {
		t.Fatal("first claim should win")
	}
	if c.Claim("x") {
		t.Fatal("second claim should be rejected")
	}
	c.MarkDone("x")
	if got := c.State("x"); got != ClaimDone {
		t.Fatalf("state = %v, want done", got)
	}

	// A done post cannot be reclaimed (idempotent) until explicitly released.
	if c.Claim("x") {
		t.Fatal("claim on a done post should be rejected")
	}
	c.Release("x")
	if !c.Claim("x") {
		t.Fatal("claim after Release should win")
	}
	c.MarkFailed("x")
	if got := c.State("x"); got != ClaimFailed {
		t.Fatalf("state = %v, want failed", got)
	}
}

// TestPost_Hashtags proves case-insensitive, '#'-tolerant hashtag matching.
func TestPost_Hashtags(t *testing.T) {
	p := Post{Hashtags: []string{"#Video", "Research"}}
	if !p.HasHashtag("video") {
		t.Fatal("HasHashtag should be case-insensitive and '#'-tolerant")
	}
	if !p.HasHashtag("#research") {
		t.Fatal("HasHashtag should match Research via #research")
	}
	if p.HasHashtag("Torrent") {
		t.Fatal("HasHashtag matched an absent tag")
	}
	if !p.HasAnyHashtag("Torrent", "video") {
		t.Fatal("HasAnyHashtag should match on the second tag")
	}
	if p.HasAnyHashtag("A", "B") {
		t.Fatal("HasAnyHashtag matched none-present tags")
	}
}

// TestKind_StringAndValid covers the stage-name mapping and validity bounds.
func TestKind_StringAndValid(t *testing.T) {
	cases := map[Kind]string{
		KindDownload: "download",
		KindConvert:  "convert",
		KindAnalyze:  "analyze",
		KindResearch: "research",
		KindReply:    "reply",
	}
	for k, want := range cases {
		if k.String() != want {
			t.Fatalf("Kind(%d).String() = %q, want %q", k, k.String(), want)
		}
		if !k.Valid() {
			t.Fatalf("Kind(%d) should be valid", k)
		}
	}
	if Kind(99).Valid() {
		t.Fatal("Kind(99) should be invalid")
	}
	if Kind(99).String() != "unknown" {
		t.Fatalf("Kind(99).String() = %q, want unknown", Kind(99).String())
	}
}

// TestRetryPolicy_Delay covers the exponential backoff schedule and the cap.
func TestRetryPolicy_Delay(t *testing.T) {
	p := RetryPolicy{MaxAttempts: 5, BaseDelay: 100, MaxDelay: 350}
	// Delay(1)=0, Delay(2)=base, Delay(3)=2*base, Delay(4)=4*base -> capped at 350.
	if got := p.Delay(1); got != 0 {
		t.Fatalf("Delay(1) = %v, want 0", got)
	}
	if got := p.Delay(2); got != 100 {
		t.Fatalf("Delay(2) = %v, want 100", got)
	}
	if got := p.Delay(3); got != 200 {
		t.Fatalf("Delay(3) = %v, want 200", got)
	}
	if got := p.Delay(4); got != 350 {
		t.Fatalf("Delay(4) = %v, want 350 (capped)", got)
	}
	// A zero/negative MaxAttempts is treated as a single attempt.
	if got := (RetryPolicy{}).attempts(); got != 1 {
		t.Fatalf("attempts() on zero policy = %d, want 1", got)
	}
}
