// -----------------------------------------------------------------------------
//  Helix Thready — Go SDK client skeleton (thin idiomatic layer)
//  Classification : PUBLIC
//  Location       : docs/public/research/mvp/development/materials/sdk/go/client.go
//  Status         : Draft — v0.1 SKELETON (illustrative)
//  Revision       : 1 (2026-07-22) — swarm (development/materials)
//  Sources        : ../../../../api/sdk-strategy.md (§5 thin layer, §3 repo layout),
//                   ../../../../api/sdk-examples.md (Go recipes R1/R2/R5/R6/R7),
//                   ../../../../api/error-model.md, ../../../../api/event-bus-contract.md
//
//  WHAT THIS IS (read before assuming anything works)
//    This file is the HAND-WRITTEN thin idiomatic layer that wraps the buf-GENERATED
//    core. It illustrates the generated-core + hand-written-wrapper pattern from
//    sdk-strategy.md §1/§5 (the mature helix_proto pattern). It shows: Config, auth
//    injection, ONE example call (Posts.List with a cursor iterator) and the events
//    subscription (Events.Subscribe).
//
//    ANTI-BLUFF: This is a SKELETON. It is NOT a compiled, tested SDK. The generated
//    core package it references (`gen/go`, produced by `buf generate` +
//    protocolbuffers/go + connectrpc/go per sdk-strategy.md §3) is NOT included here —
//    the transport/serialization bodies below are intentionally stubbed with clear
//    TODOs. Do not claim this module works; SDK publish is gated on the full test
//    suite going GREEN (sdk-strategy.md §6, [GAP: #11], [GAP: #18]).
//
//  LAYERING RULE (sdk-strategy.md §6 check-no-handwritten)
//    Ergonomics live ONLY in this thin layer. The gen/ core is regenerated from the
//    contract and MUST NOT be hand-edited — a hand edit there fails the drift gate.
// -----------------------------------------------------------------------------

// Package thready is the hand-written idiomatic wrapper over the generated Helix
// Thready core. Import path (proposed, [DEFAULT — adjustable], sdk-examples.md §10):
//
//	import thready "github.com/helix-development/helix-thready-go"
package thready

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"
	// gen "github.com/helix-development/helix-thready-go/gen/go" // GENERATED — never hand-edited.
)

// =============================================================================
// Config & construction (recipe R1)
// =============================================================================

// Config configures a Client. Only BaseURL and Auth are required.
type Config struct {
	// BaseURL is the REST /v1 root, e.g. "https://thready.hxd3v.com/v1".
	BaseURL string

	// Auth injects credentials on every request. Use APIKey(...) or JWT(...).
	Auth Authenticator

	// Retry controls the retry policy. Zero value uses DefaultRetry (retryable
	// codes only: rate_limited, unavailable, deadline_exceeded — error-model.md §3).
	Retry RetryPolicy

	// HTTPClient is optional. Defaults to an HTTP/3-capable client (vasic-digital/http3,
	// THREADY_HTTP3_ENABLED) with the HTTP/2 fallback negotiated by the transport.
	HTTPClient *http.Client
}

// DefaultRetry retries ONLY the retryable codes with exponential back-off + jitter,
// honoring Retry-After (sdk-strategy.md §5, error-model.md §3).
var DefaultRetry = RetryPolicy{MaxAttempts: 5, BaseDelay: 2 * time.Second, Factor: 2.0, Cap: 5 * time.Minute}

// RetryPolicy is the caller-visible retry configuration.
type RetryPolicy struct {
	MaxAttempts int
	BaseDelay   time.Duration
	Factor      float64
	Cap         time.Duration
}

// Client is the top-level SDK handle. Service groups (Posts, Search, Events, …)
// mirror the OpenAPI tags; each wraps the generated core.
type Client struct {
	cfg  Config
	http *http.Client

	Posts  *PostsService
	Search *SearchService
	Events *EventsService

	onTokenRotated func(TokenPair)
}

