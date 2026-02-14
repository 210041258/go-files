// Package cipher provides symmetric encryption utilities using AES.
// It supports common modes: CBC (with PKCS7 padding), CTR, and a
// wrapper for GCM (which is also available separately).
// WARNING: CBC mode is vulnerable to padding oracle attacks if not
// combined with authentication. Prefer GCM or CTR+HMAC for new designs.
package testutils

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
)

const (
	// AESKeySize128 is 16 bytes.
	AESKeySize128 = 16
	// AESKeySize192 is 24 bytes.
	AESKeySize192 = 24
	// AESKeySize256 is 32 bytes.
	AESKeySize256 = 32
)

var (
	ErrInvalidKeyLength   = errors.New("key length must be 16, 24, or 32 bytes")
	ErrInvalidBlockSize   = errors.New("plaintext is not a multiple of block size")
	ErrInvalidPadding     = errors.New("invalid padding")
	ErrInvalidCiphertext  = errors.New("ciphertext too short")
)

// GenerateKey creates a random AES key of the specified length.
func GenerateKey(length int) ([]byte, error) {
	if length != AESKeySize128 && length != AESKeySize192 && length != AESKeySize256 {
		return nil, ErrInvalidKeyLength
	}
	key := make([]byte, length)
	_, err := rand.Read(key)
	return key, err
}

// GenerateIV creates a random initialization vector of the given length.
func GenerateIV(size int) ([]byte, error) {
	iv := make([]byte, size)
	_, err := rand.Read(iv)
	return iv, err
}

// ----------------------------------------------------------------------
// PKCS7 padding
// ----------------------------------------------------------------------

// pkcs7Pad adds PKCS7 padding to a block of data.
func pkcs7Pad(data []byte, blockSize int) []byte {
	padding := blockSize - len(data)%blockSize
	padText := bytes.Repeat([]byte{byte(padding)}, padding)
	return append(data, padText...)
}

// pkcs7Unpad removes PKCS7 padding from a block of data.
func pkcs7Unpad(data []byte, blockSize int) ([]byte, error) {
	if len(data) == 0 {
		return nil, ErrInvalidPadding
	}
	if len(data)%blockSize != 0 {
		return nil, ErrInvalidBlockSize
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

// ----------------------------------------------------------------------
// AES-CBC (with PKCS7 padding)
// ----------------------------------------------------------------------

// EncryptCBC encrypts plaintext using AES in CBC mode with PKCS7 padding.
// It returns the IV prepended to the ciphertext.
func EncryptCBC(key, plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	// Generate random IV of block size.
	iv, err := GenerateIV(block.BlockSize())
	if err != nil {
		return nil, err
	}

	// Pad plaintext to block size.
	padded := pkcs7Pad(plaintext, block.BlockSize())

	// Encrypt using CBC mode.
	ciphertext := make([]byte, len(padded))
	mode := cipher.NewCBCEncrypter(block, iv)
	mode.CryptBlocks(ciphertext, padded)

	// Prepend IV.
	result := make([]byte, 0, len(iv)+len(ciphertext))
	result = append(result, iv...)
	result = append(result, ciphertext...)
	return result, nil
}

// DecryptCBC decrypts ciphertext produced by EncryptCBC (IV prepended).
func DecryptCBC(key, ciphertext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	blockSize := block.BlockSize()
	if len(ciphertext) < blockSize {
		return nil, ErrInvalidCiphertext
	}
	iv := ciphertext[:blockSize]
	ciphertext = ciphertext[blockSize:]

	if len(ciphertext)%blockSize != 0 {
		return nil, ErrInvalidCiphertext
	}

	// Decrypt using CBC mode.
	mode := cipher.NewCBCDecrypter(block, iv)
	plaintext := make([]byte, len(ciphertext))
	mode.CryptBlocks(plaintext, ciphertext)

	// Remove padding.
	return pkcs7Unpad(plaintext, blockSize)
}

// ----------------------------------------------------------------------
// AES-CTR (stream mode, no padding)
// ----------------------------------------------------------------------

// EncryptCTR encrypts plaintext using AES in CTR mode.
// It returns the IV (nonce) prepended to the ciphertext.
// CTR mode does not require padding and produces ciphertext of the same length.
func EncryptCTR(key, plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	// For CTR, a 16‑byte IV is standard (the counter starts at 0 in the last 8 bytes).
	iv, err := GenerateIV(block.BlockSize())
	if err != nil {
		return nil, err
	}

	stream := cipher.NewCTR(block, iv)
	ciphertext := make([]byte, len(plaintext))
	stream.XORKeyStream(ciphertext, plaintext)

	// Prepend IV.
	result := make([]byte, 0, len(iv)+len(ciphertext))
	result = append(result, iv...)
	result = append(result, ciphertext...)
	return result, nil
}

// DecryptCTR decrypts ciphertext produced by EncryptCTR.
func DecryptCTR(key, ciphertext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	blockSize := block.BlockSize()
	if len(ciphertext) < blockSize {
		return nil, ErrInvalidCiphertext
	}
	iv := ciphertext[:blockSize]
	ciphertext = ciphertext[blockSize:]

	stream := cipher.NewCTR(block, iv)
	plaintext := make([]byte, len(ciphertext))
	stream.XORKeyStream(plaintext, ciphertext)
	return plaintext, nil
}

// ----------------------------------------------------------------------
// Utility: hex conversions (optional)
// ----------------------------------------------------------------------

// HexEncode returns the hex encoding of data.
func HexEncode(data []byte) string {
	return hex.EncodeToString(data)
}

// HexDecode returns the bytes represented by the hex string.
func HexDecode(s string) ([]byte, error) {
	return hex.DecodeString(s)
}

// ----------------------------------------------------------------------
// Example usage (commented)
// ----------------------------------------------------------------------
// func main() {
//     key, _ := cipher.GenerateKey(cipher.AESKeySize256)
//     plaintext := []byte("secret message")
//
//     // CBC
//     ciphertext, _ := cipher.EncryptCBC(key, plaintext)
//     decrypted, _ := cipher.DecryptCBC(key, ciphertext)
//     fmt.Println(string(decrypted))
//
//     // CTR
//     ciphertext2, _ := cipher.EncryptCTR(key, plaintext)
//     decrypted2, _ := cipher.DecryptCTR(key, ciphertext2)
//     fmt.Println(string(decrypted2))
// }