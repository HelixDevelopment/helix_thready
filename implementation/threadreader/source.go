package threadreader

import "context"

// MessageSource is the messenger-agnostic seam over a concrete messenger
// (Telegram via gotd/td, Max via OneMe, Slack, ...). An implementation returns
// the RAW posts of a thread — the root plus every reply — in whatever order the
// underlying platform provides them. All ordering, dedup and system-reply
// filtering is the Assembler's job, so every messenger adapter stays trivial.
type MessageSource interface {
	// FetchThread returns the raw posts (root + all replies) for threadID.
	// The order is not guaranteed; the Assembler makes it deterministic.
	FetchThread(ctx context.Context, threadID string) ([]Post, error)
}
