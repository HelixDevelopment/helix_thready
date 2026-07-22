package semsearch

import (
	"strings"
	"testing"
)

// sampleGo has known symbols at known 1-indexed line spans:
//
//	 1: package sample
//	 2:
//	 3: import "fmt"
//	 4:
//	 5: type User struct {
//	 6:     ID   int
//	 7:     Name string
//	 8: }
//	 9:
//	10: const MaxUsers = 100
//	11:
//	12: var defaultName = "anon"
//	13:
//	14: func Greet(u User) string {
//	15:     return fmt.Sprintf("hi %s", u.Name)
//	16: }
//	17:
//	18: func (u User) Label() string {
//	19:     return u.Name
//	20: }
const sampleGo = `package sample

import "fmt"

type User struct {
	ID   int
	Name string
}

const MaxUsers = 100

var defaultName = "anon"

func Greet(u User) string {
	return fmt.Sprintf("hi %s", u.Name)
}

func (u User) Label() string {
	return u.Name
}
`

func TestChunkGo_SymbolsKindsSpans(t *testing.T) {
	chunks, err := NewChunker().ChunkGo("sample.go", sampleGo)
	if err != nil {
		t.Fatalf("ChunkGo error: %v", err)
	}
	type want struct {
		symbol string
		kind   Kind
		start  int
		end    int
	}
	wants := []want{
		{"User", KindType, 5, 8},
		{"MaxUsers", KindConst, 10, 10},
		{"defaultName", KindVar, 12, 12},
		{"Greet", KindFunc, 14, 16},
		{"User.Label", KindMethod, 18, 20},
	}
	if len(chunks) != len(wants) {
		t.Fatalf("got %d chunks, want %d: %+v", len(chunks), len(wants), symbols(chunks))
	}
	for i, w := range wants {
		c := chunks[i]
		if c.Symbol != w.symbol || c.Kind != w.kind || c.StartLine != w.start || c.EndLine != w.end {
			t.Errorf("chunk %d = {sym=%q kind=%q %d-%d}, want {sym=%q kind=%q %d-%d}",
				i, c.Symbol, c.Kind, c.StartLine, c.EndLine, w.symbol, w.kind, w.start, w.end)
		}
	}

	// The func chunk's content must be exactly its source lines.
	greet := chunks[3]
	if !strings.HasPrefix(greet.Content, "func Greet(u User) string {") ||
		!strings.HasSuffix(greet.Content, "}") ||
		!strings.Contains(greet.Content, "fmt.Sprintf") {
		t.Errorf("Greet content not the real symbol body:\n%s", greet.Content)
	}

	// IDs must be the stable sha256(path+symbol+start).
	if greet.ID != ChunkID("sample.go", "Greet", 14) {
		t.Errorf("Greet ID = %s, want %s", greet.ID, ChunkID("sample.go", "Greet", 14))
	}
}

func TestChunkGo_ParseError(t *testing.T) {
	if _, err := NewChunker().ChunkGo("bad.go", "package x\nfunc ("); err == nil {
		t.Fatal("expected parse error for malformed Go, got nil")
	}
}

const sampleMD = `# Semantic Search

The subsystem chunks source and generated materials before embedding.

## Query path

A query is embedded with the same model used for indexing.
Cosine KNN returns the nearest chunks.
`

func TestChunkMarkdown_Paragraphs(t *testing.T) {
	chunks := NewChunker().ChunkMarkdown("doc.md", sampleMD)
	// Blocks: [# Semantic Search], [The subsystem...], [## Query path], [A query...]
	if len(chunks) != 4 {
		t.Fatalf("got %d markdown chunks, want 4: %+v", len(chunks), symbols(chunks))
	}
	for _, c := range chunks {
		if c.Kind != KindMarkdown {
			t.Errorf("chunk kind = %q, want %q", c.Kind, KindMarkdown)
		}
	}
	// Heading carried forward to the following paragraph.
	if chunks[0].Symbol != "Semantic Search" {
		t.Errorf("chunk[0].Symbol = %q, want %q", chunks[0].Symbol, "Semantic Search")
	}
	if chunks[1].Symbol != "Semantic Search" {
		t.Errorf("chunk[1].Symbol = %q (should inherit heading), want %q", chunks[1].Symbol, "Semantic Search")
	}
	if chunks[2].Symbol != "Query path" || chunks[3].Symbol != "Query path" {
		t.Errorf("chunk[2/3].Symbol = %q/%q, want %q", chunks[2].Symbol, chunks[3].Symbol, "Query path")
	}
	// The last paragraph spans two lines; its content must be both.
	if !strings.Contains(chunks[3].Content, "same model") || !strings.Contains(chunks[3].Content, "Cosine KNN") {
		t.Errorf("last paragraph content wrong:\n%s", chunks[3].Content)
	}
	if chunks[3].StartLine != 7 || chunks[3].EndLine != 8 {
		t.Errorf("last paragraph span = %d-%d, want 7-8", chunks[3].StartLine, chunks[3].EndLine)
	}
}

func TestChunker_DispatchByExtension(t *testing.T) {
	goChunks, err := NewChunker().Chunk("x.go", sampleGo)
	if err != nil {
		t.Fatalf("dispatch .go error: %v", err)
	}
	if goChunks[0].Kind != KindType {
		t.Errorf(".go dispatch did not use the AST chunker (kind=%q)", goChunks[0].Kind)
	}
	mdChunks, err := NewChunker().Chunk("x.md", sampleMD)
	if err != nil {
		t.Fatalf("dispatch .md error: %v", err)
	}
	if mdChunks[0].Kind != KindMarkdown {
		t.Errorf(".md dispatch did not use the markdown chunker (kind=%q)", mdChunks[0].Kind)
	}
}

func TestChunkGo_Deterministic(t *testing.T) {
	a, _ := NewChunker().ChunkGo("sample.go", sampleGo)
	b, _ := NewChunker().ChunkGo("sample.go", sampleGo)
	if len(a) != len(b) {
		t.Fatalf("non-deterministic chunk count: %d vs %d", len(a), len(b))
	}
	for i := range a {
		if a[i].ID != b[i].ID || a[i].Content != b[i].Content {
			t.Fatalf("chunk %d non-deterministic", i)
		}
	}
}

func symbols(chunks []Chunk) []string {
	out := make([]string, len(chunks))
	for i, c := range chunks {
		out[i] = string(c.Kind) + ":" + c.Symbol
	}
	return out
}
