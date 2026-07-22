package thready

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// The SDK is a *client*; the honest unit-test approach is to exercise it against
// a net/http/httptest server that mocks the gateway's `/v1` contract (methods,
// paths, headers, and the exact wire shapes the real rest_gateway emits) — NOT
// against a live gateway. Each test asserts the request the SDK sends and the
// typed value it decodes back.

// newTestClient builds a Client pointed at srv with fast backoff so retry tests
// stay quick under -race.
func newTestClient(t *testing.T, srv *httptest.Server, cfg Config) *Client {
	t.Helper()
	if cfg.BaseURL == "" {
		cfg.BaseURL = srv.URL
	}
	if cfg.HTTPClient == nil {
		cfg.HTTPClient = srv.Client()
	}
	c, err := New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	c.backoffBase = time.Millisecond // keep retry sleeps tiny in tests
	c.backoffMax = 5 * time.Millisecond
	return c
}

// writeErrorEnvelope emits the gateway's canonical failure envelope.
func writeErrorEnvelope(w http.ResponseWriter, status int, code Code, message, requestID string) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Request-Id", requestID)
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"error": map[string]any{
			"code":       string(code),
			"message":    message,
			"status":     status,
			"request_id": requestID,
			"trace_id":   requestID,
		},
	})
}

func TestLogin_SendsCredentialsAndSubsequentCallUsesToken(t *testing.T) {
	const wantToken = "jwt-access-abc123"
	var seenAuthOnChannels string

	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/auth/login", func(w http.ResponseWriter, r *http.Request) {
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("login Content-Type = %q, want application/json", ct)
		}
		var req LoginRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode login body: %v", err)
		}
		if req.Email != "user@thready.test" || req.Password != "userpassword-123" {
			t.Errorf("unexpected credentials: %+v", req)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(TokenPair{
			AccessToken: wantToken, RefreshToken: "jwt-refresh", TokenType: "Bearer",
			ExpiresIn: 900, RefreshExpiresIn: 604800,
		})
	})
	mux.HandleFunc("GET /v1/channels", func(w http.ResponseWriter, r *http.Request) {
		seenAuthOnChannels = r.Header.Get("Authorization")
		_ = json.NewEncoder(w).Encode(listEnvelope[Channel]{Data: []Channel{}})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c := newTestClient(t, srv, Config{})
	ctx := context.Background()

	tp, err := c.Login(ctx, LoginRequest{Email: "user@thready.test", Password: "userpassword-123"})
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if tp.AccessToken != wantToken {
		t.Fatalf("token = %q, want %q", tp.AccessToken, wantToken)
	}
	if got := c.AccessToken(); got != wantToken {
		t.Fatalf("client did not store token: %q", got)
	}

	if _, err := c.ListChannels(ctx); err != nil {
		t.Fatalf("ListChannels: %v", err)
	}
	if want := "Bearer " + wantToken; seenAuthOnChannels != want {
		t.Fatalf("subsequent call Authorization = %q, want %q", seenAuthOnChannels, want)
	}
}

func TestListChannels_InjectsBearerAndDecodesEnvelope(t *testing.T) {
	var seenAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/v1/channels" {
			t.Errorf("got %s %s, want GET /v1/channels", r.Method, r.URL.Path)
		}
		seenAuth = r.Header.Get("Authorization")
		_ = json.NewEncoder(w).Encode(listEnvelope[Channel]{
			Data: []Channel{
				{ID: "chan-1", AccountID: "acct-a", Name: "general", Platform: "telegram", ExternalRef: "@g"},
				{ID: "chan-2", AccountID: "acct-a", Name: "ops", Platform: "max", ExternalRef: "@o"},
			},
			Meta: PageMeta{},
		})
	}))
	defer srv.Close()

	c := newTestClient(t, srv, Config{AccessToken: "tok-1"})
	chans, err := c.ListChannels(context.Background())
	if err != nil {
		t.Fatalf("ListChannels: %v", err)
	}
	if seenAuth != "Bearer tok-1" {
		t.Fatalf("Authorization = %q, want Bearer tok-1", seenAuth)
	}
	if len(chans) != 2 || chans[0].ID != "chan-1" || chans[1].Platform != "max" {
		t.Fatalf("decoded channels wrong: %+v", chans)
	}
}

