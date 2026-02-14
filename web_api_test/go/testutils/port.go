// Package port provides utilities for working with network ports.
// It includes functions for validation, finding free ports,
// scanning ranges, and waiting for port availability.
package testutils

import (
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"
)

// ----------------------------------------------------------------------
// Validation
// ----------------------------------------------------------------------

// IsValid reports whether the port number is within the valid range (1–65535).
func IsValid(port int) bool {
	return port > 0 && port <= 65535
}

// MustValid panics if the port is invalid.
func MustValid(port int) {
	if !IsValid(port) {
		panic(fmt.Sprintf("port.MustValid: invalid port %d", port))
	}
}

// ----------------------------------------------------------------------
// Free port discovery
// ----------------------------------------------------------------------

// Free returns a free TCP port on the local machine.
// The port is not guaranteed to remain free after the function returns.
func Free() (int, error) {
	addr, err := net.ResolveTCPAddr("tcp", "localhost:0")
	if err != nil {
		return 0, err
	}
	l, err := net.ListenTCP("tcp", addr)
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}

// MustFree is like Free but panics on error.
func MustFree() int {
	p, err := Free()
	if err != nil {
		panic("port.MustFree: " + err.Error())
	}
	return p
}

// FreeUDP returns a free UDP port.
func FreeUDP() (int, error) {
	addr, err := net.ResolveUDPAddr("udp", "localhost:0")
	if err != nil {
		return 0, err
	}
	l, err := net.ListenUDP("udp", addr)
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.LocalAddr().(*net.UDPAddr).Port, nil
}

// ----------------------------------------------------------------------
// Availability checks
// ----------------------------------------------------------------------

// IsOpenTCP checks whether a TCP port is open on the given host.
// It attempts to connect and immediately closes the connection.
func IsOpenTCP(host string, port int) bool {
	target := net.JoinHostPort(host, fmt.Sprintf("%d", port))
	conn, err := net.DialTimeout("tcp", target, 2*time.Second)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// IsOpenUDP checks whether a UDP port is reachable.
// This is a best‑effort check; it may return false positives.
func IsOpenUDP(host string, port int) bool {
	target := net.JoinHostPort(host, fmt.Sprintf("%d", port))
	conn, err := net.DialTimeout("udp", target, 2*time.Second)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// ----------------------------------------------------------------------
// Waiting
// ----------------------------------------------------------------------

// WaitForTCP waits up to timeout for a TCP port to become open.
// It polls every 100 ms.
func WaitForTCP(host string, port int, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for time.Now().Before(deadline) {
		if IsOpenTCP(host, port) {
			return nil
		}
		<-ticker.C
	}
	return fmt.Errorf("port %d on %s not open after %v", port, host, timeout)
}

// ----------------------------------------------------------------------
// Range parsing and scanning
// ----------------------------------------------------------------------

// ParseRange parses a string like "80", "8000-8080", or "22,80,443"
// into a slice of port numbers. Ranges are inclusive.
// It returns an error if any port is invalid.
func ParseRange(s string) ([]int, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, nil
	}
	var ports []int
	// Split by commas for multiple entries.
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		// Check for range (contains '-').
		if strings.Contains(part, "-") {
			parts := strings.SplitN(part, "-", 2)
			if len(parts) != 2 {
				return nil, fmt.Errorf("invalid range: %q", part)
			}
			start, err := strconv.Atoi(strings.TrimSpace(parts[0]))
			if err != nil {
				return nil, fmt.Errorf("invalid start port in range %q: %w", part, err)
			}
			end, err := strconv.Atoi(strings.TrimSpace(parts[1]))
			if err != nil {
				return nil, fmt.Errorf("invalid end port in range %q: %w", part, err)
			}
			if start > end {
				return nil, fmt.Errorf("start port (%d) > end port (%d) in range %q", start, end, part)
			}
			for p := start; p <= end; p++ {
				if !IsValid(p) {
					return nil, fmt.Errorf("port %d out of range", p)
				}
				ports = append(ports, p)
			}
		} else {
			// Single port.
			p, err := strconv.Atoi(part)
			if err != nil {
				return nil, fmt.Errorf("invalid port number %q: %w", part, err)
			}
			if !IsValid(p) {
				return nil, fmt.Errorf("port %d out of range", p)
			}
			ports = append(ports, p)
		}
	}
	return ports, nil
}

// ScanTCP scans for open TCP ports on a host within the given range.
// It returns a slice of open ports.
func ScanTCP(host string, start, end int) ([]int, error) {
	if start > end {
		return nil, errors.New("start port must be <= end port")
	}
	if !IsValid(start) || !IsValid(end) {
		return nil, errors.New("ports out of valid range")
	}
	var open []int
	for p := start; p <= end; p++ {
		if IsOpenTCP(host, p) {
			open = append(open, p)
		}
	}
	return open, nil
}

// ----------------------------------------------------------------------
// Example usage (commented)
// ----------------------------------------------------------------------
// func main() {
//     p, _ := port.Free()
//     fmt.Println("Free port:", p)
//
//     ports, _ := port.ParseRange("80,443,8000-8080")
//     fmt.Println(ports) // [80 443 8000 8001 ... 8080]
//
//     open, _ := port.ScanTCP("localhost", 8000, 8010)
//     fmt.Println("Open ports:", open)
// }