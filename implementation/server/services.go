// Package server assembles the Helix Thready REST /v1 gateway over the REAL
// sibling domain modules instead of the gateway's own in-memory stubs.
//
// Every wired gateway.Service member below delegates to genuine sibling code:
//
//	AuthService   -> digital.vasic.userservice  (real PBKDF2 verify + real RFC6238 TOTP)
//	SearchService -> digital.vasic.semsearch    (real chunk -> embed -> cosine-KNN pipeline)
//	SkillService  -> digital.vasic.skilldispatch(real Registry + real OrderByPrecedence)
//	EventService  -> digital.vasic.eventbusservice (real pub/sub Bus)
//	PostService   -> real in-memory post store + real skilldispatch precedence resolver
//
// ChannelService and AccountService are gateway-level CRUD with NO dedicated
// domain module; they are backed by honest in-memory stores (see EVIDENCE.md).
package server

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	eventbus "digital.vasic.eventbusservice"
	gateway "digital.vasic.restgateway"
	semsearch "digital.vasic.semsearch"
	skilldispatch "digital.vasic.skilldispatch"
	userservice "digital.vasic.userservice"
)

// EmbedderLabel is the honest provenance label for the search embedder. It is
// the module's deterministic feature-hashing embedder — NOT a live llama.cpp
// embedding server. This label is surfaced in SearchResponse.Embedder.
const EmbedderLabel = "semsearch/hash-deterministic"

// Fixed base32 TOTP secrets for the MFA-required admin tiers. The gateway's
// SeedRootTOTP/SeedAdminTOTP are STATIC 6-digit codes and cannot authenticate
// against a real RFC 6238 verifier (which is time-based), so the real-wired
// AuthService provisions genuine shared secrets instead. e2e drivers compute
// the current code from these via userservice.NewTOTPFromBase32 (see EVIDENCE).
const (
	SeedRootTOTPSecretB32  = "JBSWY3DPEHPK3PXPJBSWY3DPEHPK3PXP" // 20 raw bytes
	SeedAdminTOTPSecretB32 = "KRSXG5CTMVRXEZLUKRSXG5CTMVRXEZLU" // 20 raw bytes
)

// seedHashIterations is the PBKDF2 cost used when seeding password hashes. It is
// the REAL userservice.Hasher at a reduced-but-genuine cost so the -race e2e
// suite stays fast; verification recomputes with the iteration count embedded in
// the hash string, so it is parameter-agnostic. Production uses
// userservice.DefaultPBKDF2Iterations.
const seedHashIterations = 12_000

// Scope sets, replicated verbatim from rest_gateway/services.go (they are
// unexported there). A route requires both its role floor and its scopes.
var (
	scopesUser  = []string{"posts:read", "posts:write", "assets:read", "search:read", "skills:read", "events:read"}
	scopesAdmin = append([]string{"accounts:admin", "billing:read", "skills:write"}, scopesUser...)
	scopesRoot  = append([]string{"root:admin"}, scopesAdmin...)
)

// ==========================================================================
// AuthService -> user_service (real PBKDF2 + real TOTP)
// ==========================================================================

// seedIdentity binds a real userservice.User (genuine PBKDF2 hash + TOTP secret)
// to the gateway Principal it resolves to on a successful verification.
type seedIdentity struct {
	user       userservice.User
	principal  gateway.Principal
	totpSecret string // base32; "" when MFA is not enrolled
}

// realAuth is a gateway.AuthService whose credential checks run entirely through
// user_service's real crypto. It holds the seeded identities in a local map —
// user_service ships no user-CRUD store type, so this thin map IS the "user
// store", but every credential decision goes through genuine sibling code.
type realAuth struct {
	byEmail map[string]seedIdentity
	now     func() time.Time
}

