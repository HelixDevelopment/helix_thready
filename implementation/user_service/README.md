# Helix Thready — User Service (`digital.vasic.userservice`)

A self-contained, **stdlib-only** Go module implementing the identity and access-control
core of the Helix Thready User Service. It realises the intended contract in
[`docs/public/research/mvp/api/authn-authz.md`](../../docs/public/research/mvp/api/authn-authz.md)
and
[`docs/public/research/mvp/architecture/security-model.md`](../../docs/public/research/mvp/architecture/security-model.md)
and closes gap-register item **[BUILD-NEW: User Service]**.

- **Module path:** `digital.vasic.userservice`
- **Go:** 1.26 (uses the Go 1.24+ stdlib `crypto/pbkdf2`)
- **Dependencies:** none — the entire module is standard-library only. No `require`
  block, no network needed to build or test.
- **Status:** READY (domain/crypto core). See [`EVIDENCE.md`](./EVIDENCE.md) for the real
  build/vet/fmt/test capture and the TOTP known-answer proof.

## Purpose

Provides the primitives every Thready service needs to authenticate and authorize a
request, without pulling in a third-party auth stack:

| Concern | Type(s) | File |
|---------|---------|------|
| Domain models | `User`, `Account`, `Role`, `Permission`, `Membership` | `models.go` |
| Password hashing | `Hasher`, `Verify`, `CheckPasswordPolicy` | `password.go` |
| JWT (HS256 + RS256) | `Manager`, `Config`, `Claims`, `TokenPair` | `jwt.go` |
| Token revocation store | `TokenStore`, `MemoryTokenStore` | `tokenstore.go` |
| API keys | `Generator`, `APIKey`, `APIKeyStore`, `MaskKey` | `apikey.go` |
| RBAC enforcement | `Enforcer`, `Decision`, `CheckRoleFloor` | `rbac.go` |
| TOTP MFA (RFC 6238) | `TOTP`, `GenerateTOTPSecret` | `totp.go` |

## API surface (selected)

### Passwords — PBKDF2-HMAC-SHA256, per-hash salt, constant-time verify

```go
h := userservice.DefaultHasher()          // or NewHasher(iterations)
enc, _ := h.Hash("a-strong-passphrase")   // "pbkdf2-sha256$210000$<b64salt>$<b64dk>"
err := userservice.Verify(enc, "a-strong-passphrase") // nil on match
_ = userservice.CheckPasswordPolicy(pw)   // enforces min 12 chars
```

The security model specifies Argon2id; this build uses the task-permitted stdlib PBKDF2
alternative. The hash string is versioned so an Argon2id scheme can be added without
breaking stored hashes.

### JWT — access + refresh, rotation, revocation, algorithm pinning

```go
store := userservice.NewMemoryTokenStore()
mgr, _ := userservice.NewManager(userservice.Config{
    Algorithm:  userservice.HS256, // default; RS256 also supported
    Issuer:     "thready-user-service",
    Audience:   "thready-api",
    AccessTTL:  15 * time.Minute,   // contract default
    RefreshTTL: 7 * 24 * time.Hour, // contract default
    KeyID:      "2026-07-a1b2c3d4", // human-legible kid
    HMACSecret: secret,             // or RSAPrivate for RS256
}, store)

pair, _ := mgr.Issue("user-uuid", string(userservice.RoleAccountAdmin), "acct-uuid",
    []string{"posts:read", "posts:write"})
claims, err := mgr.Validate(pair.AccessToken)  // checks sig, exp, nbf, iss, aud, revocation
next, _ := mgr.Refresh(pair.RefreshToken)      // rotates; old refresh token is revoked
_ = mgr.Revoke(pair.AccessToken)               // single-session logout
_ = mgr.RevokeAllForUser("user-uuid")          // logout-all
```

Security controls enforced (contract §3, §10): the algorithm is **pinned** to the
configured one, so `alg:none` and HS256-signed-with-the-RSA-public-key downgrade attacks
are rejected; refresh **rotation** revokes the presented refresh token; expiry is enforced
authoritatively by the signed `exp` claim.

