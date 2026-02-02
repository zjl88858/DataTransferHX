package protocols

import (
	"io"
	"os"
	"path/filepath"
	"strings"
)

type LocalFileSystem struct {
	RootPath string
}

func (l *LocalFileSystem) Init() error {
	return os.MkdirAll(l.RootPath, 0755)
}

func (l *LocalFileSystem) Close() error {
	return nil
}

func (l *LocalFileSystem) List(path string) ([]FileEntry, error) {
	fullPath := filepath.Join(l.RootPath, path)
	entries, err := os.ReadDir(fullPath)
	if err != nil {
		return nil, err
	}

	var files []FileEntry
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			continue
		}
		files = append(files, FileEntry{
			Name:    entry.Name(),
			Size:    info.Size(),
			ModTime: info.ModTime(),
			IsDir:   entry.IsDir(),
			Path:    filepath.ToSlash(filepath.Join(path, entry.Name())),
		})
	}
	return files, nil
}

func (l *LocalFileSystem) Open(path string) (io.ReadCloser, error) {
	return os.Open(filepath.Join(l.RootPath, path))
}

func (l *LocalFileSystem) Create(path string) (io.WriteCloser, error) {
	fullPath := filepath.Join(l.RootPath, path)
	return os.Create(fullPath)
}

func (l *LocalFileSystem) MkdirAll(path string) error {
	fullPath := filepath.Join(l.RootPath, path)
	return os.MkdirAll(fullPath, 0755)
}

func (l *LocalFileSystem) Stat(path string) (*FileEntry, error) {
	fullPath := filepath.Join(l.RootPath, path)
	info, err := os.Stat(fullPath)
	if err != nil {
		return nil, err
	}
	return &FileEntry{
		Name:    info.Name(),
		Size:    info.Size(),
		ModTime: info.ModTime(),
		IsDir:   info.IsDir(),
		Path:    strings.TrimPrefix(filepath.ToSlash(path), "/"),
	}, nil
}

func (l *LocalFileSystem) Remove(path string) error {
	fullPath := filepath.Join(l.RootPath, path)
	return os.Remove(fullPath)
}
