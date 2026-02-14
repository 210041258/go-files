//go:build unix || linux || darwin || freebsd || netbsd || openbsd
// +build unix linux darwin freebsd netbsd openbsd

package testutils

import (
	"syscall"
	"time"
)

func (fl *FileLock) lock(blocking bool, timeout time.Duration) error {
	// flock implementation
	how := syscall.LOCK_EX
	if !blocking {
		how |= syscall.LOCK_NB
	}
	return syscall.Flock(int(fl.file.Fd()), how)
}

func (fl *FileLock) unlock() error {
	return syscall.Flock(int(fl.file.Fd()), syscall.LOCK_UN)
}
