// Package bin provides utilities for binary and bit-level manipulation.
// It includes conversions between integers and byte slices, binary string
// formatting, bit counting, and endian conversions.
package bin

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
)

// ----------------------------------------------------------------------
// Byte slice <-> integer conversions (big and little endian)
// ----------------------------------------------------------------------

// Uint16ToBytesBE converts a uint16 to a 2-byte slice in big-endian order.
func Uint16ToBytesBE(v uint16) []byte {
	b := make([]byte, 2)
	binary.BigEndian.PutUint16(b, v)
	return b
}

// Uint16ToBytesLE converts a uint16 to a 2-byte slice in little-endian order.
func Uint16ToBytesLE(v uint16) []byte {
	b := make([]byte, 2)
	binary.LittleEndian.PutUint16(b, v)
	return b
}

// BytesToUint16BE converts a 2-byte slice in big-endian order to a uint16.
// Returns an error if the slice length is less than 2.
func BytesToUint16BE(b []byte) (uint16, error) {
	if len(b) < 2 {
		return 0, errors.New("slice too short for uint16")
	}
	return binary.BigEndian.Uint16(b), nil
}

// BytesToUint16LE converts a 2-byte slice in little-endian order to a uint16.
func BytesToUint16LE(b []byte) (uint16, error) {
	if len(b) < 2 {
		return 0, errors.New("slice too short for uint16")
	}
	return binary.LittleEndian.Uint16(b), nil
}

// Uint32ToBytesBE converts a uint32 to a 4-byte slice in big-endian order.
func Uint32ToBytesBE(v uint32) []byte {
	b := make([]byte, 4)
	binary.BigEndian.PutUint32(b, v)
	return b
}

// Uint32ToBytesLE converts a uint32 to a 4-byte slice in little-endian order.
func Uint32ToBytesLE(v uint32) []byte {
	b := make([]byte, 4)
	binary.LittleEndian.PutUint32(b, v)
	return b
}

// BytesToUint32BE converts a 4-byte slice in big-endian order to a uint32.
func BytesToUint32BE(b []byte) (uint32, error) {
	if len(b) < 4 {
		return 0, errors.New("slice too short for uint32")
	}
	return binary.BigEndian.Uint32(b), nil
}

// BytesToUint32LE converts a 4-byte slice in little-endian order to a uint32.
func BytesToUint32LE(b []byte) (uint32, error) {
	if len(b) < 4 {
		return 0, errors.New("slice too short for uint32")
	}
	return binary.LittleEndian.Uint32(b), nil
}

// Uint64ToBytesBE converts a uint64 to an 8-byte slice in big-endian order.
func Uint64ToBytesBE(v uint64) []byte {
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, v)
	return b
}

// Uint64ToBytesLE converts a uint64 to an 8-byte slice in little-endian order.
func Uint64ToBytesLE(v uint64) []byte {
	b := make([]byte, 8)
	binary.LittleEndian.PutUint64(b, v)
	return b
}

// BytesToUint64BE converts an 8-byte slice in big-endian order to a uint64.
func BytesToUint64BE(b []byte) (uint64, error) {
	if len(b) < 8 {
		return 0, errors.New("slice too short for uint64")
	}
	return binary.BigEndian.Uint64(b), nil
}

// BytesToUint64LE converts an 8-byte slice in little-endian order to a uint64.
func BytesToUint64LE(b []byte) (uint64, error) {
	if len(b) < 8 {
		return 0, errors.New("slice too short for uint64")
	}
	return binary.LittleEndian.Uint64(b), nil
}

// Int64ToBytesBE converts an int64 to an 8-byte slice in big-endian order.
func Int64ToBytesBE(v int64) []byte {
	return Uint64ToBytesBE(uint64(v))
}

// Int64ToBytesLE converts an int64 to an 8-byte slice in little-endian order.
func Int64ToBytesLE(v int64) []byte {
	return Uint64ToBytesLE(uint64(v))
}

