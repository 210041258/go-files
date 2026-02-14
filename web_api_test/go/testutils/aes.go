// Package aes provides simple AES-GCM encryption utilities.
// It uses AES-256-GCM by default (key must be 32 bytes).
package testutils

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
)

const (
	// KeySize is the required key length for AES-256 (32 bytes).
	KeySize = 32
	// NonceSize is the standard GCM nonce size (12 bytes).
	NonceSize = 12
)

var (
	ErrInvalidKeySize    = errors.New("key must be 32 bytes for AES-256")
	ErrInvalidCiphertext = errors.New("ciphertext too short or corrupted")
)

// GenerateKey creates a new random 32-byte AES-256 key.
func GenerateKey() ([]byte, error) {
	key := make([]byte, KeySize)
	_, err := rand.Read(key)
	return key, err
}

// Encrypt encrypts plaintext using AES-256-GCM with a random nonce.
// It returns the ciphertext with the nonce prepended.
func Encrypt(key, plaintext []byte) ([]byte, error) {
	if len(key) != KeySize {
		return nil, ErrInvalidKeySize
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

	ciphertext := gcm.Seal(nil, nonce, plaintext, nil)

	// Prepend nonce to ciphertext.
	result := make([]byte, 0, len(nonce)+len(ciphertext))
	result = append(result, nonce...)
	result = append(result, ciphertext...)
	return result, nil
}

// Decrypt decrypts ciphertext produced by Encrypt.
// It expects the nonce to be prepended.
func Decrypt(key, ciphertext []byte) ([]byte, error) {
	if len(key) != KeySize {
		return nil, ErrInvalidKeySize
	}
	if len(ciphertext) < NonceSize {
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

	nonce := ciphertext[:NonceSize]
	ciphertext = ciphertext[NonceSize:]

	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, ErrInvalidCiphertext
	}
	return plaintext, nil
}

// EncryptString encrypts a string and returns the result as a base64 string.
func EncryptString(key []byte, plaintext string) (string, error) {
	ciphertext, err := Encrypt(key, []byte(plaintext))
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// DecryptString decrypts a base64-encoded ciphertext produced by EncryptString.
func DecryptString(key []byte, ciphertext string) (string, error) {
	data, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", err
	}
	plaintext, err := Decrypt(key, data)
	if err != nil {
		return "", err
	}
	return string(plaintext), nil
}

// MustGenerateKey is like GenerateKey but panics on error.
func MustGenerateKey() []byte {
	key, err := GenerateKey()
	if err != nil {
		panic(err)
	}
	return key
}

// Example usage (commented):
//
// func main() {
//     key := aes.MustGenerateKey()
//     ciphertext, _ := aes.EncryptString(key, "secret message")
//     fmt.Println(ciphertext)
//     plaintext, _ := aes.DecryptString(key, ciphertext)
//     fmt.Println(plaintext)
// }
