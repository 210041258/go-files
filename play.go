//go:build ignore
// +build ignore

// play.go – advanced local development playground.
// Includes profiling, benchmarks, stress tests, memory tracking,
// GC tuning, fuzzing, flamegraphs, and more.
//
// Usage:
//   go run play.go                              # basic examples
//   go run play.go -mode=memory                # memory allocation tracking
//   go run play.go -mode=heapdump              # write heap profile
//   go run play.go -mode=gctune                # GC tuning experiments
//   go run play.go -mode=compare               # performance comparison runner
//   go run play.go -mode=fuzz                  # fuzz test entry point
//   go run play.go -mode=flamegraph            # CPU profile + SVG flamegraph
//   go run play.go -mode=bench                 # micro‑benchmarks
//   go run play.go -mode=stress                # concurrency stress test
//   go run play.go -mode=pprof                 # pprof server
//   go run play.go -mode=context               # context cancellation demo

package testutils

import (
	"context"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/exec"
	"runtime"
	"runtime/debug"
	"runtime/pprof"
	"sync"
	"testing/quick"
	"time"
	// --------------------------------------------------------------------
	// Import ALL your utility packages.
	// Blank imports keep them available for `go mod tidy`.
	// Replace "yourmodule" with your actual module path.
	// --------------------------------------------------------------------
	/*_ "yourmodule/api"
	_ "yourmodule/call"
	_ "yourmodule/cpu"
	_ "yourmodule/flagutil"
	_ "yourmodule/hashutil"
	_ "yourmodule/hashutils"
	_ "yourmodule/hostname"
	_ "yourmodule/lock"
	_ "yourmodule/node"
	_ "yourmodule/osutil"
	_ "yourmodule/packet"
	_ "yourmodule/pathname"
	_ "yourmodule/platform"
	_ "yourmodule/pointer"
	_ "yourmodule/runtimex"
	_ "yourmodule/safe"
	_ "yourmodule/segment"
	_ "yourmodule/signalutil"
	_ "yourmodule/slice"
	_ "yourmodule/testutils"
	_ "yourmodule/unique"
	_ "yourmodule/value"
	_ "yourmodule/verbose"
	_ "yourmodule/zero"*/)

var mode = flag.String("mode", "basic", "basic | memory | heapdump | gctune | compare | fuzz | flamegraph | bench | stress | pprof | context")

func main() {
	defer recoverPanic()
	flag.Parse()

	fmt.Println("=== 🧪 Dev Playground ===")
	fmt.Println("Mode:", *mode)

	switch *mode {
	case "memory":
		runMemoryTracking()
	case "heapdump":
		runHeapDump()
	case "gctune":
		runGCTuning()
	case "compare":
		runPerformanceComparison()
	case "fuzz":
		runFuzzEntryPoint()
	case "flamegraph":
		runFlamegraph()
	case "bench":
		runBenchmarks()
	case "stress":
		runStressTest()
	case "pprof":
		runPprofServer()
	case "context":
		runContextDemo()
	default:
		runBasic()
	}
}

// recoverPanic prints stack traces for any panics.
func recoverPanic() {
	if r := recover(); r != nil {
		log.Printf("🔥 PANIC recovered: %v\n", r)
		debug.PrintStack()
	}
}

// --------------------------------------------------------------------
// 1. Memory Allocation Tracking
// --------------------------------------------------------------------

func runMemoryTracking() {
	fmt.Println("\n-- Memory Allocation Tracking --")

	// Enable allocation profiling (1 sample per 512KB)
	runtime.MemProfileRate = 512 * 1024

	// Force GC to start clean
	runtime.GC()
	debug.FreeOSMemory()

	var memStart, memEnd runtime.MemStats
	runtime.ReadMemStats(&memStart)

	// --- ALLOCATE ---
	const N = 5_000_000
	data := make([][]byte, 0, N)
	for i := 0; i < N; i++ {
		// Allocate random sized slice
		size := rand.Intn(100) + 1
		data = append(data, make([]byte, size))
	}
	// --- END ALLOCATE ---

	runtime.ReadMemStats(&memEnd)

	fmt.Printf("Allocated objects: %d\n", N)
	fmt.Printf("HeapAlloc before: %d KiB\n", memStart.HeapAlloc/1024)
	fmt.Printf("HeapAlloc after : %d KiB\n", memEnd.HeapAlloc/1024)
	fmt.Printf("HeapObjects before: %d\n", memStart.HeapObjects)
	fmt.Printf("HeapObjects after : %d\n", memEnd.HeapObjects)
	fmt.Printf("TotalAlloc (cumulative): %d KiB\n", (memEnd.TotalAlloc-memStart.TotalAlloc)/1024)

	// Write memory profile
	f, err := os.Create("mem.pprof")
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
	if err := pprof.WriteHeapProfile(f); err != nil {
		log.Fatal(err)
	}
	fmt.Println("Memory profile written to mem.pprof")
}

