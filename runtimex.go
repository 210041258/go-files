// Package runtimex provides high‑level runtime introspection and control.
// It includes memory statistics, goroutine info, stack dumps, GC controls,
// profiling, and build information – all with safe, panic‑free APIs.
package testutils

import (
	"fmt"
	"os"
	"runtime"
	"runtime/debug"
	"runtime/pprof"
	"strconv"
	"strings"
	"time"
)

// --------------------------------------------------------------------
// Memory statistics
// --------------------------------------------------------------------

// MemStats returns a snapshot of the current memory statistics.
// It is a thin wrapper around runtime.ReadMemStats.
func MemStats() *runtime.MemStats {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return &m
}

// MemSummary returns a human‑readable summary of current memory usage.
// Format: "alloc=X MiB, totalAlloc=Y MiB, sys=Z MiB, gc=W"
func MemSummary() string {
	m := MemStats()
	const unit = 1024 * 1024
	return fmt.Sprintf("alloc=%d MiB, totalAlloc=%d MiB, sys=%d MiB, gc=%d",
		m.Alloc/unit, m.TotalAlloc/unit, m.Sys/unit, m.NumGC)
}

// HeapAlloc returns the current heap allocation in bytes.
func HeapAlloc() uint64 {
	return MemStats().HeapAlloc
}

// HeapObjects returns the current number of objects on the heap.
func HeapObjects() uint64 {
	return MemStats().HeapObjects
}

// NumGC returns the number of completed GC cycles.
func NumGC() uint32 {
	return MemStats().NumGC
}

// LastGCTime returns the time of the last garbage collection.
func LastGCTime() time.Time {
	return time.Unix(0, int64(MemStats().LastGC))
}

// --------------------------------------------------------------------
// Goroutine information
// --------------------------------------------------------------------

// NumGoroutine returns the current number of goroutines.
func NumGoroutine() int {
	return runtime.NumGoroutine()
}

// GoroutineDump returns a stack trace of all goroutines, similar to
// what is printed on panic or SIGQUIT. It is a wrapper around
// runtime.Stack with all=true.
func GoroutineDump() []byte {
	buf := make([]byte, 64*1024)
	n := runtime.Stack(buf, true)
	for n == len(buf) {
		buf = make([]byte, len(buf)*2)
		n = runtime.Stack(buf, true)
	}
	return buf[:n]
}

// PrintGoroutineDump prints the full goroutine dump to stderr.
func PrintGoroutineDump() {
	os.Stderr.Write(GoroutineDump())
}

// GoroutineIDs returns the IDs of all goroutines. This is slow and
// requires parsing a stack dump; use sparingly.
func GoroutineIDs() []int64 {
	dump := string(GoroutineDump())
	lines := strings.Split(dump, "\n")
	var ids []int64
	for _, line := range lines {
		if strings.HasPrefix(line, "goroutine ") {
			parts := strings.Fields(line)
			if len(parts) > 1 {
				if id, err := strconv.ParseInt(parts[1], 10, 64); err == nil {
					ids = append(ids, id)
				}
			}
		}
	}
	return ids
}

// --------------------------------------------------------------------
// Garbage collection control
// --------------------------------------------------------------------

// GC runs a full garbage collection immediately.
func GC() {
	runtime.GC()
	debug.FreeOSMemory() // attempt to return memory to OS
}

// SetGCPercent sets the garbage collection target percentage.
// A negative percentage disables GC.
// Returns the previous setting.
func SetGCPercent(percent int) int {
	return debug.SetGCPercent(percent)
}

// EnableGC enables the garbage collector (sets GC percent to 100).
func EnableGC() {
	debug.SetGCPercent(100)
}

// DisableGC disables the garbage collector (sets GC percent to -1).
func DisableGC() {
	debug.SetGCPercent(-1)
}

// FreeOSMemory forces a garbage collection and then tries to return
// as much memory to the OS as possible.
func FreeOSMemory() {
	debug.FreeOSMemory()
}

// --------------------------------------------------------------------
// Profiling
// --------------------------------------------------------------------

// StartCPUProfile enables CPU profiling to the given file.
// It returns a stop function that must be called to flush and stop.
func StartCPUProfile(filename string) (stop func() error, err error) {
	f, err := os.Create(filename)
	if err != nil {
		return nil, fmt.Errorf("runtimex: create CPU profile: %w", err)
	}
	if err := pprof.StartCPUProfile(f); err != nil {
		f.Close()
		return nil, fmt.Errorf("runtimex: start CPU profile: %w", err)
	}
	return func() error {
		pprof.StopCPUProfile()
		return f.Close()
	}, nil
}

// WriteHeapProfile writes a heap profile to the given file.
func WriteHeapProfile(filename string) error {
	f, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("runtimex: create heap profile: %w", err)
	}
	defer f.Close()
	if err := pprof.Lookup("heap").WriteTo(f, 0); err != nil {
		return fmt.Errorf("runtimex: write heap profile: %w", err)
	}
	return nil
}

