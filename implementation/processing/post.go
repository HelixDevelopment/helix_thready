package processing

import "strings"

// Attachment is a file/media reference carried by a post. It mirrors
// threadreader.Attachment so a thin adapter can copy it field-for-field.
type Attachment struct {
	ID       string // messenger-native attachment/file id
	MIME     string // e.g. "video/mp4", "image/png"
	FileName string // original file name, if any
	SHA256   string // content hash, if the source already computed one
}

// Post is the unit of work handed to the Processor. It is the minimal projection
// of a claimed thread the orchestrator needs, and mirrors the threadreader.Post
// contract (id, threadID, hashtags, text, attachments) so a thin adapter maps an
// assembled thread onto it.
type Post struct {
	// ID is the stable, unique identity of the post. It is the claim key: the
	// exactly-once guarantee is keyed on this value, so it MUST be stable across
	// duplicate "new post" triggers for the same underlying post.
	ID string

	// ThreadID groups every post of one thread (mirrors threadreader.Post).
	ThreadID string

	// Hashtags is the union of hashtags across the assembled thread. Matching is
	// case-insensitive and tolerant of a leading '#'.
	Hashtags []string

	// Text is the assembled post/thread text (used by content-derived Skills).
	Text string

	// Attachments are the media/files carried by the thread, kept verbatim.
	Attachments []Attachment
}

// HasHashtag reports whether the post carries the given hashtag, case-insensitively
// and ignoring a leading '#' on either side.
func (p Post) HasHashtag(tag string) bool {
	want := normalizeTag(tag)
	for _, h := range p.Hashtags {
		if normalizeTag(h) == want {
			return true
		}
	}
	return false
}

// HasAnyHashtag reports whether the post carries any of the given hashtags.
func (p Post) HasAnyHashtag(tags ...string) bool {
	for _, t := range tags {
		if p.HasHashtag(t) {
			return true
		}
	}
	return false
}

func normalizeTag(t string) string {
	return strings.ToLower(strings.TrimPrefix(strings.TrimSpace(t), "#"))
}
