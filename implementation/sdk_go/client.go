// Package thready is the typed Go SDK client for the Helix Thready REST `/v1`
// control API (docs/public/research/mvp/api/openapi.yaml; realized by the
// implementation/rest_gateway module). It is stdlib-only and self-contained:
// it imports no sibling implementation modules and talks to the gateway purely
// over HTTP, so it can be vendored on its own.
//
// A Client injects auth (a JWT bearer access token OR an X-API-Key), encodes
// and decodes JSON, maps every non-2xx response to a typed *APIError, retries
// idempotent GETs on transient 503/429 with capped exponential backoff, and
// stamps a fresh Idempotency-Key onto unsafe POSTs. SubscribeEvents reads the
// Server-Sent-Events stream and decodes each frame into an Event.
package thready

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// Default tuning for a freshly constructed Client.
const (
	defaultTimeout     = 30 * time.Second
	defaultMaxRetries  = 3
	defaultBackoffBase = 25 * time.Millisecond
	defaultBackoffMax  = 2 * time.Second
)

// Config configures a Client. Exactly one of AccessToken or APIKey is normally
// set; if both are present the bearer AccessToken wins. A successful Login
// updates the in-flight AccessToken so later calls authenticate automatically.
type Config struct {
	// BaseURL is the gateway origin, with or without a trailing slash, e.g.
	// "https://thready.hxd3v.com/v1" or "http://127.0.0.1:8080". Required.
	BaseURL string
	// AccessToken is a JWT bearer access token (sent as "Authorization: Bearer …").
	AccessToken string
	// APIKey is a scoped API key (sent as "X-API-Key: …") for non-interactive use.
	APIKey string
	// HTTPClient overrides the transport. When nil a client with Timeout is used.
	HTTPClient *http.Client
	// Timeout bounds each unary request when HTTPClient is nil (default 30s). It
	// is deliberately NOT applied to the long-lived SubscribeEvents stream, which
	// is bounded by the caller's context instead.
	Timeout time.Duration
	// AllowInsecureHTTP, when true, permits the SDK to attach the credential
	// (bearer token or API key) even to a plaintext http request bound for a
	// NON-loopback host. It defaults to false: with a remote http BaseURL the
	// SDK refuses to send credentials and returns ErrInsecureTransport, rather
	// than leaking them to on-path observers. Loopback-http and https are always
	// allowed regardless of this flag. Enable it only on a trusted network.
	AllowInsecureHTTP bool
}

// Client is a typed, concurrency-safe client for the Thready `/v1` API.
type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client

	maxRetries  int
	backoffBase time.Duration
	backoffMax  time.Duration

	allowInsecureHTTP bool

	mu          sync.RWMutex
	accessToken string
}

// New builds a Client from cfg. It returns an error if BaseURL is empty.
func New(cfg Config) (*Client, error) {
	base := strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	if base == "" {
		return nil, fmt.Errorf("thready: Config.BaseURL is required")
	}
	hc := cfg.HTTPClient
	if hc == nil {
		timeout := cfg.Timeout
		if timeout <= 0 {
			timeout = defaultTimeout
		}
		hc = &http.Client{Timeout: timeout}
	}
	return &Client{
		baseURL:           base,
		apiKey:            cfg.APIKey,
		httpClient:        hc,
		maxRetries:        defaultMaxRetries,
		backoffBase:       defaultBackoffBase,
		backoffMax:        defaultBackoffMax,
		allowInsecureHTTP: cfg.AllowInsecureHTTP,
		accessToken:       cfg.AccessToken,
	}, nil
}

// AccessToken returns the token the Client currently authenticates with (set at
// construction or refreshed by Login).
func (c *Client) AccessToken() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.accessToken
}

// setToken atomically replaces the bearer access token.
func (c *Client) setToken(tok string) {
	c.mu.Lock()
	c.accessToken = tok
	c.mu.Unlock()
}

// applyAuth injects the credential: a bearer JWT when present, otherwise an
// X-API-Key. Login (a public endpoint) works with neither set.
//
// When a credential IS present, it first enforces the transport policy: the SDK
// refuses to attach the header to a plaintext-http request bound for a
// non-loopback host (returning ErrInsecureTransport and setting no header),
// unless Config.AllowInsecureHTTP was set. This prevents leaking a bearer token
// or API key in the clear. https and loopback-http are always allowed.
func (c *Client) applyAuth(req *http.Request) error {
	tok := c.AccessToken()
	hasCredential := tok != "" || c.apiKey != ""
	if hasCredential && !c.transportAllowed(req.URL) {
		return ErrInsecureTransport
	}
	if tok != "" {
		req.Header.Set("Authorization", "Bearer "+tok)
		return nil
	}
	if c.apiKey != "" {
		req.Header.Set("X-API-Key", c.apiKey)
	}
	return nil
}

// transportAllowed reports whether it is safe to attach a credential to a
// request bound for u. https (or any non-http scheme) is always fine; plaintext
// http is allowed only to a loopback host — or unconditionally when
// AllowInsecureHTTP was explicitly opted into.
func (c *Client) transportAllowed(u *url.URL) bool {
	if c.allowInsecureHTTP {
		return true
	}
	if u == nil || u.Scheme != "http" {
		return true // https and other non-plaintext schemes are safe
	}
	return isLoopbackHost(u.Hostname())
}

// isLoopbackHost reports whether host refers to the local machine: the literal
// "localhost", or any loopback IP (127.0.0.0/8, ::1). url.URL.Hostname() has
// already stripped the port and any IPv6 brackets.
func isLoopbackHost(host string) bool {
	if host == "localhost" {
		return true
	}
	if ip := net.ParseIP(host); ip != nil {
		return ip.IsLoopback()
	}
	return false
}

// RequestOption customises a single call.
type RequestOption func(*requestConfig)

