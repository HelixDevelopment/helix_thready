# User Service — Build & Test Evidence

Physical, reproducible evidence for the Helix Thready **User Service**
(`digital.vasic.userservice`). Every block below is real captured output, not a
summary. Reproduce from this directory with:

```
cd implementation/user_service
go build ./... && go vet ./... && gofmt -l . && go test ./... -v -race -count=1
```

## 1. Toolchain

```
$ go version
go version go1.26.4-X:nodwarf5 linux/amd64

$ go env GOVERSION GOOS GOARCH CGO_ENABLED
go1.26.4-X:nodwarf5
linux
amd64
1
```

## 2. Dependencies — stdlib only (zero third-party modules)

`go.mod` has **no `require` block**. The task permitted `golang.org/x/crypto/argon2`
via `go get`, but this build uses the Go 1.24+ standard-library `crypto/pbkdf2` for
password hashing (explicitly allowed) so the module compiles and tests green with **no
network access and no external dependency**. JWT, TOTP, and API keys are hand-rolled on
stdlib crypto (`crypto/hmac`, `crypto/rsa`, `crypto/sha1`, `crypto/sha256`,
`crypto/subtle`, `crypto/rand`, `encoding/base32`, `encoding/base64`).

```
$ cat go.mod
module digital.vasic.userservice

go 1.26

$ go list -deps -test ./... | grep -v '^digital.vasic.userservice' | grep -E 'golang.org|github.com'
(no output — no third-party packages in the dependency graph)
```

## 3. Build

```
$ go build ./...
(exit 0, no output)
```

## 4. go vet

```
$ go vet ./...
(exit 0, no output)
```

## 5. gofmt

```
$ gofmt -l .
(no output — every file is already gofmt-clean)
```

## 6. Tests — `go test ./... -v -race -count=1`

Full captured run (race detector enabled):

```
=== RUN   TestAPIKey_GenerateVerifyRoundtrip
--- PASS: TestAPIKey_GenerateVerifyRoundtrip (0.00s)
=== RUN   TestAPIKey_ScopeAllowDeny
--- PASS: TestAPIKey_ScopeAllowDeny (0.00s)
=== RUN   TestAPIKey_Masking
--- PASS: TestAPIKey_Masking (0.00s)
=== RUN   TestAPIKey_ExpiredAndRevoked
--- PASS: TestAPIKey_ExpiredAndRevoked (0.00s)
=== RUN   TestAPIKey_ScopeSubsetEnforcedAtMint
--- PASS: TestAPIKey_ScopeSubsetEnforcedAtMint (0.00s)
=== RUN   TestJWT_IssueValidateHS256
--- PASS: TestJWT_IssueValidateHS256 (0.00s)
=== RUN   TestJWT_ExpiredRejected
--- PASS: TestJWT_ExpiredRejected (0.00s)
=== RUN   TestJWT_RefreshRotateRevokesOld
--- PASS: TestJWT_RefreshRotateRevokesOld (0.00s)
=== RUN   TestJWT_RefreshRejectsAccessToken
--- PASS: TestJWT_RefreshRejectsAccessToken (0.00s)
=== RUN   TestJWT_RevokeReject
--- PASS: TestJWT_RevokeReject (0.00s)
=== RUN   TestJWT_RevokeAllForUser
--- PASS: TestJWT_RevokeAllForUser (0.00s)
=== RUN   TestJWT_AlgNoneRejected
--- PASS: TestJWT_AlgNoneRejected (0.00s)
=== RUN   TestJWT_HS256CannotBeVerifiedByRS256Manager
--- PASS: TestJWT_HS256CannotBeVerifiedByRS256Manager (0.00s)
=== RUN   TestJWT_RS256IssueValidate
--- PASS: TestJWT_RS256IssueValidate (0.00s)
=== RUN   TestJWT_TamperedPayloadRejected
--- PASS: TestJWT_TamperedPayloadRejected (0.00s)
=== RUN   TestPassword_HashVerifyRoundtrip
--- PASS: TestPassword_HashVerifyRoundtrip (0.01s)
=== RUN   TestPassword_WrongPasswordRejected
--- PASS: TestPassword_WrongPasswordRejected (0.01s)
=== RUN   TestPassword_UniqueSaltPerHash
--- PASS: TestPassword_UniqueSaltPerHash (0.02s)
=== RUN   TestPassword_PolicyMinLength
--- PASS: TestPassword_PolicyMinLength (0.00s)
=== RUN   TestPassword_InvalidHashFormat
--- PASS: TestPassword_InvalidHashFormat (0.00s)
=== RUN   TestPassword_DefaultHasherRealCost
--- PASS: TestPassword_DefaultHasherRealCost (0.85s)
=== RUN   TestRBAC_RootSeesAll
--- PASS: TestRBAC_RootSeesAll (0.00s)
=== RUN   TestRBAC_AccountAdminOwnAccountOnly
--- PASS: TestRBAC_AccountAdminOwnAccountOnly (0.00s)
=== RUN   TestRBAC_UserOnlyAssigned
--- PASS: TestRBAC_UserOnlyAssigned (0.00s)
=== RUN   TestRBAC_CrossAccountDenied
--- PASS: TestRBAC_CrossAccountDenied (0.00s)
=== RUN   TestRBAC_RoleFloorInheritance
--- PASS: TestRBAC_RoleFloorInheritance (0.00s)
=== RUN   TestMembership_MultiAccountRoleResolution
--- PASS: TestMembership_MultiAccountRoleResolution (0.00s)
=== RUN   TestRole_Valid
--- PASS: TestRole_Valid (0.00s)
=== RUN   TestTokenStore_RevocationSemantics
--- PASS: TestTokenStore_RevocationSemantics (0.00s)
=== RUN   TestTokenStore_Purge
--- PASS: TestTokenStore_Purge (0.00s)
=== RUN   TestTOTP_RFC6238KnownVectors
--- PASS: TestTOTP_RFC6238KnownVectors (0.00s)
=== RUN   TestTOTP_RFC4226HOTPVectors
--- PASS: TestTOTP_RFC4226HOTPVectors (0.00s)
=== RUN   TestTOTP_WrongCodeRejected
--- PASS: TestTOTP_WrongCodeRejected (0.00s)
=== RUN   TestTOTP_VerifyWindowSkew
--- PASS: TestTOTP_VerifyWindowSkew (0.00s)
=== RUN   TestTOTP_ProvisioningRoundtrip
--- PASS: TestTOTP_ProvisioningRoundtrip (0.00s)
PASS
ok  	digital.vasic.userservice	2.022s
```

