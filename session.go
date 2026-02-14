// Package testutils provides utilities for testing web APIs.
package testutils

import (
	"crypto/rand"
	"encoding/base64"
	"net/http"
	"sync"
	"time"
)

// Session represents a user session with an ID, arbitrary data, and expiration.
type Session struct {
	ID        string
	Data      map[string]interface{}
	CreatedAt time.Time
	ExpiresAt time.Time
}

// IsExpired checks whether the session has expired.
func (s *Session) IsExpired() bool {
	return !s.ExpiresAt.IsZero() && s.ExpiresAt.Before(time.Now())
}

// SessionStore is a simple in‑memory store for sessions.
// It is safe for concurrent use.
type SessionStore struct {
	mu       sync.RWMutex
	sessions map[string]*Session
}

// NewSessionStore creates a new empty session store.
func NewSessionStore() *SessionStore {
	return &SessionStore{
		sessions: make(map[string]*Session),
	}
}

// generateSessionID creates a new random URL‑safe session ID.
func generateSessionID() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

// NewSession creates a new session with a random ID and optional expiration.
// If expiresIn is zero, the session does not expire.
func (s *SessionStore) NewSession(expiresIn time.Duration) (*Session, error) {
	id, err := generateSessionID()
	if err != nil {
		return nil, err
	}
	sess := &Session{
		ID:        id,
		Data:      make(map[string]interface{}),
		CreatedAt: time.Now(),
	}
	if expiresIn > 0 {
		sess.ExpiresAt = sess.CreatedAt.Add(expiresIn)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions[id] = sess
	return sess, nil
}

// Get retrieves a session by ID. Returns nil if the session does not exist or has expired.
func (s *SessionStore) Get(id string) *Session {
	s.mu.RLock()
	defer s.mu.RUnlock()
	sess, ok := s.sessions[id]
	if !ok {
		return nil
	}
	if sess.IsExpired() {
		// Do not return expired sessions; caller can delete it if desired.
		return nil
	}
	return sess
}

// Set stores or updates a session.
func (s *SessionStore) Set(sess *Session) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions[sess.ID] = sess
}

// Delete removes a session by ID.
func (s *SessionStore) Delete(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sessions, id)
}

// Cleanup removes all expired sessions.
func (s *SessionStore) Cleanup() {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	for id, sess := range s.sessions {
		if !sess.ExpiresAt.IsZero() && sess.ExpiresAt.Before(now) {
			delete(s.sessions, id)
		}
	}
}

// SetSessionCookie attaches a session cookie to the response.
// The cookie is HTTP‑only and uses the given name and session ID.
// Use http.Cookie fields to customise path, domain, secure flag, etc.
func SetSessionCookie(w http.ResponseWriter, name, sessionID string) {
	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Value:    sessionID,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		// MaxAge:   86400, // optionally set cookie lifetime
	})
}

// GetSessionIDFromRequest extracts the session ID from a cookie with the given name.
// Returns an empty string if the cookie is not present or invalid.
func GetSessionIDFromRequest(r *http.Request, cookieName string) string {
	cookie, err := r.Cookie(cookieName)
	if err != nil {
		return ""
	}
	return cookie.Value
}