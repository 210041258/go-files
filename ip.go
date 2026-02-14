// Package ip provides utilities for IP address and CIDR manipulation.
// It extends the net package with common operations like conversion,
// range calculation, and membership tests.
package ip

import (
	"errors"
	"fmt"
	"math/big"
	"net"
	"strconv"
	"strings"
)

// Common errors.
var (
	ErrInvalidIP        = errors.New("invalid IP address")
	ErrInvalidCIDR      = errors.New("invalid CIDR notation")
	ErrInvalidMask      = errors.New("invalid subnet mask")
	ErrUnsupportedIPv6  = errors.New("operation not supported for IPv6")
)

// ----------------------------------------------------------------------
// IP conversion (IPv4)
// ----------------------------------------------------------------------

// ToUint32 converts an IPv4 address to a uint32.
// Returns 0 and an error if the IP is not a valid IPv4 address.
func ToUint32(ip net.IP) (uint32, error) {
	ip = ip.To4()
	if ip == nil {
		return 0, fmt.Errorf("%w: not an IPv4 address", ErrInvalidIP)
	}
	return uint32(ip[0])<<24 | uint32(ip[1])<<16 | uint32(ip[2])<<8 | uint32(ip[3]), nil
}

// FromUint32 converts a uint32 to an IPv4 address.
func FromUint32(n uint32) net.IP {
	return net.IPv4(byte(n>>24), byte(n>>16), byte(n>>8), byte(n))
}

// ToInt converts an IP address to a big.Int (supports both IPv4 and IPv6).
func ToInt(ip net.IP) (*big.Int, error) {
	ip = ip.To16()
	if ip == nil {
		return nil, ErrInvalidIP
	}
	return new(big.Int).SetBytes(ip), nil
}

// FromInt converts a big.Int to an IP address (IPv4 or IPv6).
func FromInt(i *big.Int, bits int) net.IP {
	if bits == 32 {
		// IPv4
		return FromUint32(uint32(i.Uint64()))
	}
	// IPv6: 16 bytes
	bytes := i.Bytes()
	if len(bytes) < 16 {
		// Pad to 16 bytes
		padded := make([]byte, 16)
		copy(padded[16-len(bytes):], bytes)
		bytes = padded
	}
	return net.IP(bytes)
}

// ----------------------------------------------------------------------
// CIDR helpers
// ----------------------------------------------------------------------

// CIDR contains a parsed CIDR block.
type CIDR struct {
	IP      net.IP
	Network *net.IPNet
}

// ParseCIDR parses a CIDR string (e.g., "192.168.1.0/24") and returns a CIDR struct.
func ParseCIDR(cidr string) (*CIDR, error) {
	ip, network, err := net.ParseCIDR(cidr)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidCIDR, err)
	}
	return &CIDR{IP: ip, Network: network}, nil
}

// Contains reports whether the CIDR block contains the given IP.
func (c *CIDR) Contains(ip net.IP) bool {
	return c.Network.Contains(ip)
}

// Size returns the number of IP addresses in the CIDR block.
// For IPv4, it returns 2^(32-mask). For IPv6, it returns a big.Int.
func (c *CIDR) Size() *big.Int {
	ones, bits := c.Network.Mask.Size()
	if bits == 32 {
		// IPv4: 2^(32-ones)
		count := uint64(1) << uint(bits-ones)
		return new(big.Int).SetUint64(count)
	}
	// IPv6: 2^(128-ones)
	exp := bits - ones
	count := new(big.Int).Exp(big.NewInt(2), big.NewInt(int64(exp)), nil)
	return count
}

// FirstIP returns the first IP address in the CIDR block (the network address).
func (c *CIDR) FirstIP() net.IP {
	return c.Network.IP
}

// LastIP returns the last IP address in the CIDR block (the broadcast address).
func (c *CIDR) LastIP() net.IP {
	mask := c.Network.Mask
	network := c.Network.IP.To16()
	if network == nil {
		network = c.Network.IP
	}
	last := make(net.IP, len(network))
	for i := range network {
		last[i] = network[i] | ^mask[i]
	}
	return last
}

// NextIP returns the next IP address after the given IP.
// Returns nil if the increment would overflow.
func NextIP(ip net.IP) net.IP {
	ip = ip.To16()
	if ip == nil {
		return nil
	}
	next := make(net.IP, len(ip))
	copy(next, ip)
	for i := len(next) - 1; i >= 0; i-- {
		next[i]++
		if next[i] != 0 {
			break
		}
	}
	return next
}

// PrevIP returns the previous IP address before the given IP.
// Returns nil if the decrement would underflow.
func PrevIP(ip net.IP) net.IP {
	ip = ip.To16()
	if ip == nil {
		return nil
	}
	prev := make(net.IP, len(ip))
	copy(prev, ip)
	for i := len(prev) - 1; i >= 0; i-- {
		prev[i]--
		if prev[i] != 255 {
			break
		}
	}
	return prev
}

// Range returns the inclusive start and end IPs of the CIDR block.
func (c *CIDR) Range() (start, end net.IP) {
	return c.FirstIP(), c.LastIP()
}

