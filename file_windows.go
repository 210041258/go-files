//go:build windows
// +build windows

package testutils

import (
	"syscall"
	"time"
)

var (
	modkernel32      = syscall.NewLazyDLL("kernel32.dll")
	procLockFileEx   = modkernel32.NewProc("LockFileEx")
	procUnlockFileEx = modkernel32.NewProc("UnlockFileEx")
)

const (
	lockfileExclusiveLock   = 2
	lockfileFailImmediately = 1
)

func (fl *FileLock) lock(blocking bool, timeout time.Duration) error {
	flags := lockfileExclusiveLock
	if !blocking {
		flags |= lockfileFailImmediately
	}

	var overlapped syscall.Overlapped
	var millis uint32
	if timeout > 0 {
		millis = uint32(timeout / time.Millisecond)
	}

	r1, _, err := procLockFileEx.Call(
		fl.file.Fd(),
		uintptr(flags),
		0, // reserved
		1, // low part of lock length
		0, // high part of lock length
		uintptr(&overlapped),
		uintptr(millis),
	)
	if r1 == 0 {
		if err == syscall.ERROR_LOCK_VIOLATION || err == syscall.ERROR_IO_PENDING {
			return errWouldBlock
		}
		return err
	}
	return nil
}

func (fl *FileLock) unlock() error {
	var overlapped syscall.Overlapped
	r1, _, err := procUnlockFileEx.Call(
		fl.file.Fd(),
		0, // reserved
		1, // low part of lock length
		0, // high part of lock length
		uintptr(&overlapped),
	)
	if r1 == 0 {
		return err
	}
	return nil
}
