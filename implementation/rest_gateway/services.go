package gateway

import (
	"context"
	"sort"
	"strings"
	"sync"
	"time"
)

// This file defines the injected Service interfaces (the HTTP surface's only
// dependency) plus in-memory implementations that give the gateway real,
// observable end-to-end behaviour without importing the sibling
// implementation/* domain modules. That integration is a later go.work step;
// until then these are honest in-memory stubs — see README/EVIDENCE.

// ----- Domain DTOs -----

// Principal is the authenticated caller resolved from a credential.
type Principal struct {
	UserID    string   `json:"user_id"`
	Email     string   `json:"email,omitempty"`
	Role      string   `json:"role"`
	AccountID string   `json:"account_id,omitempty"`
	Scopes    []string `json:"scopes"`
}

// Account is a tenant.
type Account struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
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

// ChannelInput is the create body for POST /channels.
type ChannelInput struct {
	Name        string `json:"name"`
	Platform    string `json:"platform"`
	ExternalRef string `json:"external_ref"`
}

// Thread is a root post plus its organic reply chain (system replies excluded).
type Thread struct {
	ID        string   `json:"id"`
	ChannelID string   `json:"channel_id"`
	RootPost  string   `json:"root_post_id"`
	Replies   []string `json:"reply_post_ids"`
}

// Post is a channel post with its processing state.
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

// ProcessingJob is returned by (re)process triggers (202 Accepted).
type ProcessingJob struct {
	JobID      string    `json:"job_id"`
	PostID     string    `json:"post_id"`
	Status     string    `json:"status"`
	Precedence []string  `json:"precedence"`
	QueuedAt   time.Time `json:"queued_at"`
}

// Skill is a knowledge unit in the Skill-Graph DAG.
type Skill struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Kind      string `json:"kind"`
	SortOrder int    `json:"sort_order"`
}

// SearchRequest is the POST /search body.
type SearchRequest struct {
	Query   string   `json:"query"`
	Mode    string   `json:"mode"`
	Sources []string `json:"sources"`
	TopK    int      `json:"top_k"`
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

// SearchResponse is the ranked result set + provenance.
type SearchResponse struct {
	Results  []SearchHit `json:"results"`
	TookMs   int         `json:"took_ms"`
	Embedder string      `json:"embedder"`
}

// Event is a single event-bus event, rendered as an SSE data: line.
type Event struct {
	ID      string         `json:"id"`
	Type    string         `json:"type"`
	Payload map[string]any `json:"payload"`
	TraceID string         `json:"trace_id"`
}

// ----- Service interfaces (the injected dependency set) -----

// AuthService verifies credentials and resolves a Principal.
type AuthService interface {
	Authenticate(email, password, totp string) (Principal, bool)
}

// ChannelService manages channels + thread reads.
type ChannelService interface {
	List(accountID string) []Channel
	Create(accountID string, in ChannelInput) (Channel, error)
	Threads(channelID string) ([]Thread, bool)
}

// PostService reads posts and triggers reprocessing.
type PostService interface {
	Get(id string) (Post, bool)
	Reprocess(id string) (ProcessingJob, error)
}

// SearchService runs semantic/keyword/hybrid search.
type SearchService interface {
	Search(req SearchRequest) (SearchResponse, error)
}

// SkillService reads the Skill-Graph.
type SkillService interface {
	List() []Skill
}

// AccountService lists accounts visible to a principal.
type AccountService interface {
	List(p Principal) []Account
}

// EventService is a minimal pub/sub for the SSE stream.
type EventService interface {
	Subscribe(ctx context.Context) (<-chan Event, func())
	Publish(ev Event)
}

// Services is the full injected dependency set for the HTTP surface.
type Services struct {
	Auth     AuthService
	Channels ChannelService
	Posts    PostService
	Search   SearchService
	Skills   SkillService
	Accounts AccountService
	Events   EventService
}

// ==========================================================================
// In-memory implementations
// ==========================================================================

// memUser is a seeded credential record.
type memUser struct {
	principal Principal
	password  string
	totp      string // required (admin tiers); "" means not required
}

// InMemoryAuth is a seeded, in-memory AuthService.
type InMemoryAuth struct {
	mu    sync.RWMutex
	users map[string]memUser // keyed by email
}

// Authenticate checks the password (and TOTP when the seeded user requires it).
func (a *InMemoryAuth) Authenticate(email, password, totp string) (Principal, bool) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	u, ok := a.users[strings.ToLower(strings.TrimSpace(email))]
	if !ok {
		return Principal{}, false
	}
	if u.password != password {
		return Principal{}, false
	}
	if u.totp != "" && u.totp != totp {
		return Principal{}, false
	}
	return u.principal, true
}

