// Package testutils provides extended operating system utilities for file handling,
// path manipulation, environment variables, and system introspection.
//
// Features Context-aware I/O, buffered pooling for performance, and
// atomic file operations with disk sync guarantees.
package testutils

import (
    "bufio"
    "context"
    "errors"
    "fmt"
    "io"
    "io/fs"
    "os"
    "os/user"
    "path/filepath"
    "runtime"
    "strconv"
    "strings"
    "sync"
    "syscall"
    "time"
)

// --------------------------------------------------------------------
// Internal Performance Helpers
// --------------------------------------------------------------------

var (
    // bufferPool is used to reuse buffers for I/O operations to reduce GC overhead.
    bufferPool = sync.Pool{
        New: func() interface{} {
            // 32KB buffer is a good default for file copying (balances memory vs syscalls)
            b := make([]byte, 32*1024)
            return &b
        },
    }
)

// ctxReader wraps an io.Reader to respect context cancellation.
type ctxReader struct {
    ctx context.Context
    r   io.Reader
}

func (cr *ctxReader) Read(p []byte) (n int, err error) {
    if err := cr.ctx.Err(); err != nil {
        return 0, err
    }
    return cr.r.Read(p)
}

// getBuffer fetches a buffer from the pool.
func getBuffer() *[]byte {
    return bufferPool.Get().(*[]byte)
}

// putBuffer returns a buffer to the pool.
func putBuffer(b *[]byte) {
    // Reset capacity if it grew (unlikely with CopyBuffer usage, but safe practice)
    // In this specific usage, we just return it as is.
    bufferPool.Put(b)
}

// --------------------------------------------------------------------
// File and directory operations
// --------------------------------------------------------------------

// FileExists reports whether the named file or directory exists.
func FileExists(path string) bool {
    _, err := os.Stat(path)
    return err == nil || !os.IsNotExist(err)
}

// IsDir reports whether path exists and is a directory.
func IsDir(path string) bool {
    info, err := os.Stat(path)
    return err == nil && info.IsDir()
}

// IsFile reports whether path exists and is a regular file.
func IsFile(path string) bool {
    info, err := os.Stat(path)
    return err == nil && info.Mode().IsRegular()
}

// EnsureDir ensures that a directory exists at the given path,
// creating it with the specified permission if necessary.
// Returns the path itself for chaining.
func EnsureDir(path string, perm fs.FileMode) (string, error) {
    err := os.MkdirAll(path, perm)
    if err != nil {
        return "", fmt.Errorf("osutil: create directory %s: %w", path, err)
    }
    return path, nil
}

// EnsureFileDir ensures the parent directory of the given file path exists.
func EnsureFileDir(path string, perm fs.FileMode) (string, error) {
    dir := filepath.Dir(path)
    if dir != "" && dir != "." {
        if err := os.MkdirAll(dir, perm); err != nil {
            return "", fmt.Errorf("osutil: create parent directory %s: %w", dir, err)
        }
    }
    return path, nil
}

// CopyFile copies a file from src to dst.
// It respects context cancellation.
// If dst already exists, it will be overwritten. Permissions are copied from the source.
func CopyFile(ctx context.Context, src, dst string) error {
    source, err := os.Open(src)
    if err != nil {
        return fmt.Errorf("osutil: open source: %w", err)
    }
    defer source.Close()

    sourceInfo, err := source.Stat()
    if err != nil {
        return fmt.Errorf("osutil: stat source: %w", err)
    }

    dest, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, sourceInfo.Mode())
    if err != nil {
        return fmt.Errorf("osutil: create destination: %w", err)
    }
    defer dest.Close()

    // Use buffered copy with context support
    bufPtr := getBuffer()
    defer putBuffer(bufPtr)
    buf := *bufPtr

    cr := &ctxReader{ctx: ctx, r: source}
    if _, err := io.CopyBuffer(dest, cr, buf); err != nil {
        return fmt.Errorf("osutil: copy: %w", err)
    }
    return nil
}

// CopyDir recursively copies a directory from src to dst.
// It respects context cancellation. Symlinks are recreated as symlinks.
func CopyDir(ctx context.Context, src, dst string) error {
    srcInfo, err := os.Stat(src)
    if err != nil {
        return fmt.Errorf("osutil: stat source dir: %w", err)
    }
    if !srcInfo.IsDir() {
        return fmt.Errorf("osutil: source is not a directory: %s", src)
    }

    if err := os.MkdirAll(dst, srcInfo.Mode()); err != nil {
        return fmt.Errorf("osutil: create destination dir: %w", err)
    }

    entries, err := os.ReadDir(src)
    if err != nil {
        return fmt.Errorf("osutil: read source dir: %w", err)
    }

    for _, entry := range entries {
        select {
        case <-ctx.Done():
            return ctx.Err()
        default:
        }

        srcPath := filepath.Join(src, entry.Name())
        dstPath := filepath.Join(dst, entry.Name())

        info, err := entry.Info()
        if err != nil {
            return fmt.Errorf("osutil: get entry info: %w", err)
        }

        // Handle Symlinks
        if info.Mode()&fs.ModeSymlink != 0 {
            target, err := os.Readlink(srcPath)
            if err != nil {
                return fmt.Errorf("osutil: read symlink: %w", err)
            }
            if err := os.Symlink(target, dstPath); err != nil {
                return fmt.Errorf("osutil: create symlink: %w", err)
            }
            continue
        }

        if entry.IsDir() {
            if err := CopyDir(ctx, srcPath, dstPath); err != nil {
                return err
            }
        } else {
            if err := CopyFile(ctx, srcPath, dstPath); err != nil {
                return err
            }
        }
    }
    return nil
}