func TestCreateChannel_SendsIdempotencyKeyAndBody(t *testing.T) {
	var seenKey, seenCT string
	var seenBody CreateChannelRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/channels" {
			t.Errorf("got %s %s, want POST /v1/channels", r.Method, r.URL.Path)
		}
		seenKey = r.Header.Get("Idempotency-Key")
		seenCT = r.Header.Get("Content-Type")
		if err := json.NewDecoder(r.Body).Decode(&seenBody); err != nil {
			t.Fatalf("decode create body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(Channel{
			ID: "chan-9", AccountID: "acct-a", Name: seenBody.Name,
			Platform: seenBody.Platform, ExternalRef: seenBody.ExternalRef,
			CreatedAt: time.Unix(1_700_000_009, 0).UTC(),
		})
	}))
	defer srv.Close()

	c := newTestClient(t, srv, Config{AccessToken: "tok-1"})
	in := CreateChannelRequest{Name: "release", Platform: "telegram", ExternalRef: "@rel"}
	ch, err := c.CreateChannel(context.Background(), in)
	if err != nil {
		t.Fatalf("CreateChannel: %v", err)
	}
	if seenKey == "" {
		t.Fatalf("Idempotency-Key header was not sent on an unsafe POST")
	}
	if seenCT != "application/json" {
		t.Fatalf("Content-Type = %q, want application/json", seenCT)
	}
	if seenBody != in {
		t.Fatalf("server saw body %+v, want %+v", seenBody, in)
	}
	if ch.ID != "chan-9" || ch.Name != "release" {
		t.Fatalf("decoded channel wrong: %+v", ch)
	}
}

func TestCreateChannel_WithIdempotencyKeyOverride(t *testing.T) {
	var seenKey string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenKey = r.Header.Get("Idempotency-Key")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(Channel{ID: "chan-9"})
	}))
	defer srv.Close()

	c := newTestClient(t, srv, Config{AccessToken: "tok-1"})
	_, err := c.CreateChannel(context.Background(),
		CreateChannelRequest{Name: "x", Platform: "telegram", ExternalRef: "@x"},
		WithIdempotencyKey("fixed-key-42"))
	if err != nil {
		t.Fatalf("CreateChannel: %v", err)
	}
	if seenKey != "fixed-key-42" {
		t.Fatalf("Idempotency-Key = %q, want fixed-key-42", seenKey)
	}
}

func TestGetChannelThreads_PathAndDecode(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/v1/channels/chan-1/threads" {
			t.Errorf("got %s %s, want GET /v1/channels/chan-1/threads", r.Method, r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(listEnvelope[Thread]{
			Data: []Thread{{ID: "thread-1", ChannelID: "chan-1", RootPostID: "post-1", ReplyPostIDs: []string{"post-2"}}},
		})
	}))
	defer srv.Close()

	c := newTestClient(t, srv, Config{AccessToken: "tok-1"})
	threads, err := c.GetChannelThreads(context.Background(), "chan-1")
	if err != nil {
		t.Fatalf("GetChannelThreads: %v", err)
	}
	if len(threads) != 1 || threads[0].RootPostID != "post-1" || threads[0].ReplyPostIDs[0] != "post-2" {
		t.Fatalf("decoded threads wrong: %+v", threads)
	}
}

func TestGetPost_DecodesTypedPost(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/posts/post-1" {
			t.Errorf("path = %q, want /v1/posts/post-1", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(Post{
			ID: "post-1", ChannelID: "chan-1", AccountID: "acct-a",
			Body: "hello", Hashtags: []string{"#research"}, Categories: []string{"research"},
			Status: "succeeded", CreatedAt: time.Unix(1_700_000_100, 0).UTC(),
		})
	}))
	defer srv.Close()

	c := newTestClient(t, srv, Config{AccessToken: "tok-1"})
	post, err := c.GetPost(context.Background(), "post-1")
	if err != nil {
		t.Fatalf("GetPost: %v", err)
	}
	if post.ID != "post-1" || post.Status != "succeeded" || post.Hashtags[0] != "#research" {
		t.Fatalf("decoded post wrong: %+v", post)
	}
}

func TestReprocess_ReturnsJobWithIdempotencyKey(t *testing.T) {
	var seenKey string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/posts/post-1/reprocess" {
			t.Errorf("got %s %s, want POST /v1/posts/post-1/reprocess", r.Method, r.URL.Path)
		}
		seenKey = r.Header.Get("Idempotency-Key")
		w.WriteHeader(http.StatusAccepted)
		_ = json.NewEncoder(w).Encode(Job{
			JobID: "job-1", PostID: "post-1", Status: "queued",
			Precedence: []string{"download", "convert", "analyze", "research", "reply"},
			QueuedAt:   time.Unix(1_700_000_001, 0).UTC(),
		})
	}))
	defer srv.Close()

	c := newTestClient(t, srv, Config{AccessToken: "tok-1"})
	job, err := c.Reprocess(context.Background(), "post-1")
	if err != nil {
		t.Fatalf("Reprocess: %v", err)
	}
	if seenKey == "" {
		t.Fatalf("Idempotency-Key header was not sent on reprocess")
	}
	if job.JobID != "job-1" || job.Status != "queued" || len(job.Precedence) != 5 {
		t.Fatalf("decoded job wrong: %+v", job)
	}
}

