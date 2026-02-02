package protocols

import (
	"io"
	"time"
)

type FileEntry struct {
	Name    string
	Size    int64
	ModTime time.Time
	IsDir   bool
	Path    string // 相对路径
}

type FileSystem interface {
	Init() error
	Close() error
	// List returns a list of files in the specified directory (non-recursive).
	List(path string) ([]FileEntry, error)
	Open(path string) (io.ReadCloser, error)
	Create(path string) (io.WriteCloser, error)
	MkdirAll(path string) error
	Stat(path string) (*FileEntry, error)
	Remove(path string) error
}
