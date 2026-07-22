package cli

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"strings"

	thready "digital.vasic.threadysdk"
)

// SDKAdapter is the production implementation of APIClient. It wraps the real
// typed digital.vasic.threadysdk client and converts between the SDK's wire DTOs
// and the CLI-local DTOs. It is intentionally thin: each method delegates to
// exactly one SDK call (or, for Whoami, decodes the SDK-held token locally).
//
// The command layer never imports the SDK; only this file and cmd/thready do.
// That keeps Run + the subcommands unit-testable against a fake, while this
// adapter is compile-checked against the actual SDK surface.
type SDKAdapter struct {
	c *thready.Client
}

// interface guard: *SDKAdapter must satisfy APIClient.
var _ APIClient = (*SDKAdapter)(nil)

// NewSDKAdapter wraps an already-constructed SDK client.
func NewSDKAdapter(c *thready.Client) *SDKAdapter { return &SDKAdapter{c: c} }

func (a *SDKAdapter) Login(ctx context.Context, creds Credentials) (*TokenPair, error) {
	tp, err := a.c.Login(ctx, thready.LoginRequest{
		Email:    creds.Email,
		Password: creds.Password,
		TOTP:     creds.TOTP,
	})
	if err != nil {
		return nil, err
	}
	return &TokenPair{
		AccessToken:  tp.AccessToken,
		RefreshToken: tp.RefreshToken,
		TokenType:    tp.TokenType,
		ExpiresIn:    tp.ExpiresIn,
	}, nil
}

func (a *SDKAdapter) ListChannels(ctx context.Context) ([]Channel, error) {
	chs, err := a.c.ListChannels(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]Channel, 0, len(chs))
	for _, ch := range chs {
		out = append(out, channelFromSDK(ch))
	}
	return out, nil
}

func (a *SDKAdapter) CreateChannel(ctx context.Context, in CreateChannelInput) (*Channel, error) {
	ch, err := a.c.CreateChannel(ctx, thready.CreateChannelRequest{
		Name:        in.Name,
		Platform:    in.Platform,
		ExternalRef: in.ExternalRef,
	})
	if err != nil {
		return nil, err
	}
	out := channelFromSDK(*ch)
	return &out, nil
}

func (a *SDKAdapter) GetPost(ctx context.Context, id string) (*Post, error) {
	p, err := a.c.GetPost(ctx, id)
	if err != nil {
		return nil, err
	}
	return &Post{
		ID:         p.ID,
		ChannelID:  p.ChannelID,
		AccountID:  p.AccountID,
		Body:       p.Body,
		Hashtags:   p.Hashtags,
		Categories: p.Categories,
		Status:     p.Status,
		CreatedAt:  p.CreatedAt,
	}, nil
}

func (a *SDKAdapter) Reprocess(ctx context.Context, id string) (*Job, error) {
	job, err := a.c.Reprocess(ctx, id)
	if err != nil {
		return nil, err
	}
	return &Job{
		JobID:      job.JobID,
		PostID:     job.PostID,
		Status:     job.Status,
		Precedence: job.Precedence,
		QueuedAt:   job.QueuedAt,
	}, nil
}

func (a *SDKAdapter) Search(ctx context.Context, query string, opts SearchOptions) (*SearchResults, error) {
	res, err := a.c.Search(ctx, thready.SearchRequest{
		Query:   query,
		Mode:    opts.Mode,
		Sources: opts.Sources,
		TopK:    opts.TopK,
		Rerank:  opts.Rerank,
	})
	if err != nil {
		return nil, err
	}
	hits := make([]SearchHit, 0, len(res.Results))
	for _, h := range res.Results {
		hits = append(hits, SearchHit{
			SourceID: h.SourceID,
			Kind:     h.Kind,
			Score:    h.Score,
			Snippet:  h.Snippet,
		})
	}
	return &SearchResults{Results: hits, TookMs: res.TookMs, Embedder: res.Embedder}, nil
}

func (a *SDKAdapter) ListSkills(ctx context.Context) ([]Skill, error) {
	sks, err := a.c.ListSkills(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]Skill, 0, len(sks))
	for _, sk := range sks {
		out = append(out, Skill{ID: sk.ID, Name: sk.Name, Kind: sk.Kind, SortOrder: sk.SortOrder})
	}
	return out, nil
}

// Whoami resolves the caller identity. The sdk_go client exposes no server-side
// whoami/introspection endpoint, so this derives identity from the bearer access
// token the SDK already holds: if it is a JWT, the standard claims (sub, email,
// tier) are decoded locally without a network round-trip. This is an honest,
// documented CLI-level convenience — when the gateway later grows a GET
// /v1/auth/me route, swap this body to call it.
func (a *SDKAdapter) Whoami(_ context.Context) (*Identity, error) {
	tok := a.c.AccessToken()
	id := &Identity{TokenPresent: tok != ""}
	claims := decodeJWTClaims(tok)
	id.Subject = claims.Sub
	id.Email = claims.Email
	id.Tier = claims.Tier
	return id, nil
}

func channelFromSDK(ch thready.Channel) Channel {
	return Channel{
		ID:          ch.ID,
		AccountID:   ch.AccountID,
		Name:        ch.Name,
		Platform:    ch.Platform,
		ExternalRef: ch.ExternalRef,
		CreatedAt:   ch.CreatedAt,
	}
}

// jwtClaims is the small subset of registered/custom claims whoami surfaces.
type jwtClaims struct {
	Sub   string `json:"sub"`
	Email string `json:"email"`
	Tier  string `json:"tier"`
}

// decodeJWTClaims best-effort decodes the payload segment of a JWT. It never
// errors: a non-JWT or malformed token yields zero-value claims.
func decodeJWTClaims(token string) jwtClaims {
	var c jwtClaims
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return c
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return c
	}
	_ = json.Unmarshal(raw, &c)
	return c
}
