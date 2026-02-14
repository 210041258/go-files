//go:build linux
// +build linux

package testutils

import (
	"bufio"
	"errors"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"
)

func init() {
	getPhysicalCores = linuxPhysicalCores
	getInfo = linuxInfo
	getUsage = linuxUsage
}

func linuxPhysicalCores() int {
	// Read /proc/cpuinfo, count "core id" unique values
	f, err := os.Open("/proc/cpuinfo")
	if err != nil {
		return fallbackPhysicalCores()
	}
	defer f.Close()

	coreIDs := make(map[string]struct{})
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "core id") {
			parts := strings.Split(line, ":")
			if len(parts) == 2 {
				id := strings.TrimSpace(parts[1])
				coreIDs[id] = struct{}{}
			}
		}
	}
	if len(coreIDs) > 0 {
		return len(coreIDs)
	}
	return fallbackPhysicalCores()
}

func linuxInfo() (*Info, error) {
	f, err := os.Open("/proc/cpuinfo")
	if err != nil {
		return nil, err
	}
	defer f.Close()

	info := &Info{
		LogicalCores: runtime.NumCPU(),
		Cores:        linuxPhysicalCores(),
		Sockets:      1,
		Flags:        []string{},
	}

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, ":") {
			parts := strings.SplitN(line, ":", 2)
			key := strings.TrimSpace(parts[0])
			val := strings.TrimSpace(parts[1])

			switch key {
			case "model name":
				info.ModelName = val
			case "cpu MHz":
				if f, err := strconv.ParseFloat(val, 64); err == nil {
					info.Frequency = f
				}
			case "cpu cores":
				if c, err := strconv.Atoi(val); err == nil {
					info.Cores = c // override per‑socket core count
				}
			case "siblings":
				if s, err := strconv.Atoi(val); err == nil {
					info.LogicalCores = s // threads per socket
				}
			case "physical id":
				if id, err := strconv.Atoi(val); err == nil && id > info.Sockets-1 {
					info.Sockets = id + 1
				}
			case "cache size":
				// Format: "6144 KB"
				if strings.HasSuffix(val, " KB") {
					trim := strings.TrimSuffix(val, " KB")
					if kb, err := strconv.Atoi(trim); err == nil {
						info.CacheSize = kb
					}
				}
			case "flags":
				info.Flags = strings.Fields(val)
			}
		}
	}
	return info, scanner.Err()
}

func linuxUsage() (*Usage, error) {
	// Read process CPU times from /proc/self/stat
	data, err := os.ReadFile("/proc/self/stat")
	if err != nil {
		return nil, err
	}
	fields := strings.Fields(string(data))
	if len(fields) < 15 {
		return nil, errors.New("invalid /proc/self/stat format")
	}
	// utime: user time in clock ticks (14), stime: kernel time (15)
	utime, _ := strconv.ParseInt(fields[13], 10, 64)
	stime, _ := strconv.ParseInt(fields[14], 10, 64)
	// cutime, cstime (children) are fields 16,17 – ignore for now

	// Clock ticks per second
	tick := int64(100) // Linux: sysconf(_SC_CLK_TCK) is usually 100
	user := time.Duration(utime) * time.Second / time.Duration(tick)
	system := time.Duration(stime) * time.Second / time.Duration(tick)

	return &Usage{
		User:    user,
		System:  system,
		Total:   user + system,
		Percent: 0.0, // Caller must compute delta
	}, nil
}