// New constructs a Client. It panics only on an obviously-invalid Config (empty
// BaseURL / nil Auth) so misconfiguration fails loudly at construction, matching the
// server's fail-loud posture (configuration.md §1).
func New(cfg Config) *Client {
	if cfg.BaseURL == "" {
		panic("thready.New: Config.BaseURL is required")
	}
	if cfg.Auth == nil {
		panic("thready.New: Config.Auth is required (use thready.APIKey or thready.JWT)")
	}
	if cfg.Retry == (RetryPolicy{}) {
		cfg.Retry = DefaultRetry
	}
	hc := cfg.HTTPClient
	if hc == nil {
		hc = &http.Client{Timeout: 30 * time.Second} // TODO: swap for the HTTP/3 transport.
	}
	c := &Client{cfg: cfg, http: hc}
	c.Posts = &PostsService{c: c}
	c.Search = &SearchService{c: c}
	c.Events = &EventsService{c: c}
	return c
}

// OnTokenRotated registers a hook fired after a transparent JWT refresh (recipe R7),
// so callers can persist the rotated pair (e.g. to a keyring).
func (c *Client) OnTokenRotated(fn func(TokenPair)) { c.onTokenRotated = fn }

// =============================================================================
// Auth injection (recipe R1 / R7)
// =============================================================================

// Authenticator attaches credentials to an outbound request. It is the single seam
// through which auth is injected into every generated-core call.
type Authenticator interface {
	// apply sets Authorization on req; for JWT it transparently refreshes a
	// near-expiry access token first (authn-authz.md §3; tokenmanager lifecycle
	// GetAccessToken/StoreTokenInfo/IsExpired, [VERIFIED]).
	apply(ctx context.Context, req *http.Request, c *Client) error
}

// APIKey authenticates with a scoped `sk-…` key (non-interactive automation).
func APIKey(key string) Authenticator { return apiKeyAuth{key: key} }

type apiKeyAuth struct{ key string }

func (a apiKeyAuth) apply(_ context.Context, req *http.Request, _ *Client) error {
	req.Header.Set("Authorization", "Bearer "+a.key) // Authorization: Bearer sk-…
	return nil
}

// TokenPair is an access/refresh JWT pair with the access-token expiry.
type TokenPair struct {
	AccessToken  string
	RefreshToken string
	ExpiresAt    time.Time
}

// JWT authenticates with a JWT pair and refreshes transparently before expiry.
func JWT(access, refresh string) Authenticator {
	return &jwtAuth{pair: TokenPair{AccessToken: access, RefreshToken: refresh}}
}

type jwtAuth struct{ pair TokenPair }

func (a *jwtAuth) apply(ctx context.Context, req *http.Request, c *Client) error {
	if a.isExpired() {
		np, err := a.refresh(ctx, c) // calls refreshToken (authn-authz.md §3)
		if err != nil {
			return err
		}
		a.pair = np
		if c.onTokenRotated != nil {
			c.onTokenRotated(np) // fire R7 rotation hook so caller can persist.
		}
	}
	req.Header.Set("Authorization", "Bearer "+a.pair.AccessToken)
	return nil
}

// isExpired mirrors tokenmanager.IsExpired with a small refresh threshold.
func (a *jwtAuth) isExpired() bool {
	return !a.pair.ExpiresAt.IsZero() && time.Until(a.pair.ExpiresAt) < 30*time.Second
}

func (a *jwtAuth) refresh(_ context.Context, _ *Client) (TokenPair, error) {
	// TODO(skeleton): POST {BaseURL}/auth/refresh with the refresh token; the old
	// refresh token is revoked server-side (pkg/token store, [VERIFIED]). Not wired.
	return TokenPair{}, errors.New("thready: JWT refresh not implemented in skeleton")
}

