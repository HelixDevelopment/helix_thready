package semsearch

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
)

// Config tunes the scoring pipeline. Zero values are replaced by
// [DefaultConfig] values in [NewEngine] except MinScore, which defaults to 0
// (no floor) and is meaningful at 0.
type Config struct {
	SourceBoost float32 // multiplier for non-test Go source chunks (default 1.15)
	TestPenalty float32 // multiplier for *_test.go chunks (default 0.75)
	MinScore    float32 // drop results whose boosted score is below this floor
}

// DefaultConfig returns the score-boost/penalty values from the architecture
// spec (source ×1.15, test ×0.75, no min-score floor).
func DefaultConfig() Config {
	return Config{SourceBoost: 1.15, TestPenalty: 0.75, MinScore: 0}
}

// Result is one ranked search hit. For merged hits, Chunk spans the union of
// the merged chunks and MergedIDs lists every underlying chunk id.
type Result struct {
	Chunk     Chunk
	Score     float32 // boosted score used for ranking
	BaseScore float32 // raw cosine before boost/penalty
	MergedIDs []string
}

// Engine indexes chunks (embed + upsert) and answers meaning-based queries.
type Engine struct {
	emb    Embedder
	index  VectorIndex
	chunks map[string]Chunk
	cfg    Config
}

// NewEngine wires an embedder and a vector index. Nil emb/index default to a
// [DeterministicEmbedder] and a [MemoryIndex]. Zero SourceBoost/TestPenalty
// default to the spec values.
func NewEngine(emb Embedder, index VectorIndex, cfg Config) *Engine {
	if emb == nil {
		emb = NewDeterministicEmbedder(0)
	}
	if index == nil {
		index = NewMemoryIndex()
	}
	if cfg.SourceBoost == 0 {
		cfg.SourceBoost = 1.15
	}
	if cfg.TestPenalty == 0 {
		cfg.TestPenalty = 0.75
	}
	return &Engine{emb: emb, index: index, chunks: make(map[string]Chunk), cfg: cfg}
}

// Index embeds every chunk's content in one batch and upserts it into the
// vector index, keeping the chunk for hydration at query time.
func (e *Engine) Index(ctx context.Context, chunks []Chunk) error {
	if len(chunks) == 0 {
		return nil
	}
	texts := make([]string, len(chunks))
	for i, c := range chunks {
		texts[i] = c.Content
	}
	vecs, err := e.emb.Embed(ctx, texts)
	if err != nil {
		return fmt.Errorf("semsearch: embed chunks: %w", err)
	}
	if len(vecs) != len(chunks) {
		return fmt.Errorf("semsearch: embedder returned %d vectors for %d chunks", len(vecs), len(chunks))
	}
	for i, c := range chunks {
		meta := map[string]any{"filePath": c.FilePath, "kind": string(c.Kind), "symbol": c.Symbol}
		if err := e.index.Upsert(c.ID, vecs[i], meta); err != nil {
			return fmt.Errorf("semsearch: upsert %s: %w", c.ID, err)
		}
		e.chunks[c.ID] = c
	}
	return nil
}

// Search embeds the query, runs cosine-KNN, applies the source boost / test
// penalty, drops results below the min-score floor, merges overlapping
// same-file chunks, and returns the top-k ranked results.
func (e *Engine) Search(ctx context.Context, query string, k int) ([]Result, error) {
	if strings.TrimSpace(query) == "" {
		return nil, errors.New("semsearch: empty query")
	}
	vecs, err := e.emb.Embed(ctx, []string{query})
	if err != nil {
		return nil, fmt.Errorf("semsearch: embed query: %w", err)
	}
	// Retrieve the full candidate pool so the boost/penalty can re-rank across
	// everything before we truncate to k.
	matches, err := e.index.Query(vecs[0], e.index.Len())
	if err != nil {
		return nil, fmt.Errorf("semsearch: knn query: %w", err)
	}

	cands := make([]Result, 0, len(matches))
	for _, mt := range matches {
		c, ok := e.chunks[mt.ID]
		if !ok {
			continue
		}
		boosted := e.boost(mt.Score, c)
		if boosted < e.cfg.MinScore {
			continue
		}
		cands = append(cands, Result{
			Chunk:     c,
			Score:     boosted,
			BaseScore: mt.Score,
			MergedIDs: []string{c.ID},
		})
	}
	sort.SliceStable(cands, func(i, j int) bool {
		if cands[i].Score != cands[j].Score {
			return cands[i].Score > cands[j].Score
		}
		return cands[i].Chunk.ID < cands[j].Chunk.ID
	})

	merged := mergeOverlaps(cands)
	if k >= 0 && k < len(merged) {
		merged = merged[:k]
	}
	return merged, nil
}

// boost applies the source-over-test/doc weighting from the spec.
func (e *Engine) boost(score float32, c Chunk) float32 {
	switch {
	case isTestFile(c.FilePath):
		return score * e.cfg.TestPenalty
	case isGoSource(c.FilePath):
		return score * e.cfg.SourceBoost
	default: // docs / markdown / other: neutral
		return score
	}
}

func isTestFile(p string) bool { return strings.HasSuffix(p, "_test.go") }

func isGoSource(p string) bool { return strings.HasSuffix(p, ".go") && !isTestFile(p) }

// mergeOverlaps collapses adjacent/overlapping same-file results into one. The
// input must already be sorted by score descending, so the first (highest)
// result of an overlapping group keeps its rank and score.
func mergeOverlaps(in []Result) []Result {
	var out []Result
	for _, r := range in {
		target := -1
		for i := range out {
			if out[i].Chunk.FilePath == r.Chunk.FilePath && spansTouch(out[i].Chunk, r.Chunk) {
				target = i
				break
			}
		}
		if target < 0 {
			out = append(out, r)
			continue
		}
		out[target] = mergeTwo(out[target], r)
	}
	return out
}

// spansTouch reports whether two line spans overlap or are adjacent (gap <= 1).
func spansTouch(a, b Chunk) bool {
	return a.StartLine <= b.EndLine+1 && b.StartLine <= a.EndLine+1
}

// mergeTwo folds b into a, keeping a's (higher) score and rank while unioning
// the span, ordering content by start line, and recording both ids.
func mergeTwo(a, b Result) Result {
	first, second := a, b
	if b.Chunk.StartLine < a.Chunk.StartLine {
		first, second = b, a
	}
	res := a // keep a's Score/BaseScore (a is the higher-ranked one)
	res.Chunk.StartLine = min(a.Chunk.StartLine, b.Chunk.StartLine)
	res.Chunk.EndLine = max(a.Chunk.EndLine, b.Chunk.EndLine)
	res.Chunk.Symbol = joinSymbols(first.Chunk.Symbol, second.Chunk.Symbol)
	res.Chunk.Content = first.Chunk.Content + "\n" + second.Chunk.Content
	res.MergedIDs = append(append([]string{}, a.MergedIDs...), b.MergedIDs...)
	return res
}

func joinSymbols(a, b string) string {
	switch {
	case a == "":
		return b
	case b == "":
		return a
	default:
		return a + "+" + b
	}
}
