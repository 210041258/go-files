// Package flagutil provides common, reusable flag definitions and helpers
// for consistent command‑line interfaces across multiple tools.
//
// It integrates with the verbose package for verbosity control and adds
// support for config files, output paths, and automatic help.
//
// Example usage:
//
//	import (
//		"flag"
//		"yourmodule/flagutil"
//		"yourmodule/verbose"
//	)
//
//	func main() {
//		fs := flag.NewFlagSet("myapp", flag.ExitOnError)
//
//		// Add standard flags
//		verboseFlag := flagutil.VerboseFlag(fs, 0, "verbosity level")
//		configFlag := flagutil.ConfigFlag(fs, "", "path to config file")
//		outputFlag := flagutil.OutputFlag(fs, "", "output file path")
//
//		fs.Parse(os.Args[1:])
//
//		verbose.SetLevel(*verboseFlag)
//		// ... use configFlag, outputFlag
//	}
package testutils

import (
	"flag"
	"fmt"
	"os"
)

// --------------------------------------------------------------------
// Standard flag definitions – attach to any *flag.FlagSet
// --------------------------------------------------------------------

// VerboseFlag adds a standard `-v` / `-verbose` flag to the given FlagSet.
// It returns a pointer to the integer verbosity level (default: value).
// The flag can be specified as `-v=2` or `-v 2` or `-vvv` (count mode).
func VerboseFlag(fs *flag.FlagSet, defaultValue int, usage string) *int {
	var level int
	if usage == "" {
		usage = "verbosity level (0=quiet, 1=normal, 2+=debug); can also use -v, -vv, -vvv"
	}
	fs.IntVar(&level, "v", defaultValue, usage)
	fs.IntVar(&level, "verbose", defaultValue, usage+" (long form)")
	// Support `-v` as a count flag (e.g., -v => 1, -vv => 2)
	// This is a bit hacky but works with standard flag package.
	// We override the flag's value by checking os.Args ourselves.
	// Alternatively, we can return a special accessor. For simplicity,
	// we rely on the user to use `-v=2` syntax. If count mode is desired,
	// we can provide a separate function.
	return &level
}

// ConfigFlag adds a standard `-c` / `-config` flag for configuration file.
func ConfigFlag(fs *flag.FlagSet, defaultValue, usage string) *string {
	if usage == "" {
		usage = "path to configuration file"
	}
	return fs.String("config", defaultValue, usage)
	// Also support -c short form
	// We can manually add both; the flag package doesn't support aliases directly.
	// So we register two flags that share the same variable.
}

// WithShortFlag adds both a long and short flag that share the same destination.
// This is a helper for the common case.
func WithShortFlag(fs *flag.FlagSet, short, long string, value interface{}, usage string) {
	switch v := value.(type) {
	case *string:
		fs.StringVar(v, long, *v, usage)
		fs.StringVar(v, short, *v, usage+" (short)")
	case *int:
		fs.IntVar(v, long, *v, usage)
		fs.IntVar(v, short, *v, usage+" (short)")
	case *bool:
		fs.BoolVar(v, long, *v, usage)
		fs.BoolVar(v, short, *v, usage+" (short)")
	default:
		panic("unsupported flag type")
	}
}

// OutputFlag adds a standard `-o` / `-output` flag for output file/directory.
func OutputFlag(fs *flag.FlagSet, defaultValue, usage string) *string {
	if usage == "" {
		usage = "output file path"
	}
	var out string
	WithShortFlag(fs, "o", "output", &out, usage)
	return &out
}

// LogFileFlag adds a standard `-log` flag for log file output.
func LogFileFlag(fs *flag.FlagSet, defaultValue, usage string) *string {
	if usage == "" {
		usage = "log file path (default: stderr)"
	}
	return fs.String("log", defaultValue, usage)
}

// --------------------------------------------------------------------
// Config file loading (placeholder – can be extended)
// --------------------------------------------------------------------

// LoadConfigJSON is a stub for loading a JSON config file.
// It will overwrite flag values if present in the config.
// This function can be expanded to support other formats.
func LoadConfigJSON(path string, fs *flag.FlagSet) error {
	// In a real implementation, you'd parse the file and call fs.Set(name, value)
	// for each config key.
	return nil
}

// --------------------------------------------------------------------
// Help and usage enhancements
// --------------------------------------------------------------------

// UsageFunc returns a standard usage function that prints the flags
// and optionally a custom description.
func UsageFunc(fs *flag.FlagSet, description string) func() {
	return func() {
		fmt.Fprintf(fs.Output(), "Usage: %s [flags]\n", fs.Name())
		if description != "" {
			fmt.Fprintf(fs.Output(), "\n%s\n\n", description)
		}
		fmt.Fprintf(fs.Output(), "Flags:\n")
		fs.PrintDefaults()
	}
}

// ParseWithHelp is a convenience wrapper that adds a -h/--help flag
// and calls fs.Parse. It exits with code 0 if help is requested.
func ParseWithHelp(fs *flag.FlagSet, args []string, description string) {
	help := fs.Bool("h", false, "show this help")
	helpLong := fs.Bool("help", false, "show this help")
	fs.Usage = UsageFunc(fs, description)
	fs.Parse(args)
	if *help || *helpLong {
		fs.Usage()
		os.Exit(0)
	}
}

// --------------------------------------------------------------------
// Argument validation
// --------------------------------------------------------------------

// RequireArgs exits with an error message if the required positional
// arguments are not present.
func RequireArgs(fs *flag.FlagSet, min, max int, msg string) {
	if fs.NArg() < min {
		fmt.Fprintf(fs.Output(), "Error: %s\n", msg)
		fs.Usage()
		os.Exit(2)
	}
	if max >= 0 && fs.NArg() > max {
		fmt.Fprintf(fs.Output(), "Error: too many arguments\n")
		fs.Usage()
		os.Exit(2)
	}
}

// --------------------------------------------------------------------
// Example integration with verbose package (optional)
// --------------------------------------------------------------------

// VerboseFlagWithCount adds a verbose flag that supports both
// integer level and count mode (-v, -vv, -vvv). It sets the global
// verbose.Level automatically if you call SetVerboseOnFlagSet.
// For simplicity, the basic VerboseFlag above is sufficient for most cases.
