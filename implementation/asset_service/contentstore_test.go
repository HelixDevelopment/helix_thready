package assetservice

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestContentStorePutIDIsSHA256(t *testing.T) {
	cs := newStore(t)
	content := []byte("helix thready asset service — content addressed bytes")

	id, size, err := cs.Put(bytes.NewReader(content))
	if err != nil {
		t.Fatalf("Put: %v", err)
	}
	if want := sha256hex(content); id != want {
		t.Fatalf("id = %s, want sha256 %s", id, want)
	}
	if size != int64(len(content)) {
		t.Fatalf("size = %d, want %d", size, len(content))
	}
	if !ValidContentID(id) {
		t.Fatalf("id %q is not a valid content id", id)
	}
}

func TestContentStoreGetRoundTripVerified(t *testing.T) {
	cs := newStore(t)
	content := []byte("round trip must be byte identical and integrity verified")

	id, _, err := cs.Put(bytes.NewReader(content))
	if err != nil {
		t.Fatalf("Put: %v", err)
	}
	rc, err := cs.Get(id)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	defer rc.Close()
	got, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if !bytes.Equal(got, content) {
		t.Fatalf("round-trip mismatch: got %q want %q", got, content)
	}
}

func TestContentStoreDedup(t *testing.T) {
	cs := newStore(t)
	content := []byte("identical bytes are stored exactly once")

	id1, _, err := cs.Put(bytes.NewReader(content))
	if err != nil {
		t.Fatalf("Put#1: %v", err)
	}
	id2, _, err := cs.Put(bytes.NewReader(content))
	if err != nil {
		t.Fatalf("Put#2: %v", err)
	}
	if id1 != id2 {
		t.Fatalf("dedup: ids differ %s vs %s", id1, id2)
	}
	// Exactly one physical blob under the store (ignore temp/index dirs).
	var blobs int
	err = filepath.WalkDir(cs.Root(), func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || strings.Contains(p, encIndexDir) || strings.HasSuffix(p, ".tmp") {
			return nil
		}
		blobs++
		return nil
	})
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
	if blobs != 1 {
		t.Fatalf("expected 1 physical blob after dedup, found %d", blobs)
	}
}

func TestContentStoreTamperDetected(t *testing.T) {
	cs := newStore(t)
	content := []byte("bytes that will be corrupted on disk")

	id, _, err := cs.Put(bytes.NewReader(content))
	if err != nil {
		t.Fatalf("Put: %v", err)
	}
	// Corrupt the stored blob in place (flip its bytes) — simulating bit-rot
	// or tampering. The id no longer matches the stored content.
	if err := os.WriteFile(cs.blobPath(id), []byte("tampered content of a different length!!"), 0o644); err != nil {
		t.Fatalf("corrupt: %v", err)
	}
	rc, err := cs.Get(id)
	if !errors.Is(err, ErrIntegrity) {
		if rc != nil {
			rc.Close()
		}
		t.Fatalf("Get after tamper: err = %v, want ErrIntegrity", err)
	}
	if rc != nil {
		t.Fatalf("Get returned a reader on integrity failure; must return nil")
	}
}

func TestContentStoreGetUnknown(t *testing.T) {
	cs := newStore(t)
	unknown := sha256hex([]byte("never stored"))
	if _, err := cs.Get(unknown); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get unknown: err = %v, want ErrNotFound", err)
	}
}

func TestContentStoreBadID(t *testing.T) {
	cs := newStore(t)
	if _, err := cs.Get("not-a-hash"); !errors.Is(err, ErrBadContentID) {
		t.Fatalf("Get bad id: err = %v, want ErrBadContentID", err)
	}
	if _, err := cs.Stat("XYZ"); !errors.Is(err, ErrBadContentID) {
		t.Fatalf("Stat bad id: err = %v, want ErrBadContentID", err)
	}
}

func TestContentStoreStat(t *testing.T) {
	cs := newStore(t)
	content := []byte("stat me")
	id, _, err := cs.Put(bytes.NewReader(content))
	if err != nil {
		t.Fatalf("Put: %v", err)
	}
	info, err := cs.Stat(id)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if info.ID != id || info.Size != int64(len(content)) {
		t.Fatalf("Stat = %+v, want id=%s size=%d", info, id, len(content))
	}
}
