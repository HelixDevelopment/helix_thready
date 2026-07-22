package threadreader

import (
	"context"
	"errors"
	"reflect"
	"sort"
	"testing"
)

// --- in-memory fake MessageSource -------------------------------------------

// fakeSource is an in-memory MessageSource for tests. It returns the raw posts it
// was seeded with, in the exact (possibly shuffled) order given, so the Assembler's
// determinism is exercised for real rather than assumed.
type fakeSource struct {
	threads map[string][]Post
	err     error
}

func (f *fakeSource) FetchThread(_ context.Context, threadID string) ([]Post, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.threads[threadID], nil
}

// Author IDs used in the fixture. threadyBot is the system's own bot.
const (
	rootID     = "root-1"
	threadID   = "t-42"
	threadyBot = "thready-bot" // the system's own processing-reply author
	otherBot   = "weatherbot"  // a DIFFERENT bot — organic traffic, must be KEPT
)

// seedRealisticThread returns a realistic thread — deliberately SHUFFLED and with a
// duplicate — modelling the messenger-ingestion.md §6 diagram:
//
//	root (link only, human)
//	 ├─ reply: "#Video #ToDownload" (human bob)
//	 ├─ reply: system "Processing started" (thready-bot)   <- excluded
//	 ├─ reply: "agreed, also #Research" (human carol)
//	 ├─ reply: forwarded post + attachment, "#Archive" (human dave, forwarded)
//	 ├─ reply: system "Processing complete" (thready-bot)  <- excluded
//	 └─ reply: "nice, #Video again" from a DIFFERENT bot   <- KEPT (organic)
//
// The organic replies (bob, carol, dave, weatherbot) must survive; the two
// thready-bot replies must not.
func seedRealisticThread() []Post {
	root := Post{
		ID: rootID, ThreadID: threadID, ParentID: "",
		AuthorID: "user-alice", Text: "check this out https://youtu.be/dQw4w9WgXcQ",
		TimestampUnix: 1000,
	}
	replyBob := Post{
		ID: "m-bob", ThreadID: threadID, ParentID: rootID,
		AuthorID: "user-bob", Text: "great find #Video #ToDownload",
		TimestampUnix: 1002,
	}
	sysStart := Post{
		ID: "m-sys-1", ThreadID: threadID, ParentID: rootID,
		AuthorID: threadyBot, Text: "Processing started…",
		TimestampUnix: 1003,
	}
	replyCarol := Post{
		ID: "m-carol", ThreadID: threadID, ParentID: "m-bob",
		AuthorID: "user-carol", Text: "agreed, also #Research this channel",
		TimestampUnix: 1004,
	}
	replyForward := Post{
		ID: "m-dave", ThreadID: threadID, ParentID: rootID,
		AuthorID: "user-dave", Text: "forwarding the source #Archive",
		TimestampUnix: 1005, IsForwarded: true,
		Attachments: []Attachment{{
			ID: "att-1", MIME: "application/pdf", FileName: "paper.pdf",
			SHA256: "abc123",
		}},
	}
	sysDone := Post{
		ID: "m-sys-2", ThreadID: threadID, ParentID: rootID,
		AuthorID: threadyBot, Text: "Processing complete ✅",
		TimestampUnix: 1006,
	}
	replyOtherBot := Post{
		ID: "m-wbot", ThreadID: threadID, ParentID: rootID,
		AuthorID: otherBot, Text: "nice, #Video again",
		TimestampUnix: 1007,
	}

	// Shuffled, and with replyBob duplicated to exercise dedup.
	return []Post{
		sysDone, replyForward, replyBob, root, replyCarol,
		replyBob, // duplicate — must collapse
		sysStart, replyOtherBot,
	}
}

func newTestReader() *ThreadReader {
	src := &fakeSource{threads: map[string][]Post{
		threadID: seedRealisticThread(),
	}}
	return New(src, threadyBot)
}

func replyIDs(t *Thread) []string {
	ids := make([]string, len(t.Replies))
	for i, r := range t.Replies {
		ids[i] = r.ID
	}
	return ids
}

// --- Test 1: only organic posts survive; system/bot replies excluded --------

