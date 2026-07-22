package userservice

import (
	"crypto/rsa"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"
)

func hsConfig(clock *time.Time) Config {
	return Config{
		Algorithm:  HS256,
		Issuer:     "thready-user-service",
		Audience:   "thready-api",
		AccessTTL:  15 * time.Minute,
		RefreshTTL: 7 * 24 * time.Hour,
		KeyID:      "2026-07",
		HMACSecret: []byte("test-secret-key-please-change-me"),
		Now:        func() time.Time { return *clock },
	}
}

func TestJWT_IssueValidateHS256(t *testing.T) {
	clock := time.Unix(1_700_000_000, 0)
	m, err := NewManager(hsConfig(&clock), NewMemoryTokenStore())
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	pair, err := m.Issue("user-1", string(RoleAccountAdmin), "acct-1", []string{"posts:read", "posts:write"})
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	if pair.TokenType != "Bearer" || pair.ExpiresIn != int64((15*time.Minute).Seconds()) {
		t.Fatalf("unexpected token pair metadata: %+v", pair)
	}
	claims, err := m.Validate(pair.AccessToken)
	if err != nil {
		t.Fatalf("Validate access: %v", err)
	}
	if claims.Subject != "user-1" || claims.Role != string(RoleAccountAdmin) || claims.AccountID != "acct-1" {
		t.Fatalf("unexpected claims: %+v", claims)
	}
	if claims.TokenType != tokenTypeAccess {
		t.Fatalf("access token_type = %q", claims.TokenType)
	}
}

func TestJWT_ExpiredRejected(t *testing.T) {
	clock := time.Unix(1_700_000_000, 0)
	m, err := NewManager(hsConfig(&clock), NewMemoryTokenStore())
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	pair, err := m.Issue("user-1", string(RoleUser), "acct-1", nil)
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	// Advance the clock past the access-token lifetime.
	clock = clock.Add(16 * time.Minute)
	_, err = m.Validate(pair.AccessToken)
	if !errors.Is(err, ErrTokenExpired) {
		t.Fatalf("expired token must be rejected with ErrTokenExpired, got %v", err)
	}
}

func TestJWT_RefreshRotateRevokesOld(t *testing.T) {
	clock := time.Unix(1_700_000_000, 0)
	m, err := NewManager(hsConfig(&clock), NewMemoryTokenStore())
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	pair, err := m.Issue("user-1", string(RoleUser), "acct-1", []string{"posts:read"})
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	newPair, err := m.Refresh(pair.RefreshToken)
	if err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	if newPair.AccessToken == pair.AccessToken || newPair.RefreshToken == pair.RefreshToken {
		t.Fatal("refresh must rotate to fresh tokens")
	}
	// The new access token validates.
	if _, err := m.Validate(newPair.AccessToken); err != nil {
		t.Fatalf("new access token must validate: %v", err)
	}
	// The OLD refresh token must be revoked (rotation): reusing it fails.
	if _, err := m.Refresh(pair.RefreshToken); !errors.Is(err, ErrTokenRevoked) {
		t.Fatalf("reusing a rotated refresh token must fail with ErrTokenRevoked, got %v", err)
	}
}

