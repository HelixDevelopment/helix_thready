package assetservice

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

// sha256hex returns the lowercase hex SHA-256 of b — the expected content id.
func sha256hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// newStore returns a ContentStore rooted at a fresh temp dir (auto-cleaned).
func newStore(t *testing.T) *ContentStore {
	t.Helper()
	cs, err := NewContentStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewContentStore: %v", err)
	}
	return cs
}
