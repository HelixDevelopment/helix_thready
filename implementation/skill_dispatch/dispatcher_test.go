package skilldispatch

import (
	"context"
	"reflect"
	"sync"
	"testing"
	"time"
)

// fastRetry is a retry policy with negligible backoff, so retry tests run fast
// while still exercising the real exponential-backoff path.
func fastRetry(maxAttempts int) RetryPolicy {
	return RetryPolicy{MaxAttempts: maxAttempts, BaseDelay: time.Millisecond, MaxDelay: 5 * time.Millisecond}
}

// A post tagged with both #Research and #Video runs BOTH the download and the
// research Skill (additive categories) — mirrors processing-pipeline.md §11.
func TestDispatch_MultiCategoryRunsBoth(t *testing.T) {
	rec := &recorder{}
	dl := &fakeSkill{name: "video.download", kind: KindDownload, tags: []string{"Video"}, rec: rec}
	rs := &fakeSkill{name: "tech.research", kind: KindResearch, tags: []string{"Research"}, rec: rec}

	reg := NewRegistry()
	reg.Register(dl, rs)
	d := NewDispatcher(reg, WithRetry(fastRetry(3)))

	post := Post{ID: "p", Hashtags: []string{"Research", "Video", "TODO", "ToDownload"}}
	res, err := d.Process(context.Background(), post)
	if err != nil {
		t.Fatalf("Process error: %v", err)
	}
	if res.State != PostCompleted {
		t.Fatalf("state = %v, want completed", res.State)
	}
	got := rec.snapshot()
	if !reflect.DeepEqual(got, []string{"video.download", "tech.research"}) {
		t.Fatalf("ran %v, want both video.download and tech.research", got)
	}
}

// Execution order matches precedence: a post tagged research+download runs the
// download-kind Skill BEFORE the research-kind one, EVEN when the research Skill
// is registered first (proving the orderer, not registration order, decides).
func TestDispatch_ExecutionOrderMatchesPrecedence(t *testing.T) {
	rec := &recorder{}
	rs := &fakeSkill{name: "tech.research", kind: KindResearch, tags: []string{"Research"}, rec: rec}
	dl := &fakeSkill{name: "video.download", kind: KindDownload, tags: []string{"Video"}, rec: rec}

	reg := NewRegistry()
	reg.Register(rs, dl) // research registered FIRST, on purpose
	d := NewDispatcher(reg, WithRetry(fastRetry(3)))

	post := Post{ID: "p", Hashtags: []string{"Research", "Video"}}
	if _, err := d.Process(context.Background(), post); err != nil {
		t.Fatalf("Process error: %v", err)
	}

	got := rec.snapshot()
	di, ri := indexOf(got, "video.download"), indexOf(got, "tech.research")
	if di < 0 || ri < 0 {
		t.Fatalf("both skills should have run, recorded %v", got)
	}
	if di >= ri {
		t.Fatalf("download ran at %d, research at %d — download must precede research (order %v)", di, ri, got)
	}
}

// Idempotency (sequential): processing the SAME post twice runs each skill
// exactly once; the second Process is a claim-rejected no-op.
func TestDispatch_Idempotency_Sequential(t *testing.T) {
	dl := &fakeSkill{name: "video.download", kind: KindDownload, tags: []string{"Video"}}
	reg := NewRegistry()
	reg.Register(dl)
	d := NewDispatcher(reg, WithRetry(fastRetry(3)))

	post := Post{ID: "dup", Hashtags: []string{"Video"}}

	first, err := d.Process(context.Background(), post)
	if err != nil {
		t.Fatalf("first Process error: %v", err)
	}
	if !first.Claimed || first.State != PostCompleted {
		t.Fatalf("first: claimed=%v state=%v, want claimed completed", first.Claimed, first.State)
	}

	second, err := d.Process(context.Background(), post)
	if err != nil {
		t.Fatalf("second Process error: %v", err)
	}
	if second.Claimed || second.State != PostRejected {
		t.Fatalf("second: claimed=%v state=%v, want rejected no-op", second.Claimed, second.State)
	}
	if len(second.Steps) != 0 {
		t.Fatalf("rejected Process ran %d steps, want 0", len(second.Steps))
	}
	if dl.Calls() != 1 {
		t.Fatalf("skill ran %d times across two Process calls, want exactly 1", dl.Calls())
	}
}