// BytesToInt64BE converts an 8-byte slice in big-endian order to an int64.
func BytesToInt64BE(b []byte) (int64, error) {
	u, err := BytesToUint64BE(b)
	if err != nil {
		return 0, err
	}
	return int64(u), nil
}

// BytesToInt64LE converts an 8-byte slice in little-endian order to an int64.
func BytesToInt64LE(b []byte) (int64, error) {
	u, err := BytesToUint64LE(b)
	if err != nil {
		return 0, err
	}
	return int64(u), nil
}

// ----------------------------------------------------------------------
// Floating point <-> byte slice conversions
// ----------------------------------------------------------------------

// Float32ToBytesBE converts a float32 to a 4-byte slice in big-endian order.
func Float32ToBytesBE(v float32) []byte {
	return Uint32ToBytesBE(math.Float32bits(v))
}

// Float32ToBytesLE converts a float32 to a 4-byte slice in little-endian order.
func Float32ToBytesLE(v float32) []byte {
	return Uint32ToBytesLE(math.Float32bits(v))
}

// BytesToFloat32BE converts a 4-byte slice in big-endian order to a float32.
func BytesToFloat32BE(b []byte) (float32, error) {
	u, err := BytesToUint32BE(b)
	if err != nil {
		return 0, err
	}
	return math.Float32frombits(u), nil
}

// BytesToFloat32LE converts a 4-byte slice in little-endian order to a float32.
func BytesToFloat32LE(b []byte) (float32, error) {
	u, err := BytesToUint32LE(b)
	if err != nil {
		return 0, err
	}
	return math.Float32frombits(u), nil
}

// Float64ToBytesBE converts a float64 to an 8-byte slice in big-endian order.
func Float64ToBytesBE(v float64) []byte {
	return Uint64ToBytesBE(math.Float64bits(v))
}

// Float64ToBytesLE converts a float64 to an 8-byte slice in little-endian order.
func Float64ToBytesLE(v float64) []byte {
	return Uint64ToBytesLE(math.Float64bits(v))
}

// BytesToFloat64BE converts an 8-byte slice in big-endian order to a float64.
func BytesToFloat64BE(b []byte) (float64, error) {
	u, err := BytesToUint64BE(b)
	if err != nil {
		return 0, err
	}
	return math.Float64frombits(u), nil
}

// BytesToFloat64LE converts an 8-byte slice in little-endian order to a float64.
func BytesToFloat64LE(b []byte) (float64, error) {
	u, err := BytesToUint64LE(b)
	if err != nil {
		return 0, err
	}
	return math.Float64frombits(u), nil
}

// ----------------------------------------------------------------------
// Binary string representation
// ----------------------------------------------------------------------

// ToBinaryString returns the binary representation of an unsigned integer
// with the specified minimum bit width (padded with leading zeros).
func ToBinaryString(v uint64, width int) string {
	if width < 1 {
		width = 1
	}
	format := fmt.Sprintf("%%0%db", width)
	return fmt.Sprintf(format, v)
}

// ToBinaryString8 returns an 8‑bit binary string for a byte.
func ToBinaryString8(b byte) string {
	return ToBinaryString(uint64(b), 8)
}

// ToBinaryString16 returns a 16‑bit binary string for a uint16.
func ToBinaryString16(v uint16) string {
	return ToBinaryString(uint64(v), 16)
}

// ToBinaryString32 returns a 32‑bit binary string for a uint32.
func ToBinaryString32(v uint32) string {
	return ToBinaryString(uint64(v), 32)
}

// ToBinaryString64 returns a 64‑bit binary string for a uint64.
func ToBinaryString64(v uint64) string {
	return ToBinaryString(v, 64)
}

// FromBinaryString parses a binary string (e.g., "1010") into a uint64.
// It accepts an optional "0b" prefix.
func FromBinaryString(s string) (uint64, error) {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "0b")
	return strconv.ParseUint(s, 2, 64)
}

// MustFromBinaryString is like FromBinaryString but panics on error.
func MustFromBinaryString(s string) uint64 {
	v, err := FromBinaryString(s)
	if err != nil {
		panic(err)
	}
	return v
}

