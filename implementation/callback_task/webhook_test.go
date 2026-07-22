package callbacktask

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
	"sync/atomic"
	"testing"
	"time"
)

// recomputeSignature independently reproduces the "sha256=<hex>" header value
// the receiver expects, using only crypto/hmac + crypto/sha256 (NOT the
// module's own Sign helper), so the test is an independent verification.
func recomputeSignature(secret, body []byte) string {
	mac := hmac.New(sha256.New, secret)
	mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

func TestWebhookFiresWithValidHMAC(t *testing.T) {
	secret := []byte("per-sink-hmac-secret-key")

	var (
		mu       sync.Mutex
		verified bool
		received Envelope
	)

	// A REAL webhook receiver that recomputes the HMAC over the exact bytes it
	// received and matches it against X-Thready-Signature.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		body, err := io.ReadAll(req.Body)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		gotSig := req.Header.Get(SignatureHeader)
		wantSig := recomputeSignature(secret, body)
		if !hmac.Equal([]byte(gotSig), []byte(wantSig)) {
			w.WriteHeader(http.StatusUnauthorized) // signature mismatch -> reject
			return
		}
		var env Envelope
		if err := json.Unmarshal(body, &env); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		mu.Lock()
		verified = true
		received = env
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	sink := &WebhookSink{URL: srv.URL, Secret: secret}
	env := Envelope{
		TaskID:    "task-abc",
		State:     StateSucceeded,
		Progress:  1.0,
		ResultRef: "asset:xyz",
		TS:        time.Unix(1_700_000_000, 0).UTC(),
	}

	if err := sink.Notify(context.Background(), env); err != nil {
		t.Fatalf("Notify: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if !verified {
		t.Fatal("receiver did not verify the HMAC signature")
	}
	if received.TaskID != "task-abc" || received.State != StateSucceeded || received.ResultRef != "asset:xyz" {
		t.Fatalf("receiver decoded wrong envelope: %+v", received)
	}
}

func TestWebhookRejectedOnTamperedSecret(t *testing.T) {
	// Receiver holds the correct secret; sink signs with the WRONG one. The
	// receiver must reject, and the sink (no retries) must surface an error.
	correct := []byte("correct-secret")
	wrong := []byte("attacker-secret")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		body, _ := io.ReadAll(req.Body)
		if !hmac.Equal([]byte(req.Header.Get(SignatureHeader)), []byte(recomputeSignature(correct, body))) {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	sink := &WebhookSink{URL: srv.URL, Secret: wrong, MaxRetries: 0}
	err := sink.Notify(context.Background(), Envelope{TaskID: "t", State: StateSucceeded})
	if err == nil {
		t.Fatal("expected delivery to fail against a mismatched HMAC secret")
	}
}

func TestWebhookRetryOn500Then200(t *testing.T) {
	secret := []byte("retry-secret")

	var (
		calls   int32
		mu      sync.Mutex
		bodies  [][]byte
		allGood = true
	)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		body, _ := io.ReadAll(req.Body)
		// Verify the HMAC on EVERY attempt, including the retry.
		if !hmac.Equal([]byte(req.Header.Get(SignatureHeader)), []byte(recomputeSignature(secret, body))) {
			mu.Lock()
			allGood = false
			mu.Unlock()
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		mu.Lock()
		bodies = append(bodies, body)
		mu.Unlock()

		n := atomic.AddInt32(&calls, 1)
		if n == 1 {
			w.WriteHeader(http.StatusInternalServerError) // first attempt: 500
			return
		}
		w.WriteHeader(http.StatusOK) // retry: 200
	}))
	defer srv.Close()

	sink := &WebhookSink{
		URL:        srv.URL,
		Secret:     secret,
		MaxRetries: 3,
		Backoff:    func(int) time.Duration { return 0 }, // no real sleeping in tests
	}
	env := Envelope{TaskID: "task-retry", State: StateSucceeded, Progress: 1.0, ResultRef: "asset:r"}

	if err := sink.Notify(context.Background(), env); err != nil {
		t.Fatalf("Notify: %v", err)
	}

	if got := atomic.LoadInt32(&calls); got != 2 {
		t.Fatalf("want exactly 2 delivery attempts (500 then 200), got %d", got)
	}
	mu.Lock()
	defer mu.Unlock()
	if !allGood {
		t.Fatal("an attempt arrived with an invalid HMAC signature")
	}
	if len(bodies) != 2 {
		t.Fatalf("want 2 received bodies, got %d", len(bodies))
	}
	if string(bodies[0]) != string(bodies[1]) {
		t.Fatalf("retry body differs from first body:\n %s\n %s", bodies[0], bodies[1])
	}
}

func TestWebhookExhaustsRetries(t *testing.T) {
	secret := []byte("always-500")
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	sink := &WebhookSink{URL: srv.URL, Secret: secret, MaxRetries: 2, Backoff: func(int) time.Duration { return 0 }}
	err := sink.Notify(context.Background(), Envelope{TaskID: "t", State: StateFailed})
	if err == nil {
		t.Fatal("expected error after exhausting retries")
	}
	if !strings.Contains(err.Error(), "delivery failed") {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := atomic.LoadInt32(&calls); got != 3 { // 1 initial + 2 retries
		t.Fatalf("want 3 total attempts, got %d", got)
	}
}

// TestRegistryCapturesExhaustedDelivery proves the anti-bluff fix: when a
// WebhookSink exhausts its retries (receiver always 500), the Registry does NOT
// silently drop the returned error. The undeliverable envelope — including the
// "succeeded" completion that would otherwise be lost — is captured in the
// delivery dead-letter list AND surfaced to the delivery-error hook.
func TestRegistryCapturesExhaustedDelivery(t *testing.T) {
	ctx := context.Background()

	var attempts int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		atomic.AddInt32(&attempts, 1)
		w.WriteHeader(http.StatusInternalServerError) // ALWAYS fail
	}))
	defer srv.Close()

	sink := &WebhookSink{
		URL:        srv.URL,
		Secret:     []byte("dlq-secret"),
		MaxRetries: 2,                                    // total 3 attempts per delivery
		Backoff:    func(int) time.Duration { return 0 }, // no real sleeping in tests
	}

	var (
		hookCalls int32
		hmu       sync.Mutex
		hookLast  Envelope
	)
	r := New(
		WithNotifier(sink),
		WithDeliveryErrorHook(func(_ context.Context, env Envelope, err error) {
			atomic.AddInt32(&hookCalls, 1)
			hmu.Lock()
			hookLast = env
			hmu.Unlock()
		}),
	)

	// Drive queued -> running -> succeeded. Every transition's delivery exhausts.
	task, err := r.Submit(ctx, "download", nil)
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if _, err := r.Start(ctx, task.ID); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if _, err := r.Complete(ctx, task.ID, "asset:would-be-lost"); err != nil {
		t.Fatalf("Complete: %v", err)
	}

	// 3 transitions * (1 initial + 2 retries) = 9 HTTP attempts.
	if got := atomic.LoadInt32(&attempts); got != 9 {
		t.Fatalf("want 9 total HTTP attempts (3 transitions * 3 attempts), got %d", got)
	}

	fails := r.DeliveryFailures()
	if len(fails) != 3 {
		t.Fatalf("want 3 recorded delivery failures (queued, running, succeeded), got %d", len(fails))
	}

	// The succeeded completion — the callback that must NOT be silently lost —
	// is captured with its result_ref and a real exhaustion error.
	last := fails[len(fails)-1]
	if last.Envelope.State != StateSucceeded || last.Envelope.ResultRef != "asset:would-be-lost" {
		t.Fatalf("exhausted succeeded delivery not captured: %+v", last.Envelope)
	}
	if !strings.Contains(last.Err, "delivery failed") {
		t.Fatalf("recorded delivery error should surface the exhaustion, got %q", last.Err)
	}
	if last.At.IsZero() {
		t.Fatal("recorded delivery failure should carry a timestamp")
	}

	// The hook observed every failure, including the succeeded envelope.
	if got := atomic.LoadInt32(&hookCalls); got != 3 {
		t.Fatalf("want delivery-error hook called 3 times, got %d", got)
	}
	hmu.Lock()
	defer hmu.Unlock()
	if hookLast.State != StateSucceeded {
		t.Fatalf("hook should have observed the succeeded envelope, got %s", hookLast.State)
	}
}

