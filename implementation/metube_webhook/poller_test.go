package metubewebhook

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

// seqSource is a canned StatusSource that returns one snapshot per Poll,
// repeating the final snapshot once the scripted sequence is exhausted (so
// re-poll / dedup behavior can be exercised).
type seqSource struct {
	mu   sync.Mutex
	seq  [][]JobStatus
	i    int
	last []JobStatus
}

func (s *seqSource) Jobs(ctx context.Context) ([]JobStatus, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.i < len(s.seq) {
		s.last = s.seq[s.i]
		s.i++
	}
	return s.last, nil
}

// recordingReceiver is an httptest webhook sink that independently recomputes
// the HMAC over the exact received body and records each decoded envelope.
type recordingReceiver struct {
	mu       sync.Mutex
	secret   []byte
	envs     []Envelope
	sigOK    []bool
	requests int
}

func newRecordingReceiver(secret []byte) (*recordingReceiver, *httptest.Server) {
	rr := &recordingReceiver{secret: secret}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)

		m := hmac.New(sha256.New, secret)
		m.Write(raw)
		want := "sha256=" + hex.EncodeToString(m.Sum(nil))
		ok := hmac.Equal([]byte(want), []byte(r.Header.Get(SignatureHeader)))

		var env Envelope
		_ = json.Unmarshal(raw, &env)

		rr.mu.Lock()
		rr.requests++
		rr.envs = append(rr.envs, env)
		rr.sigOK = append(rr.sigOK, ok)
		rr.mu.Unlock()

		w.WriteHeader(http.StatusOK)
	}))
	return rr, srv
}

func (rr *recordingReceiver) snapshot() ([]Envelope, []bool, int) {
	rr.mu.Lock()
	defer rr.mu.Unlock()
	envs := append([]Envelope(nil), rr.envs...)
	oks := append([]bool(nil), rr.sigOK...)
	return envs, oks, rr.requests
}

func TestPoller_FiresExactlyOneSuccessWebhook(t *testing.T) {
	secret := []byte("poll-secret")
	rr, srv := newRecordingReceiver(secret)
	defer srv.Close()

	src := &seqSource{seq: [][]JobStatus{
		{{ID: "vid", State: StatePending}},
		{{ID: "vid", State: StateDownloading, Progress: 0.5}},
		{{ID: "vid", State: StateFinished, Progress: 1.0, ResultPath: "/out/vid.mp4"}},
	}}
	sink := &WebhookSink{URL: srv.URL, Secret: secret}
	p := NewPoller(src, sink)
	fixedTS := time.Unix(1_700_000_500, 0).UTC()
	p.Now = func() time.Time { return fixedTS }

	total := 0
	for poll := 0; poll < 3; poll++ {
		n, err := p.Poll(context.Background())
		if err != nil {
			t.Fatalf("poll %d: %v", poll, err)
		}
		total += n
	}

	if total != 1 {
		t.Fatalf("total webhooks fired = %d, want 1", total)
	}
	envs, oks, requests := rr.snapshot()
	if requests != 1 {
		t.Fatalf("receiver requests = %d, want 1", requests)
	}
	if !oks[0] {
		t.Error("receiver-recomputed HMAC did not match")
	}
	got := envs[0]
	if got.JobID != "vid" {
		t.Errorf("job_id = %q, want vid", got.JobID)
	}
	if got.State != CompletionSuccess {
		t.Errorf("state = %q, want success", got.State)
	}
	if got.Progress != 1.0 {
		t.Errorf("progress = %v, want 1.0", got.Progress)
	}
	if got.ResultRef != "/out/vid.mp4" {
		t.Errorf("result_ref = %q, want /out/vid.mp4", got.ResultRef)
	}
	if got.Error != "" {
		t.Errorf("error = %q, want empty", got.Error)
	}
	if !got.TS.Equal(fixedTS) {
		t.Errorf("ts = %v, want %v", got.TS, fixedTS)
	}
}

func TestPoller_ErrorJobFiresFailureWebhook(t *testing.T) {
	secret := []byte("poll-secret")
	rr, srv := newRecordingReceiver(secret)
	defer srv.Close()

	src := &seqSource{seq: [][]JobStatus{
		{{ID: "bad", State: StateDownloading, Progress: 0.3}},
		{{ID: "bad", State: StateError, Progress: 0.3, Error: "ffmpeg exited 1"}},
	}}
	sink := &WebhookSink{URL: srv.URL, Secret: secret}
	p := NewPoller(src, sink)

	total := 0
	for poll := 0; poll < 2; poll++ {
		n, err := p.Poll(context.Background())
		if err != nil {
			t.Fatalf("poll %d: %v", poll, err)
		}
		total += n
	}

	if total != 1 {
		t.Fatalf("total webhooks fired = %d, want 1", total)
	}
	envs, oks, requests := rr.snapshot()
	if requests != 1 {
		t.Fatalf("receiver requests = %d, want 1", requests)
	}
	if !oks[0] {
		t.Error("receiver-recomputed HMAC did not match")
	}
	if envs[0].State != CompletionFailure {
		t.Errorf("state = %q, want failure", envs[0].State)
	}
	if envs[0].Error != "ffmpeg exited 1" {
		t.Errorf("error = %q, want %q", envs[0].Error, "ffmpeg exited 1")
	}
	if envs[0].ResultRef != "" {
		t.Errorf("result_ref = %q, want empty for failure", envs[0].ResultRef)
	}
}

