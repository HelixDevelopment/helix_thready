package semsearch

import (
	"context"
	"hash/fnv"
	"math"
	"strings"
	"unicode"
)

// Embedder turns a batch of texts into a batch of vectors. The production
// implementation is [OpenAICompatEmbedder] (HelixLLM /v1/embeddings); the
// [DeterministicEmbedder] is a real, local, dependency-free embedder used by
// tests so cosine similarity is meaningful and stable.
type Embedder interface {
	Embed(ctx context.Context, texts []string) ([][]float32, error)
}

// DeterministicEmbedder is a real feature-hashing embedder: it turns text into
// a fixed-dimension vector from whole-word tokens plus character trigrams,
// using signed feature hashing, then L2-normalizes. It is NOT a random or
// opaque hash-of-the-whole-string stub — it is a genuine bag-of-features model
// whose cosine similarity is MEANINGFUL: texts that share more words and
// subword n-grams get a higher cosine, and identical input always produces an
// identical vector (determinism). This is what lets the tests prove the
// cosine/ranking pipeline for real without a live embeddings server.
type DeterministicEmbedder struct {
	dim int
}

// NewDeterministicEmbedder returns an embedder producing dim-dimensional
// vectors. dim <= 0 defaults to 256.
func NewDeterministicEmbedder(dim int) *DeterministicEmbedder {
	if dim <= 0 {
		dim = 256
	}
	return &DeterministicEmbedder{dim: dim}
}

// Dim reports the vector dimension.
func (d *DeterministicEmbedder) Dim() int { return d.dim }

// Embed implements [Embedder].
func (d *DeterministicEmbedder) Embed(_ context.Context, texts []string) ([][]float32, error) {
	out := make([][]float32, len(texts))
	for i, t := range texts {
		out[i] = d.vec(t)
	}
	return out, nil
}

func (d *DeterministicEmbedder) vec(text string) []float32 {
	v := make([]float32, d.dim)
	for _, tok := range tokenize(text) {
		// Whole-word feature (weighted higher so lexical/semantic overlap
		// dominates the subword signal).
		addFeature(v, "w:"+tok, 2.0)
		// Character-trigram features give partial credit for morphological
		// variants (pool/pooled, configuration/configure).
		for _, g := range trigrams(tok) {
			addFeature(v, "g:"+g, 1.0)
		}
	}
	l2normalize(v)
	return v
}

func tokenize(text string) []string {
	return strings.FieldsFunc(strings.ToLower(text), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsNumber(r)
	})
}

func trigrams(tok string) []string {
	r := []rune("^" + tok + "$")
	if len(r) < 3 {
		return []string{string(r)}
	}
	out := make([]string, 0, len(r)-2)
	for i := 0; i+3 <= len(r); i++ {
		out = append(out, string(r[i:i+3]))
	}
	return out
}

// addFeature applies signed feature hashing: each feature deterministically
// maps to one bucket and one sign, so a shared feature always accumulates
// coherently across texts while hash collisions stay unbiased.
func addFeature(v []float32, feat string, weight float32) {
	h := fnv.New32a()
	_, _ = h.Write([]byte(feat))
	sum := h.Sum32()
	idx := int(sum % uint32(len(v)))
	if sum&0x80000000 != 0 {
		weight = -weight
	}
	v[idx] += weight
}

func l2normalize(v []float32) {
	var s float64
	for _, x := range v {
		s += float64(x) * float64(x)
	}
	if s == 0 {
		return
	}
	inv := float32(1.0 / math.Sqrt(s))
	for i := range v {
		v[i] *= inv
	}
}

// Cosine returns the cosine similarity of two equal-length vectors, in
// [-1, 1]. Mismatched lengths or a zero vector yield 0.
func Cosine(a, b []float32) float32 {
	if len(a) != len(b) {
		return 0
	}
	var dot, na, nb float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
		na += float64(a[i]) * float64(a[i])
		nb += float64(b[i]) * float64(b[i])
	}
	if na == 0 || nb == 0 {
		return 0
	}
	return float32(dot / (math.Sqrt(na) * math.Sqrt(nb)))
}
