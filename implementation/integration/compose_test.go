package integration

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"digital.vasic.maxadapter"
	"digital.vasic.metubewebhook"
	gateway "digital.vasic.restgateway"
	"digital.vasic.threadreader"
)

func maxToThreadReader(p maxadapter.Post) threadreader.Post {
	tr := threadreader.Post{
		ID:            p.ID,
		ThreadID:      p.ThreadID,
		ParentID:      p.ParentID,
		AuthorID:      p.AuthorID,
		Text:          p.Text,
		TimestampUnix: p.TimestampUnix,
		IsForwarded:   p.IsForwarded,
	}
	for _, a := range p.Attachments {
		tr.Attachments = append(tr.Attachments, threadreader.Attachment{
			ID: a.ID, MIME: a.MIME, FileName: a.FileName, SHA256: a.SHA256,
		})
	}
	return tr
}

// TestMaxAdapterFeedsThreadReader proves a SECOND messenger adapter (Max/OneMe)
// composes with the same threadreader.Assembler seam the telegram adapter uses:
// its parsed Posts assemble into an organic thread with unioned hashtags.
func TestMaxAdapterFeedsThreadReader(t *testing.T) {
	raw := []byte(`{"chatId":555,"messages":[
		{"id":10,"sender":"u1","text":"lab clip https://v.example/x #Video","time":1700000000000},
		{"id":11,"sender":"u2","text":"grab it #ToDownload","time":1700000001000,"link":{"type":"REPLY","messageId":10}},
		{"id":12,"sender":"bot","text":"processing","time":1700000002000,"link":{"type":"REPLY","messageId":10}}
	]}`)
	posts, err := maxadapter.ParseHistory(raw)
	if err != nil {
		t.Fatalf("maxadapter.ParseHistory: %v", err)
	}
	if len(posts) != 3 {
		t.Fatalf("parsed %d posts, want 3", len(posts))
	}
	trPosts := make([]threadreader.Post, 0, len(posts))
	for _, p := range posts {
		trPosts = append(trPosts, maxToThreadReader(p))
	}
	// "bot" is the system author to exclude — same assembler contract.
	thread, err := threadreader.NewAssembler("bot").Assemble(trPosts)
	if err != nil {
		t.Fatalf("Assemble: %v", err)
	}
	if thread.Root.ID != "10" {
		t.Fatalf("root id = %q, want 10", thread.Root.ID)
	}
	if len(thread.Replies) != 1 {
		t.Fatalf("organic replies = %d, want 1 (bot filtered)", len(thread.Replies))
	}
	tags := thread.Hashtags()
	if !containsStr(tags, "Video") || !containsStr(tags, "ToDownload") {
		t.Fatalf("hashtags %v missing Video/ToDownload", tags)
	}
}

// TestMeTubeWebhookCompletionSigned proves the metube_webhook shim composes as a
// second completion source: parse a MeTube job list, build the standardized
// completion envelope, deliver it HMAC-signed, and have a receiver INDEPENDENTLY
// verify the signature — the same X-Thready-Signature scheme as callback_task.
func TestMeTubeWebhookCompletionSigned(t *testing.T) {
	jobsJSON := []byte(`{"jobs":[
		{"id":"vid1","status":"downloading","percent":42.5,"filename":"vid1.mp4"},
		{"id":"vid2","status":"finished","filename":"/downloads/vid2.mp4"}
	]}`)
	jobs, err := metubewebhook.ParseJobs(jobsJSON)
	if err != nil {
		t.Fatalf("ParseJobs: %v", err)
	}
	var finished *metubewebhook.JobStatus
	for i := range jobs {
		if jobs[i].State == metubewebhook.StateFinished {
			finished = &jobs[i]
		}
	}
	if finished == nil {
		t.Fatal("no finished job parsed")
	}
	env := metubewebhook.EnvelopeFor(*finished, time.Unix(1700000000, 0).UTC())
	if env.State != metubewebhook.CompletionSuccess || env.ResultRef != "/downloads/vid2.mp4" {
		t.Fatalf("envelope = %+v, want success + result path", env)
	}

	secret := []byte("metube-shared-secret")
	var verified bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		mac := hmac.New(sha256.New, secret)
		mac.Write(body)
		want := "sha256=" + hex.EncodeToString(mac.Sum(nil))
		if hmac.Equal([]byte(want), []byte(r.Header.Get(metubewebhook.SignatureHeader))) {
			verified = true
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	sink := &metubewebhook.WebhookSink{URL: srv.URL, Secret: secret, MaxRetries: 1}
	if err := sink.Notify(context.Background(), env); err != nil {
		t.Fatalf("Notify: %v", err)
	}
	if !verified {
		t.Fatal("receiver did not independently verify the MeTube webhook HMAC")
	}
}

// TestRestGatewayComposes proves the rest_gateway HTTP surface builds and runs
// inside the workspace: a real httptest round-trip against /v1/health and a
// seeded /v1/auth/login returning a bearer token.
func TestRestGatewayComposes(t *testing.T) {
	signer, err := gateway.NewSigner(gateway.SignerConfig{Secret: []byte("gateway-hs256-secret")})
	if err != nil {
		t.Fatalf("NewSigner: %v", err)
	}
	srv := httptest.NewServer(gateway.New(gateway.NewInMemoryServices(), signer))
	defer srv.Close()

	// Health probe.
	resp, err := http.Get(srv.URL + "/v1/health")
	if err != nil {
		t.Fatalf("health GET: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("health status = %d, want 200", resp.StatusCode)
	}
	resp.Body.Close()

	// Seeded login.
	body, _ := json.Marshal(map[string]string{
		"email":    gateway.SeedUserEmail,
		"password": gateway.SeedUserPass,
	})
	resp, err = http.Post(srv.URL+"/v1/auth/login", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("login POST: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("login status = %d, want 200; body=%s", resp.StatusCode, b)
	}
	var out struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode login response: %v", err)
	}
	if out.AccessToken == "" {
		t.Fatal("login returned an empty access token")
	}
}