func TestAssemble_ExcludesSystemReplies(t *testing.T) {
	thread, err := newTestReader().Read(context.Background(), threadID)
	if err != nil {
		t.Fatalf("Read: unexpected error: %v", err)
	}
	for _, r := range thread.Replies {
		if r.AuthorID == threadyBot {
			t.Errorf("system/bot reply %q by %q leaked into organic thread", r.ID, r.AuthorID)
		}
	}
	// The two thready-bot messages must be gone; the DIFFERENT bot must remain.
	got := replyIDs(thread)
	want := []string{"m-bob", "m-carol", "m-dave", "m-wbot"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("organic replies = %v, want %v", got, want)
	}
	if thread.Root.ID != rootID {
		t.Errorf("root = %q, want %q", thread.Root.ID, rootID)
	}
}

// --- Test 2: chronological order even when the input is shuffled ------------

func TestAssemble_ChronologicalOrderFromShuffledInput(t *testing.T) {
	thread, err := newTestReader().Read(context.Background(), threadID)
	if err != nil {
		t.Fatalf("Read: unexpected error: %v", err)
	}
	prev := int64(-1)
	for _, r := range thread.Replies {
		if r.TimestampUnix < prev {
			t.Fatalf("replies not in chronological order: %d after %d", r.TimestampUnix, prev)
		}
		prev = r.TimestampUnix
	}
	// Explicit expected order by timestamp: bob(1002) carol(1004) dave(1005) wbot(1007).
	if got, want := replyIDs(thread), []string{"m-bob", "m-carol", "m-dave", "m-wbot"}; !reflect.DeepEqual(got, want) {
		t.Errorf("order = %v, want %v", got, want)
	}
}

// A second, independently shuffled permutation must yield the identical thread —
// determinism, not luck.
func TestAssemble_DeterministicAcrossPermutations(t *testing.T) {
	base := seedRealisticThread()
	perm := []Post{base[3], base[7], base[0], base[1], base[6], base[2], base[4], base[5]}

	a := NewAssembler(threadyBot)
	t1, err := a.Assemble(base)
	if err != nil {
		t.Fatalf("assemble base: %v", err)
	}
	t2, err := a.Assemble(perm)
	if err != nil {
		t.Fatalf("assemble perm: %v", err)
	}
	if !reflect.DeepEqual(t1, t2) {
		t.Errorf("assembly not deterministic across input permutations:\n base=%v\n perm=%v",
			replyIDs(t1), replyIDs(t2))
	}
}

// --- Test 3: hashtags extracted from root AND replies (the reply-tags case) --

func TestThread_HashtagsUnionAcrossChain(t *testing.T) {
	thread, err := newTestReader().Read(context.Background(), threadID)
	if err != nil {
		t.Fatalf("Read: unexpected error: %v", err)
	}
	// Root is a bare link (no tags); every tag lives on a reply. The union must
	// still surface them all — this is the core "#tags added as a reply" case.
	got := append([]string(nil), thread.Hashtags()...)
	sort.Strings(got)
	want := []string{"Archive", "Research", "ToDownload", "Video"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("union hashtags = %v, want %v (root had none; tags came from replies)", got, want)
	}
	// #Video appears on two replies but must be deduped to one.
	count := 0
	for _, h := range thread.Hashtags() {
		if h == "Video" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("#Video appears %d times in union, want 1 (deduped)", count)
	}
}

// --- Test 4: forwarded message content preserved ----------------------------

func TestAssemble_ForwardedMessagePreserved(t *testing.T) {
	thread, err := newTestReader().Read(context.Background(), threadID)
	if err != nil {
		t.Fatalf("Read: unexpected error: %v", err)
	}
	var fwd *Post
	for i := range thread.Replies {
		if thread.Replies[i].ID == "m-dave" {
			fwd = &thread.Replies[i]
		}
	}
	if fwd == nil {
		t.Fatal("forwarded reply m-dave was dropped")
	}
	if !fwd.IsForwarded {
		t.Error("IsForwarded flag lost on assembled forwarded reply")
	}
	if fwd.Text != "forwarding the source #Archive" {
		t.Errorf("forwarded text mangled: %q", fwd.Text)
	}
	if len(fwd.Attachments) != 1 || fwd.Attachments[0].SHA256 != "abc123" {
		t.Errorf("forwarded attachment not preserved: %+v", fwd.Attachments)
	}
}

