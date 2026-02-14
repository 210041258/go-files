// Package checksum provides functions for computing checksums and hashes
// using common algorithms: CRC32, MD5, SHA1, SHA256, SHA512.
// It supports both in‑memory data and files.
package testutils

import (
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"hash"
	"hash/crc32"
	"io"
	"os"
)

// ----------------------------------------------------------------------
// CRC32 (IEEE)
// ----------------------------------------------------------------------

// CRC32 returns the IEEE CRC32 checksum of data.
func CRC32(data []byte) uint32 {
	return crc32.ChecksumIEEE(data)
}

// CRC32String returns the IEEE CRC32 checksum of a string.
func CRC32String(s string) uint32 {
	return crc32.ChecksumIEEE([]byte(s))
}

// CRC32File computes the IEEE CRC32 checksum of a file.
func CRC32File(path string) (uint32, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	h := crc32.NewIEEE()
	_, err = io.Copy(h, f)
	return h.Sum32(), err
}

// CRC32C returns the Castagnoli CRC32C checksum of data.
func CRC32C(data []byte) uint32 {
	return crc32.Checksum(data, crc32.MakeTable(crc32.Castagnoli))
}

// CRC32CString returns the Castagnoli CRC32C checksum of a string.
func CRC32CString(s string) uint32 {
	return crc32.Checksum([]byte(s), crc32.MakeTable(crc32.Castagnoli))
}

// CRC32CFile computes the Castagnoli CRC32C checksum of a file.
func CRC32CFile(path string) (uint32, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	h := crc32.New(crc32.MakeTable(crc32.Castagnoli))
	_, err = io.Copy(h, f)
	return h.Sum32(), err
}

// ----------------------------------------------------------------------
// MD5
// ----------------------------------------------------------------------

// MD5 returns the MD5 hash of data as a byte slice.
func MD5(data []byte) []byte {
	h := md5.Sum(data)
	return h[:]
}

// MD5String returns the MD5 hash of a string as a hexadecimal string.
func MD5String(s string) string {
	return hex.EncodeToString(MD5([]byte(s)))
}

// MD5File computes the MD5 hash of a file and returns it as a hex string.
func MD5File(path string) (string, error) {
	return hashFile(path, md5.New())
}

// ----------------------------------------------------------------------
// SHA1
// ----------------------------------------------------------------------

// SHA1 returns the SHA1 hash of data as a byte slice.
func SHA1(data []byte) []byte {
	h := sha1.Sum(data)
	return h[:]
}

// SHA1String returns the SHA1 hash of a string as a hex string.
func SHA1String(s string) string {
	return hex.EncodeToString(SHA1([]byte(s)))
}

// SHA1File computes the SHA1 hash of a file and returns it as a hex string.
func SHA1File(path string) (string, error) {
	return hashFile(path, sha1.New())
}

// ----------------------------------------------------------------------
// SHA256
// ----------------------------------------------------------------------

// SHA256 returns the SHA256 hash of data as a byte slice.
func SHA256(data []byte) []byte {
	h := sha256.Sum256(data)
	return h[:]
}

// SHA256String returns the SHA256 hash of a string as a hex string.
func SHA256String(s string) string {
	return hex.EncodeToString(SHA256([]byte(s)))
}

// SHA256File computes the SHA256 hash of a file and returns it as a hex string.
func SHA256File(path string) (string, error) {
	return hashFile(path, sha256.New())
}

// ----------------------------------------------------------------------
// SHA512
// ----------------------------------------------------------------------

// SHA512 returns the SHA512 hash of data as a byte slice.
func SHA512(data []byte) []byte {
	h := sha512.Sum512(data)
	return h[:]
}

// SHA512String returns the SHA512 hash of a string as a hex string.
func SHA512String(s string) string {
	return hex.EncodeToString(SHA512([]byte(s)))
}

// SHA512File computes the SHA512 hash of a file and returns it as a hex string.
func SHA512File(path string) (string, error) {
	return hashFile(path, sha512.New())
}

// ----------------------------------------------------------------------
// internal helpers
// ----------------------------------------------------------------------

// hashFile computes the hash of a file using the given hash.Hash.
// It returns the hex‑encoded digest.
func hashFile(path string, h hash.Hash) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	_, err = io.Copy(h, f)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// ----------------------------------------------------------------------
// Example usage (commented)
// ----------------------------------------------------------------------
// func main() {
//     data := []byte("hello world")
//     fmt.Printf("CRC32: %x\n", checksum.CRC32(data))
//     fmt.Printf("MD5: %s\n", checksum.MD5String("hello world"))
//     fmt.Printf("SHA256: %s\n", checksum.SHA256String("hello world"))
//     // File checksum
//     if sum, err := checksum.SHA256File("example.txt"); err == nil {
//         fmt.Println(sum)
//     }
// }