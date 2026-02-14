// Package testutils provides utilities for testing web APIs.
package testutils

import (
	"io"
	"os"
	"path/filepath"
	"testing"
)

// ------------------------------------------------------------------------
// File existence and type checks
// ------------------------------------------------------------------------

// FileExists reports whether the named file or directory exists.
func FileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// IsDir reports whether the path exists and is a directory.
func IsDir(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}

// IsFile reports whether the path exists and is a regular file.
func IsFile(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir()
}

// ------------------------------------------------------------------------
// Reading and writing
// ------------------------------------------------------------------------

// WriteFile writes data to the named file, creating it if necessary.
// It uses os.WriteFile with 0644 permissions.
func WriteFile(path string, data []byte) error {
	return os.WriteFile(path, data, 0644)
}

// WriteFileString writes a string to the named file.
func WriteFileString(path string, content string) error {
	return WriteFile(path, []byte(content))
}

// ReadFile reads the entire named file and returns the contents.
func ReadFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}

// ReadFileString reads the entire named file and returns its contents as a string.
func ReadFileString(path string) (string, error) {
	data, err := ReadFile(path)
	return string(data), err
}

// AppendToFile appends data to the named file, creating it if necessary.
func AppendToFile(path string, data []byte) error {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(data)
	return err
}

// Touch creates an empty file at path, or updates its modification time if it exists.
func Touch(path string) error {
	f, err := os.OpenFile(path, os.O_CREATE, 0644)
	if err != nil {
		return err
	}
	return f.Close()
}

// ------------------------------------------------------------------------
// Copying
// ------------------------------------------------------------------------

// CopyFile copies a file from src to dst. If dst already exists, it is truncated.
// File permissions are copied from the source.
func CopyFile(src, dst string) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}
	if !srcInfo.Mode().IsRegular() {
		return &os.PathError{Op: "copy", Path: src, Err: os.ErrInvalid}
	}
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()
	dstFile, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, srcInfo.Mode())
	if err != nil {
		return err
	}
	defer dstFile.Close()
	_, err = io.Copy(dstFile, srcFile)
	return err
}

// CopyDir recursively copies a directory tree from src to dst.
// Existing files in dst are overwritten.
func CopyDir(src, dst string) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}
	if !srcInfo.IsDir() {
		return &os.PathError{Op: "copy", Path: src, Err: os.ErrInvalid}
	}
	if err := os.MkdirAll(dst, srcInfo.Mode()); err != nil {
		return err
	}
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())
		if entry.IsDir() {
			if err := CopyDir(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			if err := CopyFile(srcPath, dstPath); err != nil {
				return err
			}
		}
	}
	return nil
}

// ------------------------------------------------------------------------
// Temporary directory with chdir
// ------------------------------------------------------------------------

// WithTempDir creates a new temporary directory, changes the working directory to it,
// and returns a cleanup function that restores the original working directory and
// removes the temporary directory. If t is non‑nil, the cleanup is automatically
// registered with t.Cleanup.
func WithTempDir(t testing.TB) (string, func()) {
	t.Helper()
	dir, cleanupDir := TempDir(t, "testutils-*")
	cleanupChdir := Chdir(t, dir)
	return dir, func() {
		cleanupChdir()
		cleanupDir()
	}
}

// ------------------------------------------------------------------------
// Must variants – panic on error
// ------------------------------------------------------------------------

// MustWriteFile calls WriteFile and panics on error.
func MustWriteFile(path string, data []byte) {
	if err := WriteFile(path, data); err != nil {
		panic("testutils: WriteFile failed: " + err.Error())
	}
}

// MustWriteFileString calls WriteFileString and panics on error.
func MustWriteFileString(path string, content string) {
	MustWriteFile(path, []byte(content))
}

// MustReadFile calls ReadFile and panics on error.
func MustReadFile(path string) []byte {
	data, err := ReadFile(path)
	if err != nil {
		panic("testutils: ReadFile failed: " + err.Error())
	}
	return data
}

// MustReadFileString calls ReadFileString and panics on error.
func MustReadFileString(path string) string {
	return string(MustReadFile(path))
}

// MustCopyFile calls CopyFile and panics on error.
func MustCopyFile(src, dst string) {
	if err := CopyFile(src, dst); err != nil {
		panic("testutils: CopyFile failed: " + err.Error())
	}
}

// MustCopyDir calls CopyDir and panics on error.
func MustCopyDir(src, dst string) {
	if err := CopyDir(src, dst); err != nil {
		panic("testutils: CopyDir failed: " + err.Error())
	}
}