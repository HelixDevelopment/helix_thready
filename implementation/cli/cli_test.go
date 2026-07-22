package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

// fakeClient is an in-memory APIClient that records every call and returns canned
// data, so the command layer can be asserted without any network or SDK. When
// err is non-nil, every method returns it (exercising the error/exit-code paths).
type fakeClient struct {
	err error

	loginCalls    int
	loginCreds    Credentials
	listChannels  int
	createCalls   int
	createInput   CreateChannelInput
	getPostCalls  int
	getPostID     string
	reprocCalls   int
	reprocID      string
	searchCalls   int
	searchQuery   string
	searchOpts    SearchOptions
	listSkills    int
	whoamiCalls   int
	tokenReturned string
}

func (f *fakeClient) Login(_ context.Context, creds Credentials) (*TokenPair, error) {
	f.loginCalls++
	f.loginCreds = creds
	if f.err != nil {
		return nil, f.err
	}
	f.tokenReturned = "tok_ABC123"
	return &TokenPair{AccessToken: f.tokenReturned, RefreshToken: "ref_XYZ", TokenType: "Bearer", ExpiresIn: 3600}, nil
}

func (f *fakeClient) ListChannels(_ context.Context) ([]Channel, error) {
	f.listChannels++
	if f.err != nil {
		return nil, f.err
	}
	ts := time.Date(2026, 7, 22, 10, 0, 0, 0, time.UTC)
	return []Channel{
		{ID: "chan_1", AccountID: "acct_1", Name: "Design QA", Platform: "telegram", ExternalRef: "-100123", CreatedAt: ts},
		{ID: "chan_2", AccountID: "acct_1", Name: "Ops Feed", Platform: "max", ExternalRef: "grp_9", CreatedAt: ts},
	}, nil
}

func (f *fakeClient) CreateChannel(_ context.Context, in CreateChannelInput) (*Channel, error) {
	f.createCalls++
	f.createInput = in
	if f.err != nil {
		return nil, f.err
	}
	return &Channel{ID: "chan_new", AccountID: "acct_1", Name: in.Name, Platform: in.Platform, ExternalRef: in.ExternalRef, CreatedAt: time.Unix(0, 0).UTC()}, nil
}

func (f *fakeClient) GetPost(_ context.Context, id string) (*Post, error) {
	f.getPostCalls++
	f.getPostID = id
	if f.err != nil {
		return nil, f.err
	}
	return &Post{ID: id, ChannelID: "chan_1", AccountID: "acct_1", Body: "hello world", Hashtags: []string{"ai"}, Categories: []string{"news"}, Status: "processed", CreatedAt: time.Unix(0, 0).UTC()}, nil
}

func (f *fakeClient) Reprocess(_ context.Context, id string) (*Job, error) {
	f.reprocCalls++
	f.reprocID = id
	if f.err != nil {
		return nil, f.err
	}
	return &Job{JobID: "job_777", PostID: id, Status: "queued", Precedence: []string{"download", "convert", "analyze", "research", "reply"}, QueuedAt: time.Unix(0, 0).UTC()}, nil
}

func (f *fakeClient) Search(_ context.Context, query string, opts SearchOptions) (*SearchResults, error) {
	f.searchCalls++
	f.searchQuery = query
	f.searchOpts = opts
	if f.err != nil {
		return nil, f.err
	}
	return &SearchResults{
		Results:  []SearchHit{{SourceID: "post_5", Kind: "post", Score: 0.9123, Snippet: "matching text"}},
		TookMs:   12,
		Embedder: "openai-text-3",
	}, nil
}

func (f *fakeClient) ListSkills(_ context.Context) ([]Skill, error) {
	f.listSkills++
	if f.err != nil {
		return nil, f.err
	}
	return []Skill{
		{ID: "sk_1", Name: "download", Kind: "action", SortOrder: 1},
		{ID: "sk_2", Name: "analyze", Kind: "action", SortOrder: 3},
	}, nil
}

func (f *fakeClient) Whoami(_ context.Context) (*Identity, error) {
	f.whoamiCalls++
	if f.err != nil {
		return nil, f.err
	}
	return &Identity{Subject: "user_42", Email: "me@example.com", Tier: "standard", TokenPresent: true}, nil
}

// run is a helper that invokes Run and returns the exit code plus captured
// stdout/stderr.
func run(t *testing.T, f APIClient, args ...string) (int, string, string) {
	t.Helper()
	var out, errOut bytes.Buffer
	code := Run(args, f, &out, &errOut)
	return code, out.String(), errOut.String()
}

