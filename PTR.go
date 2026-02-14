// Package testutils provides utilities for testing web APIs.
package testutils

import (
	"net"
)

// LookupPTR performs a reverse DNS lookup for the given IP address.
// It returns the first PTR record (canonical host name) and any error encountered.
// If multiple PTR records exist, only the first is returned.
func LookupPTR(ip string) (string, error) {
	names, err := net.LookupAddr(ip)
	if err != nil {
		return "", err
	}
	if len(names) == 0 {
		return "", nil // no PTR record
	}
	return names[0], nil
}

// LookupPTRs performs a reverse DNS lookup for the given IP address
// and returns all PTR records found.
func LookupPTRs(ip string) ([]string, error) {
	return net.LookupAddr(ip)
}

// HasPTR checks whether the given IP address has at least one PTR record.
func HasPTR(ip string) (bool, error) {
	names, err := net.LookupAddr(ip)
	if err != nil {
		// If the domain does not exist, LookupAddr returns an error.
		// We treat that as "no PTR" but return the error for inspection.
		return false, err
	}
	return len(names) > 0, nil
}