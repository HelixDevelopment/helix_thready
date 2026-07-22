# Semantic-search core — `digital.vasic.semsearch`

A real, compiling, test-green Go module implementing the Helix Thready
**Semantic-search core** — the "Lumen-style" recipe from
[`docs/public/research/mvp/architecture/semantic-search.md`](../../docs/public/research/mvp/architecture/semantic-search.md):

```
chunk source + generated materials
      -> embed
      -> cosine-KNN over a vector store
      -> score-boost source-over-test/doc
      -> min-score noise floor
      -> merge overlapping same-file chunks
      -> ranked results
```

Self-contained, **standard library only**, module path `digital.vasic.semsearch`,
Go 1.26.

## Why this exists

The architecture doc mandates a Lumen-style "search a codebase by meaning"
capability, re-implemented in-house on `digital.vasic.embeddings` +
`digital.vasic.vectordb` (pgvector) + `digital.vasic.rag`, driven by HelixLLM's
OpenAI-compatible `/v1/embeddings`. This module is the **core pipeline** behind
that: the chunker, the embedder seam, the cosine-KNN vector index, and the
scoring/ranking engine — with a deterministic local embedder so the whole
pipeline is proven for real in tests without a live model server.

## API

### Chunker — split text into retrievable chunks

```go
ch := semsearch.NewChunker()

// Dispatch by kind: .go -> Go-AST symbol chunker; else -> Markdown/paragraph.
chunks, err := ch.Chunk("engine.go", goSource)   // func/method/type/const/var
docs,   _   := ch.Chunk("design.md", markdown)   // paragraph/section chunks
```

Each `Chunk` carries `{ID, FilePath, Symbol, Kind, StartLine, EndLine, Content}`.
`ID` is `sha256(FilePath + "\x00" + Symbol + "\x00" + StartLine)` — stable across
runs for idempotent re-indexing. The Go chunker uses the stdlib `go/ast` +
`go/parser`, so symbol names and line spans are exact, not heuristic.

### Embedder — text → vectors

```go
type Embedder interface {
    Embed(ctx context.Context, texts []string) ([][]float32, error)
}
```

Two implementations:

- **`DeterministicEmbedder`** — a real feature-hashing embedder (whole-word +
  character-trigram signed hashing, then L2-normalize). Cosine similarity is
  meaningful and stable: texts sharing more words/subwords score higher, and the
  same input always yields the same vector. Used by the tests to prove the
  cosine/ranking pipeline for real.
- **`OpenAICompatEmbedder`** — the production wiring point. A real `net/http`
  client that POSTs to `{BaseURL}/v1/embeddings` and decodes the OpenAI
  `{data:[{embedding,index}]}` shape (sorting by `index` per the contract). See
  the wiring section below. Not exercised by tests (no live server) — see
  EVIDENCE.md.

### VectorIndex — store + cosine-KNN

```go
type VectorIndex interface {
    Upsert(id string, vec []float32, meta map[string]any) error
    Query(vec []float32, k int) ([]Match, error) // top-k by cosine, desc
    Len() int
}
```

`MemoryIndex` is a real in-memory cosine-KNN implementation (full scan, exact,
deterministic tie-break by id). In production this seam is a pgvector-backed
store (`digital.vasic.vectordb`) — the engine does not care which.

### Engine — index + search

```go
eng := semsearch.NewEngine(nil, nil, semsearch.DefaultConfig())
//                          ^emb  ^idx  ^Config{SourceBoost:1.15, TestPenalty:0.75, MinScore:0}
_   = eng.Index(ctx, chunks)               // embed + upsert
res, _ := eng.Search(ctx, "cosine knn nearest vectors", 10)
```

`Search` embeds the query, runs cosine-KNN over the full candidate pool, applies
the **source boost (×1.15)** and **test-file penalty (×0.75)**, drops results
below the **min-score floor**, **merges overlapping/adjacent same-file chunks**
into one result, and returns the top-k. Each `Result` carries the (possibly
merged) `Chunk`, the boosted `Score`, the raw `BaseScore`, and `MergedIDs`.

Scoring rules:

| Chunk | Multiplier |
|---|---|
| Go source (`*.go`, not test) | ×`SourceBoost` (1.15) |
| Go test (`*_test.go`) | ×`TestPenalty` (0.75) |
| Docs / markdown / other | ×1.0 (neutral) |

## The llama.cpp / HelixLLM wiring point

The production embedder is HelixLLM's OpenAI-compatible `/v1/embeddings`
endpoint with `HELIX_EMBEDDING_PROVIDER=llama` (a real embedding GGUF). Wire it
by swapping the embedder — nothing else in the pipeline changes:

```go
emb := semsearch.NewOpenAICompatEmbedder(
    "http://127.0.0.1:8080", // HelixLLM server root (POSTs to /v1/embeddings)
    "local",                 // API key ("local" for llama.cpp)
    "voyage-code-3",          // code-tuned embedding model
)
eng := semsearch.NewEngine(emb, pgvectorIndex /* your VectorIndex */, semsearch.DefaultConfig())
```

> **[GAP: 2.1] — fail loud, do not warn.** HelixLLM's *default* local embedder is
> a non-semantic `HashEmbedder` stub: semantic search built on it silently
> returns garbage relevance. The production deployment MUST set
> `HELIX_EMBEDDING_PROVIDER=llama` with a real embedding GGUF. This module's
> `Embedder` seam is exactly where that real provider is injected; the
> deterministic embedder used in tests is a *genuine* similarity model, not the
> HashEmbedder trap.

The `OpenAICompatEmbedder` path is implemented but not integration-tested here
(no live embeddings server in this environment); likewise a pgvector
`VectorIndex` is the production store behind the same interface. See EVIDENCE.md
for the honest scope note.

## Run the tests

```bash
cd implementation/semantic_search
go build ./... && go vet ./... && gofmt -l . && go test ./... -v -race -count=1
```

All 19 tests pass under `-race` with real cosine math. See
[EVIDENCE.md](EVIDENCE.md) for captured output and a real query→ranking snapshot
that demonstrates the source-over-test boost flipping a ranking.

## Files

| File | Purpose |
|---|---|
| `chunk.go` | `Chunk`, `Kind`, `ChunkID` (stable sha256 id) |
| `chunker.go` | `Chunker` — Go-AST symbol chunker + Markdown/paragraph chunker |
| `embedder.go` | `Embedder` seam, `DeterministicEmbedder`, `Cosine` |
| `openai_embedder.go` | `OpenAICompatEmbedder` — real HelixLLM `/v1/embeddings` client |
| `index.go` | `VectorIndex` seam + `MemoryIndex` (in-memory cosine-KNN) |
| `engine.go` | `Engine` — index + search + boost + floor + merge + rank |
| `*_test.go` | TDD suite (chunking, embedding, KNN, engine) |
| `EVIDENCE.md` | Captured build/vet/gofmt/test output + real ranking |
```