func TestChannelsList_TableAndCall(t *testing.T) {
	f := &fakeClient{}
	code, out, errOut := run(t, f, "channels", "list")

	if code != exitOK {
		t.Fatalf("exit = %d, want %d (stderr=%q)", code, exitOK, errOut)
	}
	if f.listChannels != 1 {
		t.Fatalf("ListChannels called %d times, want 1", f.listChannels)
	}
	for _, want := range []string{"ID", "NAME", "PLATFORM", "EXTERNAL_REF", "CREATED_AT", "chan_1", "Design QA", "telegram", "Ops Feed"} {
		if !strings.Contains(out, want) {
			t.Errorf("table output missing %q\n---\n%s", want, out)
		}
	}
	if errOut != "" {
		t.Errorf("unexpected stderr: %q", errOut)
	}
}

func TestChannelsAdd_CreateCalledWithName(t *testing.T) {
	f := &fakeClient{}
	code, out, errOut := run(t, f, "channels", "add", "--name", "Design QA", "--platform", "telegram", "--external-ref", "-100123")

	if code != exitOK {
		t.Fatalf("exit = %d, want %d (stderr=%q)", code, exitOK, errOut)
	}
	if f.createCalls != 1 {
		t.Fatalf("CreateChannel called %d times, want 1", f.createCalls)
	}
	if f.createInput.Name != "Design QA" {
		t.Errorf("Name = %q, want %q", f.createInput.Name, "Design QA")
	}
	if f.createInput.Platform != "telegram" {
		t.Errorf("Platform = %q, want %q", f.createInput.Platform, "telegram")
	}
	if f.createInput.ExternalRef != "-100123" {
		t.Errorf("ExternalRef = %q, want %q", f.createInput.ExternalRef, "-100123")
	}
	if !strings.Contains(out, "chan_new") {
		t.Errorf("output missing created channel id: %q", out)
	}
}

func TestChannelsAdd_MissingNameIsUsageError(t *testing.T) {
	f := &fakeClient{}
	code, _, errOut := run(t, f, "channels", "add", "--platform", "telegram")

	if code != exitUsage {
		t.Fatalf("exit = %d, want %d", code, exitUsage)
	}
	if f.createCalls != 0 {
		t.Errorf("CreateChannel should not be called on usage error, got %d", f.createCalls)
	}
	if !strings.Contains(errOut, "--name") {
		t.Errorf("stderr should mention --name: %q", errOut)
	}
}

func TestPostGet_MethodAndID(t *testing.T) {
	f := &fakeClient{}
	code, out, errOut := run(t, f, "post", "get", "post_42")

	if code != exitOK {
		t.Fatalf("exit = %d, want %d (stderr=%q)", code, exitOK, errOut)
	}
	if f.getPostCalls != 1 || f.getPostID != "post_42" {
		t.Fatalf("GetPost calls=%d id=%q, want 1 / post_42", f.getPostCalls, f.getPostID)
	}
	if f.reprocCalls != 0 {
		t.Errorf("Reprocess unexpectedly called %d times", f.reprocCalls)
	}
	if !strings.Contains(out, "post_42") || !strings.Contains(out, "processed") {
		t.Errorf("output missing post fields: %q", out)
	}
}

func TestPostGet_FlagAfterPositional(t *testing.T) {
	// --json AFTER the positional id must still parse (interspersed handling).
	f := &fakeClient{}
	code, out, _ := run(t, f, "post", "get", "post_42", "--json")
	if code != exitOK {
		t.Fatalf("exit = %d, want %d", code, exitOK)
	}
	if f.getPostID != "post_42" {
		t.Fatalf("id = %q, want post_42", f.getPostID)
	}
	if !json.Valid([]byte(out)) {
		t.Fatalf("output not valid JSON: %q", out)
	}
}

func TestPostReprocess_202AndJob(t *testing.T) {
	f := &fakeClient{}
	code, out, errOut := run(t, f, "post", "reprocess", "post_9")

	if code != exitOK {
		t.Fatalf("exit = %d, want %d (stderr=%q)", code, exitOK, errOut)
	}
	if f.reprocCalls != 1 || f.reprocID != "post_9" {
		t.Fatalf("Reprocess calls=%d id=%q, want 1 / post_9", f.reprocCalls, f.reprocID)
	}
	if !strings.Contains(out, "202") {
		t.Errorf("output missing 202 status: %q", out)
	}
	if !strings.Contains(out, "job_777") {
		t.Errorf("output missing job id: %q", out)
	}
}

