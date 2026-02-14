// Package token provides utilities for creating and validating JSON Web Tokens (JWT)
// and generating cryptographically secure random tokens (e.g., API keys).
package testutils

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Common errors returned by the package.
var (
	ErrInvalidToken     = errors.New("invalid token")
	ErrExpiredToken     = errors.New("token has expired")
	ErrInvalidSigningMethod = errors.New("unexpected signing method")
)

// ----------------------------------------------------------------------
// JWT support
// ----------------------------------------------------------------------

// Claims defines the custom claims structure for JWT.
// It embeds jwt.RegisteredClaims for standard fields (exp, iat, etc.).
type Claims struct {
	UserID string `json:"user_id,omitempty"`
	Role   string `json:"role,omitempty"`
	jwt.RegisteredClaims
}

// SigningKey represents the key material used to sign tokens.
// It can be a symmetric key (HMAC) or an RSA private key.
type SigningKey struct {
	Secret   []byte           // for HMAC
	Private  *rsa.PrivateKey  // for RSA
	Method   jwt.SigningMethod
}

// NewHMACSigningKey creates a signing key for HMAC (HS256, HS384, HS512).
func NewHMACSigningKey(secret []byte, method *jwt.SigningMethodHMAC) *SigningKey {
	return &SigningKey{
		Secret: secret,
		Method: method,
	}
}

// NewRSASigningKey creates a signing key for RSA (RS256, RS384, RS512).
func NewRSASigningKey(private *rsa.PrivateKey, method *jwt.SigningMethodRSA) *SigningKey {
	return &SigningKey{
		Private: private,
		Method:  method,
	}
}

// GenerateJWT creates a new JWT token with the given claims and signing key.
// The token is signed and returned as a string.
func GenerateJWT(claims Claims, key *SigningKey) (string, error) {
	token := jwt.NewWithClaims(key.Method, claims)
	signed, err := token.SignedString(key.secretOrPrivate())
	if err != nil {
		return "", fmt.Errorf("sign token: %w", err)
	}
	return signed, nil
}

// ValidateJWT parses and validates a JWT token string using the provided signing key.
// It returns the parsed claims if valid, or an error.
func ValidateJWT(tokenString string, key *SigningKey) (*Claims, error) {
	// Parse token with custom claims.
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		// Validate the signing method.
		if token.Method != key.Method {
			return nil, ErrInvalidSigningMethod
		}
		return key.secretOrPublic(), nil
	})
	if err != nil {
		// Check for specific JWT errors.
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrExpiredToken
		}
		return nil, ErrInvalidToken
	}
	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}
	return nil, ErrInvalidToken
}

// secretOrPrivate returns the appropriate signing material.
func (k *SigningKey) secretOrPrivate() interface{} {
	if k.Private != nil {
		return k.Private
	}
	return k.Secret
}

// secretOrPublic returns the appropriate verification material.
func (k *SigningKey) secretOrPublic() interface{} {
	if k.Private != nil {
		return &k.Private.PublicKey
	}
	return k.Secret
}

// ----------------------------------------------------------------------
// Random token generation (e.g., API keys, refresh tokens)
// ----------------------------------------------------------------------

// RandomToken generates a cryptographically secure random token of the given byte length,
// encoded in URL‑safe base64 without padding. The resulting string length is approximately
// 4/3 * n bytes.
func RandomToken(byteLength int) (string, error) {
	if byteLength <= 0 {
		return "", errors.New("byte length must be positive")
	}
	b := make([]byte, byteLength)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("rand read: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// MustRandomToken is like RandomToken but panics on error.
func MustRandomToken(byteLength int) string {
	t, err := RandomToken(byteLength)
	if err != nil {
		panic(err)
	}
	return t
}

// RandomAPIKey generates a typical API key: a random token with a prefix and checksum?
// Simpler: just return a random token.
func RandomAPIKey() (string, error) {
	return RandomToken(32) // 32 bytes → ~43 characters
}

// ----------------------------------------------------------------------
// Example usage (commented)
// ----------------------------------------------------------------------
// func exampleJWT() {
//     // HMAC signing
//     secret := []byte("my-secret")
//     key := token.NewHMACSigningKey(secret, jwt.SigningMethodHS256)
//
//     claims := token.Claims{
//         UserID: "user123",
//         Role:   "admin",
//         RegisteredClaims: jwt.RegisteredClaims{
//             ExpiresAt: jwt.NewNumericDate(time.Now().Add(1 * time.Hour)),
//             IssuedAt:  jwt.NewNumericDate(time.Now()),
//             NotBefore: jwt.NewNumericDate(time.Now()),
//         },
//     }
//     tokenString, err := token.GenerateJWT(claims, key)
//     if err != nil {
//         log.Fatal(err)
//     }
//
//     parsedClaims, err := token.ValidateJWT(tokenString, key)
//     if err != nil {
//         log.Fatal(err)
//     }
//     fmt.Println(parsedClaims.UserID)
// }
//
// func exampleRandomToken() {
//     key, _ := token.RandomToken(32)
//     fmt.Println(key)
// }