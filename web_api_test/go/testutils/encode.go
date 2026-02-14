// Package testutils provides cryptographic utilities for encryption, decryption,
// and key derivation using industry-standard primitives (AES-256-GCM, Scrypt).
package testutils

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"fmt"
	"io"

	"golang.org/x/crypto/scrypt"
)

var (
	// ErrInvalidCiphertext is returned when decryption fails (e.g., wrong password or tampered data).
	ErrInvalidCiphertext = errors.New("encrypt: invalid ciphertext or incorrect password")
)

// Encryption parameters for Scrypt (N, r, p).
// These values are tuned for interactive use (approx 100ms on modern hardware).
// Increase 'N' for higher security (at the cost of speed).
const (
	scryptN      = 32768
	scryptR      = 8
	scryptP      = 1
	keyLen       = 32 // AES-256
	saltLen      = 32
	gcmNonceSize = 12 // Standard GCM nonce size
)

// Encrypt encrypts plaintext using a passphrase.
//
// Algorithm:
// 1. Generate a random Salt.
// 2. Derive a key from the passphrase and salt using Scrypt.
// 3. Generate a random Nonce.
// 4. Encrypt plaintext using AES-256-GCM.
// 5. Return: Salt + Nonce + Ciphertext.
//
// The output is safe to store in files or databases.
func Encrypt(ctx context.Context, plaintext, passphrase []byte) ([]byte, error) {
	if len(passphrase) == 0 {
		return nil, errors.New("encrypt: passphrase cannot be empty")
	}

	// 1. Generate Salt
	salt := make([]byte, saltLen)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return nil, fmt.Errorf("encrypt: generate salt: %w", err)
	}

	// 2. Derive Key
	key, err := DeriveKey(ctx, passphrase, salt)
	if err != nil {
		return nil, fmt.Errorf("encrypt: derive key: %w", err)
	}

	// 3. Create Cipher Block
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("encrypt: create cipher: %w", err)
	}

	// 4. Create GCM Mode
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("encrypt: create gcm: %w", err)
	}

	// 5. Generate Nonce
	nonce := make([]byte, gcmNonceSize)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("encrypt: generate nonce: %w", err)
	}

	// 6. Seal (Encrypt)
	// 'ciphertext' here contains the actual encrypted data + the authentication tag.
	ciphertext := gcm.Seal(nil, nonce, plaintext, nil)

	// 7. Format Output: Salt + Nonce + Ciphertext
	// We prepend the salt and nonce so that Decrypt() has everything it needs.
	output := make([]byte, 0, saltLen+gcmNonceSize+len(ciphertext))
	output = append(output, salt...)
	output = append(output, nonce...)
	output = append(output, ciphertext...)

	return output, nil
}

// Decrypt decrypts data that was encrypted with Encrypt.
// It extracts the salt and nonce from the ciphertext, derives the key,
// and authenticates and decrypts the data.
func Decrypt(ctx context.Context, ciphertext, passphrase []byte) ([]byte, error) {
	if len(passphrase) == 0 {
		return nil, errors.New("encrypt: passphrase cannot be empty")
	}

	// Minimum size check: Salt + Nonce + Tag (16 bytes) + at least 1 byte of data
	minLen := saltLen + gcmNonceSize + 16 // 16 is GCM tag size
	if len(ciphertext) < minLen {
		return nil, ErrInvalidCiphertext
	}

	// 1. Extract Metadata
	salt := ciphertext[0:saltLen]
	nonce := ciphertext[saltLen : saltLen+gcmNonceSize]
	encryptedData := ciphertext[saltLen+gcmNonceSize:]

	// 2. Derive Key
	key, err := DeriveKey(ctx, passphrase, salt)
	if err != nil {
		return nil, fmt.Errorf("encrypt: derive key: %w", err)
	}

	// 3. Create Cipher
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("encrypt: create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("encrypt: create gcm: %w", err)
	}

	// 4. Open (Decrypt & Verify)
	// GCM will verify the authentication tag automatically.
	// If the password is wrong or data was tampered with, this returns an error.
	plaintext, err := gcm.Open(nil, nonce, encryptedData, nil)
	if err != nil {
		return nil, ErrInvalidCiphertext
	}

	return plaintext, nil
}

// DeriveKey derives a cryptographic key from a passphrase and salt using Scrypt.
// It is context-aware, allowing cancellation of the expensive CPU operation.
func DeriveKey(ctx context.Context, passphrase, salt []byte) ([]byte, error) {
	// We run Scrypt in a goroutine to monitor for context cancellation.
	// Scrypt itself does not natively support context.
	type result struct {
		key []byte
		err error
	}

	resCh := make(chan result, 1)

	go func() {
		key, err := scrypt.Key(passphrase, salt, scryptN, scryptR, scryptP, keyLen)
		resCh <- result{key, err}
	}()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case res := <-resCh:
		return res.key, res.err
	}
}

// --------------------------------------------------------------------
// Stream Encryption Helper (Simple wrapper for large data)
// --------------------------------------------------------------------
// Note: True streaming AEAD (Authenticated Encryption with Associated Data)
// is complex because the auth tag must be calculated over the whole stream.
// For large files, standard practice is to chunk the data.
// The functions below provide a convenient way to encrypt files by reading
// them into memory. For multi-GB files, implement chunk-based encryption.

// EncryptFile reads a file, encrypts it, and writes it to a destination path.
// Warning: Loads the entire file into memory. Use only for files smaller than available RAM.
func EncryptFile(ctx context.Context, srcPath, dstPath string, passphrase []byte) error {
	data, err := ReadFileAtomic(srcPath) // Use our optimized reader from osutil
	if err != nil {
		return err
	}

	encrypted, err := Encrypt(ctx, data, passphrase)
	if err != nil {
		return err
	}

	// Write atomically to ensure we don't corrupt the destination on error
	return WriteFileAtomic(ctx, dstPath, encrypted, 0644)
}

// DecryptFile reads an encrypted file, decrypts it, and writes it to a destination path.
// Warning: Loads the entire file into memory.
func DecryptFile(ctx context.Context, srcPath, dstPath string, passphrase []byte) error {
	data, err := ReadFileAtomic(srcPath)
	if err != nil {
		return err
	}

	decrypted, err := Decrypt(ctx, data, passphrase)
	if err != nil {
		return err
	}

	return WriteFileAtomic(ctx, dstPath, decrypted, 0644)
}