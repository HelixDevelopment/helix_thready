package semsearch

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"
)

// OpenAICompatEmbedder is a REAL net/http client for an OpenAI-compatible
// /v1/embeddings endpoint. This is the production wiring point for llama.cpp /
// HelixLLM: construct it with BaseURL pointed at HelixLLM's server root and
// HELIX_EMBEDDING_PROVIDER=llama (a real embedding GGUF) on the server side.
//
// HONEST SCOPE NOTE: this client is implemented and compiles, but it is NOT
// invoked by the test suite because there is no live embeddings server in this
// environment. The [DeterministicEmbedder] is what proves the cosine/ranking
// pipeline for real in tests. See EVIDENCE.md.
//
// It POSTs to {BaseURL}/v1/embeddings and decodes the OpenAI response shape
// {data:[{embedding,index}]}, sorting by index per the contract (consumers
// must not rely on server ordering).
type OpenAICompatEmbedder struct {
	BaseURL string       // server root, e.g. "http://127.0.0.1:8080" (HelixLLM)
	APIKey  string       // sent as "Authorization: Bearer ..."; "local" for llama.cpp
	Model   string       // e.g. "voyage-code-3"
	Client  *http.Client // defaults to a 30s-timeout client
}

// NewOpenAICompatEmbedder builds a client with a sane default HTTP timeout.
func NewOpenAICompatEmbedder(baseURL, apiKey, model string) *OpenAICompatEmbedder {
	return &OpenAICompatEmbedder{
		BaseURL: strings.TrimRight(baseURL, "/"),
		APIKey:  apiKey,
		Model:   model,
		Client:  &http.Client{Timeout: 30 * time.Second},
	}
}

type embeddingsRequest struct {
	Model string   `json:"model"`
	Input []string `json:"input"`
}

type embeddingsResponse struct {
	Data []struct {
		Embedding []float32 `json:"embedding"`
		Index     int       `json:"index"`
	} `json:"data"`
}

// Embed implements [Embedder] against the OpenAI-compatible endpoint.
func (o *OpenAICompatEmbedder) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	body, err := json.Marshal(embeddingsRequest{Model: o.Model, Input: texts})
	if err != nil {
		return nil, fmt.Errorf("semsearch: marshal embeddings request: %w", err)
	}
	url := o.BaseURL + "/v1/embeddings"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("semsearch: build embeddings request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if o.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+o.APIKey)
	}
	client := o.Client
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("semsearch: embeddings request to %s: %w", url, err)
	}
	defer func() { _ = resp.Body.Close() }()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("semsearch: read embeddings response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("semsearch: embeddings status %d: %s", resp.StatusCode, strings.TrimSpace(string(data)))
	}
	var er embeddingsResponse
	if err := json.Unmarshal(data, &er); err != nil {
		return nil, fmt.Errorf("semsearch: decode embeddings response: %w", err)
	}
	// OpenAI contract: results may arrive out of order; sort by index.
	sort.Slice(er.Data, func(i, j int) bool { return er.Data[i].Index < er.Data[j].Index })
	out := make([][]float32, len(er.Data))
	for i := range er.Data {
		out[i] = er.Data[i].Embedding
	}
	if len(out) != len(texts) {
		return nil, fmt.Errorf("semsearch: expected %d embeddings, got %d", len(texts), len(out))
	}
	return out, nil
}
