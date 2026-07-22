package semsearch

import (
	"context"
	"testing"
)

// mkChunk builds a Chunk directly (bypassing the chunker) so tests can control
// file path, span, and content precisely.
func mkChunk(path, symbol string, kind Kind, start, end int, content string) Chunk {
	return Chunk{
		ID:        ChunkID(path, symbol, start),
		FilePath:  path,
		Symbol:    symbol,
		Kind:      kind,
		StartLine: start,
		EndLine:   end,
		Content:   content,
	}
}

func indexChunks(t *testing.T, eng *Engine, chunks []Chunk) {
	t.Helper()
	if err := eng.Index(context.Background(), chunks); err != nil {
		t.Fatalf("Index error: %v", err)
	}
}

// TestSearch_RetrievesSemanticNearest: a query semantically closest to a
// specific seeded chunk must retrieve THAT chunk first — real similarity
// ordering, driven by the deterministic embedder's cosine.
func TestSearch_RetrievesSemanticNearest(t *testing.T) {
	eng := NewEngine(nil, nil, DefaultConfig())
	chunks := []Chunk{
		mkChunk("vectors.go", "Cosine", KindFunc, 1, 3,
			"func Cosine returns the cosine similarity between two float vectors"),
		mkChunk("render.go", "RenderMarkdown", KindFunc, 1, 3,
			"func RenderMarkdown converts markdown documents into html for the browser"),
		mkChunk("db.go", "OpenDatabase", KindFunc, 1, 3,
			"func OpenDatabase establishes a pooled postgres database connection"),
	}
	indexChunks(t, eng, chunks)

	res, err := eng.Search(context.Background(), "cosine similarity between two vectors", 3)
	if err != nil {
		t.Fatalf("Search error: %v", err)
	}
	if len(res) == 0 || res[0].Chunk.Symbol != "Cosine" {
		t.Fatalf("top result = %q, want Cosine (real semantic ranking)", topSymbol(res))
	}

	// A different query must retrieve the database chunk first.
	res2, _ := eng.Search(context.Background(), "postgres database connection pool", 3)
	if len(res2) == 0 || res2[0].Chunk.Symbol != "OpenDatabase" {
		t.Fatalf("top result = %q, want OpenDatabase", topSymbol(res2))
	}
}

// TestSearch_SourceBoostChangesRanking: the source ×1.15 / test ×0.75 weighting
// must flip a ranking where the test file has a higher RAW cosine. We first
// prove the raw cosine ordering (test > source), then prove the boosted search
// ranks source first.
func TestSearch_SourceBoostChangesRanking(t *testing.T) {
	ctx := context.Background()
	emb := NewDeterministicEmbedder(0)

	// Both chunks share every query token, so their raw cosines are CLOSE
	// (within the boost ratio 1.15/0.75 = 1.53). The test chunk carries fewer
	// extra tokens, giving it a smaller norm and thus a marginally HIGHER raw
	// cosine — which the source boost must then overturn.
	query := "parse user configuration file"
	source := mkChunk("config.go", "ParseConfig", KindFunc, 1, 3,
		"parse user configuration file from the settings store")
	test := mkChunk("config_test.go", "TestParseConfig", KindFunc, 1, 3,
		"parse user configuration file test fixture")

	// Establish the RAW cosine ordering: the test chunk is closer to the query.
	vecs, _ := emb.Embed(ctx, []string{source.Content, test.Content})
	q, _ := emb.Embed(ctx, []string{query})
	rawSource := Cosine(q[0], vecs[0])
	rawTest := Cosine(q[0], vecs[1])
	if !(rawTest > rawSource) {
		t.Fatalf("precondition failed: raw cosine test=%v not > source=%v", rawTest, rawSource)
	}

	eng := NewEngine(emb, NewMemoryIndex(), DefaultConfig())
	indexChunks(t, eng, []Chunk{source, test})
	res, err := eng.Search(ctx, query, 2)
	if err != nil {
		t.Fatalf("Search error: %v", err)
	}
	if len(res) != 2 {
		t.Fatalf("got %d results, want 2", len(res))
	}
	if res[0].Chunk.FilePath != "config.go" {
		t.Fatalf("boost failed to flip ranking: top=%q (raw test=%v>source=%v, boosted src=%v test=%v)",
			res[0].Chunk.FilePath, rawTest, rawSource, rawSource*1.15, rawTest*0.75)
	}
	// Sanity: the boosted score for the source is its raw cosine ×1.15.
	if got := res[0].Score; got < rawSource*1.15-1e-4 || got > rawSource*1.15+1e-4 {
		t.Fatalf("source boosted score = %v, want %v", got, rawSource*1.15)
	}
}

