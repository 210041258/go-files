// Package pathname provides utilities for safe filename generation, path
// manipulation, and naming conventions. It is cross‑platform and uses zero
// dependencies except for optional regexp support (build tag).
//
// Core features:
//   - Slugify: convert any string to a safe filename
//   - Unique names: timestamp, random, or counter‑based
//   - Temporary file/directory naming
//   - Extension manipulation
//   - Cross‑platform path conversion
//   - Human‑readable name generation
package testutils

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"strings"
	"time"
	"unicode"
)

// --------------------------------------------------------------------
// Slugify / Safe filename conversion
// --------------------------------------------------------------------

// Slugify converts a string to a safe filename: lowercase, hyphens instead
// of spaces, removes all non‑alphanumeric characters except hyphens.
//
// Example:
//   Slugify("Hello World! 123") // "hello-world-123"
func Slugify(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	space := false
	for _, r := range strings.ToLower(s) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			space = false
		} else if unicode.IsSpace(r) {
			if !space {
				b.WriteByte('-')
				space = true
			}
		} else if r == '-' || r == '_' {
			b.WriteRune(r)
			space = false
		}
		// all other punctuation removed
	}
	return strings.Trim(b.String(), "-")
}

// SafeName returns a filename that is safe for all operating systems.
// It optionally preserves case and allows extra safe characters.
func SafeName(s string, preserveCase bool, extraSafe ...rune) string {
	allow := make(map[rune]bool)
	for _, r := range extraSafe {
		allow[r] = true
	}
	var b strings.Builder
	b.Grow(len(s))
	if !preserveCase {
		s = strings.ToLower(s)
	}
	for _, r := range s {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(r)
		case r == ' ' || r == '-':
			b.WriteByte('-')
		case allow[r]:
			b.WriteRune(r)
		}
	}
	return strings.Trim(b.String(), "-_.")
}

// --------------------------------------------------------------------
// Unique name generation
// --------------------------------------------------------------------

// WithTimestamp appends a timestamp to the base name, before the extension.
// Format: name_20060102-150405.ext
func WithTimestamp(path string) string {
	ts := time.Now().Format("20060102-150405")
	return AppendBeforeExt(path, "_"+ts)
}

// WithRandom appends a random hex string to the base name, before the extension.
// If n <= 0, defaults to 8 characters.
func WithRandom(path string, n int) (string, error) {
	if n <= 0 {
		n = 8
	}
	bytes := make([]byte, (n+1)/2) // hex encoding doubles length
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	randStr := hex.EncodeToString(bytes)[:n]
	return AppendBeforeExt(path, "_"+randStr), nil
}

// WithCounter appends a zero‑padded counter to the base name, before the extension.
// Format: name_001.ext, name_002.ext, etc.
func WithCounter(path string, counter int, digits int) string {
	if digits <= 0 {
		digits = 3
	}
	format := fmt.Sprintf("_%%0%dd", digits)
	return AppendBeforeExt(path, fmt.Sprintf(format, counter))
}

// UniqueName generates a unique filename based on a pattern.
// Pattern placeholders:
//   {timestamp} -> 20060102-150405
//   {date}      -> 20060102
//   {time}      -> 150405
//   {rand:8}    -> random hex of length 8
//   {counter:3} -> zero‑padded counter (use WithCounter for external counter)
//
// Example:
//   UniqueName("backup-{date}-{rand:4}.db") // backup-20250321-a1f2.db
func UniqueName(pattern string) (string, error) {
	// Simple placeholder replacement
	result := pattern
	now := time.Now()
	result = strings.ReplaceAll(result, "{timestamp}", now.Format("20060102-150405"))
	result = strings.ReplaceAll(result, "{date}", now.Format("20060102"))
	result = strings.ReplaceAll(result, "{time}", now.Format("150405"))

	// Random placeholders: {rand:4}
	for {
		start := strings.Index(result, "{rand:")
		if start == -1 {
			break
		}
		end := strings.Index(result[start:], "}")
		if end == -1 {
			break
		}
		end += start
		lengthStr := result[start+6 : end]
		length := 8
		if n, err := fmt.Sscanf(lengthStr, "%d", &length); err == nil && n == 1 {
			// valid
		}
		randStr, err := randomHex(length)
		if err != nil {
			return "", err
		}
		result = result[:start] + randStr + result[end+1:]
	}
	return result, nil
}

// --------------------------------------------------------------------
// Extension manipulation
// --------------------------------------------------------------------

// AppendBeforeExt inserts text before the file extension.
// If the file has no extension, text is appended at the end.
func AppendBeforeExt(path, text string) string {
	ext := filepath.Ext(path)
	if ext == "" {
		return path + text
	}
	return path[:len(path)-len(ext)] + text + ext
}

// ReplaceExt changes the file extension. If ext does not start with a dot,
// one is added. If ext is empty, the extension is removed.
func ReplaceExt(path, ext string) string {
	if ext != "" && !strings.HasPrefix(ext, ".") {
		ext = "." + ext
	}
	base := strings.TrimSuffix(path, filepath.Ext(path))
	return base + ext
}