// Idempotency (concurrent): two goroutines Process the same post at once; the
// skill still runs exactly once and exactly one Process wins the claim. Must be
// race-clean under -race.
func TestDispatch_Idempotency_Concurrent(t *testing.T) {
	dl := &fakeSkill{name: "video.download", kind: KindDownload, tags: []string{"Video"}}
	reg := NewRegistry()
	reg.Register(dl)
	d := NewDispatcher(reg, WithRetry(fastRetry(3)))

	post := Post{ID: "race", Hashtags: []string{"Video"}}

	var wg sync.WaitGroup
	results := make([]PostResult, 2)
	start := make(chan struct{})
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			<-start
			r, err := d.Process(context.Background(), post)
			if err != nil {
				t.Errorf("Process error: %v", err)
			}
			results[idx] = r
		}(i)
	}
	close(start)
	wg.Wait()

	claimed := 0
	rejected := 0
	for _, r := range results {
		if r.Claimed {
			claimed++
		} else {
			rejected++
		}
	}
	if claimed != 1 || rejected != 1 {
		t.Fatalf("concurrent Process: claimed=%d rejected=%d, want 1 and 1", claimed, rejected)
	}
	if dl.Calls() != 1 {
		t.Fatalf("skill ran %d times under concurrent duplicate triggers, want exactly 1", dl.Calls())
	}
}

// Retry: a skill that fails transiently twice then succeeds runs exactly 3 times
// and the post completes.
func TestDispatch_Retry_TransientThenSucceed(t *testing.T) {
	sink := &spySink{}
	flaky := &fakeSkill{name: "video.download", kind: KindDownload, tags: []string{"Video"}, failFirst: 2}
	reg := NewRegistry()
	reg.Register(flaky)
	d := NewDispatcher(reg, WithRetry(fastRetry(3)), WithEventSink(sink))

	res, err := d.Process(context.Background(), Post{ID: "p", Hashtags: []string{"Video"}})
	if err != nil {
		t.Fatalf("Process error: %v", err)
	}
	if flaky.Calls() != 3 {
		t.Fatalf("skill ran %d times, want 3 (2 transient failures then success)", flaky.Calls())
	}
	if res.State != PostCompleted {
		t.Fatalf("state = %v, want completed", res.State)
	}
	if len(res.Steps) != 1 || !res.Steps[0].Succeeded || res.Steps[0].Attempts != 3 {
		t.Fatalf("step = %+v, want 1 succeeded step with 3 attempts", res.Steps)
	}
	if got := sink.count(EventStepFailed); got != 2 {
		t.Fatalf("failed events = %d, want 2", got)
	}
	if got := sink.count(EventStepSucceeded); got != 1 {
		t.Fatalf("succeeded events = %d, want 1", got)
	}
}

// Retry to death: a skill that always fails is retried to the ceiling, then the
// step is marked dead, failure events are emitted, and the post fails.
func TestDispatch_Retry_AlwaysFail_Dead(t *testing.T) {
	sink := &spySink{}
	broken := &fakeSkill{name: "video.download", kind: KindDownload, tags: []string{"Video"}, always: true}
	reg := NewRegistry()
	reg.Register(broken)
	d := NewDispatcher(reg, WithRetry(fastRetry(3)), WithEventSink(sink))

	res, err := d.Process(context.Background(), Post{ID: "p", Hashtags: []string{"Video"}})
	if err != nil {
		t.Fatalf("Process error: %v", err)
	}
	if broken.Calls() != 3 {
		t.Fatalf("skill ran %d times, want 3 (retried to max)", broken.Calls())
	}
	if res.State != PostFailed {
		t.Fatalf("state = %v, want failed", res.State)
	}
	if len(res.Steps) != 1 || !res.Steps[0].Dead {
		t.Fatalf("step = %+v, want 1 dead step", res.Steps)
	}
	if got := sink.count(EventStepFailed); got != 3 {
		t.Fatalf("failed events = %d, want 3 (one per attempt)", got)
	}
	if got := sink.count(EventStepDead); got != 1 {
		t.Fatalf("dead events = %d, want 1", got)
	}
	if got := sink.count(EventPostFailed); got != 1 {
		t.Fatalf("post.failed events = %d, want 1", got)
	}
	if d.Claims().State("p") != ClaimFailed {
		t.Fatalf("claim state = %v, want failed", d.Claims().State("p"))
	}
}

// A Permanent error is NOT retried: the skill runs once, the step dies, and a
// single failure event is emitted.
func TestDispatch_PermanentError_NoRetry(t *testing.T) {
	sink := &spySink{}
	bad := &fakeSkill{name: "video.download", kind: KindDownload, tags: []string{"Video"}, permanent: true}
	reg := NewRegistry()
	reg.Register(bad)
	d := NewDispatcher(reg, WithRetry(fastRetry(3)), WithEventSink(sink))

	res, err := d.Process(context.Background(), Post{ID: "p", Hashtags: []string{"Video"}})
	if err != nil {
		t.Fatalf("Process error: %v", err)
	}
	if bad.Calls() != 1 {
		t.Fatalf("permanent-failing skill ran %d times, want exactly 1 (no retry)", bad.Calls())
	}
	if res.State != PostFailed || !res.Steps[0].Dead {
		t.Fatalf("want failed post with dead step, got state=%v steps=%+v", res.State, res.Steps)
	}
	if got := sink.count(EventStepFailed); got != 1 {
		t.Fatalf("failed events = %d, want 1", got)
	}
}

