// Package verbose provides global verbosity control for command‑line tools.
// It allows conditional output based on a numeric verbosity level, with
// simple integration with the standard flag package.
//
// Example usage in a main package:
//
//	import (
//		"flag"
//		"yourmodule/verbose"
//	)
//
//	func main() {
//		level := flag.Int("v", 0, "verbosity level (0=quiet, 1=normal, 2+=debug)")
//		flag.Parse()
//		verbose.SetLevel(*level)
//
//		verbose.Println(1, "Starting process...") // printed if level >= 1
//		verbose.Printf(2, "Debug: %v\n", data)    // printed if level >= 2
//	}
package testutils

import (
	"fmt"
	"io"
	"os"
)

// DefaultOutput is the destination for verbose messages.
// Can be reassigned for testing or GUI applications.
var DefaultOutput io.Writer = os.Stderr

// level holds the current verbosity threshold.
var level int

// SetLevel sets the global verbosity threshold.
// Messages with a level <= this threshold will be printed.
func SetLevel(l int) {
	level = l
}

// Level returns the current verbosity threshold.
func Level() int {
	return level
}

// V returns true if the given verbosity level is active.
// It allows conditional execution of expensive logging operations:
//
//	if verbose.V(2) {
//		verbose.Println(2, expensiveDebugInfo())
//	}
func V(l int) bool {
	return l <= level
}

// Print prints to DefaultOutput if the verbosity level is sufficient.
// Arguments are handled in the manner of fmt.Print.
func Print(l int, a ...interface{}) {
	if V(l) {
		fmt.Fprint(DefaultOutput, a...)
	}
}

// Println prints to DefaultOutput with a newline if the verbosity level is sufficient.
// Arguments are handled in the manner of fmt.Println.
func Println(l int, a ...interface{}) {
	if V(l) {
		fmt.Fprintln(DefaultOutput, a...)
	}
}

// Printf prints to DefaultOutput according to a format if the verbosity level is sufficient.
// Arguments are handled in the manner of fmt.Printf.
func Printf(l int, format string, a ...interface{}) {
	if V(l) {
		fmt.Fprintf(DefaultOutput, format, a...)
	}
}

// Fatal is equivalent to Print(l, a...) followed by os.Exit(1).
// It always prints, regardless of verbosity level.
func Fatal(l int, a ...interface{}) {
	fmt.Fprint(DefaultOutput, a...)
	os.Exit(1)
}

// Fatalln is equivalent to Println(l, a...) followed by os.Exit(1).
// It always prints, regardless of verbosity level.
func Fatalln(l int, a ...interface{}) {
	fmt.Fprintln(DefaultOutput, a...)
	os.Exit(1)
}

// Fatalf is equivalent to Printf(l, format, a...) followed by os.Exit(1).
// It always prints, regardless of verbosity level.
func Fatalf(l int, format string, a ...interface{}) {
	fmt.Fprintf(DefaultOutput, format, a...)
	os.Exit(1)
}

// --------------------------------------------------------------------
// Flag integration helpers (optional)
// --------------------------------------------------------------------

// FlagVar defines a flag with the specified name and usage that sets the
// verbosity level. It is a wrapper around flag.IntVar.
func FlagVar(p *int, name string, value int, usage string) {
	flagIntVar(p, name, value, usage)
}

// Flag defines a flag with the specified name and usage that sets the
// verbosity level, and returns the address of the level variable.
// It is a wrapper around flag.Int.
func Flag(name string, value int, usage string) *int {
	return flagInt(name, value, usage)
}

// To avoid requiring callers to import "flag" unless they use these helpers,
// we use the standard flag package internally. If your tool already imports
// flag, you can use flag.Int directly; these helpers are for convenience.
import "flag"

func flagIntVar(p *int, name string, value int, usage string) {
	flag.IntVar(p, name, value, usage)
}

func flagInt(name string, value int, usage string) *int {
	return flag.Int(name, value, usage)
}