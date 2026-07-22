package bobaadapter

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

// recordingReceiver is an httptest webhook sink that INDEPENDENTLY recomputes
// the HMAC over the exact received body (crypto/hmac + crypto/sha256, with no
// reference to the package's own Sign) and records each decoded Envelope. It is
// the shared assertion harness for the webhook and bridge tests.
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

func TestSignVerifyRoundTrip(t *testing.T) {
	secret := []byte("shared-secret")
	body := []byte(`{"job_id":"x","state":"success"}`)

	sig := SignatureValue(secret, body)
	if !Verify(secret, body, sig) {
		t.Error("Verify rejected a signature it should accept")
	}
	// Bare hex (no prefix) must also verify.
	if !Verify(secret, body, Sign(secret, body)) {
		t.Error("Verify rejected the bare hex digest")
	}
	// Wrong secret must fail.
	if Verify([]byte("other"), body, sig) {
		t.Error("Verify accepted a signature under the wrong secret")
	}
	// Tampered body must fail.
	if Verify(secret, []byte(`{"job_id":"y"}`), sig) {
		t.Error("Verify accepted a signature over a tampered body")
	}
}

func TestEnvelopeFor_ResultRefFallsBackMagnetThenTorrent(t *testing.T) {
	ts := time.Unix(1_700_000_000, 0).UTC()

	cases := []struct {
		name string
		ev   BobaEvent
		want string
	}{
		{
			name: "path wins when present",
			ev: BobaEvent{
				Type: EventDownloadComplete, ResultID: "H",
				Path: "/downloads/ubuntu.iso", Magnet: "magnet:?xt=urn:btih:H", Torrent: "http://boba/H.torrent",
			},
			want: "/downloads/ubuntu.iso",
		},
		{
			name: "falls back to magnet when path is empty",
			ev: BobaEvent{
				Type: EventDownloadComplete, ResultID: "H",
				Magnet: "magnet:?xt=urn:btih:H", Torrent: "http://boba/H.torrent",
			},
			want: "magnet:?xt=urn:btih:H",
		},
		{
			name: "falls back to torrent when path and magnet are empty",
			ev: BobaEvent{
				Type: EventDownloadComplete, ResultID: "H",
				Torrent: "http://boba/H.torrent",
			},
			want: "http://boba/H.torrent",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			env := EnvelopeFor(tc.ev, ts)
			if env.State != CompletionSuccess {
				t.Fatalf("state = %q, want %q", env.State, CompletionSuccess)
			}
			if env.ResultRef != tc.want {
				t.Errorf("result_ref = %q, want %q", env.ResultRef, tc.want)
			}
		})
	}
}

func TestWebhookSink_SignsExactBodyReceiverRecomputes(t *testing.T) {
	secret := []byte("sink-secret")
	rr, srv := newRecordingReceiver(secret)
	defer srv.Close()

	sink := &WebhookSink{URL: srv.URL, Secret: secret}
	env := Envelope{
		JobID:     "HASH1",
		State:     CompletionSuccess,
		Progress:  1.0,
		ResultRef: "/downloads/ubuntu.iso",
		TS:        time.Unix(1_700_000_000, 0).UTC(),
	}
	if err := sink.Notify(context.Background(), env); err != nil {
		t.Fatalf("Notify: %v", err)
	}

	envs, oks, requests := rr.snapshot()
	if requests != 1 {
		t.Fatalf("requests = %d, want 1", requests)
	}
	if !oks[0] {
		t.Error("receiver-recomputed HMAC did not match the sent signature")
	}
	if envs[0] != env {
		t.Errorf("received envelope = %+v, want %+v", envs[0], env)
	}
}

func TestWebhookSink_RetryOn500Then200(t *testing.T) {
	secret := []byte("retry-secret")
	var mu sync.Mutex
	var attempts int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		attempts++
		n := attempts
		mu.Unlock()
		if n == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	sink := &WebhookSink{
		URL:        srv.URL,
		Secret:     secret,
		MaxRetries: 3,
		Backoff:    func(int) time.Duration { return time.Millisecond },
	}
	if err := sink.Notify(context.Background(), Envelope{JobID: "r"}); err != nil {
		t.Fatalf("Notify: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if attempts != 2 {
		t.Fatalf("attempts = %d, want 2 (500 then 200)", attempts)
	}
}

func TestWebhookSink_FailsAfterExhaustingRetries(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	sink := &WebhookSink{
		URL:        srv.URL,
		Secret:     []byte("k"),
		MaxRetries: 2,
		Backoff:    func(int) time.Duration { return time.Millisecond },
	}
	if err := sink.Notify(context.Background(), Envelope{JobID: "z"}); err == nil {
		t.Fatal("expected an error after exhausting all retries")
	}
}
