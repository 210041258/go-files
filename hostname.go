// Package hostname provides cross‑platform hostname and domain detection.
// It returns the system hostname, fully qualified domain name (FQDN),
// short name, domain, and associated IP addresses.
//
// All functions return sensible defaults or empty strings on error.
package testutils

import (
	"net"
	"strings"
)

// Info contains the full hostname information.
type Info struct {
	Hostname   string   // full hostname as reported by the kernel
	Short      string   // first component before the first dot
	Domain     string   // domain part after the first dot (may be empty)
	FQDN       string   // fully qualified domain name (best effort)
	Addresses  []net.IP // IP addresses associated with the hostname
}

// Get returns the system hostname as reported by the kernel.
// It never panics; returns empty string on error.
func Get() string {
	name, err := getSystemHostname()
	if err != nil {
		return ""
	}
	return name
}

// Short returns the hostname without domain component.
// Example: "webserver" from "webserver.example.com".
func Short() string {
	name := Get()
	if idx := strings.IndexByte(name, '.'); idx != -1 {
		return name[:idx]
	}
	return name
}

// Domain returns the domain component of the hostname.
// Example: "example.com" from "webserver.example.com".
func Domain() string {
	name := Get()
	if idx := strings.IndexByte(name, '.'); idx != -1 {
		return name[idx+1:]
	}
	return ""
}

// FQDN returns the fully qualified domain name.
// This is a best‑effort detection; may fall back to Get() on failure.
func FQDN() string {
	fqdn, err := getFQDN()
	if err != nil {
		return Get()
	}
	return fqdn
}

// Info returns a complete Info struct with all available details.
func Info() *Info {
	host := Get()
	fqdn := FQDN()
	short := Short()
	domain := Domain()
	addrs, _ := net.LookupIP(fqdn)

	return &Info{
		Hostname:  host,
		Short:     short,
		Domain:    domain,
		FQDN:      fqdn,
		Addresses: addrs,
	}
}

// --------------------------------------------------------------------
// Platform‑specific implementations (see *_platform.go)
// --------------------------------------------------------------------

var (
	getSystemHostname func() (string, error)
	getFQDN           func() (string, error)
)

func init() {
	// Default fallbacks (overridden by build tags)
	getSystemHostname = fallbackSystemHostname
	getFQDN           = fallbackFQDN
}

// fallbackSystemHostname uses os.Hostname (works everywhere).
func fallbackSystemHostname() (string, error) {
	return osHostname()
}

// fallbackFQDN attempts to build FQDN by appending domain to short hostname.
func fallbackFQDN() (string, error) {
	h, err := fallbackSystemHostname()
	if err != nil {
		return "", err
	}
	// If already contains dot, assume it's FQDN
	if strings.Contains(h, ".") {
		return h, nil
	}
	// Try to get domain via search domain or config
	domain := getSearchDomain()
	if domain != "" {
		return h + "." + domain, nil
	}
	return h, nil
}

// getSearchDomain attempts to read the system's DNS search domain.
// Platform‑specific implementations override this.
var getSearchDomain = fallbackSearchDomain

func fallbackSearchDomain() string {
	// No generic fallback
	return ""
}

// Helper to access os.Hostname without import cycle.
import "os"
func osHostname() (string, error) { return os.Hostname() }