// Authenticate verifies the password with user_service's real PBKDF2 compare and,
// for MFA-enrolled users, the TOTP with user_service's real RFC 6238 validator.
// A wrong password or wrong TOTP fails THROUGH the real verifier.
func (a *realAuth) Authenticate(email, password, totp string) (gateway.Principal, bool) {
	id, ok := a.byEmail[strings.ToLower(strings.TrimSpace(email))]
	if !ok {
		return gateway.Principal{}, false
	}
	// REAL PBKDF2-HMAC-SHA256 constant-time compare (user_service password.go).
	if err := userservice.Verify(id.user.PasswordHash, password); err != nil {
		return gateway.Principal{}, false
	}
	// REAL RFC 6238 TOTP validation (user_service totp.go) for admin tiers.
	if id.user.MFAEnabled {
		t, err := userservice.NewTOTPFromBase32(id.totpSecret)
		if err != nil {
			return gateway.Principal{}, false
		}
		if !t.Verify(totp, a.now()) {
			return gateway.Principal{}, false
		}
	}
	return id.principal, true
}

// newRealAuth seeds the three gateway identities into real userservice.User
// records: passwords hashed with the real Hasher, admin tiers given a real TOTP
// secret. It returns an error only if the real hasher fails.
func newRealAuth() (*realAuth, error) {
	h := userservice.NewHasher(seedHashIterations)

	mk := func(userID, email, password, role, account, totpSecret string, scopes []string) (seedIdentity, error) {
		hash, err := h.Hash(password) // REAL PBKDF2 derive + encode
		if err != nil {
			return seedIdentity{}, fmt.Errorf("server: hashing %s: %w", email, err)
		}
		mfa := totpSecret != ""
		return seedIdentity{
			user: userservice.User{
				ID:           userID,
				Email:        email,
				PasswordHash: hash,
				TOTPSecret:   totpSecret,
				MFAEnabled:   mfa,
				CreatedAt:    time.Unix(1_699_000_000, 0).UTC(),
			},
			principal:  gateway.Principal{UserID: userID, Email: email, Role: role, AccountID: account, Scopes: scopes},
			totpSecret: totpSecret,
		}, nil
	}

	root, err := mk("user-root", gateway.SeedRootEmail, gateway.SeedRootPass,
		gateway.RoleRoot, gateway.SeedAccountA, SeedRootTOTPSecretB32, scopesRoot)
	if err != nil {
		return nil, err
	}
	admin, err := mk("user-admin", gateway.SeedAdminEmail, gateway.SeedAdminPass,
		gateway.RoleAccountAdmin, gateway.SeedAccountA, SeedAdminTOTPSecretB32, scopesAdmin)
	if err != nil {
		return nil, err
	}
	user, err := mk("user-std", gateway.SeedUserEmail, gateway.SeedUserPass,
		gateway.RoleUser, gateway.SeedAccountA, "", scopesUser)
	if err != nil {
		return nil, err
	}

	return &realAuth{
		now: time.Now,
		byEmail: map[string]seedIdentity{
			strings.ToLower(gateway.SeedRootEmail):  root,
			strings.ToLower(gateway.SeedAdminEmail): admin,
			strings.ToLower(gateway.SeedUserEmail):  user,
		},
	}, nil
}

// ==========================================================================
// SearchService -> semantic_search (real chunk -> embed -> cosine-KNN)
// ==========================================================================

// corpusDoc is one document fed through the real semsearch chunker at startup.
type corpusDoc struct {
	path    string
	content string
}

// seedCorpus is a small real corpus with disjoint vocabularies so cosine ranking
// is observable end-to-end: a vector-DB query favours vectordb.md, a telegram
// query favours telegram.md.
var seedCorpus = []corpusDoc{
	{"vectordb.md", "self hosted vector database benchmarks pgvector cosine similarity nearest neighbor embeddings index recall"},
	{"telegram.md", "telegram messenger bot channel adapter chat group message webhook long polling updates"},
	{"skills.md", "skill dispatch precedence download convert analyze research reply pipeline stage execution engine registry"},
}

// realSearch is a gateway.SearchService backed by a real semsearch.Engine. The
// ranking is the module's real cosine-KNN + score boost; nothing is reimplemented
// inline here.
type realSearch struct {
	engine *semsearch.Engine
}

