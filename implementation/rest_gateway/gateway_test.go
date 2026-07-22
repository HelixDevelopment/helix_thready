package gateway

import (
	"bufio"
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// newTestServer builds a Server backed by freshly-seeded in-memory services and
// a deterministic HMAC signer. It returns the server plus concrete handles to
// the event bus and channel store so tests can publish events / inspect side
// effects.
func newTestServer(t *testing.T) (*Server, *InMemoryEvents, *InMemoryChannels) {
	t.Helper()
	signer, err := NewSigner(SignerConfig{Secret: []byte("test-secret-key-please-change-32b")})
	if err != nil {
		t.Fatalf("NewSigner: %v", err)
	}
	svc := NewInMemoryServices()
	srv := New(svc, signer)
	events := svc.Events.(*InMemoryEvents)
	channels := svc.Channels.(*InMemoryChannels)
	return srv, events, channels
}

// errBody mirrors the canonical error envelope for assertions.
type errBody struct {
	Error struct {
		Code      string `json:"code"`
		Message   string `json:"message"`
		RequestID string `json:"request_id"`
		Status    int    `json:"status"`
		TraceID   string `json:"trace_id"`
	} `json:"error"`
}

// do performs an in-process request against the server.
func do(t *testing.T, srv *Server, method, path, token string, body any, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	var rdr io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal body: %v", err)
		}
		rdr = bytes.NewReader(b)
	}
	req := httptest.NewRequest(method, path, rdr)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	return rec
}

// login authenticates and returns the access token.
func login(t *testing.T, srv *Server, email, pass, totp string) string {
	t.Helper()
	rec := do(t, srv, http.MethodPost, "/v1/auth/login", "", loginRequest{Email: email, Password: pass, TOTP: totp}, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("login %s: want 200, got %d: %s", email, rec.Code, rec.Body.String())
	}
	var tp tokenPair
	if err := json.Unmarshal(rec.Body.Bytes(), &tp); err != nil {
		t.Fatalf("login decode: %v", err)
	}
	if tp.AccessToken == "" {
		t.Fatalf("login %s: empty access token", email)
	}
	return tp.AccessToken
}

// --- 1. Unauthenticated protected route -> 401 ---

func TestProtectedRouteRequiresAuth(t *testing.T) {
	srv, _, _ := newTestServer(t)
	rec := do(t, srv, http.MethodGet, "/v1/accounts", "", nil, nil)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d: %s", rec.Code, rec.Body.String())
	}
	var eb errBody
	if err := json.Unmarshal(rec.Body.Bytes(), &eb); err != nil {
		t.Fatalf("decode error body: %v", err)
	}
	if eb.Error.Code != string(CodeUnauthenticated) {
		t.Fatalf("want code %q, got %q", CodeUnauthenticated, eb.Error.Code)
	}
	if eb.Error.RequestID == "" {
		t.Fatalf("error envelope missing request_id")
	}
}

// --- 2. Login valid -> 200 + token; invalid -> 401 ---

func TestLogin(t *testing.T) {
	srv, _, _ := newTestServer(t)

	// Valid (admin tier requires TOTP).
	rec := do(t, srv, http.MethodPost, "/v1/auth/login", "",
		loginRequest{Email: SeedRootEmail, Password: SeedRootPass, TOTP: SeedRootTOTP}, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("valid login: want 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var tp tokenPair
	if err := json.Unmarshal(rec.Body.Bytes(), &tp); err != nil {
		t.Fatalf("decode token pair: %v", err)
	}
	if tp.AccessToken == "" || tp.RefreshToken == "" {
		t.Fatalf("missing tokens: %+v", tp)
	}
	if tp.TokenType != "Bearer" || tp.ExpiresIn != 900 {
		t.Fatalf("unexpected token metadata: %+v", tp)
	}

	// Invalid password.
	rec = do(t, srv, http.MethodPost, "/v1/auth/login", "",
		loginRequest{Email: SeedRootEmail, Password: "wrong-password", TOTP: SeedRootTOTP}, nil)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("invalid login: want 401, got %d: %s", rec.Code, rec.Body.String())
	}

	// Missing admin TOTP.
	rec = do(t, srv, http.MethodPost, "/v1/auth/login", "",
		loginRequest{Email: SeedRootEmail, Password: SeedRootPass}, nil)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("missing totp: want 401, got %d: %s", rec.Code, rec.Body.String())
	}
}

