package maxadapter

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// TestParseHistory_RepresentativeFrame is the primary golden test: it feeds the
// canned, research-shaped OneMe opcode-49 frame (testdata/history_frame.json)
// through ParseHistory and asserts the full []Post — ids, author, text, reply
// and forward linkage, forwarded flag, timestamps (ms->s) and attachments.
func TestParseHistory_RepresentativeFrame(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("testdata", "history_frame.json"))
	if err != nil {
		t.Fatalf("read testdata: %v", err)
	}

	got, err := ParseHistory(raw)
	if err != nil {
		t.Fatalf("ParseHistory returned error: %v", err)
	}

	want := []Post{
		{
			ID:            "1754258500000",
			ThreadID:      "1730000000001",
			ParentID:      "", // root post
			AuthorID:      "13446207",
			Text:          "Original post with a link https://example.org #Design",
			TimestampUnix: 1754258500, // 1754258500000 ms -> s
			IsForwarded:   false,
			Attachments: []Attachment{
				{ID: "11549759", MIME: "image/*", FileName: "", SHA256: ""},
			},
		},
		{
			ID:            "1754258510000",
			ThreadID:      "1730000000001",
			ParentID:      "1754258500000", // reply via link.messageId
			AuthorID:      "42",
			Text:          "#Wireframes here is the spec",
			TimestampUnix: 1754258510,
			IsForwarded:   false,
			Attachments: []Attachment{
				{ID: "990001", MIME: "application/pdf", FileName: "spec.pdf", SHA256: ""},
			},
		},
		{
			ID:            "1754258520000",
			ThreadID:      "1730000000001",
			ParentID:      "1754258510000", // reply via prevMessageId
			AuthorID:      "42",
			Text:          "and a follow-up",
			TimestampUnix: 1754258520,
			IsForwarded:   false,
		},
		{
			ID:            "1754258530000", // id given as a JSON string
			ThreadID:      "1730000000001",
			ParentID:      "", // FORWARD is cross-chat, not a within-thread parent
			AuthorID:      "99",
			Text:          "forwarded from another channel",
			TimestampUnix: 1754258530,
			IsForwarded:   true,
		},
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ParseHistory mismatch\n got: %#v\nwant: %#v", got, want)
	}
}

// TestParseHistory_ReplyLinkage isolates the two reply-encoding paths OneMe uses
// (an explicit link{type:REPLY,messageId} and a top-level prevMessageId) and the
// FORWARD path, since correct parent linkage is what lets the assembler rebuild
// a thread.
func TestParseHistory_ReplyLinkage(t *testing.T) {
	cases := []struct {
		name       string
		payload    string
		wantParent string
		wantFwd    bool
	}{
		{
			name:       "reply via link.messageId",
			payload:    `{"messages":[{"id":2,"time":1,"type":"REPLY","link":{"type":"REPLY","messageId":"1"}}]}`,
			wantParent: "1",
			wantFwd:    false,
		},
		{
			name:       "reply via prevMessageId",
			payload:    `{"messages":[{"id":2,"time":1,"type":"TEXT","prevMessageId":1}]}`,
			wantParent: "1",
			wantFwd:    false,
		},
		{
			name:       "forward sets flag, not parent",
			payload:    `{"messages":[{"id":2,"time":1,"type":"FORWARD","link":{"type":"FORWARD","messageId":"1","chatId":9}}]}`,
			wantParent: "",
			wantFwd:    true,
		},
		{
			name:       "explicit forwarded flag",
			payload:    `{"messages":[{"id":2,"time":1,"type":"TEXT","forwarded":true}]}`,
			wantParent: "",
			wantFwd:    true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseHistory([]byte(tc.payload))
			if err != nil {
				t.Fatalf("ParseHistory error: %v", err)
			}
			if len(got) != 1 {
				t.Fatalf("want 1 post, got %d", len(got))
			}
			if got[0].ParentID != tc.wantParent {
				t.Errorf("ParentID = %q, want %q", got[0].ParentID, tc.wantParent)
			}
			if got[0].IsForwarded != tc.wantFwd {
				t.Errorf("IsForwarded = %v, want %v", got[0].IsForwarded, tc.wantFwd)
			}
		})
	}
}

