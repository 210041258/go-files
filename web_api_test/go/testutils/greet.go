// Package greet provides simple greeting functions.
// It includes basic hello messages and formatted greetings.
package greet

import (
	"fmt"
	"strings"
)

// Hello returns a simple "Hello, World!" greeting.
func Hello() string {
	return "Hello, World!"
}

// Greet returns a greeting to the specified name.
// Example: Greet("Alice") -> "Hello, Alice!"
func Greet(name string) string {
	return fmt.Sprintf("Hello, %s!", name)
}

// Greetf returns a formatted greeting using a custom format string.
// The format string should contain exactly one %s placeholder for the name.
// If the format does not contain a placeholder, it is used as is and the name is appended.
func Greetf(format, name string) string {
	if strings.Contains(format, "%s") {
		return fmt.Sprintf(format, name)
	}
	return format + " " + name
}

// GreetMany returns a greeting to multiple names, separated by commas.
// Example: GreetMany("Alice", "Bob") -> "Hello, Alice, Bob!"
func GreetMany(names ...string) string {
	if len(names) == 0 {
		return Hello()
	}
	return fmt.Sprintf("Hello, %s!", strings.Join(names, ", "))
}

// ----------------------------------------------------------------------
// Example usage (commented)
// ----------------------------------------------------------------------
// func main() {
//     fmt.Println(greet.Hello())               // "Hello, World!"
//     fmt.Println(greet.Greet("Alice"))        // "Hello, Alice!"
//     fmt.Println(greet.Greetf("Hi %s", "Bob")) // "Hi Bob"
//     fmt.Println(greet.GreetMany("Alice", "Bob", "Charlie")) // "Hello, Alice, Bob, Charlie!"
// }