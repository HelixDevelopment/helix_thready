package processing

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

// fastRetry is a tiny retry policy so retry tests run instantly.
func fastRetry(max int) RetryPolicy {
	return RetryPolicy{MaxAttempts: max, BaseDelay: time.Millisecond, MaxDelay: 5 * time.Millisecond}
}

// videoResearchSet returns a SkillSet that resolves the three capstone skills in
// DELIBERATELY reversed precedence order (research, analyze, download), so a passing
// order assertion proves the orderer — not the registration order — imposes
// download < analyze < research.
func videoResearchSet(dl, oc, rs Skill) SkillSet {
	return SkillSetFunc(func(p Post) []Skill {
		var out []Skill
		if p.HasHashtag("Research") {
			out = append(out, rs)
		}
		if p.HasAnyHashtag("Video", "Image") {
			out = append(out, oc)
		}
		if p.HasAnyHashtag("Video", "ToDownload") {
			out = append(out, dl)
		}
		return out
	})
}

func videoResearchPost() Post {
	return Post{
		ID:       "post-1",
		ThreadID: "thread-1",
		Hashtags: []string{"Video", "Research", "ToDownload"},
		Text:     "New lab drop https://youtu.be/x #Video #Research #ToDownload",
	}
}

func eq(t *testing.T, name string, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s length = %d, want %d\n got=%v\nwant=%v", name, len(got), len(want), got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("%s[%d] = %q, want %q\n got=%v\nwant=%v", name, i, got[i], want[i], got, want)
		}
	}
}

// TestProcess_MultiCategory_PrecedenceOrder_EventsAndCallback is the headline
// scenario: a #Video #Research post runs its skills in precedence order
// (download-kind before analyze-kind before research-kind), with the full lifecycle
// event stream and a completion callback carrying the succeeded state.
func TestProcess_MultiCategory_PrecedenceOrder_EventsAndCallback(t *testing.T) {
	rec := &recorder{}
	dl := succeeds("video.download", KindDownload, rec, "asset:abc123")
	oc := succeeds("vision.ocr", KindAnalyze, rec, "ocr:eng")
	rs := succeeds("topic.research", KindResearch, rec)

	emit := &recordingEmitter{}
	cb := &recordingCallbacker{}
	proc := NewProcessor(NewMemoryClaimer(), videoResearchSet(dl, oc, rs),
		WithEmitter(emit), WithCallbacker(cb), WithRetry(fastRetry(2)))

	res, err := proc.Process(context.Background(), videoResearchPost())
	if err != nil {
		t.Fatalf("Process err = %v", err)
	}
	if res.State != StateCompleted || !res.ProcessedOnce {
		t.Fatalf("res = %+v, want completed/ProcessedOnce", res)
	}

	// Precedence: recorded run order is download -> analyze -> research despite the
	// reversed resolve order.
	eq(t, "run order", rec.snapshot(),
		[]string{"run:video.download", "run:vision.ocr", "run:topic.research"})

	// Steps recorded in execution order with the right kinds.
	if len(res.Steps) != 3 {
		t.Fatalf("steps = %d, want 3", len(res.Steps))
	}
	wantKinds := []Kind{KindDownload, KindAnalyze, KindResearch}
	for i, k := range wantKinds {
		if res.Steps[i].Kind != k || !res.Steps[i].Succeeded {
			t.Fatalf("step[%d] = %+v, want kind %v succeeded", i, res.Steps[i], k)
		}
	}

	// Assets are the union of succeeded step artifacts, in order.
	eq(t, "assets", res.Assets, []string{"asset:abc123", "ocr:eng"})

	// Full lifecycle event stream, in order.
	eq(t, "events", emit.types(), []string{
		"post.claimed",
		"step.started", "step.succeeded", // download
		"step.started", "step.succeeded", // analyze
		"step.started", "step.succeeded", // research
		"post.completed",
	})

	// Completion callback carries the right final state.
	comp, ok := cb.last()
	if !ok {
		t.Fatal("no completion callback fired")
	}
	if comp.TaskID != "post-1" || comp.State != CompletionSucceeded {
		t.Fatalf("completion = %+v, want task post-1 / succeeded", comp)
	}
	if comp.Progress != 1.0 {
		t.Fatalf("completion progress = %v, want 1.0", comp.Progress)
	}
	if comp.ResultRef != "asset:abc123" {
		t.Fatalf("completion result_ref = %q, want asset:abc123", comp.ResultRef)
	}
	if comp.Error != "" {
		t.Fatalf("completion error = %q, want empty on success", comp.Error)
	}
}

