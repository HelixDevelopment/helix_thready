package integration

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"digital.vasic.assetservice"
	"digital.vasic.callbacktask"
	"digital.vasic.downloadmanager"
	"digital.vasic.eventbusservice"
	"digital.vasic.metering"
	"digital.vasic.ocr"
	"digital.vasic.semsearch"
	"digital.vasic.skilldispatch"
	"digital.vasic.telegramadapter"
	"digital.vasic.threadreader"
	"digital.vasic.userservice"
)

// ---------------------------------------------------------------------------
// Bridges between the messenger-adapter Post shape and threadreader.Post.
// telegramadapter.Post mirrors threadreader.Post field-for-field but is a
// DISTINCT type in a DISTINCT module, so the wiring layer converts explicitly.
// ---------------------------------------------------------------------------

func telegramToThreadReader(p telegramadapter.Post) threadreader.Post {
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

func sha256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// ---------------------------------------------------------------------------
// Real skill_dispatch Skills that drive the REAL download/asset/OCR modules.
// ---------------------------------------------------------------------------

// downloadSkill: KindDownload. Fetches bytes with the REAL download_manager
// (sha256-verified) and stores them in the REAL asset_service content store.
type downloadSkill struct {
	mgr     *downloadmanager.Manager
	store   *assetservice.ContentStore
	url     string
	dest    string
	wantSHA string

	runs    atomic.Int64
	assetID string
	nbytes  int64
}

func (s *downloadSkill) Name() string             { return "video.download" }
func (s *downloadSkill) Kind() skilldispatch.Kind { return skilldispatch.KindDownload }
func (s *downloadSkill) Match(p skilldispatch.Post) bool {
	return p.HasAnyHashtag("ToDownload", "Video")
}

func (s *downloadSkill) Run(ctx context.Context, _ skilldispatch.Post) (skilldispatch.Result, error) {
	s.runs.Add(1)
	id, err := s.mgr.Enqueue(downloadmanager.TaskSpec{
		URL:            s.url,
		DestPath:       s.dest,
		Segments:       4,
		ExpectedSHA256: s.wantSHA,
	})
	if err != nil {
		return skilldispatch.Result{}, err
	}
	upd, err := s.mgr.Wait(ctx, id)
	if err != nil {
		return skilldispatch.Result{}, err
	}
	if upd.State != downloadmanager.StateSucceeded {
		return skilldispatch.Result{}, skilldispatch.Permanent(
			fmt.Errorf("download ended in state %s: %s", upd.State, upd.Err))
	}
	f, err := os.Open(s.dest)
	if err != nil {
		return skilldispatch.Result{}, err
	}
	defer f.Close()
	aid, size, err := s.store.Put(f)
	if err != nil {
		return skilldispatch.Result{}, err
	}
	s.assetID = aid
	s.nbytes = size
	return skilldispatch.Result{
		SkillName: s.Name(),
		Output:    "asset:" + aid,
		Artifacts: []string{aid},
	}, nil
}

// ocrSkill: KindAnalyze. Runs the REAL ocr_adapter (real tesseract) over a real PNG.
type ocrSkill struct {
	prov    *ocr.TesseractProvider
	imgPath string

	runs atomic.Int64
	text string
}

func (s *ocrSkill) Name() string                    { return "vision.ocr" }
func (s *ocrSkill) Kind() skilldispatch.Kind        { return skilldispatch.KindAnalyze }
func (s *ocrSkill) Match(p skilldispatch.Post) bool { return p.HasAnyHashtag("Video", "Image") }

func (s *ocrSkill) Run(ctx context.Context, _ skilldispatch.Post) (skilldispatch.Result, error) {
	s.runs.Add(1)
	res, err := s.prov.Recognize(ctx, s.imgPath)
	if err != nil {
		return skilldispatch.Result{}, err
	}
	s.text = res.FullText
	return skilldispatch.Result{
		SkillName: s.Name(),
		Output:    res.FullText,
		Artifacts: []string{"ocr:" + res.Engine},
	}, nil
}

// researchSkill: KindResearch. The later stage in the precedence chain.
type researchSkill struct {
	runs atomic.Int64
}

func (s *researchSkill) Name() string                    { return "topic.research" }
func (s *researchSkill) Kind() skilldispatch.Kind        { return skilldispatch.KindResearch }
func (s *researchSkill) Match(p skilldispatch.Post) bool { return p.HasHashtag("Research") }

func (s *researchSkill) Run(_ context.Context, _ skilldispatch.Post) (skilldispatch.Result, error) {
	s.runs.Add(1)
	return skilldispatch.Result{SkillName: s.Name(), Output: "researched"}, nil
}

// busSink bridges skill_dispatch step events onto the REAL event_bus_service.
type busSink struct{ bus *eventbusservice.Bus }

func (s busSink) Emit(e skilldispatch.StepEvent) {
	ev := eventbusservice.
		NewEvent("pipeline."+e.Type.String(), e.Type.String(), e).
		WithMetadata("post_id", e.PostID)
	_, _ = s.bus.Publish(ev)
}

// TestThreadyPipelineEndToEnd wires the real Thready processing flow across the
// committed modules and proves they compose: ingest -> dispatch -> download+store
// -> OCR -> index+search -> events -> callback -> metering.
func TestThreadyPipelineEndToEnd(t *testing.T) {
	if _, err := exec.LookPath("tesseract"); err != nil {
		t.Skip("tesseract not installed; the OCR leg of the capstone requires it")
	}
	if _, err := exec.LookPath("convert"); err != nil {
		t.Skip("ImageMagick convert not installed; needed to synthesise the OCR image")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	// -----------------------------------------------------------------------
	// 0. IDENTITY / RBAC (user_service): authenticate the account owner. Its
	//    account id is the tenant used by asset ownership + metering below.
	// -----------------------------------------------------------------------
	const password = "operator-strong-pass-2026"
	hasher := userservice.NewHasher(4096) // real PBKDF2, low cost for test speed
	pwHash, err := hasher.Hash(password)
	if err != nil {
		t.Fatalf("password hash: %v", err)
	}
	if err := userservice.Verify(pwHash, password); err != nil {
		t.Fatalf("password verify: %v", err)
	}
	const accountID = "acct-operator-1"
	owner := userservice.User{
		ID:           "user-op",
		Email:        "op@thready.test",
		PasswordHash: pwHash,
		Memberships: []userservice.Membership{{
			UserID: "user-op", AccountID: accountID, Role: userservice.RoleAccountAdmin,
		}},
	}
	enf := userservice.NewEnforcer()
	if !enf.Allow(owner, accountID, userservice.PermPostsWrite) {
		t.Fatal("owner should be allowed posts:write")
	}
	if !enf.Allow(owner, accountID, userservice.PermAssetsWrite) {
		t.Fatal("owner should be allowed assets:write")
	}

	// -----------------------------------------------------------------------
	// 1. INGEST (telegram_adapter -> threadreader): assemble a realistic thread.
	//    Root carries #Video #Research; an organic reply adds #ToDownload; a
	//    system/bot reply must be filtered; one reply carries media.
	// -----------------------------------------------------------------------
	const (
		channelPeer = "channel:1001"
		botAuthor   = "user:9999"
		videoURL    = "https://videos.example/clip.mp4"
	)
	tgMsgs := []telegramadapter.TGMessage{
		{ // root (channel broadcast, no reply-to)
			ID: 1, Peer: telegramadapter.TGPeer{Kind: telegramadapter.PeerChannel, ID: 1001},
			Date: 1000, Message: "New lab drop " + videoURL + " #Video #Research",
		},
		{ // organic human reply that adds the download tag
			ID: 2, Peer: telegramadapter.TGPeer{Kind: telegramadapter.PeerChannel, ID: 1001},
			FromID: &telegramadapter.TGPeer{Kind: telegramadapter.PeerUser, ID: 5001},
			Date:   1001, Message: "Please grab this one #ToDownload",
			ReplyTo: &telegramadapter.TGReplyTo{ReplyToMsgID: 1},
		},
		{ // system/bot status reply — must be excluded by the assembler
			ID: 3, Peer: telegramadapter.TGPeer{Kind: telegramadapter.PeerChannel, ID: 1001},
			FromID: &telegramadapter.TGPeer{Kind: telegramadapter.PeerUser, ID: 9999},
			Date:   1002, Message: "processing…",
			ReplyTo: &telegramadapter.TGReplyTo{ReplyToMsgID: 1},
		},
		{ // organic human reply with media
			ID: 4, Peer: telegramadapter.TGPeer{Kind: telegramadapter.PeerChannel, ID: 1001},
			FromID: &telegramadapter.TGPeer{Kind: telegramadapter.PeerUser, ID: 5002},
			Date:   1003, Message: "chart attached",
			ReplyTo: &telegramadapter.TGReplyTo{ReplyToMsgID: 1},
			Media:   &telegramadapter.TGMedia{Kind: telegramadapter.MediaPhoto, ID: 77},
		},
	}
	tgPosts, err := telegramadapter.MapMessages(tgMsgs)
	if err != nil {
		t.Fatalf("telegramadapter.MapMessages: %v", err)
	}
	trPosts := make([]threadreader.Post, 0, len(tgPosts))
	for _, p := range tgPosts {
		trPosts = append(trPosts, telegramToThreadReader(p))
	}
	asm := threadreader.NewAssembler(botAuthor)
	thread, err := asm.Assemble(trPosts)
	if err != nil {
		t.Fatalf("threadreader.Assemble: %v", err)
	}
	if thread.Root.ID != "1" {
		t.Fatalf("root id = %q, want 1", thread.Root.ID)
	}
	// bot reply (id 3) filtered -> 2 organic replies remain, chronological.
	if len(thread.Replies) != 2 {
		t.Fatalf("organic replies = %d, want 2 (bot reply must be filtered)", len(thread.Replies))
	}
	for _, r := range thread.Replies {
		if r.AuthorID == botAuthor {
			t.Fatalf("system/bot reply %q leaked into organic replies", r.ID)
		}
	}
	tags := thread.Hashtags()
	for _, want := range []string{"Video", "Research", "ToDownload"} {
		if !containsStr(tags, want) {
			t.Fatalf("thread hashtags %v missing %q", tags, want)
		}
	}
	// Independent check of the extractor on the raw root text.
	if got := threadreader.ExtractHashtags(thread.Root.Text); !containsStr(got, "Video") || !containsStr(got, "Research") {
		t.Fatalf("ExtractHashtags(root) = %v, want Video+Research", got)
	}
	postText := thread.Root.Text
	for _, r := range thread.Replies {
		postText += "\n" + r.Text
	}

	// -----------------------------------------------------------------------
	// 2. Real byte source (net/http/httptest) with Range support for the
	//    download_manager, plus content store + OCR image.
	// -----------------------------------------------------------------------
	payload := []byte(strings.Repeat(
		"Helix Thready integration capstone — deterministic clip payload block.\n", 64))
	wantSHA := sha256Hex(payload)
	fileSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// http.ServeContent gives real Range/206 support, exercising the
		// segmented/resumable download path for real.
		http.ServeContent(w, r, "clip.mp4", time.Unix(1000, 0), newSeeker(payload))
	}))
	defer fileSrv.Close()

	tmp := t.TempDir()
	dest := filepath.Join(tmp, "clip.mp4")

	store, err := assetservice.NewContentStore(filepath.Join(tmp, "assets"))
	if err != nil {
		t.Fatalf("NewContentStore: %v", err)
	}

	imgPath := makeOCRImage(t, "Helix Thready 42")

	mgr := downloadmanager.New(downloadmanager.Config{Workers: 2, MaxRetries: 2})
	mgr.Start()
	defer mgr.Shutdown(context.Background())

	// -----------------------------------------------------------------------
	// 3. DISPATCH (skill_dispatch): register skills in a DELIBERATELY reversed
	//    order and prove the orderer imposes stage precedence, then run twice to
	//    prove idempotent single-claim.
	// -----------------------------------------------------------------------
	dl := &downloadSkill{mgr: mgr, store: store, url: fileSrv.URL, dest: dest, wantSHA: wantSHA}
	oc := &ocrSkill{prov: ocr.NewTesseractProvider("eng"), imgPath: imgPath}
	rs := &researchSkill{}

	reg := skilldispatch.NewRegistry()
	reg.Register(rs, oc, dl) // reversed vs. precedence on purpose

	dispatchPost := skilldispatch.Post{
		ID:       "post-" + thread.Root.ID,
		Hashtags: tags,
		Text:     postText,
		Links:    []string{videoURL},
	}

	// Assert the orderer yields download < analyze < research regardless of
	// registration order.
	ordered := skilldispatch.OrderByPrecedence(reg.Resolve(dispatchPost))
	if len(ordered) != 3 {
		t.Fatalf("resolved skills = %d, want 3", len(ordered))
	}
	gotKinds := []skilldispatch.Kind{ordered[0].Kind(), ordered[1].Kind(), ordered[2].Kind()}
	wantKinds := []skilldispatch.Kind{skilldispatch.KindDownload, skilldispatch.KindAnalyze, skilldispatch.KindResearch}
	for i := range wantKinds {
		if gotKinds[i] != wantKinds[i] {
			t.Fatalf("precedence[%d] = %v, want %v (download-kind must precede research-kind)", i, gotKinds[i], wantKinds[i])
		}
	}

	bus := eventbusservice.NewDefault()
	defer bus.Close()

	disp := skilldispatch.NewDispatcher(reg,
		skilldispatch.WithEventSink(busSink{bus: bus}),
		skilldispatch.WithRetry(skilldispatch.RetryPolicy{MaxAttempts: 2, BaseDelay: 10 * time.Millisecond, MaxDelay: 50 * time.Millisecond}),
	)

	res1, err := disp.Process(ctx, dispatchPost)
	if err != nil {
		t.Fatalf("first Process: %v", err)
	}
	if res1.State != skilldispatch.PostCompleted {
		t.Fatalf("first Process state = %v, want completed; steps=%+v", res1.State, res1.Steps)
	}
	// Duplicate trigger for the SAME post -> rejected, no skill re-runs.
	res2, err := disp.Process(ctx, dispatchPost)
	if err != nil {
		t.Fatalf("second Process: %v", err)
	}
	if res2.State != skilldispatch.PostRejected || res2.Claimed {
		t.Fatalf("second Process = %+v, want rejected/unclaimed (idempotent single-claim)", res2)
	}
	if got := disp.Claims().State(dispatchPost.ID); got != skilldispatch.ClaimDone {
		t.Fatalf("claim state = %v, want done", got)
	}
	if dl.runs.Load() != 1 || oc.runs.Load() != 1 || rs.runs.Load() != 1 {
		t.Fatalf("skill run counts = download:%d ocr:%d research:%d, want 1/1/1 (exactly-once)",
			dl.runs.Load(), oc.runs.Load(), rs.runs.Load())
	}

	// -----------------------------------------------------------------------
	// 4. ASSET stored + integrity-verified on read + tamper detected.
	// -----------------------------------------------------------------------
	if dl.assetID != wantSHA {
		t.Fatalf("stored content id = %q, want sha256 of payload %q", dl.assetID, wantSHA)
	}
	rc, err := store.Get(dl.assetID)
	if err != nil {
		t.Fatalf("store.Get: %v", err)
	}
	gotBytes, _ := io.ReadAll(rc)
	rc.Close()
	if sha256Hex(gotBytes) != wantSHA {
		t.Fatalf("retrieved bytes hash mismatch")
	}
	// Tamper the on-disk blob and prove integrity verification catches it.
	blob := filepath.Join(store.Root(), dl.assetID[0:2], dl.assetID[2:4], dl.assetID)
	if err := os.WriteFile(blob, []byte("tampered-bytes"), 0o644); err != nil {
		t.Fatalf("tamper write: %v", err)
	}
	if _, err := store.Get(dl.assetID); !errors.Is(err, assetservice.ErrIntegrity) {
		t.Fatalf("tampered read err = %v, want ErrIntegrity", err)
	}

	// Record asset ownership under the authenticated tenant (identity -> asset).
	assetIx := assetservice.NewAssetIndex()
	assetIx.Put(assetservice.Asset{
		ID: "asset-" + dl.assetID[:12], SHA256: dl.assetID, Size: dl.nbytes,
		ContentType: "video/mp4", OriginalName: "clip.mp4", AccountID: accountID,
		CreatedAt: time.Unix(1000, 0).UTC(),
	})
	if a, ok := assetIx.Get("asset-" + dl.assetID[:12]); !ok || a.AccountID != accountID {
		t.Fatalf("asset ownership not recorded under account %q", accountID)
	}

	// -----------------------------------------------------------------------
	// 5. OCR text is real and non-empty.
	// -----------------------------------------------------------------------
	ocrText := strings.TrimSpace(oc.text)
	if ocrText == "" {
		t.Fatal("OCR produced empty text")
	}
	for _, w := range []string{"Helix", "42"} {
		if !strings.Contains(ocrText, w) {
			t.Fatalf("OCR text %q missing expected word %q", ocrText, w)
		}
	}

	// -----------------------------------------------------------------------
	// 6. INDEX + SEMANTIC QUERY (semantic_search): index post text + OCR text +
	//    a doc; a query for the OCR text must retrieve the OCR chunk (real cosine).
	// -----------------------------------------------------------------------
	eng := semsearch.NewEngine(nil, nil, semsearch.DefaultConfig())
	ocrFile := "ocr-1.md"
	chunks := []semsearch.Chunk{
		{ID: semsearch.ChunkID("post-1.md", "post", 1), FilePath: "post-1.md", Symbol: "post", Kind: semsearch.KindMarkdown, StartLine: 1, EndLine: 1, Content: postText},
		{ID: semsearch.ChunkID(ocrFile, "ocr", 1), FilePath: ocrFile, Symbol: "ocr", Kind: semsearch.KindMarkdown, StartLine: 1, EndLine: 1, Content: ocrText},
		{ID: semsearch.ChunkID("design.md", "doc", 1), FilePath: "design.md", Symbol: "doc", Kind: semsearch.KindMarkdown, StartLine: 1, EndLine: 1, Content: "Content-addressed asset storage verifies sha256 integrity on read and detects tampering."},
	}
	if err := eng.Index(ctx, chunks); err != nil {
		t.Fatalf("semsearch Index: %v", err)
	}
	hits, err := eng.Search(ctx, ocrText, 3)
	if err != nil {
		t.Fatalf("semsearch Search: %v", err)
	}
	if len(hits) == 0 {
		t.Fatal("semantic search returned no hits")
	}
	if hits[0].Chunk.FilePath != ocrFile {
		t.Fatalf("top hit = %q (score %.4f), want OCR chunk %q", hits[0].Chunk.FilePath, hits[0].Score, ocrFile)
	}

	// -----------------------------------------------------------------------
	// 7. EVENTS (event_bus_service): a durable subscriber replays the ordered
	//    pipeline events emitted by the dispatcher through the bus.
	// -----------------------------------------------------------------------
	wantEvents := []string{
		"post.claimed",
		"step.started", "step.succeeded", // download
		"step.started", "step.succeeded", // analyze/OCR
		"step.started", "step.succeeded", // research
		"post.completed",
		"post.rejected", // the duplicate trigger
	}
	sub := bus.SubscribeDurable(eventbusservice.Filter{Subject: "pipeline.>"}, 0)
	defer bus.Unsubscribe(sub)
	gotEvents := make([]string, 0, len(wantEvents))
	deadline := time.After(10 * time.Second)
	for len(gotEvents) < len(wantEvents) {
		select {
		case ev := <-sub.C:
			gotEvents = append(gotEvents, ev.Type)
		case <-deadline:
			t.Fatalf("durable replay timeout: got %v", gotEvents)
		}
	}
	for i := range wantEvents {
		if gotEvents[i] != wantEvents[i] {
			t.Fatalf("event[%d] = %q, want %q (full=%v)", i, gotEvents[i], wantEvents[i], gotEvents)
		}
	}

	// -----------------------------------------------------------------------
	// 8. CALLBACK (callback_task): fire an HMAC-signed completion webhook to an
	//    httptest receiver that INDEPENDENTLY recomputes the HMAC.
	// -----------------------------------------------------------------------
	secret := []byte("thready-webhook-shared-secret")
	var callbackOK atomic.Bool
	cbSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		mac := hmac.New(sha256.New, secret)
		mac.Write(body)
		want := "sha256=" + hex.EncodeToString(mac.Sum(nil))
		if hmac.Equal([]byte(want), []byte(r.Header.Get(callbacktask.SignatureHeader))) {
			callbackOK.Store(true)
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer cbSrv.Close()

	sink := &callbacktask.WebhookSink{URL: cbSrv.URL, Secret: secret, MaxRetries: 2}
	env := callbacktask.Envelope{
		TaskID: dispatchPost.ID, State: callbacktask.StateSucceeded, Progress: 1.0,
		ResultRef: "asset:" + dl.assetID, TS: time.Unix(1000, 0).UTC(),
	}
	if err := sink.Notify(ctx, env); err != nil {
		t.Fatalf("callback delivery: %v", err)
	}
	if !callbackOK.Load() {
		t.Fatal("callback receiver did not independently verify the HMAC")
	}

	// -----------------------------------------------------------------------
	// 9. METERING (metering): record usage for the tenant, bill, assert a line.
	// -----------------------------------------------------------------------
	rec := metering.NewRecorder()
	usageTS := time.Date(2026, time.July, 15, 12, 0, 0, 0, time.UTC).Unix()
	rec.RecordUsage(accountID, metering.MetricPostsProcessed, 1, usageTS, "post")
	rec.RecordUsage(accountID, metering.MetricBytesDownloaded, dl.nbytes, usageTS, "byte")
	rec.RecordUsage(accountID, metering.MetricAssetsStored, 1, usageTS, "asset")
	rec.RecordUsage(accountID, metering.MetricSearches, 1, usageTS, "search")

	plan := metering.NewPlan("Pro", 4900,
		metering.MetricRate{Metric: metering.MetricPostsProcessed, IncludedUnits: 0, BlockUnits: 1, CentsPerBlock: 100},
	)
	inv := metering.NewBiller(plan, rec).Bill(accountID, metering.MonthUTC(2026, time.July))
	var postsLine *metering.LineItem
	for i := range inv.LineItems {
		if inv.LineItems[i].Metric == metering.MetricPostsProcessed {
			postsLine = &inv.LineItems[i]
		}
	}
	if postsLine == nil {
		t.Fatalf("invoice has no posts_processed line: %+v", inv.LineItems)
	}
	if postsLine.OverageUnits != 1 || postsLine.AmountCents != 100 {
		t.Fatalf("posts_processed line = %+v, want overage 1 / 100c", *postsLine)
	}
	if inv.TotalCents != 5000 {
		t.Fatalf("invoice total = %d, want 5000 (base 4900 + 100)", inv.TotalCents)
	}

	// -----------------------------------------------------------------------
	// FINAL: single consolidated statement of everything the capstone proved.
	// -----------------------------------------------------------------------
	t.Logf("CAPSTONE PROVEN: post %s processed exactly once (claim=%s); asset %s stored+integrity-verified (tamper caught); OCR %q indexed+searchable; %d events replayed in order; callback HMAC verified; invoice total %d cents",
		dispatchPost.ID, disp.Claims().State(dispatchPost.ID), dl.assetID[:12], ocrText, len(gotEvents), inv.TotalCents)
}

// containsStr reports whether xs contains s.
func containsStr(xs []string, s string) bool {
	for _, x := range xs {
		if x == s {
			return true
		}
	}
	return false
}

// newSeeker adapts a byte slice to the io.ReadSeeker http.ServeContent needs.
func newSeeker(b []byte) *strings.Reader { return strings.NewReader(string(b)) }

// makeOCRImage renders a real PNG with ImageMagick's convert, mirroring the
// ocr_adapter test's known-good invocation.
func makeOCRImage(t *testing.T, text string) string {
	t.Helper()
	out := filepath.Join(t.TempDir(), "ocr.png")
	cmd := exec.Command("convert",
		"-size", "600x120", "xc:white",
		"-gravity", "center",
		"-pointsize", "40",
		"-annotate", "0", text,
		out,
	)
	if b, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("convert failed: %v\n%s", err, b)
	}
	return out
}