// MoveFile moves (renames) a file from src to dst.
// If the move fails (e.g., cross-device), it attempts a copy followed by a delete.
func MoveFile(src, dst string) error {
    err := os.Rename(src, dst)
    if err == nil {
        return nil
    }

    // Check for cross-device link error specifically to avoid retrying on permission errors
    var linkErr *os.LinkError
    if errors.As(err, &linkErr) && errors.Is(err, syscall.EXDEV) {
        // Fallback for cross-device moves
        if err := CopyFile(context.Background(), src, dst); err != nil {
            return fmt.Errorf("move fallback copy failed: %w", err)
        }
        return os.Remove(src)
    }

    return err
}

// TempFile creates a temporary file with a given pattern and content.
// It returns the path to the file.
func TempFile(pattern string, content []byte) (string, error) {
    f, err := os.CreateTemp("", pattern)
    if err != nil {
        return "", fmt.Errorf("osutil: create temp file: %w", err)
    }
    defer f.Close()
    if _, err := f.Write(content); err != nil {
        os.Remove(f.Name())
        return "", fmt.Errorf("osutil: write temp file: %w", err)
    }
    return f.Name(), nil
}

// TempDir creates a temporary directory and returns its path.
func TempDir(pattern string) (string, error) {
    return os.MkdirTemp("", pattern)
}

// --------------------------------------------------------------------
// Path and home directory utilities
// --------------------------------------------------------------------

// HomeDir returns the current user's home directory.
// Returns an error if the home directory cannot be determined.
func HomeDir() (string, error) {
    usr, err := user.Current()
    if err == nil && usr.HomeDir != "" {
        return usr.HomeDir, nil
    }
    // Fallback to environment variables
    if home := os.Getenv("HOME"); home != "" {
        return home, nil
    }
    if runtime.GOOS == "windows" {
        // Check USERPROFILE first
        if home := os.Getenv("USERPROFILE"); home != "" {
            return home, nil
        }
        // Fallback to HOMEDRIVE + HOMEPATH
        drive := os.Getenv("HOMEDRIVE")
        path := os.Getenv("HOMEPATH")
        if drive != "" && path != "" {
            return drive + path, nil
        }
    }
    return "", errors.New("osutil: cannot determine home directory")
}

// ExpandHome replaces a leading "~" with the user's home directory.
func ExpandHome(path string) string {
    if !strings.HasPrefix(path, "~") {
        return path
    }
    home, err := HomeDir()
    if err != nil {
        return path
    }
    if len(path) == 1 {
        return home
    }
    // Support both unix "/" and windows "\"
    if path[1] == '/' || path[1] == '\\' {
        return filepath.Join(home, path[2:])
    }
    return path
}

// Abs expands home and returns an absolute path.
func Abs(path string) (string, error) {
    path = ExpandHome(path)
    return filepath.Abs(path)
}

// --------------------------------------------------------------------
// Environment variable helpers
// --------------------------------------------------------------------

// EnvOr returns the value of the environment variable key,
// or defaultValue if the variable is not set or empty.
func EnvOr(key, defaultValue string) string {
    if v := os.Getenv(key); v != "" {
        return v
    }
    return defaultValue
}

// EnvInt parses an environment variable as an integer.
// Returns defaultValue if the variable is not set or invalid.
func EnvInt(key string, defaultValue int) int {
    if v := os.Getenv(key); v != "" {
        if i, err := strconv.Atoi(v); err == nil {
            return i
        }
    }
    return defaultValue
}

// EnvBool parses an environment variable as a boolean.
// Recognizes true/1/yes/on as true, false/0/no/off as false.
// Returns defaultValue if the variable is not set or unrecognized.
func EnvBool(key string, defaultValue bool) bool {
    v := os.Getenv(key)
    if v == "" {
        return defaultValue
    }
    switch strings.ToLower(v) {
    case "true", "1", "yes", "on", "y", "t":
        return true
    case "false", "0", "no", "off", "n", "f":
        return false
    default:
        return defaultValue
    }
}

// EnvMap returns all environment variables as a map.
func EnvMap() map[string]string {
    env := os.Environ()
    m := make(map[string]string, len(env))
    for _, e := range env {
        parts := strings.SplitN(e, "=", 2)
        if len(parts) == 2 {
            m[parts[0]] = parts[1]
        }
    }
    return m
}

// --------------------------------------------------------------------
// System information
// --------------------------------------------------------------------

