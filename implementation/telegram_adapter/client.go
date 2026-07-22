package telegramadapter

import (
	"context"
	"strconv"
	"strings"
)

// TelegramThreadReader is the [BUILD-NEW] live reader for Telegram channel/group
// history over the gotd/td MTProto USER client. It is a compile-time-complete
// implementation of MessageSource whose network methods are honest stubs: each
// returns ErrNotImplemented because this environment has no api_id/api_hash, no
// login session and no Telegram account, and the real client lives in Herald's
// qaherald/internal/mtproto awaiting promotion (§3, [GAP: 5.1.1]).
//
// The real, tested capability of this package is MapMessages, which the intended
// FetchThreadHistory body below is wired to call once a live client exists.
//
// TelegramThreadReader also exposes FetchThread(ctx, threadID) so it satisfies
// the threadreader.MessageSource shape (FetchThread(ctx, string) ([]Post,
// error)); the assembler consumes exactly that.
type TelegramThreadReader struct {
	appID   int
	appHash string
	creds   Credentials
	// client *telegram.Client // gotd/td — nil here; wired on promotion.
}

// NewTelegramThreadReader constructs a reader with the api_id / api_hash the
// gotd/td client will need. No socket is opened and no session is loaded here.
func NewTelegramThreadReader(appID int, appHash string) *TelegramThreadReader {
	return &TelegramThreadReader{appID: appID, appHash: appHash}
}

// Connect would bring up the MTProto transport (gotd/td telegram.Client.Run and
// the DC handshake).
//
// [BUILD-NEW]: not implemented. Dialing Telegram needs api_id/api_hash plus a
// reachable DC and would open a real session this environment cannot provide, so
// it fails loudly rather than fake a connection.
func (r *TelegramThreadReader) Connect(ctx context.Context) error {
	return ErrNotImplemented
}

// Authenticate would run the user login: auth.sendCode → auth.signIn(code) and,
// when the account has 2FA, the SRP password check, then persist the session via
// security/pkg/securestorage.
//
// [BUILD-NEW]: not implemented. The credentials are retained so a future real
// implementation has them, but no auth traffic is sent.
func (r *TelegramThreadReader) Authenticate(ctx context.Context, creds Credentials) error {
	r.creds = creds
	if creds.AppID != 0 {
		r.appID = creds.AppID
	}
	if creds.AppHash != "" {
		r.appHash = creds.AppHash
	}
	return ErrNotImplemented
}

// FetchThreadHistory would read the complete thread (root + replies) for rootID
// in ch and map the tg.Message values through MapMessages.
//
// [BUILD-NEW]: the NETWORK half is not implemented and returns ErrNotImplemented.
// The MAPPING half it would call — MapMessages — is real and independently
// tested; see map_test.go. Intended shape once a live client exists:
//
//	input := &tg.InputChannel{ChannelID: ch.ChannelID, AccessHash: ch.AccessHash}
//	// Reply threads / forum topics: messages.getReplies rooted at rootID.
//	resp, err := api.MessagesGetReplies(ctx, &tg.MessagesGetRepliesRequest{
//	    Peer: input, MsgID: rootID, Limit: 100,
//	})
//	// (fall back to messages.getHistory for a plain channel with no thread)
//	if err != nil { return nil, err }
//	msgs := collectTGMessages(resp) // tg.Message → TGMessage (field-for-field)
//	return MapMessages(msgs)
func (r *TelegramThreadReader) FetchThreadHistory(ctx context.Context, ch ChannelRef, rootID int) ([]Post, error) {
	return nil, ErrNotImplemented
}

// FetchThread adapts FetchThreadHistory to the threadreader.MessageSource
// signature (FetchThread(ctx, threadID) ([]Post, error)). threadID is the
// ThreadID this adapter emits: a channel key ("channel:<id>") optionally
// suffixed with the topic root ("channel:<id>/<rootID>"). This is the exact seam
// the assembler consumes.
//
// [BUILD-NEW]: delegates to FetchThreadHistory, which is not implemented.
func (r *TelegramThreadReader) FetchThread(ctx context.Context, threadID string) ([]Post, error) {
	ch, rootID := parseThreadID(threadID)
	return r.FetchThreadHistory(ctx, ch, rootID)
}

// parseThreadID splits a ThreadID emitted by this adapter back into a ChannelRef
// (id only; the access_hash must be resolved separately) and an optional topic
// root id. It is the inverse of resolveThreadID for the id portion and is pure,
// so it is exercised offline even while the network half is a stub.
func parseThreadID(threadID string) (ChannelRef, int) {
	base, topic := threadID, 0
	if i := strings.LastIndex(threadID, "/"); i >= 0 {
		base = threadID[:i]
		if n, err := strconv.Atoi(threadID[i+1:]); err == nil {
			topic = n
		}
	}
	// Strip the kind prefix ("channel:" / "chat:" / "user:") to recover the id.
	if i := strings.IndexByte(base, ':'); i >= 0 {
		base = base[i+1:]
	}
	ref := ChannelRef{}
	if id, err := strconv.ParseInt(base, 10, 64); err == nil {
		ref.ChannelID = id
	}
	return ref, topic
}

// compile-time assertion: TelegramThreadReader implements MessageSource.
var _ MessageSource = (*TelegramThreadReader)(nil)
