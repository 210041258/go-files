// Package testutils provides cryptographic utilities including
// low-level AES-GCM encryption and decryption functions.
package testutils

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
)

const (
	// GCMNonceSize is the recommended nonce size for AES-GCM (12 bytes).
	GCMNonceSize = 12
)

var (
	// ErrInvalidKeyLength is returned when the key length is not 16, 24, or 32 bytes.
	ErrInvalidKeyLength = errors.New("key must be 16, 24, or 32 bytes")
	// ErrInvalidCiphertext is returned when decryption fails (authentication error or invalid data).
	ErrInvalidCiphertext = errors.New("invalid ciphertext or authentication failed")
)

// EncryptGCM encrypts the plaintext using AES-GCM with the given key.
// It generates a random nonce of 12 bytes and prepends it to the ciphertext.
// If additionalData is non-nil, it is used as additional authenticated data.
// The returned slice is: nonce (12 bytes) + ciphertext.
func EncryptGCM(key, plaintext, additionalData []byte) ([]byte, error) {
	if err := validateKey(key); err != nil {
		return nil, err
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("failed to generate nonce: %w", err)
	}

	// Seal encrypts and authenticates plaintext, appends result to nonce (which we discard).
	ciphertext := gcm.Seal(nil, nonce, plaintext, additionalData)

	// Prepend nonce to ciphertext for convenience.
	result := make([]byte, 0, len(nonce)+len(ciphertext))
	result = append(result, nonce...)
	result = append(result, ciphertext...)

	return result, nil
}

// DecryptGCM decrypts a ciphertext produced by EncryptGCM.
// It expects the input to be nonce (12 bytes) followed by the actual ciphertext.
// If additionalData was used during encryption, the same value must be provided.
func DecryptGCM(key, data, additionalData []byte) ([]byte, error) {
	if err := validateKey(key); err != nil {
		return nil, err
	}
	if len(data) < GCMNonceSize {
		return nil, ErrInvalidCiphertext
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	nonce := data[:GCMNonceSize]
	ciphertext := data[GCMNonceSize:]

	plaintext, err := gcm.Open(nil, nonce, ciphertext, additionalData)
	if err != nil {
		return nil, ErrInvalidCiphertext
	}
	return plaintext, nil
}

// EncryptGCMWithNonce encrypts using a provided nonce (must be 12 bytes).
// This is useful when nonce management is handled externally (e.g., counter mode).
// WARNING: Never reuse a (key, nonce) pair with the same plaintext.
func EncryptGCMWithNonce(key, nonce, plaintext, additionalData []byte) ([]byte, error) {
	if err := validateKey(key); err != nil {
		return nil, err
	}
	if len(nonce) != GCMNonceSize {
		return nil, fmt.Errorf("nonce must be %d bytes", GCMNonceSize)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	ciphertext := gcm.Seal(nil, nonce, plaintext, additionalData)
	return ciphertext, nil
}

// DecryptGCMWithNonce decrypts using a provided nonce.
func DecryptGCMWithNonce(key, nonce, ciphertext, additionalData []byte) ([]byte, error) {
	if err := validateKey(key); err != nil {
		return nil, err
	}
	if len(nonce) != GCMNonceSize {
		return nil, fmt.Errorf("nonce must be %d bytes", GCMNonceSize)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	plaintext, err := gcm.Open(nil, nonce, ciphertext, additionalData)
	if err != nil {
		return nil, ErrInvalidCiphertext
	}
	return plaintext, nil
}

// GenerateGCMNonce creates a cryptographically secure random nonce of the recommended size.
func GenerateGCMNonce() ([]byte, error) {
	nonce := make([]byte, GCMNonceSize)
	_, err := rand.Read(nonce)
	return nonce, err
}

// validateKey checks that the key has an acceptable length for AES.
func validateKey(key []byte) error {
	switch len(key) {
	case 16, 24, 32:
		return nil
	default:
		return ErrInvalidKeyLength
	}
}