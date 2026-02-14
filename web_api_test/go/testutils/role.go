// Package testutils provides utilities for testing web APIs.
package testutils

// Role represents a user role in the system.
type Role string

// Predefined roles.
const (
	RoleAdmin   Role = "admin"
	RoleUser    Role = "user"
	RoleGuest   Role = "guest"
	RoleManager Role = "manager"
	RoleEditor  Role = "editor"
	RoleViewer  Role = "viewer"
)

// String returns the string representation of the role.
func (r Role) String() string {
	return string(r)
}

// ------------------------------------------------------------------------
// Role list helpers
// ------------------------------------------------------------------------

// Roles is a slice of Role with convenience methods.
type Roles []Role

// Contains reports whether the list contains the given role.
func (rs Roles) Contains(role Role) bool {
	for _, r := range rs {
		if r == role {
			return true
		}
	}
	return false
}

// Add appends a role if it is not already present. It returns a new slice.
func (rs Roles) Add(role Role) Roles {
	if rs.Contains(role) {
		return rs
	}
	return append(rs, role)
}

// Remove removes all occurrences of the given role. It returns a new slice.
func (rs Roles) Remove(role Role) Roles {
	result := make(Roles, 0, len(rs))
	for _, r := range rs {
		if r != role {
			result = append(result, r)
		}
	}
	return result
}

// ------------------------------------------------------------------------
// User role helpers (work with the User struct from user.go)
// ------------------------------------------------------------------------

// HasRole reports whether the user has the given role.
// This is a convenience wrapper around User.HasRole.
func HasRole(user *User, role Role) bool {
	if user == nil {
		return false
	}
	return user.HasRole(role)
}

// AddRole adds a role to the user if not already present.
// It modifies the user's Roles slice.
func AddRole(user *User, role Role) {
	if user == nil {
		return
	}
	for _, r := range user.Roles {
		if r == role {
			return
		}
	}
	user.Roles = append(user.Roles, role)
}

// RemoveRole removes all occurrences of the given role from the user.
func RemoveRole(user *User, role Role) {
	if user == nil {
		return
	}
	newRoles := make([]Role, 0, len(user.Roles))
	for _, r := range user.Roles {
		if r != role {
			newRoles = append(newRoles, r)
		}
	}
	user.Roles = newRoles
}

// RequireRole fails the test if the user does not have the given role.
func RequireRole(t TestingT, user *User, role Role) {
	t.Helper()
	if user == nil {
		t.Fatalf("RequireRole: user is nil, expected role %s", role)
	}
	if !HasRole(user, role) {
		t.Fatalf("RequireRole: user %s does not have role %s (has %v)", user.ID, role, user.Roles)
	}
}

// RequireAnyRole fails the test if the user does not have at least one of the given roles.
func RequireAnyRole(t TestingT, user *User, roles ...Role) {
	t.Helper()
	if user == nil {
		t.Fatalf("RequireAnyRole: user is nil")
	}
	for _, role := range roles {
		if HasRole(user, role) {
			return
		}
	}
	t.Fatalf("RequireAnyRole: user %s has none of the roles %v (has %v)", user.ID, roles, user.Roles)
}

// ------------------------------------------------------------------------
// RoleSet – a set of roles for efficient membership checks
// ------------------------------------------------------------------------

// RoleSet is a set of roles, useful for caching and repeated checks.
type RoleSet map[Role]struct{}

// NewRoleSet creates a RoleSet from a slice of roles.
func NewRoleSet(roles []Role) RoleSet {
	rs := make(RoleSet, len(roles))
	for _, r := range roles {
		rs[r] = struct{}{}
	}
	return rs
}

// Has reports whether the set contains the given role.
func (rs RoleSet) Has(role Role) bool {
	_, ok := rs[role]
	return ok
}

// Add inserts a role into the set.
func (rs RoleSet) Add(role Role) {
	rs[role] = struct{}{}
}

// Remove deletes a role from the set.
func (rs RoleSet) Remove(role Role) {
	delete(rs, role)
}