// =============================================================================
// Typed error model (recipe R6) — maps the wire Error envelope (error-model.md §3)
// =============================================================================

// Code is a stable, string-typed error code. Unknown codes round-trip as CodeUnknown
// so added codes are non-breaking (versioning.md, sdk-strategy.md §5).
type Code string

const (
	CodeInvalidArgument   Code = "invalid_argument"
	CodePermissionDenied  Code = "permission_denied"
	CodeConflict          Code = "conflict"          // e.g. single-claim 409 (rest-endpoints §2.6)
	CodeRateLimited       Code = "rate_limited"      // retryable
	CodeUnavailable       Code = "unavailable"       // retryable; also the fail-loud hash-embedder 503
	CodeDeadlineExceeded  Code = "deadline_exceeded" // retryable
	CodeUnknown           Code = "unknown"
)

// retryable reports whether the layer may retry this code (error-model.md §3).
func (c Code) retryable() bool {
	switch c {
	case CodeRateLimited, CodeUnavailable, CodeDeadlineExceeded:
		return true
	default:
		return false
	}
}

// Error is the idiomatic typed error carrying the stable code, trace id and details.
type Error struct {
	Code       Code
	Message    string
	TraceID    string
	RetryAfter int              // seconds, from Retry-After (0 if absent)
	Details    []map[string]any // structured details[]
}

func (e *Error) Error() string { return fmt.Sprintf("thready: %s: %s (trace %s)", e.Code, e.Message, e.TraceID) }

// =============================================================================
// Posts service — ONE example call (recipe R2: list with cursor pagination)
// =============================================================================

// PostsService wraps the generated Posts client.
type PostsService struct{ c *Client }

// PostFilter is the typed filter for List. Status is the processing enum:
// pending|running|succeeded|failed|skipped.
type PostFilter struct {
	ChannelID string
	Status    string
	Limit     int // 1..200
}

// Post is a thin re-export of the generated DTO (illustrative subset).
type Post struct {
	ID         string
	Hashtags   []string
	Categories []string
}

// List returns a lazy cursor iterator over data[] + meta.next_cursor. The caller
// writes an ordinary `for it.Next()` loop and never touches cursors (recipe R2).
//
//	it := client.Posts.List(ctx, thready.PostFilter{ChannelID: id, Status: "failed", Limit: 100})
//	for it.Next() { p := it.Post(); /* … */ }
//	if err := it.Err(); err != nil { /* typed *thready.Error */ }
func (s *PostsService) List(ctx context.Context, f PostFilter) *PostIterator {
	return &PostIterator{ctx: ctx, svc: s, filter: f}
}

// PostIterator lazily pages through results, following the cursor transparently.
type PostIterator struct {
	ctx    context.Context
	svc    *PostsService
	filter PostFilter

	page   []Post
	idx    int
	cursor string
	done   bool
	err    error
}

// Next advances to the next Post, fetching the next page (via the generated core)
// when the current page is exhausted. Returns false at end or on error.
func (it *PostIterator) Next() bool {
	if it.err != nil {
		return false
	}
	if it.idx < len(it.page) {
		it.idx++
		return true
	}
	if it.done {
		return false
	}
	// TODO(skeleton): call gen core `listPosts(filter, cursor)`, map to []Post,
	// set it.cursor = meta.next_cursor, it.done = (cursor == ""). Apply retry policy
	// (DefaultRetry) around the call; map any non-2xx to *thready.Error.
	it.err = errors.New("thready: PostIterator.Next not implemented in skeleton")
	return false
}

// Post returns the current item (valid after a true Next()).
func (it *PostIterator) Post() *Post {
	if it.idx == 0 || it.idx > len(it.page) {
		return nil
	}
	return &it.page[it.idx-1]
}

// Err returns the terminal error, if any (a *thready.Error for API failures).
func (it *PostIterator) Err() error { return it.err }

// =============================================================================
// Search service (recipe R4 stub — fail-loud embedder guard)
// =============================================================================

