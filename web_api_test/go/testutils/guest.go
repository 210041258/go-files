// Package testutils provides utilities for testing web APIs.
package testutils

// NewGuestUser creates a new user with the guest role.
// It is a convenience wrapper around NewUser that sets the role to RoleGuest.
func NewGuestUser(id, username, email, password string) *User {
	return NewUser(id, username, email, password, RoleGuest)
}

// IsGuest reports whether the user has the guest role.
func IsGuest(user *User) bool {
	return user.HasRole(RoleGuest)
}

// WithGuestSession adds a guest session cookie to the request builder.
// It creates a session for the given user ID in the FullTest's session store
// and attaches it to the request. The user ID should correspond to a guest user.
func WithGuestSession(rb *FullRequestBuilder, ft *FullTest, userID string) *FullRequestBuilder {
	sessionID := ft.CreateUserSession(userID, 0)
	return rb.WithSessionCookie(sessionID)
}

// RequireGuest fails the test if the user does not have the guest role.
func RequireGuest(t TestingT, user *User) {
	t.Helper()
	if !IsGuest(user) {
		t.Fatal("testutils: user is not a guest")
	}
}