func TestSearch_ParsedOptsAndResults(t *testing.T) {
	f := &fakeClient{}
	code, out, errOut := run(t, f, "search", "vector db", "--mode", "semantic", "--top-k", "5", "--sources", "posts,generated", "--rerank")

	if code != exitOK {
		t.Fatalf("exit = %d, want %d (stderr=%q)", code, exitOK, errOut)
	}
	if f.searchCalls != 1 {
		t.Fatalf("Search called %d times, want 1", f.searchCalls)
	}
	if f.searchQuery != "vector db" {
		t.Errorf("query = %q, want %q", f.searchQuery, "vector db")
	}
	if f.searchOpts.Mode != "semantic" {
		t.Errorf("mode = %q, want semantic", f.searchOpts.Mode)
	}
	if f.searchOpts.TopK != 5 {
		t.Errorf("top-k = %d, want 5", f.searchOpts.TopK)
	}
	if !f.searchOpts.Rerank {
		t.Errorf("rerank = false, want true")
	}
	if len(f.searchOpts.Sources) != 2 || f.searchOpts.Sources[0] != "posts" || f.searchOpts.Sources[1] != "generated" {
		t.Errorf("sources = %v, want [posts generated]", f.searchOpts.Sources)
	}
	if !strings.Contains(out, "post_5") || !strings.Contains(out, "openai-text-3") {
		t.Errorf("results not printed: %q", out)
	}
}

func TestSearch_MissingQueryIsUsageError(t *testing.T) {
	f := &fakeClient{}
	code, _, errOut := run(t, f, "search", "--mode", "semantic")
	if code != exitUsage {
		t.Fatalf("exit = %d, want %d", code, exitUsage)
	}
	if f.searchCalls != 0 {
		t.Errorf("Search should not be called, got %d", f.searchCalls)
	}
	if !strings.Contains(errOut, "query") {
		t.Errorf("stderr should mention query: %q", errOut)
	}
}

func TestJSONFlag_ProducesValidJSON(t *testing.T) {
	f := &fakeClient{}
	code, out, _ := run(t, f, "channels", "list", "--json")
	if code != exitOK {
		t.Fatalf("exit = %d, want %d", code, exitOK)
	}
	if !json.Valid([]byte(out)) {
		t.Fatalf("output is not valid JSON:\n%s", out)
	}
	var chans []Channel
	if err := json.Unmarshal([]byte(out), &chans); err != nil {
		t.Fatalf("unmarshal channels: %v", err)
	}
	if len(chans) != 2 || chans[0].ID != "chan_1" {
		t.Fatalf("decoded channels wrong: %+v", chans)
	}
}

func TestLogin_StoresPrintsToken(t *testing.T) {
	f := &fakeClient{}
	code, out, errOut := run(t, f, "login", "--email", "me@example.com", "--password", "s3cret")

	if code != exitOK {
		t.Fatalf("exit = %d, want %d (stderr=%q)", code, exitOK, errOut)
	}
	if f.loginCalls != 1 {
		t.Fatalf("Login called %d times, want 1", f.loginCalls)
	}
	if f.loginCreds.Email != "me@example.com" || f.loginCreds.Password != "s3cret" {
		t.Errorf("creds = %+v, want email/password set", f.loginCreds)
	}
	if !strings.Contains(out, "tok_ABC123") {
		t.Errorf("token not printed: %q", out)
	}
}

func TestLogin_MissingCredsIsUsageError(t *testing.T) {
	f := &fakeClient{}
	code, _, errOut := run(t, f, "login", "--email", "me@example.com")
	if code != exitUsage {
		t.Fatalf("exit = %d, want %d", code, exitUsage)
	}
	if f.loginCalls != 0 {
		t.Errorf("Login should not be called, got %d", f.loginCalls)
	}
	if !strings.Contains(errOut, "--password") {
		t.Errorf("stderr should mention --password: %q", errOut)
	}
}

func TestSkills_TableAndCall(t *testing.T) {
	f := &fakeClient{}
	code, out, _ := run(t, f, "skills")
	if code != exitOK {
		t.Fatalf("exit = %d, want %d", code, exitOK)
	}
	if f.listSkills != 1 {
		t.Fatalf("ListSkills called %d times, want 1", f.listSkills)
	}
	for _, want := range []string{"ID", "NAME", "KIND", "SORT_ORDER", "download", "analyze"} {
		if !strings.Contains(out, want) {
			t.Errorf("skills table missing %q\n%s", want, out)
		}
	}
}

