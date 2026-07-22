# EVIDENCE — Semantic-search core (`digital.vasic.semsearch`)

Physical, reproducible evidence. Captured by running the exact command sequence
below and pasting the real, unedited output. No output was fabricated or
hand-edited. stdlib-only; no external dependencies.

## Environment

```
captured_at: 2026-07-22T12:44:45Z
go_version:  go version go1.26.4-X:nodwarf5 linux/amd64
module:      module digital.vasic.semsearch
uname:       Linux 6.12.41-6.12-alt1 x86_64
```

## Command

```bash
cd implementation/semantic_search && go build ./... && go vet ./... && gofmt -l . && go test ./... -v -race -count=1
```

## Output

```text
$ go build ./...
(exit 0 — no output means success)

$ go vet ./...
(exit 0 — no output means success)

$ gofmt -l .
(no output — all files formatted)

$ go test ./... -v -race -count=1
=== RUN   TestChunkGo_SymbolsKindsSpans
--- PASS: TestChunkGo_SymbolsKindsSpans (0.00s)
=== RUN   TestChunkGo_ParseError
--- PASS: TestChunkGo_ParseError (0.00s)
=== RUN   TestChunkMarkdown_Paragraphs
--- PASS: TestChunkMarkdown_Paragraphs (0.00s)
=== RUN   TestChunker_DispatchByExtension
--- PASS: TestChunker_DispatchByExtension (0.00s)
=== RUN   TestChunkGo_Deterministic
--- PASS: TestChunkGo_Deterministic (0.00s)
=== RUN   TestDeterministicEmbedder_Stable
--- PASS: TestDeterministicEmbedder_Stable (0.00s)
=== RUN   TestCosine_SelfIsOne
--- PASS: TestCosine_SelfIsOne (0.00s)
=== RUN   TestCosine_SemanticOrdering
--- PASS: TestCosine_SemanticOrdering (0.00s)
=== RUN   TestSearch_RetrievesSemanticNearest
--- PASS: TestSearch_RetrievesSemanticNearest (0.00s)
=== RUN   TestSearch_SourceBoostChangesRanking
--- PASS: TestSearch_SourceBoostChangesRanking (0.00s)
=== RUN   TestSearch_MinScoreFloor
--- PASS: TestSearch_MinScoreFloor (0.00s)
=== RUN   TestSearch_MergeOverlappingSameFile
--- PASS: TestSearch_MergeOverlappingSameFile (0.00s)
=== RUN   TestSearch_Deterministic
--- PASS: TestSearch_Deterministic (0.00s)
=== RUN   TestSearch_EmptyQueryRejected
--- PASS: TestSearch_EmptyQueryRejected (0.00s)
=== RUN   TestMemoryIndex_KNNRealCosineOrder
--- PASS: TestMemoryIndex_KNNRealCosineOrder (0.00s)
=== RUN   TestMemoryIndex_TopKTruncates
--- PASS: TestMemoryIndex_TopKTruncates (0.00s)
=== RUN   TestMemoryIndex_Deterministic
--- PASS: TestMemoryIndex_Deterministic (0.00s)
=== RUN   TestMemoryIndex_UpsertReplaces
--- PASS: TestMemoryIndex_UpsertReplaces (0.00s)
=== RUN   TestMemoryIndex_RejectsEmptyID
--- PASS: TestMemoryIndex_RejectsEmptyID (0.00s)
PASS
ok  	digital.vasic.semsearch	1.020s
```

## Summary

- Tests run: 19
- Passed:    19
- Failed:    0
- Race detector: enabled (`-race`), clean (no DATA RACE reports)
- `go build`: exit 0 · `go vet`: exit 0 · `gofmt -l .`: clean · `go test`: ok

### Requirement coverage

| Required behavior | Test | Result |
|---|---|---|
| Go-AST symbol chunker: per-symbol chunks with correct kinds/spans | `TestChunkGo_SymbolsKindsSpans` | PASS |
| Malformed Go source fails loud | `TestChunkGo_ParseError` | PASS |
| Markdown/paragraph chunker: paragraph chunks + heading attribution + spans | `TestChunkMarkdown_Paragraphs` | PASS |
| Dispatch by kind (.go → AST, else → markdown) | `TestChunker_DispatchByExtension` | PASS |
| Chunking determinism (same input → same ids/content) | `TestChunkGo_Deterministic` | PASS |
| Deterministic embedder: same input → identical vectors | `TestDeterministicEmbedder_Stable` | PASS |
| Cosine(v,v) = 1 (real vector math) | `TestCosine_SelfIsOne` | PASS |
| Meaningful cosine ordering (near > far, non-coincidental) | `TestCosine_SemanticOrdering` | PASS |
| KNN retrieves the semantically-closest seeded chunk first | `TestSearch_RetrievesSemanticNearest` | PASS |
| Source boost ×1.15 / test penalty ×0.75 flips a raw-cosine ranking | `TestSearch_SourceBoostChangesRanking` | PASS |
| Min-score floor filters low-similarity noise | `TestSearch_MinScoreFloor` | PASS |
| Overlapping same-file chunks merge into one result | `TestSearch_MergeOverlappingSameFile` | PASS |
| Search determinism (same query/index → same order + scores) | `TestSearch_Deterministic` | PASS |
| Empty query rejected | `TestSearch_EmptyQueryRejected` | PASS |
| In-memory cosine-KNN: real similarity ordering, sorted desc, metadata | `TestMemoryIndex_KNNRealCosineOrder` | PASS |
| Top-k truncation | `TestMemoryIndex_TopKTruncates` | PASS |
| Query determinism + stable tie-break | `TestMemoryIndex_Deterministic` | PASS |
| Upsert replaces (no duplicate id) | `TestMemoryIndex_UpsertReplaces` | PASS |
| Empty id rejected | `TestMemoryIndex_RejectsEmptyID` | PASS |

