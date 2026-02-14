package testutils

import (
	"flag"
	"fmt"
	"time"
)

func main() {
	// Define command-line flags
	bootMsg := flag.String("message", "System booting...", "boot message to display")
	delay := flag.Duration("delay", 2*time.Second, "delay before boot completes")
	flag.Parse()

	// Simulate boot sequence
	fmt.Println(*bootMsg)
	time.Sleep(*delay)
	fmt.Println("Boot completed successfully.")
}