// TestVerifyRoundTrip exercises Verify against both header forms and rejection
// paths, so the receiver-side helper is covered independently of the sink.
func TestVerifyRoundTrip(t *testing.T) {
	secret := []byte("verify-secret")
	body := []byte(`{"task_id":"t","state":"succeeded"}`)

	// Accepts the full "sha256=<hex>" header value.
	if !Verify(secret, body, SignatureValue(secret, body)) {
		t.Fatal("Verify should accept a valid sha256=-prefixed signature")
	}
	// Accepts a bare hex digest (no prefix).
	if !Verify(secret, body, Sign(secret, body)) {
		t.Fatal("Verify should accept a valid bare hex signature")
	}
	// Rejects a tampered body.
	if Verify(secret, append(append([]byte(nil), body...), '!'), SignatureValue(secret, body)) {
		t.Fatal("Verify must reject when the body was tampered")
	}
	// Rejects a signature made under a different secret.
	if Verify([]byte("other-secret"), body, SignatureValue(secret, body)) {
		t.Fatal("Verify must reject a signature under a different secret")
	}
}

// TestRegistryWebhookIntegration wires a WebhookSink into the Registry as its
// Notifier and drives a task to completion, asserting the receiver got a valid,
// HMAC-verified "succeeded" envelope carrying the result_ref.
func TestRegistryWebhookIntegration(t *testing.T) {
	ctx := context.Background()
	secret := []byte("integration-secret")

	var (
		mu             sync.Mutex
		succeededBody  []byte
		everyValidHMAC = true
	)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		body, _ := io.ReadAll(req.Body)
		if !hmac.Equal([]byte(req.Header.Get(SignatureHeader)), []byte(recomputeSignature(secret, body))) {
			mu.Lock()
			everyValidHMAC = false
			mu.Unlock()
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		var env Envelope
		_ = json.Unmarshal(body, &env)
		if env.State == StateSucceeded {
			mu.Lock()
			succeededBody = body
			mu.Unlock()
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	sink := &WebhookSink{URL: srv.URL, Secret: secret, Backoff: func(int) time.Duration { return 0 }}
	r := New(WithNotifier(sink))

	task, err := r.Submit(ctx, "download", []byte(`{"url":"http://example/z"}`))
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if _, err := r.Start(ctx, task.ID); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if _, err := r.Progress(ctx, task.ID, 0.5); err != nil {
		t.Fatalf("Progress: %v", err)
	}
	if _, err := r.Complete(ctx, task.ID, "asset:integration"); err != nil {
		t.Fatalf("Complete: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if !everyValidHMAC {
		t.Fatal("registry-fired webhook carried an invalid HMAC signature")
	}
	if succeededBody == nil {
		t.Fatal("receiver never got a succeeded envelope")
	}
	var got Envelope
	if err := json.Unmarshal(succeededBody, &got); err != nil {
		t.Fatalf("decode succeeded body: %v", err)
	}
	if got.TaskID != task.ID {
		t.Fatalf("succeeded envelope task_id: want %s, got %s", task.ID, got.TaskID)
	}
	if got.ResultRef != "asset:integration" {
		t.Fatalf("succeeded envelope result_ref: want asset:integration, got %q", got.ResultRef)
	}
	if got.Progress != 1.0 {
		t.Fatalf("succeeded envelope progress: want 1.0, got %v", got.Progress)
	}
}