// ----------------------------------------------------------------------
// IPSet: a set of IP addresses or CIDR blocks for fast membership tests.
// ----------------------------------------------------------------------

// IPSet represents a set of IP addresses and CIDR ranges.
type IPSet struct {
	ranges   []*net.IPNet
	singles  map[string]struct{} // string representation of IP
}

// NewIPSet creates an empty IPSet.
func NewIPSet() *IPSet {
	return &IPSet{
		singles: make(map[string]struct{}),
	}
}

// AddIP adds a single IP to the set.
func (s *IPSet) AddIP(ip net.IP) {
	s.singles[ip.String()] = struct{}{}
}

// AddCIDR adds a CIDR block to the set.
func (s *IPSet) AddCIDR(cidr *net.IPNet) {
	s.ranges = append(s.ranges, cidr)
}

// AddCIDRString parses and adds a CIDR string.
func (s *IPSet) AddCIDRString(cidr string) error {
	_, ipnet, err := net.ParseCIDR(cidr)
	if err != nil {
		return err
	}
	s.AddCIDR(ipnet)
	return nil
}

// Contains reports whether the set contains the given IP.
func (s *IPSet) Contains(ip net.IP) bool {
	// Check singles first (fast map lookup)
	if _, ok := s.singles[ip.String()]; ok {
		return true
	}
	// Check ranges
	for _, r := range s.ranges {
		if r.Contains(ip) {
			return true
		}
	}
	return false
}

// Len returns the total number of single IPs and CIDR blocks.
func (s *IPSet) Len() int {
	return len(s.singles) + len(s.ranges)
}

// ----------------------------------------------------------------------
// Validation and parsing
// ----------------------------------------------------------------------

// IsIPv4 reports whether the IP is an IPv4 address.
func IsIPv4(ip net.IP) bool {
	return ip.To4() != nil
}

// IsIPv6 reports whether the IP is an IPv6 address.
func IsIPv6(ip net.IP) bool {
	return ip.To16() != nil && ip.To4() == nil
}

// IsUnspecified reports whether the IP is the unspecified address (0.0.0.0 or ::).
func IsUnspecified(ip net.IP) bool {
	return ip.IsUnspecified()
}

// IsLoopback reports whether the IP is a loopback address.
func IsLoopback(ip net.IP) bool {
	return ip.IsLoopback()
}

// IsPrivate reports whether the IP is a private IPv4 address (RFC 1918).
// For IPv6, it checks for unique local addresses (fc00::/7).
func IsPrivate(ip net.IP) bool {
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
		return false
	}
	// IPv6: Unique Local Address (fc00::/7)
	return len(ip) == 16 && ip[0]&0xfe == 0xfc
}

// ParseIPOrCIDR parses a string that could be either an IP address or a CIDR.
// Returns a net.IP if it's a single IP, or a *net.IPNet if it's a CIDR.
func ParseIPOrCIDR(s string) (interface{}, error) {
	if strings.Contains(s, "/") {
		ip, network, err := net.ParseCIDR(s)
		if err != nil {
			return nil, err
		}
		// Return the CIDR with the network IP
		return &CIDR{IP: ip, Network: network}, nil
	}
	ip := net.ParseIP(s)
	if ip == nil {
		return nil, ErrInvalidIP
	}
	return ip, nil
}

// MustParseIPOrCIDR panics if s is not a valid IP or CIDR.
func MustParseIPOrCIDR(s string) interface{} {
	v, err := ParseIPOrCIDR(s)
	if err != nil {
		panic(err)
	}
	return v
}

// ----------------------------------------------------------------------
// IP range expansion
// ----------------------------------------------------------------------

// ExpandCIDR returns a slice of all IP addresses in the given CIDR block.
// WARNING: This can be huge for large blocks (e.g., /16 expands to 65536 IPs).
// Use with caution.
func ExpandCIDR(cidr string) ([]net.IP, error) {
	c, err := ParseCIDR(cidr)
	if err != nil {
		return nil, err
	}
	var ips []net.IP
	start := c.FirstIP()
	end := c.LastIP()
	for {
		ips = append(ips, start)
		if start.Equal(end) {
			break
		}
		start = NextIP(start)
	}
	return ips, nil
}

// ----------------------------------------------------------------------
// String formatting
// ----------------------------------------------------------------------

// IPRangeString returns a human-readable string of an IP range.
func IPRangeString(start, end net.IP) string {
	return fmt.Sprintf("%s-%s", start, end)
}

// ----------------------------------------------------------------------
// Example usage (commented)
// ----------------------------------------------------------------------
// func main() {
//     cidr, _ := ip.ParseCIDR("192.168.1.0/29")
//     fmt.Println(cidr.FirstIP())  // 192.168.1.0
//     fmt.Println(cidr.LastIP())   // 192.168.1.7
//     fmt.Println(cidr.Size())     // 8
//
//     set := ip.NewIPSet()
//     set.AddIP(net.ParseIP("10.0.0.1"))
//     set.AddCIDRString("10.0.0.0/24")
//     fmt.Println(set.Contains(net.ParseIP("10.0.0.1"))) // true
// }