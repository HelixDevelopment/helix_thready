package downloadmanager

import (
	"bytes"
	"context"
	"errors"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func mustURL(t *testing.T, raw string) *url.URL {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("parse url %q: %v", raw, err)
	}
	return u
}

// Test 1: full download, sha256 matches the source bytes exactly.
func TestFullDownloadSHA256(t *testing.T) {
	data := randomBytes(t, 200*1024)
	want := sha256Hex(data)

	srv := httptest.NewServer(&rangeServer{data: data, etag: `"full"`})
	defer srv.Close()

	dest := filepath.Join(t.TempDir(), "full.bin")
	res, err := NewHTTPFetcher().Fetch(context.Background(), FetchRequest{
		URL:      mustURL(t, srv.URL+"/full.bin"),
		DestPath: dest,
		Segments: 1,
	})
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if res.SHA256 != want {
		t.Errorf("sha256 = %s, want %s", res.SHA256, want)
	}
	if res.BytesWritten != int64(len(data)) {
		t.Errorf("bytes written = %d, want %d", res.BytesWritten, len(data))
	}
	got, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("read dest: %v", err)
	}
	if !bytes.Equal(got, data) {
		t.Fatal("downloaded bytes differ from source")
	}
	if sha256Hex(got) != want {
		t.Fatal("on-disk sha256 differs from source")
	}
}

// Test 2: segmented / ranged parallel download reassembles byte-identically.
func TestSegmentedDownloadByteIdentical(t *testing.T) {
	data := randomBytes(t, 300*1024)
	want := sha256Hex(data)

	srv := httptest.NewServer(&rangeServer{data: data, etag: `"seg"`, chunk: 16 * 1024})
	defer srv.Close()

	dest := filepath.Join(t.TempDir(), "seg.bin")
	res, err := NewHTTPFetcher().Fetch(context.Background(), FetchRequest{
		URL:            mustURL(t, srv.URL+"/seg.bin"),
		DestPath:       dest,
		Segments:       4,
		ExpectedSHA256: want,
	})
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if res.SHA256 != want {
		t.Errorf("sha256 = %s, want %s", res.SHA256, want)
	}
	if res.BytesWritten != int64(len(data)) {
		t.Errorf("bytes written = %d, want %d", res.BytesWritten, len(data))
	}
	got, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("read dest: %v", err)
	}
	if !bytes.Equal(got, data) {
		t.Fatal("reassembled bytes differ from source")
	}
}

// Test 3: resume after a simulated mid-transfer interruption completes correctly.
func TestResumeAfterInterruption(t *testing.T) {
	data := randomBytes(t, 512*1024)
	want := sha256Hex(data)

	srv := httptest.NewServer(&rangeServer{
		data: data, etag: `"resume"`, delay: time.Millisecond, chunk: 8 * 1024,
	})
	defer srv.Close()

	dest := filepath.Join(t.TempDir(), "resume.bin")
	u := mustURL(t, srv.URL+"/resume.bin")
	f := NewHTTPFetcher()

	// First attempt: cancel once ~25% has been committed.
	ctx, cancel := context.WithCancel(context.Background())
	var once sync.Once
	_, err := f.Fetch(ctx, FetchRequest{
		URL:      u,
		DestPath: dest,
		Segments: 1,
		Progress: func(done, total int64) {
			if total > 0 && done*4 >= total {
				once.Do(cancel)
			}
		},
	})
	cancel()
	if err == nil {
		t.Fatal("expected an interruption error on the first attempt")
	}

	st := loadState(dest + ".dlstate")
	if st == nil {
		t.Fatal("expected persisted resume state after interruption")
	}
	var partial int64
	for _, s := range st.Segments {
		partial += s.Done
	}
	if partial <= 0 || partial >= int64(len(data)) {
		t.Fatalf("expected partial progress, got %d of %d bytes", partial, len(data))
	}
	if _, err := os.Stat(dest); err == nil {
		t.Fatal("destination should not exist until the download completes")
	}

	// Second attempt: resume to completion.
	res, err := f.Fetch(context.Background(), FetchRequest{
		URL:            u,
		DestPath:       dest,
		Segments:       1,
		ExpectedSHA256: want,
	})
	if err != nil {
		t.Fatalf("resume fetch: %v", err)
	}
	if !res.Resumed {
		t.Error("expected Resumed = true on the second attempt")
	}
	if res.SHA256 != want {
		t.Errorf("sha256 = %s, want %s", res.SHA256, want)
	}
	got, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("read dest: %v", err)
	}
	if !bytes.Equal(got, data) {
		t.Fatal("resumed file differs from source")
	}
}

// A checksum mismatch must be reported as a permanent (non-retryable) error.
func TestChecksumMismatchIsPermanent(t *testing.T) {
	data := randomBytes(t, 64*1024)

	srv := httptest.NewServer(&rangeServer{data: data})
	defer srv.Close()

	dest := filepath.Join(t.TempDir(), "bad.bin")
	_, err := NewHTTPFetcher().Fetch(context.Background(), FetchRequest{
		URL:            mustURL(t, srv.URL+"/bad.bin"),
		DestPath:       dest,
		Segments:       2,
		ExpectedSHA256: sha256Hex([]byte("the wrong content")),
	})
	if err == nil {
		t.Fatal("expected a checksum mismatch error")
	}
	if !IsPermanent(err) {
		t.Errorf("checksum mismatch should be permanent, got %v", err)
	}
	if _, statErr := os.Stat(dest); statErr == nil {
		t.Error("destination must not be created on checksum mismatch")
	}
}

// A server that ignores Range headers must still download correctly (single stream).
func TestNoRangeServerFallback(t *testing.T) {
	data := randomBytes(t, 128*1024)
	want := sha256Hex(data)

	srv := httptest.NewServer(&rangeServer{data: data, noRanges: true})
	defer srv.Close()

	dest := filepath.Join(t.TempDir(), "norange.bin")
	res, err := NewHTTPFetcher().Fetch(context.Background(), FetchRequest{
		URL:      mustURL(t, srv.URL+"/norange.bin"),
		DestPath: dest,
		Segments: 4, // requested, but ignored because ranges are unsupported
	})
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if res.SHA256 != want {
		t.Errorf("sha256 = %s, want %s", res.SHA256, want)
	}
	got, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("read dest: %v", err)
	}
	if !bytes.Equal(got, data) {
		t.Fatal("bytes differ from source")
	}
}

// The FTP/SMB/NFS/WebDav stubs must fail honestly with ErrNotImplemented.
func TestStubFetcherNotImplemented(t *testing.T) {
	f := NewStubFetcher("ftp", "smb", "nfs", "webdav")
	if got := len(f.Schemes()); got != 4 {
		t.Errorf("Schemes len = %d, want 4", got)
	}
	_, err := f.Fetch(context.Background(), FetchRequest{
		URL:      mustURL(t, "ftp://example.invalid/file"),
		DestPath: filepath.Join(t.TempDir(), "x"),
	})
	if !errors.Is(err, ErrNotImplemented) {
		t.Fatalf("err = %v, want ErrNotImplemented", err)
	}
	if !IsPermanent(err) {
		t.Error("ErrNotImplemented must be treated as permanent")
	}
}
