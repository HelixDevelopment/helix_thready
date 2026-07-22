package userservice

import (
	"errors"
	"time"
)

// Role is one of the three fixed tiers in the Helix Thready hierarchy.
//
// The hierarchy is fixed (research_request_final §6.1): exactly one Root Admin,
// per-account Account Admins, and Standard Users with consumer access.
type Role string

const (
	// RoleRoot has full control of every account and user (cross-tenant). Exactly one exists.
	RoleRoot Role = "root_admin"
	// RoleAccountAdmin has full control of its own account and that account's users.
	RoleAccountAdmin Role = "account_admin"
	// RoleUser has consumer access to the accounts it is a member of.
	RoleUser Role = "user"
)

// rank returns the ordinal used for role-floor comparisons. Higher outranks lower.
func (r Role) rank() int {
	switch r {
	case RoleRoot:
		return 3
	case RoleAccountAdmin:
		return 2
	case RoleUser:
		return 1
	default:
		return 0
	}
}

// Valid reports whether r is one of the three known roles.
func (r Role) Valid() bool { return r.rank() > 0 }

// Permission is a fine-grained, orthogonal-to-role scope string. The catalog mirrors
// the OAuth2 scopes in openapi.yaml (authn-authz.md §7).
type Permission string

const (
	PermPostsRead     Permission = "posts:read"
	PermPostsWrite    Permission = "posts:write"
	PermAssetsRead    Permission = "assets:read"
	PermAssetsWrite   Permission = "assets:write"
	PermSearchRead    Permission = "search:read"
	PermSkillsRead    Permission = "skills:read"
	PermSkillsWrite   Permission = "skills:write"
	PermEventsRead    Permission = "events:read"
	PermAccountsAdmin Permission = "accounts:admin"
	PermBillingRead   Permission = "billing:read"
	PermRootAdmin     Permission = "root:admin"
)

// ScopeCatalog is the full set of known permissions (authn-authz.md §7).
var ScopeCatalog = []Permission{
	PermPostsRead, PermPostsWrite, PermAssetsRead, PermAssetsWrite,
	PermSearchRead, PermSkillsRead, PermSkillsWrite, PermEventsRead,
	PermAccountsAdmin, PermBillingRead, PermRootAdmin,
}

// Account is a tenant. Every domain row carries an account_id; every handler runs
// account-scoped queries (security-model.md §3).
type Account struct {
	ID        string
	Name      string
	CreatedAt time.Time
}

// Membership binds a user to an account with a role, and — for the user tier — the
// explicit set of permissions granted to that user within that account.
//
// Membership is many-to-many: a user may belong to multiple accounts with a
// different role in each (authn-authz.md §6, security-model.md §3).
type Membership struct {
	UserID    string
	AccountID string
	Role      Role
	// Scopes are the permissions explicitly assigned to a RoleUser member within
	// this account. They are ignored for admin tiers, whose grants derive from role.
	Scopes []Permission
}

// User is an authenticating principal. A user may be a member of several accounts.
type User struct {
	ID           string
	Email        string
	PasswordHash string
	// TOTPSecret is the base32-encoded shared secret; empty when MFA is not enrolled.
	TOTPSecret  string
	MFAEnabled  bool
	Memberships []Membership
	CreatedAt   time.Time
}

// ErrNoMembership is returned when a user has no membership in the requested account.
var ErrNoMembership = errors.New("userservice: user has no membership in account")

// IsRoot reports whether the user holds the global root_admin role in any membership.
// A root admin is global and may cross tenants.
func (u User) IsRoot() bool {
	for _, m := range u.Memberships {
		if m.Role == RoleRoot {
			return true
		}
	}
	return false
}

// MembershipIn returns the user's membership in accountID, if any.
func (u User) MembershipIn(accountID string) (Membership, bool) {
	for _, m := range u.Memberships {
		if m.AccountID == accountID {
			return m, true
		}
	}
	return Membership{}, false
}

// RoleIn resolves the user's role in a specific account. This is the multi-account
// resolver: the same user can be account_admin in one account and user in another.
//
// A global root admin resolves to RoleRoot for every account.
func (u User) RoleIn(accountID string) (Role, error) {
	if u.IsRoot() {
		return RoleRoot, nil
	}
	if m, ok := u.MembershipIn(accountID); ok {
		return m.Role, nil
	}
	return "", ErrNoMembership
}

// AccountIDs returns the set of account IDs the user belongs to (order-preserving).
func (u User) AccountIDs() []string {
	ids := make([]string, 0, len(u.Memberships))
	for _, m := range u.Memberships {
		ids = append(ids, m.AccountID)
	}
	return ids
}

// HasScopeIn reports whether a RoleUser membership was granted a specific permission
// within an account. Admin tiers do not use this (their grants come from role).
func (m Membership) HasScopeIn(p Permission) bool {
	for _, s := range m.Scopes {
		if s == p {
			return true
		}
	}
	return false
}