// --------------------------------------------------------------------
// 2. Heap Dump Export (Go 1.17+)
// --------------------------------------------------------------------

func runHeapDump() {
	fmt.Println("\n-- Heap Dump Export --")

	// Force GC and write heap dump
	runtime.GC()
	debug.FreeOSMemory()

	filename := fmt.Sprintf("heap-%d.dump", time.Now().Unix())
	f, err := os.Create(filename)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	// runtime.WriteHeapDump is available from Go 1.17
	if err := runtime.WriteHeapDump(f.Fd()); err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Heap dump written to %s\n", filename)
	fmt.Println("View with: go tool pprof -http=:8080", filename)
}

// --------------------------------------------------------------------
// 3. GC Tuning Experiments
// --------------------------------------------------------------------

func runGCTuning() {
	fmt.Println("\n-- GC Tuning Experiments --")

	// Baseline: default GC percent (100)
	defaultGC := debug.SetGCPercent(-1)
	debug.SetGCPercent(defaultGC) // restore

	// Test different GC percent values
	targets := []int{50, 100, 200, 400, -1} // -1 = off

	for _, p := range targets {
		prev := debug.SetGCPercent(p)
		fmt.Printf("\n--- GCPercent = %d ---\n", p)

		// Allocate a bunch of memory
		start := time.Now()
		var mem runtime.MemStats
		runtime.ReadMemStats(&mem)
		allocStart := mem.HeapAlloc

		// Allocate 100MB in chunks
		var sink [][]byte
		for i := 0; i < 100; i++ {
			sink = append(sink, make([]byte, 1024*1024)) // 1MB
			time.Sleep(10 * time.Millisecond)
		}

		elapsed := time.Since(start)
		runtime.ReadMemStats(&mem)
		allocEnd := mem.HeapAlloc

		fmt.Printf("Allocation time : %v\n", elapsed)
		fmt.Printf("HeapAlloc delta: %d KiB\n", (allocEnd-allocStart)/1024)
		fmt.Printf("Number of GCs   : %d\n", mem.NumGC)

		// Prevent sink from being optimised away
		runtime.KeepAlive(sink)

		debug.SetGCPercent(prev) // restore
	}
}

// --------------------------------------------------------------------
// 4. Automatic Performance Comparison Runner
// --------------------------------------------------------------------

func runPerformanceComparison() {
	fmt.Println("\n-- Performance Comparison Runner --")

	// Define two implementations to compare
	impl1 := func(data []int) int {
		sum := 0
		for _, v := range data {
			sum += v
		}
		return sum
	}

	impl2 := func(data []int) int {
		sum := 0
		for i := 0; i < len(data); i++ {
			sum += data[i]
		}
		return sum
	}

	// Prepare data
	data := make([]int, 10_000_000)
	for i := range data {
		data[i] = i
	}

	// Benchmark function
	bench := func(fn func([]int) int) (time.Duration, int64) {
		start := time.Now()
		result := fn(data)
		elapsed := time.Since(start)
		runtime.GC()
		return elapsed, int64(result)
	}

	fmt.Println("Comparing range vs index loops:")
	t1, r1 := bench(impl1)
	t2, r2 := bench(impl2)

	fmt.Printf("  range loop : %v (result=%d)\n", t1, r1)
	fmt.Printf("  index loop : %v (result=%d)\n", t2, r2)
	fmt.Printf("  difference : %.2f%%\n", float64(t2-t1)/float64(t1)*100)
}

// --------------------------------------------------------------------
// 5. Fuzz Test Entry Point (quick, random testing)
// --------------------------------------------------------------------

func runFuzzEntryPoint() {
	fmt.Println("\n-- Fuzz Test Entry Point --")

	// Property: reversing a string twice yields the original
	prop := func(s string) bool {
		// Simple reverse
		rev := func(s string) string {
			r := []rune(s)
			for i, j := 0, len(r)-1; i < j; i, j = i+1, j-1 {
				r[i], r[j] = r[j], r[i]
			}
			return string(r)
		}
		return rev(rev(s)) == s
	}

	if err := quick.Check(prop, nil); err != nil {
		fmt.Println("❌ Property failed:", err)
	} else {
		fmt.Println("✅ Property holds for random inputs")
	}

	// Example: add a fuzz target that runs forever (press Ctrl+C to stop)
	fmt.Println("\nFuzzing (Ctrl+C to stop)...")
	fz := func(in []byte) int {
		if len(in) > 0 && in[0] == 'a' && in[1] == 'b' && in[2] == 'c' {
			panic("found abc")
		}
		return 0
	}

	// Simple fuzz loop
	for i := 0; i < 10_000; i++ {
		input := make([]byte, rand.Intn(20))
		rand.Read(input)
		func() {
			defer func() {
				if r := recover(); r != nil {
					fmt.Printf("🔥 Fuzz found crash on input %q: %v\n", input, r)
				}
			}()
			fz(input)
		}()
	}
	fmt.Println("Fuzzing complete (10k iterations).")
}