// TestProcess_Idempotency_Sequential proves the same post processed twice runs its
// skills exactly once; the second call is a claim-rejected no-op.
func TestProcess_Idempotency_Sequential(t *testing.T) {
	rec := &recorder{}
	dl := succeeds("video.download", KindDownload, rec, "asset:x")
	claimer := NewMemoryClaimer()
	cb := &recordingCallbacker{}
	proc := NewProcessor(claimer,
		SkillSetFunc(func(p Post) []Skill { return []Skill{dl} }),
		WithCallbacker(cb))

	post := Post{ID: "dup-1", Hashtags: []string{"Video"}}

	res1, err := proc.Process(context.Background(), post)
	if err != nil {
		t.Fatalf("first Process err = %v", err)
	}
	if res1.State != StateCompleted || !res1.ProcessedOnce {
		t.Fatalf("first = %+v, want completed/ProcessedOnce", res1)
	}

	res2, err := proc.Process(context.Background(), post)
	if err != nil {
		t.Fatalf("second Process err = %v", err)
	}
	if res2.State != StateRejected || res2.ProcessedOnce {
		t.Fatalf("second = %+v, want rejected/!ProcessedOnce (idempotent single-claim)", res2)
	}
	if len(res2.Steps) != 0 {
		t.Fatalf("rejected duplicate ran %d steps, want 0", len(res2.Steps))
	}
	if got := dl.runs.Load(); got != 1 {
		t.Fatalf("download ran %d times, want exactly 1", got)
	}
	if got := claimer.State("dup-1"); got != ClaimDone {
		t.Fatalf("claim state = %v, want done", got)
	}
	// Exactly one completion callback (for the winning run; the duplicate fires none).
	if got := cb.count(); got != 1 {
		t.Fatalf("callbacks = %d, want 1 (duplicate must not fire a completion)", got)
	}
}

// TestProcess_Idempotency_Concurrent proves that under concurrent duplicate triggers
// exactly one caller wins the claim and the skills run exactly once.
func TestProcess_Idempotency_Concurrent(t *testing.T) {
	rec := &recorder{}
	dl := succeeds("video.download", KindDownload, rec, "asset:x")
	oc := succeeds("vision.ocr", KindAnalyze, rec, "ocr:eng")
	rs := succeeds("topic.research", KindResearch, rec)
	claimer := NewMemoryClaimer()
	proc := NewProcessor(claimer, videoResearchSet(dl, oc, rs))

	post := videoResearchPost()

	const n = 8
	results := make(chan Result, n)
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			r, err := proc.Process(context.Background(), post)
			if err != nil {
				t.Errorf("concurrent Process err = %v", err)
			}
			results <- r
		}()
	}
	wg.Wait()
	close(results)

	winners, rejects := 0, 0
	for r := range results {
		switch {
		case r.ProcessedOnce && r.State == StateCompleted:
			winners++
		case !r.ProcessedOnce && r.State == StateRejected:
			rejects++
		default:
			t.Fatalf("unexpected result under concurrency: %+v", r)
		}
	}
	if winners != 1 || rejects != n-1 {
		t.Fatalf("winners=%d rejects=%d, want 1/%d (single claim winner)", winners, rejects, n-1)
	}
	if dl.runs.Load() != 1 || oc.runs.Load() != 1 || rs.runs.Load() != 1 {
		t.Fatalf("run counts = dl:%d oc:%d rs:%d, want 1/1/1 (exactly-once)",
			dl.runs.Load(), oc.runs.Load(), rs.runs.Load())
	}
	if got := claimer.State(post.ID); got != ClaimDone {
		t.Fatalf("claim state = %v, want done", got)
	}
}

// TestProcess_Retry_TransientThenSucceed proves a step that fails transiently twice
// then succeeds runs three times and the post completes.
func TestProcess_Retry_TransientThenSucceed(t *testing.T) {
	rec := &recorder{}
	dl := failsThenSucceeds("video.download", KindDownload, rec, 2, "asset:ok")
	emit := &recordingEmitter{}
	proc := NewProcessor(NewMemoryClaimer(),
		SkillSetFunc(func(Post) []Skill { return []Skill{dl} }),
		WithEmitter(emit), WithRetry(fastRetry(3)))

	res, err := proc.Process(context.Background(), Post{ID: "retry-1", Hashtags: []string{"Video"}})
	if err != nil {
		t.Fatalf("Process err = %v", err)
	}
	if res.State != StateCompleted {
		t.Fatalf("state = %v, want completed", res.State)
	}
	if got := dl.runs.Load(); got != 3 {
		t.Fatalf("runs = %d, want 3 (2 transient failures + 1 success)", got)
	}
	if res.Steps[0].Attempts != 3 || !res.Steps[0].Succeeded {
		t.Fatalf("step = %+v, want attempts 3 / succeeded", res.Steps[0])
	}
	eq(t, "events", emit.types(), []string{
		"post.claimed",
		"step.started",
		"step.failed", "step.failed", // two transient failures
		"step.succeeded",
		"post.completed",
	})
}

