package maxadapter

import (
	"bytes"
	"encoding/json"
	"fmt"
	"mime"
	"path/filepath"
	"strings"
)

// ParseHistory maps a OneMe internal-API chat-history payload into []Post.
//
// This is the REAL, offline-testable core of the adapter. It accepts any of the
// three shapes a caller might hand it, so it works whether you pass the whole
// WebSocket frame or just its body:
//
//  1. Full frame:      {"ver":11,"cmd":1,"opcode":49,"payload":{"messages":[...]}}
//  2. Bare payload:    {"messages":[...],"chatId":123}
//  3. Bare array:      [ {message}, {message}, ... ]
//
// Per-message mapping (field provenance is documented in PROTOCOL.md):
//
//   - ID            <- message.id                (number OR string; large ids kept exact)
//   - ThreadID      <- chatId (message > payload) (the OneMe chat this history is for)
//   - ParentID      <- link.messageId when link.type=="REPLY", else prevMessageId
//   - AuthorID      <- message.sender
//   - Text          <- message.text
//   - TimestampUnix <- message.time, normalized from epoch ms to Unix seconds
//   - IsForwarded   <- link.type=="FORWARD" (or a top-level "forwarded":true)
//   - Attachments   <- message.attaches[] ( _type discriminated: FILE/PHOTO/VIDEO/... )
//
// A FORWARD link references a message in a DIFFERENT chat, so it is treated as
// the forwarded-flag signal only and never populates ParentID (which is a
// within-thread reply edge). Missing/empty fields degrade gracefully to zero
// values rather than erroring; only malformed JSON returns an error.
func ParseHistory(raw []byte) ([]Post, error) {
	msgs, threadID, err := extractMessages(raw)
	if err != nil {
		return nil, err
	}

	posts := make([]Post, 0, len(msgs))
	for _, m := range msgs {
		posts = append(posts, mapMessage(m, threadID))
	}
	return posts, nil
}

// extractMessages pulls the message list (and the enclosing chatId, if any) out
// of whichever of the three accepted shapes `raw` happens to be.
func extractMessages(raw []byte) ([]wireMessage, string, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return nil, "", fmt.Errorf("maxadapter: empty history payload")
	}

	// Shape 3: a bare JSON array of messages.
	if trimmed[0] == '[' {
		var arr []wireMessage
		if err := json.Unmarshal(trimmed, &arr); err != nil {
			return nil, "", fmt.Errorf("maxadapter: decode message array: %w", err)
		}
		return arr, "", nil
	}

	// Shapes 1 & 2: an object that either IS the payload (has "messages") or
	// WRAPS it under "payload".
	var root wireRoot
	if err := json.Unmarshal(trimmed, &root); err != nil {
		return nil, "", fmt.Errorf("maxadapter: decode history payload: %w", err)
	}
	if root.Payload != nil {
		return root.Payload.Messages, root.Payload.ChatID.String(), nil
	}
	return root.Messages, root.ChatID.String(), nil
}

// mapMessage converts one on-wire OneMe message into a normalized Post.
func mapMessage(m wireMessage, enclosingChatID string) Post {
	threadID := m.ChatID.String()
	if threadID == "" {
		threadID = enclosingChatID
	}

	parentID := ""
	forwarded := false
	if m.Link != nil {
		switch strings.ToUpper(m.Link.Type) {
		case "REPLY":
			parentID = m.Link.MessageID.String()
		case "FORWARD":
			forwarded = true
		}
	}
	if parentID == "" {
		parentID = m.PrevMessageID.String()
	}
	if m.Forwarded != nil && *m.Forwarded {
		forwarded = true
	}

	post := Post{
		ID:            m.ID.String(),
		ThreadID:      threadID,
		ParentID:      parentID,
		AuthorID:      m.Sender.String(),
		Text:          m.Text,
		TimestampUnix: normalizeToUnixSeconds(m.Time),
		IsForwarded:   forwarded,
	}
	for _, a := range m.Attaches {
		post.Attachments = append(post.Attachments, mapAttachment(a))
	}
	return post
}

// mapAttachment converts one OneMe attachment (discriminated by its `_type`
// field) into a normalized Attachment. OneMe carries no MIME/hash, so MIME is
// inferred (see inferMIME) and SHA256 is left empty.
func mapAttachment(a wireAttach) Attachment {
	id := firstNonEmpty(a.FileID.String(), a.PhotoID.String(), a.VideoID.String(), a.AudioID.String(), a.ID.String())
	return Attachment{
		ID:       id,
		MIME:     inferMIME(a.Type, a.Name),
		FileName: a.Name,
		SHA256:   "", // OneMe never sends a content hash
	}
}

// inferMIME derives a best-effort media type. OneMe does NOT transmit MIME, so
// this is documented inference, not ground truth: for FILE we read the filename
// extension (RFC-real MIME when known), for media kinds we return a wildcard
// media range, and otherwise "".
func inferMIME(attachType, name string) string {
	switch strings.ToUpper(attachType) {
	case "PHOTO":
		return "image/*"
	case "VIDEO":
		return "video/*"
	case "AUDIO":
		return "audio/*"
	case "FILE":
		if ext := filepath.Ext(name); ext != "" {
			if byExt := mime.TypeByExtension(ext); byExt != "" {
				return byExt
			}
		}
		return "application/octet-stream"
	default:
		return ""
	}
}

// normalizeToUnixSeconds converts OneMe's epoch-millisecond timestamps to Unix
// seconds. Values already in seconds (< ~2001-09 in ms terms) are passed
// through, so the function is safe against either unit.
func normalizeToUnixSeconds(t int64) int64 {
	const msThreshold = 1_000_000_000_000 // 1e12: epoch ms crosses this in 2001; epoch s stays below until year ~33658
	if t >= msThreshold {
		return t / 1000
	}
	return t
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
