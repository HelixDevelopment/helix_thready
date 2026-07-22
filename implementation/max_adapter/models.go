// Package maxadapter is a Go client skeleton for the Max messenger (max.ru,
// codename "OneMe", by VK) targeting Helix Thready's ThreadReader ingestion seam.
//
// Scope split (deliberate, and honest):
//
//   - OFFLINE, REAL, TESTED: ParseHistory maps a representative OneMe internal
//     WebSocket history payload (opcode 49 response) into []Post. This is pure,
//     stdlib-only, and covered by go test. See parse.go and PROTOCOL.md.
//   - LIVE, [BUILD-NEW] STUB: the WebSocket connect / phone+token auth / fetch
//     calls return ErrNotImplemented. No live Max account exists in this
//     environment; nothing about the on-wire session is faked. See client.go.
//
// The protocol shapes ParseHistory consumes are documented — with CONFIRMED vs
// INFERRED provenance and source URLs — in PROTOCOL.md.
package maxadapter

import (
	"context"
	"errors"
)

// ErrNotImplemented is returned by every live-network method of the OneMe
// client. The live WebSocket protocol is [BUILD-NEW]: it requires a real Max
// account plus on-wire confirmation that this environment cannot provide, so
// these calls fail loudly rather than pretend to have talked to a server.
var ErrNotImplemented = errors.New("maxadapter: live OneMe WebSocket call not implemented ([BUILD-NEW]; needs a real Max account + on-wire confirmation)")

// Attachment is a file/media reference carried by a Post. It mirrors
// threadreader.Attachment field-for-field so a trivial bridge can convert
// between the two without reflection. Content-addressed download/dedup is the
// Asset Service's job, not this adapter's — we keep the messenger-native ids.
//
// OneMe does NOT transmit a MIME type or content hash on the wire, so:
//   - MIME is INFERRED from the attachment `_type` (image/*, video/*, audio/*)
//     or, for FILE, from the filename extension; it may be empty.
//   - SHA256 is always "" here (the source never computed one).
type Attachment struct {
	ID       string // messenger-native attachment id (fileId / photoId / videoId)
	MIME     string // inferred media type (see note above); may be "" or a wildcard range
	FileName string // original file name, when the attachment carries one
	SHA256   string // always "" for OneMe; the source provides no content hash
}

// Post is a single message in a thread, normalized across messengers. It mirrors
// threadreader.Post exactly (ID, ThreadID, ParentID, AuthorID, Text,
// TimestampUnix, IsForwarded, Attachments) so this adapter can feed the
// messenger-agnostic ThreadReader assembler.
//
// ParentID is the id of the message this post replies to; an empty ParentID (or
// ParentID == ID) marks a root post. ThreadID groups every post of one thread
// (the OneMe chatId). TimestampUnix is Unix SECONDS — OneMe sends epoch
// milliseconds and ParseHistory normalizes them (see parse.go).
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

// Credentials carries the two ways a OneMe user session is established:
//
//   - Phone: E.164 phone number, for the interactive SMS-code login
//     (opcode 17 START_AUTH -> opcode 18 CHECK_CODE -> token).
//   - Token: a previously obtained LOGIN token, for silent re-login (opcode 19).
//
// At least one must be set; Token is preferred when present.
type Credentials struct {
	Phone string
	Token string
}

// MaxClient is the ingestion seam over a concrete Max/OneMe session. It is the
// thing that will implement threadreader.MessageSource: FetchThreadHistory here
// has the same shape as MessageSource.FetchThread(ctx, threadID) ([]Post, error),
// and OneMeClient additionally exposes that exact method (see client.go) so the
// bridge is a one-liner.
//
// Connect / Authenticate / FetchThreadHistory are LIVE calls — in this skeleton
// they are honest [BUILD-NEW] stubs returning ErrNotImplemented. The real,
// tested work — turning an on-wire history payload into []Post — lives in the
// standalone ParseHistory function, which needs no client at all.
type MaxClient interface {
	// Connect opens the OneMe WebSocket (wss://ws-api.oneme.ru/websocket) and
	// performs the handshake (opcode 6). [BUILD-NEW]
	Connect(ctx context.Context) error

	// Authenticate establishes a user session from a phone number (SMS flow) or
	// a saved token (opcode 19). [BUILD-NEW]
	Authenticate(ctx context.Context, creds Credentials) error

	// FetchThreadHistory requests chat history (opcode 49) for threadID and maps
	// the response through ParseHistory into []Post. [BUILD-NEW] network; the
	// mapping half is real (ParseHistory).
	FetchThreadHistory(ctx context.Context, threadID string) ([]Post, error)
}