// --- 3. Authorized GET with token -> 200 + expected body ---

func TestAuthorizedGetPost(t *testing.T) {
	srv, _, _ := newTestServer(t)
	token := login(t, srv, SeedUserEmail, SeedUserPass, "")
	rec := do(t, srv, http.MethodGet, "/v1/posts/"+SeedPostA, token, nil, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var post Post
	if err := json.Unmarshal(rec.Body.Bytes(), &post); err != nil {
		t.Fatalf("decode post: %v", err)
	}
	if post.ID != SeedPostA {
		t.Fatalf("want post id %q, got %q", SeedPostA, post.ID)
	}
	if post.Status != "succeeded" {
		t.Fatalf("unexpected post status %q", post.Status)
	}
}

// --- 4. RBAC: user -> 403, root -> 200 on /v1/accounts ---

func TestRBACAccounts(t *testing.T) {
	srv, _, _ := newTestServer(t)

	userTok := login(t, srv, SeedUserEmail, SeedUserPass, "")
	rec := do(t, srv, http.MethodGet, "/v1/accounts", userTok, nil, nil)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("user on /accounts: want 403, got %d: %s", rec.Code, rec.Body.String())
	}
	var eb errBody
	_ = json.Unmarshal(rec.Body.Bytes(), &eb)
	if eb.Error.Code != string(CodePermissionDenied) {
		t.Fatalf("want permission_denied, got %q", eb.Error.Code)
	}

	rootTok := login(t, srv, SeedRootEmail, SeedRootPass, SeedRootTOTP)
	rec = do(t, srv, http.MethodGet, "/v1/accounts", rootTok, nil, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("root on /accounts: want 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var lst struct {
		Data []Account `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &lst); err != nil {
		t.Fatalf("decode accounts: %v", err)
	}
	if len(lst.Data) == 0 {
		t.Fatalf("root should see accounts, got none")
	}
}

// --- 5. Reprocess -> 202 with a job/status body ---

func TestReprocessAccepted(t *testing.T) {
	srv, _, _ := newTestServer(t)
	adminTok := login(t, srv, SeedAdminEmail, SeedAdminPass, SeedAdminTOTP)
	rec := do(t, srv, http.MethodPost, "/v1/posts/"+SeedPostA+"/reprocess", adminTok, nil, nil)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("want 202, got %d: %s", rec.Code, rec.Body.String())
	}
	var job ProcessingJob
	if err := json.Unmarshal(rec.Body.Bytes(), &job); err != nil {
		t.Fatalf("decode job: %v", err)
	}
	if job.JobID == "" || job.Status != "queued" || job.PostID != SeedPostA {
		t.Fatalf("unexpected job body: %+v", job)
	}

	// Missing post -> 404.
	rec = do(t, srv, http.MethodPost, "/v1/posts/does-not-exist/reprocess", adminTok, nil, nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("reprocess missing post: want 404, got %d", rec.Code)
	}
}

// --- 6. Search -> 200 ranked results ---

func TestSearchRanked(t *testing.T) {
	srv, _, _ := newTestServer(t)
	token := login(t, srv, SeedUserEmail, SeedUserPass, "")
	rec := do(t, srv, http.MethodPost, "/v1/search", token,
		SearchRequest{Query: "vector database", Mode: "hybrid", TopK: 10}, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var res SearchResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
		t.Fatalf("decode search: %v", err)
	}
	if len(res.Results) == 0 {
		t.Fatalf("expected ranked results, got none")
	}
	if res.Embedder != "llama" {
		t.Fatalf("expected real llama embedder, got %q", res.Embedder)
	}
	for i := 1; i < len(res.Results); i++ {
		if res.Results[i-1].Score < res.Results[i].Score {
			t.Fatalf("results not sorted by score desc: %v", res.Results)
		}
	}
	// Top hit should be the post that mentions the query terms.
	if res.Results[0].SourceID != SeedPostA {
		t.Fatalf("expected top hit %q, got %q", SeedPostA, res.Results[0].SourceID)
	}

	// Invalid top_k -> 400 invalid_argument.
	rec = do(t, srv, http.MethodPost, "/v1/search", token,
		SearchRequest{Query: "x", TopK: 500}, nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("top_k out of range: want 400, got %d", rec.Code)
	}
}

// --- 7. SSE stream receives a published event line ---

func TestSSEStream(t *testing.T) {
	srv, events, _ := newTestServer(t)
	token := login(t, srv, SeedUserEmail, SeedUserPass, "")

	ts := httptest.NewServer(srv)
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/v1/events", nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("SSE: want 200, got %d", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/event-stream") {
		t.Fatalf("SSE: unexpected content-type %q", ct)
	}

	reader := bufio.NewReader(resp.Body)

	// Wait for the initial subscription comment so the subscriber is registered.
	line, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("read subscription comment: %v", err)
	}
	if !strings.HasPrefix(line, ": subscribed") {
		t.Fatalf("expected subscription comment, got %q", line)
	}

	// Publish an event and expect a data: line carrying it.
	events.Publish(Event{
		ID:      "evt-1",
		Type:    "processing.completed",
		Payload: map[string]any{"post_id": SeedPostA},
		TraceID: "trace-1",
	})

	dataCh := make(chan string, 1)
	errCh := make(chan error, 1)
	go func() {
		for {
			l, err := reader.ReadString('\n')
			if err != nil {
				errCh <- err
				return
			}
			if strings.HasPrefix(l, "data:") {
				dataCh <- strings.TrimSpace(strings.TrimPrefix(l, "data:"))
				return
			}
		}
	}()

	select {
	case data := <-dataCh:
		var ev Event
		if err := json.Unmarshal([]byte(data), &ev); err != nil {
			t.Fatalf("decode SSE data line %q: %v", data, err)
		}
		if ev.ID != "evt-1" || ev.Type != "processing.completed" {
			t.Fatalf("unexpected event: %+v", ev)
		}
	case err := <-errCh:
		t.Fatalf("reading SSE stream: %v", err)
	case <-time.After(3 * time.Second):
		t.Fatalf("timed out waiting for SSE data line")
	}
}

// --- 8. Error-model shape asserted ---

func TestErrorModelShape(t *testing.T) {
	srv, _, _ := newTestServer(t)
	rec := do(t, srv, http.MethodGet, "/v1/nope-not-a-route", "", nil, nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("want 404, got %d", rec.Code)
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(rec.Body.Bytes(), &raw); err != nil {
		t.Fatalf("error body is not JSON: %v", err)
	}
	inner, ok := raw["error"]
	if !ok {
		t.Fatalf("error body missing top-level 'error' key: %s", rec.Body.String())
	}
	var env map[string]json.RawMessage
	if err := json.Unmarshal(inner, &env); err != nil {
		t.Fatalf("error envelope is not an object: %v", err)
	}
	for _, field := range []string{"code", "message", "request_id"} {
		if _, ok := env[field]; !ok {
			t.Fatalf("error envelope missing %q: %s", field, rec.Body.String())
		}
	}
}

// --- 9. Idempotency-Key replay returns same result without duplicate side effect ---

func TestIdempotencyReplay(t *testing.T) {
	srv, _, channels := newTestServer(t)
	adminTok := login(t, srv, SeedAdminEmail, SeedAdminPass, SeedAdminTOTP)

	before := channels.count()
	key := "11111111-1111-4111-8111-111111111111"
	body := ChannelInput{Name: "ops", Platform: "telegram", ExternalRef: "@ops"}
	hdr := map[string]string{"Idempotency-Key": key}

	rec1 := do(t, srv, http.MethodPost, "/v1/channels", adminTok, body, hdr)
	if rec1.Code != http.StatusCreated {
		t.Fatalf("first create: want 201, got %d: %s", rec1.Code, rec1.Body.String())
	}

	rec2 := do(t, srv, http.MethodPost, "/v1/channels", adminTok, body, hdr)
	if rec2.Code != http.StatusCreated {
		t.Fatalf("replay: want 201, got %d: %s", rec2.Code, rec2.Body.String())
	}
	if rec1.Body.String() != rec2.Body.String() {
		t.Fatalf("replay body differs:\n first=%s\n replay=%s", rec1.Body.String(), rec2.Body.String())
	}
	if rec2.Header().Get("Idempotency-Replayed") != "true" {
		t.Fatalf("replay should be flagged Idempotency-Replayed:true")
	}
	if got := channels.count(); got != before+1 {
		t.Fatalf("duplicate side effect: channel count went from %d to %d (want +1)", before, got)
	}

	// Same key, different body -> 409 conflict.
	rec3 := do(t, srv, http.MethodPost, "/v1/channels", adminTok,
		ChannelInput{Name: "different", Platform: "telegram", ExternalRef: "@x"}, hdr)
	if rec3.Code != http.StatusConflict {
		t.Fatalf("same-key different-body: want 409, got %d: %s", rec3.Code, rec3.Body.String())
	}
	if got := channels.count(); got != before+1 {
		t.Fatalf("409 path must not create a channel: count=%d want=%d", got, before+1)
	}
}

// --- 10. 404 on unknown route ---

func TestUnknownRoute404(t *testing.T) {
	srv, _, _ := newTestServer(t)
	rec := do(t, srv, http.MethodGet, "/v1/does/not/exist", "", nil, nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("want 404, got %d: %s", rec.Code, rec.Body.String())
	}
	var eb errBody
	if err := json.Unmarshal(rec.Body.Bytes(), &eb); err != nil {
		t.Fatalf("decode 404 body: %v", err)
	}
	if eb.Error.Code != string(CodeNotFound) {
		t.Fatalf("want not_found, got %q", eb.Error.Code)
	}
}

// --- bonus: algorithm-confusion / tampered token rejected ---

func TestTamperedTokenRejected(t *testing.T) {
	srv, _, _ := newTestServer(t)
	token := login(t, srv, SeedUserEmail, SeedUserPass, "")
	// Flip a character in the signature segment.
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		t.Fatalf("unexpected token shape")
	}
	tampered := parts[0] + "." + parts[1] + "." + parts[2] + "x"
	rec := do(t, srv, http.MethodGet, "/v1/posts/"+SeedPostA, tampered, nil, nil)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("tampered token: want 401, got %d", rec.Code)
	}
}

// ==========================================================================
// Coverage-fix additions (Constitution honesty gap): the behaviours below are
// implemented but were not asserted by the committed suite.
// ==========================================================================

// testSecret matches the HMAC secret newTestServer wires into its Signer, so the
// forged-token helpers below can produce byte-for-byte what an attacker (or the
// real Signer) would present.
const testSecret = "test-secret-key-please-change-32b"

// forgeToken hand-builds a compact JWT with an ATTACKER-CONTROLLED header. This
// is what lets the tests reach the algorithm pin at token.go:119 — the existing
// TestTamperedTokenRejected only corrupts the signature and therefore exits at
// the hmac.Equal mismatch, never evaluating `alg != "HS256"`.
//
// When sign is true the signature segment is a GENUINE HMAC-SHA256 over the
// signing input using the gateway's own secret: absent the alg pin, hmac.Equal
// would accept it. When sign is false the signature segment is left empty (the
// canonical alg:"none" form, still three dot-separated segments so it clears the
// len(parts)!=3 guard and actually exercises the alg check).
func forgeToken(t *testing.T, header map[string]any, claims Claims, sign bool) string {
	t.Helper()
	hj, err := json.Marshal(header)
	if err != nil {
		t.Fatalf("marshal forged header: %v", err)
	}
	cj, err := json.Marshal(claims)
	if err != nil {
		t.Fatalf("marshal forged claims: %v", err)
	}
	seg := b64(hj) + "." + b64(cj)
	if !sign {
		return seg + "." // empty signature -> three segments, alg:"none" form
	}
	m := hmac.New(sha256.New, []byte(testSecret))
	m.Write([]byte(seg))
	return seg + "." + b64(m.Sum(nil))
}

// mintToken signs a valid access token with an arbitrary role/account/scopes via
// the REAL Signer (same secret as newTestServer). Used to construct principals
// the seeded credential set does not offer — e.g. a role that clears the floor
// but is missing a required scope.
func mintToken(t *testing.T, role, accountID string, scopes []string) string {
	t.Helper()
	signer, err := NewSigner(SignerConfig{Secret: []byte(testSecret)})
	if err != nil {
		t.Fatalf("NewSigner: %v", err)
	}
	tok, err := signer.Sign(Claims{
		Sub: "user-craft", Role: role, AccountID: accountID,
		Scopes: scopes, TokenType: "access",
	}, time.Hour)
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}
	return tok
}

// validCraftedClaims returns a well-formed, unexpired access-token claim set for
// a standard acct-a user (used as the payload for the alg-pin forgeries).
func validCraftedClaims() Claims {
	now := time.Now()
	return Claims{
		Sub: "user-std", Role: RoleUser, AccountID: SeedAccountA,
		Scopes: scopesUser, TokenType: "access",
		Iat: now.Unix(), Exp: now.Add(time.Hour).Unix(),
	}
}

// --- 12. JWT alg pin: forged alg:"none" token -> 401 (exercises token.go:119) ---

func TestForgedAlgNoneRejected(t *testing.T) {
	srv, _, _ := newTestServer(t)
	forged := forgeToken(t, map[string]any{"alg": "none", "typ": "JWT"}, validCraftedClaims(), false)
	rec := do(t, srv, http.MethodGet, "/v1/posts/"+SeedPostA, forged, nil, nil)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("alg:none forged token: want 401, got %d: %s", rec.Code, rec.Body.String())
	}
	var eb errBody
	if err := json.Unmarshal(rec.Body.Bytes(), &eb); err != nil {
		t.Fatalf("decode error body: %v", err)
	}
	if eb.Error.Code != string(CodeUnauthenticated) {
		t.Fatalf("alg:none: want unauthenticated, got %q", eb.Error.Code)
	}
}

// --- 13. JWT alg pin: alg-swap (HS512) forgery -> 401; the pin is load-bearing ---

func TestForgedAlgSwapRejected(t *testing.T) {
	srv, _, _ := newTestServer(t)
	claims := validCraftedClaims()

	// Header says HS512 but the signature is a genuine HMAC-SHA256 over the
	// signing input with the real secret. Absent the alg pin, hmac.Equal would
	// accept it — so the pin at token.go:119 is the only thing rejecting it.
	swap := forgeToken(t, map[string]any{"alg": "HS512", "typ": "JWT"}, claims, true)
	rec := do(t, srv, http.MethodGet, "/v1/posts/"+SeedPostA, swap, nil, nil)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("alg-swap forged token: want 401, got %d: %s", rec.Code, rec.Body.String())
	}
	var eb errBody
	if err := json.Unmarshal(rec.Body.Bytes(), &eb); err != nil {
		t.Fatalf("decode error body: %v", err)
	}
	if eb.Error.Code != string(CodeUnauthenticated) {
		t.Fatalf("alg-swap: want unauthenticated, got %q", eb.Error.Code)
	}

	// Control: the IDENTICAL claims + signature, differing ONLY in the header's
	// alg ("HS256"), DO authenticate (200). This proves the signature is really
	// valid and that the alg field alone is what rejects the swap above.
	control := forgeToken(t, map[string]any{"alg": "HS256", "typ": "JWT"}, claims, true)
	rec = do(t, srv, http.MethodGet, "/v1/posts/"+SeedPostA, control, nil, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("control HS256 token should authenticate: got %d: %s", rec.Code, rec.Body.String())
	}
}

// --- 14. Scope-denied 403: sufficient ROLE but a missing required SCOPE ---

func TestScopeDenied403(t *testing.T) {
	srv, _, _ := newTestServer(t)
	// GET /v1/channels needs role floor `user` + scope `posts:read`. This token
	// clears the role floor (role=user) but lacks `posts:read`, so it reaches
	// requireScopes (middleware.go:207-228) and is denied there — a path every
	// existing 403 (all role-floor failures) never touches.
	tok := mintToken(t, RoleUser, SeedAccountA, []string{"skills:read"})
	rec := do(t, srv, http.MethodGet, "/v1/channels", tok, nil, nil)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("scope-denied: want 403, got %d: %s", rec.Code, rec.Body.String())
	}
	var eb errBody
	if err := json.Unmarshal(rec.Body.Bytes(), &eb); err != nil {
		t.Fatalf("decode error body: %v", err)
	}
	if eb.Error.Code != string(CodePermissionDenied) {
		t.Fatalf("scope-denied: want permission_denied, got %q", eb.Error.Code)
	}
	if !strings.Contains(eb.Error.Message, "missing required scope") || !strings.Contains(eb.Error.Message, "posts:read") {
		t.Fatalf("scope-denied: message should name the missing scope posts:read, got %q", eb.Error.Message)
	}

	// Sanity: the SAME role WITH the scope is admitted (200) — isolating the
	// denial to the missing scope, not the role floor.
	ok := mintToken(t, RoleUser, SeedAccountA, []string{"posts:read"})
	rec = do(t, srv, http.MethodGet, "/v1/channels", ok, nil, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("role+scope should pass: want 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

// --- 15. Panic recovery: a panicking handler -> 500 envelope, not a crash ---

func TestPanicRecovery(t *testing.T) {
	srv, _, _ := newTestServer(t)
	panicky := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("boom: induced handler panic")
	})
	// Wrap the panicking handler in the SAME global chain the router uses
	// (request-id -> access log -> panic recovery) so recoverPanic
	// (middleware.go:136-149) runs with a request_id already stamped.
	h := chain(panicky, srv.requestID, srv.accessLog, srv.recoverPanic)

	req := httptest.NewRequest(http.MethodGet, "/v1/induce-panic", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req) // must NOT propagate the panic

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("panic recovery: want 500, got %d: %s", rec.Code, rec.Body.String())
	}
	var eb errBody
	if err := json.Unmarshal(rec.Body.Bytes(), &eb); err != nil {
		t.Fatalf("panic recovery: body is not a JSON error envelope: %v (%s)", err, rec.Body.String())
	}
	if eb.Error.Code != string(CodeInternal) {
		t.Fatalf("panic recovery: want internal, got %q", eb.Error.Code)
	}
	if eb.Error.RequestID == "" {
		t.Fatalf("panic recovery: error envelope missing request_id")
	}
	if rec.Header().Get("X-Request-Id") == "" {
		t.Fatalf("panic recovery: response missing X-Request-Id header")
	}
}

// --- 16. GET /v1/health -> 200 + expected shape ---

func TestHealth(t *testing.T) {
	srv, _, _ := newTestServer(t)
	rec := do(t, srv, http.MethodGet, "/v1/health", "", nil, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("health: want 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Fatalf("health content-type: want application/json, got %q", ct)
	}
	var body struct {
		Status  string `json:"status"`
		Service string `json:"service"`
		Version string `json:"version"`
		Time    string `json:"time"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode health: %v", err)
	}
	if body.Status != "ok" {
		t.Fatalf("health status: want ok, got %q", body.Status)
	}
	if body.Service != "rest-gateway" {
		t.Fatalf("health service: want rest-gateway, got %q", body.Service)
	}
	if body.Version != "v1" {
		t.Fatalf("health version: want v1, got %q", body.Version)
	}
	if body.Time == "" {
		t.Fatalf("health: time field must be present")
	}
}

// --- 17. GET /v1/channels: root-sees-all vs per-tenant filtering ---

func TestListChannelsTenantFiltering(t *testing.T) {
	srv, _, _ := newTestServer(t)

	list := func(tok string) []Channel {
		t.Helper()
		rec := do(t, srv, http.MethodGet, "/v1/channels", tok, nil, nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("list channels: want 200, got %d: %s", rec.Code, rec.Body.String())
		}
		var env struct {
			Data []Channel `json:"data"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
			t.Fatalf("decode channel list: %v", err)
		}
		return env.Data
	}

	// Seed a second channel in account B (via a crafted acct-b admin token) so
	// root-sees-all is observably different from a single-tenant view.
	adminB := mintToken(t, RoleAccountAdmin, SeedAccountB, scopesAdmin)
	recCreate := do(t, srv, http.MethodPost, "/v1/channels", adminB,
		ChannelInput{Name: "b-ops", Platform: "telegram", ExternalRef: "@b_ops"}, nil)
	if recCreate.Code != http.StatusCreated {
		t.Fatalf("seed acct-b channel: want 201, got %d: %s", recCreate.Code, recCreate.Body.String())
	}

	// Root sees every tenant's channels (acct-a seed + the acct-b one above).
	rootChannels := list(login(t, srv, SeedRootEmail, SeedRootPass, SeedRootTOTP))
	if len(rootChannels) < 2 {
		t.Fatalf("root should see all tenants' channels (>=2), got %d: %+v", len(rootChannels), rootChannels)
	}

	// Account-A user: only acct-a channels, never acct-b's.
	aChannels := list(login(t, srv, SeedUserEmail, SeedUserPass, ""))
	if len(aChannels) == 0 {
		t.Fatalf("acct-a user should see acct-a channels, got none")
	}
	for _, ch := range aChannels {
		if ch.AccountID != SeedAccountA {
			t.Fatalf("acct-a user saw a foreign-tenant channel: %+v", ch)
		}
	}

	// Account-B user: only the acct-b channel; the acct-a seed is filtered out.
	bChannels := list(mintToken(t, RoleUser, SeedAccountB, scopesUser))
	if len(bChannels) == 0 {
		t.Fatalf("acct-b user should see the acct-b channel, got none")
	}
	for _, ch := range bChannels {
		if ch.AccountID != SeedAccountB {
			t.Fatalf("acct-b user saw a foreign-tenant channel: %+v", ch)
		}
	}

	// Root's full view must exceed each single-tenant view.
	if len(rootChannels) <= len(aChannels) {
		t.Fatalf("root view (%d) should exceed acct-a view (%d)", len(rootChannels), len(aChannels))
	}
	if len(rootChannels) <= len(bChannels) {
		t.Fatalf("root view (%d) should exceed acct-b view (%d)", len(rootChannels), len(bChannels))
	}
}

// --- 18. GET /v1/channels/{id}/threads -> 200 + body (and 404 for unknown) ---

func TestChannelThreads(t *testing.T) {
	srv, _, _ := newTestServer(t)
	tok := login(t, srv, SeedUserEmail, SeedUserPass, "")

	rec := do(t, srv, http.MethodGet, "/v1/channels/"+SeedChannelA+"/threads", tok, nil, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("channel threads: want 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var env struct {
		Data []Thread `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("decode threads: %v", err)
	}
	if len(env.Data) == 0 {
		t.Fatalf("expected at least one thread for %s, got none", SeedChannelA)
	}
	if th := env.Data[0]; th.ChannelID != SeedChannelA || th.RootPost != SeedPostA {
		t.Fatalf("unexpected thread body: %+v", th)
	}

	// Unknown channel -> 404 not_found.
	rec = do(t, srv, http.MethodGet, "/v1/channels/does-not-exist/threads", tok, nil, nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("threads for unknown channel: want 404, got %d: %s", rec.Code, rec.Body.String())
	}
}

// --- 19. GET /v1/skills -> 200 + body in precedence order ---

func TestListSkills(t *testing.T) {
	srv, _, _ := newTestServer(t)
	tok := login(t, srv, SeedUserEmail, SeedUserPass, "")
	rec := do(t, srv, http.MethodGet, "/v1/skills", tok, nil, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("list skills: want 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var env struct {
		Data []Skill `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("decode skills: %v", err)
	}
	if len(env.Data) == 0 {
		t.Fatalf("expected seeded skills, got none")
	}
	// List returns skills in dispatch-precedence order (SortOrder ascending).
	for i := 1; i < len(env.Data); i++ {
		if env.Data[i-1].SortOrder > env.Data[i].SortOrder {
			t.Fatalf("skills not in precedence order: %+v", env.Data)
		}
	}
}
