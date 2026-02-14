// Package testutils provides utilities for testing web APIs.
package testutils

import (
	"fmt"
	"net"
	"strings"

	"github.com/miekg/dns"
)

// SOARecord represents the fields of a DNS Start of Authority record.
type SOARecord struct {
	MName   string // primary master name server
	RName   string // responsible party's email (as a domain name)
	Serial  uint32 // serial number
	Refresh uint32 // refresh interval
	Retry   uint32 // retry interval
	Expire  uint32 // expire interval
	Minimum uint32 // minimum TTL
}

// String returns a human-readable representation of the SOA record.
func (s *SOARecord) String() string {
	return fmt.Sprintf("MName=%s, RName=%s, Serial=%d, Refresh=%d, Retry=%d, Expire=%d, Minimum=%d",
		s.MName, s.RName, s.Serial, s.Refresh, s.Retry, s.Expire, s.Minimum)
}

// LookupSOA performs a DNS query for the SOA record of the given domain
// using the specified nameserver (formatted as "host:port", e.g., "8.8.8.8:53").
// Returns the SOA record or an error if the query fails or no SOA record exists.
func LookupSOA(domain string, nameserver string) (*SOARecord, error) {
	c := new(dns.Client)
	m := new(dns.Msg)

	// Ensure domain is fully qualified (add trailing dot if not present)
	if !strings.HasSuffix(domain, ".") {
		domain = domain + "."
	}

	m.SetQuestion(domain, dns.TypeSOA)
	m.RecursionDesired = true

	r, _, err := c.Exchange(m, nameserver)
	if err != nil {
		return nil, err
	}
	if r.Rcode != dns.RcodeSuccess {
		return nil, fmt.Errorf("DNS query failed: %s", dns.RcodeToString[r.Rcode])
	}

	for _, ans := range r.Answer {
		if soa, ok := ans.(*dns.SOA); ok {
			return &SOARecord{
				MName:   soa.Ns,
				RName:   soa.Mbox,
				Serial:  soa.Serial,
				Refresh: soa.Refresh,
				Retry:   soa.Retry,
				Expire:  soa.Expire,
				Minimum: soa.Minttl,
			}, nil
		}
	}
	return nil, fmt.Errorf("no SOA record found for %s", domain)
}

// LookupSOAWithDefaultNS tries to find an authoritative nameserver for the domain
// and then queries that server for the SOA record. This is a convenience function
// that uses public resolvers as a fallback.
// NOTE: This is a best‑effort function; for deterministic tests, prefer specifying
// a known nameserver directly with LookupSOA.
func LookupSOAWithDefaultNS(domain string) (*SOARecord, error) {
	// Try to get the nameservers for the domain itself (may give authoritative servers)
	ns, err := net.LookupNS(domain)
	if err == nil && len(ns) > 0 {
		// Use the first nameserver found
		server := net.JoinHostPort(ns[0].Host, "53")
		soa, err := LookupSOA(domain, server)
		if err == nil {
			return soa, nil
		}
	}
	// Fallback to Google's public DNS
	return LookupSOA(domain, "8.8.8.8:53")
}

// MockSOA returns a synthetic SOA record suitable for tests.
// It does not perform any network lookup.
func MockSOA(domain string) *SOARecord {
	// Convert domain to a zone‑file format: replace dots with dots? We'll keep as is.
	// The responsible email is often the admin's email with '@' replaced by '.'.
	// For example, "admin.example.com" for admin@example.com.
	rname := "hostmaster." + strings.TrimSuffix(domain, ".")
	if !strings.HasSuffix(rname, ".") {
		rname = rname + "."
	}
	return &SOARecord{
		MName:   "ns1." + domain,
		RName:   rname,
		Serial:  20250101,
		Refresh: 3600,
		Retry:   600,
		Expire:  86400,
		Minimum: 300,
	}
}