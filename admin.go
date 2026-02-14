// Package testutils provides utilities for testing web APIs.
package testutils

// IsAdmin reports whether the user has the admin role.
func IsAdmin(user *User) bool {
	return user.HasRole(RoleAdmin)
}

// PromoteToAdmin adds the admin role to the user. If the user already has the
// admin role, it does nothing.
func PromoteToAdmin(user *User) {
	for _, r := range user.Roles {
		if r == RoleAdmin {
			return
		}
	}
	user.Roles = append(user.Roles, RoleAdmin)
}

// WithAdminSession adds an admin session cookie to the request builder.
// It creates a session for the given user ID in the FullTest's session store
// and attaches it to the request. The user ID must correspond to a user that
// is an admin (the test is responsible for ensuring that).
func WithAdminSession(rb *FullRequestBuilder, ft *FullTest, userID string) *FullRequestBuilder {
	sessionID := ft.CreateUserSession(userID, 0)
	return rb.WithSessionCookie(sessionID)
}

// RequireAdmin fails the test if the user does not have the admin role.
func RequireAdmin(t TestingT, user *User) {
	t.Helper()
	if !IsAdmin(user) {
		t.Fatal("testutils: user is not an admin")
	}
}