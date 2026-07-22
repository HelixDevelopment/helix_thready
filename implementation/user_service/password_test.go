package userservice

import (
	"errors"
	"strings"
	"testing"
)

// fastHasher keeps the -race suite quick while still exercising real PBKDF2 crypto.
func fastHasher() *Hasher { return NewHasher(4096) }

func TestPassword_HashVerifyRoundtrip(t *testing.T) {
	h := fastHasher()
	const pw = "correct horse battery staple"

	encoded, err := h.Hash(pw)
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}
	if !strings.HasPrefix(encoded, pbkdf2Scheme+"$") {
		t.Fatalf("encoded hash has unexpected scheme: %q", encoded)
	}
	if strings.Contains(encoded, pw) {
		t.Fatal("plaintext password leaked into the encoded hash")
	}
	if err := Verify(encoded, pw); err != nil {
		t.Fatalf("Verify roundtrip should succeed: %v", err)
	}
}

func TestPassword_WrongPasswordRejected(t *testing.T) {
	h := fastHasher()
	encoded, err := h.Hash("the-right-password")
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}
	err = Verify(encoded, "the-wrong-password")
	if !errors.Is(err, ErrMismatchedHashAndPassword) {
		t.Fatalf("wrong password must be rejected with ErrMismatchedHashAndPassword, got %v", err)
	}
}

func TestPassword_UniqueSaltPerHash(t *testing.T) {
	h := fastHasher()
	const pw = "same-password-twice"
	a, err := h.Hash(pw)
	if err != nil {
		t.Fatalf("Hash a: %v", err)
	}
	b, err := h.Hash(pw)
	if err != nil {
		t.Fatalf("Hash b: %v", err)
	}
	if a == b {
		t.Fatal("two hashes of the same password must differ (per-hash random salt)")
	}
	// Both must still verify.
	if err := Verify(a, pw); err != nil {
		t.Fatalf("Verify a: %v", err)
	}
	if err := Verify(b, pw); err != nil {
		t.Fatalf("Verify b: %v", err)
	}
}

func TestPassword_PolicyMinLength(t *testing.T) {
	if err := CheckPasswordPolicy("short"); !errors.Is(err, ErrPasswordTooShort) {
		t.Fatalf("11-char-or-less password must fail policy, got %v", err)
	}
	if err := CheckPasswordPolicy("exactly12abc"); err != nil { // 12 chars
		t.Fatalf("12-char password must pass policy, got %v", err)
	}
}

func TestPassword_InvalidHashFormat(t *testing.T) {
	for _, bad := range []string{"", "notahash", "pbkdf2-sha256$notint$xx$yy", "scheme$1$@@@$@@@"} {
		if err := Verify(bad, "whatever"); err == nil {
			t.Fatalf("malformed hash %q must fail verification", bad)
		}
	}
}

func TestPassword_DefaultHasherRealCost(t *testing.T) {
	// Exercise the production cost factor once to prove the real parameters work.
	h := DefaultHasher()
	if h.iterations != DefaultPBKDF2Iterations {
		t.Fatalf("default iterations = %d, want %d", h.iterations, DefaultPBKDF2Iterations)
	}
	encoded, err := h.Hash("a-sufficiently-long-password")
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}
	if err := Verify(encoded, "a-sufficiently-long-password"); err != nil {
		t.Fatalf("default-cost roundtrip: %v", err)
	}
}
