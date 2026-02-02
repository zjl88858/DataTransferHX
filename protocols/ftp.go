package protocols

import (
	"fmt"
	"io"
	"path"
	"time"

	"github.com/jlaffaye/ftp"
)

type FTPFileSystem struct {
	Host     string
	Port     int
	User     string
	Password string
	RootPath string
	conn     *ftp.ServerConn
}

func (f *FTPFileSystem) Init() error {
	addr := fmt.Sprintf("%s:%d", f.Host, f.Port)
	c, err := ftp.Dial(addr, ftp.DialWithTimeout(30*time.Second))
	if err != nil {
		return err
	}

	if err := c.Login(f.User, f.Password); err != nil {
		c.Quit()
		return err
	}
	f.conn = c
	return nil
}

func (f *FTPFileSystem) Close() error {
	if f.conn != nil {
		return f.conn.Quit()
	}
	return nil
}

func (f *FTPFileSystem) List(relPath string) ([]FileEntry, error) {
	fullPath := path.Join(f.RootPath, relPath)
	entries, err := f.conn.List(fullPath)
	if err != nil {
		return nil, err
	}

	var files []FileEntry
	for _, entry := range entries {
		if entry.Name == "." || entry.Name == ".." {
			continue
		}
		isDir := entry.Type == ftp.EntryTypeFolder
		files = append(files, FileEntry{
			Name:    entry.Name,
			Size:    int64(entry.Size),
			ModTime: entry.Time,
			IsDir:   isDir,
			Path:    path.Join(relPath, entry.Name),
		})
	}
	return files, nil
}

func (f *FTPFileSystem) Open(relPath string) (io.ReadCloser, error) {
	fullPath := path.Join(f.RootPath, relPath)
	return f.conn.Retr(fullPath)
}

func (f *FTPFileSystem) Create(relPath string) (io.WriteCloser, error) {
	fullPath := path.Join(f.RootPath, relPath)
	// FTP Stor requires a reader, but our interface expects returning a writer.
	// This is a mismatch. The standard io.Copy works with Reader -> Writer.
	// If we return a WriteCloser, we need to pipe it.
	r, w := io.Pipe()
	go func() {
		err := f.conn.Stor(fullPath, r)
		if err != nil {
			r.CloseWithError(err)
		} else {
			r.Close()
		}
	}()
	return w, nil
}

func (f *FTPFileSystem) MkdirAll(relPath string) error {
	fullPath := path.Join(f.RootPath, relPath)
	// FTP doesn't have MkdirAll, need to create recursively manually or try best effort.
	// For simplicity, let's try to create the directory directly.
	// If parent doesn't exist, it might fail.
	// A robust implementation would split path and create one by one.
	
	// Simple recursive implementation
	dirs := []string{}
	curr := fullPath
	for curr != "." && curr != "/" && curr != "" {
		dirs = append(dirs, curr)
		curr = path.Dir(curr)
	}
	
	// Iterate in reverse (root to leaf)
	for i := len(dirs) - 1; i >= 0; i-- {
		f.conn.MakeDir(dirs[i]) // Ignore error as it might already exist
	}
	
	return nil
}

func (f *FTPFileSystem) Stat(relPath string) (*FileEntry, error) {
	fullPath := path.Join(f.RootPath, relPath)
	// FTP LIST is often the only way to get stat
	parent := path.Dir(fullPath)
	name := path.Base(fullPath)
	
	entries, err := f.conn.List(parent)
	if err != nil {
		return nil, err
	}
	
	for _, entry := range entries {
		if entry.Name == name {
			return &FileEntry{
				Name:    entry.Name,
				Size:    int64(entry.Size),
				ModTime: entry.Time,
				IsDir:   entry.Type == ftp.EntryTypeFolder,
				Path:    relPath,
			}, nil
		}
	}
	return nil, fmt.Errorf("file not found: %s", relPath)
}

func (f *FTPFileSystem) Remove(relPath string) error {
	fullPath := path.Join(f.RootPath, relPath)
	return f.conn.Delete(fullPath)
}
