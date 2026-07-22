package semsearch

import (
	"context"
	"testing"
)

// buildIndex embeds the given texts with the deterministic embedder and upserts
// them under ids "0","1",... returning the index and the embedder.
func buildIndex(t *testing.T, texts []string) (*MemoryIndex, *DeterministicEmbedder) {
	t.Helper()
	e := NewDeterministicEmbedder(0)
	vecs, _ := e.Embed(context.Background(), texts)
	idx := NewMemoryIndex()
	for i, v := range vecs {
		if err := idx.Upsert(string(rune('0'+i)), v, map[string]any{"text": texts[i]}); err != nil {
			t.Fatalf("upsert %d: %v", i, err)
		}
	}
	return idx, e
}

func TestMemoryIndex_KNNRealCosineOrder(t *testing.T) {
	texts := []string{
		"cosine similarity between two float vectors", // id 0
		"postgres database connection pooling",        // id 1
		"markdown to html document rendering",         // id 2
	}
	idx, e := buildIndex(t, texts)
	if idx.Len() != 3 {
		t.Fatalf("Len = %d, want 3", idx.Len())
	}
	q, _ := e.Embed(context.Background(), []string{"database connection pool for postgres"})
	res, err := idx.Query(q[0], 3)
	if err != nil {
		t.Fatalf("Query error: %v", err)
	}
	if res[0].ID != "1" {
		t.Fatalf("nearest = %q, want %q (real cosine must rank the database text first)", res[0].ID, "1")
	}
	// Scores must be sorted descending.
	for i := 1; i < len(res); i++ {
		if res[i-1].Score < res[i].Score {
			t.Fatalf("not sorted desc: %v < %v at %d", res[i-1].Score, res[i].Score, i)
		}
	}
	// Metadata round-trips.
	if res[0].Meta["text"] != texts[1] {
		t.Fatalf("meta not carried: got %v", res[0].Meta["text"])
	}
}

func TestMemoryIndex_TopKTruncates(t *testing.T) {
	idx, e := buildIndex(t, []string{"alpha vectors", "beta database", "gamma markdown"})
	q, _ := e.Embed(context.Background(), []string{"alpha vectors"})
	res, _ := idx.Query(q[0], 2)
	if len(res) != 2 {
		t.Fatalf("top-2 returned %d results", len(res))
	}
}

func TestMemoryIndex_Deterministic(t *testing.T) {
	idx, e := buildIndex(t, []string{"one", "two", "three", "four"})
	q, _ := e.Embed(context.Background(), []string{"two"})
	first, _ := idx.Query(q[0], 4)
	for trial := 0; trial < 5; trial++ {
		again, _ := idx.Query(q[0], 4)
		for i := range first {
			if first[i].ID != again[i].ID || first[i].Score != again[i].Score {
				t.Fatalf("non-deterministic query at trial %d idx %d", trial, i)
			}
		}
	}
}

func TestMemoryIndex_UpsertReplaces(t *testing.T) {
	e := NewDeterministicEmbedder(0)
	idx := NewMemoryIndex()
	v1, _ := e.Embed(context.Background(), []string{"first"})
	v2, _ := e.Embed(context.Background(), []string{"second"})
	_ = idx.Upsert("x", v1[0], nil)
	_ = idx.Upsert("x", v2[0], map[string]any{"v": 2})
	if idx.Len() != 1 {
		t.Fatalf("Len = %d after replace, want 1", idx.Len())
	}
	res, _ := idx.Query(v2[0], 1)
	if res[0].Meta["v"] != 2 {
		t.Fatalf("replace did not take effect: %v", res[0].Meta)
	}
}

func TestMemoryIndex_RejectsEmptyID(t *testing.T) {
	if err := NewMemoryIndex().Upsert("", []float32{1}, nil); err == nil {
		t.Fatal("expected error upserting empty id")
	}
}
