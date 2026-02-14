//go:build unix || linux || darwin || freebsd || netbsd || openbsd || solaris
// +build unix linux darwin freebsd netbsd openbsd solaris

package testutils

import (
	"bufio"
	"os"
	"os/exec"
	"strings"
)

func init() {
	getSystemHostname = unixSystemHostname
	getFQDN = unixFQDN
	getSearchDomain = unixSearchDomain
}

func unixSystemHostname() (string, error) {
	return osHostname()
}

func unixFQDN() (string, error) {
	// Try `hostname -f` first (most reliable on Unix)
	if fqdn, err := exec.Command("hostname", "-f").Output(); err == nil {
		return strings.TrimSpace(string(fqdn)), nil
	}
	// Fallback to building from hostname and domain
	h, err := unixSystemHostname()
	if err != nil {
		return "", err
	}
	if strings.Contains(h, ".") {
		return h, nil
	}
	domain := unixSearchDomain()
	if domain != "" {
		return h + "." + domain, nil
	}
	return h, nil
}

func unixSearchDomain() string {
	// Read /etc/resolv.conf for search/domain directive
	f, err := os.Open("/etc/resolv.conf")
	if err != nil {
		return ""
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "search ") {
			fields := strings.Fields(line)
			if len(fields) > 1 {
				return fields[1]
			}
		}
		if strings.HasPrefix(line, "domain ") {
			fields := strings.Fields(line)
			if len(fields) > 1 {
				return fields[1]
			}
		}
	}
	return ""
}