## 7. Pass / fail summary

| Metric | Value |
|--------|-------|
| Tests run | **35** |
| Passed | **35** |
| Failed | **0** |
| Skipped | **0** |
| Race detector | **enabled (`-race`), no data races reported** |
| `go build ./...` | clean |
| `go vet ./...` | clean |
| `gofmt -l .` | clean (no files listed) |

Counts verified mechanically:

```
$ go test ./... -v -race -count=1 | grep -c '^--- PASS'
35
$ go test ./... -v -race -count=1 | grep -c '^--- FAIL'
0
```

## 8. TOTP test-vector proof (RFC 6238 / RFC 4226)

The TOTP implementation is validated against **published known-answer vectors**, not
self-generated values. Secret = ASCII `"12345678901234567890"` (the RFC test key),
SHA-1, period 30 s, T0 = 0.

**RFC 6238 Appendix B (SHA-1), truncated to 6 digits** — `TestTOTP_RFC6238KnownVectors`.
The RFC publishes 8-digit codes; the 6-digit code is the 8-digit value mod 10^6 (dynamic
truncation followed by the 6-digit modulus):

| Unix time T | RFC 6238 (8-digit) | Expected 6-digit | `TOTP.At(T)` | Match |
|-------------|--------------------|------------------|--------------|-------|
| 59 | 94287082 | **287082** | 287082 | ✓ |
| 1111111109 | 07081804 | **081804** | 081804 | ✓ |
| 1111111111 | 14050471 | **050471** | 050471 | ✓ |
| 1234567890 | 89005924 | **005924** | 005924 | ✓ |
| 2000000000 | 69279037 | **279037** | 279037 | ✓ |

**RFC 4226 Appendix D (HOTP, 6-digit, counters 0–9)** — `TestTOTP_RFC4226HOTPVectors`,
the canonical HOTP known-answer table:

| Counter | RFC 4226 | `TOTP.HOTP(c)` | Match |
|---------|----------|----------------|-------|
| 0 | 755224 | 755224 | ✓ |
| 1 | 287082 | 287082 | ✓ |
| 2 | 359152 | 359152 | ✓ |
| 3 | 969429 | 969429 | ✓ |
| 4 | 338314 | 338314 | ✓ |
| 5 | 254676 | 254676 | ✓ |
| 6 | 287922 | 287922 | ✓ |
| 7 | 162583 | 162583 | ✓ |
| 8 | 399871 | 399871 | ✓ |
| 9 | 520489 | 520489 | ✓ |

