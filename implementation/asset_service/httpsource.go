package assetservice

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"
)

// HTTPSource is a real [FileSource] that fetches bytes over HTTP(S). It is the
// remote-fetch seam of the Asset Service (the Download Manager owns richer
// job/resume/segment semantics; this source is the simple synchronous fetch).
//
// OpenSeekable buffers the full response into memory and returns a seekable
// reader, because HTTP responses are not natively seekable — this makes remote
// content usable with http.ServeContent / Range serving after it lands.
type HTTPSource struct {
	client *http.Client
}

// NewHTTPSource returns an HTTPSource. If client is nil a default client with a
// 30s timeout is used.
func NewHTTPSource(client *http.Client) *HTTPSource {
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	return &HTTPSource{client: client}
}

// Scheme returns "http" (the source also handles https URLs).
func (*HTTPSource) Scheme() string { return "http" }

// Open issues a GET and returns the response body as a streaming reader.
func (s *HTTPSource) Open(ctx context.Context, ref string) (io.ReadCloser, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ref, nil)
	if err != nil {
		return nil, fmt.Errorf("assetservice: http request: %w", err)
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("assetservice: http get: %w", err)
	}
	if resp.StatusCode == http.StatusNotFound {
		_ = resp.Body.Close()
		return nil, ErrNotFound
	}
	if resp.StatusCode != http.StatusOK {
		_ = resp.Body.Close()
		return nil, fmt.Errorf("assetservice: http get %s: status %d", ref, resp.StatusCode)
	}
	return resp.Body, nil
}

// OpenSeekable fetches the full body into memory and returns a seekable reader.
func (s *HTTPSource) OpenSeekable(ctx context.Context, ref string) (io.ReadSeekCloser, error) {
	body, err := s.Open(ctx, ref)
	if err != nil {
		return nil, err
	}
	defer body.Close()
	buf, err := io.ReadAll(body)
	if err != nil {
		return nil, fmt.Errorf("assetservice: http read body: %w", err)
	}
	return newBytesReadSeekCloser(buf), nil
}

// Stat issues a HEAD (falling back to GET when HEAD is unsupported) to report
// size and content type without necessarily downloading the body.
func (s *HTTPSource) Stat(ctx context.Context, ref string) (FileInfo, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, ref, nil)
	if err != nil {
		return FileInfo{}, fmt.Errorf("assetservice: http head request: %w", err)
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return FileInfo{}, fmt.Errorf("assetservice: http head: %w", err)
	}
	_ = resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return FileInfo{}, ErrNotFound
	}
	// Some servers reject HEAD (405); fall back to a GET to learn the metadata.
	if resp.StatusCode == http.StatusMethodNotAllowed {
		body, gerr := s.Open(ctx, ref)
		if gerr != nil {
			return FileInfo{}, gerr
		}
		n, cerr := io.Copy(io.Discard, body)
		_ = body.Close()
		if cerr != nil {
			return FileInfo{}, cerr
		}
		return FileInfo{Name: ref, Size: n}, nil
	}
	if resp.StatusCode != http.StatusOK {
		return FileInfo{}, fmt.Errorf("assetservice: http head %s: status %d", ref, resp.StatusCode)
	}
	return FileInfo{
		Name:        ref,
		Size:        resp.ContentLength,
		ContentType: resp.Header.Get("Content-Type"),
	}, nil
}
