package telegramadapter

import (
	"reflect"
	"strings"
	"testing"
)

// chan1001 is the peer every message in the representative slice belongs to.
var chan1001 = TGPeer{Kind: PeerChannel, ID: 1001}

func userPeer(id int64) *TGPeer { return &TGPeer{Kind: PeerUser, ID: id} }

// TestMapMessages_RepresentativeThread is the primary golden test: a real-shaped
// slice — channel root + human reply carrying hashtags + a reply-to-a-reply + a
// forwarded message + a message with media — mapped through MapMessages, with the
// full []Post asserted (ids, parent linkage from reply-to, author, verbatim
// hashtag text, timestamps, forwarded flag, attachments).
func TestMapMessages_RepresentativeThread(t *testing.T) {
	msgs := []TGMessage{
		{ // root: a channel broadcast (no FromID) — often just a link
			ID:      100,
			Peer:    chan1001,
			Date:    1000,
			Message: "Check this https://youtu.be/x",
		},
		{ // human reply that carries the hashtags (the spec's key case)
			ID:      101,
			Peer:    chan1001,
			FromID:  userPeer(42),
			Date:    1005,
			Message: "#Research #Video",
			ReplyTo: &TGReplyTo{ReplyToMsgID: 100},
		},
		{ // reply to a reply — nested chain must linearize via reply_to_msg_id
			ID:      102,
			Peer:    chan1001,
			FromID:  userPeer(43),
			Date:    1010,
			Message: "agreed, downloading",
			ReplyTo: &TGReplyTo{ReplyToMsgID: 101},
		},
		{ // forwarded message — presence of FwdFrom sets the flag, no parent
			ID:      103,
			Peer:    chan1001,
			FromID:  userPeer(44),
			Date:    1015,
			Message: "reposting an announcement",
			FwdFrom: &TGFwdFrom{FromName: "Some Channel", Date: 900},
		},
		{ // message with media (a document) replying to the root
			ID:      104,
			Peer:    chan1001,
			FromID:  userPeer(45),
			Date:    1020,
			Message: "spec attached #ToDownload",
			ReplyTo: &TGReplyTo{ReplyToMsgID: 100},
			Media:   &TGMedia{Kind: MediaDocument, ID: 99999, MIME: "application/pdf", FileName: "spec.pdf"},
		},
	}

	got, err := MapMessages(msgs)
	if err != nil {
		t.Fatalf("MapMessages error: %v", err)
	}

	want := []Post{
		{
			ID:            "100",
			ThreadID:      "channel:1001",
			ParentID:      "", // root — no reply-to
			AuthorID:      "channel:1001",
			Text:          "Check this https://youtu.be/x",
			TimestampUnix: 1000,
			IsForwarded:   false,
		},
		{
			ID:            "101",
			ThreadID:      "channel:1001",
			ParentID:      "100",
			AuthorID:      "user:42",
			Text:          "#Research #Video",
			TimestampUnix: 1005,
		},
		{
			ID:            "102",
			ThreadID:      "channel:1001",
			ParentID:      "101", // reply-to-a-reply
			AuthorID:      "user:43",
			Text:          "agreed, downloading",
			TimestampUnix: 1010,
		},
		{
			ID:            "103",
			ThreadID:      "channel:1001",
			ParentID:      "",
			AuthorID:      "user:44",
			Text:          "reposting an announcement",
			TimestampUnix: 1015,
			IsForwarded:   true,
		},
		{
			ID:            "104",
			ThreadID:      "channel:1001",
			ParentID:      "100",
			AuthorID:      "user:45",
			Text:          "spec attached #ToDownload",
			TimestampUnix: 1020,
			Attachments: []Attachment{
				{ID: "99999", MIME: "application/pdf", FileName: "spec.pdf", SHA256: ""},
			},
		},
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("MapMessages mismatch\n got: %#v\nwant: %#v", got, want)
	}
}

// TestMapMessages_HashtagTextPreservedVerbatim guards the core Thready
// requirement that a reply's hashtag-bearing text survives mapping untouched
// (extraction happens downstream in threadreader; the adapter must not mangle it).
func TestMapMessages_HashtagTextPreservedVerbatim(t *testing.T) {
	const text = "leading #Tag mid-#not_a_boundary end #Проект #CamelCase!"
	got, err := MapMessages([]TGMessage{{ID: 1, Peer: chan1001, Date: 1, Message: text}})
	if err != nil {
		t.Fatalf("MapMessages error: %v", err)
	}
	if got[0].Text != text {
		t.Errorf("Text = %q, want verbatim %q", got[0].Text, text)
	}
}

