// Package cpu provides cross‑platform CPU information and control.
// It supports Linux, Windows, macOS, and FreeBSD with sensible fallbacks.
package testutils

import (
	"fmt"
	"runtime"
	"strings"
	"time"
)

// --------------------------------------------------------------------
// Common types
// --------------------------------------------------------------------

// Info contains detailed information about the CPU.
type Info struct {
	ModelName    string   // e.g., "Intel(R) Core(TM) i7-8700K CPU @ 3.70GHz"
	Cores        int      // physical cores per socket
	LogicalCores int      // threads / logical processors
	Sockets      int      // number of physical CPU packages
	Frequency    float64  // current frequency in MHz (0 if unknown)
	MaxFreq      float64  // maximum frequency in MHz
	CacheSize    int      // L3 cache size in KB (0 if unknown)
	Flags        []string // CPU feature flags (SSE, AVX, etc.)
}

// Usage contains CPU time and percentage for the current process.
type Usage struct {
	User    time.Duration // time spent in user mode
	System  time.Duration // time spent in kernel mode
	Total   time.Duration // total CPU time (user + system)
	Percent float64       // CPU usage percentage (0‑100) since last call
	PerCore []float64     // per‑core usage percentages (if available)
}

// --------------------------------------------------------------------
// High‑level API
// --------------------------------------------------------------------

// Count returns the number of logical CPUs usable by the current process.
// Equivalent to runtime.NumCPU().
func Count() int {
	return runtime.NumCPU()
}

// PhysicalCores returns an estimate of physical core count.
// May be less accurate on some platforms; falls back to logical count.
func PhysicalCores() int {
	return getPhysicalCores()
}

// Info returns detailed CPU information.
// Fields that cannot be determined are left zero/empty.
func Info() (*Info, error) {
	return getInfo()
}

// Usage returns CPU usage statistics for the current process.
// Percent is calculated since the previous call to Usage().
// To get absolute CPU time from process start, use Usage() without percent.
func Usage() (*Usage, error) {
	return getUsage()
}

// SetMaxProcs sets the maximum number of CPUs that can be executing simultaneously.
// Equivalent to runtime.GOMAXPROCS(n). Returns the previous setting.
func SetMaxProcs(n int) int {
	return runtime.GOMAXPROCS(n)
}

// --------------------------------------------------------------------
// Platform‑specific implementations (see *_platform.go)
// --------------------------------------------------------------------

var (
	getPhysicalCores func() int
	getInfo          func() (*Info, error)
	getUsage         func() (*Usage, error)
)

func init() {
	// Set platform defaults (overridden by build tags)
	getPhysicalCores = fallbackPhysicalCores
	getInfo = fallbackInfo
	getUsage = fallbackUsage
}

// Fallback implementations
func fallbackPhysicalCores() int {
	// Assume 2 threads per core on hyper‑threaded systems.
	// This is a rough guess; platform‑specific implementations are better.
	cpus := runtime.NumCPU()
	if cpus > 1 {
		return cpus / 2
	}
	return 1
}

func fallbackInfo() (*Info, error) {
	return &Info{
		LogicalCores: runtime.NumCPU(),
		Cores:        fallbackPhysicalCores(),
		Sockets:      1,
	}, nil
}

func fallbackUsage() (*Usage, error) {
	// Minimal implementation using runtime package.
	var u Usage
	u.Percent = 0.0 // cannot compute without previous sample
	return &u, nil
}

// --------------------------------------------------------------------
// Utility functions
// --------------------------------------------------------------------

// Summary returns a human‑readable one‑line summary of the CPU.
func Summary() string {
	info, err := Info()
	if err != nil {
		return fmt.Sprintf("%d logical cores", runtime.NumCPU())
	}
	var b strings.Builder
	b.WriteString(info.ModelName)
	if info.ModelName == "" {
		b.WriteString("CPU")
	}
	b.WriteString(fmt.Sprintf(" (%d physical, %d logical)",
		info.Cores, info.LogicalCores))
	if info.Frequency > 0 {
		b.WriteString(fmt.Sprintf(" @ %.0fMHz", info.Frequency))
	}
	return b.String()
}
