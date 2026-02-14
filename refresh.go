// Package testutils provides utilities for testing web APIs.
package testutils

import (
	"sync"
	"time"
)

// RefreshToken represents a refresh token with its associated user ID and expiration.
type RefreshToken struct {
	Token     string
	UserID    string
	CreatedAt time.Time
	ExpiresAt time.Time
}

// IsExpired checks whether the refresh token has expired.
func (rt *RefreshToken) IsExpired() bool {
	return !rt.ExpiresAt.IsZero() && rt.ExpiresAt.Before(time.Now())
}

// RefreshTokenStore is a thread-safe in-memory store for refresh tokens.
type RefreshTokenStore struct {
	mu      sync.RWMutex
	tokens  map[string]*RefreshToken
	userIdx map[string][]string // userID -> list of token strings
}

// NewRefreshTokenStore creates a new empty refresh token store.
func NewRefreshTokenStore() *RefreshTokenStore {
	return &RefreshTokenStore{
		tokens:  make(map[string]*RefreshToken),
		userIdx: make(map[string][]string),
	}
}

// Store saves a refresh token. If a token with the same string already exists, it is overwritten.
func (s *RefreshTokenStore) Store(rt *RefreshToken) {
	s.mu.Lock()
	defer s.mu.Unlock()
	// Remove old token if it existed
	if old, ok := s.tokens[rt.Token]; ok {
		s.removeUserIndex(old.UserID, rt.Token)
	}
	s.tokens[rt.Token] = rt
	s.userIdx[rt.UserID] = append(s.userIdx[rt.UserID], rt.Token)
}

// Get retrieves a refresh token by its token string. Returns nil if not found or expired.
// If the token is expired, it is automatically deleted.
func (s *RefreshTokenStore) Get(token string) *RefreshToken {
	s.mu.Lock()
	defer s.mu.Unlock()
	rt, ok := s.tokens[token]
	if !ok {
		return nil
	}
	if rt.IsExpired() {
		s.deleteLocked(token)
		return nil
	}
	return rt
}

// Delete removes a refresh token.
func (s *RefreshTokenStore) Delete(token string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.deleteLocked(token)
}

// deleteLocked assumes the lock is already held.
func (s *RefreshTokenStore) deleteLocked(token string) {
	rt, ok := s.tokens[token]
	if !ok {
		return
	}
	delete(s.tokens, token)
	s.removeUserIndex(rt.UserID, token)
}

// removeUserIndex removes a token from the user index slice.
func (s *RefreshTokenStore) removeUserIndex(userID, token string) {
	tokens := s.userIdx[userID]
	for i, t := range tokens {
		if t == token {
			s.userIdx[userID] = append(tokens[:i], tokens[i+1:]...)
			break
		}
	}
	if len(s.userIdx[userID]) == 0 {
		delete(s.userIdx, userID)
	}
}

// CleanupExpired removes all expired tokens from the store.
func (s *RefreshTokenStore) CleanupExpired() {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	for token, rt := range s.tokens {
		if !rt.ExpiresAt.IsZero() && rt.ExpiresAt.Before(now) {
			s.deleteLocked(token)
		}
	}
}

// RevokeAllForUser removes all refresh tokens belonging to a given user.
func (s *RefreshTokenStore) RevokeAllForUser(userID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, token := range s.userIdx[userID] {
		delete(s.tokens, token)
	}
	delete(s.userIdx, userID)
}

// ------------------------------------------------------------------------
// Helpers for generating test tokens
// ------------------------------------------------------------------------

// GenerateTestRefreshToken creates a new refresh token with the given user ID and expiration.
// The token string is generated using a random hex string of the specified length.
// Note: This function does NOT store the token; you must call Store separately.
func GenerateTestRefreshToken(userID string, expiresIn time.Duration, tokenLength int) (*RefreshToken, error) {
	token, err := GenerateRandomHex(tokenLength) // from secret.go
	if err != nil {
		return nil, err
	}
	now := time.Now()
	rt := &RefreshToken{
		Token:     token,
		UserID:    userID,
		CreatedAt: now,
	}
	if expiresIn > 0 {
		rt.ExpiresAt = now.Add(expiresIn)
	}
	return rt, nil
}