// ----------------------------------------------------------------------
// Bit manipulation
// ----------------------------------------------------------------------

// SetBit sets the bit at position pos (0 = least significant) to 1.
func SetBit(n uint64, pos uint) uint64 {
	return n | (1 << pos)
}

// ClearBit sets the bit at position pos to 0.
func ClearBit(n uint64, pos uint) uint64 {
	return n & ^(1 << pos)
}

// ToggleBit flips the bit at position pos.
func ToggleBit(n uint64, pos uint) uint64 {
	return n ^ (1 << pos)
}

// HasBit reports whether the bit at position pos is 1.
func HasBit(n uint64, pos uint) bool {
	return (n>>pos)&1 == 1
}

// BitCount returns the number of 1 bits (population count) using the
// Hamming weight algorithm (also known as popcount).
func BitCount(n uint64) int {
	// Using the standard popcount algorithm.
	n = n - ((n >> 1) & 0x5555555555555555)
	n = (n & 0x3333333333333333) + ((n >> 2) & 0x3333333333333333)
	n = (n + (n >> 4)) & 0x0f0f0f0f0f0f0f0f
	n = n + (n >> 8)
	n = n + (n >> 16)
	n = n + (n >> 32)
	return int(n & 0x7f)
}

// BitCount8 returns the number of 1 bits in a byte.
func BitCount8(b byte) int {
	return BitCount(uint64(b))
}

// ReverseBits reverses the order of bits in a 64‑bit word.
func ReverseBits(n uint64) uint64 {
	n = (n&0x5555555555555555)<<1 | (n&0xAAAAAAAAAAAAAAAA)>>1
	n = (n&0x3333333333333333)<<2 | (n&0xCCCCCCCCCCCCCCCC)>>2
	n = (n&0x0F0F0F0F0F0F0F0F)<<4 | (n&0xF0F0F0F0F0F0F0F0)>>4
	n = (n&0x00FF00FF00FF00FF)<<8 | (n&0xFF00FF00FF00FF00)>>8
	n = (n&0x0000FFFF0000FFFF)<<16 | (n&0xFFFF0000FFFF0000)>>16
	n = (n&0x00000000FFFFFFFF)<<32 | (n&0xFFFFFFFF00000000)>>32
	return n
}

// ReverseBits8 reverses the bits in a byte.
func ReverseBits8(b byte) byte {
	return byte(ReverseBits(uint64(b)) >> 56)
}

// ----------------------------------------------------------------------
// Endian swap
// ----------------------------------------------------------------------

// SwapEndian16 swaps the byte order of a 16‑bit value.
func SwapEndian16(v uint16) uint16 {
	return v<<8 | v>>8
}

// SwapEndian32 swaps the byte order of a 32‑bit value.
func SwapEndian32(v uint32) uint32 {
	return (v << 24) |
		((v << 8) & 0x00FF0000) |
		((v >> 8) & 0x0000FF00) |
		(v >> 24)
}

// SwapEndian64 swaps the byte order of a 64‑bit value.
func SwapEndian64(v uint64) uint64 {
	return (v << 56) |
		((v << 40) & 0x00FF000000000000) |
		((v << 24) & 0x0000FF0000000000) |
		((v << 8) & 0x000000FF00000000) |
		((v >> 8) & 0x00000000FF000000) |
		((v >> 24) & 0x0000000000FF0000) |
		((v >> 40) & 0x000000000000FF00) |
		(v >> 56)
}

// ----------------------------------------------------------------------
// Example usage (commented)
// ----------------------------------------------------------------------
// func main() {
//     // Convert uint32 to bytes (big endian)
//     b := bin.Uint32ToBytesBE(0xdeadbeef)
//     fmt.Printf("%x\n", b) // deadbeef
//
//     // Binary string
//     s := bin.ToBinaryString(42, 8)
//     fmt.Println(s) // 00101010
//
//     // Bit counting
//     fmt.Println(bin.BitCount(42)) // 3 (since 42 = 101010)
//
//     // Endian swap
//     fmt.Printf("%x\n", bin.SwapEndian16(0x1234)) // 3412
// }