func TestSearch_SendsBodyAndDecodesResults(t *testing.T) {
	var seenReq SearchRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/search" {
			t.Errorf("got %s %s, want POST /v1/search", r.Method, r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&seenReq); err != nil {
			t.Fatalf("decode search body: %v", err)
		}
		span := "section:1"
		_ = json.NewEncoder(w).Encode(SearchResults{
			Results: []SearchHit{
				{SourceID: "post-1", Kind: "post", Score: 0.81, Span: &span, Snippet: "…benchmarks…"},
			},
			TookMs: 7, Embedder: "llama",
		})
	}))
	defer srv.Close()

	c := newTestClient(t, srv, Config{AccessToken: "tok-1"})
	res, err := c.Search(context.Background(), SearchRequest{
		Query: "vector database benchmarks", Mode: "hybrid",
		Sources: []string{"posts", "generated"}, TopK: 20, Rerank: true,
	})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if seenReq.Query != "vector database benchmarks" || seenReq.Mode != "hybrid" || seenReq.TopK != 20 {
		t.Fatalf("server saw request %+v", seenReq)
	}
	if res.Embedder != "llama" || len(res.Results) != 1 || res.Results[0].SourceID != "post-1" {
		t.Fatalf("decoded results wrong: %+v", res)
	}
}

func TestListSkills_DecodesEnvelope(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/skills" {
			t.Errorf("path = %q, want /v1/skills", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(listEnvelope[Skill]{
			Data: []Skill{
				{ID: "skill-download", Name: "download", Kind: "atomic", SortOrder: 1},
				{ID: "skill-reply", Name: "reply", Kind: "atomic", SortOrder: 5},
			},
		})
	}))
	defer srv.Close()

	c := newTestClient(t, srv, Config{AccessToken: "tok-1"})
	skills, err := c.ListSkills(context.Background())
	if err != nil {
		t.Fatalf("ListSkills: %v", err)
	}
	if len(skills) != 2 || skills[0].Name != "download" || skills[1].SortOrder != 5 {
		t.Fatalf("decoded skills wrong: %+v", skills)
	}
}

func TestNon2xx_MapsToTypedAPIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeErrorEnvelope(w, http.StatusNotFound, CodeNotFound, "post not found", "req-abc-123")
	}))
	defer srv.Close()

	c := newTestClient(t, srv, Config{AccessToken: "tok-1"})
	_, err := c.GetPost(context.Background(), "missing")
	if err == nil {
		t.Fatalf("expected an error for 404")
	}
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("error is not *APIError: %T (%v)", err, err)
	}
	if apiErr.Code != CodeNotFound {
		t.Errorf("Code = %q, want %q", apiErr.Code, CodeNotFound)
	}
	if apiErr.Status != http.StatusNotFound {
		t.Errorf("Status = %d, want 404", apiErr.Status)
	}
	if apiErr.RequestID != "req-abc-123" {
		t.Errorf("RequestID = %q, want req-abc-123", apiErr.RequestID)
	}
	if apiErr.Message != "post not found" {
		t.Errorf("Message = %q", apiErr.Message)
	}
}

func TestRetry_GET_503ThenSuccess(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/skills" {
			t.Errorf("path = %q", r.URL.Path)
		}
		if atomic.AddInt32(&calls, 1) == 1 {
			writeErrorEnvelope(w, http.StatusServiceUnavailable, CodeUnavailable, "embedder warming up", "req-1")
			return
		}
		_ = json.NewEncoder(w).Encode(listEnvelope[Skill]{
			Data: []Skill{{ID: "skill-download", Name: "download", Kind: "atomic", SortOrder: 1}},
		})
	}))
	defer srv.Close()

	c := newTestClient(t, srv, Config{AccessToken: "tok-1"})
	skills, err := c.ListSkills(context.Background())
	if err != nil {
		t.Fatalf("ListSkills after retry: %v", err)
	}
	if n := atomic.LoadInt32(&calls); n != 2 {
		t.Fatalf("server saw %d calls, want 2 (one 503 retried once)", n)
	}
	if len(skills) != 1 {
		t.Fatalf("decoded skills wrong after retry: %+v", skills)
	}
}

func TestRetry_GET_ExhaustedReturnsAPIError(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		writeErrorEnvelope(w, http.StatusServiceUnavailable, CodeUnavailable, "still down", "req-x")
	}))
	defer srv.Close()

	c := newTestClient(t, srv, Config{AccessToken: "tok-1"})
	_, err := c.ListSkills(context.Background())
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.Code != CodeUnavailable {
		t.Fatalf("want APIError unavailable after exhausted retries, got %v", err)
	}
	// 1 initial + maxRetries(3) retries = 4 attempts.
	if n := atomic.LoadInt32(&calls); n != 4 {
		t.Fatalf("server saw %d calls, want 4", n)
	}
}