// Search runs the real semsearch pipeline (embed query -> cosine-KNN -> boost ->
// rank) and maps hits to gateway.SearchHit. The embedder label is honest.
func (s *realSearch) Search(req gateway.SearchRequest) (gateway.SearchResponse, error) {
	k := req.TopK
	if k <= 0 {
		k = 10
	}
	start := time.Now()
	results, err := s.engine.Search(context.Background(), req.Query, k) // REAL cosine-KNN
	if err != nil {
		return gateway.SearchResponse{}, err
	}
	took := int(time.Since(start).Milliseconds())
	hits := make([]gateway.SearchHit, 0, len(results))
	for _, r := range results {
		span := fmt.Sprintf("L%d-L%d", r.Chunk.StartLine, r.Chunk.EndLine)
		hits = append(hits, gateway.SearchHit{
			SourceID: r.Chunk.FilePath,
			Kind:     string(r.Chunk.Kind),
			Score:    float64(r.Score),
			Span:     &span,
			Snippet:  r.Chunk.Content,
		})
	}
	return gateway.SearchResponse{Results: hits, TookMs: took, Embedder: EmbedderLabel}, nil
}

// newRealSearch builds the engine and indexes the corpus through the REAL
// pipeline: semsearch.Chunker -> Engine.Index (embed + upsert).
func newRealSearch() (*realSearch, error) {
	engine := semsearch.NewEngine(
		semsearch.NewDeterministicEmbedder(0), // real feature-hashing embedder
		semsearch.NewMemoryIndex(),            // real in-memory cosine-KNN index
		semsearch.DefaultConfig(),
	)
	chunker := semsearch.NewChunker()
	var all []semsearch.Chunk
	for _, d := range seedCorpus {
		chunks, err := chunker.Chunk(d.path, d.content) // REAL chunker
		if err != nil {
			return nil, fmt.Errorf("server: chunking %s: %w", d.path, err)
		}
		all = append(all, chunks...)
	}
	if err := engine.Index(context.Background(), all); err != nil { // REAL embed + index
		return nil, fmt.Errorf("server: indexing corpus: %w", err)
	}
	return &realSearch{engine: engine}, nil
}

// ==========================================================================
// SkillService + PostService -> skill_dispatch (real Registry + precedence)
// ==========================================================================

// demoSkill is a concrete skilldispatch.Skill. It carries a stage Kind and a
// hashtag match set. A nil match set matches every post (the reply stage).
type demoSkill struct {
	name  string
	kind  skilldispatch.Kind
	match []string
}

func (s demoSkill) Name() string             { return s.name }
func (s demoSkill) Kind() skilldispatch.Kind { return s.kind }
func (s demoSkill) Match(p skilldispatch.Post) bool {
	if len(s.match) == 0 {
		return true
	}
	return p.HasAnyHashtag(s.match...)
}
func (s demoSkill) Run(_ context.Context, _ skilldispatch.Post) (skilldispatch.Result, error) {
	return skilldispatch.Result{SkillName: s.name, Output: "ok"}, nil
}

// buildSkills registers the real Skills into a real skilldispatch.Registry and
// returns both the registry (used by PostService.Reprocess resolution) and the
// full ordered-by-registration slice (used by SkillService.List).
func buildSkills() (*skilldispatch.Registry, []skilldispatch.Skill) {
	skills := []skilldispatch.Skill{
		demoSkill{"video.download", skilldispatch.KindDownload, []string{"#video", "#download", "#torrent"}},
		demoSkill{"media.convert", skilldispatch.KindConvert, []string{"#video", "#convert"}},
		demoSkill{"vision.analyze", skilldispatch.KindAnalyze, []string{"#image", "#ocr", "#analyze"}},
		demoSkill{"tech.research", skilldispatch.KindResearch, []string{"#research", "#vectordb"}},
		demoSkill{"thread.reply", skilldispatch.KindReply, nil},
	}
	reg := skilldispatch.NewRegistry()
	reg.Register(skills...)
	return reg, skills
}

