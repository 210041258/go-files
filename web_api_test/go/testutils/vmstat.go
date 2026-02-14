// Package testutils provides utilities for testing, including
// reading virtual memory statistics from /proc/vmstat on Linux.
package testutils

import (
	"bufio"
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"
)

// VMStat represents fields from /proc/vmstat.
// Only common fields are included; add more as needed.
type VMStat struct {
	NrFreePages   uint64
	NrDirty       uint64
	NrWriteback   uint64
	NrMapped      uint64
	NrSlab        uint64
	PgpgIn        uint64
	PgpgOut       uint64
	Pswpin        uint64
	Pswpout       uint64
	PgAllocDMA    uint64
	PgAllocNormal uint64
	PgAllocMovable uint64
	// Add other fields as needed.
}

// ReadVMStat reads virtual memory statistics from the system.
// On Linux, it reads /proc/vmstat. On other platforms, it returns an error.
func ReadVMStat() (VMStat, error) {
	if runtime.GOOS != "linux" {
		return VMStat{}, fmt.Errorf("ReadVMStat not implemented on %s", runtime.GOOS)
	}
	return parseVMStat("/proc/vmstat")
}

// parseVMStat reads and parses the given vmstat file (for testing with custom files).
func parseVMStat(path string) (VMStat, error) {
	f, err := os.Open(path)
	if err != nil {
		return VMStat{}, err
	}
	defer f.Close()

	var stat VMStat
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) != 2 {
			continue
		}
		key := fields[0]
		value, err := strconv.ParseUint(fields[1], 10, 64)
		if err != nil {
			continue
		}
		switch key {
		case "nr_free_pages":
			stat.NrFreePages = value
		case "nr_dirty":
			stat.NrDirty = value
		case "nr_writeback":
			stat.NrWriteback = value
		case "nr_mapped":
			stat.NrMapped = value
		case "nr_slab":
			stat.NrSlab = value
		case "pgpgin":
			stat.PgpgIn = value
		case "pgpgout":
			stat.PgpgOut = value
		case "pswpin":
			stat.Pswpin = value
		case "pswpout":
			stat.Pswpout = value
		case "pgalloc_dma":
			stat.PgAllocDMA = value
		case "pgalloc_normal":
			stat.PgAllocNormal = value
		case "pgalloc_movable":
			stat.PgAllocMovable = value
		}
	}
	if err := scanner.Err(); err != nil {
		return stat, err
	}
	return stat, nil
}

// MockVMStat returns a dummy VMStat struct for use in tests.
func MockVMStat() VMStat {
	return VMStat{
		NrFreePages:   1024000,
		NrDirty:       512,
		NrWriteback:   0,
		NrMapped:      204800,
		NrSlab:        102400,
		PgpgIn:        51200,
		PgpgOut:       204800,
		Pswpin:        1024,
		Pswpout:       2048,
		PgAllocDMA:    0,
		PgAllocNormal: 1024000,
		PgAllocMovable: 512000,
	}
}

// ----------------------------------------------------------------------
// Example usage (commented)
// ----------------------------------------------------------------------
// func main() {
//     if stat, err := testutils.ReadVMStat(); err == nil {
//         fmt.Printf("Free pages: %d\n", stat.NrFreePages)
//     } else {
//         fmt.Println("Not on Linux, using mock:")
//         stat := testutils.MockVMStat()
//         fmt.Printf("Mock free pages: %d\n", stat.NrFreePages)
//     }
// }