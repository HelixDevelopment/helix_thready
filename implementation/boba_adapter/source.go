package bobaadapter

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// DefaultHooksPath is Boba-Base's hook-registration endpoint. Boba pushes
// events to a URL registered here. VERIFIED to exist; the path is Boba's
// documented POST /api/v1/hooks.
const DefaultHooksPath = "/api/v1/hooks"

// DefaultSSEPath is the Boba-Base SSE endpoint that streams "result_found" (and
// download) events. VERIFIED to exist on Boba's search surface; the concrete
// path is [inferred] and overridable via SSEReader.Path.
const DefaultSSEPath = "/api/v1/search"

// EventHandler consumes one normalized Boba event. Returning a non-nil error
// stops the stream and is propagated by Stream.
type EventHandler func(BobaEvent) error

// EventSource streams normalized Boba events until the context is cancelled, the
// underlying stream ends, or a handler returns an error. It is the seam the
// Bridge consumes; tests substitute a canned in-memory source or a real
// SSEReader pointed at an httptest server.
type EventSource interface {
	Stream(ctx context.Context, handle EventHandler) error
}

// SSEReader is a real EventSource that connects to a Boba SSE endpoint over HTTP
// (GET), reads the "text/event-stream" body, splits it into frames on blank
// lines, and parses each frame with ParseSSE. Comment / keep-alive frames (no
// data: line) are skipped. It implements EventSource.
type SSEReader struct {
	// BaseURL is the Boba origin, e.g. "http://boba:8080".
	BaseURL string
	// Path overrides the SSE endpoint; empty uses DefaultSSEPath.
	Path string
	// Query is an optional raw query string (without a leading "?") appended to
	// the SSE URL, e.g. "q=ubuntu". [inferred]
	Query string
	// Client overrides the HTTP client; nil uses http.DefaultClient.
	Client *http.Client
	// OnError, if set, is called for a frame that fails to parse (malformed
	// JSON); the stream continues. nil silently skips such frames.
	OnError func(error)
}

func (r *SSEReader) client() *http.Client {
	if r.Client != nil {
		return r.Client
	}
	return http.DefaultClient
}

func (r *SSEReader) url() string {
	path := r.Path
	if path == "" {
		path = DefaultSSEPath
	}
	u := strings.TrimRight(r.BaseURL, "/") + path
	if r.Query != "" {
		u += "?" + r.Query
	}
	return u
}

// Stream opens the SSE connection and delivers each parsed event to handle. It
// returns when the server closes the stream (nil), the context is cancelled
// (ctx.Err()), or handle returns an error (that error).
func (r *SSEReader) Stream(ctx context.Context, handle EventHandler) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, r.url(), nil)
	if err != nil {
		return fmt.Errorf("bobaadapter: build SSE request: %w", err)
	}
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Cache-Control", "no-cache")

	resp, err := r.client().Do(req)
	if err != nil {
		return fmt.Errorf("bobaadapter: open SSE stream %s: %w", r.url(), err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		_, _ = io.Copy(io.Discard, resp.Body)
		return fmt.Errorf("bobaadapter: SSE %s returned HTTP %d", r.url(), resp.StatusCode)
	}

	sc := bufio.NewScanner(resp.Body)
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)

	var frame []byte
	flush := func() error {
		if len(frame) == 0 {
			return nil
		}
		ev, perr := ParseSSE(frame)
		frame = frame[:0]
		if perr != nil {
			if errors.Is(perr, errNoData) {
				return nil // comment / keep-alive frame
			}
			if r.OnError != nil {
				r.OnError(perr)
			}
			return nil // tolerate a single malformed frame
		}
		return handle(ev)
	}

	for sc.Scan() {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		line := sc.Bytes()
		if len(line) == 0 { // blank line == frame boundary
			if err := flush(); err != nil {
				return err
			}
			continue
		}
		frame = append(frame, line...)
		frame = append(frame, '\n')
	}
	if err := sc.Err(); err != nil {
		return fmt.Errorf("bobaadapter: read SSE stream: %w", err)
	}
	// Deliver a trailing frame not terminated by a blank line.
	return flush()
}

// HookRegistration is the result of registering a downstream callback URL with
// Boba. ID is Boba's identifier for the created hook, when it returns one. [inferred]
type HookRegistration struct {
	ID string
}

// HTTPHookRegistrar registers a callback URL with a live Boba instance via
// POST {BaseURL}/api/v1/hooks, so Boba will push hook payloads (which the
// downstream endpoint feeds through ParseHookPayload) to that URL. Boba's hook
// verb VERIFIED; the request/response JSON field names are [inferred] and
// lenient.
type HTTPHookRegistrar struct {
	// BaseURL is the Boba origin, e.g. "http://boba:8080".
	BaseURL string
	// Path overrides the hooks endpoint; empty uses DefaultHooksPath.
	Path string
	// Client overrides the HTTP client; nil uses http.DefaultClient.
	Client *http.Client
}

// hookRegRequest is the [inferred] POST body registering a callback URL.
type hookRegRequest struct {
	URL    string   `json:"url"`
	Events []string `json:"events,omitempty"`
}

// hookRegResponse is the [inferred] lenient response shape carrying the created
// hook id under any of several accepted keys.
type hookRegResponse struct {
	ID     string `json:"id"`
	HookID string `json:"hook_id"`
}

// Register subscribes callbackURL to Boba's hook stream. events optionally
// filters which Boba event kinds to receive (empty = all). It returns the
// created hook's registration on a 2xx response.
func (h *HTTPHookRegistrar) Register(ctx context.Context, callbackURL string, events ...string) (HookRegistration, error) {
	path := h.Path
	if path == "" {
		path = DefaultHooksPath
	}
	url := strings.TrimRight(h.BaseURL, "/") + path

	body, err := json.Marshal(hookRegRequest{URL: callbackURL, Events: events})
	if err != nil {
		return HookRegistration{}, fmt.Errorf("bobaadapter: marshal hook registration: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return HookRegistration{}, fmt.Errorf("bobaadapter: build hook request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	client := h.Client
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return HookRegistration{}, fmt.Errorf("bobaadapter: POST %s: %w", url, err)
	}
	defer func() { _ = resp.Body.Close() }()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return HookRegistration{}, fmt.Errorf("bobaadapter: read hook response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return HookRegistration{}, fmt.Errorf("bobaadapter: Boba %s returned HTTP %d", url, resp.StatusCode)
	}

	var out hookRegResponse
	if len(bytes.TrimSpace(raw)) > 0 {
		// A body is optional; a 2xx with no/other JSON still counts as success.
		_ = json.Unmarshal(raw, &out)
	}
	return HookRegistration{ID: firstNonEmpty(out.ID, out.HookID)}, nil
}
