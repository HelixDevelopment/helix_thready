package assetservice

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
)

// contentIDPattern matches a lowercase hex SHA-256 (the canonical content id).
var contentIDPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

// ValidContentID reports whether id is a well-formed content id (64 lowercase
// hex characters, i.e. a hex SHA-256).
func ValidContentID(id string) bool { return contentIDPattern.MatchString(id) }

// ContentStore is content-addressed byte storage. The id of a blob is the
// lowercase hex SHA-256 of its bytes, so identical content is stored once
// (dedup) and every read can be integrity-checked against its own id.
//
// On-disk layout under root: <root>/<id[0:2]>/<id[2:4]>/<id>. Writes are atomic
// (temp file + rename) so a crash mid-write never leaves a half-written blob at
// a content path.
type ContentStore struct {
	root string
}

// NewContentStore opens (creating if necessary) a content store rooted at root.
func NewContentStore(root string) (*ContentStore, error) {
	if root == "" {
		return nil, fmt.Errorf("assetservice: content store root must not be empty")
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, fmt.Errorf("assetservice: mkdir content store root: %w", err)
	}
	return &ContentStore{root: root}, nil
}

// Root returns the on-disk root directory of the store.
func (s *ContentStore) Root() string { return s.root }

// blobPath returns the on-disk path a blob with the given content id lives at.
// It is unexported on purpose: clients never receive a filesystem path.
func (s *ContentStore) blobPath(id string) string {
	return filepath.Join(s.root, id[0:2], id[2:4], id)
}

// Put streams r into the store, returning the content id (hex SHA-256) and the
// number of bytes stored. Storing the same bytes twice is a no-op that returns
// the same id (content-addressed dedup).
func (s *ContentStore) Put(r io.Reader) (id string, size int64, err error) {
	tmp, err := os.CreateTemp(s.root, ".put-*.tmp")
	if err != nil {
		return "", 0, fmt.Errorf("assetservice: create temp: %w", err)
	}
	tmpName := tmp.Name()
	// Best-effort cleanup: removing an already-renamed temp is harmless.
	defer func() { _ = os.Remove(tmpName) }()

	h := sha256.New()
	n, err := io.Copy(io.MultiWriter(tmp, h), r)
	if err != nil {
		_ = tmp.Close()
		return "", 0, fmt.Errorf("assetservice: write temp: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return "", 0, fmt.Errorf("assetservice: close temp: %w", err)
	}

	id = hex.EncodeToString(h.Sum(nil))
	dst := s.blobPath(id)

	if _, statErr := os.Stat(dst); statErr == nil {
		// Identical content already present: dedup, drop the temp.
		return id, n, nil
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return "", 0, fmt.Errorf("assetservice: mkdir shard: %w", err)
	}
	if err := os.Rename(tmpName, dst); err != nil {
		return "", 0, fmt.Errorf("assetservice: commit blob: %w", err)
	}
	return id, n, nil
}

// Has reports whether a blob with the given content id exists.
func (s *ContentStore) Has(id string) (bool, error) {
	if !ValidContentID(id) {
		return false, ErrBadContentID
	}
	_, err := os.Stat(s.blobPath(id))
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

// ContentInfo describes a stored blob.
type ContentInfo struct {
	ID   string
	Size int64
}

// Stat returns metadata for a stored blob without reading its bytes.
func (s *ContentStore) Stat(id string) (ContentInfo, error) {
	if !ValidContentID(id) {
		return ContentInfo{}, ErrBadContentID
	}
	fi, err := os.Stat(s.blobPath(id))
	if os.IsNotExist(err) {
		return ContentInfo{}, ErrNotFound
	}
	if err != nil {
		return ContentInfo{}, err
	}
	return ContentInfo{ID: id, Size: fi.Size()}, nil
}

// Get returns the bytes stored under id as an io.ReadSeekCloser, having first
// verified that they still hash to id. If the stored bytes have been tampered
// with or corrupted, Get returns ErrIntegrity and NO reader — a mismatch is
// never silently served.
//
// The blob is read fully and verified up front so the returned seeker composes
// safely with http.ServeContent (which seeks around to satisfy Range requests);
// a streaming hash cannot be completed when only a sub-range is read.
func (s *ContentStore) Get(id string) (io.ReadSeekCloser, error) {
	if !ValidContentID(id) {
		return nil, ErrBadContentID
	}
	f, err := os.Open(s.blobPath(id))
	if os.IsNotExist(err) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()

	buf, err := io.ReadAll(f)
	if err != nil {
		return nil, fmt.Errorf("assetservice: read blob: %w", err)
	}
	sum := sha256.Sum256(buf)
	if got := hex.EncodeToString(sum[:]); got != id {
		return nil, fmt.Errorf("%w: stored=%s id=%s", ErrIntegrity, got, id)
	}
	return newBytesReadSeekCloser(buf), nil
}

// bytesReadSeekCloser adapts an in-memory buffer to io.ReadSeekCloser.
type bytesReadSeekCloser struct {
	*bytes.Reader
}

func newBytesReadSeekCloser(b []byte) *bytesReadSeekCloser {
	return &bytesReadSeekCloser{Reader: bytes.NewReader(b)}
}

// Close satisfies io.Closer; an in-memory reader has nothing to release.
func (*bytesReadSeekCloser) Close() error { return nil }