// Hostname returns the system hostname.
func Hostname() string {
    name, _ := os.Hostname()
    return name
}

// NumCPU returns the number of logical CPUs usable.
func NumCPU() int {
    return runtime.NumCPU()
}

// GoVersion returns the Go version used to build the program.
func GoVersion() string {
    return runtime.Version()
}

// OS returns the operating system target.
func OS() string {
    return runtime.GOOS
}

// Arch returns the architecture target.
func Arch() string {
    return runtime.GOARCH
}

// --------------------------------------------------------------------
// Process information
// --------------------------------------------------------------------

// ProcessExists checks whether a process with the given PID exists.
func ProcessExists(pid int) bool {
    process, err := os.FindProcess(pid)
    if err != nil {
        return false
    }
    // On Unix, FindProcess always succeeds; we must signal to check existence.
    // On Windows, FindProcess actually checks for existence.
    if runtime.GOOS != "windows" {
        // Signal 0 checks for process existence without killing it.
        err = process.Signal(syscall.Signal(0))
    }
    return err == nil
}

// Executable returns the path of the current executable.
func Executable() (string, error) {
    exe, err := os.Executable()
    if err != nil {
        return "", fmt.Errorf("osutil: get executable path: %w", err)
    }
    return filepath.EvalSymlinks(exe)
}

// ExecutableDir returns the directory containing the current executable.
func ExecutableDir() (string, error) {
    exe, err := Executable()
    if err != nil {
        return "", err
    }
    return filepath.Dir(exe), nil
}

// --------------------------------------------------------------------
// Atomic file write (safe replacement)
// --------------------------------------------------------------------

// WriteFileAtomic atomically writes data to path.
// It writes to a temporary file in the same directory, Syncs it to disk,
// and then renames it over the target. This prevents partial writes and
// ensures crash consistency.
func WriteFileAtomic(ctx context.Context, path string, data []byte, perm fs.FileMode) error {
    dir := filepath.Dir(path)
    if err := EnsureDir(dir, 0755); err != nil {
        return err
    }
    
    // Create temp file in the same directory to ensure rename is atomic (same filesystem)
    tmp, err := os.CreateTemp(dir, filepath.Base(path)+".tmp-*")
    if err != nil {
        return fmt.Errorf("osutil: create temp file: %w", err)
    }
    tmpName := tmp.Name()
    
    // Ensure cleanup on failure
    var cleanup bool
    defer func() {
        if cleanup {
            os.Remove(tmpName)
        }
    }()

    bufPtr := getBuffer()
    defer putBuffer(bufPtr)
    buf := *bufPtr

    cr := &ctxReader{ctx: ctx, r: tmp} // Context check during write

    // Write data
    if _, err := io.CopyBuffer(tmp, io.NopCloser(cr), buf); err != nil {
        cleanup = true
        return fmt.Errorf("osutil: write temp file: %w", err)
    }

    // Sync to disk (Crucial for Enterprise safety: ensure data is physically written before rename)
    if err := tmp.Sync(); err != nil {
        cleanup = true
        return fmt.Errorf("osutil: sync temp file: %w", err)
    }

    if err := tmp.Chmod(perm); err != nil {
        cleanup = true
        return fmt.Errorf("osutil: chmod temp file: %w", err)
    }

    if err := tmp.Close(); err != nil {
        cleanup = true
        return fmt.Errorf("osutil: close temp file: %w", err)
    }

    // Atomic rename
    if err := os.Rename(tmpName, path); err != nil {
        cleanup = true
        return fmt.Errorf("osutil: rename temp file: %w", err)
    }

    // Success, do not cleanup the temp file (it is now the real file)
    cleanup = false
    return nil
}

// ReadFileAtomic reads a file that was written atomically.
func ReadFileAtomic(path string) ([]byte, error) {
    return os.ReadFile(path)
}

// --------------------------------------------------------------------
// Line reading utilities
// --------------------------------------------------------------------

// ReadLines reads a file and returns its lines as a slice.
// Trailing newlines are removed. Supports context cancellation.
func ReadLines(ctx context.Context, path string) ([]string, error) {
    f, err := os.Open(path)
    if err != nil {
        return nil, err
    }
    defer f.Close()

    var lines []string
    scanner := bufio.NewScanner(f)
    
    // Increase buffer size for very long lines if necessary
    // buf := make([]byte, 0, 64*1024)
    // scanner.Buffer(buf, 1024*1024)

    for scanner.Scan() {
        select {
        case <-ctx.Done():
            return nil, ctx.Err()
        default:
            lines = append(lines, scanner.Text())
        }
    }
    return lines, scanner.Err()
}

// WriteLines writes lines to a file, each followed by a newline.
func WriteLines(path string, lines []string, perm fs.FileMode) error {
    f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, perm)
    if err != nil {
        return err
    }
    defer f.Close()
    w := bufio.NewWriter(f)
    for _, line := range lines {
        if _, err := w.WriteString(line + "\n"); err != nil {
            return err
        }
    }
    return w.Flush()
}