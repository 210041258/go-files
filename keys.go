// Package testutils provides utilities for testing, including
// generation and handling of cryptographic keys.
package testutils

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
)

// ----------------------------------------------------------------------
// Key generation for testing
// ----------------------------------------------------------------------

// GenerateRSAKey generates a new RSA private key with the given bit size.
// For testing, 2048 bits is usually sufficient.
func GenerateRSAKey(bits int) (*rsa.PrivateKey, error) {
	if bits < 2048 {
		bits = 2048 // force a minimum for security
	}
	return rsa.GenerateKey(rand.Reader, bits)
}

// MustGenerateRSAKey is like GenerateRSAKey but panics on error.
// Useful for one‑time setup in tests.
func MustGenerateRSAKey(bits int) *rsa.PrivateKey {
	key, err := GenerateRSAKey(bits)
	if err != nil {
		panic(fmt.Sprintf("GenerateRSAKey: %v", err))
	}
	return key
}

// GenerateECKey generates a new ECDSA private key using the given curve.
// Common curves: elliptic.P256(), elliptic.P384(), elliptic.P521().
func GenerateECKey(curve elliptic.Curve) (*ecdsa.PrivateKey, error) {
	return ecdsa.GenerateKey(curve, rand.Reader)
}

// MustGenerateECKey is like GenerateECKey but panics on error.
func MustGenerateECKey(curve elliptic.Curve) *ecdsa.PrivateKey {
	key, err := GenerateECKey(curve)
	if err != nil {
		panic(fmt.Sprintf("GenerateECKey: %v", err))
	}
	return key
}

// GenerateHMACSecret creates a random secret of the given length (in bytes)
// suitable for HMAC signing.
func GenerateHMACSecret(length int) ([]byte, error) {
	if length <= 0 {
		length = 32 // default to 256 bits
	}
	secret := make([]byte, length)
	_, err := rand.Read(secret)
	return secret, err
}

// MustGenerateHMACSecret is like GenerateHMACSecret but panics on error.
func MustGenerateHMACSecret(length int) []byte {
	secret, err := GenerateHMACSecret(length)
	if err != nil {
		panic(fmt.Sprintf("GenerateHMACSecret: %v", err))
	}
	return secret
}

// ----------------------------------------------------------------------
// PEM encoding / decoding
// ----------------------------------------------------------------------

// PrivateKeyToPEM encodes a private key (RSA or ECDSA) as a PEM block.
// The returned bytes include the PEM header and footer.
func PrivateKeyToPEM(key interface{}) ([]byte, error) {
	var der []byte
	var err error
	switch k := key.(type) {
	case *rsa.PrivateKey:
		der = x509.MarshalPKCS1PrivateKey(k)
	case *ecdsa.PrivateKey:
		der, err = x509.MarshalECPrivateKey(k)
		if err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("unsupported private key type: %T", key)
	}
	block := &pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: der,
	}
	return pem.EncodeToMemory(block), nil
}

// PublicKeyToPEM encodes a public key (RSA or ECDSA) as a PEM block.
func PublicKeyToPEM(key interface{}) ([]byte, error) {
	der, err := x509.MarshalPKIXPublicKey(key)
	if err != nil {
		return nil, err
	}
	block := &pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: der,
	}
	return pem.EncodeToMemory(block), nil
}

// LoadPrivateKeyFromPEM parses a PEM‑encoded private key (RSA or ECDSA).
// It supports both PKCS1 and PKCS8 formats.
func LoadPrivateKeyFromPEM(pemData []byte) (interface{}, error) {
	block, _ := pem.Decode(pemData)
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM block")
	}
	// Try PKCS1 first (RSA)
	if key, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return key, nil
	}
	// Try EC private key
	if key, err := x509.ParseECPrivateKey(block.Bytes); err == nil {
		return key, nil
	}
	// Try PKCS8 (handles RSA, EC, etc.)
	if key, err := x509.ParsePKCS8PrivateKey(block.Bytes); err == nil {
		return key, nil
	}
	return nil, fmt.Errorf("failed to parse private key: unsupported format")
}

// LoadPublicKeyFromPEM parses a PEM‑encoded public key.
func LoadPublicKeyFromPEM(pemData []byte) (interface{}, error) {
	block, _ := pem.Decode(pemData)
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM block")
	}
	return x509.ParsePKIXPublicKey(block.Bytes)
}

// ----------------------------------------------------------------------
// Example usage (commented)
// ----------------------------------------------------------------------
// func TestSomething(t *testing.T) {
//     // Generate a test RSA key
//     key := testutils.MustGenerateRSAKey(2048)
//
//     // Convert to PEM for temporary storage
//     pemBytes, _ := testutils.PrivateKeyToPEM(key)
//     ioutil.WriteFile("testkey.pem", pemBytes, 0600)
//
//     // Later, load it back
//     loaded, _ := testutils.LoadPrivateKeyFromPEM(pemBytes)
//     _ = loaded
// }