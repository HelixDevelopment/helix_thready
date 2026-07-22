package bobaadapter

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func TestBridge_DownloadCompleteFiresExactlyOneSignedWebhook(t *testing.T) {
	secret := []byte("bridge-secret")
	rr, srv := newRecordingReceiver(secret)
	defer srv.Close()

	sink := &WebhookSink{URL: srv.URL, Secret: secret}
	b := NewBridge(sink)
	fixedTS := time.Unix(1_700_000_500, 0).UTC()
	b.Now = func() time.Time { return fixedTS }

	// A non-terminal event first — must not fire.
	found := BobaEvent{Type: EventResultFound, ResultID: "HASH1", Title: "Ubuntu"}
	if fired, err := b.Handle(context.Background(), found); err != nil || fired {
		t.Fatalf("result_found: fired=%v err=%v, want false/nil", fired, err)
	}

	// Terminal download_complete — must fire exactly one signed webhook.
	complete := BobaEvent{
		Type:     EventDownloadComplete,
		ResultID: "HASH1",
		Progress: 1.0,
		Path:     "/downloads/ubuntu.iso",
	}
	fired, err := b.Handle(context.Background(), complete)
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if !fired {
		t.Fatal("download_complete did not fire")
	}

	envs, oks, requests := rr.snapshot()
	if requests != 1 {
		t.Fatalf("receiver requests = %d, want 1", requests)
	}
	if !oks[0] {
		t.Error("receiver-recomputed HMAC did not match")
	}
	got := envs[0]
	if got.JobID != "HASH1" {
		t.Errorf("job_id = %q, want HASH1", got.JobID)
	}
	if got.State != CompletionSuccess {
		t.Errorf("state = %q, want success", got.State)
	}
	if got.Progress != 1.0 {
		t.Errorf("progress = %v, want 1.0", got.Progress)
	}
	if got.ResultRef != "/downloads/ubuntu.iso" {
		t.Errorf("result_ref = %q, want /downloads/ubuntu.iso", got.ResultRef)
	}
	if got.Error != "" {
		t.Errorf("error = %q, want empty", got.Error)
	}
	if !got.TS.Equal(fixedTS) {
		t.Errorf("ts = %v, want %v", got.TS, fixedTS)
	}
}

func TestBridge_DownloadErrorFiresFailureWebhook(t *testing.T) {
	secret := []byte("bridge-secret")
	rr, srv := newRecordingReceiver(secret)
	defer srv.Close()

	sink := &WebhookSink{URL: srv.URL, Secret: secret}
	b := NewBridge(sink)

	ev := BobaEvent{Type: EventDownloadError, ResultID: "HASH9", Error: "no seeders"}
	fired, err := b.Handle(context.Background(), ev)
	if err != nil || !fired {
		t.Fatalf("Handle: fired=%v err=%v, want true/nil", fired, err)
	}

	envs, oks, requests := rr.snapshot()
	if requests != 1 {
		t.Fatalf("requests = %d, want 1", requests)
	}
	if !oks[0] {
		t.Error("receiver-recomputed HMAC did not match")
	}
	if envs[0].State != CompletionFailure {
		t.Errorf("state = %q, want failure", envs[0].State)
	}
	if envs[0].Error != "no seeders" {
		t.Errorf("error = %q, want %q", envs[0].Error, "no seeders")
	}
	if envs[0].ResultRef != "" {
		t.Errorf("result_ref = %q, want empty for a failure", envs[0].ResultRef)
	}
}

func TestBridge_DedupSameResultIDFiresOnce(t *testing.T) {
	secret := []byte("dedup-secret")
	rr, srv := newRecordingReceiver(secret)
	defer srv.Close()

	sink := &WebhookSink{URL: srv.URL, Secret: secret}
	b := NewBridge(sink)

	ev := BobaEvent{Type: EventDownloadComplete, ResultID: "DUP", Path: "/x"}
	// Same terminal event delivered 5 times (SSE re-emit / hook retry).
	firedCount := 0
	for i := 0; i < 5; i++ {
		fired, err := b.Handle(context.Background(), ev)
		if err != nil {
			t.Fatalf("Handle #%d: %v", i, err)
		}
		if fired {
			firedCount++
		}
	}

	if firedCount != 1 {
		t.Fatalf("fired %d times, want exactly 1", firedCount)
	}
	if _, _, requests := rr.snapshot(); requests != 1 {
		t.Fatalf("DEDUP violated: receiver got %d requests, want exactly 1", requests)
	}
	if !b.AlreadyFired("DUP") {
		t.Error("AlreadyFired(DUP) = false, want true")
	}
}

