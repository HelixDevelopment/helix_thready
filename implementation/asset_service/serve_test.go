package assetservice

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// principalFromHeader is a test PrincipalFunc: it reads X-Account / X-Roles
// headers to build a principal (stand-in for real bearer-token verification).
func principalFromHeader(r *http.Request) (Principal, error) {
	acct := r.Header.Get("X-Account")
	p := Principal{Subject: r.Header.Get("X-Subject"), AccountID: acct}
	if r.Header.Get("X-Roles") != "" {
		p.Roles = []string{r.Header.Get("X-Roles")}
	}
	return p, nil
}

// buildServer wires a full asset-serving HTTP stack around content bytes and
// returns the server plus the asset id.
func buildServer(t *testing.T, content []byte) (*httptest.Server, string) {
	t.Helper()
	cs := newStore(t)
	cid, size, err := cs.Put(bytes.NewReader(content))
	if err != nil {
		t.Fatalf("Put: %v", err)
	}
	ix := NewAssetIndex()
	ix.Put(Asset{
		ID:           "asset-1",
		SHA256:       cid,
		Size:         size,
		ContentType:  "application/octet-stream",
		OriginalName: "file.bin",
		AccountID:    "acct-A",
	})
	r := NewResolver(ix, cs, SameAccountAuthorizer{})
	h := NewHandler(r, principalFromHeader)
	mux := http.NewServeMux()
	mux.Handle("/v1/assets/", h)
	return httptest.NewServer(mux), "asset-1"
}

func authGet(t *testing.T, srv *httptest.Server, path string, rangeHdr string) *http.Response {
	t.Helper()
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL+path, nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("X-Account", "acct-A")
	req.Header.Set("X-Roles", "reader")
	if rangeHdr != "" {
		req.Header.Set("Range", rangeHdr)
	}
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	return resp
}

func TestServeFullContent(t *testing.T) {
	content := []byte("the quick brown fox jumps over the lazy dog, repeatedly, for bytes")
	srv, id := buildServer(t, content)
	defer srv.Close()

	resp := authGet(t, srv, "/v1/assets/"+id, "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if !bytes.Equal(body, content) {
		t.Fatalf("body mismatch")
	}
	if resp.Header.Get("Accept-Ranges") != "bytes" {
		t.Fatalf("Accept-Ranges = %q, want bytes", resp.Header.Get("Accept-Ranges"))
	}
}

// TestServeRange is the byte-range assertion: a Range header must yield 206
// Partial Content with the correct sub-slice and a Content-Range header, via
// http.ServeContent over the seekable resolved reader.
func TestServeRange(t *testing.T) {
	content := []byte("0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ-range-serving-proof-bytes")
	srv, id := buildServer(t, content)
	defer srv.Close()

	resp := authGet(t, srv, "/v1/assets/"+id, "bytes=10-19")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusPartialContent {
		t.Fatalf("status = %d, want 206", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	want := content[10:20]
	if !bytes.Equal(body, want) {
		t.Fatalf("range body = %q, want %q", body, want)
	}
	if cr := resp.Header.Get("Content-Range"); cr == "" {
		t.Fatalf("missing Content-Range header")
	}
	if cl := resp.ContentLength; cl != int64(len(want)) {
		t.Fatalf("Content-Length = %d, want %d", cl, len(want))
	}
}

func TestServeForbidden(t *testing.T) {
	srv, id := buildServer(t, []byte("private bytes"))
	defer srv.Close()

	// Principal from a different account -> 403.
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL+"/v1/assets/"+id, nil)
	req.Header.Set("X-Account", "acct-OTHER")
	req.Header.Set("X-Roles", "reader")
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", resp.StatusCode)
	}
}

func TestServeNotFound(t *testing.T) {
	srv, _ := buildServer(t, []byte("bytes"))
	defer srv.Close()

	resp := authGet(t, srv, "/v1/assets/does-not-exist", "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", resp.StatusCode)
	}
}

func TestServeRenditionWeb(t *testing.T) {
	cs := newStore(t)
	raw := []byte("raw original bytes here")
	web := []byte("web optimized bytes")
	rawID, _, err := cs.Put(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("Put raw: %v", err)
	}
	webID, sz, err := cs.Put(bytes.NewReader(web))
	if err != nil {
		t.Fatalf("Put web: %v", err)
	}
	webName := WebRenditionName("clip.mp4")
	ix := NewAssetIndex()
	ix.Put(Asset{
		ID:           "asset-1",
		SHA256:       rawID,
		OriginalName: "clip.mp4",
		AccountID:    "acct-A",
		Renditions:   map[string]Rendition{webName: {Name: webName, ContentID: webID, ContentType: "video/mp4", Size: sz}},
	})
	r := NewResolver(ix, cs, SameAccountAuthorizer{})
	mux := http.NewServeMux()
	mux.Handle("/v1/assets/", NewHandler(r, principalFromHeader))
	srv := httptest.NewServer(mux)
	defer srv.Close()

	resp := authGet(t, srv, "/v1/assets/asset-1/web", "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if !bytes.Equal(body, web) {
		t.Fatalf("web rendition body = %q, want %q", body, web)
	}
}

// TestServeNoDirectPath asserts the client-facing surface never exposes a
// filesystem path: the URL carries only the opaque asset id, and the store's
// blob path is unexported (not reachable through any public API).
func TestServeNoDirectPath(t *testing.T) {
	content := []byte("no direct path bytes")
	srv, id := buildServer(t, content)
	defer srv.Close()

	// The reference the client uses is the asset id, which is not a path.
	if bytes.ContainsAny([]byte(id), "/\\") {
		t.Fatalf("asset id %q looks like a path", id)
	}
	resp := authGet(t, srv, "/v1/assets/"+id, "")
	defer resp.Body.Close()
	// No header leaks an on-disk location.
	for _, h := range []string{"X-Accel-Redirect", "X-Sendfile", "Location"} {
		if v := resp.Header.Get(h); v != "" {
			t.Fatalf("response leaked path via %s: %q", h, v)
		}
	}
}