// TestSearch_MinScoreFloor: the noise floor must drop low-similarity chunks.
// Threshold is derived from the real scores (no magic constant), so the test
// asserts the mechanism, not a coincidence.
func TestSearch_MinScoreFloor(t *testing.T) {
	ctx := context.Background()
	relevant := mkChunk("a.md", "", KindMarkdown, 1, 1,
		"semantic search embeds text into vectors for cosine similarity")
	noise := mkChunk("b.md", "", KindMarkdown, 1, 1,
		"unrelated cooking recipe with tomatoes and basil")

	eng0 := NewEngine(nil, nil, DefaultConfig()) // no floor
	indexChunks(t, eng0, []Chunk{relevant, noise})
	all, _ := eng0.Search(ctx, "vector cosine similarity search", 10)
	if len(all) != 2 {
		t.Fatalf("without floor got %d results, want 2", len(all))
	}
	var relScore, noiseScore float32
	for _, r := range all {
		if r.Chunk.FilePath == "a.md" {
			relScore = r.Score
		} else {
			noiseScore = r.Score
		}
	}
	if !(relScore > noiseScore) {
		t.Fatalf("precondition failed: relevant=%v not > noise=%v", relScore, noiseScore)
	}

	// Floor strictly between the two scores must drop the noise chunk only.
	mid := (relScore + noiseScore) / 2
	eng := NewEngine(nil, nil, Config{SourceBoost: 1.15, TestPenalty: 0.75, MinScore: mid})
	indexChunks(t, eng, []Chunk{relevant, noise})
	filtered, _ := eng.Search(ctx, "vector cosine similarity search", 10)
	if len(filtered) != 1 || filtered[0].Chunk.FilePath != "a.md" {
		t.Fatalf("min-score floor did not filter noise: got %+v", filePaths(filtered))
	}
}

// TestSearch_MergeOverlappingSameFile: two adjacent same-file chunks that are
// both retrieved must collapse into a single result spanning both; a different
// file must not be merged in.
func TestSearch_MergeOverlappingSameFile(t *testing.T) {
	ctx := context.Background()
	// Two adjacent chunks of the same file (spans 10-15 and 16-20 -> gap 0).
	c1 := mkChunk("engine.go", "SearchPartA", KindFunc, 10, 15,
		"func SearchPartA embeds the query vector and runs cosine knn over the index")
	c2 := mkChunk("engine.go", "SearchPartB", KindFunc, 16, 20,
		"func SearchPartB applies the boost and cosine knn score floor to the vector results")
	// A distractor in a different file must stay separate.
	other := mkChunk("unrelated.go", "Ping", KindFunc, 1, 2,
		"func Ping returns pong for a health check endpoint")

	eng := NewEngine(nil, nil, DefaultConfig())
	indexChunks(t, eng, []Chunk{c1, c2, other})

	res, err := eng.Search(ctx, "cosine knn over the query vector index score", 10)
	if err != nil {
		t.Fatalf("Search error: %v", err)
	}

	// engine.go's two adjacent chunks must appear as exactly one merged result.
	var engineResults int
	var merged Result
	for _, r := range res {
		if r.Chunk.FilePath == "engine.go" {
			engineResults++
			merged = r
		}
	}
	if engineResults != 1 {
		t.Fatalf("expected engine.go chunks merged into 1 result, got %d: %+v", engineResults, filePaths(res))
	}
	if merged.Chunk.StartLine != 10 || merged.Chunk.EndLine != 20 {
		t.Fatalf("merged span = %d-%d, want 10-20", merged.Chunk.StartLine, merged.Chunk.EndLine)
	}
	if len(merged.MergedIDs) != 2 {
		t.Fatalf("merged ids = %v, want 2 underlying chunks", merged.MergedIDs)
	}
}

// TestSearch_Deterministic: identical query over identical index yields the
// identical ranked result order and scores every time.
func TestSearch_Deterministic(t *testing.T) {
	eng := NewEngine(nil, nil, DefaultConfig())
	indexChunks(t, eng, []Chunk{
		mkChunk("a.go", "A", KindFunc, 1, 2, "alpha vector cosine similarity"),
		mkChunk("b.go", "B", KindFunc, 1, 2, "beta database connection postgres"),
		mkChunk("c.go", "C", KindFunc, 1, 2, "gamma markdown html render"),
	})
	first, _ := eng.Search(context.Background(), "vector cosine similarity", 3)
	for trial := 0; trial < 5; trial++ {
		again, _ := eng.Search(context.Background(), "vector cosine similarity", 3)
		if len(first) != len(again) {
			t.Fatalf("trial %d: length changed", trial)
		}
		for i := range first {
			if first[i].Chunk.ID != again[i].Chunk.ID || first[i].Score != again[i].Score {
				t.Fatalf("trial %d idx %d: non-deterministic (%s/%v vs %s/%v)",
					trial, i, first[i].Chunk.ID, first[i].Score, again[i].Chunk.ID, again[i].Score)
			}
		}
	}
}

func TestSearch_EmptyQueryRejected(t *testing.T) {
	eng := NewEngine(nil, nil, DefaultConfig())
	if _, err := eng.Search(context.Background(), "   ", 5); err == nil {
		t.Fatal("expected error for empty query")
	}
}

func topSymbol(res []Result) string {
	if len(res) == 0 {
		return "<none>"
	}
	return res[0].Chunk.Symbol
}

func filePaths(res []Result) []string {
	out := make([]string, len(res))
	for i, r := range res {
		out[i] = r.Chunk.FilePath
	}
	return out
}
