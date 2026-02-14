// Package testutils provides utilities for testing web APIs.
package testutils

import (
	"fmt"
	"os"
	"path/filepath"
)

// ------------------------------------------------------------------------
// Symlink operations
// ------------------------------------------------------------------------

// IsSymlink reports whether the path exists and is a symbolic link.
// It does not follow the link; it checks the link itself.
func IsSymlink(path string) bool {
	info, err := os.Lstat(path)
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeSymlink != 0
}

// SymlinkTarget returns the target of the symbolic link at path.
// It returns an error if path is not a symlink or cannot be read.
func SymlinkTarget(path string) (string, error) {
	target, err := os.Readlink(path)
	if err != nil {
		return "", fmt.Errorf("read symlink %s: %w", path, err)
	}
	return target, nil
}

// CreateSymlink creates a symbolic link at link pointing to target.
// If force is true and link already exists (as any file type), it is removed first.
// On Windows, creating symlinks may require special privileges (developer mode or admin).
func CreateSymlink(target, link string, force bool) error {
	if force {
		if err := os.RemoveAll(link); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove existing link: %w", err)
		}
	}
	// Ensure parent directory exists.
	if err := os.MkdirAll(filepath.Dir(link), 0755); err != nil {
		return fmt.Errorf("create parent directory: %w", err)
	}
	return os.Symlink(target, link)
}

// ------------------------------------------------------------------------
// Hard link operations
// ------------------------------------------------------------------------

// CreateHardLink creates a hard link at link pointing to target.
// Both target and link must be on the same filesystem.
// If force is true and link already exists, it is removed first.
func CreateHardLink(target, link string, force bool) error {
	if force {
		if err := os.RemoveAll(link); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove existing link: %w", err)
		}
	}
	if err := os.MkdirAll(filepath.Dir(link), 0755); err != nil {
		return fmt.Errorf("create parent directory: %w", err)
	}
	return os.Link(target, link)
}

// ------------------------------------------------------------------------
// Temporary symlink helpers
// ------------------------------------------------------------------------

// TempSymlink creates a new symbolic link pointing to target in the
// system's temporary directory (or the directory specified by dir).
// The link name is generated using pattern (same semantics as ioutil.TempFile).
// It returns the full path to the created symlink and a cleanup function
// that removes it. If t is non‑nil, the cleanup is automatically registered
// with t.Cleanup.
func TempSymlink(t TestingT, dir, target, pattern string) (string, func()) {
	t.Helper()
	if dir == "" {
		dir = os.TempDir()
	}
	// Create a temporary file name (we'll delete it and create a symlink with the same name).
	f, err := os.CreateTemp(dir, pattern)
	if err != nil {
		t.Fatalf("TempSymlink: create temp file: %v", err)
	}
	tmpName := f.Name()
	f.Close()
	os.Remove(tmpName) // we only wanted the name

	// Now create the symlink.
	if err := os.Symlink(target, tmpName); err != nil {
		// Clean up the name if symlink creation failed.
		os.Remove(tmpName)
		t.Fatalf("TempSymlink: create symlink: %v", err)
	}
	cleanup := func() {
		if err := os.Remove(tmpName); err != nil && !os.IsNotExist(err) {
			t.Errorf("TempSymlink: failed to remove %s: %v", tmpName, err)
		}
	}
	if t != nil {
		t.Cleanup(cleanup)
	}
	return tmpName, cleanup
}

// ------------------------------------------------------------------------
// Must variants – panic on error
// ------------------------------------------------------------------------

// MustSymlinkTarget calls SymlinkTarget and panics on error.
func MustSymlinkTarget(path string) string {
	target, err := SymlinkTarget(path)
	if err != nil {
		panic("testutils: SymlinkTarget failed: " + err.Error())
	}
	return target
}

// MustCreateSymlink calls CreateSymlink and panics on error.
func MustCreateSymlink(target, link string, force bool) {
	if err := CreateSymlink(target, link, force); err != nil {
		panic("testutils: CreateSymlink failed: " + err.Error())
	}
}

// MustCreateHardLink calls CreateHardLink and panics on error.
func MustCreateHardLink(target, link string, force bool) {
	if err := CreateHardLink(target, link, force); err != nil {
		panic("testutils: CreateHardLink failed: " + err.Error())
	}
}