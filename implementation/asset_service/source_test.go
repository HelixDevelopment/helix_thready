package assetservice

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestLocalSourceOpenStatSeekable(t *testing.T) {
	dir := t.TempDir()
	content := []byte("local source bytes for open/stat/seek")
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), content, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	src, err := NewLocalSource(dir)
	if err != nil {
		t.Fatalf("NewLocalSource: %v", err)
	}
	ctx := context.Background()

	rc, err := src.Open(ctx, "a.txt")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	got, _ := io.ReadAll(rc)
	rc.Close()
	if string(got) != string(content) {
		t.Fatalf("Open bytes = %q, want %q", got, content)
	}

	info, err := src.Stat(ctx, "a.txt")
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if info.Size != int64(len(content)) {
		t.Fatalf("Stat size = %d, want %d", info.Size, len(content))
	}
	if info.ContentType != "text/plain; charset=utf-8" {
		t.Fatalf("Stat content-type = %q", info.ContentType)
	}

	rs, err := src.OpenSeekable(ctx, "a.txt")
	if err != nil {
		t.Fatalf("OpenSeekable: %v", err)
	}
	defer rs.Close()
	if _, err := rs.Seek(6, io.SeekStart); err != nil {
		t.Fatalf("Seek: %v", err)
	}
	rest, _ := io.ReadAll(rs)
	if string(rest) != string(content[6:]) {
		t.Fatalf("seeked read = %q, want %q", rest, content[6:])
	}
}

func TestLocalSourceTraversalRefused(t *testing.T) {
	dir := t.TempDir()
	// Put a secret one level above the source base.
	if err := os.WriteFile(filepath.Join(dir, "secret.txt"), []byte("secret"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	base := filepath.Join(dir, "public")
	if err := os.MkdirAll(base, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	src, err := NewLocalSource(base)
	if err != nil {
		t.Fatalf("NewLocalSource: %v", err)
	}
	if _, err := src.Open(context.Background(), "../secret.txt"); err == nil {
		t.Fatalf("Open traversal: expected refusal, got nil")
	}
}

func TestLocalSourceNotFound(t *testing.T) {
	src, err := NewLocalSource(t.TempDir())
	if err != nil {
		t.Fatalf("NewLocalSource: %v", err)
	}
	if _, err := src.Open(context.Background(), "missing.txt"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Open missing: err = %v, want ErrNotFound", err)
	}
}

func TestStubSourcesNotImplemented(t *testing.T) {
	ctx := context.Background()
	for _, scheme := range []string{"ftp", "smb", "nfs", "webdav"} {
		src := NewStubSource(scheme)
		if src.Scheme() != scheme {
			t.Fatalf("Scheme = %q, want %q", src.Scheme(), scheme)
		}
		if _, err := src.Open(ctx, "x"); !errors.Is(err, ErrNotImplemented) {
			t.Fatalf("%s Open: err = %v, want ErrNotImplemented", scheme, err)
		}
		if _, err := src.OpenSeekable(ctx, "x"); !errors.Is(err, ErrNotImplemented) {
			t.Fatalf("%s OpenSeekable: err = %v, want ErrNotImplemented", scheme, err)
		}
		if _, err := src.Stat(ctx, "x"); !errors.Is(err, ErrNotImplemented) {
			t.Fatalf("%s Stat: err = %v, want ErrNotImplemented", scheme, err)
		}
	}
}
