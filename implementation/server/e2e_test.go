package server_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	gateway "digital.vasic.restgateway"
	userservice "digital.vasic.userservice"
	server "thready.server"
)

// newTestServer boots the REAL-wired handler over httptest, exercising the true
// HTTP path (routing + middleware + real domain services).
func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	// The signer secret is runtime-loaded from the environment and NewServer
	// fails closed without it; provide a throwaway test value so the REAL signer
	// still runs (this exercises the genuine sign/verify path).
	t.Setenv("THREADY_JWT_SECRET", "test-secret-thready-server-e2e-please-rotate")
	h, err := server.NewServer()
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	ts := httptest.NewServer(h)
	t.Cleanup(ts.Close)
	return ts
}

type tokenPair struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
}

// login POSTs credentials and returns the status code and (on 200) the token.
func login(t *testing.T, ts *httptest.Server, email, password, totp string) (int, tokenPair) {
	t.Helper()
	body, _ := json.Marshal(map[string]string{"email": email, "password": password, "totp": totp})
	resp, err := http.Post(ts.URL+"/v1/auth/login", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("login POST: %v", err)
	}
	defer resp.Body.Close()
	var tp tokenPair
	if resp.StatusCode == http.StatusOK {
		if err := json.NewDecoder(resp.Body).Decode(&tp); err != nil {
			t.Fatalf("decode token: %v", err)
		}
	} else {
		io.Copy(io.Discard, resp.Body)
	}
	return resp.StatusCode, tp
}

// currentRootTOTP computes the live RFC 6238 code from the real seed secret via
// user_service — proving the success path drives the real TOTP verifier.
func currentRootTOTP(t *testing.T) string {
	t.Helper()
	totp, err := userservice.NewTOTPFromBase32(server.SeedRootTOTPSecretB32)
	if err != nil {
		t.Fatalf("NewTOTPFromBase32: %v", err)
	}
	return totp.Now()
}

// authGet issues an authenticated GET and returns the response.
func authGet(t *testing.T, ts *httptest.Server, path, token string) *http.Response {
	t.Helper()
	req, _ := http.NewRequest(http.MethodGet, ts.URL+path, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET %s: %v", path, err)
	}
	return resp
}

// authPost issues an authenticated POST with a JSON body.
func authPost(t *testing.T, ts *httptest.Server, path, token string, payload any) *http.Response {
	t.Helper()
	body, _ := json.Marshal(payload)
	req, _ := http.NewRequest(http.MethodPost, ts.URL+path, bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST %s: %v", path, err)
	}
	return resp
}

func rootToken(t *testing.T, ts *httptest.Server) string {
	t.Helper()
	status, tp := login(t, ts, gateway.SeedRootEmail, gateway.SeedRootPass, currentRootTOTP(t))
	if status != http.StatusOK {
		t.Fatalf("root login expected 200, got %d", status)
	}
	if tp.AccessToken == "" {
		t.Fatal("root login returned empty access token")
	}
	return tp.AccessToken
}

// Test 1: correct seed password + real TOTP -> 200 + token; wrong password ->
// 401. The password is genuinely PBKDF2-hashed at seed time, so a 200 proves the
// real user_service verifier ran (not a string compare).
func TestLogin_RealPBKDF2AndTOTP(t *testing.T) {
	ts := newTestServer(t)

	status, tp := login(t, ts, gateway.SeedRootEmail, gateway.SeedRootPass, currentRootTOTP(t))
	if status != http.StatusOK {
		t.Fatalf("correct creds: expected 200, got %d", status)
	}
	if tp.AccessToken == "" {
		t.Fatal("correct creds: expected a non-empty access token")
	}

	// Wrong password must fail THROUGH the real PBKDF2 verifier.
	badStatus, _ := login(t, ts, gateway.SeedRootEmail, "definitely-wrong-pass", currentRootTOTP(t))
	if badStatus != http.StatusUnauthorized {
		t.Fatalf("wrong password: expected 401, got %d", badStatus)
	}

	// Wrong TOTP must fail THROUGH the real RFC 6238 verifier.
	badTOTP, _ := login(t, ts, gateway.SeedRootEmail, gateway.SeedRootPass, "000000")
	if badTOTP != http.StatusUnauthorized {
		t.Fatalf("wrong TOTP: expected 401, got %d", badTOTP)
	}
}

// Test 2: real cosine ranking. A vector-DB query ranks vectordb.md top; a
// disjoint telegram query ranks telegram.md top (negative control).
func TestSearch_RealCosineRanking(t *testing.T) {
	ts := newTestServer(t)
	token := rootToken(t, ts)

	type searchResp struct {
		Results []struct {
			SourceID string  `json:"source_id"`
			Score    float64 `json:"score"`
		} `json:"results"`
		Embedder string `json:"embedder"`
	}

	do := func(query string) searchResp {
		resp := authPost(t, ts, "/v1/search", token, map[string]any{"query": query, "top_k": 5})
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			b, _ := io.ReadAll(resp.Body)
			t.Fatalf("search %q: expected 200, got %d (%s)", query, resp.StatusCode, b)
		}
		var sr searchResp
		if err := json.NewDecoder(resp.Body).Decode(&sr); err != nil {
			t.Fatalf("decode search: %v", err)
		}
		if len(sr.Results) == 0 {
			t.Fatalf("search %q: expected at least one hit", query)
		}
		return sr
	}

	vdb := do("vector database cosine similarity benchmarks")
	if vdb.Results[0].SourceID != "vectordb.md" {
		t.Fatalf("vector query: expected top source vectordb.md, got %q", vdb.Results[0].SourceID)
	}
	if vdb.Embedder != server.EmbedderLabel {
		t.Fatalf("expected honest embedder label %q, got %q", server.EmbedderLabel, vdb.Embedder)
	}

	// Negative control: a disjoint-terms query must rank a different doc top.
	tg := do("telegram bot channel webhook")
	if tg.Results[0].SourceID != "telegram.md" {
		t.Fatalf("telegram query: expected top source telegram.md, got %q", tg.Results[0].SourceID)
	}
	if vdb.Results[0].SourceID == tg.Results[0].SourceID {
		t.Fatal("expected the two disjoint queries to rank different docs top")
	}
}

