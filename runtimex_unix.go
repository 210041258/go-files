//go:build unix || linux || darwin || freebsd || netbsd || openbsd || solaris
// +build unix linux darwin freebsd netbsd openbsd solaris

package testutils

import (
	"os"
	"os/signal"
	"syscall"
)

// InstallGoroutineDumpSignal installs a signal handler that prints a
// full goroutine stack dump to stderr when SIGUSR1 is received.
// This is a common debugging pattern on Unix systems.
//
// Example:
//
//	runtimex.InstallGoroutineDumpSignal()
//	// Now `kill -SIGUSR1 <pid>` prints goroutine stacks.
func InstallGoroutineDumpSignal() {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGUSR1)
	go func() {
		for range ch {
			PrintGoroutineDump()
		}
	}()
}

// InstallQuitSignalDump installs a signal handler that prints a
// goroutine dump when SIGQUIT is received (Ctrl+\). On most Unix systems,
// SIGQUIT already causes a dump, but this can be used to customise output.
func InstallQuitSignalDump() {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGQUIT)
	go func() {
		for range ch {
			PrintGoroutineDump()
			os.Exit(1) // match default behaviour
		}
	}()
}