func TestBridge_DistinctResultIDsFireIndependently(t *testing.T) {
	secret := []byte("multi-secret")
	rr, srv := newRecordingReceiver(secret)
	defer srv.Close()

	sink := &WebhookSink{URL: srv.URL, Secret: secret}
	b := NewBridge(sink)

	for _, id := range []string{"A", "B", "A", "B"} {
		if _, err := b.Handle(context.Background(), BobaEvent{Type: EventDownloadComplete, ResultID: id, Path: "/" + id}); err != nil {
			t.Fatalf("Handle %s: %v", id, err)
		}
	}
	if _, _, requests := rr.snapshot(); requests != 2 {
		t.Fatalf("requests = %d, want 2 (A and B once each)", requests)
	}
}

func TestBridge_DeliveryFailureLeavesResultUnfiredForRetry(t *testing.T) {
	var mu sync.Mutex
	fail := true
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		f := fail
		mu.Unlock()
		if f {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	// No retries in the sink so the first delivery fails outright.
	sink := &WebhookSink{URL: srv.URL, Secret: []byte("k"), MaxRetries: 0}
	b := NewBridge(sink)
	ev := BobaEvent{Type: EventDownloadComplete, ResultID: "retry", Path: "/r"}

	if fired, err := b.Handle(context.Background(), ev); err == nil || fired {
		t.Fatalf("first Handle: fired=%v err=%v, want false/non-nil", fired, err)
	}
	if b.AlreadyFired("retry") {
		t.Fatal("result must NOT be marked fired after a failed delivery")
	}

	// Receiver becomes healthy; the same event now delivers and marks fired.
	mu.Lock()
	fail = false
	mu.Unlock()
	fired, err := b.Handle(context.Background(), ev)
	if err != nil || !fired {
		t.Fatalf("second Handle: fired=%v err=%v, want true/nil", fired, err)
	}
	if !b.AlreadyFired("retry") {
		t.Error("result should be fired after successful redelivery")
	}
}

// TestBridge_FullChainSSEToWebhook exercises the whole adapter: a real httptest
// SSE server streams a multi-frame Boba event stream (result_found, progress,
// then download_complete); SSEReader parses it; Bridge.Consume normalizes and
// dedups; WebhookSink signs and POSTs; a real httptest receiver independently
// recomputes the HMAC and asserts the standard envelope.
func TestBridge_FullChainSSEToWebhook(t *testing.T) {
	secret := []byte("chain-secret")
	rr, hook := newRecordingReceiver(secret)
	defer hook.Close()

	stream := "event: result_found\n" +
		`data: {"search_id":"s1","query":"ubuntu","result":{"infohash":"CHAIN","title":"Ubuntu"}}` + "\n" +
		"\n" +
		": keep-alive\n" +
		"\n" +
		"event: download_progress\n" +
		`data: {"id":"CHAIN","progress":40}` + "\n" +
		"\n" +
		"event: download_complete\n" +
		`data: {"id":"CHAIN","status":"complete","path":"/downloads/ubuntu.iso"}` + "\n" +
		"\n"

	sse := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte(stream))
	}))
	defer sse.Close()

	src := &SSEReader{BaseURL: sse.URL}
	sink := &WebhookSink{URL: hook.URL, Secret: secret}
	b := NewBridge(sink)

	if err := b.Consume(context.Background(), src); err != nil {
		t.Fatalf("Consume: %v", err)
	}

	envs, oks, requests := rr.snapshot()
	if requests != 1 {
		t.Fatalf("webhook requests = %d, want 1 (only the terminal event fires)", requests)
	}
	if !oks[0] {
		t.Error("independently-recomputed HMAC did not match")
	}
	if envs[0].JobID != "CHAIN" || envs[0].State != CompletionSuccess || envs[0].ResultRef != "/downloads/ubuntu.iso" {
		t.Errorf("envelope = %+v", envs[0])
	}
}
