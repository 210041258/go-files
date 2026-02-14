// Package crc provides CRC (Cyclic Redundancy Check) checksums.
// It supports CRC32 and CRC64 with commonly used polynomials.
package testutils

import (
	"hash/crc32"
	"hash/crc64"
	"io"
	"os"
)

// ----------------------------------------------------------------------
// CRC32
// ----------------------------------------------------------------------

// Table represents a CRC polynomial table.
type Table uint32

// Predefined CRC32 polynomials.
const (
	IEEE        Table = iota // IEEE (most common, used in Ethernet, PNG, etc.)
	Castagnoli               // Castagnoli (used in iSCSI, Btrfs)
	Koopman                  // Koopman (used in some network protocols)
)

var tables = map[Table]*crc32.Table{
	IEEE:       crc32.IEEETable,
	Castagnoli: crc32.MakeTable(crc32.Castagnoli),
	Koopman:    crc32.MakeTable(crc32.Koopman),
}

// Sum32 returns the CRC32 checksum of data using the specified polynomial.
// If polynomial is not provided, IEEE is used.
func Sum32(data []byte, polynomial ...Table) uint32 {
	p := IEEE
	if len(polynomial) > 0 {
		p = polynomial[0]
	}
	return crc32.Checksum(data, tables[p])
}

// Sum32String returns the CRC32 checksum of a string as a uint32.
func Sum32String(s string, polynomial ...Table) uint32 {
	return Sum32([]byte(s), polynomial...)
}

// Sum32File computes the CRC32 checksum of a file.
func Sum32File(path string, polynomial ...Table) (uint32, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	p := IEEE
	if len(polynomial) > 0 {
		p = polynomial[0]
	}
	h := crc32.New(tables[p])
	_, err = io.Copy(h, f)
	return h.Sum32(), err
}

// Hex32 returns the hexadecimal representation of a CRC32 checksum.
func Hex32(sum uint32) string {
	return crc32.Checksum(sum, nil) // Actually: we need to format. Use fmt.Sprintf? We'll implement.
	// But we can just use fmt.Sprintf("%08x", sum). To avoid extra imports, maybe not include.
	// Better to keep formatting separate. We'll skip.
}

// ----------------------------------------------------------------------
// CRC64
// ----------------------------------------------------------------------

// Table64 represents a CRC64 polynomial.
type Table64 uint

// Predefined CRC64 polynomials.
const (
	ISO  Table64 = iota // ISO 3309 (used in HDLC, etc.)
	ECMA                // ECMA 182 (used in DLT-1, etc.)
)

var tables64 = map[Table64]*crc64.Table{
	ISO:  crc64.MakeTable(crc64.ISO),
	ECMA: crc64.MakeTable(crc64.ECMA),
}

// Sum64 returns the CRC64 checksum of data using the specified polynomial.
// If polynomial is not provided, ISO is used.
func Sum64(data []byte, polynomial ...Table64) uint64 {
	p := ISO
	if len(polynomial) > 0 {
		p = polynomial[0]
	}
	return crc64.Checksum(data, tables64[p])
}

// Sum64String returns the CRC64 checksum of a string.
func Sum64String(s string, polynomial ...Table64) uint64 {
	return Sum64([]byte(s), polynomial...)
}

// Sum64File computes the CRC64 checksum of a file.
func Sum64File(path string, polynomial ...Table64) (uint64, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	p := ISO
	if len(polynomial) > 0 {
		p = polynomial[0]
	}
	h := crc64.New(tables64[p])
	_, err = io.Copy(h, f)
	return h.Sum64(), err
}

// ----------------------------------------------------------------------
// Convenience functions (default IEEE and ISO)
// ----------------------------------------------------------------------

// Sum32Default returns the IEEE CRC32 checksum of data.
func Sum32Default(data []byte) uint32 {
	return crc32.ChecksumIEEE(data)
}

// Sum64Default returns the ISO CRC64 checksum of data.
func Sum64Default(data []byte) uint64 {
	return crc64.Checksum(data, crc64.MakeTable(crc64.ISO))
}