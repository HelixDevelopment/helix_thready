package assetservice

import (
	"bytes"
	"crypto/rand"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func mustKey(t *testing.T) []byte {
	t.Helper()
	k := make([]byte, 32)
	if _, err := rand.Read(k); err != nil {
		t.Fatalf("rand key: %v", err)
	}
	return k
}

func TestEncryptedRoundTrip(t *testing.T) {
	cs := newStore(t)
	key := mustKey(t)
	es, err := NewEncryptedStore(cs, key)
	if err != nil {
		t.Fatalf("NewEncryptedStore: %v", err)
	}
	plain := []byte("sensitive: credit-card scan bytes that must be encrypted at rest")

	id, size, err := es.Put(bytes.NewReader(plain))
	if err != nil {
		t.Fatalf("Put: %v", err)
	}
	if id != sha256hex(plain) {
		t.Fatalf("id = %s, want plaintext sha256 %s", id, sha256hex(plain))
	}
	if size != int64(len(plain)) {
		t.Fatalf("size = %d, want %d", size, len(plain))
	}

	rc, err := es.Get(id)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	defer rc.Close()
	got, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if !bytes.Equal(got, plain) {
		t.Fatalf("decrypt mismatch: got %q want %q", got, plain)
	}
}

// TestEncryptedAtRest proves the plaintext never appears on disk (only
// ciphertext blobs are written).
func TestEncryptedAtRest(t *testing.T) {
	cs := newStore(t)
	es, err := NewEncryptedStore(cs, mustKey(t))
	if err != nil {
		t.Fatalf("NewEncryptedStore: %v", err)
	}
	plain := []byte("PLAINTEXT-MARKER-should-never-touch-disk")
	if _, _, err := es.Put(bytes.NewReader(plain)); err != nil {
		t.Fatalf("Put: %v", err)
	}
	err = filepath.WalkDir(cs.Root(), func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || strings.Contains(p, encIndexDir) {
			return nil
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		if bytes.Contains(b, plain) {
			t.Fatalf("plaintext found on disk at %s — not encrypted at rest", p)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
}

// TestEncryptedWrongKeyRejected is the core security assertion: a store opened
// with the WRONG key must fail authentication (ErrDecrypt) on Get, never return
// silent garbage; the RIGHT key still decrypts.
func TestEncryptedWrongKeyRejected(t *testing.T) {
	cs := newStore(t)
	rightKey := mustKey(t)
	wrongKey := mustKey(t)

	es, err := NewEncryptedStore(cs, rightKey)
	if err != nil {
		t.Fatalf("NewEncryptedStore(right): %v", err)
	}
	plain := []byte("only the right key may reveal this")
	id, _, err := es.Put(bytes.NewReader(plain))
	if err != nil {
		t.Fatalf("Put: %v", err)
	}

	// A fresh store over the SAME content store but the WRONG key. It loads the
	// durable index (so it locates the ciphertext) and must reject it.
	esWrong, err := NewEncryptedStore(cs, wrongKey)
	if err != nil {
		t.Fatalf("NewEncryptedStore(wrong): %v", err)
	}
	rc, err := esWrong.Get(id)
	if !errors.Is(err, ErrDecrypt) {
		if rc != nil {
			rc.Close()
		}
		t.Fatalf("Get(wrong key): err = %v, want ErrDecrypt", err)
	}
	if rc != nil {
		t.Fatalf("Get(wrong key) returned a reader; must be nil (no silent garbage)")
	}

	// The right key, freshly reopened, still works.
	esRight, err := NewEncryptedStore(cs, rightKey)
	if err != nil {
		t.Fatalf("NewEncryptedStore(right reopen): %v", err)
	}
	rc2, err := esRight.Get(id)
	if err != nil {
		t.Fatalf("Get(right key): %v", err)
	}
	defer rc2.Close()
	got, err := io.ReadAll(rc2)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if !bytes.Equal(got, plain) {
		t.Fatalf("right-key decrypt mismatch: got %q want %q", got, plain)
	}
}

func TestEncryptedDedup(t *testing.T) {
	cs := newStore(t)
	es, err := NewEncryptedStore(cs, mustKey(t))
	if err != nil {
		t.Fatalf("NewEncryptedStore: %v", err)
	}
	plain := []byte("dedup by plaintext content id even though nonces differ")
	id1, _, err := es.Put(bytes.NewReader(plain))
	if err != nil {
		t.Fatalf("Put#1: %v", err)
	}
	id2, _, err := es.Put(bytes.NewReader(plain))
	if err != nil {
		t.Fatalf("Put#2: %v", err)
	}
	if id1 != id2 {
		t.Fatalf("dedup: ids differ %s vs %s", id1, id2)
	}
}

func TestEncryptedBadKey(t *testing.T) {
	cs := newStore(t)
	if _, err := NewEncryptedStore(cs, make([]byte, 16)); !errors.Is(err, ErrBadKey) {
		t.Fatalf("NewEncryptedStore(16-byte key): err = %v, want ErrBadKey", err)
	}
}

// TestEncryptedTamperedCiphertextRejected corrupts the on-disk ciphertext and
// asserts Get fails (either ciphertext integrity or GCM auth) — never garbage.
func TestEncryptedTamperedCiphertextRejected(t *testing.T) {
	cs := newStore(t)
	key := mustKey(t)
	es, err := NewEncryptedStore(cs, key)
	if err != nil {
		t.Fatalf("NewEncryptedStore: %v", err)
	}
	plain := []byte("tamper the ciphertext blob")
	id, _, err := es.Put(bytes.NewReader(plain))
	if err != nil {
		t.Fatalf("Put: %v", err)
	}
	cid := es.index[id] // white-box: locate the ciphertext blob
	if err := os.WriteFile(cs.blobPath(cid), []byte("garbage garbage garbage garbage garbage!"), 0o644); err != nil {
		t.Fatalf("corrupt: %v", err)
	}
	if _, err := es.Get(id); err == nil {
		t.Fatalf("Get after ciphertext tamper: err = nil, want a failure")
	}
}
