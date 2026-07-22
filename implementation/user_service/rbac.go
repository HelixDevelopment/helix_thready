package userservice

// The RBAC enforcer implements the three-tier matrix (authn-authz.md §6,
// security-model.md §3). Two gates stand between a caller and a resource:
//
//  1. Tenant isolation: the target account must match the principal's account, unless
//     the principal is root_admin (which may cross tenants).
//  2. Role floor + grant: the principal's role must meet the permission's role floor,
//     and — for the user tier — the permission must be explicitly assigned in that
//     account's membership.
//
// Summarised: root sees all; account_admin only its own account; user only what it was
// assigned. Decisions follow the documented most-restrictive Deny > Audit > Allow
// ordering of security/pkg/policy.

// Decision is the outcome of an authorization evaluation.
type Decision string

const (
	// DecisionAllow permits the action.
	DecisionAllow Decision = "allow"
	// DecisionDeny forbids the action.
	DecisionDeny Decision = "deny"
	// DecisionAudit permits the action but flags it for the audit trail.
	DecisionAudit Decision = "audit"
)

// permissionFloor maps a permission to the minimum role required to hold it. Any
// permission not listed defaults to RoleUser (the lowest authenticated floor).
var permissionFloor = map[Permission]Role{
	PermAccountsAdmin: RoleAccountAdmin,
	PermBillingRead:   RoleAccountAdmin,
	PermRootAdmin:     RoleRoot,
}

// floorOf returns the role floor for a permission.
func floorOf(p Permission) Role {
	if r, ok := permissionFloor[p]; ok {
		return r
	}
	return RoleUser
}

// Enforcer decides whether a principal may perform an action in an account.
type Enforcer struct{}

// NewEnforcer returns a ready enforcer.
func NewEnforcer() *Enforcer { return &Enforcer{} }

// Evaluate returns the full decision for (user, targetAccount, permission).
//
// Rules, in order:
//   - root_admin (global): Allow anything, any account.
//   - not a member of targetAccount: Deny (tenant isolation).
//   - role does not meet the permission's floor: Deny.
//   - account_admin (floor satisfied): Allow (full control of its own account).
//   - user (floor satisfied): Allow iff the permission was explicitly assigned in the
//     membership; otherwise Deny.
func (e *Enforcer) Evaluate(u User, targetAccount string, p Permission) Decision {
	if u.IsRoot() {
		return DecisionAllow
	}

	m, ok := u.MembershipIn(targetAccount)
	if !ok {
		// Not a member of the target tenant. Cross-account access is denied before any
		// existence check, so a probe cannot distinguish "not yours" from "not found".
		return DecisionDeny
	}

	floor := floorOf(p)
	if m.Role.rank() < floor.rank() {
		return DecisionDeny
	}

	switch m.Role {
	case RoleAccountAdmin:
		// Account admins have full control of their own account. root:admin has a root
		// floor and was already denied above.
		return DecisionAllow
	case RoleUser:
		if m.HasScopeIn(p) {
			return DecisionAllow
		}
		return DecisionDeny
	default:
		return DecisionDeny
	}
}

// Allow is the terse boolean form of Evaluate (Allow or Audit both permit the action).
func (e *Enforcer) Allow(u User, targetAccount string, p Permission) bool {
	d := e.Evaluate(u, targetAccount, p)
	return d == DecisionAllow || d == DecisionAudit
}

// CheckRoleFloor reports whether have meets the required role floor (need). It encodes
// the inheritance semantics from authn-authz.md §6: account_admin and root inherit
// everything a user can do, root inherits everything account_admin can do.
func CheckRoleFloor(have, need Role) bool {
	return have.rank() >= need.rank() && need.rank() > 0
}
