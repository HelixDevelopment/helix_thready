package userservice

import (
	"testing"
	"time"
)

func TestTokenStore_RevocationSemantics(t *testing.T) {
	s := NewMemoryTokenStore()
	s.Register("jti-1", "user-1", time.Now().Add(time.Hour))

	if !s.Active("jti-1") {
		t.Fatal("freshly registered token must be active")
	}
	if s.Active("unknown") {
		t.Fatal("an unknown jti must be inactive (only issued tokens are accepted)")
	}
	if !s.Revoke("jti-1") {
		t.Fatal("Revoke must find a registered token")
	}
	if s.Active("jti-1") {
		t.Fatal("revoked token must be inactive")
	}
	if s.Revoke("unknown") {
		t.Fatal("Revoke of an unknown jti must return false")
	}
}

func TestTokenStore_Purge(t *testing.T) {
	s := NewMemoryTokenStore()
	fixed := time.Unix(1_700_000_000, 0)
	s.now = func() time.Time { return fixed }

	s.Register("expired", "user-1", fixed.Add(-time.Minute))
	s.Register("live", "user-1", fixed.Add(time.Hour))

	if n := s.Purge(); n != 1 {
		t.Fatalf("Purge removed %d records, want 1", n)
	}
	// The live token survives and remains active; the expired record is gone.
	if !s.Active("live") {
		t.Fatal("live token must survive purge")
	}
	if s.Active("expired") {
		t.Fatal("purged token must be inactive")
	}
}