// InMemoryChannels is a seeded, in-memory ChannelService.
type InMemoryChannels struct {
	mu       sync.Mutex
	channels []Channel
	threads  map[string][]Thread
	seq      int
}

// List returns channels scoped to the account (or all when accountID is "").
func (c *InMemoryChannels) List(accountID string) []Channel {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := []Channel{}
	for _, ch := range c.channels {
		if accountID == "" || ch.AccountID == accountID {
			out = append(out, ch)
		}
	}
	return out
}

// Create registers a channel and returns it.
func (c *InMemoryChannels) Create(accountID string, in ChannelInput) (Channel, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.seq++
	ch := Channel{
		ID:          "chan-" + itoa(c.seq),
		AccountID:   accountID,
		Name:        in.Name,
		Platform:    in.Platform,
		ExternalRef: in.ExternalRef,
		CreatedAt:   time.Unix(int64(1_700_000_000+c.seq), 0).UTC(),
	}
	c.channels = append(c.channels, ch)
	return ch, nil
}

// Threads returns the thread list for a channel.
func (c *InMemoryChannels) Threads(channelID string) ([]Thread, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	t, ok := c.threads[channelID]
	if !ok {
		return nil, false
	}
	return t, true
}

// count is a test/inspection helper returning the number of channels.
func (c *InMemoryChannels) count() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.channels)
}

// InMemoryPosts is a seeded, in-memory PostService.
type InMemoryPosts struct {
	mu    sync.Mutex
	posts map[string]Post
	seq   int
}

// Get returns a post by id.
func (p *InMemoryPosts) Get(id string) (Post, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	post, ok := p.posts[id]
	return post, ok
}

// Reprocess forces a fresh processing run and returns a queued job. A missing
// post is reported via not_found.
func (p *InMemoryPosts) Reprocess(id string) (ProcessingJob, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if _, ok := p.posts[id]; !ok {
		return ProcessingJob{}, NewError(CodeNotFound, "post not found")
	}
	p.seq++
	return ProcessingJob{
		JobID:  "job-" + itoa(p.seq),
		PostID: id,
		Status: "queued",
		// Deterministic Skill precedence: download > convert > analyze > research > reply.
		Precedence: []string{"download", "convert", "analyze", "research", "reply"},
		QueuedAt:   time.Unix(int64(1_700_000_000+p.seq), 0).UTC(),
	}, nil
}

// InMemorySearch is a seeded, in-memory SearchService with a small corpus.
type InMemorySearch struct {
	corpus   []SearchHit
	embedder string
}

