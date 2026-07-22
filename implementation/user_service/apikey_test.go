package userservice

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestAPIKey_GenerateVerifyRoundtrip(t *testing.T) {
	gen := NewGenerator(DefaultAPIKeyPrefix)
	store := NewAPIKeyStore()

	rec, plaintext, err := gen.Generate("ci-bot", []Permission{PermPostsRead, PermSearchRead}, "acct-1", RoleUser, time.Time{}, nil)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	store.Store(rec)

	if !strings.HasPrefix(plaintext, "sk-") {
		t.Fatalf("key must carry the sk- prefix, got %q", plaintext)
	}
	// The store must never hold the plaintext.
	if strings.Contains(rec.Hash, plaintext) || rec.Hash == plaintext {
		t.Fatal("stored record must not contain the plaintext secret")
	}

	got, err := store.Verify(plaintext)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if got.ID != rec.ID {
		t.Fatalf("verified wrong key: got %s want %s", got.ID, rec.ID)
	}

	// A bogus key must fail.
	if _, err := store.Verify("sk-does-not-exist"); !errors.Is(err, ErrAPIKeyNotFound) {
		t.Fatalf("unknown key must fail with ErrAPIKeyNotFound, got %v", err)
	}
}

func TestAPIKey_ScopeAllowDeny(t *testing.T) {
	gen := NewGenerator("")
	store := NewAPIKeyStore()
	rec, plaintext, err := gen.Generate("reader", []Permission{PermPostsRead, PermAssetsRead}, "acct-1", RoleUser, time.Time{}, nil)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	store.Store(rec)

	// Allowed: key has posts:read.
	if _, err := store.VerifyScopes(plaintext, PermPostsRead); err != nil {
		t.Fatalf("posts:read should be allowed: %v", err)
	}
	// Allowed: key has both required.
	if _, err := store.VerifyScopes(plaintext, PermPostsRead, PermAssetsRead); err != nil {
		t.Fatalf("posts:read+assets:read should be allowed: %v", err)
	}
	// Denied: key lacks posts:write.
	if _, err := store.VerifyScopes(plaintext, PermPostsWrite); !errors.Is(err, ErrScopeNotSubset) {
		t.Fatalf("posts:write should be denied, got %v", err)
	}
}

func TestAPIKey_Masking(t *testing.T) {
	masked := MaskKey("sk-abcdefghijklmnop")
	if strings.Contains(masked, "cdefghijklmn") {
		t.Fatalf("masked key leaks the middle of the secret: %q", masked)
	}
	if !strings.HasPrefix(masked, "sk-") || !strings.Contains(masked, "…") {
		t.Fatalf("unexpected mask format: %q", masked)
	}
	// Prefix + first 2 + … + last 2.
	if masked != "sk-ab…op" {
		t.Fatalf("mask = %q, want sk-ab…op", masked)
	}
}

func TestAPIKey_ExpiredAndRevoked(t *testing.T) {
	gen := NewGenerator("")
	store := NewAPIKeyStore()
	fixed := time.Unix(1_700_000_000, 0)
	store.now = func() time.Time { return fixed }

	// Expired key.
	expRec, expKey, err := gen.Generate("expiring", []Permission{PermPostsRead}, "acct-1", RoleUser, fixed.Add(-time.Minute), nil)
	if err != nil {
		t.Fatalf("Generate expiring: %v", err)
	}
	store.Store(expRec)
	if _, err := store.Verify(expKey); !errors.Is(err, ErrAPIKeyExpired) {
		t.Fatalf("expired key must fail with ErrAPIKeyExpired, got %v", err)
	}

	// Revoked key.
	revRec, revKey, err := gen.Generate("revoked", []Permission{PermPostsRead}, "acct-1", RoleUser, time.Time{}, nil)
	if err != nil {
		t.Fatalf("Generate revoked: %v", err)
	}
	store.Store(revRec)
	if _, err := store.Verify(revKey); err != nil {
		t.Fatalf("key should verify before revoke: %v", err)
	}
	if !store.Revoke(revRec.ID) {
		t.Fatal("Revoke must find the key")
	}
	if _, err := store.Verify(revKey); !errors.Is(err, ErrAPIKeyRevoked) {
		t.Fatalf("revoked key must fail with ErrAPIKeyRevoked, got %v", err)
	}
}

func TestAPIKey_ScopeSubsetEnforcedAtMint(t *testing.T) {
	gen := NewGenerator("")
	// Minter (a standard user) holds only read scopes.
	minter := []Permission{PermPostsRead, PermAssetsRead}

	// Attempt to mint a key with root:admin -> must be rejected.
	if _, _, err := gen.Generate("evil", []Permission{PermRootAdmin}, "acct-1", RoleUser, time.Time{}, minter); !errors.Is(err, ErrScopeNotSubset) {
		t.Fatalf("minting a key exceeding the minter's scopes must fail, got %v", err)
	}
	// A subset of the minter's scopes is fine.
	if _, _, err := gen.Generate("ok", []Permission{PermPostsRead}, "acct-1", RoleUser, time.Time{}, minter); err != nil {
		t.Fatalf("subset key mint should succeed: %v", err)
	}
}
