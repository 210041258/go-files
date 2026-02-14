//go:build windows
// +build windows

package hostname

import (
	"syscall"
	"unsafe"
)

var (
	modkernel32 = syscall.NewLazyDLL("kernel32.dll")
	moddnsapi   = syscall.NewLazyDLL("dnsapi.dll")

	procGetComputerNameExW = modkernel32.NewProc("GetComputerNameExW")
	procDnsGetHostName     = moddnsapi.NewProc("DnsGetHostName")
)

const (
	ComputerNameDnsFullyQualified = 3
	ComputerNameDnsHostname       = 2
)

func init() {
	getSystemHostname = windowsSystemHostname
	getFQDN = windowsFQDN
	getSearchDomain = windowsSearchDomain
}

func windowsSystemHostname() (string, error) {
	// Use GetComputerNameEx with ComputerNameDnsHostname
	return windowsGetName(ComputerNameDnsHostname)
}

func windowsFQDN() (string, error) {
	// Try DnsGetHostName first (more reliable on some Windows versions)
	if fqdn, err := windowsDnsGetHostName(); err == nil {
		return fqdn, nil
	}
	// Fallback to GetComputerNameEx with FQDN
	return windowsGetName(ComputerNameDnsFullyQualified)
}

func windowsGetName(nameType uint32) (string, error) {
	var size uint32 = 0
	procGetComputerNameExW.Call(uintptr(nameType), 0, uintptr(unsafe.Pointer(&size)))
	if size == 0 {
		return "", syscall.GetLastError()
	}
	buf := make([]uint16, size)
	ret, _, err := procGetComputerNameExW.Call(
		uintptr(nameType),
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(unsafe.Pointer(&size)),
	)
	if ret == 0 {
		return "", err
	}
	return syscall.UTF16ToString(buf), nil
}

func windowsDnsGetHostName() (string, error) {
	var size uint32 = 256 // initial buffer size
	buf := make([]uint16, size)
	ret, _, err := procDnsGetHostName.Call(
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(size),
	)
	if ret == 0 {
		return "", err
	}
	return syscall.UTF16ToString(buf), nil
}

func windowsSearchDomain() string {
	// Windows doesn't have a global search domain like /etc/resolv.conf.
	// Could read from registry, but leave empty for now.
	return ""
}