// WriteBlockProfile writes a block profile to the given file.
// Block profiling must have been enabled with SetBlockProfileRate.
func WriteBlockProfile(filename string) error {
	f, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("runtimex: create block profile: %w", err)
	}
	defer f.Close()
	p := pprof.Lookup("block")
	if p == nil {
		return fmt.Errorf("runtimex: block profiling not enabled")
	}
	if err := p.WriteTo(f, 0); err != nil {
		return fmt.Errorf("runtimex: write block profile: %w", err)
	}
	return nil
}

// WriteMutexProfile writes a mutex profile to the given file.
// Mutex profiling must have been enabled with SetMutexProfileFraction.
func WriteMutexProfile(filename string) error {
	f, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("runtimex: create mutex profile: %w", err)
	}
	defer f.Close()
	p := pprof.Lookup("mutex")
	if p == nil {
		return fmt.Errorf("runtimex: mutex profiling not enabled")
	}
	if err := p.WriteTo(f, 0); err != nil {
		return fmt.Errorf("runtimex: write mutex profile: %w", err)
	}
	return nil
}

// SetBlockProfileRate controls the fraction of goroutine blocking events
// that are reported in the blocking profile. Rate of 1 includes every event,
// rate of 0 disables profiling.
func SetBlockProfileRate(rate int) {
	runtime.SetBlockProfileRate(rate)
}

// SetMutexProfileFraction controls the fraction of mutex contention events
// that are reported. Fraction of 1 includes every event, fraction of 0 disables.
func SetMutexProfileFraction(fraction int) {
	runtime.SetMutexProfileFraction(fraction)
}

// --------------------------------------------------------------------
// Build information
// --------------------------------------------------------------------

// BuildInfo returns the module build information embedded in the binary.
// Returns nil if the binary was not built with module support.
func BuildInfo() *debug.BuildInfo {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return nil
	}
	return info
}

// ModuleVersion returns the version of the main module.
// Returns "unknown" if not available.
func ModuleVersion() string {
	info := BuildInfo()
	if info == nil || info.Main.Version == "" {
		return "unknown"
	}
	return info.Main.Version
}

// ModulePath returns the path of the main module.
// Returns "unknown" if not available.
func ModulePath() string {
	info := BuildInfo()
	if info == nil || info.Main.Path == "" {
		return "unknown"
	}
	return info.Main.Path
}

// Dependencies returns a slice of module dependencies.
func Dependencies() []*debug.Module {
	info := BuildInfo()
	if info == nil {
		return nil
	}
	return info.Deps
}

// GoVersion returns the Go version used to build the binary.
func GoVersion() string {
	info := BuildInfo()
	if info == nil {
		return runtime.Version()
	}
	return info.GoVersion
}

// --------------------------------------------------------------------
// Panic handling
// --------------------------------------------------------------------

// Recoverer is a function that can be used in defer to capture panics
// and log them with stack traces. It returns the recovered value if any.
//
// Example:
//
//	defer runtimex.Recoverer(func(p interface{}, stack []byte) {
//		log.Printf("panic: %v\n%s", p, stack)
//	})
func Recoverer(handler func(interface{}, []byte)) interface{} {
	if p := recover(); p != nil {
		buf := make([]byte, 64*1024)
		n := runtime.Stack(buf, false)
		handler(p, buf[:n])
		return p
	}
	return nil
}

// SafeGo launches a goroutine with automatic panic recovery.
// The handler is called if the goroutine panics.
//
// Example:
//
//	runtimex.SafeGo(func() {
//		// risky operation
//	}, func(p interface{}, stack []byte) {
//		log.Printf("goroutine panicked: %v\n%s", p, stack)
//	})
func SafeGo(fn func(), handler func(interface{}, []byte)) {
	go func() {
		defer Recoverer(handler)
		fn()
	}()
}

// --------------------------------------------------------------------
// Misc utilities
// --------------------------------------------------------------------

// CallerName returns the name of the calling function, skipping 'skip' frames.
// Useful for logging and debugging.
func CallerName(skip int) string {
	pc, _, _, ok := runtime.Caller(skip + 1)
	if !ok {
		return "unknown"
	}
	fn := runtime.FuncForPC(pc)
	if fn == nil {
		return "unknown"
	}
	return fn.Name()
}

// CallerFileLine returns the file and line number of the caller.
func CallerFileLine(skip int) (file string, line int) {
	_, file, line, ok := runtime.Caller(skip + 1)
	if !ok {
		return "unknown", 0
	}
	return file, line
}

// PrintStats prints a summary of current runtime statistics to stderr.
func PrintStats() {
	fmt.Fprintf(os.Stderr, "goroutines: %d\n", NumGoroutine())
	fmt.Fprintf(os.Stderr, "memory: %s\n", MemSummary())
	fmt.Fprintf(os.Stderr, "GC cycles: %d\n", NumGC())
	fmt.Fprintf(os.Stderr, "Go version: %s\n", GoVersion())
	fmt.Fprintf(os.Stderr, "module: %s@%s\n", ModulePath(), ModuleVersion())
}
