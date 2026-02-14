// Package pkgutil provides utilities for working with Go package paths.
// It helps with extracting package names, checking standard library,
// splitting and joining import paths, and sanitizing for use in identifiers.
package testutils

import (
	"path"
	"strings"
)

// Name returns the base package name from a full import path.
// For example, "github.com/user/repo/subpkg" -> "subpkg".
// If the path ends with a version (like /v2), it is stripped before extracting the name.
func Name(importPath string) string {
	// Remove trailing version if present (e.g., /v2, /v3)
	importPath = stripVersion(importPath)
	// Take the last element of the path.
	base := path.Base(importPath)
	// If base is "." or "/", return empty.
	if base == "." || base == "/" || base == "" {
		return ""
	}
	return base
}

// IsStdLib reports whether the import path belongs to the Go standard library.
// It does a simple check against known standard library prefixes.
func IsStdLib(importPath string) bool {
	// If it contains a dot, it's probably not stdlib (except for "crypto/...", etc.)
	// We use a simple heuristic: if the first part contains a dot, it's external.
	firstPart := strings.SplitN(importPath, "/", 2)[0]
	if strings.Contains(firstPart, ".") {
		return false
	}
	// Also, standard library paths are never versioned like "v2".
	if strings.Contains(importPath, "/v") {
		// Could be a pseudo-version but we assume it's external.
		return false
	}
	// Otherwise, assume it's standard library.
	// This is not 100% accurate (e.g., "gopkg.in/yaml.v2" would pass firstPart check),
	// but for many cases it's sufficient.
	return true
}

// Split breaks an import path into its slash-separated components.
// It returns an empty slice for empty input.
func Split(importPath string) []string {
	if importPath == "" {
		return []string{}
	}
	return strings.Split(importPath, "/")
}

// Join combines components into an import path using slash separators.
func Join(parts []string) string {
	return strings.Join(parts, "/")
}

// Sanitize converts an import path into a string safe for use as an identifier
// (e.g., in generated code). It replaces slashes and dots with underscores.
func Sanitize(importPath string) string {
	sanitized := strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			return r
		default:
			return '_'
		}
	}, importPath)
	// Avoid leading underscore if the first character was sanitized.
	if len(sanitized) > 0 && sanitized[0] == '_' {
		sanitized = "x" + sanitized
	}
	return sanitized
}

// Parent returns the parent import path.
// For example, "github.com/user/repo/subpkg" -> "github.com/user/repo".
// If there is no parent, it returns an empty string.
func Parent(importPath string) string {
	// Strip version suffix before finding parent?
	// Usually we want the logical parent even with version.
	dir := path.Dir(importPath)
	if dir == "." || dir == "/" {
		return ""
	}
	return dir
}

// IsInternal reports whether the import path is under an "internal" directory.
func IsInternal(importPath string) bool {
	parts := Split(importPath)
	for _, part := range parts {
		if part == "internal" {
			return true
		}
	}
	return false
}

// IsVendored reports whether the import path appears to be from a vendor directory.
// This is a simple check for the presence of "vendor/" in the path.
func IsVendored(importPath string) bool {
	return strings.Contains(importPath, "/vendor/") ||
		strings.HasPrefix(importPath, "vendor/")
}

// stripVersion removes a trailing version component like "/v2", "/v3", etc.
// It does not handle major version suffixes in detail; it simply removes the last
// component if it starts with "v" followed by digits.
func stripVersion(importPath string) string {
	parts := Split(importPath)
	if len(parts) == 0 {
		return importPath
	}
	last := parts[len(parts)-1]
	if len(last) > 1 && last[0] == 'v' {
		// Check if remaining part is digits.
		allDigits := true
		for i := 1; i < len(last); i++ {
			if last[i] < '0' || last[i] > '9' {
				allDigits = false
				break
			}
		}
		if allDigits {
			return Join(parts[:len(parts)-1])
		}
	}
	return importPath
}

// ----------------------------------------------------------------------
// Example usage (commented)
// ----------------------------------------------------------------------
// func main() {
//     fmt.Println(pkgutil.Name("github.com/user/repo/subpkg")) // "subpkg"
//     fmt.Println(pkgutil.Name("github.com/user/repo/v2"))     // "repo"
//     fmt.Println(pkgutil.IsStdLib("fmt"))                     // true
//     fmt.Println(pkgutil.IsStdLib("github.com/user/repo"))    // false
//     fmt.Println(pkgutil.Sanitize("github.com/user/repo"))    // "github_com_user_repo"
//     fmt.Println(pkgutil.Parent("a/b/c"))                     // "a/b"
//     fmt.Println(pkgutil.IsInternal("a/b/internal/c"))        // true
// }