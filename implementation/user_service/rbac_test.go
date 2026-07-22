package userservice

import "testing"

// Test fixtures: three accounts and users at each tier.
const (
	acctA = "acct-A"
	acctB = "acct-B"
)

func rootUser() User {
	return User{ID: "u-root", Email: "root@thready", Memberships: []Membership{
		{UserID: "u-root", AccountID: "system", Role: RoleRoot},
	}}
}

func adminOfA() User {
	return User{ID: "u-admin-a", Email: "admin-a@thready", Memberships: []Membership{
		{UserID: "u-admin-a", AccountID: acctA, Role: RoleAccountAdmin},
	}}
}

func userOfA() User {
	return User{ID: "u-user-a", Email: "user-a@thready", Memberships: []Membership{
		{UserID: "u-user-a", AccountID: acctA, Role: RoleUser, Scopes: []Permission{PermPostsRead, PermAssetsRead}},
	}}
}

func TestRBAC_RootSeesAll(t *testing.T) {
	e := NewEnforcer()
	root := rootUser()
	// Root is allowed every permission across every account, including cross-tenant.
	for _, acct := range []string{acctA, acctB, "any-other"} {
		for _, p := range ScopeCatalog {
			if !e.Allow(root, acct, p) {
				t.Fatalf("root must be allowed %s in %s", p, acct)
			}
		}
	}
}

func TestRBAC_AccountAdminOwnAccountOnly(t *testing.T) {
	e := NewEnforcer()
	admin := adminOfA()

	// Allowed in own account (account-tier permissions).
	for _, p := range []Permission{PermPostsRead, PermPostsWrite, PermAssetsWrite, PermAccountsAdmin, PermBillingRead, PermSkillsWrite} {
		if !e.Allow(admin, acctA, p) {
			t.Fatalf("account admin must be allowed %s in own account", p)
		}
	}
	// Denied root-only permission even in own account.
	if e.Allow(admin, acctA, PermRootAdmin) {
		t.Fatal("account admin must NOT hold root:admin")
	}
	// Denied entirely in a different account (tenant isolation).
	for _, p := range []Permission{PermPostsRead, PermAccountsAdmin} {
		if e.Allow(admin, acctB, p) {
			t.Fatalf("account admin must be denied %s in a foreign account", p)
		}
	}
}

func TestRBAC_UserOnlyAssigned(t *testing.T) {
	e := NewEnforcer()
	u := userOfA()

	// Allowed only the explicitly-assigned scopes.
	if !e.Allow(u, acctA, PermPostsRead) {
		t.Fatal("user must be allowed its assigned posts:read")
	}
	if !e.Allow(u, acctA, PermAssetsRead) {
		t.Fatal("user must be allowed its assigned assets:read")
	}
	// Denied unassigned scopes.
	if e.Allow(u, acctA, PermPostsWrite) {
		t.Fatal("user must be denied unassigned posts:write")
	}
	// Denied admin-tier permission regardless of assignment (role floor).
	if e.Allow(u, acctA, PermAccountsAdmin) {
		t.Fatal("user must be denied accounts:admin (role floor)")
	}
	// Denied in a foreign account.
	if e.Allow(u, acctB, PermPostsRead) {
		t.Fatal("user must be denied in a foreign account")
	}
}

func TestRBAC_CrossAccountDenied(t *testing.T) {
	// Mirrors security-model.md §11 RED skeleton: user of account B must be forbidden a
	// resource owned by account A.
	e := NewEnforcer()
	userB := User{ID: "u-b", Memberships: []Membership{
		{UserID: "u-b", AccountID: acctB, Role: RoleUser, Scopes: []Permission{PermPostsWrite}},
	}}
	if e.Allow(userB, acctA, PermPostsWrite) {
		t.Fatal("cross-account access must be denied")
	}
	if got := e.Evaluate(userB, acctA, PermPostsWrite); got != DecisionDeny {
		t.Fatalf("cross-account Evaluate = %s, want deny", got)
	}
}

func TestRBAC_RoleFloorInheritance(t *testing.T) {
	// account_admin and root inherit everything a user can do.
	if !CheckRoleFloor(RoleAccountAdmin, RoleUser) {
		t.Fatal("account_admin must meet the user floor")
	}
	if !CheckRoleFloor(RoleRoot, RoleAccountAdmin) {
		t.Fatal("root must meet the account_admin floor")
	}
	// user does not meet the account_admin floor.
	if CheckRoleFloor(RoleUser, RoleAccountAdmin) {
		t.Fatal("user must NOT meet the account_admin floor")
	}
	// Unknown roles never satisfy a floor.
	if CheckRoleFloor(Role("bogus"), RoleUser) {
		t.Fatal("unknown role must never satisfy a floor")
	}
}

func TestMembership_MultiAccountRoleResolution(t *testing.T) {
	// A single user is account_admin of A and a plain user of B simultaneously.
	u := User{ID: "u-multi", Memberships: []Membership{
		{UserID: "u-multi", AccountID: acctA, Role: RoleAccountAdmin},
		{UserID: "u-multi", AccountID: acctB, Role: RoleUser, Scopes: []Permission{PermPostsRead}},
	}}

	if r, err := u.RoleIn(acctA); err != nil || r != RoleAccountAdmin {
		t.Fatalf("RoleIn(A) = %v, %v; want account_admin", r, err)
	}
	if r, err := u.RoleIn(acctB); err != nil || r != RoleUser {
		t.Fatalf("RoleIn(B) = %v, %v; want user", r, err)
	}
	if _, err := u.RoleIn("acct-unknown"); err == nil {
		t.Fatal("RoleIn on a non-member account must error")
	}

	e := NewEnforcer()
	// As admin of A: can administer A.
	if !e.Allow(u, acctA, PermAccountsAdmin) {
		t.Fatal("multi-account user must administer account A")
	}
	// As a plain user of B: cannot administer B, but can read its posts.
	if e.Allow(u, acctB, PermAccountsAdmin) {
		t.Fatal("multi-account user must NOT administer account B")
	}
	if !e.Allow(u, acctB, PermPostsRead) {
		t.Fatal("multi-account user must read posts in account B (assigned)")
	}
	if e.Allow(u, acctB, PermPostsWrite) {
		t.Fatal("multi-account user must not have unassigned posts:write in B")
	}
}

func TestRole_Valid(t *testing.T) {
	for _, r := range []Role{RoleRoot, RoleAccountAdmin, RoleUser} {
		if !r.Valid() {
			t.Fatalf("%s should be a valid role", r)
		}
	}
	if Role("nope").Valid() {
		t.Fatal("unknown role must be invalid")
	}
}
