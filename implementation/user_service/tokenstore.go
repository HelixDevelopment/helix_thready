package userservice

import (
	"sync"
	"time"
)

// TokenStore tracks issued token identifiers (jti) so they can be revoked. This mirrors
// the module's pkg/token store ("TTL + revocation", authn-authz.md §8) and the
// accesstoken.RevokeAllForUser logout-all capability.
//
// Expiry is NOT gated here: a token's lifetime is cryptographically bound in its signed
// exp claim and authoritatively enforced by Manager.Validate against the manager's clock
// — a single source of truth, so two clocks can never disagree about validity. The store
// records each token's expiry only so expired records can be purged for memory hygiene
// (Purge), and treats an unknown jti as inactive (only tokens we issued are accepted).
type TokenStore interface {
	// Register records an issued token identifier with its owner and expiry.
	Register(jti, userID string, expiresAt time.Time)
	// Active reports whether jti is known and not revoked.
	Active(jti string) bool
	// Revoke marks a single token identifier as revoked. Returns false if unknown.
	Revoke(jti string) bool
	// RevokeIfActive atomically transitions jti from active to revoked. It returns true
	// only for the single caller that performed that transition, and false if the jti is
	// unknown or was already revoked. Implementations MUST make the "is it active?" check
	// and the "mark revoked" write one indivisible operation, so it can be used as a
	// compare-and-swap gate that admits exactly one winner under concurrency (refresh
	// rotation: only that winner may mint a replacement, closing the replay/double-issue
	// race that a separate Active-then-Revoke sequence leaves open).
	RevokeIfActive(jti string) bool
	// RevokeAllForUser revokes every active token owned by userID and returns the count.
	RevokeAllForUser(userID string) int
}

type tokenRecord struct {
	userID    string
	expiresAt time.Time
	revoked   bool
}

// MemoryTokenStore is an in-memory, goroutine-safe TokenStore.
type MemoryTokenStore struct {
	mu      sync.RWMutex
	records map[string]*tokenRecord
	now     func() time.Time
}

// NewMemoryTokenStore returns an empty in-memory token store using the wall clock.
func NewMemoryTokenStore() *MemoryTokenStore {
	return &MemoryTokenStore{
		records: make(map[string]*tokenRecord),
		now:     time.Now,
	}
}

// Register records an issued token identifier.
func (s *MemoryTokenStore) Register(jti, userID string, expiresAt time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.records[jti] = &tokenRecord{userID: userID, expiresAt: expiresAt}
}

// Active reports whether jti is known and not revoked. Expiry is enforced by
// Manager.Validate against the signed exp claim, not here (see the TokenStore doc).
func (s *MemoryTokenStore) Active(jti string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	rec, ok := s.records[jti]
	if !ok || rec.revoked {
		return false
	}
	return true
}

// Revoke marks a single token identifier as revoked.
func (s *MemoryTokenStore) Revoke(jti string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	rec, ok := s.records[jti]
	if !ok {
		return false
	}
	rec.revoked = true
	return true
}

// RevokeIfActive atomically transitions jti from active to revoked, returning true only
// for the caller that performed the transition. The read-and-write happens under a single
// write lock, so concurrent callers presenting the same jti are serialized and exactly one
// observes it active — the atomic compare-and-revoke that makes refresh rotation
// replay-safe. Returns false if jti is unknown or already revoked.
func (s *MemoryTokenStore) RevokeIfActive(jti string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	rec, ok := s.records[jti]
	if !ok || rec.revoked {
		return false
	}
	rec.revoked = true
	return true
}

// RevokeAllForUser revokes every active token owned by userID.
func (s *MemoryTokenStore) RevokeAllForUser(userID string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := 0
	for _, rec := range s.records {
		if rec.userID == userID && !rec.revoked {
			rec.revoked = true
			n++
		}
	}
	return n
}

// Purge drops records whose expiry has passed, for memory hygiene (the "TTL" half of the
// store). It does not affect validity decisions — those come from the signed exp claim.
// Returns the number of records removed.
func (s *MemoryTokenStore) Purge() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now()
	n := 0
	for jti, rec := range s.records {
		if !rec.expiresAt.IsZero() && !now.Before(rec.expiresAt) {
			delete(s.records, jti)
			n++
		}
	}
	return n
}