### API keys — prefixed, scoped, masked, subset-enforced

```go
gen := userservice.NewGenerator("sk-")
key, plaintext, _ := gen.Generate("ci-bot",
    []userservice.Permission{userservice.PermPostsRead, userservice.PermSearchRead},
    "acct-uuid", userservice.RoleUser, expiresAt, minterScopes) // subset rule enforced
keyStore := userservice.NewAPIKeyStore()
keyStore.Store(key)                                  // only a SHA-256 hash is kept
k, err := keyStore.VerifyScopes(plaintext, userservice.PermPostsRead) // allow/deny
_ = userservice.MaskKey(plaintext)                   // "sk-ab…yz" for logs/UI
```

The full secret is returned exactly once at creation; the store never holds plaintext; a
key's scopes must be a subset of the minting principal's scopes.

### RBAC — three-tier, tenant-isolated

```go
enf := userservice.NewEnforcer()
ok := enf.Allow(user, "acct-uuid", userservice.PermPostsWrite) // bool
d  := enf.Evaluate(user, "acct-uuid", userservice.PermRootAdmin) // allow|deny|audit
```

### TOTP — RFC 6238, mandatory for admin tiers

```go
totp, b32, _ := userservice.GenerateTOTPSecret()      // provision
uri := totp.ProvisioningURI("thready-user-service", "alice@example.com")
ok := totp.Verify(userSuppliedCode, time.Now())       // constant-time, ±skew window
```

## RBAC matrix

Roles (fixed hierarchy, `research_request_final §6.1`): `root_admin` > `account_admin` >
`user`. Two gates stand between a caller and a resource — **tenant isolation** (target
account must match, unless root) and **role floor + grant**.

| Principal | Own account | Other account | root:admin | accounts:admin / billing:read | Assigned user scopes |
|-----------|-------------|---------------|------------|-------------------------------|----------------------|
| **root_admin** (global) | allow all | **allow all** (cross-tenant) | allow | allow | allow |
| **account_admin** | allow all (its account) | **deny** | deny | allow (own account) | allow (own account) |
| **user** | only explicitly-assigned permissions | **deny** | deny | deny (role floor) | allow if assigned |

Per-permission **role floor** (a route requires both its floor and its scope):

| Permission | Minimum role |
|------------|--------------|
| `posts:read/write`, `assets:read/write`, `search:read`, `skills:read/write`, `events:read` | `user` |
| `accounts:admin`, `billing:read` | `account_admin` |
| `root:admin` | `root_admin` |

**Multi-account:** a `User` carries a list of `Membership{AccountID, Role, Scopes}`, so the
same user can be `account_admin` in account A and a plain `user` in account B; `RoleIn` and
the enforcer resolve the correct role per target account.

## Scope catalog

`posts:read`, `posts:write`, `assets:read`, `assets:write`, `search:read`, `skills:read`,
`skills:write`, `events:read`, `accounts:admin`, `billing:read`, `root:admin`
(authn-authz.md §7; exposed as `userservice.ScopeCatalog`).

## Running the tests

```
cd implementation/user_service
go build ./...
go vet ./...
gofmt -l .          # expect no output
go test ./... -v -race -count=1
```

Current result: **35 tests, all passing under `-race`**, `go vet` clean, `gofmt` clean.
The TOTP suite validates against the published **RFC 6238 Appendix B** and **RFC 4226
Appendix D** known-answer vectors. See [`EVIDENCE.md`](./EVIDENCE.md) for the full capture.

## Scope & non-goals

This module is the **domain/crypto core**, not a wired HTTP server. Out of scope for this
build (natural next wave): REST handlers and the `pkg/middleware` chain, the
`/.well-known/jwks.json` endpoint and monthly key rotation, a durable database behind the
in-memory stores, OAuth2 external-account linking, and HMAC callback authentication. The
contracts for those live in the referenced docs.

---

*Made with love ♥ by Helix Development.*
