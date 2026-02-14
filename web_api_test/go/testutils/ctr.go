// Package ctr provides AES encryption in CTR (counter) mode.
// It uses a random nonce prepended to the ciphertext.
// No padding is required; ciphertext length equals plaintext length.
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
	// KeySize128 is 16 bytes.
	KeySize128 = 16
	// KeySize192 is 24 bytes.
	KeySize192 = 24
	// KeySize256 is 32 bytes.
	KeySize256 = 32
	// NonceSize is the size of the nonce (IV) used for CTR mode (16 bytes).
	NonceSize = 16
)

var (
	ErrInvalidKeySize   = errors.New("key size must be 16, 24, or 32 bytes")
	ErrInvalidCiphertext = errors.New("ciphertext too short to contain nonce")
)

// GenerateKey creates a random AES key of the specified length.
func GenerateKey(length int) ([]byte, error) {
	if length != KeySize128 && length != KeySize192 && length != KeySize256 {
		return nil, ErrInvalidKeySize
	}
	key := make([]byte, length)
	_, err := rand.Read(key)
	return key, err
}

// MustGenerateKey is like GenerateKey but panics on error.
func MustGenerateKey(length int) []byte {
	key, err := GenerateKey(length)
	if err != nil {
		panic(err)
	}
	return key
}

// GenerateNonce creates a random nonce of NonceSize bytes.
func GenerateNonce() ([]byte, error) {
	nonce := make([]byte, NonceSize)
	_, err := rand.Read(nonce)
	return nonce, err
}

// Encrypt encrypts plaintext using AES‑CTR with a random nonce.
// It returns the nonce prepended to the ciphertext.
func Encrypt(key, plaintext []byte) ([]byte, error) {
	if err := checkKey(key); err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}
	nonce, err := GenerateNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to generate nonce: %w", err)
	}
	stream := cipher.NewCTR(block, nonce)
	ciphertext := make([]byte, len(plaintext))
	stream.XORKeyStream(ciphertext, plaintext)
	// Prepend nonce.
	result := make([]byte, 0, NonceSize+len(ciphertext))
	result = append(result, nonce...)
	result = append(result, ciphertext...)
	return result, nil
}

// Decrypt decrypts ciphertext produced by Encrypt. It expects the nonce to be prepended.
func Decrypt(key, ciphertext []byte) ([]byte, error) {
	if err := checkKey(key); err != nil {
		return nil, err
	}
	if len(ciphertext) < NonceSize {
		return nil, ErrInvalidCiphertext
	}
	nonce := ciphertext[:NonceSize]
	ciphertext = ciphertext[NonceSize:]
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}
	stream := cipher.NewCTR(block, nonce)
	plaintext := make([]byte, len(ciphertext))
	stream.XORKeyStream(plaintext, ciphertext)
	return plaintext, nil
}

func checkKey(key []byte) error {
	if len(key) != KeySize128 && len(key) != KeySize192 && len(key) != KeySize256 {
		return ErrInvalidKeySize
	}
	return nil
}