// Package netutil provides utility functions for common networking tasks.
// It complements the standard net package with higher-level operations.
package testutils

import (
	"errors"
	"net"
	"strconv"
	"strings"
)

// ----------------------------------------------------------------------
// IP address helpers
// ----------------------------------------------------------------------

// GetLocalIPs returns all non-loopback IPv4 addresses of the local machine.
func GetLocalIPs() ([]net.IP, error) {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return nil, err
	}
	var ips []net.IP
	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				ips = append(ips, ipnet.IP)
			}
		}
	}
	return ips, nil
}

// GetLocalIP returns the first non-loopback IPv4 address of the local machine.
// Returns nil if none found.
func GetLocalIP() net.IP {
	ips, _ := GetLocalIPs()
	if len(ips) > 0 {
		return ips[0]
	}
	return nil
}

// IsPrivateIP reports whether ip is a private address according to RFC 1918.
func IsPrivateIP(ip net.IP) bool {
	if ip4 := ip.To4(); ip4 != nil {
		// 10.0.0.0/8
		if ip4[0] == 10 {
			return true
		}
		// 172.16.0.0/12
		if ip4[0] == 172 && ip4[1] >= 16 && ip4[1] <= 31 {
			return true
		}
		// 192.168.0.0/16
		if ip4[0] == 192 && ip4[1] == 168 {
			return true
		}
	}
	return false
}

// IPToUint32 converts an IPv4 address to a uint32.
func IPToUint32(ip net.IP) uint32 {
	ip = ip.To4()
	if ip == nil {
		return 0
	}
	return uint32(ip[0])<<24 | uint32(ip[1])<<16 | uint32(ip[2])<<8 | uint32(ip[3])
}

// Uint32ToIP converts a uint32 to an IPv4 address.
func Uint32ToIP(n uint32) net.IP {
	return net.IPv4(byte(n>>24), byte(n>>16), byte(n>>8), byte(n))
}

// ----------------------------------------------------------------------
// CIDR and subnet helpers
// ----------------------------------------------------------------------

// CIDRContains reports whether cidr contains the given IP.
func CIDRContains(cidr string, ip net.IP) (bool, error) {
	_, ipnet, err := net.ParseCIDR(cidr)
	if err != nil {
		return false, err
	}
	return ipnet.Contains(ip), nil
}

// ParseCIDRMask returns the network address and broadcast address for a CIDR.
func ParseCIDRMask(cidr string) (network, broadcast net.IP, err error) {
	ip, ipnet, err := net.ParseCIDR(cidr)
	if err != nil {
		return nil, nil, err
	}
	network = ipnet.IP
	// Calculate broadcast: take network, OR with inverted mask.
	mask := ipnet.Mask
	broadcast = make(net.IP, len(network))
	for i := range network {
		broadcast[i] = network[i] | ^mask[i]
	}
	return network, broadcast, nil
}

// ----------------------------------------------------------------------
// Port utilities
// ----------------------------------------------------------------------

// IsPortOpen checks if a TCP port is open on the given host.
// The timeout is in seconds; a zero timeout uses the default net.Dial timeout.
func IsPortOpen(host string, port int, timeoutSec int) bool {
	target := net.JoinHostPort(host, strconv.Itoa(port))
	conn, err := net.DialTimeout("tcp", target, time.Duration(timeoutSec)*time.Second)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// RandomPort returns a random free TCP port by asking the kernel to allocate one.
// The port is not guaranteed to remain free after the function returns.
func RandomPort() (int, error) {
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

// ----------------------------------------------------------------------
// Hostname and DNS
// ----------------------------------------------------------------------

// Hostname returns the system's hostname, with a fallback to "localhost".
func Hostname() string {
	name, err := os.Hostname()
	if err != nil {
		return "localhost"
	}
	return name
}

// ResolveHostname returns the IP addresses for the given hostname.
// If hostname is empty, it returns the local machine's IPs.
func ResolveHostname(hostname string) ([]net.IP, error) {
	if hostname == "" {
		return GetLocalIPs()
	}
	ips, err := net.LookupIP(hostname)
	if err != nil {
		return nil, err
	}
	return ips, nil
}

// ReverseLookup performs a reverse DNS lookup on an IP address.
func ReverseLookup(ip net.IP) (string, error) {
	names, err := net.LookupAddr(ip.String())
	if err != nil || len(names) == 0 {
		return "", err
	}
	return names[0], nil
}

// ----------------------------------------------------------------------
// Network interface helpers
// ----------------------------------------------------------------------

// InterfaceIPs returns all IP addresses assigned to a specific network interface.
func InterfaceIPs(ifaceName string) ([]net.IP, error) {
	iface, err := net.InterfaceByName(ifaceName)
	if err != nil {
		return nil, err
	}
	addrs, err := iface.Addrs()
	if err != nil {
		return nil, err
	}
	var ips []net.IP
	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok {
			ips = append(ips, ipnet.IP)
		}
	}
	return ips, nil
}

// ----------------------------------------------------------------------
// String parsing helpers
// ----------------------------------------------------------------------

// SplitHostPort splits a string of the form "host:port" and returns
// host and port as separate strings. It handles IPv6 addresses with brackets.
func SplitHostPort(hostport string) (host, port string, err error) {
	return net.SplitHostPort(hostport)
}

// JoinHostPort combines host and port into a "host:port" string.
func JoinHostPort(host string, port int) string {
	return net.JoinHostPort(host, strconv.Itoa(port))
}

// ----------------------------------------------------------------------
// Example usage (commented)
// ----------------------------------------------------------------------
// func main() {
//     fmt.Println(netutil.GetLocalIP())
//     fmt.Println(netutil.IsPrivateIP(net.ParseIP("192.168.1.1"))) // true
//     port, _ := netutil.RandomPort()
//     fmt.Println("Random port:", port)
// }