// Test 3: real skill_dispatch skills returned in real precedence order.
func TestSkills_RealPrecedenceOrder(t *testing.T) {
	ts := newTestServer(t)
	token := rootToken(t, ts)

	resp := authGet(t, ts, "/v1/skills", token)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("skills: expected 200, got %d", resp.StatusCode)
	}
	var env struct {
		Data []struct {
			Name string `json:"name"`
			Kind string `json:"kind"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&env); err != nil {
		t.Fatalf("decode skills: %v", err)
	}
	got := make([]string, len(env.Data))
	for i, s := range env.Data {
		got[i] = s.Name
	}
	want := []string{"video.download", "media.convert", "vision.analyze", "tech.research", "thread.reply"}
	if len(got) != len(want) {
		t.Fatalf("skills: expected %v, got %v", want, got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("skills precedence: expected %v, got %v", want, got)
		}
	}
}

// Test 4: POST then GET channels — the created channel is present.
func TestChannels_CreateThenList(t *testing.T) {
	ts := newTestServer(t)
	token := rootToken(t, ts)

	created := authPost(t, ts, "/v1/channels", token, map[string]string{
		"name": "releases", "platform": "telegram", "external_ref": "@thready_releases",
	})
	defer created.Body.Close()
	if created.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(created.Body)
		t.Fatalf("create channel: expected 201, got %d (%s)", created.StatusCode, b)
	}
	var ch gateway.Channel
	if err := json.NewDecoder(created.Body).Decode(&ch); err != nil {
		t.Fatalf("decode created channel: %v", err)
	}
	if ch.Name != "releases" || ch.ID == "" {
		t.Fatalf("unexpected created channel: %+v", ch)
	}

	resp := authGet(t, ts, "/v1/channels", token)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("list channels: expected 200, got %d", resp.StatusCode)
	}
	var env struct {
		Data []gateway.Channel `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&env); err != nil {
		t.Fatalf("decode channels: %v", err)
	}
	found := false
	for _, c := range env.Data {
		if c.ID == ch.ID && c.Name == "releases" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("created channel %q not present in list %+v", ch.ID, env.Data)
	}
}

// Test 5: reprocessing a MISSING post returns 404 (not the generic 500 it
// produced while the gateway coded-error type was unexported). realPosts.Reprocess
// now signals the miss with gateway.NewError(gateway.CodeNotFound, …), which the
// gateway's writeServiceError maps to 404 + a not_found envelope. This is the
// reviewer-disclosed 500->404 correctness fix, asserted end-to-end over real HTTP.
func TestReprocessMissingPost_404(t *testing.T) {
	ts := newTestServer(t)
	token := rootToken(t, ts)

	resp := authPost(t, ts, "/v1/posts/does-not-exist/reprocess", token, nil)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("reprocess missing post: want 404, got %d (%s)", resp.StatusCode, b)
	}
	var env struct {
		Error struct {
			Code   string `json:"code"`
			Status int    `json:"status"`
		} `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&env); err != nil {
		t.Fatalf("decode error envelope: %v", err)
	}
	if env.Error.Code != string(gateway.CodeNotFound) {
		t.Fatalf("want envelope code not_found, got %q", env.Error.Code)
	}
	if env.Error.Status != http.StatusNotFound {
		t.Fatalf("want envelope status 404, got %d", env.Error.Status)
	}

	// Control: the seed post DOES reprocess (202) — proving the 404 is specific
	// to the missing post, not a broken route.
	ok := authPost(t, ts, "/v1/posts/"+gateway.SeedPostA+"/reprocess", token, nil)
	defer ok.Body.Close()
	if ok.StatusCode != http.StatusAccepted {
		b, _ := io.ReadAll(ok.Body)
		t.Fatalf("reprocess seed post: want 202, got %d (%s)", ok.StatusCode, b)
	}
}

// Test 6: NewServer fails closed when the signing secret is absent. A committed
// signing secret would let anyone forge tokens; the server must refuse to start
// rather than fall back to a hardcoded key (constitution §11.4.10).
func TestNewServer_FailsClosedWithoutSecret(t *testing.T) {
	t.Setenv("THREADY_JWT_SECRET", "") // simulate the env var being unset
	if _, err := server.NewServer(); err == nil {
		t.Fatal("NewServer must fail closed when THREADY_JWT_SECRET is empty, but returned nil error")
	}
}
