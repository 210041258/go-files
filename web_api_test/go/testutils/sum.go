// Package sum implements traditional Unix‑style checksum algorithms,
// specifically the BSD sum (16‑bit ones‑complement sum) and System V sum
// (also 16‑bit but with a different algorithm). These are useful for
// legacy compatibility or simple integrity checks.
package testutils

import (
	"bufio"
	"encoding/binary"
	"io"
	"os"
)

// BSD computes the BSD sum (16‑bit ones‑complement sum) of data.
// It returns the checksum and the total number of bytes.
func BSD(data []byte) (checksum uint16, bytes int) {
	bytes = len(data)
	// Pad with zero to even length if necessary.
	if len(data)%2 == 1 {
		data = append(data, 0)
	}
	var sum uint32
	for i := 0; i < len(data); i += 2 {
		word := binary.BigEndian.Uint16(data[i:])
		sum += uint32(word)
		// Ones‑complement addition: carry wraps around.
		if sum&0xFFFF0000 != 0 {
			sum = (sum & 0xFFFF) + 1
		}
	}
	checksum = uint16(sum)
	return
}

// BSDFile computes the BSD sum of a file.
// It returns the checksum, the file size in bytes, and any error.
func BSDFile(path string) (checksum uint16, size int64, err error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, 0, err
	}
	defer f.Close()
	r := bufio.NewReader(f)
	var sum uint32
	var total int64
	buf := make([]byte, 4096)
	for {
		n, err := r.Read(buf)
		if n > 0 {
			total += int64(n)
			// Process full 16‑bit words.
			data := buf[:n]
			if len(data)%2 == 1 {
				// If odd, temporarily pad with zero for this chunk.
				// But we need to remember the real byte count.
				data = append(data, 0)
			}
			for i := 0; i < len(data); i += 2 {
				word := binary.BigEndian.Uint16(data[i:])
				sum += uint32(word)
				if sum&0xFFFF0000 != 0 {
					sum = (sum & 0xFFFF) + 1
				}
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return 0, total, err
		}
	}
	// If total bytes was odd, we added a zero byte at the end of the last chunk,
	// which is correct for BSD sum (it effectively processes the last byte as if
	// followed by a zero).
	checksum = uint16(sum)
	return checksum, total, nil
}

// SysV computes the System V sum (16‑bit sum with end‑around carry) of data.
// It returns the checksum and the total number of bytes.
// The algorithm is: sum of all bytes as 16‑bit integers, with carry added back.
func SysV(data []byte) (checksum uint16, bytes int) {
	bytes = len(data)
	var sum uint32
	for _, b := range data {
		sum += uint32(b)
		// End‑around carry: if sum exceeds 16 bits, add carry back.
		if sum&0xFFFF0000 != 0 {
			sum = (sum & 0xFFFF) + (sum >> 16)
		}
	}
	checksum = uint16(sum)
	return
}

// SysVFile computes the System V sum of a file.
// It returns the checksum, the file size in bytes, and any error.
func SysVFile(path string) (checksum uint16, size int64, err error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, 0, err
	}
	defer f.Close()
	var sum uint32
	var total int64
	buf := make([]byte, 4096)
	for {
		n, err := f.Read(buf)
		if n > 0 {
			total += int64(n)
			for _, b := range buf[:n] {
				sum += uint32(b)
				if sum&0xFFFF0000 != 0 {
					sum = (sum & 0xFFFF) + (sum >> 16)
				}
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return 0, total, err
		}
	}
	checksum = uint16(sum)
	return checksum, total, nil
}