// TestJWT_RefreshConcurrentReplayIssuesExactlyOnce is the regression guard for the
// refresh-token replay / double-issuance race: N goroutines present the SAME valid
// refresh token concurrently. Rotation must be atomic — EXACTLY ONE presentation may
// mint a replacement pair; every other must be rejected as revoked. Before the fix,
// Validate (a read of Active) and Revoke were separate steps, so multiple goroutines
// could all pass Validate before any Revoke landed and all receive new token pairs.
//
// The race window (Validate-read → Revoke) is tiny, so a single round is only
// probabilistically triggering; we run many independent rounds — each on a freshly
// issued refresh token — so the check-then-act bug is exercised reliably. Post-fix the
// atomic compare-and-revoke makes every round deterministically yield exactly one
// winner. Runs clean under -race.
func TestJWT_RefreshConcurrentReplayIssuesExactlyOnce(t *testing.T) {
	clock := time.Unix(1_700_000_000, 0)
	m, err := NewManager(hsConfig(&clock), NewMemoryTokenStore())
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	const rounds = 100
	const n = 50
	for round := 0; round < rounds; round++ {
		pair, err := m.Issue("user-1", string(RoleUser), "acct-1", []string{"posts:read"})
		if err != nil {
			t.Fatalf("round %d: Issue: %v", round, err)
		}

		var (
			start     = make(chan struct{})
			wg        sync.WaitGroup
			mu        sync.Mutex
			successes int
			winner    TokenPair
			badErrs   []error
		)
		wg.Add(n)
		for i := 0; i < n; i++ {
			go func() {
				defer wg.Done()
				<-start // release all goroutines together to widen the race window
				np, err := m.Refresh(pair.RefreshToken)
				mu.Lock()
				defer mu.Unlock()
				switch {
				case err == nil:
					successes++
					winner = np
				case errors.Is(err, ErrTokenRevoked):
					// expected loss: the refresh token was already rotated away
				default:
					badErrs = append(badErrs, err)
				}
			}()
		}
		close(start)
		wg.Wait()

		for _, e := range badErrs {
			t.Errorf("round %d: concurrent Refresh loser returned unexpected error (want ErrTokenRevoked): %v", round, e)
		}
		if successes != 1 {
			t.Fatalf("round %d: refresh-token replay: exactly ONE concurrent Refresh must succeed, got %d "+
				"(more than one == double issuance / replay)", round, successes)
		}
		// The lone winner's new access token validates...
		if _, err := m.Validate(winner.AccessToken); err != nil {
			t.Fatalf("round %d: winning pair's access token must validate: %v", round, err)
		}
		// ...and the presented (now rotated) refresh token is permanently dead.
		if _, err := m.Refresh(pair.RefreshToken); !errors.Is(err, ErrTokenRevoked) {
			t.Fatalf("round %d: rotated refresh token must stay revoked, got %v", round, err)
		}
	}
}

func TestJWT_RefreshRejectsAccessToken(t *testing.T) {
	clock := time.Unix(1_700_000_000, 0)
	m, err := NewManager(hsConfig(&clock), NewMemoryTokenStore())
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	pair, err := m.Issue("user-1", string(RoleUser), "acct-1", nil)
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	if _, err := m.Refresh(pair.AccessToken); !errors.Is(err, ErrWrongTokenType) {
		t.Fatalf("refreshing with an access token must fail with ErrWrongTokenType, got %v", err)
	}
}

func TestJWT_RevokeReject(t *testing.T) {
	clock := time.Unix(1_700_000_000, 0)
	m, err := NewManager(hsConfig(&clock), NewMemoryTokenStore())
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	pair, err := m.Issue("user-1", string(RoleUser), "acct-1", nil)
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	if _, err := m.Validate(pair.AccessToken); err != nil {
		t.Fatalf("token should validate before revoke: %v", err)
	}
	if err := m.Revoke(pair.AccessToken); err != nil {
		t.Fatalf("Revoke: %v", err)
	}
	if _, err := m.Validate(pair.AccessToken); !errors.Is(err, ErrTokenRevoked) {
		t.Fatalf("revoked token must be rejected with ErrTokenRevoked, got %v", err)
	}
}

func TestJWT_RevokeAllForUser(t *testing.T) {
	clock := time.Unix(1_700_000_000, 0)
	m, err := NewManager(hsConfig(&clock), NewMemoryTokenStore())
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	p1, _ := m.Issue("user-1", string(RoleUser), "acct-1", nil)
	p2, _ := m.Issue("user-1", string(RoleUser), "acct-1", nil)
	other, _ := m.Issue("user-2", string(RoleUser), "acct-1", nil)

	n := m.RevokeAllForUser("user-1")
	if n < 4 { // 2 access + 2 refresh for user-1
		t.Fatalf("RevokeAllForUser revoked %d tokens, want >= 4", n)
	}
	if _, err := m.Validate(p1.AccessToken); !errors.Is(err, ErrTokenRevoked) {
		t.Fatalf("user-1 token 1 must be revoked, got %v", err)
	}
	if _, err := m.Validate(p2.AccessToken); !errors.Is(err, ErrTokenRevoked) {
		t.Fatalf("user-1 token 2 must be revoked, got %v", err)
	}
	// user-2's token is unaffected.
	if _, err := m.Validate(other.AccessToken); err != nil {
		t.Fatalf("user-2 token must remain valid, got %v", err)
	}
}