// --- Test 5: missing root → deterministic error -----------------------------

func TestAssemble_MissingRootIsDeterministicError(t *testing.T) {
	// Only replies; every ParentID points at an absent root.
	orphans := []Post{
		{ID: "r1", ThreadID: threadID, ParentID: "gone", AuthorID: "u1", Text: "hi", TimestampUnix: 5},
		{ID: "r2", ThreadID: threadID, ParentID: "gone", AuthorID: "u2", Text: "yo", TimestampUnix: 6},
	}
	_, err := NewAssembler(threadyBot).Assemble(orphans)
	if !errors.Is(err, ErrMissingRoot) {
		t.Fatalf("missing root: got err=%v, want ErrMissingRoot", err)
	}

	// Empty input is a distinct, deterministic error too.
	if _, err := NewAssembler().Assemble(nil); !errors.Is(err, ErrNoPosts) {
		t.Fatalf("empty input: got err=%v, want ErrNoPosts", err)
	}
}

// --- Test 6: duplicate posts deduped ----------------------------------------

func TestAssemble_DuplicatesDeduped(t *testing.T) {
	thread, err := newTestReader().Read(context.Background(), threadID)
	if err != nil {
		t.Fatalf("Read: unexpected error: %v", err)
	}
	seen := map[string]int{}
	for _, r := range thread.Replies {
		seen[r.ID]++
	}
	for id, n := range seen {
		if n != 1 {
			t.Errorf("reply %q appears %d times, want 1 (dedup failed)", id, n)
		}
	}
	// The fixture seeds m-bob twice; exactly four organic replies must remain.
	if len(thread.Replies) != 4 {
		t.Errorf("got %d replies, want 4 after dedup", len(thread.Replies))
	}
}

// A duplicated ROOT must also collapse and not corrupt assembly.
func TestAssemble_DuplicateRootDeduped(t *testing.T) {
	root := Post{ID: rootID, ThreadID: threadID, AuthorID: "u", Text: "hello", TimestampUnix: 1}
	reply := Post{ID: "c1", ThreadID: threadID, ParentID: rootID, AuthorID: "v", Text: "hi", TimestampUnix: 2}
	thread, err := NewAssembler().Assemble([]Post{root, reply, root})
	if err != nil {
		t.Fatalf("assemble: %v", err)
	}
	if thread.Root.ID != rootID || len(thread.Replies) != 1 {
		t.Errorf("duplicate root not handled: root=%q replies=%d", thread.Root.ID, len(thread.Replies))
	}
}

// --- Test 6b: deterministic multi-root tie-break (rootLess) -----------------

