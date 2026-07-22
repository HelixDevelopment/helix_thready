// Package userservice implements the Helix Thready User Service: a self-contained,
// stdlib-only, three-tier multi-tenant RBAC identity core.
//
// It provides the primitives required by the intended contract in
// docs/public/research/mvp/api/authn-authz.md and
// docs/public/research/mvp/architecture/security-model.md:
//
//   - Models: User, Account, Role, Permission, Membership (multi-account).
//   - Password hashing: PBKDF2-HMAC-SHA256 with per-hash salt + constant-time verify.
//   - JWT: HS256 (default) and RS256, access + refresh, validate, refresh-rotate,
//     revoke via a token store, with algorithm-confusion protection.
//   - API keys: prefixed generation, scopes, verify + scope check, masking,
//     scope-subset enforcement.
//   - RBAC enforcer: three-tier (root_admin / account_admin / user) matrix with
//     tenant isolation and per-permission role floors.
//   - TOTP: RFC 6238 provisioning + Verify, validated against published RFC test vectors.
//
// The package intentionally depends only on the Go standard library so it compiles
// and tests green with zero network access and no third-party modules.
package userservice