// TestProcess_AlwaysFail_Dead_FailedPost proves an always-failing step is retried to
// the ceiling, dead-lettered with failure events, and the post ends failed with a
// completion callback carrying the failed state.
func TestProcess_AlwaysFail_Dead_FailedPost(t *testing.T) {
	rec := &recorder{}
	dl := alwaysFails("video.download", KindDownload, rec)
	emit := &recordingEmitter{}
	cb := &recordingCallbacker{}
	claimer := NewMemoryClaimer()
	proc := NewProcessor(claimer,
		SkillSetFunc(func(Post) []Skill { return []Skill{dl} }),
		WithEmitter(emit), WithCallbacker(cb), WithRetry(fastRetry(3)))

	res, err := proc.Process(context.Background(), Post{ID: "dead-1", Hashtags: []string{"Video"}})
	if err != nil {
		t.Fatalf("Process err = %v (a dead step is reported via State, not error)", err)
	}
	if res.State != StateFailed {
		t.Fatalf("state = %v, want failed", res.State)
	}
	if got := dl.runs.Load(); got != 3 {
		t.Fatalf("runs = %d, want 3 (retried to the ceiling)", got)
	}
	if !res.Steps[0].Dead || res.Steps[0].Succeeded {
		t.Fatalf("step = %+v, want dead / !succeeded", res.Steps[0])
	}
	if res.Steps[0].Err == "" {
		t.Fatal("dead step has no error string")
	}
	eq(t, "events", emit.types(), []string{
		"post.claimed",
		"step.started",
		"step.failed", "step.failed", "step.failed", // three exhausted attempts
		"step.dead",
		"post.failed",
	})
	if got := claimer.State("dead-1"); got != ClaimFailed {
		t.Fatalf("claim state = %v, want failed", got)
	}
	comp, ok := cb.last()
	if !ok {
		t.Fatal("no completion callback fired for a failed post")
	}
	if comp.State != CompletionFailed || comp.Error == "" {
		t.Fatalf("completion = %+v, want failed with error", comp)
	}
	if comp.Progress != 0.0 {
		t.Fatalf("completion progress = %v, want 0.0 (no step succeeded)", comp.Progress)
	}
}

// TestProcess_PermanentError_NoRetry proves a Permanent error is not retried: the
// step runs once, dies, and the post fails.
func TestProcess_PermanentError_NoRetry(t *testing.T) {
	rec := &recorder{}
	dl := alwaysPermanent("video.download", KindDownload, rec)
	emit := &recordingEmitter{}
	proc := NewProcessor(NewMemoryClaimer(),
		SkillSetFunc(func(Post) []Skill { return []Skill{dl} }),
		WithEmitter(emit), WithRetry(fastRetry(5)))

	res, err := proc.Process(context.Background(), Post{ID: "perm-1", Hashtags: []string{"Video"}})
	if err != nil {
		t.Fatalf("Process err = %v", err)
	}
	if res.State != StateFailed {
		t.Fatalf("state = %v, want failed", res.State)
	}
	if got := dl.runs.Load(); got != 1 {
		t.Fatalf("runs = %d, want 1 (Permanent error must not be retried)", got)
	}
	eq(t, "events", emit.types(), []string{
		"post.claimed", "step.started", "step.failed", "step.dead", "post.failed",
	})
}

// TestProcess_FailFast_SkipsLaterStages proves that with fail-fast on (the default) a
// dead earlier step skips the later stage that would consume its output.
func TestProcess_FailFast_SkipsLaterStages(t *testing.T) {
	rec := &recorder{}
	dl := alwaysFails("video.download", KindDownload, rec)
	rs := succeeds("topic.research", KindResearch, rec)
	proc := NewProcessor(NewMemoryClaimer(),
		SkillSetFunc(func(Post) []Skill { return []Skill{rs, dl} }), // reversed on purpose
		WithRetry(fastRetry(2)))

	res, err := proc.Process(context.Background(), Post{ID: "ff-1"})
	if err != nil {
		t.Fatalf("Process err = %v", err)
	}
	if res.State != StateFailed {
		t.Fatalf("state = %v, want failed", res.State)
	}
	if rs.runs.Load() != 0 {
		t.Fatalf("research ran %d times, want 0 (fail-fast must skip it)", rs.runs.Load())
	}
	if len(res.Steps) != 1 || !res.Steps[0].Dead {
		t.Fatalf("steps = %+v, want 1 dead download step", res.Steps)
	}
}

