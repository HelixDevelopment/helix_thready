package semsearch

import (
	"context"
	"testing"
)

func TestDeterministicEmbedder_Stable(t *testing.T) {
	e := NewDeterministicEmbedder(0)
	ctx := context.Background()
	a, _ := e.Embed(ctx, []string{"the quick brown fox"})
	b, _ := e.Embed(ctx, []string{"the quick brown fox"})
	if len(a[0]) != e.Dim() {
		t.Fatalf("dim = %d, want %d", len(a[0]), e.Dim())
	}
	for i := range a[0] {
		if a[0][i] != b[0][i] {
			t.Fatalf("embedding not deterministic at index %d: %v vs %v", i, a[0][i], b[0][i])
		}
	}
}

func TestCosine_SelfIsOne(t *testing.T) {
	e := NewDeterministicEmbedder(0)
	v, _ := e.Embed(context.Background(), []string{"cosine similarity between vectors"})
	if got := Cosine(v[0], v[0]); got < 0.999 || got > 1.001 {
		t.Fatalf("cosine(v,v) = %v, want ~1", got)
	}
}

// TestCosine_SemanticOrdering proves the deterministic embedder yields a
// MEANINGFUL, non-coincidental similarity ordering: a query is closer (higher
// cosine) to the text that shares its words/subwords than to unrelated text.
func TestCosine_SemanticOrdering(t *testing.T) {
	e := NewDeterministicEmbedder(0)
	ctx := context.Background()

	texts := []string{
		"func Cosine returns the cosine similarity between two float vectors", // 0: near
		"func OpenDatabase establishes a pooled postgres database connection", // 1: far
		"func RenderMarkdown converts markdown documents into html",           // 2: far
	}
	vecs, _ := e.Embed(ctx, texts)
	q, _ := e.Embed(ctx, []string{"cosine similarity between two vectors"})

	near := Cosine(q[0], vecs[0])
	far1 := Cosine(q[0], vecs[1])
	far2 := Cosine(q[0], vecs[2])

	if !(near > far1 && near > far2) {
		t.Fatalf("ordering not meaningful: near=%v far1=%v far2=%v (near must be highest)", near, far1, far2)
	}
	// The relevant match should be clearly, not marginally, closer.
	if near < 0.3 {
		t.Fatalf("near cosine unexpectedly low (%v); overlap signal too weak", near)
	}
}
