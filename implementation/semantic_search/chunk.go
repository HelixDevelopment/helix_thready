// Package semsearch is the Helix Thready Semantic-search core.
//
// It implements the "Lumen-style" recipe described in
// docs/public/research/mvp/architecture/semantic-search.md:
//
//	chunk source + generated materials  ->  embed  ->  cosine-KNN over a
//	vector store  ->  score-boost source-over-test/doc  ->  min-score noise
//	floor  ->  ranked results.
//
// The production wiring points HelixLLM's OpenAI-compatible /v1/embeddings at
// the [OpenAICompatEmbedder] (HELIX_EMBEDDING_PROVIDER=llama) and a pgvector
// backend behind the [VectorIndex] seam. This module is self-contained,
// depends only on the Go standard library, and ships a real, in-memory
// cosine-KNN [VectorIndex] plus a deterministic embedder so the whole
// chunk -> embed -> rank pipeline is exercised for real in tests without a
// live embeddings server. See README.md and the honest note in EVIDENCE.md
// about the OpenAI-compat path not being exercised against a live server.
//
// Relates to [GAP: 2.1] — HelixLLM's default HashEmbedder is a non-semantic
// stub; real semantic search requires HELIX_EMBEDDING_PROVIDER=llama pointed
// at a real embedding GGUF.
package semsearch

import (
	"crypto/sha256"
	"encoding/hex"
	"strconv"
)

// Kind is the semantic kind of a chunk. For Go source it is the symbol kind
// (func/method/type/const/var); for docs it is "markdown".
type Kind string

const (
	KindFunc     Kind = "func"
	KindMethod   Kind = "method"
	KindType     Kind = "type"
	KindConst    Kind = "const"
	KindVar      Kind = "var"
	KindMarkdown Kind = "markdown"
)

// Chunk is a single retrievable unit of text with provenance. Its ID is the
// SHA-256 of path + symbol + start line, which is stable across runs for the
// same input (a requirement for idempotent re-indexing).
type Chunk struct {
	ID        string // sha256(FilePath + "\x00" + Symbol + "\x00" + StartLine)
	FilePath  string
	Symbol    string
	Kind      Kind
	StartLine int // 1-indexed, inclusive
	EndLine   int // 1-indexed, inclusive
	Content   string
}

// ChunkID computes the stable identifier for a chunk from its provenance.
func ChunkID(filePath, symbol string, startLine int) string {
	h := sha256.New()
	h.Write([]byte(filePath))
	h.Write([]byte{0})
	h.Write([]byte(symbol))
	h.Write([]byte{0})
	h.Write([]byte(strconv.Itoa(startLine)))
	return hex.EncodeToString(h.Sum(nil))
}