// SearchService wraps the generated Search client.
type SearchService struct{ c *Client }

// SearchRequest is the typed search input (illustrative subset).
type SearchRequest struct {
	Query   string
	Mode    string // semantic|keyword|hybrid
	Sources []string
	TopK    int
	Rerank  bool
}

// SearchResult mirrors the generated response; Embedder MUST be a real provider —
// a "hash" value (or a 503 CodeUnavailable) means the HashEmbedder stub is active
// and results are garbage ([GAP: #1], configuration.md §8.1).
type SearchResult struct {
	Embedder string
	Results  []struct {
		SourceID string
		Score    float64
		Snippet  string
	}
}

// Query runs semantic/keyword/hybrid search. Callers SHOULD defensively reject
// res.Embedder == "hash" (recipe R4).
func (s *SearchService) Query(ctx context.Context, req SearchRequest) (*SearchResult, error) {
	// TODO(skeleton): call gen core `search(req)`; on 503 return &Error{Code: CodeUnavailable}.
	_ = ctx
	_ = req
	return nil, errors.New("thready: Search.Query not implemented in skeleton")
}

// =============================================================================
// Events service — the events subscription (recipe R5)
// =============================================================================

// EventsService wraps the client-facing subscription (WS/SSE; Go may use Connect
// streaming, [OPEN: sdk-1], event-bus-contract.md §11).
type EventsService struct{ c *Client }

// EventFilter selects event types (glob supported, filter.ByGlob [VERIFIED]) and
// whether to replay the last sticky value on connect.
type EventFilter struct {
	Types        []string // e.g. []string{"processing.*", "asset.ready"}
	ReplaySticky bool
}

// EventEnvelope is the typed event (event-bus-contract.md); Payload is left as a
// map in the skeleton (the generated core provides strongly-typed payloads).
type EventEnvelope struct {
	ID      string
	Type    string
	Payload map[string]any
}

// Subscription is a live event stream. Range over C; Ack advances the durable
// cursor; the layer auto-reconnects from the last Ack'd id and reconciles via
// getStickyEvent after long outages (event-bus-contract.md §7).
type Subscription struct {
	C      chan EventEnvelope
	cancel context.CancelFunc
}

// Ack advances the durable cursor past ev.ID.
func (s *Subscription) Ack(id string) error {
	// TODO(skeleton): persist the cursor / send the ack frame. Not wired.
	_ = id
	return nil
}

// Close terminates the subscription and its reconnect loop.
func (s *Subscription) Close() error {
	if s.cancel != nil {
		s.cancel()
	}
	return nil
}

// Subscribe opens a typed, auto-reconnecting subscription (recipe R5).
//
//	sub, _ := client.Events.Subscribe(ctx, thready.EventFilter{
//	    Types: []string{"processing.*", "asset.ready"}, ReplaySticky: true,
//	})
//	defer sub.Close()
//	for ev := range sub.C {
//	    switch ev.Type {
//	    case "processing.completed": /* ev.Payload["post_id"] */
//	    }
//	    sub.Ack(ev.ID)
//	}
func (s *EventsService) Subscribe(ctx context.Context, f EventFilter) (*Subscription, error) {
	ctx, cancel := context.WithCancel(ctx)
	sub := &Subscription{C: make(chan EventEnvelope), cancel: cancel}
	// TODO(skeleton): dial WS/SSE (or Connect stream), apply Auth via s.c.cfg.Auth,
	// send the EventFilter, then run a reconnect loop that: (a) on connect replays the
	// sticky value when f.ReplaySticky, (b) decodes frames into EventEnvelope onto
	// sub.C, (c) on socket drop reconnects from the last Ack'd id with back-off. Not wired.
	go func() {
		<-ctx.Done()
		close(sub.C)
	}()
	return sub, errors.New("thready: Events.Subscribe not implemented in skeleton")
}
