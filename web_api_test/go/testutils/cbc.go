// Package cbc provides AES encryption in CBC (cipher block chaining) mode
// with PKCS#7 padding. A random IV is generated and prepended to the ciphertext.
// WARNING: CBC mode alone does not provide authentication. Use with an HMAC
// or prefer authenticated modes like GCM.
package testutils

import (
	"bytes"
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
	// IVSize is the AES block size (16 bytes).
	IVSize = aes.BlockSize
)

var (
	ErrInvalidKeySize   = errors.New("key size must be 16, 24, or 32 bytes")
	ErrInvalidIVSize    = errors.New("IV size must be 16 bytes")
	ErrInvalidCiphertext = errors.New("ciphertext too short to contain IV")
	ErrInvalidPadding   = errors.New("invalid padding")
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

// GenerateIV creates a random IV of the required block size.
func GenerateIV() ([]byte, error) {
	iv := make([]byte, IVSize)
	_, err := rand.Read(iv)
	return iv, err
}

// pkcs7Pad adds PKCS#7 padding to data to reach the given block size.
func pkcs7Pad(data []byte, blockSize int) []byte {
	padding := blockSize - len(data)%blockSize
	padText := bytes.Repeat([]byte{byte(padding)}, padding)
	return append(data, padText...)
}

// pkcs7Unpad removes PKCS#7 padding.
func pkcs7Unpad(data []byte, blockSize int) ([]byte, error) {
	if len(data) == 0 {
		return nil, ErrInvalidPadding
	}
	if len(data)%blockSize != 0 {
		return nil, ErrInvalidPadding
	}
	padding := int(data[len(data)-1])
	if padding < 1 || padding > blockSize {
		return nil, ErrInvalidPadding
	}
	for i := len(data) - padding; i < len(data); i++ {
		if int(data[i]) != padding {
			return nil, ErrInvalidPadding
		}
	}
	return data[:len(data)-padding], nil
}

// Encrypt encrypts plaintext using AES‑CBC with PKCS#7 padding.
// It returns the IV prepended to the ciphertext.
func Encrypt(key, plaintext []byte) ([]byte, error) {
	if err := checkKey(key); err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}
	iv, err := GenerateIV()
	if err != nil {
		return nil, fmt.Errorf("failed to generate IV: %w", err)
	}
	padded := pkcs7Pad(plaintext, aes.BlockSize)
	ciphertext := make([]byte, len(padded))
	mode := cipher.NewCBCEncrypter(block, iv)
	mode.CryptBlocks(ciphertext, padded)
	// Prepend IV.
	result := make([]byte, 0, IVSize+len(ciphertext))
	result = append(result, iv...)
	result = append(result, ciphertext...)
	return result, nil
}

// Decrypt decrypts ciphertext produced by Encrypt. It expects the IV to be prepended.
func Decrypt(key, ciphertext []byte) ([]byte, error) {
	if err := checkKey(key); err != nil {
		return nil, err
	}
	if len(ciphertext) < IVSize {
		return nil, ErrInvalidCiphertext
	}
	iv := ciphertext[:IVSize]
	ciphertext = ciphertext[IVSize:]
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}
	if len(ciphertext)%aes.BlockSize != 0 {
		return nil, ErrInvalidCiphertext
	}
	mode := cipher.NewCBCDecrypter(block, iv)
	plaintextPadded := make([]byte, len(ciphertext))
	mode.CryptBlocks(plaintextPadded, ciphertext)
	return pkcs7Unpad(plaintextPadded, aes.BlockSize)
}

func checkKey(key []byte) error {
	if len(key) != KeySize128 && len(key) != KeySize192 && len(key) != KeySize256 {
		return ErrInvalidKeySize
	}
	return nil
}