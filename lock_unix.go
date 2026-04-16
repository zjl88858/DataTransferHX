//go:build !windows

package main

import (
	"fmt"
	"os"
	"syscall"
)

type lockFile struct {
	f *os.File
}

// acquireLock tries to acquire an exclusive file lock.
// On Unix, flock is automatically released when the process exits (even on crash).
func acquireLock(path string) (*lockFile, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open lock file: %v", err)
	}

	err = syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
	if err != nil {
		f.Close()
		return nil, fmt.Errorf("another instance is already running (lock file: %s)", path)
	}

	// Write PID for informational purposes
	f.Truncate(0)
	f.Seek(0, 0)
	fmt.Fprintf(f, "%d\n", os.Getpid())
	f.Sync()

	return &lockFile{f: f}, nil
}

func (l *lockFile) Release() {
	if l.f != nil {
		syscall.Flock(int(l.f.Fd()), syscall.LOCK_UN)
		l.f.Close()
	}
}
