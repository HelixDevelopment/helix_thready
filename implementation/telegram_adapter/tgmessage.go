package telegramadapter

import "strconv"

// TGPeerKind mirrors the three concrete tg.Peer* implementations of gotd/td's
// tg.PeerClass. A zero value (PeerNone) is an absent peer.
type TGPeerKind uint8

const (
	// PeerNone is an absent/zero peer.
	PeerNone TGPeerKind = iota
	// PeerUser mirrors tg.PeerUser{UserID}.
	PeerUser
	// PeerChat mirrors tg.PeerChat{ChatID} (a small basic group).
	PeerChat
	// PeerChannel mirrors tg.PeerChannel{ChannelID} (a channel/supergroup).
	PeerChannel
)

// TGPeer is the intermediate form of gotd/td's tg.PeerClass. gotd models a peer
// as an interface with three concrete types; we flatten it to a kind + int64 id
// (all of tg.PeerUser.UserID / tg.PeerChat.ChatID / tg.PeerChannel.ChannelID are
// int64). String renders a collision-free, kind-qualified id so a user 42 and a
// channel 42 never alias in AuthorID / ThreadID.
type TGPeer struct {
	Kind TGPeerKind
	ID   int64
}

// String returns a stable, kind-qualified identifier such as "channel:1001" or
// "user:42". An absent peer renders as "".
func (p TGPeer) String() string {
	if p.Kind == PeerNone {
		return ""
	}
	switch p.Kind {
	case PeerUser:
		return "user:" + strconv.FormatInt(p.ID, 10)
	case PeerChat:
		return "chat:" + strconv.FormatInt(p.ID, 10)
	case PeerChannel:
		return "channel:" + strconv.FormatInt(p.ID, 10)
	default:
		return strconv.FormatInt(p.ID, 10)
	}
}

// TGReplyTo is the intermediate form of gotd/td's tg.MessageReplyHeader (the
// tg.Message.ReplyTo field). It carries exactly the fields thread assembly
// needs:
//
//   - ReplyToMsgID: the message this one replies to → the immediate parent edge.
//   - ReplyToTopID: the "top" message of the discussion/forum thread this reply
//     belongs to (tg's reply_to_top_id) → the thread-grouping key. 0 when unset.
//   - ForumTopic: tg's forum_topic flag — set on replies inside a forum topic;
//     a direct reply to a topic root carries ReplyToMsgID == topicRoot with this
//     flag set and ReplyToTopID possibly 0.
type TGReplyTo struct {
	ReplyToMsgID int
	ReplyToTopID int
	ForumTopic   bool
}

// TGFwdFrom is the intermediate form of gotd/td's tg.MessageFwdHeader (the
// tg.Message.FwdFrom field). Its mere PRESENCE on a message is what marks the
// message as forwarded; the inner fields are documentary provenance only (this
// adapter maps presence → Post.IsForwarded and does not model the original
// author beyond keeping the shape faithful).
type TGFwdFrom struct {
	FromID   *TGPeer // original sender, when not hidden
	FromName string  // original sender name, for hidden-account forwards
	Date     int     // original message date (Unix seconds)
}

// TGMediaKind mirrors the tg.MessageMedia* variant of gotd/td's
// tg.MessageMediaClass that this adapter maps to an attachment.
type TGMediaKind uint8

const (
	// MediaNone is no media.
	MediaNone TGMediaKind = iota
	// MediaPhoto mirrors tg.MessageMediaPhoto (a tg.Photo).
	MediaPhoto
	// MediaDocument mirrors tg.MessageMediaDocument (a tg.Document: files,
	// video, audio, stickers, voice — everything that is not a bare photo).
	MediaDocument
)

// TGMedia is the intermediate form of the tg.Message.Media field. A real
// tg.Message carries at most one MessageMediaClass, so this is a single value,
// not a slice (albums are separate messages sharing a grouped_id). Only the
// fields the adapter maps are modelled:
//
//   - ID:       the document/photo id (tg.Document.ID / tg.Photo.ID, int64).
//   - MIME:     tg.Document.MimeType verbatim; empty for photos.
//   - FileName: the tg.DocumentAttributeFilename value, when present.
type TGMedia struct {
	Kind     TGMediaKind
	ID       int64
	MIME     string
	FileName string
}

// TGMessage is the intermediate form of gotd/td's tg.Message, carrying exactly
// the fields the ThreadReader mapping consumes. Field-by-field provenance
// against tg.Message:
//
//	ID       ← tg.Message.ID       (int; MTProto message id, positive)
//	Peer     ← tg.Message.PeerID    (the channel/chat/user the message lives in)
//	FromID   ← tg.Message.FromID    (sender; nil for a channel broadcast, whose
//	                                 author is Peer itself)
//	Date     ← tg.Message.Date      (int; Unix seconds)
//	Message  ← tg.Message.Message   (the text; hashtags live here, kept verbatim)
//	ReplyTo  ← tg.Message.ReplyTo   (reply header; presence + reply_to_msg_id /
//	                                 reply_to_top_id / forum_topic)
//	FwdFrom  ← tg.Message.FwdFrom   (forward header; presence ⇒ forwarded)
//	Media    ← tg.Message.Media     (at most one; → 0 or 1 attachment)
//	Out      ← tg.Message.Out       (outgoing/self flag; informative, not mapped)
//
// A nil pointer field means the corresponding tg.Message optional was absent.
type TGMessage struct {
	ID      int
	Peer    TGPeer
	FromID  *TGPeer
	Date    int
	Message string
	ReplyTo *TGReplyTo
	FwdFrom *TGFwdFrom
	Media   *TGMedia
	Out     bool
}