type requestConfig struct {
	idempotencyKey string
}

// WithIdempotencyKey overrides the auto-generated Idempotency-Key on an unsafe
// POST. Reusing a key with the same body replays the original result; reusing it
// with a different body is a 409 conflict.
func WithIdempotencyKey(key string) RequestOption {
	return func(rc *requestConfig) { rc.idempotencyKey = key }
}

func resolveOptions(opts []RequestOption) requestConfig {
	var rc requestConfig
	for _, o := range opts {
		o(&rc)
	}
	return rc
}

// do performs a request with JSON encode/decode, auth injection, an optional
// Idempotency-Key, typed error mapping, and — for idempotent GETs — capped
// exponential-backoff retries on 503/429 and transient transport errors.
//
// body, when non-nil, is JSON-encoded as the request payload. out, when
// non-nil, receives the decoded 2xx response body.
func (c *Client) do(ctx context.Context, method, path string, query url.Values, body, out any, idempotencyKey string) error {
	var bodyBytes []byte
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("thready: encode request body: %w", err)
		}
		bodyBytes = b
	}

	fullURL := c.baseURL + path
	if len(query) > 0 {
		fullURL += "?" + query.Encode()
	}

	attempts := 1
	if method == http.MethodGet {
		attempts = c.maxRetries + 1
	}

	var lastErr error
	for attempt := 0; attempt < attempts; attempt++ {
		if attempt > 0 {
			if err := c.backoff(ctx, attempt); err != nil {
				return err
			}
		}

		var reqBody io.Reader
		if bodyBytes != nil {
			reqBody = bytes.NewReader(bodyBytes)
		}
		req, err := http.NewRequestWithContext(ctx, method, fullURL, reqBody)
		if err != nil {
			return fmt.Errorf("thready: build request: %w", err)
		}
		req.Header.Set("Accept", "application/json")
		if bodyBytes != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		if idempotencyKey != "" {
			req.Header.Set("Idempotency-Key", idempotencyKey)
		}
		// Enforce the credential-transport policy BEFORE any send: a refusal
		// here means no header was attached and no request left the process.
		if err := c.applyAuth(req); err != nil {
			return err
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("thready: %s %s: %w", method, path, err)
			if method == http.MethodGet && attempt < attempts-1 {
				continue // transient transport error on an idempotent GET
			}
			return lastErr
		}

		// Retry idempotent GETs on transient upstream unavailability.
		if method == http.MethodGet && attempt < attempts-1 &&
			(resp.StatusCode == http.StatusServiceUnavailable || resp.StatusCode == http.StatusTooManyRequests) {
			lastErr = parseAPIError(resp) // resp.Body is drained+closed by parseAPIError
			continue
		}

		return decodeResponse(resp, out)
	}
	return lastErr
}

// backoff sleeps before retry attempt (1-based) using capped exponential
// backoff, returning early if ctx is cancelled.
func (c *Client) backoff(ctx context.Context, attempt int) error {
	d := c.backoffBase << (attempt - 1)
	if d > c.backoffMax || d <= 0 {
		d = c.backoffMax
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// decodeResponse renders a response: a 2xx decodes into out (skipped for 204 or
// a nil out); anything else becomes a typed *APIError.
func decodeResponse(resp *http.Response, out any) error {
	defer resp.Body.Close()
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		if out == nil || resp.StatusCode == http.StatusNoContent {
			_, _ = io.Copy(io.Discard, resp.Body)
			return nil
		}
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			return fmt.Errorf("thready: decode response body: %w", err)
		}
		return nil
	}
	return parseAPIError(resp)
}

// parseAPIError reads a non-2xx body and maps it to a typed *APIError. It
// consumes and closes resp.Body. It prefers the canonical
// {"error":{code,message,status,request_id,…}} envelope, backfilling any missing
// status/request_id from the HTTP status line and headers, and degrades
// gracefully to a status-derived error for a non-envelope body.
func parseAPIError(resp *http.Response) *APIError {
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()

	var env errorEnvelope
	if err := json.Unmarshal(body, &env); err == nil && env.Error.Code != "" {
		ae := env.Error
		if ae.Status == 0 {
			ae.Status = resp.StatusCode
		}
		if ae.RequestID == "" {
			ae.RequestID = resp.Header.Get("X-Request-Id")
		}
		return &ae
	}

	msg := strings.TrimSpace(string(body))
	if msg == "" {
		msg = http.StatusText(resp.StatusCode)
	}
	return &APIError{
		Code:      codeForStatus(resp.StatusCode),
		Message:   msg,
		Status:    resp.StatusCode,
		RequestID: resp.Header.Get("X-Request-Id"),
	}
}

// codeForStatus maps an HTTP status to the canonical code for the fallback path
// (a non-envelope error body). The gateway itself always sends the envelope.
func codeForStatus(status int) Code {
	switch status {
	case http.StatusBadRequest:
		return CodeInvalidArgument
	case http.StatusUnprocessableEntity:
		return CodeUnprocessable
	case http.StatusUnauthorized:
		return CodeUnauthenticated
	case http.StatusForbidden:
		return CodePermissionDenied
	case http.StatusNotFound:
		return CodeNotFound
	case http.StatusConflict:
		return CodeConflict
	case http.StatusPreconditionFailed:
		return CodeFailedPrecond
	case http.StatusTooManyRequests:
		return CodeRateLimited
	case http.StatusGatewayTimeout:
		return CodeDeadlineExceeded
	case http.StatusServiceUnavailable:
		return CodeUnavailable
	default:
		return CodeInternal
	}
}

// newIdempotencyKey mints a UUIDv4 for an unsafe POST.
func newIdempotencyKey() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("idem-%d", time.Now().UnixNano())
	}
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant 10
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
