package skilldispatch

import "strings"

// Post is the unit of work handed to the dispatch engine. It is the minimal
// projection of a claimed thread that the engine needs to classify and route:
// a stable identity plus the signals Skills match on (hashtags, content type,
// text, links). It is intentionally a plain value — callers assemble it from the
// ingestion layer (the union of root + reply hashtags, etc.).
type Post struct {
	// ID is the stable, unique identity of the post. It is the claim key: the
	// exactly-once guarantee is keyed on this value, so it MUST be stable across
	// duplicate "new post" triggers for the same underlying post.
	ID string

	// Hashtags is the union of hashtags across the assembled thread. Matching is
	// case-insensitive and tolerant of a leading '#'.
	Hashtags []string

	// ContentType is an optional coarse classification (e.g. "video", "torrent").
	ContentType string

	// Text is the assembled post/thread text (used by content-derived Skills).
	Text string

	// Links are the URLs found in the thread (used by indirect determination).
	Links []string
}

// HasHashtag reports whether the post carries the given hashtag, case-insensitively
// and ignoring a leading '#' on either side. It is the primary building block for
// hashtag-keyed Skill matching.
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