// Search scores the corpus against the query terms and returns the top_k ranked
// hits (score descending). If the active embedder is the non-semantic hash stub
// it fails loudly with 503, mirroring the api/rest-endpoints.md §2.8 trap.
func (s *InMemorySearch) Search(req SearchRequest) (SearchResponse, error) {
	if s.embedder == "" || s.embedder == "hash" {
		return SearchResponse{}, NewError(CodeUnavailable,
			"semantic search requires a real llama.cpp embedder; hash stub active")
	}
	terms := strings.Fields(strings.ToLower(req.Query))
	scored := make([]SearchHit, 0, len(s.corpus))
	for i, h := range s.corpus {
		hit := h
		// Base score keeps ordering stable; term matches boost relevance.
		hit.Score = 1.0 / float64(i+2)
		for _, t := range terms {
			if strings.Contains(strings.ToLower(h.Snippet), t) {
				hit.Score += 1.0
			}
		}
		scored = append(scored, hit)
	}
	sort.SliceStable(scored, func(i, j int) bool { return scored[i].Score > scored[j].Score })
	topK := req.TopK
	if topK <= 0 {
		topK = 20
	}
	if topK < len(scored) {
		scored = scored[:topK]
	}
	return SearchResponse{Results: scored, TookMs: 7, Embedder: s.embedder}, nil
}

// InMemorySkills is a seeded, in-memory SkillService.
type InMemorySkills struct {
	skills []Skill
}

// List returns the skills sorted by dispatch precedence.
func (s *InMemorySkills) List() []Skill {
	out := make([]Skill, len(s.skills))
	copy(out, s.skills)
	sort.SliceStable(out, func(i, j int) bool { return out[i].SortOrder < out[j].SortOrder })
	return out
}

// InMemoryAccounts is a seeded, in-memory AccountService.
type InMemoryAccounts struct {
	accounts []Account
}

// List returns all accounts for root, else the principal's own account.
func (a *InMemoryAccounts) List(p Principal) []Account {
	if p.Role == RoleRoot {
		out := make([]Account, len(a.accounts))
		copy(out, a.accounts)
		return out
	}
	out := []Account{}
	for _, ac := range a.accounts {
		if ac.ID == p.AccountID {
			out = append(out, ac)
		}
	}
	return out
}

// InMemoryEvents is a minimal in-process pub/sub for the SSE stream.
type InMemoryEvents struct {
	mu   sync.Mutex
	subs map[int]chan Event
	seq  int
}

// Subscribe registers a subscriber synchronously and returns its channel plus a
// cancel func that unregisters it. Because registration completes before the
// call returns, a Publish that happens after Subscribe is delivered.
func (e *InMemoryEvents) Subscribe(ctx context.Context) (<-chan Event, func()) {
	e.mu.Lock()
	e.seq++
	id := e.seq
	ch := make(chan Event, 16)
	if e.subs == nil {
		e.subs = map[int]chan Event{}
	}
	e.subs[id] = ch
	e.mu.Unlock()

	var once sync.Once
	cancel := func() {
		once.Do(func() {
			e.mu.Lock()
			if c, ok := e.subs[id]; ok {
				delete(e.subs, id)
				close(c)
			}
			e.mu.Unlock()
		})
	}
	return ch, cancel
}

// Publish fans an event out to every current subscriber (non-blocking).
func (e *InMemoryEvents) Publish(ev Event) {
	e.mu.Lock()
	defer e.mu.Unlock()
	for _, ch := range e.subs {
		select {
		case ch <- ev:
		default:
			// Drop for a slow consumer rather than block the publisher.
		}
	}
}

// ----- Seeding -----

// Seeded credential set for the in-memory gateway (real, working end-to-end).
const (
	SeedRootEmail  = "root@thready.test"
	SeedRootPass   = "rootpassword-12"
	SeedRootTOTP   = "123456"
	SeedAdminEmail = "admin@thready.test"
	SeedAdminPass  = "adminpassword-12"
	SeedAdminTOTP  = "654321"
	SeedUserEmail  = "user@thready.test"
	SeedUserPass   = "userpassword-123"

	SeedAccountA = "acct-a"
	SeedAccountB = "acct-b"
	SeedChannelA = "chan-1"
	SeedPostA    = "post-1"
)