// TestParseHistory_AcceptedShapes proves the mapper is agnostic to whether it is
// handed the full WebSocket frame, the bare payload, or a bare message array.
func TestParseHistory_AcceptedShapes(t *testing.T) {
	frame := `{"opcode":49,"payload":{"chatId":7,"messages":[{"id":1,"chatId":7,"sender":5,"text":"hi","time":1000000000000}]}}`
	payload := `{"chatId":7,"messages":[{"id":1,"chatId":7,"sender":5,"text":"hi","time":1000000000000}]}`
	array := `[{"id":1,"chatId":7,"sender":5,"text":"hi","time":1000000000000}]`

	want := Post{
		ID:            "1",
		ThreadID:      "7",
		AuthorID:      "5",
		Text:          "hi",
		TimestampUnix: 1000000000, // 1e12 ms -> 1e9 s
	}

	for name, in := range map[string]string{"frame": frame, "payload": payload, "array": array} {
		t.Run(name, func(t *testing.T) {
			got, err := ParseHistory([]byte(in))
			if err != nil {
				t.Fatalf("ParseHistory error: %v", err)
			}
			if len(got) != 1 {
				t.Fatalf("want 1 post, got %d", len(got))
			}
			if !reflect.DeepEqual(got[0], want) {
				t.Errorf("post = %#v, want %#v", got[0], want)
			}
		})
	}
}

// TestParseHistory_MissingFields checks that sparse messages (only id+time) map
// to zero-valued Post fields instead of panicking.
func TestParseHistory_MissingFields(t *testing.T) {
	got, err := ParseHistory([]byte(`{"messages":[{"id":42,"time":0}]}`))
	if err != nil {
		t.Fatalf("ParseHistory error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("want 1 post, got %d", len(got))
	}
	p := got[0]
	if p.ID != "42" {
		t.Errorf("ID = %q, want 42", p.ID)
	}
	if p.AuthorID != "" || p.Text != "" || p.ParentID != "" || p.IsForwarded {
		t.Errorf("expected zero-valued optional fields, got %#v", p)
	}
	if p.TimestampUnix != 0 {
		t.Errorf("TimestampUnix = %d, want 0", p.TimestampUnix)
	}
	if len(p.Attachments) != 0 {
		t.Errorf("expected no attachments, got %d", len(p.Attachments))
	}
}

// TestParseHistory_EmptyHistory verifies an empty message list yields an empty,
// non-nil-error result (a chat with no messages is not an error).
func TestParseHistory_EmptyHistory(t *testing.T) {
	for name, in := range map[string]string{
		"empty payload messages": `{"payload":{"messages":[]}}`,
		"empty bare array":       `[]`,
	} {
		t.Run(name, func(t *testing.T) {
			got, err := ParseHistory([]byte(in))
			if err != nil {
				t.Fatalf("ParseHistory error: %v", err)
			}
			if len(got) != 0 {
				t.Fatalf("want 0 posts, got %d", len(got))
			}
		})
	}
}

// TestParseHistory_Attachments exercises the attachment discriminator and the
// MIME inference rules (image wildcard, extension-derived, octet-stream, empty).
func TestParseHistory_Attachments(t *testing.T) {
	payload := `{"messages":[{"id":1,"time":1,"attaches":[
		{"_type":"PHOTO","photoId":100},
		{"_type":"VIDEO","videoId":200},
		{"_type":"FILE","fileId":300,"name":"report.pdf"},
		{"_type":"FILE","fileId":301,"name":"blob.bin"},
		{"_type":"STICKER","id":400}
	]}]}`

	got, err := ParseHistory([]byte(payload))
	if err != nil {
		t.Fatalf("ParseHistory error: %v", err)
	}
	want := []Attachment{
		{ID: "100", MIME: "image/*"},
		{ID: "200", MIME: "video/*"},
		{ID: "300", MIME: "application/pdf", FileName: "report.pdf"},
		{ID: "301", MIME: "application/octet-stream", FileName: "blob.bin"},
		{ID: "400", MIME: ""},
	}
	if !reflect.DeepEqual(got[0].Attachments, want) {
		t.Fatalf("attachments mismatch\n got: %#v\nwant: %#v", got[0].Attachments, want)
	}
}

// TestParseHistory_LargeIDPrecision guards against float64 precision loss: a
// 19-digit numeric id must round-trip to its exact decimal digits.
func TestParseHistory_LargeIDPrecision(t *testing.T) {
	got, err := ParseHistory([]byte(`{"messages":[{"id":9223372036854775807,"time":1,"sender":1234567890123456789}]}`))
	if err != nil {
		t.Fatalf("ParseHistory error: %v", err)
	}
	if got[0].ID != "9223372036854775807" {
		t.Errorf("ID = %q, want 9223372036854775807", got[0].ID)
	}
	if got[0].AuthorID != "1234567890123456789" {
		t.Errorf("AuthorID = %q, want 1234567890123456789", got[0].AuthorID)
	}
}

// TestParseHistory_InvalidJSON confirms malformed input is a real error, not a
// silent empty result.
func TestParseHistory_InvalidJSON(t *testing.T) {
	if _, err := ParseHistory([]byte(`{not json`)); err == nil {
		t.Fatal("expected error for malformed JSON, got nil")
	}
	if _, err := ParseHistory(nil); err == nil {
		t.Fatal("expected error for empty input, got nil")
	}
}
