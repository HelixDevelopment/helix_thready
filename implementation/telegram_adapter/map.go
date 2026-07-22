package telegramadapter

import (
	"fmt"
	"mime"
	"path/filepath"
	"strconv"
)

// MapMessages maps a slice of intermediate TGMessage values into []Post.
//
// This is the REAL, offline-testable core of the adapter — the exact mapping
// the promoted gotd/td MTProto reader (§3, [GAP: 5.1.1]) applies to the
// tg.Message values it reads via messages.getReplies / messages.getHistory.
//
// Per-message mapping:
//
//	ID            ← TGMessage.ID                       (as decimal string)
//	ThreadID      ← channel key (+ topic root, for forum/discussion threads)
//	ParentID      ← ReplyTo.ReplyToMsgID               ("" when no reply-to → root)
//	AuthorID      ← FromID, else Peer                  (channel posts author = channel)
//	Text          ← TGMessage.Message                  (verbatim; hashtags preserved)
//	TimestampUnix ← TGMessage.Date                     (already Unix seconds)
//	IsForwarded   ← FwdFrom != nil                     (presence of a forward header)
//	Attachments   ← Media                              (0 or 1, MIME verbatim/inferred)
//
// The only failure is a structurally invalid message: a non-positive ID cannot
// occur on a real tg.Message (MTProto ids are >= 1) and signals a malformed or
// empty (tg.MessageEmpty) input, so it is a hard error rather than a silently
// mapped zero post. Every optional field degrades gracefully to its zero value.
// Empty input yields an empty, non-nil slice with a nil error.
func MapMessages(msgs []TGMessage) ([]Post, error) {
	posts := make([]Post, 0, len(msgs))
	for i, m := range msgs {
		if m.ID <= 0 {
			return nil, fmt.Errorf("telegramadapter: message at index %d has non-positive id %d (malformed or empty tg.Message)", i, m.ID)
		}
		posts = append(posts, mapMessage(m))
	}
	return posts, nil
}

// mapMessage converts one intermediate TGMessage into a normalized Post.
func mapMessage(m TGMessage) Post {
	// Parent edge: the immediate message replied to. This is the SAME field for
	// a reply-to-a-root and a reply-to-a-reply, so nested chains linearize
	// correctly; absence marks a root.
	parentID := ""
	if m.ReplyTo != nil && m.ReplyTo.ReplyToMsgID != 0 {
		parentID = strconv.Itoa(m.ReplyTo.ReplyToMsgID)
	}

	// Author: the sender if the message has one; otherwise the peer itself, i.e.
	// a channel broadcast is authored by the channel. This is the identity the
	// herald self-filter (IsSelfEcho / BotSelfIdentity) compares against.
	authorPeer := m.Peer
	if m.FromID != nil {
		authorPeer = *m.FromID
	}

	post := Post{
		ID:            strconv.Itoa(m.ID),
		ThreadID:      resolveThreadID(m),
		ParentID:      parentID,
		AuthorID:      authorPeer.String(),
		Text:          m.Message,
		TimestampUnix: int64(m.Date),
		IsForwarded:   m.FwdFrom != nil,
	}
	if m.Media != nil {
		post.Attachments = append(post.Attachments, mapMedia(*m.Media))
	}
	return post
}

// resolveThreadID computes the thread-grouping key. For a forum topic or a
// discussion (getReplies) thread the messages share a "top" message id
// (reply_to_top_id); a direct reply to a forum-topic root instead carries the
// topic root as reply_to_msg_id with the forum_topic flag set. In both cases the
// thread key is the channel key plus that topic root, so every message of one
// topic groups together and distinct topics in the same channel stay separate.
// A plain channel message (no thread context) groups under the bare channel key.
func resolveThreadID(m TGMessage) string {
	base := m.Peer.String()

	topic := 0
	if m.ReplyTo != nil {
		switch {
		case m.ReplyTo.ReplyToTopID != 0:
			topic = m.ReplyTo.ReplyToTopID
		case m.ReplyTo.ForumTopic && m.ReplyTo.ReplyToMsgID != 0:
			topic = m.ReplyTo.ReplyToMsgID
		}
	}
	if topic == 0 {
		return base
	}
	if base == "" {
		return strconv.Itoa(topic)
	}
	return base + "/" + strconv.Itoa(topic)
}

// mapMedia converts one intermediate TGMedia into a normalized Attachment.
func mapMedia(md TGMedia) Attachment {
	return Attachment{
		ID:       strconv.FormatInt(md.ID, 10),
		MIME:     inferMIME(md),
		FileName: md.FileName,
		SHA256:   "", // MTProto media carries no content hash
	}
}

// inferMIME resolves the media type. A document's mime_type is authoritative and
// used verbatim; when absent it is derived from the filename extension, else
// application/octet-stream. Telegram photos always arrive as JPEG, so a photo
// maps to image/jpeg.
func inferMIME(md TGMedia) string {
	switch md.Kind {
	case MediaPhoto:
		return "image/jpeg"
	case MediaDocument:
		if md.MIME != "" {
			return md.MIME
		}
		if ext := filepath.Ext(md.FileName); ext != "" {
			if t := mime.TypeByExtension(ext); t != "" {
				return t
			}
		}
		return "application/octet-stream"
	default:
		return ""
	}
}

// GroupByThread buckets posts by their ThreadID, preserving input order within
// each bucket. It makes the forum-topic / getReplies grouping computed by
// resolveThreadID concrete: feed a channel's mixed messages in and each forum
// topic / discussion thread comes back as its own slice.
func GroupByThread(posts []Post) map[string][]Post {
	out := make(map[string][]Post)
	for _, p := range posts {
		out[p.ThreadID] = append(out[p.ThreadID], p)
	}
	return out
}