// EnsureExt ensures the path has the given extension. If it already has a
// different extension, it is replaced. If it has no extension, it is added.
func EnsureExt(path, ext string) string {
	if ext != "" && !strings.HasPrefix(ext, ".") {
		ext = "." + ext
	}
	current := filepath.Ext(path)
	if current == ext {
		return path
	}
	return strings.TrimSuffix(path, current) + ext
}

// Extensions returns the full list of extensions for a path (e.g., ".tar.gz").
func Extensions(path string) []string {
	base := filepath.Base(path)
	var exts []string
	for {
		ext := filepath.Ext(base)
		if ext == "" {
			break
		}
		exts = append([]string{ext}, exts...)
		base = strings.TrimSuffix(base, ext)
	}
	return exts
}

// --------------------------------------------------------------------
// Temporary paths (cross‑platform)
// --------------------------------------------------------------------

// TempName returns a path in the system temporary directory with an optional
// pattern. The file is not created; only the path is generated.
func TempName(pattern string) string {
	// filepath.Join doesn't accept pattern; we mimic os.CreateTemp behavior.
	dir := filepath.TempDir()
	if pattern == "" {
		pattern = "temp-{rand:6}"
	}
	name, _ := UniqueName(pattern) // error ignored; fallback
	if name == "" {
		name = fmt.Sprintf("temp-%d", time.Now().UnixNano())
	}
	return filepath.Join(dir, name)
}

// TempDirName returns a path for a temporary directory.
func TempDirName(pattern string) string {
	return TempName(pattern)
}

// --------------------------------------------------------------------
// Path conversion (Unix / Windows)
// --------------------------------------------------------------------

// ToUnix converts a Windows path to Unix style (backslashes to slashes).
// On non‑Windows, it returns the input unchanged.
func ToUnix(path string) string {
	return strings.ReplaceAll(path, "\\", "/")
}

// ToWindows converts a Unix path to Windows style (slashes to backslashes).
// On non‑Windows, it returns the input unchanged.
func ToWindows(path string) string {
	return strings.ReplaceAll(path, "/", "\\")
}

// ToNative converts a path to the native OS separator.
func ToNative(path string) string {
	if filepath.Separator == '/' {
		return ToUnix(path)
	}
	return ToWindows(path)
}

// --------------------------------------------------------------------
// URL‑friendly paths
// --------------------------------------------------------------------

// URLPath converts a file system path to a URL‑safe path.
// - Replaces backslashes with forward slashes
// - URL‑encodes characters? Usually not needed.
func URLPath(path string) string {
	clean := filepath.ToSlash(path)
	if !strings.HasPrefix(clean, "/") {
		clean = "/" + clean
	}
	return clean
}

// --------------------------------------------------------------------
// Human‑readable name generation
// --------------------------------------------------------------------

// Adjectives and nouns for human‑readable names.
var (
	adjectives = []string{
		"happy", "sad", "fast", "slow", "big", "small", "bright", "dark",
		"smart", "brave", "calm", "eager", "gentle", "proud", "witty",
		"ancient", "modern", "cozy", "wild", "mighty", "silent", "peaceful",
	}
	nouns = []string{
		"cat", "dog", "bird", "fish", "tree", "flower", "cloud", "star",
		"moon", "sun", "river", "mountain", "ocean", "forest", "desert",
		"panda", "tiger", "eagle", "whale", "dolphin", "lion", "wolf",
	}
)

// HumanName generates a random human‑readable name like "happy-panda-42".
// If separator is empty, defaults to "-". If rng is nil, a fast non‑cryptographic
// generator is used (see init).
func HumanName(separator string, rng interface{ RandIntn(n int) int }) string {
	if separator == "" {
		separator = "-"
	}
	adj := randomChoice(adjectives, rng)
	noun := randomChoice(nouns, rng)
	num := randomInt(1, 1000, rng)
	return fmt.Sprintf("%s%s%s%s%d", adj, separator, noun, separator, num)
}

// randomChoice picks a random element from a slice.
func randomChoice[T any](slice []T, rng interface{ RandIntn(n int) int }) T {
	var zero T
	if len(slice) == 0 {
		return zero
	}
	idx := randomInt(0, len(slice), rng)
	return slice[idx]
}

// randomInt returns a random int in [min, max).
func randomInt(min, max int, rng interface{ RandIntn(n int) int }) int {
	if max <= min {
		return min
	}
	if rng != nil {
		return min + rng.RandIntn(max-min)
	}
	// Fallback to default RNG
	return min + defaultRNG.Intn(max-min)
}

// default RNG – seeded with time
import (
	"math/rand"
	_ "unsafe" // for go:linkname
)

var defaultRNG *rand.Rand

func init() {
	defaultRNG = rand.New(rand.NewSource(time.Now().UnixNano()))
}

// --------------------------------------------------------------------
// Private helpers
// --------------------------------------------------------------------

// randomHex returns a random hex string of length n.
func randomHex(n int) (string, error) {
	bytes := make([]byte, (n+1)/2)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes)[:n], nil
}