package cli

import (
	"context"
	"time"
)

// APIClient is the subset of the Helix Thready `/v1` control surface the CLI
// command layer depends on. It is deliberately narrow — only the operations the
// subcommands need — and it is defined in terms of CLI-local DTOs (below) rather
// than SDK types, so the command layer compiles and is unit-tested WITHOUT any
// dependency on the sdk_go module. Production wiring supplies *SDKAdapter (see
// adapter.go), which delegates to the real digital.vasic.threadysdk client; the
// tests supply an in-memory fake. The two are interchangeable behind this
// interface.
//
// Every method takes a context.Context, mirroring the SDK, so callers keep
// cancellation/deadline control.
type APIClient interface {
	// Login exchanges credentials (plus TOTP for admin tiers) for a token pair.
	// The concrete client is expected to store the returned access token so
	// later calls authenticate automatically.
	Login(ctx context.Context, creds Credentials) (*TokenPair, error)
	// ListChannels lists the channels registered for the caller's tenant.
	ListChannels(ctx context.Context) ([]Channel, error)
	// CreateChannel registers a channel/group to read.
	CreateChannel(ctx context.Context, in CreateChannelInput) (*Channel, error)
	// GetPost fetches a single post by id.
	GetPost(ctx context.Context, id string) (*Post, error)
	// Reprocess forces a fresh processing run for a post and returns the queued
	// job (the gateway answers 202 Accepted).
	Reprocess(ctx context.Context, id string) (*Job, error)
	// Search runs a semantic / keyword / hybrid search over posts and generated
	// materials.
	Search(ctx context.Context, query string, opts SearchOptions) (*SearchResults, error)
	// ListSkills lists the Skill-Graph knowledge units.
	ListSkills(ctx context.Context) ([]Skill, error)
	// Whoami reports the identity the client is authenticated as.
	Whoami(ctx context.Context) (*Identity, error)
}

// ----- CLI-local DTOs -----
//
// These mirror the shapes the gateway emits (and the sdk_go types) closely
// enough that the real adapter's conversions are field-for-field trivial, while
// keeping the command layer free of any SDK import.

// Credentials is the login input.
type Credentials struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	TOTP     string `json:"totp,omitempty"`
}

// TokenPair is the login success result.
type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
}

// Channel is a registered messenger channel/group.
type Channel struct {
	ID          string    `json:"id"`
	AccountID   string    `json:"account_id"`
	Name        string    `json:"name"`
	Platform    string    `json:"platform"`
	ExternalRef string    `json:"external_ref"`
	CreatedAt   time.Time `json:"created_at"`
}

// CreateChannelInput is the create-channel input.
type CreateChannelInput struct {
	Name        string `json:"name"`
	Platform    string `json:"platform"`
	ExternalRef string `json:"external_ref"`
}

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

// Job is the async (re)processing job returned by Reprocess.
type Job struct {
	JobID      string    `json:"job_id"`
	PostID     string    `json:"post_id"`
	Status     string    `json:"status"`
	Precedence []string  `json:"precedence"`
	QueuedAt   time.Time `json:"queued_at"`
}

// SearchOptions carries the tunable search parameters parsed from flags.
type SearchOptions struct {
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
	Snippet  string  `json:"snippet"`
}

// SearchResults is the ranked result set plus provenance.
type SearchResults struct {
	Results  []SearchHit `json:"results"`
	TookMs   int         `json:"took_ms"`
	Embedder string      `json:"embedder"`
}

// Skill is a knowledge unit in the Skill-Graph DAG.
type Skill struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Kind      string `json:"kind"`
	SortOrder int    `json:"sort_order"`
}

// Identity is the resolved caller identity for `whoami`.
type Identity struct {
	Subject      string `json:"subject"`
	Email        string `json:"email"`
	Tier         string `json:"tier"`
	TokenPresent bool   `json:"token_present"`
}
