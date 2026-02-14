//go:build darwin
// +build darwin

package testutils

import (
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"
)

func init() {
	getPhysicalCores = darwinPhysicalCores
	getInfo = darwinInfo
	getUsage = darwinUsage
}

func darwinPhysicalCores() int {
	// sysctl hw.physicalcpu
	return int(sysctlInt64("hw.physicalcpu"))
}

func darwinInfo() (*Info, error) {
	info := &Info{
		LogicalCores: runtime.NumCPU(),
		Cores:        darwinPhysicalCores(),
		Sockets:      int(sysctlInt64("hw.packages")),
		ModelName:    sysctlString("machdep.cpu.brand_string"),
		Frequency:    float64(sysctlInt64("hw.cpufrequency")) / 1e6, // Hz -> MHz
		MaxFreq:      float64(sysctlInt64("hw.cpufrequency_max")) / 1e6,
		CacheSize:    int(sysctlInt64("hw.l3cachesize") / 1024), // bytes -> KB
	}
	// Features
	features := sysctlString("machdep.cpu.features")
	if features != "" {
		info.Flags = strings.Fields(strings.ToLower(features))
	}
	return info, nil
}

func darwinUsage() (*Usage, error) {
	// Getrusage for current process
	var ru syscall.Rusage
	err := syscall.Getrusage(syscall.RUSAGE_SELF, &ru)
	if err != nil {
		return nil, err
	}
	user := time.Duration(ru.Utime.Sec)*time.Second + time.Duration(ru.Utime.Usec)*time.Microsecond
	system := time.Duration(ru.Stime.Sec)*time.Second + time.Duration(ru.Stime.Usec)*time.Microsecond
	return &Usage{
		User:   user,
		System: system,
		Total:  user + system,
	}, nil
}

// sysctl helpers
func sysctlInt64(name string) int64 {
	v, err := syscall.Sysctl(name)
	if err != nil {
		return 0
	}
	if i, err := strconv.ParseInt(v, 10, 64); err == nil {
		return i
	}
	return 0
}

func sysctlString(name string) string {
	v, err := syscall.Sysctl(name)
	if err != nil {
		return ""
	}
	return v
}