func TestWhoami_Call(t *testing.T) {
	f := &fakeClient{}
	code, out, _ := run(t, f, "whoami")
	if code != exitOK {
		t.Fatalf("exit = %d, want %d", code, exitOK)
	}
	if f.whoamiCalls != 1 {
		t.Fatalf("Whoami called %d times, want 1", f.whoamiCalls)
	}
	if !strings.Contains(out, "user_42") || !strings.Contains(out, "me@example.com") {
		t.Errorf("identity not printed: %q", out)
	}
}

func TestUnknownCommand_NonzeroUsageOnStderr(t *testing.T) {
	f := &fakeClient{}
	code, out, errOut := run(t, f, "frobnicate")
	if code == exitOK {
		t.Fatalf("exit = %d, want nonzero", code)
	}
	if code != exitUsage {
		t.Errorf("exit = %d, want %d", code, exitUsage)
	}
	if out != "" {
		t.Errorf("stdout should be empty on unknown command, got %q", out)
	}
	if !strings.Contains(errOut, "unknown command") || !strings.Contains(errOut, "Usage:") {
		t.Errorf("stderr should carry error + usage, got %q", errOut)
	}
}

func TestNoArgs_UsageOnStderr(t *testing.T) {
	f := &fakeClient{}
	code, out, errOut := run(t, f)
	if code != exitUsage {
		t.Fatalf("exit = %d, want %d", code, exitUsage)
	}
	if out != "" {
		t.Errorf("stdout should be empty, got %q", out)
	}
	if !strings.Contains(errOut, "Usage:") {
		t.Errorf("stderr should carry usage, got %q", errOut)
	}
}

func TestHelp_UsageOnStdoutExitZero(t *testing.T) {
	f := &fakeClient{}
	code, out, errOut := run(t, f, "help")
	if code != exitOK {
		t.Fatalf("exit = %d, want %d", code, exitOK)
	}
	if !strings.Contains(out, "Usage:") {
		t.Errorf("help should print usage to stdout, got %q", out)
	}
	if errOut != "" {
		t.Errorf("help should not write stderr, got %q", errOut)
	}
}

func TestAPIError_ExitCodeOne(t *testing.T) {
	f := &fakeClient{err: errors.New("boom: gateway unavailable")}
	code, out, errOut := run(t, f, "channels", "list")
	if code != exitError {
		t.Fatalf("exit = %d, want %d", code, exitError)
	}
	if f.listChannels != 1 {
		t.Errorf("ListChannels should still be attempted, got %d", f.listChannels)
	}
	if out != "" {
		t.Errorf("stdout should be empty on error, got %q", out)
	}
	if !strings.Contains(errOut, "boom") {
		t.Errorf("stderr should carry the error, got %q", errOut)
	}
}

func TestChannels_MissingSubcommand(t *testing.T) {
	f := &fakeClient{}
	code, _, errOut := run(t, f, "channels")
	if code != exitUsage {
		t.Fatalf("exit = %d, want %d", code, exitUsage)
	}
	if !strings.Contains(errOut, "subcommand") {
		t.Errorf("stderr should mention subcommand, got %q", errOut)
	}
}

func TestPost_UnknownSubcommand(t *testing.T) {
	f := &fakeClient{}
	code, _, errOut := run(t, f, "post", "delete", "x")
	if code != exitUsage {
		t.Fatalf("exit = %d, want %d", code, exitUsage)
	}
	if !strings.Contains(errOut, "unknown post subcommand") {
		t.Errorf("stderr should reject subcommand, got %q", errOut)
	}
}

func TestSplitCSV(t *testing.T) {
	cases := map[string][]string{
		"":                    nil,
		"  ":                  nil,
		"posts":               {"posts"},
		"posts,generated":     {"posts", "generated"},
		" posts , generated ": {"posts", "generated"},
		"posts,,assets":       {"posts", "assets"},
	}
	for in, want := range cases {
		got := splitCSV(in)
		if len(got) != len(want) {
			t.Errorf("splitCSV(%q) = %v, want %v", in, got, want)
			continue
		}
		for i := range want {
			if got[i] != want[i] {
				t.Errorf("splitCSV(%q)[%d] = %q, want %q", in, i, got[i], want[i])
			}
		}
	}
}

// TestSDKAdapterSatisfiesInterface pins the production adapter to the interface
// at compile time (the var _ guard also does this; this makes it explicit in the
// test binary).
func TestSDKAdapterSatisfiesInterface(t *testing.T) {
	var _ APIClient = (*SDKAdapter)(nil)
}
