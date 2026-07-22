package thready

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// This file implements the typed methods over the Thready `/v1` surface. Each
// maps to exactly one gateway route, injects auth, and decodes into the right
// typed struct. Unsafe POSTs (CreateChannel, Reprocess) carry an
// Idempotency-Key; idempotent GETs inherit do()'s transient-failure retries.

// Login exchanges credentials (plus TOTP for admin tiers) for a token pair and
// stores the returned access token on the Client so subsequent calls
// authenticate automatically. POST /v1/auth/login.
func (c *Client) Login(ctx context.Context, req LoginRequest) (*TokenPair, error) {
	var tp TokenPair
	if err := c.do(ctx, http.MethodPost, "/v1/auth/login", nil, req, &tp, ""); err != nil {
		return nil, err
	}
	if tp.AccessToken != "" {
		c.setToken(tp.AccessToken)
	}
	return &tp, nil
}

// ListChannels lists the channels registered for the caller's tenant.
// GET /v1/channels.
func (c *Client) ListChannels(ctx context.Context) ([]Channel, error) {
	var env listEnvelope[Channel]
	if err := c.do(ctx, http.MethodGet, "/v1/channels", nil, nil, &env, ""); err != nil {
		return nil, err
	}
	return env.Data, nil
}

// CreateChannel registers a channel/group to read. It is an unsafe POST, so it
// carries an Idempotency-Key (auto-generated unless WithIdempotencyKey is
// passed). POST /v1/channels.
func (c *Client) CreateChannel(ctx context.Context, in CreateChannelRequest, opts ...RequestOption) (*Channel, error) {
	rc := resolveOptions(opts)
	if rc.idempotencyKey == "" {
		rc.idempotencyKey = newIdempotencyKey()
	}
	var ch Channel
	if err := c.do(ctx, http.MethodPost, "/v1/channels", nil, in, &ch, rc.idempotencyKey); err != nil {
		return nil, err
	}
	return &ch, nil
}

// GetChannelThreads fetches the threads (root + organic reply chains) for a
// channel. GET /v1/channels/{channelID}/threads.
func (c *Client) GetChannelThreads(ctx context.Context, channelID string) ([]Thread, error) {
	path := "/v1/channels/" + url.PathEscape(channelID) + "/threads"
	var env listEnvelope[Thread]
	if err := c.do(ctx, http.MethodGet, path, nil, nil, &env, ""); err != nil {
		return nil, err
	}
	return env.Data, nil
}

// GetPost fetches a single post by id. GET /v1/posts/{postID}.
func (c *Client) GetPost(ctx context.Context, postID string) (*Post, error) {
	path := "/v1/posts/" + url.PathEscape(postID)
	var p Post
	if err := c.do(ctx, http.MethodGet, path, nil, nil, &p, ""); err != nil {
		return nil, err
	}
	return &p, nil
}

// Reprocess forces a fresh processing run for a post and returns the queued job
// (202 Accepted). It is an unsafe POST and carries an Idempotency-Key
// (auto-generated unless WithIdempotencyKey is passed).
// POST /v1/posts/{postID}/reprocess.
func (c *Client) Reprocess(ctx context.Context, postID string, opts ...RequestOption) (*Job, error) {
	rc := resolveOptions(opts)
	if rc.idempotencyKey == "" {
		rc.idempotencyKey = newIdempotencyKey()
	}
	path := "/v1/posts/" + url.PathEscape(postID) + "/reprocess"
	var job Job
	if err := c.do(ctx, http.MethodPost, path, nil, nil, &job, rc.idempotencyKey); err != nil {
		return nil, err
	}
	return &job, nil
}

// Search runs a semantic / keyword / hybrid search over posts and generated
// materials. POST /v1/search.
func (c *Client) Search(ctx context.Context, req SearchRequest) (*SearchResults, error) {
	var res SearchResults
	if err := c.do(ctx, http.MethodPost, "/v1/search", nil, req, &res, ""); err != nil {
		return nil, err
	}
	return &res, nil
}

// ListSkills lists the Skill-Graph knowledge units. GET /v1/skills.
func (c *Client) ListSkills(ctx context.Context) ([]Skill, error) {
	var env listEnvelope[Skill]
	if err := c.do(ctx, http.MethodGet, "/v1/skills", nil, nil, &env, ""); err != nil {
		return nil, err
	}
	return env.Data, nil
}

// SubscribeEvents opens the Server-Sent-Events stream and returns a channel of
// decoded Events. GET /v1/events.
//
// The returned channel is closed when the stream ends, the server disconnects,
// or ctx is cancelled — so cancelling ctx is how a caller unsubscribes. A
// non-2xx handshake returns a typed *APIError and no channel. The stream is not
// subject to the Client's unary Timeout; its lifetime is bounded by ctx.
func (c *Client) SubscribeEvents(ctx context.Context) (<-chan Event, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/v1/events", nil)
	if err != nil {
		return nil, fmt.Errorf("thready: build events request: %w", err)
	}
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Cache-Control", "no-cache")
	if err := c.applyAuth(req); err != nil {
		return nil, err
	}

	resp, err := c.streamClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("thready: GET /v1/events: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, parseAPIError(resp)
	}

	out := make(chan Event)
	go readSSE(ctx, resp.Body, out)
	return out, nil
}

// streamClient returns a client sharing this Client's transport but WITHOUT a
// request Timeout, so the long-lived SSE connection is bounded only by the
// caller's context.
func (c *Client) streamClient() *http.Client {
	return &http.Client{
		Transport:     c.httpClient.Transport,
		CheckRedirect: c.httpClient.CheckRedirect,
		Jar:           c.httpClient.Jar,
	}
}

// readSSE parses the SSE wire format from body, decoding each event's `data:`
// payload into an Event and delivering it on out. It closes out and body on
// exit. Comment lines (`: …`, e.g. the gateway's `: subscribed` heartbeat) and
// non-data fields are ignored; a blank line dispatches the accumulated event.
func readSSE(ctx context.Context, body io.ReadCloser, out chan<- Event) {
	defer close(out)
	defer body.Close()

	reader := bufio.NewReader(body)
	var data bytes.Buffer

	for {
		line, err := reader.ReadString('\n')
		if len(line) > 0 {
			field := strings.TrimRight(line, "\r\n")
			switch {
			case field == "":
				// End of an event: dispatch the accumulated data payload.
				if data.Len() > 0 {
					var ev Event
					if json.Unmarshal(data.Bytes(), &ev) == nil {
						select {
						case out <- ev:
						case <-ctx.Done():
							return
						}
					}
					data.Reset()
				}
			case strings.HasPrefix(field, ":"):
				// Comment / heartbeat line — ignore.
			default:
				name, value, _ := strings.Cut(field, ":")
				value = strings.TrimPrefix(value, " ")
				if name == "data" {
					if data.Len() > 0 {
						data.WriteByte('\n')
					}
					data.WriteString(value)
				}
			}
		}
		if err != nil {
			return // EOF or transport error (incl. ctx cancellation)
		}
		select {
		case <-ctx.Done():
			return
		default:
		}
	}
}
