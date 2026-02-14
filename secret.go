// Package testutils provides utilities for testing web APIs.
package testutils

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"os"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

// GenerateRandomBytes returns securely generated random bytes.
// It returns an error if the system's secure random number generator fails.
func GenerateRandomBytes(n int) ([]byte, error) {
	b := make([]byte, n)
	_, err := rand.Read(b)
	if err != nil {
		return nil, err
	}
	return b, nil
}

// GenerateRandomString returns a URL-safe, base64-encoded securely generated random string.
// The length of the resulting string is approximately 4/3 of n bytes.
func GenerateRandomString(n int) (string, error) {
	b, err := GenerateRandomBytes(n)
	if err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

// GenerateRandomHex returns a hex-encoded random string of length 2*n (each byte becomes two hex chars).
func GenerateRandomHex(n int) (string, error) {
	b, err := GenerateRandomBytes(n)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// HashPassword returns a bcrypt hash of the password with a default cost.
func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(bytes), err
}

// CheckPasswordHash compares a password with a bcrypt hash and returns true if they match.
func CheckPasswordHash(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

// SignHMAC creates an HMAC-SHA256 signature of the message using the given secret.
// Returns the signature as a base64-URL encoded string.
func SignHMAC(message, secret string) string {
	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(message))
	return base64.URLEncoding.EncodeToString(h.Sum(nil))
}

// VerifyHMAC checks whether the provided signature is valid for the message.
func VerifyHMAC(message, secret, signature string) bool {
	expected := SignHMAC(message, secret)
	return hmac.Equal([]byte(signature), []byte(expected))
}

// GetEnvSecret retrieves a secret from an environment variable.
// If the variable is not set or empty, it returns the provided fallback value.
// This is useful for tests that can use default secrets but allow overriding.
func GetEnvSecret(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

// ConstantTimeCompare performs a constant-time comparison of two strings.
// Useful for comparing secrets to avoid timing attacks.
func ConstantTimeCompare(a, b string) bool {
	return hmac.Equal([]byte(a), []byte(b))
}

// MaskSecret masks a secret by showing only the first and last few characters.
// Useful for logging without revealing the entire secret.
func MaskSecret(secret string, visibleChars int) string {
	if len(secret) <= visibleChars*2 {
		return strings.Repeat("*", len(secret))
	}
	prefix := secret[:visibleChars]
	suffix := secret[len(secret)-visibleChars:]
	return prefix + strings.Repeat("*", len(secret)-visibleChars*2) + suffix
}