// When SEVERAL posts qualify as roots (empty or self-referential ParentID),
// Assemble must still pick a single, deterministic Root via rootLess
// (assembler.go): earliest TimestampUnix wins, and on an exact timestamp tie the
// lexicographically-smaller ID wins. The other fixtures only ever have one root
// candidate, so this is the case that actually reaches rootLess.
func TestAssemble_MultiRootTieBreak(t *testing.T) {
	// permutations returns several orderings of posts (every rotation plus the
	// reversal) so that input order cannot accidentally decide the winner.
	permutations := func(posts []Post) [][]Post {
		out := make([][]Post, 0, len(posts)+1)
		for shift := 0; shift < len(posts); shift++ {
			rot := make([]Post, 0, len(posts))
			rot = append(rot, posts[shift:]...)
			rot = append(rot, posts[:shift]...)
			out = append(out, rot)
		}
		rev := make([]Post, len(posts))
		for i := range posts {
			rev[len(posts)-1-i] = posts[i]
		}
		out = append(out, rev)
		return out
	}

	// assertStableRoot runs Assemble over every permutation and asserts (a) the
	// Root.ID is exactly wantRoot in each, and (b) the full assembled Thread is
	// byte-for-byte identical across permutations — i.e. shuffling the input never
	// changes the result.
	assertStableRoot := func(t *testing.T, posts []Post, wantRoot string) {
		t.Helper()
		var first *Thread
		for i, in := range permutations(posts) {
			thread, err := NewAssembler().Assemble(in)
			if err != nil {
				t.Fatalf("perm %d: unexpected error: %v", i, err)
			}
			if thread.Root.ID != wantRoot {
				t.Errorf("perm %d: Root.ID = %q, want %q", i, thread.Root.ID, wantRoot)
			}
			if first == nil {
				first = thread
				continue
			}
			if !reflect.DeepEqual(first, thread) {
				t.Errorf("perm %d: assembly is not deterministic across input order:\n first=%v root=%q\n this =%v root=%q",
					i, replyIDs(first), first.Root.ID, replyIDs(thread), thread.Root.ID)
			}
		}
	}

	// Case A: two roots, different timestamps. rootEarly deliberately carries the
	// lexicographically LARGER ID but the EARLIER timestamp, so a correct rootLess
	// must let timestamp dominate the ID tie-break.
	t.Run("earliest timestamp wins over smaller ID", func(t *testing.T) {
		rootEarly := Post{ID: "root-zzz", ThreadID: threadID, ParentID: "", AuthorID: "u1", Text: "early root", TimestampUnix: 1000}
		rootLate := Post{ID: "root-aaa", ThreadID: threadID, ParentID: "", AuthorID: "u2", Text: "late root", TimestampUnix: 2000}
		child := Post{ID: "c1", ThreadID: threadID, ParentID: "root-zzz", AuthorID: "u3", Text: "a reply", TimestampUnix: 3000}
		assertStableRoot(t, []Post{rootEarly, rootLate, child}, "root-zzz")
	})

	// Case B: two roots, SAME timestamp. The documented tie-break selects the
	// smaller ID. rootHi additionally uses a SELF-referential ParentID (ParentID ==
	// ID) to prove that self-root path is treated as a root candidate too.
	t.Run("equal timestamp: lexicographically smaller ID wins", func(t *testing.T) {
		rootHi := Post{ID: "root-b", ThreadID: threadID, ParentID: "root-b", AuthorID: "u1", Text: "root b", TimestampUnix: 5000}
		rootLo := Post{ID: "root-a", ThreadID: threadID, ParentID: "", AuthorID: "u2", Text: "root a", TimestampUnix: 5000}
		child := Post{ID: "c1", ThreadID: threadID, ParentID: "root-a", AuthorID: "u3", Text: "a reply", TimestampUnix: 6000}
		assertStableRoot(t, []Post{rootHi, rootLo, child}, "root-a")
	})
}

// --- Test 7: ExtractHashtags edge cases -------------------------------------

func TestExtractHashtags_EdgeCases(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want []string
	}{
		{"empty", "", nil},
		{"no tags", "just a plain sentence, nothing here", nil},
		{"single", "hello #World", []string{"World"}},
		{"multiple", "#a #b #c", []string{"a", "b", "c"}},
		{"case preserved", "#CamelCase #lower #UPPER", []string{"CamelCase", "lower", "UPPER"}},
		{"case-sensitive dedup keeps distinct", "#Tag #tag #Tag", []string{"Tag", "tag"}},
		{"dedup identical", "#dup #dup #dup", []string{"dup"}},
		{"adjacent trailing punctuation", "#Video. #ToDownload! (#Research), #end;", []string{"Video", "ToDownload", "Research", "end"}},
		{"leading and wrapped", "(#wrapped) [#bracket]", []string{"wrapped", "bracket"}},
		{"digits and underscores", "#v2 #to_download #_private #a1b2", []string{"v2", "to_download", "_private", "a1b2"}},
		{"unicode cyrillic", "#Исследование and #Видео", []string{"Исследование", "Видео"}},
		{"unicode cjk and accents", "#研究 #café", []string{"研究", "café"}},
		{"lone hash", "a # b ## c", nil},
		{"not-a-tag mid-token", "C# email a@b#c http://x/y#frag", nil},
		{"hash glued after word", "word#notag end", nil},
		{"newlines separate", "#one\n#two\t#three", []string{"one", "two", "three"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ExtractHashtags(tc.in)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("ExtractHashtags(%q) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}

// --- source error propagation (belt-and-braces) -----------------------------

func TestRead_SourceErrorPropagates(t *testing.T) {
	sentinel := errors.New("network down")
	tr := New(&fakeSource{err: sentinel}, threadyBot)
	if _, err := tr.Read(context.Background(), threadID); !errors.Is(err, sentinel) {
		t.Fatalf("source error not propagated: got %v", err)
	}
}