## Real ranking snapshot (example query → top result)

Produced by a standalone program that imports the module (deterministic
embedder + in-memory cosine-KNN), indexing a mixed corpus of Go source symbols,
a `_test.go` symbol, and a Markdown paragraph. Real, unedited output:

```text
QUERY: "cosine knn nearest vectors in the index"
  #1  score=0.6302  base=0.5480  MemoryIndex.Query index.go:40-60       "method"
  #2  score=0.5652  base=0.7536  TestQuery         index_test.go:12-28  "func"
  #3  score=0.5221  base=0.4540  Cosine            vectors.go:10-14     "func"

QUERY: "postgres database connection pool"
  #1  score=0.7589  base=0.6599  OpenDatabase      db.go:5-20           "func"
  #2  score=0.1141  base=0.0992  Cosine            vectors.go:10-14     "func"
  #3  score=0.0000  base=0.0000  MemoryIndex.Query index.go:40-60       "method"
```

Read this closely — it is the anti-bluff proof that the pipeline is real, not a
lookup table:

- **The boost genuinely re-ranks.** For query 1, the `_test.go` symbol
  `TestQuery` has the **highest raw cosine** (`base=0.7536`) yet lands at #2:
  the ×0.75 test penalty drops it to `0.5652`, while the source method
  `MemoryIndex.Query` (`base=0.5480`) is lifted by ×1.15 to `0.6302` and takes
  #1. The ranking flip is caused by the boost, not by the raw similarity.
- **The similarity is meaningful.** Query 2 ("postgres database connection
  pool") ranks `OpenDatabase` first by a wide margin (`0.7589`) and drives the
  unrelated `MemoryIndex.Query` to `0.0000` — cosine reflects real lexical/
  subword overlap, not noise.

## Honest scope note (no bluff)

- The **deterministic embedder** (`DeterministicEmbedder`) is a *real*
  feature-hashing model (whole-word + character-trigram signed hashing, then
  L2-normalize). Its cosine similarity is genuinely meaningful and stable, which
  is what lets every ranking/boost/floor/merge assertion above be a real
  similarity claim rather than a mock. It is **not** a random or opaque
  hash-of-the-whole-string stub.
- The **`OpenAICompatEmbedder`** — the production wiring point for
  llama.cpp/HelixLLM's OpenAI-compatible `/v1/embeddings` — is implemented as a
  real `net/http` client (POST `{BaseURL}/v1/embeddings`, decode
  `{data:[{embedding,index}]}`, sort by `index`). It **is NOT exercised by the
  test suite**: there is no live embeddings server in this environment, so no
  test posts to it. This is stated plainly rather than faked with a mocked
  "live" run. The cosine/KNN/scoring pipeline is proven for real by the
  deterministic embedder; swapping in `OpenAICompatEmbedder` (or a pgvector
  `VectorIndex`) changes only the vector source/store, not the proven ranking
  logic.
- Relates to **[GAP: 2.1]**: HelixLLM's default `HashEmbedder` is a non-semantic
  stub; the production path must set `HELIX_EMBEDDING_PROVIDER=llama` with a real
  embedding GGUF. This module's `Embedder` seam is where that real provider is
  injected.

## Verdict

**READY** (as a self-contained core). The module compiles (`go build` exit 0),
passes `go vet` (exit 0) and `gofmt -l .` (clean), and all 19 tests pass under
the race detector (`-race`) with real cosine math on real vectors. The
chunk → embed → cosine-KNN → boost → floor → merge → rank pipeline is proven
end-to-end with a deterministic-but-meaningful embedder.

**PARTIAL** only on the live-embeddings edge: the `OpenAICompatEmbedder` →
HelixLLM `/v1/embeddings` network path and a pgvector-backed `VectorIndex` are
implemented/seam-ready but not integration-tested here (no live model server /
Postgres in this environment). Those are the two documented wiring points to
exercise against a running HelixLLM (`HELIX_EMBEDDING_PROVIDER=llama`) and
pgvector to reach full end-to-end READY.
