// Package threadreader is the messenger-agnostic ThreadReader abstraction for
// Helix Thready ([GAP: 5.1] region; Herald extension).
//
// Its single responsibility is to assemble a COMPLETE post — the root post plus
// the full chain of ORGANIC replies (real humans), excluding the system's own
// processing/status replies — while preserving order and extracting hashtags.
//
// See docs/public/research/mvp/architecture/messenger-ingestion.md (§5, §6) and
// processing-pipeline.md (§4) for the contract this package implements.
package threadreader

// Attachment is a file/media reference carried by a post. The ThreadReader keeps
// attachments verbatim; content-addressed download/dedup is the Asset Service's
// concern (herald DownloadAttachment), not this abstraction's.
type Attachment struct {
	ID       string // messenger-native attachment/file id
	MIME     string // e.g. "video/mp4", "image/png"
	FileName string // original file name, if any
	SHA256   string // content hash, if the source already computed one
}

// Post is a single message in a thread, normalized across messengers.
//
// ParentID is the id of the message this post replies to; an empty ParentID (or
// ParentID == ID) marks a root post. ThreadID groups every post of one thread.
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

// Thread is an assembled complete post: the root followed by its organic replies
// in chronological order. System/bot replies have already been filtered out.
type Thread struct {
	Root    Post
	Replies []Post
}

// Hashtags returns the union of all "#Tag" tokens found across the root and every
// organic reply, in first-seen order (root first, then replies in order), deduped
// with case preserved.
//
// This is the "hashtags added as a reply" case from the spec: a link-only root
// still classifies because the tags live on a reply, and this method unions the
// whole chain.
func (t *Thread) Hashtags() []string {
	seen := make(map[string]struct{})
	var out []string
	add := func(text string) {
		for _, tag := range ExtractHashtags(text) {
			if _, ok := seen[tag]; ok {
				continue
			}
			seen[tag] = struct{}{}
			out = append(out, tag)
		}
	}
	add(t.Root.Text)
	for _, r := range t.Replies {
		add(r.Text)
	}
	return out
}