// TestProcess_FailFastOff_RunsRemaining proves that with fail-fast off the remaining
// steps still run after a dead step, and the post is still marked failed.
func TestProcess_FailFastOff_RunsRemaining(t *testing.T) {
	rec := &recorder{}
	dl := alwaysFails("video.download", KindDownload, rec)
	rs := succeeds("topic.research", KindResearch, rec)
	proc := NewProcessor(NewMemoryClaimer(),
		SkillSetFunc(func(Post) []Skill { return []Skill{rs, dl} }),
		WithRetry(fastRetry(2)), WithFailFast(false))

	res, err := proc.Process(context.Background(), Post{ID: "ff-2"})
	if err != nil {
		t.Fatalf("Process err = %v", err)
	}
	if res.State != StateFailed {
		t.Fatalf("state = %v, want failed (a step died)", res.State)
	}
	if rs.runs.Load() != 1 {
		t.Fatalf("research ran %d times, want 1 (fail-fast off runs remaining)", rs.runs.Load())
	}
	if len(res.Steps) != 2 {
		t.Fatalf("steps = %d, want 2 (both stages ran)", len(res.Steps))
	}
}

// TestProcess_ContextCancellation proves cancellation mid-run (here, during backoff)
// is honored: the post ends canceled and Process returns the context error.
func TestProcess_ContextCancellation(t *testing.T) {
	rec := &recorder{}
	ctx, cancel := context.WithCancel(context.Background())
	dl := cancelsThenTransient("video.download", KindDownload, rec, cancel)
	emit := &recordingEmitter{}
	claimer := NewMemoryClaimer()
	proc := NewProcessor(claimer,
		SkillSetFunc(func(Post) []Skill { return []Skill{dl} }),
		WithEmitter(emit), WithRetry(fastRetry(3)))

	res, err := proc.Process(ctx, Post{ID: "cancel-1", Hashtags: []string{"Video"}})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
	if res.State != StateCanceled {
		t.Fatalf("state = %v, want canceled", res.State)
	}
	if got := dl.runs.Load(); got != 1 {
		t.Fatalf("runs = %d, want 1 (canceled during backoff after first attempt)", got)
	}
	if got := claimer.State("cancel-1"); got != ClaimFailed {
		t.Fatalf("claim state = %v, want failed", got)
	}
}

// TestProcess_RejectedDuplicate_FiresOnlyRejectedEvent proves a pre-claimed post is
// rejected with a single post.rejected event, no steps, and no completion callback.
func TestProcess_RejectedDuplicate_FiresOnlyRejectedEvent(t *testing.T) {
	rec := &recorder{}
	dl := succeeds("video.download", KindDownload, rec, "asset:x")
	claimer := NewMemoryClaimer()
	claimer.Claim("already") // someone else already claimed it
	emit := &recordingEmitter{}
	cb := &recordingCallbacker{}
	proc := NewProcessor(claimer,
		SkillSetFunc(func(Post) []Skill { return []Skill{dl} }),
		WithEmitter(emit), WithCallbacker(cb))

	res, err := proc.Process(context.Background(), Post{ID: "already", Hashtags: []string{"Video"}})
	if err != nil {
		t.Fatalf("Process err = %v", err)
	}
	if res.State != StateRejected || res.ProcessedOnce {
		t.Fatalf("res = %+v, want rejected/!ProcessedOnce", res)
	}
	if dl.runs.Load() != 0 {
		t.Fatalf("download ran %d times, want 0 for a rejected duplicate", dl.runs.Load())
	}
	eq(t, "events", emit.types(), []string{"post.rejected"})
	if cb.count() != 0 {
		t.Fatalf("callbacks = %d, want 0 (rejected duplicate fires no completion)", cb.count())
	}
}

// TestProcess_CallbackDeliveryError_Surfaced proves that when the completion callback
// fails to deliver, Process surfaces the delivery error while Result.State still
// reports the true (completed) processing outcome.
func TestProcess_CallbackDeliveryError_Surfaced(t *testing.T) {
	rec := &recorder{}
	dl := succeeds("video.download", KindDownload, rec, "asset:x")
	cb := &recordingCallbacker{err: errors.New("webhook 503")}
	claimer := NewMemoryClaimer()
	proc := NewProcessor(claimer,
		SkillSetFunc(func(Post) []Skill { return []Skill{dl} }),
		WithCallbacker(cb))

	res, err := proc.Process(context.Background(), Post{ID: "cbfail-1", Hashtags: []string{"Video"}})
	if err == nil {
		t.Fatal("Process err = nil, want a surfaced callback delivery error")
	}
	var ce *callbackError
	if !errors.As(err, &ce) {
		t.Fatalf("err = %v, want a *callbackError", err)
	}
	if res.State != StateCompleted {
		t.Fatalf("state = %v, want completed (processing succeeded despite delivery failure)", res.State)
	}
	if cb.count() != 1 {
		t.Fatalf("callbacks attempted = %d, want 1", cb.count())
	}
	if got := claimer.State("cbfail-1"); got != ClaimDone {
		t.Fatalf("claim state = %v, want done", got)
	}
}
