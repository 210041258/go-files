// Package in provides simple input utilities for reading from stdin.
// It includes functions for reading lines, integers, floats, and prompting.
package testutils

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

var (
	// Reader is the input source. It defaults to os.Stdin.
	// Change this for testing.
	Reader = os.Stdin

	scanner = bufio.NewScanner(Reader)
)

// init ensures the scanner is ready.
func init() {
	scanner.Split(bufio.ScanLines)
}

// Line reads a single line from stdin, trimming the trailing newline.
// It returns an empty string if an error occurs (e.g., EOF).
func Line() string {
	if scanner.Scan() {
		return strings.TrimRight(scanner.Text(), "\r\n")
	}
	return ""
}

// MustLine reads a line and panics if no input is available.
func MustLine() string {
	if scanner.Scan() {
		return strings.TrimRight(scanner.Text(), "\r\n")
	}
	panic("in: unexpected EOF")
}

// Int reads an integer from stdin. It returns an error if the input
// is not a valid integer.
func Int() (int, error) {
	line := Line()
	if line == "" {
		return 0, fmt.Errorf("empty input")
	}
	return strconv.Atoi(strings.TrimSpace(line))
}

// MustInt reads an integer and panics on error.
func MustInt() int {
	v, err := Int()
	if err != nil {
		panic("in: " + err.Error())
	}
	return v
}

// Float reads a float64 from stdin.
func Float() (float64, error) {
	line := Line()
	if line == "" {
		return 0, fmt.Errorf("empty input")
	}
	return strconv.ParseFloat(strings.TrimSpace(line), 64)
}

// MustFloat reads a float and panics on error.
func MustFloat() float64 {
	v, err := Float()
	if err != nil {
		panic("in: " + err.Error())
	}
	return v
}

// Prompt prints a prompt and reads a line.
func Prompt(prompt string) string {
	fmt.Print(prompt)
	return Line()
}

// Promptf prints a formatted prompt and reads a line.
func Promptf(format string, a ...interface{}) string {
	fmt.Printf(format, a...)
	return Line()
}

// Confirm asks a yes/no question and returns true for yes.
// It accepts y/Y/yes/YES and n/N/no/NO. The default value is false.
// If the user enters empty, it returns false.
func Confirm(prompt string) bool {
	fmt.Print(prompt + " [y/N]: ")
	line := strings.ToLower(strings.TrimSpace(Line()))
	return line == "y" || line == "yes"
}

// ConfirmDefault allows specifying the default answer.
// If defaultYes is true, the default is yes (displayed as [Y/n]).
func ConfirmDefault(prompt string, defaultYes bool) bool {
	suffix := "[y/N]"
	if defaultYes {
		suffix = "[Y/n]"
	}
	fmt.Print(prompt + " " + suffix + ": ")
	line := strings.ToLower(strings.TrimSpace(Line()))
	if line == "" {
		return defaultYes
	}
	return line == "y" || line == "yes"
}

// ----------------------------------------------------------------------
// Example usage (commented)
// ----------------------------------------------------------------------
// func main() {
//     name := in.Prompt("Enter your name: ")
//     fmt.Println("Hello,", name)
//
//     age := in.MustInt()
//     fmt.Println("Next year you will be", age+1)
//
//     if in.Confirm("Continue?") {
//         fmt.Println("Continuing...")
//     }
// }