// Events: the injected sink receives the expected start/success/failure events in
// order for a two-stage happy path.
func TestDispatch_Events_InOrder(t *testing.T) {
	sink := &spySink{}
	dl := &fakeSkill{name: "video.download", kind: KindDownload, tags: []string{"Video"}}
	rs := &fakeSkill{name: "tech.research", kind: KindResearch, tags: []string{"Research"}}
	reg := NewRegistry()
	reg.Register(dl, rs)
	d := NewDispatcher(reg, WithRetry(fastRetry(3)), WithEventSink(sink))

	post := Post{ID: "p", Hashtags: []string{"Video", "Research"}}
	if _, err := d.Process(context.Background(), post); err != nil {
		t.Fatalf("Process error: %v", err)
	}

	want := []pair{
		{EventPostClaimed, ""},
		{EventStepStarted, "video.download"},
		{EventStepSucceeded, "video.download"},
		{EventStepStarted, "tech.research"},
		{EventStepSucceeded, "tech.research"},
		{EventPostCompleted, ""},
	}
	got := sink.pairs()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("event sequence mismatch:\n got  %v\n want %v", got, want)
	}
}

// A duplicate trigger emits a single post.rejected event and nothing else.
func TestDispatch_Events_RejectedDuplicate(t *testing.T) {
	sink := &spySink{}
	dl := &fakeSkill{name: "video.download", kind: KindDownload, tags: []string{"Video"}}
	reg := NewRegistry()
	reg.Register(dl)
	d := NewDispatcher(reg, WithRetry(fastRetry(3)), WithEventSink(sink))

	post := Post{ID: "p", Hashtags: []string{"Video"}}
	if _, err := d.Process(context.Background(), post); err != nil {
		t.Fatalf("first Process error: %v", err)
	}
	before := len(sink.snapshot())
	if _, err := d.Process(context.Background(), post); err != nil {
		t.Fatalf("second Process error: %v", err)
	}
	after := sink.snapshot()
	if len(after)-before != 1 {
		t.Fatalf("duplicate Process emitted %d events, want exactly 1", len(after)-before)
	}
	if last := after[len(after)-1]; last.Type != EventPostRejected {
		t.Fatalf("last event = %v, want post.rejected", last.Type)
	}
}

// Fail-fast: when an early stage dies, later stages are skipped (they consume
// earlier outputs), and the post is marked failed.
func TestDispatch_DeadStep_SkipsLaterStages(t *testing.T) {
	rec := &recorder{}
	dl := &fakeSkill{name: "video.download", kind: KindDownload, tags: []string{"Video"}, always: true, rec: rec}
	rs := &fakeSkill{name: "tech.research", kind: KindResearch, tags: []string{"Research"}, rec: rec}
	reg := NewRegistry()
	reg.Register(dl, rs)
	d := NewDispatcher(reg, WithRetry(fastRetry(2)), WithEventSink(&spySink{}))

	post := Post{ID: "p", Hashtags: []string{"Video", "Research"}}
	res, err := d.Process(context.Background(), post)
	if err != nil {
		t.Fatalf("Process error: %v", err)
	}
	if res.State != PostFailed {
		t.Fatalf("state = %v, want failed", res.State)
	}
	if got := rec.snapshot(); indexOf(got, "tech.research") != -1 {
		t.Fatalf("research ran after download died; recorded %v, want download only", got)
	}
	if len(res.Steps) != 1 {
		t.Fatalf("recorded %d steps, want 1 (fail-fast after dead download)", len(res.Steps))
	}
}

// Context cancellation mid-run stops retries and surfaces a canceled post.
func TestDispatch_ContextCancellation(t *testing.T) {
	broken := &fakeSkill{name: "video.download", kind: KindDownload, tags: []string{"Video"}, always: true}
	reg := NewRegistry()
	reg.Register(broken)
	// Long backoff so cancellation lands during the wait between attempts.
	d := NewDispatcher(reg, WithRetry(RetryPolicy{MaxAttempts: 5, BaseDelay: time.Second, MaxDelay: time.Second}))

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(20 * time.Millisecond)
		cancel()
	}()

	res, err := d.Process(ctx, Post{ID: "p", Hashtags: []string{"Video"}})
	if err == nil {
		t.Fatal("want context error, got nil")
	}
	if res.State != PostCanceled {
		t.Fatalf("state = %v, want canceled", res.State)
	}
	if broken.Calls() >= 5 {
		t.Fatalf("skill ran %d times; cancellation should have stopped retries early", broken.Calls())
	}
}
