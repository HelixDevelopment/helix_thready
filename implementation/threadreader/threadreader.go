package threadreader

import (
	"context"
	"fmt"
)

// ThreadReader ties a MessageSource to an Assembler: it fetches the raw posts of a
// thread from any messenger and returns the assembled complete post (root + organic
// replies, system replies excluded). This is the reusable, messenger-agnostic seam
// the processing pipeline consumes.
type ThreadReader struct {
	source    MessageSource
	assembler *Assembler
}

// New builds a ThreadReader over source, treating systemAuthorIDs as the system's
// own bot/status-reply authors to exclude from every assembled thread.
func New(source MessageSource, systemAuthorIDs ...string) *ThreadReader {
	return &ThreadReader{
		source:    source,
		assembler: NewAssembler(systemAuthorIDs...),
	}
}

// Read fetches threadID from the source and assembles the complete post.
func (tr *ThreadReader) Read(ctx context.Context, threadID string) (*Thread, error) {
	posts, err := tr.source.FetchThread(ctx, threadID)
	if err != nil {
		return nil, fmt.Errorf("threadreader: fetch thread %q: %w", threadID, err)
	}
	thread, err := tr.assembler.Assemble(posts)
	if err != nil {
		return nil, fmt.Errorf("threadreader: assemble thread %q: %w", threadID, err)
	}
	return thread, nil
}
