package assetservice

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
)

// encIndexDir is the sidecar directory (under the inner store root) that maps a
// plaintext content id to the content id of its stored ciphertext blob. It is
// durable so a fresh EncryptedStore over the same root — including one opened
// with a WRONG key — can locate the ciphertext and fail authentication cleanly.
const encIndexDir = ".encindex"

// EncryptedStore wraps a [ContentStore] with AES-256-GCM encryption at rest.
// Assets are addressed by the SHA-256 of their PLAINTEXT (so dedup and asset
// ids are stable regardless of the random nonce), while the bytes on disk are
// ciphertext. Each Put uses a fresh random 12-byte nonce; the plaintext content
// id is bound in as GCM additional data. A wrong key makes Get fail with
// [ErrDecrypt] (GCM authentication) — never silent garbage.
//
// This is the mechanism behind the "specially encrypted directory decrypts only
// inside the Asset Service" rule (architecture §2, §8): point a [Resolver] at an
// EncryptedStore and sensitive assets are only ever handed out decrypted after
// an RBAC check.
type EncryptedStore struct {
	inner *ContentStore
	gcm   cipher.AEAD

	mu    sync.RWMutex
	index map[string]string // plaintextID -> ciphertextID
}

// NewEncryptedStore wraps inner with AES-256-GCM under key, which MUST be
// exactly 32 bytes (AES-256). Any plaintext->ciphertext mappings already on
// disk under the inner root are loaded so previously-stored assets are
// resolvable (this is also how a wrong-key store finds the ciphertext to reject
// it).
func NewEncryptedStore(inner *ContentStore, key []byte) (*EncryptedStore, error) {
	if len(key) != 32 {
		return nil, ErrBadKey
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("assetservice: aes cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("assetservice: gcm: %w", err)
	}
	es := &EncryptedStore{inner: inner, gcm: gcm, index: make(map[string]string)}
	if err := es.loadIndex(); err != nil {
		return nil, err
	}
	return es, nil
}

func (e *EncryptedStore) indexDir() string { return filepath.Join(e.inner.Root(), encIndexDir) }

// loadIndex reads the durable plaintextID->ciphertextID mappings from disk.
func (e *EncryptedStore) loadIndex() error {
	dir := e.indexDir()
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("assetservice: read enc index: %w", err)
	}
	for _, ent := range entries {
		if ent.IsDir() {
			continue
		}
		b, err := os.ReadFile(filepath.Join(dir, ent.Name()))
		if err != nil {
			return fmt.Errorf("assetservice: read enc index entry: %w", err)
		}
		e.index[ent.Name()] = string(bytes.TrimSpace(b))
	}
	return nil
}

// persistMapping durably records plaintextID -> ciphertextID.
func (e *EncryptedStore) persistMapping(plaintextID, ciphertextID string) error {
	dir := e.indexDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("assetservice: mkdir enc index: %w", err)
	}
	tmp, err := os.CreateTemp(dir, ".idx-*.tmp")
	if err != nil {
		return fmt.Errorf("assetservice: temp enc index: %w", err)
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()
	if _, err := tmp.WriteString(ciphertextID); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("assetservice: write enc index: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("assetservice: close enc index: %w", err)
	}
	if err := os.Rename(tmpName, filepath.Join(dir, plaintextID)); err != nil {
		return fmt.Errorf("assetservice: commit enc index: %w", err)
	}
	return nil
}

// Put encrypts the plaintext from r and stores the ciphertext, returning the
// plaintext content id (hex SHA-256) and plaintext size. Identical plaintext is
// deduped (returns the existing id without re-encrypting).
func (e *EncryptedStore) Put(r io.Reader) (id string, size int64, err error) {
	plain, err := io.ReadAll(r)
	if err != nil {
		return "", 0, fmt.Errorf("assetservice: read plaintext: %w", err)
	}
	sum := sha256.Sum256(plain)
	id = hex.EncodeToString(sum[:])

	e.mu.RLock()
	_, exists := e.index[id]
	e.mu.RUnlock()
	if exists {
		return id, int64(len(plain)), nil
	}

	nonce := make([]byte, e.gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", 0, fmt.Errorf("assetservice: nonce: %w", err)
	}
	// Layout: nonce || ciphertext+tag. AAD binds the ciphertext to the
	// plaintext id so a blob cannot be swapped under another id undetected.
	sealed := e.gcm.Seal(nonce, nonce, plain, []byte(id))

	cid, _, err := e.inner.Put(bytes.NewReader(sealed))
	if err != nil {
		return "", 0, err
	}
	if err := e.persistMapping(id, cid); err != nil {
		return "", 0, err
	}
	e.mu.Lock()
	e.index[id] = cid
	e.mu.Unlock()
	return id, int64(len(plain)), nil
}

// Get returns the decrypted plaintext for the plaintext content id as an
// io.ReadSeekCloser. It first verifies the ciphertext blob's own integrity via
// the inner [ContentStore], then GCM-authenticates and decrypts. A wrong key or
// tampered ciphertext yields [ErrDecrypt]; the result is re-checked against id
// for defense in depth ([ErrIntegrity]).
func (e *EncryptedStore) Get(id string) (io.ReadSeekCloser, error) {
	if !ValidContentID(id) {
		return nil, ErrBadContentID
	}
	e.mu.RLock()
	cid, ok := e.index[id]
	e.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("%w: encrypted asset %q", ErrNotFound, id)
	}

	rc, err := e.inner.Get(cid) // verifies ciphertext integrity (sha256)
	if err != nil {
		return nil, err
	}
	sealed, err := io.ReadAll(rc)
	_ = rc.Close()
	if err != nil {
		return nil, fmt.Errorf("assetservice: read ciphertext: %w", err)
	}
	ns := e.gcm.NonceSize()
	if len(sealed) < ns {
		return nil, fmt.Errorf("%w: ciphertext shorter than nonce", ErrDecrypt)
	}
	nonce, body := sealed[:ns], sealed[ns:]
	plain, err := e.gcm.Open(nil, nonce, body, []byte(id))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrDecrypt, err)
	}
	sum := sha256.Sum256(plain)
	if got := hex.EncodeToString(sum[:]); got != id {
		return nil, fmt.Errorf("%w: decrypted=%s id=%s", ErrIntegrity, got, id)
	}
	return newBytesReadSeekCloser(plain), nil
}

// Has reports whether an encrypted asset with the plaintext id is present.
func (e *EncryptedStore) Has(id string) bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	_, ok := e.index[id]
	return ok
}
