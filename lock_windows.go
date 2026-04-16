//go:build windows

package main

import (
	"fmt"
	"os"
)

type lockFile struct {
	f *os.File
}

// acquireLock on Windows is a best-effort implementation for development.
// For production deployment on Linux, the Unix flock-based version is used.
func acquireLock(path string) (*lockFile, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open lock file: %v", err)
	}

	f.Truncate(0)
	f.Seek(0, 0)
	fmt.Fprintf(f, "%d\n", os.Getpid())
	f.Sync()

	return &lockFile{f: f}, nil
}

func (l *lockFile) Release() {
	if l.f != nil {
		l.f.Close()
	}
}
