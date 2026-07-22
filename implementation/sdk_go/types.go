package thready

import "time"

// This file holds the typed request/response DTOs the SDK marshals to and
// decodes from the Helix Thready REST `/v1` surface. The field sets and JSON
// tags mirror the wire shapes the gateway actually emits (the concrete
// realization of docs/public/research/mvp/api/openapi.yaml — see the sibling
// implementation/rest_gateway module), so a *Client decodes a live gateway
// response with no transformation layer.

// ----- Pagination -----

// PageMeta is the list-envelope cursor metadata: {"next_cursor","total_estimate"}.
type PageMeta struct {
	NextCursor    *string `json:"next_cursor"`
	TotalEstimate *int    `json:"total_estimate"`
}

// listEnvelope is the standard collection wrapper `{"data":[...],"meta":{...}}`
// returned by the list endpoints (channels, threads, skills, accounts). It is
// generic so each method decodes into the right element type.
type listEnvelope[T any] struct {
	Data []T      `json:"data"`
	Meta PageMeta `json:"meta"`
}

// ----- Auth -----

// LoginRequest is the credential body for POST /v1/auth/login. TOTP is required
// for admin tiers and omitted for standard users.
type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	TOTP     string `json:"totp,omitempty"`
}

// TokenPair is the login/refresh success body: a short-lived access token plus
// a rotating refresh token.
type TokenPair struct {
	AccessToken      string `json:"access_token"`
	RefreshToken     string `json:"refresh_token"`
	TokenType        string `json:"token_type"`
	ExpiresIn        int    `json:"expires_in"`
	RefreshExpiresIn int    `json:"refresh_expires_in"`
}

// ----- Channels -----

// Channel is a registered messenger channel/group.
type Channel struct {
	ID          string    `json:"id"`
	AccountID   string    `json:"account_id"`
	Name        string    `json:"name"`
	Platform    string    `json:"platform"`
	ExternalRef string    `json:"external_ref"`
	CreatedAt   time.Time `json:"created_at"`
}

// CreateChannelRequest is the create body for POST /v1/channels.
type CreateChannelRequest struct {
	Name        string `json:"name"`
	Platform    string `json:"platform"`
	ExternalRef string `json:"external_ref"`
}

// Thread is a root post plus its organic reply chain (system replies excluded).
type Thread struct {
	ID           string   `json:"id"`
	ChannelID    string   `json:"channel_id"`
	RootPostID   string   `json:"root_post_id"`
	ReplyPostIDs []string `json:"reply_post_ids"`
}

// ----- Posts / processing -----

// Post is a channel post with its processing status.
type Post struct {
	ID         string    `json:"id"`
	ChannelID  string    `json:"channel_id"`
	AccountID  string    `json:"account_id"`
	Body       string    `json:"body"`
	Hashtags   []string  `json:"hashtags"`
	Categories []string  `json:"categories"`
	Status     string    `json:"status"`
	CreatedAt  time.Time `json:"created_at"`
}

// Job is the async (re)processing job returned (202 Accepted) by Reprocess. The
// deterministic Precedence encodes the Skill dispatch order
// (download > convert > analyze > research > reply).
type Job struct {
	JobID      string    `json:"job_id"`
	PostID     string    `json:"post_id"`
	Status     string    `json:"status"`
	Precedence []string  `json:"precedence"`
	QueuedAt   time.Time `json:"queued_at"`
}

// ----- Search -----

// SearchRequest is the POST /v1/search body. Mode is one of
// semantic|keyword|hybrid; Sources selects the corpora (posts|generated|assets).
type SearchRequest struct {
	Query   string   `json:"query"`
	Mode    string   `json:"mode,omitempty"`
	Sources []string `json:"sources,omitempty"`
	TopK    int      `json:"top_k,omitempty"`
	Rerank  bool     `json:"rerank"`
}

// SearchHit is a single ranked result.
type SearchHit struct {
	SourceID string  `json:"source_id"`
	Kind     string  `json:"kind"`
	Score    float64 `json:"score"`
	Span     *string `json:"span"`
	Snippet  string  `json:"snippet"`
}

// SearchResults is the ranked result set plus provenance. Embedder echoes the
// active embedding provider (the gateway fails loud with 503 unavailable when a
// non-semantic hash stub is active).
type SearchResults struct {
	Results  []SearchHit `json:"results"`
	TookMs   int         `json:"took_ms"`
	Embedder string      `json:"embedder"`
}

// ----- Skills -----

// Skill is a knowledge unit in the Skill-Graph DAG. SortOrder is the dispatch
// precedence within a stage.
type Skill struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Kind      string `json:"kind"`
	SortOrder int    `json:"sort_order"`
}

// ----- Events -----

// Event is a single event-bus event as delivered on the SSE stream
// (GET /v1/events). Payload is the event-type-specific body.
type Event struct {
	ID      string         `json:"id"`
	Type    string         `json:"type"`
	Payload map[string]any `json:"payload"`
	TraceID string         `json:"trace_id"`
}
