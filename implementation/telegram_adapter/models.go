// Package telegramadapter is a Go adapter for reading Telegram channel/group
// thread history into Helix Thready's ThreadReader ingestion seam.
//
// Scope split (deliberate, and honest — see EVIDENCE.md):
//
//   - OFFLINE, REAL, TESTED: MapMessages maps a slice of intermediate TGMessage
//     values (each mirroring the fields Herald's vendored gotd/td tg.Message
//     exposes) into []Post. It resolves reply-to → ParentID, the forwarded flag
//     from a forward header, media → attachments, preserves hashtag-bearing
//     text verbatim, and derives forum-topic / getReplies thread grouping into
//     ThreadID. This is pure, stdlib-only, and covered by go test. See map.go.
//   - LIVE, [BUILD-NEW] STUB: the MTProto user-client Connect / Authenticate /
//     FetchThreadHistory calls return ErrNotImplemented. This environment has no
//     api_id/api_hash/session and no Telegram account; nothing about the on-wire
//     MTProto session is faked. See client.go.
//
// Why an intermediate TGMessage instead of importing gotd/td: keeping this
// module stdlib-only and dependency-free makes the mapper offline-testable and
// keeps the "no bluff" guarantee mechanical — there is no live client to
// accidentally exercise. The live promotion (§3 of
// docs/public/research/mvp/architecture/messenger-ingestion.md, [GAP: 5.1.1])
// wires Herald's real gotd/td tg.Message into this same TGMessage shape and
// calls MapMessages unchanged.
package telegramadapter

import (
	"context"
	"errors"
)

// ErrNotImplemented is returned by every live-network method of the Telegram
// MTProto reader. The live gotd/td user client is [BUILD-NEW] here: it requires
// api_id/api_hash, an interactive phone+code(+2FA) login and a stored session
// this environment cannot provide, so these calls fail loudly rather than
// pretend to have reached Telegram.
var ErrNotImplemented = errors.New("telegramadapter: live MTProto (gotd/td) call not implemented ([BUILD-NEW]; needs api_id/api_hash + session + Herald promotion of qaherald/internal/mtproto)")

// Attachment is a file/media reference carried by a Post. It mirrors
// threadreader.Attachment field-for-field so a trivial bridge can convert
// between the two without reflection. Content-addressed download/dedup is the
// Asset Service's job (herald DownloadAttachment), not this adapter's — we keep
// the messenger-native ids.
//
// Telegram's MTProto media does not travel with a content hash, so SHA256 is
// always "" here. MIME is verbatim from a document's mime_type when present;
// otherwise it is inferred (photos are JPEG; unknown documents fall back to the
// filename extension or application/octet-stream). See inferMIME in map.go.
type Attachment struct {
	ID       string // messenger-native document/photo id
	MIME     string // document mime_type verbatim, or inferred (see note above)
	FileName string // document filename attribute, when present
	SHA256   string // always "" for MTProto; the source provides no content hash
}

// Post is a single message in a thread, normalized across messengers. It mirrors
// threadreader.Post exactly (ID, ThreadID, ParentID, AuthorID, Text,
// TimestampUnix, IsForwarded, Attachments) so this adapter can feed the
// messenger-agnostic ThreadReader assembler.
//
// ParentID is the id of the message this post replies to (from the tg.Message
// reply header's reply_to_msg_id); an empty ParentID marks a message with no
// reply-to, i.e. a root. ThreadID groups every post of one thread: for a forum
// topic or a discussion (getReplies) thread it is the channel key plus the topic
// root id; for a plain channel message it is just the channel key (see
// resolveThreadID). TimestampUnix is Unix SECONDS (tg.Message.Date is already
// Unix seconds).
type Post struct {
	ID            string
	ThreadID      string
	ParentID      string
	AuthorID      string
	Text          string
	TimestampUnix int64
	IsForwarded   bool
	Attachments   []Attachment
}

// Credentials carries the inputs the gotd/td user-client login needs:
//
//   - AppID / AppHash: the api_id / api_hash issued at https://my.telegram.org.
//   - Phone: E.164 number for the interactive login.
//   - Code: the login code Telegram sends (auth.sendCode → auth.signIn).
//   - Password: optional 2FA (SRP) password when the account has one set.
//
// In this skeleton the values are retained by Authenticate but no auth traffic
// is sent ([BUILD-NEW]).
type Credentials struct {
	AppID    int
	AppHash  string
	Phone    string
	Code     string
	Password string
}

// ChannelRef identifies a Telegram channel/supergroup to read. MTProto needs the
// access_hash (not just the id) to construct an InputChannel, so it is carried
// here alongside the numeric id and the optional @username.
type ChannelRef struct {
	ChannelID  int64
	AccessHash int64
	Username   string
}

// MessageSource is the ingestion seam over a concrete gotd/td MTProto user
// session. It is what the promoted TelegramThreadReader implements; in this
// skeleton every method is an honest [BUILD-NEW] stub returning
// ErrNotImplemented. The real, tested work — turning tg.Message values into
// []Post — lives in the standalone MapMessages function, which needs no client
// at all.
//
// TelegramThreadReader additionally exposes FetchThread(ctx, threadID) so it
// also satisfies the threadreader.MessageSource shape; wiring it into the
// assembler is then a one-liner once a live transport exists.
type MessageSource interface {
	// Connect dials Telegram and brings up the MTProto transport (gotd/td
	// telegram.Client.Run). [BUILD-NEW]
	Connect(ctx context.Context) error

	// Authenticate performs the user login: auth.sendCode → auth.signIn(code)
	// and, if required, the 2FA SRP check, then persists the session. [BUILD-NEW]
	Authenticate(ctx context.Context, creds Credentials) error

	// FetchThreadHistory reads a complete thread (root + replies) for rootID in
	// ch via messages.getReplies (falling back to messages.getHistory) and maps
	// the tg.Message values through MapMessages into []Post. [BUILD-NEW] network;
	// the mapping half is real (MapMessages).
	FetchThreadHistory(ctx context.Context, ch ChannelRef, rootID int) ([]Post, error)
}
