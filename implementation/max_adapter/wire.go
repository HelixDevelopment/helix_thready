package maxadapter

import (
	"bytes"
	"encoding/json"
	"strconv"
)

// wireRoot models shapes 1 & 2 accepted by ParseHistory: an object that either
// carries "messages" directly (bare payload) or nests it under "payload" (full
// WebSocket frame). Whichever is populated wins in extractMessages.
type wireRoot struct {
	Messages []wireMessage `json:"messages"`
	ChatID   flexID        `json:"chatId"`
	Payload  *struct {
		Messages []wireMessage `json:"messages"`
		ChatID   flexID        `json:"chatId"`
	} `json:"payload"`
}

// wireMessage is one OneMe message as it appears in an opcode-49 history
// response. Field names are the camelCase wire aliases used by PyMax's Message
// model (see PROTOCOL.md for provenance). Unknown fields are ignored.
type wireMessage struct {
	ID            flexID       `json:"id"`
	ChatID        flexID       `json:"chatId"`
	Sender        flexID       `json:"sender"`
	Text          string       `json:"text"`
	Time          int64        `json:"time"`
	Type          string       `json:"type"`
	Link          *wireLink    `json:"link"`
	PrevMessageID flexID       `json:"prevMessageId"`
	Attaches      []wireAttach `json:"attaches"`
	Forwarded     *bool        `json:"forwarded"` // defensive: some captures expose an explicit flag
}

// wireLink is the reply/forward edge a message carries. On send, both vkmax and
// PyMax construct message.link as {type:"REPLY",messageId} or
// {type:"FORWARD",messageId,chatId}; received messages echo the same shape.
type wireLink struct {
	Type      string `json:"type"` // "REPLY" | "FORWARD"
	MessageID flexID `json:"messageId"`
	ChatID    flexID `json:"chatId"`
}

// wireAttach is one attachment, discriminated by its `_type` field. Only the
// id/name fields the adapter maps are modelled; tokens, urls and dimensions are
// intentionally ignored.
type wireAttach struct {
	Type    string `json:"_type"` // PHOTO | VIDEO | FILE | AUDIO | STICKER | ...
	ID      flexID `json:"id"`
	FileID  flexID `json:"fileId"`
	PhotoID flexID `json:"photoId"`
	VideoID flexID `json:"videoId"`
	AudioID flexID `json:"audioId"`
	Name    string `json:"name"`
}

// flexID is a JSON value that OneMe expresses inconsistently as either a number
// or a string (message ids, sender ids and chat ids all vary by opcode). It
// decodes both without the float64 precision loss that a plain interface{}
// would suffer on large 64-bit ids, and renders back to its exact digits.
type flexID string

func (f flexID) String() string { return string(f) }

func (f *flexID) UnmarshalJSON(b []byte) error {
	b = bytes.TrimSpace(b)
	if len(b) == 0 || bytes.Equal(b, []byte("null")) {
		*f = ""
		return nil
	}
	// String form: "12345" -> 12345 (strip the quotes, unescape).
	if b[0] == '"' {
		var s string
		if err := json.Unmarshal(b, &s); err != nil {
			return err
		}
		*f = flexID(s)
		return nil
	}
	// Number form: decode via json.Number so large integers keep every digit.
	var n json.Number
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber()
	if err := dec.Decode(&n); err != nil {
		return err
	}
	// Normalize a float-looking integer (e.g. 1.75e12) to plain digits.
	if i, err := n.Int64(); err == nil {
		*f = flexID(strconv.FormatInt(i, 10))
		return nil
	}
	*f = flexID(n.String())
	return nil
}
