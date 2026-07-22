package assetservice

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// FileInfo is the protocol-neutral metadata a [FileSource] reports for a ref.
type FileInfo struct {
	Name        string
	Size        int64
	ModTime     time.Time
	ContentType string // best-effort; may be empty
}

// FileSource is the multi-protocol access seam. A ref is a source-relative
// locator (a path for [LocalSource], a URL for [HTTPSource]). OpenSeekable
// returns an io.ReadSeekCloser so callers can serve HTTP Range requests.
//
// FTP/SMB/NFS/WebDAV implementations are honest stubs ([ErrNotImplemented]) that
// mark the digital.vasic.filesystem reuse seam.
type FileSource interface {
	// Open returns a streaming reader over the ref's bytes.
	Open(ctx context.Context, ref string) (io.ReadCloser, error)
	// Stat returns metadata about the ref without reading its bytes.
	Stat(ctx context.Context, ref string) (FileInfo, error)
	// OpenSeekable returns a seekable reader suitable for http.ServeContent /
	// byte-range serving.
	OpenSeekable(ctx context.Context, ref string) (io.ReadSeekCloser, error)
	// Scheme names the protocol this source speaks (e.g. "local", "http").
	Scheme() string
}

// LocalSource is a real [FileSource] backed by the local filesystem, rooted at
// a base directory. Refs are resolved relative to base and confined to it
// (path-traversal outside base is refused).
type LocalSource struct {
	base string
}

// NewLocalSource returns a LocalSource rooted at base. base is cleaned to an
// absolute path.
func NewLocalSource(base string) (*LocalSource, error) {
	abs, err := filepath.Abs(base)
	if err != nil {
		return nil, fmt.Errorf("assetservice: local source base: %w", err)
	}
	return &LocalSource{base: abs}, nil
}

// Scheme returns "local".
func (*LocalSource) Scheme() string { return "local" }

// resolve joins ref onto base and confirms the result stays within base.
func (s *LocalSource) resolve(ref string) (string, error) {
	clean := filepath.Clean("/" + strings.TrimPrefix(ref, "/"))
	full := filepath.Join(s.base, clean)
	if full != s.base && !strings.HasPrefix(full, s.base+string(os.PathSeparator)) {
		return "", fmt.Errorf("assetservice: ref escapes source base: %q", ref)
	}
	return full, nil
}

// Open opens the ref for streaming reads.
func (s *LocalSource) Open(_ context.Context, ref string) (io.ReadCloser, error) {
	full, err := s.resolve(ref)
	if err != nil {
		return nil, err
	}
	f, err := os.Open(full)
	if os.IsNotExist(err) {
		return nil, ErrNotFound
	}
	return f, err
}

// OpenSeekable opens the ref for seekable reads (*os.File satisfies
// io.ReadSeekCloser natively).
func (s *LocalSource) OpenSeekable(_ context.Context, ref string) (io.ReadSeekCloser, error) {
	full, err := s.resolve(ref)
	if err != nil {
		return nil, err
	}
	f, err := os.Open(full)
	if os.IsNotExist(err) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return f, nil
}

// Stat returns filesystem metadata for the ref, sniffing a content type from
// the file extension.
func (s *LocalSource) Stat(_ context.Context, ref string) (FileInfo, error) {
	full, err := s.resolve(ref)
	if err != nil {
		return FileInfo{}, err
	}
	fi, err := os.Stat(full)
	if os.IsNotExist(err) {
		return FileInfo{}, ErrNotFound
	}
	if err != nil {
		return FileInfo{}, err
	}
	return FileInfo{
		Name:        fi.Name(),
		Size:        fi.Size(),
		ModTime:     fi.ModTime(),
		ContentType: contentTypeByExt(fi.Name()),
	}, nil
}

// stubSource is an honest not-yet-implemented [FileSource]. Every method
// returns [ErrNotImplemented]. These mark the digital.vasic.filesystem reuse
// points for FTP/SMB/NFS/WebDAV — declared, never faked.
type stubSource struct{ scheme string }

// NewStubSource returns a not-implemented source for the given scheme
// (e.g. "ftp", "smb", "nfs", "webdav").
func NewStubSource(scheme string) FileSource { return &stubSource{scheme: scheme} }

func (s *stubSource) Scheme() string { return s.scheme }

func (s *stubSource) Open(context.Context, string) (io.ReadCloser, error) {
	return nil, fmt.Errorf("%w: %s", ErrNotImplemented, s.scheme)
}

func (s *stubSource) OpenSeekable(context.Context, string) (io.ReadSeekCloser, error) {
	return nil, fmt.Errorf("%w: %s", ErrNotImplemented, s.scheme)
}

func (s *stubSource) Stat(context.Context, string) (FileInfo, error) {
	return FileInfo{}, fmt.Errorf("%w: %s", ErrNotImplemented, s.scheme)
}

// contentTypeByExt maps a small set of common asset extensions to MIME types.
// It is intentionally minimal (stdlib mime.TypeByExtension varies by host);
// an empty string means "let the serving layer sniff".
func contentTypeByExt(name string) string {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".mp4", ".m4v":
		return "video/mp4"
	case ".m4a":
		return "audio/mp4"
	case ".mp3":
		return "audio/mpeg"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".txt":
		return "text/plain; charset=utf-8"
	case ".json":
		return "application/json"
	case ".bin":
		return "application/octet-stream"
	default:
		return ""
	}
}
