package userservice

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"strings"
	"sync"
	"time"
)

// API keys are the non-interactive credential path (authn-authz.md §4). They are
// prefixed ("sk-"), scoped, returned in full exactly once at creation, and thereafter
// only ever shown masked. The store keeps only a SHA-256 hash of the secret, never the
// plaintext. A key's scopes must be a subset of the minting principal's scopes.

const (
	// DefaultAPIKeyPrefix is the human-legible key prefix.
	DefaultAPIKeyPrefix = "sk-"
	// apiKeyRandomBytes is the entropy of the secret portion.
	apiKeyRandomBytes = 32
)

var (
	// ErrAPIKeyNotFound is returned when a presented key is unknown.
	ErrAPIKeyNotFound = errors.New("userservice: api key not found")
	// ErrAPIKeyRevoked is returned when a key has been revoked.
	ErrAPIKeyRevoked = errors.New("userservice: api key revoked")
	// ErrAPIKeyExpired is returned when a key is past its expiry.
	ErrAPIKeyExpired = errors.New("userservice: api key expired")
	// ErrScopeNotSubset is returned when requested scopes exceed the minting principal's.
	ErrScopeNotSubset = errors.New("userservice: requested scopes are not a subset of the minter's scopes")
)

// APIKey is the stored, non-secret record of an issued key. The plaintext secret is
// never stored; only Hash (SHA-256 of the secret) is kept for verification.
type APIKey struct {
	ID        string
	Name      string
	Prefix    string
	Hash      string // hex SHA-256 of the full presented secret
	Masked    string // display form, e.g. "sk-ab…yz"
	Scopes    []Permission
	AccountID string
	Role      Role
	ExpiresAt time.Time // zero => no expiry
	Revoked   bool
	CreatedAt time.Time
}

// HasScope reports whether the key carries a specific permission.
func (k *APIKey) HasScope(p Permission) bool {
	for _, s := range k.Scopes {
		if s == p {
			return true
		}
	}
	return false
}

// HasAllScopes reports whether the key carries every required permission.
func (k *APIKey) HasAllScopes(required ...Permission) bool {
	for _, r := range required {
		if !k.HasScope(r) {
			return false
		}
	}
	return true
}

// Generator mints API keys with a configured prefix.
type Generator struct {
	Prefix string
}

// NewGenerator returns a Generator; an empty prefix defaults to DefaultAPIKeyPrefix.
func NewGenerator(prefix string) *Generator {
	if prefix == "" {
		prefix = DefaultAPIKeyPrefix
	}
	return &Generator{Prefix: prefix}
}

// Generate creates a new key. It returns the stored record and the plaintext secret,
// which is the ONLY time the caller ever sees the full secret. minterScopes, when
// non-nil, enforces the subset rule (a key cannot exceed its minter's scopes).
func (g *Generator) Generate(name string, scopes []Permission, accountID string, role Role, expiresAt time.Time, minterScopes []Permission) (*APIKey, string, error) {
	if minterScopes != nil && !scopesSubset(scopes, minterScopes) {
		return nil, "", ErrScopeNotSubset
	}
	raw := make([]byte, apiKeyRandomBytes)
	if _, err := rand.Read(raw); err != nil {
		return nil, "", err
	}
	body := base64.RawURLEncoding.EncodeToString(raw)
	plaintext := g.Prefix + body

	rec := &APIKey{
		ID:        hex.EncodeToString(mustRandom(8)),
		Name:      name,
		Prefix:    g.Prefix,
		Hash:      hashSecret(plaintext),
		Masked:    MaskKey(plaintext),
		Scopes:    append([]Permission(nil), scopes...),
		AccountID: accountID,
		Role:      role,
		ExpiresAt: expiresAt,
		CreatedAt: time.Now(),
	}
	return rec, plaintext, nil
}

// MaskKey returns a display-safe form of a key: prefix + first 2 + "…" + last 2 of the
// secret body. Full secrets must never be logged (authn-authz.md §4).
func MaskKey(plaintext string) string {
	prefix := DefaultAPIKeyPrefix
	body := plaintext
	if i := strings.Index(plaintext, "-"); i >= 0 && i < len(plaintext)-1 {
		prefix = plaintext[:i+1]
		body = plaintext[i+1:]
	}
	if len(body) <= 4 {
		return prefix + "…"
	}
	return prefix + body[:2] + "…" + body[len(body)-2:]
}

func hashSecret(plaintext string) string {
	sum := sha256.Sum256([]byte(plaintext))
	return hex.EncodeToString(sum[:])
}

func mustRandom(n int) []byte {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return b
}

// scopesSubset reports whether every scope in child is present in parent.
func scopesSubset(child, parent []Permission) bool {
	set := make(map[Permission]struct{}, len(parent))
	for _, p := range parent {
		set[p] = struct{}{}
	}
	for _, c := range child {
		if _, ok := set[c]; !ok {
			return false
		}
	}
	return true
}

// APIKeyStore is an in-memory store keyed by the secret hash.
type APIKeyStore struct {
	mu   sync.RWMutex
	keys map[string]*APIKey // hash -> record
	now  func() time.Time
}

// NewAPIKeyStore returns an empty store using the wall clock.
func NewAPIKeyStore() *APIKeyStore {
	return &APIKeyStore{keys: make(map[string]*APIKey), now: time.Now}
}

// Store persists a generated key record.
func (s *APIKeyStore) Store(k *APIKey) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.keys[k.Hash] = k
}

// Revoke marks the key with the given ID revoked. Returns false if not found.
func (s *APIKeyStore) Revoke(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, k := range s.keys {
		if k.ID == id {
			k.Revoked = true
			return true
		}
	}
	return false
}

// Verify authenticates a presented plaintext key against the store, checking existence,
// revocation, and expiry in constant time with respect to the secret.
func (s *APIKeyStore) Verify(presented string) (*APIKey, error) {
	want := hashSecret(presented)
	s.mu.RLock()
	defer s.mu.RUnlock()
	// Constant-time lookup: compare against every stored hash so timing does not reveal
	// which (if any) key matched.
	var match *APIKey
	for h, k := range s.keys {
		if subtle.ConstantTimeCompare([]byte(h), []byte(want)) == 1 {
			match = k
		}
	}
	if match == nil {
		return nil, ErrAPIKeyNotFound
	}
	if match.Revoked {
		return nil, ErrAPIKeyRevoked
	}
	if !match.ExpiresAt.IsZero() && !s.now().Before(match.ExpiresAt) {
		return nil, ErrAPIKeyExpired
	}
	return match, nil
}

// VerifyScopes authenticates a presented key and additionally requires a scope set.
func (s *APIKeyStore) VerifyScopes(presented string, required ...Permission) (*APIKey, error) {
	k, err := s.Verify(presented)
	if err != nil {
		return nil, err
	}
	if !k.HasAllScopes(required...) {
		return nil, ErrScopeNotSubset
	}
	return k, nil
}