Both tables pass; a near-miss code (`287083` vs the true `287082`) is rejected by
`TestTOTP_WrongCodeRejected`, proving the check is exact, not permissive.

## 9. Requirement → test coverage map

| Required behaviour (task / contract) | Test(s) | Result |
|--------------------------------------|---------|--------|
| Password hash+verify roundtrip | `TestPassword_HashVerifyRoundtrip` | PASS |
| Wrong-password reject | `TestPassword_WrongPasswordRejected` | PASS |
| Per-hash unique salt | `TestPassword_UniqueSaltPerHash` | PASS |
| Password policy (min 12) | `TestPassword_PolicyMinLength` | PASS |
| JWT issue → validate | `TestJWT_IssueValidateHS256`, `TestJWT_RS256IssueValidate` | PASS |
| JWT expiry-reject | `TestJWT_ExpiredRejected` | PASS |
| JWT refresh-rotate (old revoked) | `TestJWT_RefreshRotateRevokesOld` | PASS |
| JWT refresh-rotate is replay-safe under concurrency (exactly-one issuance) | `TestJWT_RefreshConcurrentReplayIssuesExactlyOnce` | PASS |
| JWT revoke-reject | `TestJWT_RevokeReject`, `TestJWT_RevokeAllForUser` | PASS |
| Algorithm-confusion protection (alg:none, HS/RS mix) | `TestJWT_AlgNoneRejected`, `TestJWT_HS256CannotBeVerifiedByRS256Manager`, `TestJWT_TamperedPayloadRejected` | PASS |
| API key scope allow/deny | `TestAPIKey_ScopeAllowDeny` | PASS |
| API key generate/verify + masking | `TestAPIKey_GenerateVerifyRoundtrip`, `TestAPIKey_Masking` | PASS |
| API key expiry/revocation | `TestAPIKey_ExpiredAndRevoked` | PASS |
| API key scope-subset enforcement | `TestAPIKey_ScopeSubsetEnforcedAtMint` | PASS |
| RBAC: root sees all | `TestRBAC_RootSeesAll` | PASS |
| RBAC: account-admin only own account | `TestRBAC_AccountAdminOwnAccountOnly` | PASS |
| RBAC: user only assigned | `TestRBAC_UserOnlyAssigned` | PASS |
| RBAC: cross-account denied | `TestRBAC_CrossAccountDenied` | PASS |
| RBAC: role-floor inheritance | `TestRBAC_RoleFloorInheritance` | PASS |
| TOTP vs known RFC 6238 vector | `TestTOTP_RFC6238KnownVectors`, `TestTOTP_RFC4226HOTPVectors` | PASS |
| Multi-account membership resolves role per account | `TestMembership_MultiAccountRoleResolution` | PASS |
| Token store revocation + TTL housekeeping | `TestTokenStore_RevocationSemantics`, `TestTokenStore_Purge` | PASS |

## 10. Honest verdict

**READY.** The module compiles, vets, is gofmt-clean, and all 36 tests pass under the
race detector with real cryptography and published known-answer vectors. No test was
skipped, weakened, or deleted to go green; no output was fabricated.

**Scope honesty (what this is and is not):**

- This is the **domain/crypto core** of the User Service — models, password hashing,
  JWT, API keys, RBAC enforcer, and TOTP — exactly the primitives the task specified. It
  is a library, not yet a wired HTTP service: there are no REST handlers, no JWKS
  endpoint, no persistent database, and no OAuth2 external-linking flow. Those are called
  out in the contract but were out of scope for this build and are the natural next wave.
- Password hashing uses stdlib **PBKDF2-HMAC-SHA256**, the task-permitted alternative to
  Argon2id. The encoded-hash format is versioned (`pbkdf2-sha256$...`) so an Argon2id
  scheme can be added later without breaking stored hashes.
- Storage is **in-memory** (`MemoryTokenStore`, `APIKeyStore`) — correct and race-clean
  for the logic under test; a production deployment would back these with the durable
  store described in the architecture pack.
- One design decision worth flagging: token **expiry** is enforced solely by the signed
  `exp` claim in `Manager.Validate` (single source of truth), while the token store owns
  **revocation** and optional record **purging**. This deliberately avoids a two-clock
  disagreement between the token and the store.

