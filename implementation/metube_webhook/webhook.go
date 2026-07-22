package metubewebhook

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// SignatureHeader carries the HMAC-SHA256 signature of the raw request body,
// matching the event-bus contract §9 ("X-Thready-Signature").
const SignatureHeader = "X-Thready-Signature"

// signaturePrefix is prepended to the hex digest ("sha256=<hex>").
const signaturePrefix = "sha256="

// CompletionState is the terminal state carried by an outbound Envelope.
type CompletionState string

const (
	// CompletionSuccess maps from a MeTube "finished" job.
	CompletionSuccess CompletionState = "success"
	// CompletionFailure maps from a MeTube "error" job.
	CompletionFailure CompletionState = "failure"
)

// Envelope is the stable outbound completion payload, matching the canonical
// callback_task shape {job_id, state, progress, result_ref, error, ts}. Field
// presence is stable (no omitempty) so the JSON shape — and therefore the HMAC
// signature computed over it — is deterministic.
type Envelope struct {
	JobID     string          `json:"job_id"`
	State     CompletionState `json:"state"`
	Progress  float64         `json:"progress"`
	ResultRef string          `json:"result_ref"`
	Error     string          `json:"error"`
	TS        time.Time       `json:"ts"`
}

// EnvelopeFor builds the completion Envelope for a terminal job, stamped with
// ts. A finished job yields success (result_ref = output path, progress 1.0); an
// error job yields failure (error = message).
func EnvelopeFor(j JobStatus, ts time.Time) Envelope {
	env := Envelope{
		JobID:    j.ID,
		Progress: j.Progress,
		TS:       ts,
	}
	if j.State == StateError {
		env.State = CompletionFailure
		env.Error = j.Error
	} else {
		env.State = CompletionSuccess
		env.ResultRef = j.ResultPath
	}
	return env
}

// Sign returns the lowercase hex HMAC-SHA256 digest of body under secret.
func Sign(secret, body []byte) string {
	mac := hmac.New(sha256.New, secret)
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}

// SignatureValue returns the full header value ("sha256=<hex>") for body.
func SignatureValue(secret, body []byte) string {
	return signaturePrefix + Sign(secret, body)
}

// Verify recomputes the HMAC over body and constant-time compares it against the
// SignatureHeader value (with or without the "sha256=" prefix).
func Verify(secret, body []byte, headerValue string) bool {
	want := Sign(secret, body)
	got := strings.TrimPrefix(headerValue, signaturePrefix)
	return hmac.Equal([]byte(want), []byte(got))
}

// Notifier delivers a completion Envelope. The Poller depends on this seam so it
// can be exercised with an in-memory notifier or a real HTTP sink.
type Notifier interface {
	Notify(ctx context.Context, env Envelope) error
}

// WebhookSink delivers Envelopes to a sink URL as an HMAC-signed HTTP POST,
// retrying on transport errors and non-2xx responses with back-off. It
// implements Notifier.
type WebhookSink struct {
	// URL is the destination endpoint (required).
	URL string
	// Secret is the HMAC key (required).
	Secret []byte
	// Client overrides the HTTP client; nil uses http.DefaultClient.
	Client *http.Client
	// MaxRetries is the number of retries after the first attempt. Total
	// attempts = MaxRetries + 1.
	MaxRetries int
	// Backoff maps a 1-based retry index to a delay before that retry. nil uses
	// a default exponential schedule.
	Backoff func(retry int) time.Duration
}

// Notify marshals env and delivers it. It satisfies Notifier.
func (w *WebhookSink) Notify(ctx context.Context, env Envelope) error {
	body, err := json.Marshal(env)
	if err != nil {
		return fmt.Errorf("metubewebhook: marshal envelope: %w", err)
	}
	return w.deliver(ctx, body)
}

func (w *WebhookSink) client() *http.Client {
	if w.Client != nil {
		return w.Client
	}
	return http.DefaultClient
}

func (w *WebhookSink) backoff(retry int) time.Duration {
	if w.Backoff != nil {
		return w.Backoff(retry)
	}
	if retry < 1 {
		retry = 1
	}
	return 100 * time.Millisecond * (1 << (retry - 1))
}

// deliver POSTs body with the signature header, retrying on non-2xx or transport
// error. The signature is computed over the exact bytes sent so the receiver can
// recompute and match it.
func (w *WebhookSink) deliver(ctx context.Context, body []byte) error {
	sig := SignatureValue(w.Secret, body)
	var lastErr error

	for attempt := 0; attempt <= w.MaxRetries; attempt++ {
		if attempt > 0 {
			select {
			case <-time.After(w.backoff(attempt)):
			case <-ctx.Done():
				return ctx.Err()
			}
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, w.URL, bytes.NewReader(body))
		if err != nil {
			return fmt.Errorf("metubewebhook: build webhook request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set(SignatureHeader, sig)

		resp, err := w.client().Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return nil
		}
		lastErr = fmt.Errorf("metubewebhook: webhook %s returned HTTP %d", w.URL, resp.StatusCode)
	}

	return fmt.Errorf("metubewebhook: delivery failed after %d attempt(s): %w", w.MaxRetries+1, lastErr)
}