var (
	scopesUser  = []string{"posts:read", "posts:write", "assets:read", "search:read", "skills:read", "events:read"}
	scopesAdmin = append([]string{"accounts:admin", "billing:read", "skills:write"}, scopesUser...)
	scopesRoot  = append([]string{"root:admin"}, scopesAdmin...)
)

// NewInMemoryServices builds a fully seeded Services set. It is the honest
// self-contained backend used by tests and the standalone binary.
func NewInMemoryServices() Services {
	auth := &InMemoryAuth{users: map[string]memUser{
		SeedRootEmail: {
			principal: Principal{UserID: "user-root", Email: SeedRootEmail, Role: RoleRoot, AccountID: SeedAccountA, Scopes: scopesRoot},
			password:  SeedRootPass, totp: SeedRootTOTP,
		},
		SeedAdminEmail: {
			principal: Principal{UserID: "user-admin", Email: SeedAdminEmail, Role: RoleAccountAdmin, AccountID: SeedAccountA, Scopes: scopesAdmin},
			password:  SeedAdminPass, totp: SeedAdminTOTP,
		},
		SeedUserEmail: {
			principal: Principal{UserID: "user-std", Email: SeedUserEmail, Role: RoleUser, AccountID: SeedAccountA, Scopes: scopesUser},
			password:  SeedUserPass, totp: "",
		},
	}}

	channels := &InMemoryChannels{
		channels: []Channel{{
			ID: SeedChannelA, AccountID: SeedAccountA, Name: "general",
			Platform: "telegram", ExternalRef: "@thready_general",
			CreatedAt: time.Unix(1_700_000_000, 0).UTC(),
		}},
		threads: map[string][]Thread{
			SeedChannelA: {{ID: "thread-1", ChannelID: SeedChannelA, RootPost: SeedPostA, Replies: []string{"post-2"}}},
		},
		seq: 1,
	}

	posts := &InMemoryPosts{posts: map[string]Post{
		SeedPostA: {
			ID: SeedPostA, ChannelID: SeedChannelA, AccountID: SeedAccountA,
			Body:       "self-hosted vector database benchmarks thread",
			Hashtags:   []string{"#research", "#vectordb"},
			Categories: []string{"research", "software"},
			Status:     "succeeded",
			CreatedAt:  time.Unix(1_700_000_100, 0).UTC(),
		},
	}}

	span := "section:1"
	search := &InMemorySearch{
		embedder: "llama", // real semantic embedder; not the HashEmbedder stub.
		corpus: []SearchHit{
			{SourceID: SeedPostA, Kind: "post", Score: 0, Span: &span, Snippet: "self-hosted vector database benchmarks and pgvector cosine"},
			{SourceID: "gen-1", Kind: "generated", Score: 0, Snippet: "semantic search over posts and generated materials"},
			{SourceID: "post-2", Kind: "post", Score: 0, Snippet: "thread reply adding hashtags to a link-only root"},
		},
	}

	skills := &InMemorySkills{skills: []Skill{
		{ID: "skill-download", Name: "download", Kind: "atomic", SortOrder: 1},
		{ID: "skill-analyze", Name: "analyze", Kind: "composite", SortOrder: 3},
		{ID: "skill-reply", Name: "reply", Kind: "atomic", SortOrder: 5},
	}}

	accounts := &InMemoryAccounts{accounts: []Account{
		{ID: SeedAccountA, Name: "Acme", CreatedAt: time.Unix(1_699_000_000, 0).UTC()},
		{ID: SeedAccountB, Name: "Globex", CreatedAt: time.Unix(1_699_500_000, 0).UTC()},
	}}

	events := &InMemoryEvents{subs: map[int]chan Event{}}

	return Services{
		Auth:     auth,
		Channels: channels,
		Posts:    posts,
		Search:   search,
		Skills:   skills,
		Accounts: accounts,
		Events:   events,
	}
}

// itoa is a tiny int->string helper (avoids pulling strconv into hot paths that
// only ever format small sequence numbers).
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
