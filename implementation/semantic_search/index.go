package semsearch

import (
	"errors"
	"sort"
)

// Match is a single scored hit from a [VectorIndex] query.
type Match struct {
	ID    string
	Score float32 // cosine similarity of the stored vector to the query vector
	Meta  map[string]any
}

// VectorIndex is the store/search seam. The production implementation is a
// pgvector-backed store (digital.vasic.vectordb); [MemoryIndex] is a real,
// in-memory cosine-KNN implementation used by the engine and tests.
type VectorIndex interface {
	// Upsert inserts or replaces the vector and metadata for id.
	Upsert(id string, vec []float32, meta map[string]any) error
	// Query returns the top-k entries by cosine similarity, descending. k < 0
	// returns all entries. Ordering is deterministic: ties break by id ascending.
	Query(vec []float32, k int) ([]Match, error)
	// Len reports the number of stored vectors.
	Len() int
}

type indexEntry struct {
	vec  []float32
	meta map[string]any
}

// MemoryIndex is an in-memory [VectorIndex] doing real cosine-similarity KNN.
type MemoryIndex struct {
	ids     []string
	entries map[string]indexEntry
}

// NewMemoryIndex returns an empty in-memory index.
func NewMemoryIndex() *MemoryIndex {
	return &MemoryIndex{entries: make(map[string]indexEntry)}
}

// Upsert implements [VectorIndex]. The vector is copied defensively.
func (m *MemoryIndex) Upsert(id string, vec []float32, meta map[string]any) error {
	if id == "" {
		return errors.New("semsearch: MemoryIndex.Upsert: empty id")
	}
	cp := make([]float32, len(vec))
	copy(cp, vec)
	if _, ok := m.entries[id]; !ok {
		m.ids = append(m.ids, id)
	}
	m.entries[id] = indexEntry{vec: cp, meta: meta}
	return nil
}

// Len implements [VectorIndex].
func (m *MemoryIndex) Len() int { return len(m.ids) }

// Query implements [VectorIndex] with a full cosine scan (correct and fast at
// MVP scale; a pgvector/HNSW backend serves the production ANN path).
func (m *MemoryIndex) Query(vec []float32, k int) ([]Match, error) {
	matches := make([]Match, 0, len(m.ids))
	for _, id := range m.ids {
		e := m.entries[id]
		matches = append(matches, Match{ID: id, Score: Cosine(vec, e.vec), Meta: e.meta})
	}
	sort.SliceStable(matches, func(i, j int) bool {
		if matches[i].Score != matches[j].Score {
			return matches[i].Score > matches[j].Score
		}
		return matches[i].ID < matches[j].ID
	})
	if k >= 0 && k < len(matches) {
		matches = matches[:k]
	}
	return matches, nil
}
