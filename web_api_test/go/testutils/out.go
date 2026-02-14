// Package out provides simple terminal output utilities,
// including colored text and basic wrappers around fmt.
package testutils

import (
	"fmt"
	"io"
	"os"
)

// colors defines ANSI escape codes for common colors.
var colors = map[string]string{
	"reset": "\033[0m",
	"red":   "\033[31m",
	"green": "\033[32m",
	"yellow": "\033[33m",
	"blue":   "\033[34m",
	"magenta": "\033[35m",
	"cyan":    "\033[36m",
	"white":   "\033[37m",
	"bold":    "\033[1m",
}

// EnableColors can be set to false to disable colored output (e.g., for non‑TTY).
var EnableColors = true

// output is the default writer; can be changed for testing.
var output io.Writer = os.Stdout

// ----------------------------------------------------------------------
// Basic output
// ----------------------------------------------------------------------

// Print prints to the output with no newline.
func Print(a ...interface{}) {
	fmt.Fprint(output, a...)
}

// Printf prints a formatted string to the output.
func Printf(format string, a ...interface{}) {
	fmt.Fprintf(output, format, a...)
}

// Println prints to the output with a newline.
func Println(a ...interface{}) {
	fmt.Fprintln(output, a...)
}

// ----------------------------------------------------------------------
// Colored output
// ----------------------------------------------------------------------

// colorize wraps the text with the specified color if colors are enabled.
func colorize(text string, color string) string {
	if !EnableColors {
		return text
	}
	return colors[color] + text + colors["reset"]
}

// Red prints text in red.
func Red(a ...interface{}) {
	Print(colorize(fmt.Sprint(a...), "red"))
}

// Redf prints a formatted string in red.
func Redf(format string, a ...interface{}) {
	Print(colorize(fmt.Sprintf(format, a...), "red"))
}

// Redln prints a line in red.
func Redln(a ...interface{}) {
	Println(colorize(fmt.Sprint(a...), "red"))
}

// Green prints text in green.
func Green(a ...interface{}) {
	Print(colorize(fmt.Sprint(a...), "green"))
}

// Greenf prints a formatted string in green.
func Greenf(format string, a ...interface{}) {
	Print(colorize(fmt.Sprintf(format, a...), "green"))
}

// Greenln prints a line in green.
func Greenln(a ...interface{}) {
	Println(colorize(fmt.Sprint(a...), "green"))
}

// Yellow prints text in yellow.
func Yellow(a ...interface{}) {
	Print(colorize(fmt.Sprint(a...), "yellow"))
}

// Yellowf prints a formatted string in yellow.
func Yellowf(format string, a ...interface{}) {
	Print(colorize(fmt.Sprintf(format, a...), "yellow"))
}

// Yellowln prints a line in yellow.
func Yellowln(a ...interface{}) {
	Println(colorize(fmt.Sprint(a...), "yellow"))
}

// Blue prints text in blue.
func Blue(a ...interface{}) {
	Print(colorize(fmt.Sprint(a...), "blue"))
}

// Bluef prints a formatted string in blue.
func Bluef(format string, a ...interface{}) {
	Print(colorize(fmt.Sprintf(format, a...), "blue"))
}

// Blueln prints a line in blue.
func Blueln(a ...interface{}) {
	Println(colorize(fmt.Sprint(a...), "blue"))
}

// ----------------------------------------------------------------------
// Convenience helpers
// ----------------------------------------------------------------------

// Success prints a green checkmark and the message.
func Success(format string, a ...interface{}) {
	Green("✓ ")
	Printf(format+"\n", a...)
}

// Error prints a red cross and the message.
func Error(format string, a ...interface{}) {
	Red("✗ ")
	Printf(format+"\n", a...)
}

// Warn prints a yellow warning symbol and the message.
func Warn(format string, a ...interface{}) {
	Yellow("⚠ ")
	Printf(format+"\n", a...)
}

// Info prints a blue info symbol and the message.
func Info(format string, a ...interface{}) {
	Blue("ℹ ")
	Printf(format+"\n", a...)
}

// ----------------------------------------------------------------------
// Example usage (commented)
// ----------------------------------------------------------------------
// func main() {
//     out.Println("Hello, world!")
//     out.Redln("This is an error message")
//     out.Success("Operation completed successfully")
//     out.Warn("This is a warning")
// }