func TestJWT_AlgNoneRejected(t *testing.T) {
	clock := time.Unix(1_700_000_000, 0)
	m, err := NewManager(hsConfig(&clock), NewMemoryTokenStore())
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	// Forge a token with header alg="none" and an empty signature.
	header := b64([]byte(`{"alg":"none","typ":"JWT"}`))
	payload := b64([]byte(`{"sub":"attacker","iss":"thready-user-service","aud":"thready-api","exp":9999999999}`))
	forged := header + "." + payload + "."
	if _, err := m.Validate(forged); !errors.Is(err, ErrAlgorithmMismatch) {
		t.Fatalf("alg:none token must be rejected with ErrAlgorithmMismatch, got %v", err)
	}
}

func TestJWT_HS256CannotBeVerifiedByRS256Manager(t *testing.T) {
	// Algorithm-confusion: a token signed HS256 must be rejected by an RS256-configured
	// manager (the classic RSA-public-key-as-HMAC-secret downgrade attack).
	clock := time.Unix(1_700_000_000, 0)
	hsm, err := NewManager(hsConfig(&clock), NewMemoryTokenStore())
	if err != nil {
		t.Fatalf("NewManager HS: %v", err)
	}
	pair, err := hsm.Issue("user-1", string(RoleUser), "acct-1", nil)
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}

	key := testRSAKey(t)
	rsm, err := NewManager(Config{
		Algorithm:  RS256,
		Issuer:     "thready-user-service",
		Audience:   "thready-api",
		RSAPrivate: key,
		Now:        func() time.Time { return clock },
	}, NewMemoryTokenStore())
	if err != nil {
		t.Fatalf("NewManager RS: %v", err)
	}
	if _, err := rsm.Validate(pair.AccessToken); !errors.Is(err, ErrAlgorithmMismatch) {
		t.Fatalf("HS256 token must be rejected by RS256 manager, got %v", err)
	}
}

func TestJWT_RS256IssueValidate(t *testing.T) {
	clock := time.Unix(1_700_000_000, 0)
	key := testRSAKey(t)
	m, err := NewManager(Config{
		Algorithm:  RS256,
		Issuer:     "thready-user-service",
		Audience:   "thready-api",
		AccessTTL:  15 * time.Minute,
		RefreshTTL: 7 * 24 * time.Hour,
		KeyID:      "2026-07-a1b2c3d4",
		RSAPrivate: key,
		Now:        func() time.Time { return clock },
	}, NewMemoryTokenStore())
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	pair, err := m.Issue("user-1", string(RoleRoot), "acct-1", []string{"root:admin"})
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	claims, err := m.Validate(pair.AccessToken)
	if err != nil {
		t.Fatalf("Validate RS256: %v", err)
	}
	if claims.Role != string(RoleRoot) {
		t.Fatalf("unexpected role claim: %q", claims.Role)
	}
	// A token with a corrupted signature must fail.
	tampered := pair.AccessToken[:len(pair.AccessToken)-4] + "AAAA"
	if _, err := m.Validate(tampered); err == nil {
		t.Fatal("tampered RS256 signature must fail verification")
	}
}

func TestJWT_TamperedPayloadRejected(t *testing.T) {
	clock := time.Unix(1_700_000_000, 0)
	m, err := NewManager(hsConfig(&clock), NewMemoryTokenStore())
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	pair, err := m.Issue("user-1", string(RoleUser), "acct-1", nil)
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	parts := strings.Split(pair.AccessToken, ".")
	// Swap the payload for an escalated one; the HMAC over the original input no longer matches.
	parts[1] = b64([]byte(`{"sub":"user-1","role":"root_admin","iss":"thready-user-service","aud":"thready-api","exp":9999999999}`))
	forged := strings.Join(parts, ".")
	if _, err := m.Validate(forged); !errors.Is(err, ErrSignatureInvalid) {
		t.Fatalf("tampered payload must fail with ErrSignatureInvalid, got %v", err)
	}
}

// testRSAKey generates a 2048-bit RSA key for tests.
func testRSAKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	return generatedRSAKey
}
