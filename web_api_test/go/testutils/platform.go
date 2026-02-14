// Package platform provides detailed system identification including OS, architecture,
// Linux distribution, container detection, and hardware capabilities.
//
// All functions return "unknown" or zero values when detection fails, never panic.
package testutils

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"os"
	"os/user"
	"runtime"
	"strings"
)

// --------------------------------------------------------------------
// OS identification
// --------------------------------------------------------------------

// OS returns the runtime.GOOS value ("linux", "windows", "darwin", etc.).
func OS() string {
	return runtime.GOOS
}

// OSFamily returns the broad family of the operating system.
// Returns "windows", "unix", or "unknown".
func OSFamily() string {
	switch runtime.GOOS {
	case "windows":
		return "windows"
	case "darwin", "linux", "freebsd", "netbsd", "openbsd", "solaris", "illumos", "aix":
		return "unix"
	default:
		return "unknown"
	}
}

// IsWindows returns true if running on Windows.
func IsWindows() bool {
	return runtime.GOOS == "windows"
}

// IsLinux returns true if running on Linux.
func IsLinux() bool {
	return runtime.GOOS == "linux"
}

// IsMacOS returns true if running on macOS (Darwin).
func IsMacOS() bool {
	return runtime.GOOS == "darwin"
}

// IsUnix returns true if running on any Unix-like system (including macOS, Linux, BSD).
func IsUnix() bool {
	return OSFamily() == "unix"
}

// --------------------------------------------------------------------
// Architecture identification
// --------------------------------------------------------------------

// Arch returns the runtime.GOARCH value ("amd64", "arm64", "386", etc.).
func Arch() string {
	return runtime.GOARCH
}

// ArchBits returns the bitness of the architecture (32 or 64).
func ArchBits() int {
	switch runtime.GOARCH {
	case "386", "arm":
		return 32
	case "amd64", "arm64", "ppc64le", "s390x", "mips64", "mips64le", "riscv64":
		return 64
	default:
		return 0 // unknown
	}
}

// Endianness returns the byte order of the current architecture.
// Returns "little", "big", or "unknown".
func Endianness() string {
	var i uint32 = 0x01020304
	buf := bytes.NewBuffer(nil)
	if err := binary.Write(buf, binary.LittleEndian, i); err == nil {
		if bytes.Equal(buf.Bytes(), []byte{0x04, 0x03, 0x02, 0x01}) {
			return "little"
		}
	}
	buf.Reset()
	if err := binary.Write(buf, binary.BigEndian, i); err == nil {
		if bytes.Equal(buf.Bytes(), []byte{0x01, 0x02, 0x03, 0x04}) {
			return "big"
		}
	}
	return "unknown"
}

// --------------------------------------------------------------------
// Linux distribution detection
// --------------------------------------------------------------------

type DistroInfo struct {
	ID        string // "ubuntu", "debian", "fedora", "alpine", etc.
	Name      string // "Ubuntu", "Debian", "Fedora", etc.
	Version   string // "22.04", "11", etc.
	VersionID string // "22.04", "11", etc.
	Pretty    string // "Ubuntu 22.04 LTS"
}

// LinuxDistro attempts to detect the Linux distribution by reading /etc/os-release.
// Returns a DistroInfo struct with available fields; unknown fields are empty strings.
// On non‑Linux or if detection fails, returns zero values.
func LinuxDistro() DistroInfo {
	if runtime.GOOS != "linux" {
		return DistroInfo{}
	}
	f, err := os.Open("/etc/os-release")
	if err != nil {
		// Fallback: try /usr/lib/os-release
		f, err = os.Open("/usr/lib/os-release")
		if err != nil {
			return DistroInfo{}
		}
	}
	defer f.Close()

	info := DistroInfo{}
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := parts[0]
		value := strings.Trim(parts[1], "\"'")
		switch key {
		case "ID":
			info.ID = value
		case "NAME":
			info.Name = value
		case "VERSION":
			info.Version = value
		case "VERSION_ID":
			info.VersionID = value
		case "PRETTY_NAME":
			info.Pretty = value
		}
	}
	return info
}

// DistroString returns a short, human‑readable distribution identifier.
// Examples: "ubuntu22.04", "debian11", "alpine", "unknown".
func DistroString() string {
	if runtime.GOOS != "linux" {
		return "unknown"
	}
	info := LinuxDistro()
	if info.ID == "" {
		return "unknown"
	}
	if info.VersionID != "" {
		return info.ID + info.VersionID
	}
	return info.ID
}