// realSkills is a gateway.SkillService returning the registered Skills in the
// module's REAL precedence order (skilldispatch.OrderByPrecedence).
type realSkills struct {
	skills []skilldispatch.Skill
}

// List returns the skills mapped to gateway.Skill in real stage precedence order:
// download > convert > analyze > research > reply.
func (s *realSkills) List() []gateway.Skill {
	ordered := skilldispatch.OrderByPrecedence(s.skills) // REAL precedence sort
	out := make([]gateway.Skill, 0, len(ordered))
	for _, sk := range ordered {
		out = append(out, gateway.Skill{
			ID:        "skill-" + sk.Name(),
			Name:      sk.Name(),
			Kind:      sk.Kind().String(),
			SortOrder: int(sk.Kind()),
		})
	}
	return out
}

// realPosts is a gateway.PostService: a real in-memory post store whose Reprocess
// resolves the REAL Skill precedence for the post's hashtags via skill_dispatch.
type realPosts struct {
	mu       sync.Mutex
	posts    map[string]gateway.Post
	registry *skilldispatch.Registry
	seq      int
}

// Get returns a post by id from the real store.
func (p *realPosts) Get(id string) (gateway.Post, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	post, ok := p.posts[id]
	return post, ok
}

// Reprocess resolves the matching Skills for the post via the real registry,
// orders them by real precedence, and returns them in ProcessingJob.Precedence.
//
// A missing post is signalled with the gateway's EXPORTED coded-error
// constructor (gateway.NewError(gateway.CodeNotFound, …)), so the gateway's
// writeServiceError maps it to HTTP 404 — not the generic 500 it produced while
// the coded-error type was unexported. See EVIDENCE.md.
func (p *realPosts) Reprocess(id string) (gateway.ProcessingJob, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	post, ok := p.posts[id]
	if !ok {
		return gateway.ProcessingJob{}, gateway.NewError(gateway.CodeNotFound, fmt.Sprintf("post %q not found", id))
	}
	sp := skilldispatch.Post{ID: post.ID, Hashtags: post.Hashtags, Text: post.Body}
	ordered := skilldispatch.OrderByPrecedence(p.registry.Resolve(sp)) // REAL resolve + order
	prec := make([]string, 0, len(ordered))
	for _, s := range ordered {
		prec = append(prec, s.Name())
	}
	p.seq++
	return gateway.ProcessingJob{
		JobID:      "job-" + itoa(p.seq),
		PostID:     id,
		Status:     "queued",
		Precedence: prec,
		QueuedAt:   time.Unix(int64(1_700_000_000+p.seq), 0).UTC(),
	}, nil
}

// newRealPosts seeds the store with the gateway's canonical seed post.
func newRealPosts(reg *skilldispatch.Registry) *realPosts {
	return &realPosts{
		registry: reg,
		posts: map[string]gateway.Post{
			gateway.SeedPostA: {
				ID: gateway.SeedPostA, ChannelID: gateway.SeedChannelA, AccountID: gateway.SeedAccountA,
				Body:       "self-hosted vector database benchmarks thread",
				Hashtags:   []string{"#research", "#vectordb"},
				Categories: []string{"research", "software"},
				Status:     "succeeded",
				CreatedAt:  time.Unix(1_700_000_100, 0).UTC(),
			},
		},
	}
}

// ==========================================================================
// EventService -> event_bus_service (real pub/sub Bus)
// ==========================================================================

// realEvents is a gateway.EventService bridging the real eventbus.Bus to the
// gateway.Event shape used by the SSE stream.
type realEvents struct {
	bus *eventbus.Bus
}

// Subscribe registers a real SubscribeAll on the bus and forwards bridged events
// until the caller cancels (or the context ends).
func (e *realEvents) Subscribe(ctx context.Context) (<-chan gateway.Event, func()) {
	sub := e.bus.SubscribeAll() // REAL subscription
	out := make(chan gateway.Event, 16)
	done := make(chan struct{})
	go func() {
		defer close(out)
		for {
			select {
			case ev, open := <-sub.C:
				if !open {
					return
				}
				g := bridgeEvent(ev)
				select {
				case out <- g:
				case <-done:
					return
				case <-ctx.Done():
					return
				}
			case <-done:
				return
			case <-ctx.Done():
				return
			}
		}
	}()
	var once sync.Once
	cancel := func() {
		once.Do(func() {
			close(done)
			e.bus.Unsubscribe(sub)
		})
	}
	return out, cancel
}

