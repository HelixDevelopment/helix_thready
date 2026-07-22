package threadreader

import (
	"errors"
	"sort"
)

// Errors returned by Assemble.
var (
	// ErrNoPosts is returned when there are no posts at all to assemble.
	ErrNoPosts = errors.New("threadreader: no posts provided")
	// ErrMissingRoot is returned when the input contains replies but no root post
	// (every post has a parent that is not present). Deterministic, never a panic.
	ErrMissingRoot = errors.New("threadreader: no root post found in thread")
)

// Assembler builds an ordered organic Thread from raw posts.
//
// It is configured with the set of system/bot author IDs whose posts are the
// system's own processing/status replies. Those posts — and any exact duplicates
// — are filtered out; the surviving organic replies are ordered chronologically.
//
// This mirrors the herald anti-echo-loop primitive (IsSelfEcho fed by
// BotSelfIdentity / StampSender): a reply from THIS bot is dropped, a reply from a
// DIFFERENT author (human or another bot not in the set) is kept.
type Assembler struct {
	systemAuthors map[string]struct{}
}

// NewAssembler returns an Assembler that treats the given author IDs as system/bot
// authors whose replies must be excluded. Empty IDs are ignored.
func NewAssembler(systemAuthorIDs ...string) *Assembler {
	m := make(map[string]struct{}, len(systemAuthorIDs))
	for _, id := range systemAuthorIDs {
		if id == "" {
			continue
		}
		m[id] = struct{}{}
	}
	return &Assembler{systemAuthors: m}
}

// IsSystemAuthor reports whether authorID is one of the configured system/bot
// authors. An empty set never classifies anyone as system.
func (a *Assembler) IsSystemAuthor(authorID string) bool {
	if authorID == "" {
		return false
	}
	_, ok := a.systemAuthors[authorID]
	return ok
}

// Assemble builds the complete post from raw posts. The result is fully
// deterministic regardless of input order or duplicates:
//
//  1. Dedupe by post ID (exact duplicates collapse to one).
//  2. Identify the root deterministically (ParentID empty or self-referential;
//     if several qualify, the earliest by timestamp then ID wins).
//  3. Drop system/bot-authored posts (the system's own processing replies).
//  4. Order the surviving organic replies by (timestamp, then ID) so shuffled
//     input always yields the same chronological chain.
//
// Forwarded posts are ordinary replies here: their content, forward flag and
// attachments are preserved untouched.
func (a *Assembler) Assemble(posts []Post) (*Thread, error) {
	if len(posts) == 0 {
		return nil, ErrNoPosts
	}

	// 1. Dedupe by ID. Duplicates are the same message, so keeping the first
	//    occurrence is correct; final ordering is imposed by the sort below, so
	//    the output does not depend on which duplicate was kept.
	seen := make(map[string]struct{}, len(posts))
	deduped := make([]Post, 0, len(posts))
	for _, p := range posts {
		if _, ok := seen[p.ID]; ok {
			continue
		}
		seen[p.ID] = struct{}{}
		deduped = append(deduped, p)
	}

	// 2. Identify the root.
	rootIdx := -1
	for i := range deduped {
		if !isRootPost(deduped[i]) {
			continue
		}
		if rootIdx == -1 || rootLess(deduped[i], deduped[rootIdx]) {
			rootIdx = i
		}
	}
	if rootIdx == -1 {
		return nil, ErrMissingRoot
	}

	// 3 & 4. Gather organic replies, excluding the root and system authors, then
	//        order them chronologically with an ID tie-break for determinism.
	replies := make([]Post, 0, len(deduped))
	for i := range deduped {
		if i == rootIdx {
			continue
		}
		if a.IsSystemAuthor(deduped[i].AuthorID) {
			continue
		}
		replies = append(replies, deduped[i])
	}
	sort.Slice(replies, func(i, j int) bool {
		if replies[i].TimestampUnix != replies[j].TimestampUnix {
			return replies[i].TimestampUnix < replies[j].TimestampUnix
		}
		return replies[i].ID < replies[j].ID
	})

	return &Thread{Root: deduped[rootIdx], Replies: replies}, nil
}

// isRootPost reports whether p is a thread root: it has no parent, or points at
// itself (some messengers set ParentID == ID for the root message).
func isRootPost(p Post) bool {
	return p.ParentID == "" || p.ParentID == p.ID
}

// rootLess gives a total, deterministic order over root candidates: earliest
// timestamp first, then lexicographically smallest ID.
func rootLess(a, b Post) bool {
	if a.TimestampUnix != b.TimestampUnix {
		return a.TimestampUnix < b.TimestampUnix
	}
	return a.ID < b.ID
}