// TestMapMessages_RootDetection covers the "missing reply-to (root)" edge: a
// message with no reply header, and one whose header carries a zero msg id, both
// map to an empty ParentID.
func TestMapMessages_RootDetection(t *testing.T) {
	msgs := []TGMessage{
		{ID: 1, Peer: chan1001, Date: 1, Message: "no reply header"},
		{ID: 2, Peer: chan1001, Date: 2, Message: "zero reply id", ReplyTo: &TGReplyTo{ReplyToMsgID: 0}},
	}
	got, err := MapMessages(msgs)
	if err != nil {
		t.Fatalf("MapMessages error: %v", err)
	}
	for _, p := range got {
		if p.ParentID != "" {
			t.Errorf("post %s ParentID = %q, want empty (root)", p.ID, p.ParentID)
		}
	}
}

// TestMapMessages_Empty verifies empty/nil input is not an error and yields no posts.
func TestMapMessages_Empty(t *testing.T) {
	for name, in := range map[string][]TGMessage{"nil": nil, "empty": {}} {
		t.Run(name, func(t *testing.T) {
			got, err := MapMessages(in)
			if err != nil {
				t.Fatalf("MapMessages error: %v", err)
			}
			if len(got) != 0 {
				t.Fatalf("want 0 posts, got %d", len(got))
			}
		})
	}
}

// TestMapMessages_MediaOnly covers the media-only edge: no text, just an
// attachment. Text is empty and the single attachment is mapped.
func TestMapMessages_MediaOnly(t *testing.T) {
	got, err := MapMessages([]TGMessage{{
		ID:    7,
		Peer:  chan1001,
		Date:  1,
		Media: &TGMedia{Kind: MediaPhoto, ID: 555},
	}})
	if err != nil {
		t.Fatalf("MapMessages error: %v", err)
	}
	if got[0].Text != "" {
		t.Errorf("Text = %q, want empty", got[0].Text)
	}
	want := []Attachment{{ID: "555", MIME: "image/jpeg"}}
	if !reflect.DeepEqual(got[0].Attachments, want) {
		t.Fatalf("attachments = %#v, want %#v", got[0].Attachments, want)
	}
}

// TestMapMessages_MIMEInference exercises the inferMIME rules: photo → JPEG,
// document with explicit MIME kept verbatim, document without MIME derived from
// the filename extension, and the octet-stream fallback.
func TestMapMessages_MIMEInference(t *testing.T) {
	cases := []struct {
		name  string
		media TGMedia
		want  string // exact, unless usePrefix
		pref  bool
	}{
		{name: "photo is jpeg", media: TGMedia{Kind: MediaPhoto, ID: 1}, want: "image/jpeg"},
		{name: "document mime verbatim", media: TGMedia{Kind: MediaDocument, ID: 2, MIME: "video/mp4"}, want: "video/mp4"},
		{name: "document infers from ext", media: TGMedia{Kind: MediaDocument, ID: 3, FileName: "cover.png"}, want: "image/png", pref: true},
		{name: "document octet-stream fallback", media: TGMedia{Kind: MediaDocument, ID: 4, FileName: "blob.unknownext"}, want: "application/octet-stream"},
		{name: "document no name no mime", media: TGMedia{Kind: MediaDocument, ID: 5}, want: "application/octet-stream"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := MapMessages([]TGMessage{{ID: 1, Peer: chan1001, Date: 1, Media: &tc.media}})
			if err != nil {
				t.Fatalf("MapMessages error: %v", err)
			}
			mime := got[0].Attachments[0].MIME
			if tc.pref {
				if !strings.HasPrefix(mime, tc.want) {
					t.Errorf("MIME = %q, want prefix %q", mime, tc.want)
				}
				return
			}
			if mime != tc.want {
				t.Errorf("MIME = %q, want %q", mime, tc.want)
			}
		})
	}
}

