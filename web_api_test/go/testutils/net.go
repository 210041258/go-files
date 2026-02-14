// Package testutils provides utilities for testing, including
// network helpers like finding free ports, checking port availability,
// and waiting for services to become reachable.
package testutils

import (
	"errors"
	"fmt"
	"net"
	"time"
)

// FreePort returns a free TCP port on the local machine.
// The port is not guaranteed to remain free after the function returns.
func FreePort() (int, error) {
	addr, err := net.ResolveTCPAddr("tcp", "localhost:0")
	if err != nil {
		return 0, err
	}
	listener, err := net.ListenTCP("tcp", addr)
	if err != nil {
		return 0, err
	}
	defer listener.Close()
	return listener.Addr().(*net.TCPAddr).Port, nil
}

// MustFreePort is like FreePort but panics on error.
func MustFreePort() int {
	port, err := FreePort()
	if err != nil {
		panic("testutils: " + err.Error())
	}
	return port
}

// IsPortOpen checks whether a TCP port is open on the given host.
// It attempts to connect and immediately closes the connection.
func IsPortOpen(host string, port int) bool {
	target := net.JoinHostPort(host, fmt.Sprintf("%d", port))
	conn, err := net.DialTimeout("tcp", target, 2*time.Second)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// WaitForPort waits up to timeout for a TCP port to become open on the given host.
// It polls every 100ms. Returns nil if the port becomes open, otherwise an error.
func WaitForPort(host string, port int, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for time.Now().Before(deadline) {
		if IsPortOpen(host, port) {
			return nil
		}
		<-ticker.C
	}
	return errors.New("timeout waiting for port to become open")
}

// LocalIP returns the first non‑loopback IPv4 address of the local machine.
// Returns an error if none found.
func LocalIP() (net.IP, error) {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return nil, err
	}
	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() && ipnet.IP.To4() != nil {
			return ipnet.IP, nil
		}
	}
	return nil, errors.New("no non‑loopback IPv4 address found")
}

// RandomLocalAddr returns a string like "localhost:freeport" suitable for binding.
// It uses FreePort to allocate a temporary port.
func RandomLocalAddr() (string, error) {
	port, err := FreePort()
	if err != nil {
		return "", err
	}
	return net.JoinHostPort("localhost", fmt.Sprintf("%d", port)), nil
}

// MustRandomLocalAddr is like RandomLocalAddr but panics on error.
func MustRandomLocalAddr() string {
	addr, err := RandomLocalAddr()
	if err != nil {
		panic("testutils: " + err.Error())
	}
	return addr
}

// ----------------------------------------------------------------------
// Example usage (commented)
// ----------------------------------------------------------------------
// func TestSomething(t *testing.T) {
//     port := testutils.MustFreePort()
//     go startTestServer(port)
//     err := testutils.WaitForPort("localhost", port, 5*time.Second)
//     if err != nil {
//         t.Fatal(err)
//     }
// }