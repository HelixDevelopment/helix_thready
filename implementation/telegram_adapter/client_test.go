package telegramadapter

import (
	"context"
	"errors"
	"testing"
)

// TestTelegramThreadReader_LiveCallsAreHonestStubs asserts that every live
// MTProto method returns ErrNotImplemented. This is the "no bluff" guarantee:
// the reader never pretends to have reached Telegram.
func TestTelegramThreadReader_LiveCallsAreHonestStubs(t *testing.T) {
	r := NewTelegramThreadReader(12345, "deadbeefdeadbeefdeadbeefdeadbeef")
	ctx := context.Background()

	if err := r.Connect(ctx); !errors.Is(err, ErrNotImplemented) {
		t.Errorf("Connect err = %v, want ErrNotImplemented", err)
	}
	if err := r.Authenticate(ctx, Credentials{Phone: "+10000000000", Code: "00000"}); !errors.Is(err, ErrNotImplemented) {
		t.Errorf("Authenticate err = %v, want ErrNotImplemented", err)
	}
	if _, err := r.FetchThreadHistory(ctx, ChannelRef{ChannelID: 1001, AccessHash: 7}, 100); !errors.Is(err, ErrNotImplemented) {
		t.Errorf("FetchThreadHistory err = %v, want ErrNotImplemented", err)
	}
	if _, err := r.FetchThread(ctx, "channel:1001/100"); !errors.Is(err, ErrNotImplemented) {
		t.Errorf("FetchThread err = %v, want ErrNotImplemented", err)
	}
}

// TestParseThreadID checks the pure ThreadID → (ChannelRef, topic) split used by
// FetchThread, so the offline half of the seam is verified even while the network
// half is a stub.
func TestParseThreadID(t *testing.T) {
	cases := []struct {
		in        string
		wantID    int64
		wantTopic int
	}{
		{"channel:1001", 1001, 0},
		{"channel:1001/100", 1001, 100},
		{"chat:42/7", 42, 7},
		{"user:99", 99, 0},
		{"channel:1234567890123456789/55", 1234567890123456789, 55},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			ref, topic := parseThreadID(tc.in)
			if ref.ChannelID != tc.wantID {
				t.Errorf("ChannelID = %d, want %d", ref.ChannelID, tc.wantID)
			}
			if topic != tc.wantTopic {
				t.Errorf("topic = %d, want %d", topic, tc.wantTopic)
			}
		})
	}
}
