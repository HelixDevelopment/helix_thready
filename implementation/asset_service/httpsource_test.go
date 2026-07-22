package assetservice

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHTTPSourceFetchLiveServer(t *testing.T) {
	content := []byte("bytes fetched from a live httptest server over HTTP")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/asset.bin" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Content-Length", itoa(len(content)))
		_, _ = w.Write(content)
	}))
	defer srv.Close()

	src := NewHTTPSource(srv.Client())
	ctx := context.Background()

	// Open (streaming).
	rc, err := src.Open(ctx, srv.URL+"/asset.bin")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	got, _ := io.ReadAll(rc)
	rc.Close()
	if !bytes.Equal(got, content) {
		t.Fatalf("Open bytes = %q, want %q", got, content)
	}

	// OpenSeekable (buffered, seekable).
	rs, err := src.OpenSeekable(ctx, srv.URL+"/asset.bin")
	if err != nil {
		t.Fatalf("OpenSeekable: %v", err)
	}
	defer rs.Close()
	if _, err := rs.Seek(-4, io.SeekEnd); err != nil {
		t.Fatalf("Seek: %v", err)
	}
	tail, _ := io.ReadAll(rs)
	if !bytes.Equal(tail, content[len(content)-4:]) {
		t.Fatalf("seeked tail = %q, want %q", tail, content[len(content)-4:])
	}

	// Stat.
	info, err := src.Stat(ctx, srv.URL+"/asset.bin")
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if info.Size != int64(len(content)) {
		t.Fatalf("Stat size = %d, want %d", info.Size, len(content))
	}
	if info.ContentType != "application/octet-stream" {
		t.Fatalf("Stat content-type = %q", info.ContentType)
	}
}

// TestHTTPSourceIntoContentStore proves the remote-fetch → content-address flow:
// fetch bytes from a live server and Put them into the ContentStore, verifying
// the id equals sha256 of what the server served.
func TestHTTPSourceIntoContentStore(t *testing.T) {
	content := []byte("remote bytes that become a content-addressed asset")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(content)
	}))
	defer srv.Close()

	src := NewHTTPSource(srv.Client())
	rc, err := src.Open(context.Background(), srv.URL+"/")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer rc.Close()

	cs := newStore(t)
	id, _, err := cs.Put(rc)
	if err != nil {
		t.Fatalf("Put: %v", err)
	}
	if id != sha256hex(content) {
		t.Fatalf("id = %s, want %s", id, sha256hex(content))
	}
}

func TestHTTPSourceNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer srv.Close()
	src := NewHTTPSource(srv.Client())
	if _, err := src.Open(context.Background(), srv.URL+"/nope"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Open 404: err = %v, want ErrNotFound", err)
	}
}

// itoa avoids importing strconv just for the Content-Length header.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}