// --------------------------------------------------------------------
// Container / virtualisation detection
// --------------------------------------------------------------------

// InContainer returns true if the process appears to be running inside a container.
// Detection is best‑effort: checks for /.dockerenv, /run/.containerenv, and cgroup hints.
func InContainer() bool {
	if _, err := os.Stat("/.dockerenv"); err == nil {
		return true
	}
	if _, err := os.Stat("/run/.containerenv"); err == nil {
		return true
	}
	// Check cgroup for Docker/k8s indicators
	data, err := os.ReadFile("/proc/self/cgroup")
	if err != nil {
		return false
	}
	content := string(data)
	return strings.Contains(content, "docker") ||
		strings.Contains(content, "kubepods") ||
		strings.Contains(content, "containerd") ||
		strings.Contains(content, "/lxc/")
}

// InWSL returns true if running under Windows Subsystem for Linux.
func InWSL() bool {
	if runtime.GOOS != "linux" {
		return false
	}
	data, err := os.ReadFile("/proc/sys/kernel/osrelease")
	if err != nil {
		return false
	}
	return strings.Contains(strings.ToLower(string(data)), "microsoft")
}

// --------------------------------------------------------------------
// User and host information
// --------------------------------------------------------------------

// Username returns the current user's username, or "unknown".
func Username() string {
	u, err := user.Current()
	if err != nil {
		return os.Getenv("USER") // fallback
	}
	return u.Username
}

// HomeDir returns the current user's home directory, or empty string if unknown.
func HomeDir() string {
	u, err := user.Current()
	if err != nil {
		return os.Getenv("HOME") // fallback
	}
	return u.HomeDir
}

// Hostname returns the system hostname, or "unknown".
func Hostname() string {
	h, err := os.Hostname()
	if err != nil {
		return "unknown"
	}
	return h
}

// --------------------------------------------------------------------
// Runtime and build information
// --------------------------------------------------------------------

// GoVersion returns the Go version used to build the binary.
func GoVersion() string {
	return runtime.Version()
}

// Compiler returns the compiler used to build the binary (usually "gc").
func Compiler() string {
	return runtime.Compiler
}

// NumCPU returns the number of logical CPUs.
func NumCPU() int {
	return runtime.NumCPU()
}

// NumGoroutine returns the current number of goroutines.
func NumGoroutine() int {
	return runtime.NumGoroutine()
}

// --------------------------------------------------------------------
// Environment detection
// --------------------------------------------------------------------

// IsCI attempts to detect if running in a continuous integration environment.
// Checks common CI environment variables.
func IsCI() bool {
	// List of common CI environment variables
	ciVars := []string{
		"CI", "GITHUB_ACTIONS", "GITLAB_CI", "JENKINS_URL",
		"TRAVIS", "CIRCLECI", "APPVEYOR", "BUILDKITE",
		"DRONE", "TEAMCITY_VERSION", "TF_BUILD", "bamboo_buildKey",
	}
	for _, v := range ciVars {
		if os.Getenv(v) != "" {
			return true
		}
	}
	return false
}

// IsDevelopment returns true if the environment suggests development mode.
// Currently checks if NODE_ENV=development or ENV=development, and not CI.
func IsDevelopment() bool {
	if IsCI() {
		return false
	}
	if os.Getenv("NODE_ENV") == "development" {
		return true
	}
	if os.Getenv("ENV") == "development" {
		return true
	}
	return false
}

// IsProduction returns true if the environment suggests production mode.
// Currently checks if ENV=production or not development.
func IsProduction() bool {
	if os.Getenv("ENV") == "production" {
		return true
	}
	return !IsDevelopment() && !IsCI()
}

// --------------------------------------------------------------------
// Summary string
// --------------------------------------------------------------------

// String returns a concise summary of the platform.
// Example: "linux/amd64 (ubuntu22.04) container=false"
func String() string {
	parts := []string{
		OS() + "/" + Arch(),
	}
	if IsLinux() {
		distro := DistroString()
		if distro != "unknown" {
			parts = append(parts, "("+distro+")")
		}
		if InWSL() {
			parts = append(parts, "wsl")
		}
	}
	if InContainer() {
		parts = append(parts, "container")
	}
	return strings.Join(parts, " ")
}