// --------------------------------------------------------------------
// 6. Flamegraph Automation (CPU profile + SVG)
// --------------------------------------------------------------------

func runFlamegraph() {
	fmt.Println("\n-- Flamegraph Automation --")

	// Create CPU profile file
	cpuFile := "cpu.pprof"
	f, err := os.Create(cpuFile)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	// Start CPU profiling
	if err := pprof.StartCPUProfile(f); err != nil {
		log.Fatal(err)
	}
	defer pprof.StopCPUProfile()

	// Burn CPU for a few seconds
	fmt.Println("Profiling CPU for 5 seconds...")
	end := time.Now().Add(5 * time.Second)
	for time.Now().Before(end) {
		_ = 42 * 42
	}

	fmt.Println("CPU profile written to", cpuFile)

	// Generate flamegraph SVG if 'go' tool and flamegraph.pl are available
	svgFile := "flamegraph.svg"
	if _, err := exec.LookPath("go"); err == nil {
		// Use `go tool pprof` to generate raw stacks, then pipe to flamegraph.pl
		// For simplicity, we'll just print instructions.
		fmt.Println("\nTo generate flamegraph:")
		fmt.Printf("  go tool pprof -raw %s > cpu.folded\n", cpuFile)
		fmt.Println("  git clone https://github.com/brendangregg/FlameGraph")
		fmt.Println("  cd FlameGraph")
		fmt.Println("  ./stackcollapse-go.pl ../cpu.folded | ./flamegraph.pl > ../flamegraph.svg")
		fmt.Println("\nOr use pprof's built-in web interface:")
		fmt.Printf("  go tool pprof -http=:8081 %s\n", cpuFile)
	} else {
		fmt.Println("go tool not found; cannot automate flamegraph generation.")
	}
}

// --------------------------------------------------------------------
// Existing modes (kept as before, with minor enhancements)
// --------------------------------------------------------------------

func runBasic() {
	fmt.Println("\n-- Basic Examples --")
	fmt.Println("Playground ready. Edit play.go and enable the -mode flag.")
}

func runContextDemo() {
	fmt.Println("\n-- Context Timeout Demo --")
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	go func() {
		time.Sleep(2 * time.Second)
		fmt.Println("Work completed (should not print)")
	}()
	select {
	case <-ctx.Done():
		fmt.Println("Context expired:", ctx.Err())
	}
}

func runStressTest() {
	fmt.Println("\n-- Concurrency Stress Test --")
	const workers = 8
	const iterations = 1_000_000
	var wg sync.WaitGroup
	wg.Add(workers)
	start := time.Now()
	for w := 0; w < workers; w++ {
		go func(id int) {
			defer wg.Done()
			sum := 0
			for i := 0; i < iterations; i++ {
				sum += i
			}
			if id == 0 {
				fmt.Printf("Worker %d sum: %d\n", id, sum)
			}
		}(w)
	}
	wg.Wait()
	fmt.Printf("Completed in: %v\n", time.Since(start))
	fmt.Printf("Goroutines: %d\n", runtime.NumGoroutine())
}

func runBenchmarks() {
	fmt.Println("\n-- Micro Benchmark Runner --")
	const N = 20_000_000
	start := time.Now()
	sum := 0
	for i := 0; i < N; i++ {
		sum += i
	}
	elapsed := time.Since(start)
	fmt.Printf("Iterations: %d\n", N)
	fmt.Printf("Elapsed:    %v\n", elapsed)
	fmt.Printf("Ops/sec:    %.0f\n", float64(N)/elapsed.Seconds())
	fmt.Printf("Sum:        %d\n", sum)
}

func runPprofServer() {
	fmt.Println("\n-- pprof Profiling Server --")
	fmt.Println("   Listening on http://localhost:6060/debug/pprof/")
	fmt.Println()
	go func() {
		log.Println(http.ListenAndServe("localhost:6060", nil))
	}()
	select {}
}
