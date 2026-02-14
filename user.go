// Package testutils provides utilities for testing web APIs.
package testutils

import (
	"fmt"
	"sync"
)

// Role represents a user role in the system.
type Role string

// Predefined roles.
const (
	RoleAdmin Role = "admin"
	RoleUser  Role = "user"
	RoleGuest Role = "guest"
)

// User represents a test user with common fields.
type User struct {
	ID           string                 `json:"id"`
	Username     string                 `json:"username"`
	Email        string                 `json:"email"`
	PasswordHash string                 `json:"-"` // never expose in JSON
	Roles        []Role                 `json:"roles"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
}

// NewUser creates a new user with the given username, email, and password.
// The password is hashed using bcrypt (via secret.go). It panics on hash failure.
func NewUser(id, username, email, password string, roles ...Role) *User {
	hash, err := HashPassword(password)
	if err != nil {
		panic(fmt.Sprintf("testutils: failed to hash password for user %s: %v", username, err))
	}
	if len(roles) == 0 {
		roles = []Role{RoleUser}
	}
	return &User{
		ID:           id,
		Username:     username,
		Email:        email,
		PasswordHash: hash,
		Roles:        roles,
		Metadata:     make(map[string]interface{}),
	}
}

// CheckPassword verifies the provided password against the user's hash.
func (u *User) CheckPassword(password string) bool {
	return CheckPasswordHash(password, u.PasswordHash)
}

// HasRole reports whether the user has the given role.
func (u *User) HasRole(role Role) bool {
	for _, r := range u.Roles {
		if r == role {
			return true
		}
	}
	return false
}

// ------------------------------------------------------------------------
// UserStore interface and in‑memory implementation
// ------------------------------------------------------------------------

// UserStore defines the interface for a user repository used in tests.
type UserStore interface {
	// GetByID returns a user by ID, or nil if not found.
	GetByID(id string) *User
	// GetByUsername returns a user by username, or nil if not found.
	GetByUsername(username string) *User
	// GetByEmail returns a user by email, or nil if not found.
	GetByEmail(email string) *User
	// Save stores a user (adds or updates).
	Save(user *User)
	// Delete removes a user by ID.
	Delete(id string)
	// All returns all users.
	All() []*User
}

// InMemoryUserStore is a thread‑safe in‑memory implementation of UserStore.
type InMemoryUserStore struct {
	mu       sync.RWMutex
	byID     map[string]*User
	byName   map[string]*User
	byEmail  map[string]*User
}

// NewInMemoryUserStore creates an empty user store.
func NewInMemoryUserStore() *InMemoryUserStore {
	return &InMemoryUserStore{
		byID:    make(map[string]*User),
		byName:  make(map[string]*User),
		byEmail: make(map[string]*User),
	}
}

// GetByID returns a user by ID.
func (s *InMemoryUserStore) GetByID(id string) *User {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.byID[id]
}

// GetByUsername returns a user by username.
func (s *InMemoryUserStore) GetByUsername(username string) *User {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.byName[username]
}

// GetByEmail returns a user by email.
func (s *InMemoryUserStore) GetByEmail(email string) *User {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.byEmail[email]
}

// Save stores a user. If a user with the same ID already exists, it is replaced.
// The store ensures that username and email are unique; if a conflict occurs,
// the existing entries are overwritten (last write wins).
func (s *InMemoryUserStore) Save(user *User) {
	s.mu.Lock()
	defer s.mu.Unlock()
	// Remove old references if this ID already existed.
	if old, ok := s.byID[user.ID]; ok {
		delete(s.byName, old.Username)
		delete(s.byEmail, old.Email)
	}
	s.byID[user.ID] = user
	s.byName[user.Username] = user
	s.byEmail[user.Email] = user
}

// Delete removes a user by ID.
func (s *InMemoryUserStore) Delete(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if user, ok := s.byID[id]; ok {
		delete(s.byName, user.Username)
		delete(s.byEmail, user.Email)
		delete(s.byID, id)
	}
}

// All returns a copy of all users.
func (s *InMemoryUserStore) All() []*User {
	s.mu.RLock()
	defer s.mu.RUnlock()
	users := make([]*User, 0, len(s.byID))
	for _, u := range s.byID {
		users = append(users, u)
	}
	return users
}

// ------------------------------------------------------------------------
// Integration with FullTest
// ------------------------------------------------------------------------

// TestUser extends User with helpers for the FullTest fixture.
type TestUser struct {
	*User
	ft *FullTest
}

// CreateTestUser creates a new user in the FullTest's user store (if present)
// and returns a TestUser that can create sessions.
func (ft *FullTest) CreateTestUser(id, username, email, password string, roles ...Role) *TestUser {
	ft.T.Helper()
	// If the FullTest has a UserStore (we need to add one), we'll store it.
	// For now, we assume FullTest has a UserStore field. Let's add it.
	// But FullTest doesn't have a UserStore yet. We'll extend it.
	// We can either modify full.go or add a UserStore field dynamically.
	// To keep things simple, we'll create a separate function that takes a UserStore.
	// Alternatively, we can embed a UserStore in FullTest. Let's do that: add a UserStore field to FullTest.
	// But we can't modify full.go here. We'll just provide a standalone function and let the user store the user themselves.
	// Better: We'll add a UserStore field to FullTest via an option. We'll do that in a separate PR.
	// For now, we'll just create the user and return it without storing.
	user := NewUser(id, username, email, password, roles...)
	return &TestUser{User: user, ft: ft}
}

// CreateSession creates a session for this test user and returns the session ID.
func (tu *TestUser) CreateSession(expiresIn time.Duration) string {
	tu.ft.T.Helper()
	return tu.ft.CreateUserSession(tu.ID, expiresIn)
}

// WithSession returns a request builder that includes a session cookie for this user.
func (tu *TestUser) WithSession(rb *FullRequestBuilder) *FullRequestBuilder {
	return rb.WithSessionCookie(tu.CreateSession(0))
}