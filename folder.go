// Package testutils provides utilities for testing web APIs.
package testutils

import (
	"io/ioutil"
	"os"
	"path/filepath"
)

// ------------------------------------------------------------------------
// Basic directory operations
// ------------------------------------------------------------------------

// EnsureDir creates a directory at path and any necessary parents.
// It returns nil if the directory already exists or was successfully created.
func EnsureDir(path string) error {
	return os.MkdirAll(path, 0755)
}

// RemoveAll removes the named path and any children it contains.
// It ignores "not exist" errors.
func RemoveAll(path string) error {
	err := os.RemoveAll(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// IsEmptyDir reports whether the path exists and is an empty directory.
// If the path does not exist or is not a directory, it returns false.
func IsEmptyDir(path string) bool {
	info, err := os.Stat(path)
	if err != nil || !info.IsDir() {
		return false
	}
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	_, err = f.Readdirnames(1)
	// If we read at least one name, directory is not empty.
	return err == io.EOF
}

// ------------------------------------------------------------------------
// Directory content inspection
// ------------------------------------------------------------------------

// ListFiles returns a slice of file names (not directories) in the given directory.
// It does not recurse into subdirectories.
func ListFiles(dir string) ([]string, error) {
	entries, err := ioutil.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var files []string
	for _, e := range entries {
		if !e.IsDir() {
			files = append(files, e.Name())
		}
	}
	return files, nil
}

// ListDirs returns a slice of subdirectory names in the given directory.
// It does not recurse.
func ListDirs(dir string) ([]string, error) {
	entries, err := ioutil.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var dirs []string
	for _, e := range entries {
		if e.IsDir() {
			dirs = append(dirs, e.Name())
		}
	}
	return dirs, nil
}

// DirSize computes the total size in bytes of all files under root,
// including files in subdirectories. It does not follow symlinks.
func DirSize(root string) (int64, error) {
	var total int64
	err := filepath.Walk(root, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			total += info.Size()
		}
		return nil
	})
	return total, err
}

// FileCount returns the total number of regular files under root,
// including files in subdirectories. Directories themselves are not counted.
func FileCount(root string) (int, error) {
	var count int
	err := filepath.Walk(root, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			count++
		}
		return nil
	})
	return count, err
}

// ------------------------------------------------------------------------
// Temporary directory helpers
// ------------------------------------------------------------------------

// TempSubDir creates a new temporary directory inside parent.
// The directory name is generated using pattern (same semantics as ioutil.TempDir).
// It returns the full path to the new directory. The caller is responsible for
// cleaning it up. To automatically clean up, use TempSubDirWithCleanup.
func TempSubDir(parent, pattern string) (string, error) {
	return ioutil.TempDir(parent, pattern)
}

// TempSubDirWithCleanup creates a temporary subdirectory inside parent
// and registers a cleanup function that removes it. If t is non‑nil,
// the cleanup is automatically registered with t.Cleanup.
func TempSubDirWithCleanup(t testingTB, parent, pattern string) (string, func()) {
	t.Helper()
	dir, err := ioutil.TempDir(parent, pattern)
	if err != nil {
		t.Fatalf("TempSubDirWithCleanup: %v", err)
	}
	cleanup := func() {
		if err := os.RemoveAll(dir); err != nil && !os.IsNotExist(err) {
			t.Errorf("failed to remove temp dir %s: %v", dir, err)
		}
	}
	if t != nil {
		t.Cleanup(cleanup)
	}
	return dir, cleanup
}

// ------------------------------------------------------------------------
// Must variants – panic on error
// ------------------------------------------------------------------------

// MustEnsureDir calls EnsureDir and panics on error.
func MustEnsureDir(path string) {
	if err := EnsureDir(path); err != nil {
		panic("testutils: EnsureDir failed: " + err.Error())
	}
}

// MustRemoveAll calls RemoveAll and panics on error.
func MustRemoveAll(path string) {
	if err := RemoveAll(path); err != nil {
		panic("testutils: RemoveAll failed: " + err.Error())
	}
}

// MustListFiles calls ListFiles and panics on error.
func MustListFiles(dir string) []string {
	files, err := ListFiles(dir)
	if err != nil {
		panic("testutils: ListFiles failed: " + err.Error())
	}
	return files
}

// MustListDirs calls ListDirs and panics on error.
func MustListDirs(dir string) []string {
	dirs, err := ListDirs(dir)
	if err != nil {
		panic("testutils: ListDirs failed: " + err.Error())
	}
	return dirs
}

// MustDirSize calls DirSize and panics on error.
func MustDirSize(root string) int64 {
	size, err := DirSize(root)
	if err != nil {
		panic("testutils: DirSize failed: " + err.Error())
	}
	return size
}

// MustFileCount calls FileCount and panics on error.
func MustFileCount(root string) int {
	count, err := FileCount(root)
	if err != nil {
		panic("testutils: FileCount failed: " + err.Error())
	}
	return count
}