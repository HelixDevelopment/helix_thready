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
	"strings"
	"sync"
	"testing"
	"time"
)

func TestSignVerifyRoundTrip(t *testing.T) {
	secret := []byte("s3cr3t")
	body := []byte(`{"job_id":"j1"}`)

	val := SignatureValue(secret, body)
	if !strings.HasPrefix(val, "sha256=") {
		t.Fatalf("signature value missing prefix: %q", val)
	}
	if !Verify(secret, body, val) {
		t.Error("Verify failed on freshly signed body")
	}
	if !Verify(secret, body, strings.TrimPrefix(val, "sha256=")) {
		t.Error("Verify should accept a bare hex digest too")
	}
	if Verify([]byte("wrong"), body, val) {
		t.Error("Verify must fail under the wrong secret")
	}
	if Verify(secret, []byte("tampered"), val) {
		t.Error("Verify must fail when the body is tampered")
	}
}

// independentHMAC recomputes the signature the way an external receiver would,
// with no reference to the package's Sign helper.
func independentHMAC(secret, body []byte) string {
	m := hmac.New(sha256.New, secret)
	m.Write(body)
	return "sha256=" + hex.EncodeToString(m.Sum(nil))
}

func TestWebhookSink_SignsExactBodyReceiverRecomputes(t *testing.T) {
	secret := []byte("hook-key")

	var (
		mu      sync.Mutex
		gotEnv  Envelope
		gotSig  string
		matched bool
		hits    int
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		mu.Lock()
		hits++
		gotSig = r.Header.Get(SignatureHeader)
		matched = hmac.Equal([]byte(gotSig), []byte(independentHMAC(secret, raw)))
		_ = json.Unmarshal(raw, &gotEnv)
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	sink := &WebhookSink{URL: srv.URL, Secret: secret}
	env := Envelope{
		JobID:     "j-42",
		State:     CompletionSuccess,
		Progress:  1.0,
		ResultRef: "/out/j-42.mp4",
		TS:        time.Unix(1_700_000_000, 0).UTC(),
	}
	if err := sink.Notify(context.Background(), env); err != nil {
		t.Fatalf("Notify: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if hits != 1 {
		t.Fatalf("receiver hits = %d, want 1", hits)
	}
	if !matched {
		t.Errorf("receiver-recomputed HMAC did not match sent signature %q", gotSig)
	}
	if gotEnv.JobID != "j-42" || gotEnv.State != CompletionSuccess || gotEnv.ResultRef != "/out/j-42.mp4" {
		t.Errorf("received envelope = %+v", gotEnv)
	}
}

func TestWebhookSink_RetryOn500Then200(t *testing.T) {
	var (
		mu       sync.Mutex
		attempts int
	)
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
		Secret:     []byte("k"),
		MaxRetries: 3,
		Backoff:    func(int) time.Duration { return time.Millisecond },
	}
	if err := sink.Notify(context.Background(), Envelope{JobID: "r1"}); err != nil {
		t.Fatalf("Notify should have succeeded after retry: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if attempts != 2 {
		t.Fatalf("attempts = %d, want 2 (500 then 200)", attempts)
	}
}

func TestWebhookSink_FailsAfterExhaustingRetries(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer srv.Close()

	sink := &WebhookSink{
		URL:        srv.URL,
		Secret:     []byte("k"),
		MaxRetries: 2,
		Backoff:    func(int) time.Duration { return time.Millisecond },
	}
	if err := sink.Notify(context.Background(), Envelope{JobID: "x"}); err == nil {
		t.Fatal("expected delivery failure after exhausting retries")
	}
}