func TestPoller_DedupNoSecondFire(t *testing.T) {
	secret := []byte("poll-secret")
	rr, srv := newRecordingReceiver(secret)
	defer srv.Close()

	// Source keeps reporting the same finished job on every poll.
	src := &seqSource{seq: [][]JobStatus{
		{{ID: "once", State: StateFinished, Progress: 1.0, ResultPath: "/out/once.mp4"}},
	}}
	sink := &WebhookSink{URL: srv.URL, Secret: secret}
	p := NewPoller(src, sink)

	// Poll many times against a persistently-finished job.
	for i := 0; i < 5; i++ {
		if _, err := p.Poll(context.Background()); err != nil {
			t.Fatalf("poll %d: %v", i, err)
		}
	}

	_, _, requests := rr.snapshot()
	if requests != 1 {
		t.Fatalf("DEDUP violated: receiver got %d requests, want exactly 1", requests)
	}
	if !p.AlreadyFired("once") {
		t.Error("AlreadyFired(once) = false, want true")
	}
}

func TestPoller_MultipleJobsIndependentDedup(t *testing.T) {
	secret := []byte("poll-secret")
	rr, srv := newRecordingReceiver(secret)
	defer srv.Close()

	src := &seqSource{seq: [][]JobStatus{
		{
			{ID: "a", State: StateDownloading},
			{ID: "b", State: StateFinished, ResultPath: "/out/b.mp4"},
		},
		{
			{ID: "a", State: StateFinished, ResultPath: "/out/a.mp4"},
			{ID: "b", State: StateFinished, ResultPath: "/out/b.mp4"},
		},
	}}
	sink := &WebhookSink{URL: srv.URL, Secret: secret}
	p := NewPoller(src, sink)

	total := 0
	for poll := 0; poll < 2; poll++ {
		n, err := p.Poll(context.Background())
		if err != nil {
			t.Fatalf("poll %d: %v", poll, err)
		}
		total += n
	}

	// b fires on poll 1, a fires on poll 2 — two distinct jobs, no double-fire.
	if total != 2 {
		t.Fatalf("total fired = %d, want 2", total)
	}
	_, _, requests := rr.snapshot()
	if requests != 2 {
		t.Fatalf("receiver requests = %d, want 2", requests)
	}
}

// TestPoller_FullChainMeTubeMockToWebhook exercises the whole shim: a real
// httptest MeTube-mock server (driving a job pending -> downloading -> finished
// by request count) feeding HTTPStatusSource -> Poller -> WebhookSink -> a real
// httptest webhook receiver that recomputes the HMAC independently.
func TestPoller_FullChainMeTubeMockToWebhook(t *testing.T) {
	secret := []byte("chain-secret")
	rr, hook := newRecordingReceiver(secret)
	defer hook.Close()

	var mu sync.Mutex
	call := 0
	metube := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		call++
		n := call
		mu.Unlock()
		var status, extra string
		switch n {
		case 1:
			status = "pending"
		case 2:
			status = "downloading"
		default:
			status = "finished"
			extra = `,"filename":"/downloads/final.mkv"`
		}
		_, _ = w.Write([]byte(`{"jobs":[{"id":"chain","status":"` + status + `"` + extra + `}]}`))
	}))
	defer metube.Close()

	src := &HTTPStatusSource{BaseURL: metube.URL}
	sink := &WebhookSink{URL: hook.URL, Secret: secret}
	p := NewPoller(src, sink)

	total := 0
	for poll := 0; poll < 3; poll++ {
		n, err := p.Poll(context.Background())
		if err != nil {
			t.Fatalf("poll %d: %v", poll, err)
		}
		total += n
	}

	if total != 1 {
		t.Fatalf("total fired = %d, want 1", total)
	}
	envs, oks, requests := rr.snapshot()
	if requests != 1 {
		t.Fatalf("webhook requests = %d, want 1", requests)
	}
	if !oks[0] {
		t.Error("independently-recomputed HMAC did not match")
	}
	if envs[0].JobID != "chain" || envs[0].State != CompletionSuccess || envs[0].ResultRef != "/downloads/final.mkv" {
		t.Errorf("envelope = %+v", envs[0])
	}
}

func TestPoller_DeliveryFailureLeavesJobUnfiredForRetry(t *testing.T) {
	// Webhook receiver fails on the first delivery attempt-set, succeeds after.
	var mu sync.Mutex
	fail := true
	hook := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		f := fail
		mu.Unlock()
		if f {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer hook.Close()

	src := &seqSource{seq: [][]JobStatus{
		{{ID: "retry-job", State: StateFinished, ResultPath: "/out/r.mp4"}},
	}}
	// No retries inside the sink, so the first Poll's delivery fails outright.
	sink := &WebhookSink{URL: hook.URL, Secret: []byte("k"), MaxRetries: 0}
	p := NewPoller(src, sink)

	if _, err := p.Poll(context.Background()); err == nil {
		t.Fatal("expected delivery error on first poll")
	}
	if p.AlreadyFired("retry-job") {
		t.Fatal("job must NOT be marked fired after a failed delivery")
	}

	// Receiver now healthy; next poll should deliver and mark fired.
	mu.Lock()
	fail = false
	mu.Unlock()

	n, err := p.Poll(context.Background())
	if err != nil {
		t.Fatalf("second poll: %v", err)
	}
	if n != 1 {
		t.Fatalf("second poll fired = %d, want 1", n)
	}
	if !p.AlreadyFired("retry-job") {
		t.Error("job should be fired after successful redelivery")
	}
}