// TestMapMessages_ForumTopicGrouping proves forum-topic / getReplies thread
// grouping: two topics in one channel produce two distinct ThreadIDs while the
// per-message ParentID still tracks the immediate reply edge. GroupByThread then
// buckets them.
func TestMapMessages_ForumTopicGrouping(t *testing.T) {
	msgs := []TGMessage{
		// Topic A (root msg 50): a direct reply to the topic root.
		{ID: 200, Peer: chan1001, FromID: userPeer(1), Date: 1, Message: "topic A first",
			ReplyTo: &TGReplyTo{ReplyToMsgID: 50, ForumTopic: true}},
		// Topic A: a nested reply — top id groups it, msg id links the parent.
		{ID: 201, Peer: chan1001, FromID: userPeer(2), Date: 2, Message: "topic A nested",
			ReplyTo: &TGReplyTo{ReplyToMsgID: 200, ReplyToTopID: 50}},
		// Topic B (root msg 60): a different topic in the same channel.
		{ID: 202, Peer: chan1001, FromID: userPeer(3), Date: 3, Message: "topic B first",
			ReplyTo: &TGReplyTo{ReplyToMsgID: 60, ForumTopic: true}},
	}
	got, err := MapMessages(msgs)
	if err != nil {
		t.Fatalf("MapMessages error: %v", err)
	}

	wantThread := map[string]string{"200": "channel:1001/50", "201": "channel:1001/50", "202": "channel:1001/60"}
	wantParent := map[string]string{"200": "50", "201": "200", "202": "60"}
	for _, p := range got {
		if p.ThreadID != wantThread[p.ID] {
			t.Errorf("post %s ThreadID = %q, want %q", p.ID, p.ThreadID, wantThread[p.ID])
		}
		if p.ParentID != wantParent[p.ID] {
			t.Errorf("post %s ParentID = %q, want %q", p.ID, p.ParentID, wantParent[p.ID])
		}
	}

	buckets := GroupByThread(got)
	if len(buckets) != 2 {
		t.Fatalf("GroupByThread: want 2 threads, got %d (%v)", len(buckets), keys(buckets))
	}
	if len(buckets["channel:1001/50"]) != 2 {
		t.Errorf("topic A bucket size = %d, want 2", len(buckets["channel:1001/50"]))
	}
	if len(buckets["channel:1001/60"]) != 1 {
		t.Errorf("topic B bucket size = %d, want 1", len(buckets["channel:1001/60"]))
	}
}

// TestMapMessages_InvalidID confirms a non-positive message id is a hard error,
// not a silently-mapped zero post.
func TestMapMessages_InvalidID(t *testing.T) {
	for _, id := range []int{0, -1} {
		if _, err := MapMessages([]TGMessage{{ID: id, Peer: chan1001, Date: 1}}); err == nil {
			t.Errorf("MapMessages(id=%d) err = nil, want error", id)
		}
	}
}

// TestMapMessages_LargeIDPrecision guards 64-bit ids: a large peer id must
// render to its exact decimal digits (no float64 rounding), and a large message
// id round-trips too.
func TestMapMessages_LargeIDPrecision(t *testing.T) {
	got, err := MapMessages([]TGMessage{{
		ID:     2000000123,
		Peer:   TGPeer{Kind: PeerChannel, ID: 1234567890123456789},
		FromID: &TGPeer{Kind: PeerUser, ID: 9223372036854775807},
		Date:   1,
	}})
	if err != nil {
		t.Fatalf("MapMessages error: %v", err)
	}
	if got[0].ID != "2000000123" {
		t.Errorf("ID = %q, want 2000000123", got[0].ID)
	}
	if got[0].ThreadID != "channel:1234567890123456789" {
		t.Errorf("ThreadID = %q", got[0].ThreadID)
	}
	if got[0].AuthorID != "user:9223372036854775807" {
		t.Errorf("AuthorID = %q", got[0].AuthorID)
	}
}

// TestMapMessages_ChannelPostAuthorIsChannel documents that a broadcast with no
// FromID is authored by the channel peer itself (the identity the herald
// self-filter compares against).
func TestMapMessages_ChannelPostAuthorIsChannel(t *testing.T) {
	got, err := MapMessages([]TGMessage{{ID: 1, Peer: chan1001, Date: 1, Message: "broadcast"}})
	if err != nil {
		t.Fatalf("MapMessages error: %v", err)
	}
	if got[0].AuthorID != "channel:1001" {
		t.Errorf("AuthorID = %q, want channel:1001", got[0].AuthorID)
	}
}

func keys(m map[string][]Post) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