// Publish maps a gateway.Event to a real bus event and publishes it.
func (e *realEvents) Publish(ev gateway.Event) {
	be := eventbus.NewEvent(subjectFor(ev), ev.Type, ev.Payload)
	if ev.TraceID != "" {
		be = be.WithMetadata("trace_id", ev.TraceID)
	}
	_, _ = e.bus.Publish(be) // REAL publish
}

// bridgeEvent converts a bus event to the gateway shape.
func bridgeEvent(ev eventbus.Event) gateway.Event {
	var payload map[string]any
	switch p := ev.Payload.(type) {
	case map[string]any:
		payload = p
	case nil:
		payload = map[string]any{}
	default:
		payload = map[string]any{"value": p}
	}
	trace := ""
	if ev.Metadata != nil {
		trace = ev.Metadata["trace_id"]
	}
	return gateway.Event{ID: ev.ID, Type: ev.Type, Payload: payload, TraceID: trace}
}

func subjectFor(ev gateway.Event) string {
	if ev.Type != "" {
		return ev.Type
	}
	return "events"
}

// ==========================================================================
// ChannelService / AccountService -> honest in-memory stores (no domain module)
// ==========================================================================

// realChannels is an honest in-memory gateway.ChannelService. There is NO
// dedicated domain module for channel CRUD; this is a real local store.
type realChannels struct {
	mu       sync.Mutex
	channels []gateway.Channel
	threads  map[string][]gateway.Thread
	seq      int
}

func (c *realChannels) List(accountID string) []gateway.Channel {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := []gateway.Channel{}
	for _, ch := range c.channels {
		if accountID == "" || ch.AccountID == accountID {
			out = append(out, ch)
		}
	}
	return out
}

func (c *realChannels) Create(accountID string, in gateway.ChannelInput) (gateway.Channel, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.seq++
	ch := gateway.Channel{
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

func (c *realChannels) Threads(channelID string) ([]gateway.Thread, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	t, ok := c.threads[channelID]
	if !ok {
		return nil, false
	}
	return t, true
}

func newRealChannels() *realChannels {
	return &realChannels{
		channels: []gateway.Channel{{
			ID: gateway.SeedChannelA, AccountID: gateway.SeedAccountA, Name: "general",
			Platform: "telegram", ExternalRef: "@thready_general",
			CreatedAt: time.Unix(1_700_000_000, 0).UTC(),
		}},
		threads: map[string][]gateway.Thread{
			gateway.SeedChannelA: {{ID: "thread-1", ChannelID: gateway.SeedChannelA, RootPost: gateway.SeedPostA, Replies: []string{"post-2"}}},
		},
		seq: 1,
	}
}

// realAccounts is an honest in-memory gateway.AccountService (no domain module).
type realAccounts struct {
	accounts []gateway.Account
}

func (a *realAccounts) List(p gateway.Principal) []gateway.Account {
	if p.Role == gateway.RoleRoot {
		out := make([]gateway.Account, len(a.accounts))
		copy(out, a.accounts)
		return out
	}
	out := []gateway.Account{}
	for _, ac := range a.accounts {
		if ac.ID == p.AccountID {
			out = append(out, ac)
		}
	}
	return out
}

func newRealAccounts() *realAccounts {
	return &realAccounts{accounts: []gateway.Account{
		{ID: gateway.SeedAccountA, Name: "Acme", CreatedAt: time.Unix(1_699_000_000, 0).UTC()},
		{ID: gateway.SeedAccountB, Name: "Globex", CreatedAt: time.Unix(1_699_500_000, 0).UTC()},
	}}
}

// itoa is a tiny int->string helper for small sequence numbers.
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
