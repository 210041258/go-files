// Package testutils provides utilities for testing web APIs.
package testutils

import (
	"sync"
	"testing"
	"time"
)

// ------------------------------------------------------------------------
// Manager – unified test fixture for user and session management
// ------------------------------------------------------------------------

// Manager is a test fixture that manages users, sessions, and time.
// It does not include an HTTP server; for HTTP tests, see FullTest.
type Manager struct {
	t            TestingT
	clock        Clock
	userStore    UserStore
	sessionStore *SessionStore
	cleanup      *Cleanup
	mu           sync.Mutex
}

// ManagerOption configures a Manager.
type ManagerOption func(*Manager)

// WithManagerClock sets the clock used by the manager.
func WithManagerClock(clock Clock) ManagerOption {
	return func(m *Manager) {
		m.clock = clock
	}
}

// WithManagerUserStore sets the user store used by the manager.
// If store is nil, a new InMemoryUserStore is created.
func WithManagerUserStore(store UserStore) ManagerOption {
	return func(m *Manager) {
		if store != nil {
			m.userStore = store
		}
	}
}

// WithManagerSessionStore sets the session store used by the manager.
// If store is nil, a new SessionStore is created.
func WithManagerSessionStore(store *SessionStore) ManagerOption {
	return func(m *Manager) {
		if store != nil {
			m.sessionStore = store
		}
	}
}

// NewManager creates a new Manager with sensible defaults.
// It automatically registers a cleanup that runs the Manager's Close method.
func NewManager(t TestingT, opts ...ManagerOption) *Manager {
	t.Helper()
	m := &Manager{
		t:            t,
		clock:        RealClock{},
		userStore:    NewInMemoryUserStore(),
		sessionStore: NewSessionStore(),
		cleanup:      &Cleanup{},
	}
	for _, opt := range opts {
		opt(m)
	}
	m.cleanup.Add(func() {
		// No explicit close needed for in‑memory stores, but we keep the hook.
	})
	m.cleanup.Defer(t)
	return m
}

// ------------------------------------------------------------------------
// User management
// ------------------------------------------------------------------------

// CreateUser creates a new user with the given details, stores it in the
// user store, and returns the user.
func (m *Manager) CreateUser(id, username, email, password string, roles ...Role) *User {
	m.t.Helper()
	user := NewUser(id, username, email, password, roles...)
	m.userStore.Save(user)
	return user
}

// CreateAdminUser creates a new user with the admin role.
func (m *Manager) CreateAdminUser(id, username, email, password string) *User {
	return m.CreateUser(id, username, email, password, RoleAdmin)
}

// CreateGuestUser creates a new user with the guest role.
func (m *Manager) CreateGuestUser(id, username, email, password string) *User {
	return m.CreateUser(id, username, email, password, RoleGuest)
}

// GetUser retrieves a user by ID. Returns nil if not found.
func (m *Manager) GetUser(id string) *User {
	return m.userStore.GetByID(id)
}

// AllUsers returns a slice of all stored users.
func (m *Manager) AllUsers() []*User {
	return m.userStore.All()
}

// DeleteUser removes a user by ID.
func (m *Manager) DeleteUser(id string) {
	m.userStore.Delete(id)
}

// ------------------------------------------------------------------------
// Session management
// ------------------------------------------------------------------------

// CreateSessionForUser creates a new session for the given user ID and
// returns the session ID. If expiresIn > 0, the session will expire after
// that duration (using the manager's clock). It panics if the user does not exist.
func (m *Manager) CreateSessionForUser(userID string, expiresIn time.Duration) string {
	m.t.Helper()
	user := m.userStore.GetByID(userID)
	if user == nil {
		m.t.Fatalf("CreateSessionForUser: user %s not found", userID)
	}
	sess, err := m.sessionStore.NewSession(expiresIn)
	if err != nil {
		m.t.Fatalf("CreateSessionForUser: %v", err)
	}
	sess.Data["user_id"] = userID
	m.sessionStore.Set(sess)
	return sess.ID
}

// GetSession returns a session by ID, or nil if not found/expired.
func (m *Manager) GetSession(sessionID string) *Session {
	return m.sessionStore.Get(sessionID)
}

// DeleteSession removes a session.
func (m *Manager) DeleteSession(sessionID string) {
	m.sessionStore.Delete(sessionID)
}

// CleanupSessions removes all expired sessions.
func (m *Manager) CleanupSessions() {
	m.sessionStore.Cleanup()
}

// ------------------------------------------------------------------------
// Clock access
// ------------------------------------------------------------------------

// Now returns the current time according to the manager's clock.
func (m *Manager) Now() time.Time {
	return m.clock.Now()
}

// AdvanceTime advances the mock clock by d (if the clock is mockable).
// If the clock is not a *MockClock, it does nothing.
func (m *Manager) AdvanceTime(d time.Duration) {
	if mc, ok := m.clock.(*MockClock); ok {
		mc.Advance(d)
	}
}

// ------------------------------------------------------------------------
// Cleanup
// ------------------------------------------------------------------------

// Close releases any resources held by the manager. It is called automatically
// when the test ends if NewManager was used. It may be called manually.
func (m *Manager) Close() {
	// Nothing to close for in‑memory stores, but we keep the method for symmetry.
}

// ------------------------------------------------------------------------
// Must variants
// ------------------------------------------------------------------------

// MustCreateUser is like CreateUser but panics on error (though CreateUser does not error).
// It exists for consistency with other Must functions.
func (m *Manager) MustCreateUser(id, username, email, password string, roles ...Role) *User {
	return m.CreateUser(id, username, email, password, roles...)
}

// MustCreateSessionForUser is like CreateSessionForUser but panics on error.
func (m *Manager) MustCreateSessionForUser(userID string, expiresIn time.Duration) string {
	m.t.Helper()
	sessID := m.CreateSessionForUser(userID, expiresIn)
	return sessID
}