func TestPOST_NotRetriedOn503(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		writeErrorEnvelope(w, http.StatusServiceUnavailable, CodeUnavailable, "down", "req-p")
	}))
	defer srv.Close()

	c := newTestClient(t, srv, Config{AccessToken: "tok-1"})
	_, err := c.Reprocess(context.Background(), "post-1")
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("want APIError, got %v", err)
	}
	if n := atomic.LoadInt32(&calls); n != 1 {
		t.Fatalf("unsafe POST retried: server saw %d calls, want 1", n)
	}
}

func TestSubscribeEvents_ReceivesDecodedSSEEvent(t *testing.T) {
	var seenAccept string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/events" {
			t.Errorf("path = %q, want /v1/events", r.URL.Path)
		}
		seenAccept = r.Header.Get("Accept")
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Fatalf("test server does not support flushing")
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		// Heartbeat comment, then a real framed event.
		_, _ = io.WriteString(w, ": subscribed\n\n")
		flusher.Flush()
		_, _ = io.WriteString(w, "id: e1\nevent: processing.progress\n"+
			`data: {"id":"e1","type":"processing.progress","payload":{"stage":"analyze","progress":0.6},"trace_id":"t1"}`+"\n\n")
		flusher.Flush()
		// Hold the stream open until the client cancels (request context ends).
		<-r.Context().Done()
	}))
	defer srv.Close()

	c := newTestClient(t, srv, Config{AccessToken: "tok-1"})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	events, err := c.SubscribeEvents(ctx)
	if err != nil {
		t.Fatalf("SubscribeEvents: %v", err)
	}
	if !strings.Contains(seenAccept, "text/event-stream") {
		t.Errorf("Accept = %q, want text/event-stream", seenAccept)
	}

	select {
	case ev, ok := <-events:
		if !ok {
			t.Fatalf("event channel closed before delivering an event")
		}
		if ev.ID != "e1" || ev.Type != "processing.progress" {
			t.Fatalf("event fields wrong: %+v", ev)
		}
		if ev.Payload["stage"] != "analyze" {
			t.Fatalf("payload wrong: %+v", ev.Payload)
		}
		if ev.TraceID != "t1" {
			t.Fatalf("trace_id = %q, want t1", ev.TraceID)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for an SSE event")
	}

	// Cancelling ctx must close the channel (unsubscribe).
	cancel()
	select {
	case _, ok := <-events:
		if ok {
			// Drain a possibly-buffered trailing event, then it must close.
			select {
			case _, ok2 := <-events:
				if ok2 {
					t.Fatalf("channel did not close after cancel")
				}
			case <-time.After(2 * time.Second):
				t.Fatalf("channel not closed after cancel")
			}
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("channel not closed after cancel")
	}
}

func TestSubscribeEvents_Non2xxReturnsAPIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeErrorEnvelope(w, http.StatusUnauthorized, CodeUnauthenticated, "missing token", "req-e")
	}))
	defer srv.Close()

	c := newTestClient(t, srv, Config{}) // no credential
	_, err := c.SubscribeEvents(context.Background())
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.Code != CodeUnauthenticated {
		t.Fatalf("want APIError unauthenticated, got %v", err)
	}
}

func TestAPIKeyAuth_SendsXAPIKeyHeader(t *testing.T) {
	var seenAuth, seenKey string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenAuth = r.Header.Get("Authorization")
		seenKey = r.Header.Get("X-API-Key")
		_ = json.NewEncoder(w).Encode(listEnvelope[Channel]{Data: []Channel{}})
	}))
	defer srv.Close()

	c := newTestClient(t, srv, Config{APIKey: "sk-secret-123"})
	if _, err := c.ListChannels(context.Background()); err != nil {
		t.Fatalf("ListChannels: %v", err)
	}
	if seenKey != "sk-secret-123" {
		t.Fatalf("X-API-Key = %q, want sk-secret-123", seenKey)
	}
	if seenAuth != "" {
		t.Fatalf("Authorization should be empty when using an API key, got %q", seenAuth)
	}
}

func TestNew_RequiresBaseURL(t *testing.T) {
	if _, err := New(Config{}); err == nil {
		t.Fatalf("expected error for empty BaseURL")
	}
	if _, err := New(Config{BaseURL: "https://x/v1/"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAPIError_ErrorStringAndRetryable(t *testing.T) {
	e := &APIError{Code: CodeRateLimited, Message: "slow down", Status: 429, RequestID: "req-9"}
	if !strings.Contains(e.Error(), "rate_limited") || !strings.Contains(e.Error(), "req-9") {
		t.Errorf("Error() = %q", e.Error())
	}
	if !e.Retryable() {
		t.Errorf("rate_limited should be Retryable")
	}
	if (&APIError{Code: CodeNotFound}).Retryable() {
		t.Errorf("not_found should not be Retryable")
	}
}