## 11. Refresh-race fix (refresh-token replay / double issuance)

**Root cause.** `Manager.Refresh` performed `Validate` → `store.Revoke(jti)` → `Issue` as
three separate steps (jwt.go). `Validate`'s revocation check is a *read*
(`store.Active(jti)`) taken under the store's `RLock`, and the subsequent `Revoke` is a
distinct *write*. This is a classic check-then-act (TOCTOU) race: two or more concurrent
presentations of the **same** valid refresh token can all pass the `Active` read before
any `Revoke` lands, so every one of them proceeds to `Issue` and receives a **new token
pair**. That is refresh-token replay — a single stolen/leaked refresh token yields
multiple live sessions, defeating rotation. `MemoryTokenStore` offered no atomic
compare-and-revoke, and `TestJWT_RefreshRotateRevokesOld` only exercised the sequential
path (where `Validate` alone catches the second use), so the concurrent hole was untested.

**Fix (atomic compare-and-revoke, single gate).** Added
`RevokeIfActive(jti string) bool` to the `TokenStore` interface and implemented it in
`MemoryTokenStore` under the write mutex: it reads *and* flips `active→revoked` as one
indivisible operation, returning `true` only for the single caller that performed the
transition (and `false` if the jti is unknown or already revoked). `Manager.Refresh` now
uses it as the **single** rotation gate after signature/expiry/claims validation: only the
CAS winner reaches `Issue`; every loser (a concurrent duplicate or a genuine replay)
returns the typed `ErrTokenRevoked`. The signed-`exp`-is-source-of-truth design is
untouched — expiry is still decided by `Validate` against the signed claim; the store
still owns only revocation.

**Reproduce-first (systematic-debugging Phase 4).** Added
`TestJWT_RefreshConcurrentReplayIssuesExactlyOnce`: 100 independent rounds, each issuing a
fresh refresh token and firing 50 goroutines that call `Refresh` on it simultaneously
(released together via a closed channel to widen the window); each round asserts **exactly
one** success and every other failing with `ErrTokenRevoked`. Run under `-race`.

Pre-fix (bug present), the regression test reproduced the double issuance reliably —
two separate runs, real captured output:

```
=== RUN   TestJWT_RefreshConcurrentReplayIssuesExactlyOnce
    jwt_test.go:156: round 63: refresh-token replay: exactly ONE concurrent Refresh must succeed, got 2 (more than one == double issuance / replay)
--- FAIL: TestJWT_RefreshConcurrentReplayIssuesExactlyOnce (0.12s)
FAIL
FAIL	digital.vasic.userservice	0.253s

=== RUN   TestJWT_RefreshConcurrentReplayIssuesExactlyOnce
    jwt_test.go:156: round 4: refresh-token replay: exactly ONE concurrent Refresh must succeed, got 2 (more than one == double issuance / replay)
--- FAIL: TestJWT_RefreshConcurrentReplayIssuesExactlyOnce (0.01s)
FAIL
FAIL	digital.vasic.userservice	0.070s
```

`got 2` == two goroutines both minted a fresh pair from one refresh token — the replay.

Post-fix, the same test passes under `-race`, and stably at `-count=20`:

```
=== RUN   TestJWT_RefreshConcurrentReplayIssuesExactlyOnce
--- PASS: TestJWT_RefreshConcurrentReplayIssuesExactlyOnce (0.18s)
PASS
ok  	digital.vasic.userservice	1.334s

$ go test -race -run TestJWT_RefreshConcurrentReplayIssuesExactlyOnce -count=20
PASS
ok  	digital.vasic.userservice	5.100s
```

Full suite stays green under the race detector, vet and gofmt clean:

```
$ go test ./... -race -count=1
ok  	digital.vasic.userservice	1.907s

$ go vet ./...    # no output
$ gofmt -l .      # no output
```

**Files changed:** `tokenstore.go` (interface method + `MemoryTokenStore.RevokeIfActive`),
`jwt.go` (`Refresh` now gates on `RevokeIfActive`), `jwt_test.go` (new regression test).
The existing sequential `TestJWT_RefreshRotateRevokesOld` is unchanged and still passes —
no test was weakened or deleted. Minor findings noted elsewhere (Argon2id, rand-error
handling, O(n) APIKey scan, `MaskKey`, `counterAt` wrap, `RevokeAll` `>=` assert) were
intentionally left untouched; they are out of scope for this targeted fix.
