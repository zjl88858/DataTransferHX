package protocols

import (
	"fmt"
	"io"
	"net"
	"path"
	"time"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

type SFTPFileSystem struct {
	Host     string
	Port     int
	User     string
	Password string
	RootPath string
	client   *sftp.Client
	sshConn  *ssh.Client
	tcpConn  net.Conn
}

func (s *SFTPFileSystem) Init() error {
	sshConfig := &ssh.ClientConfig{
		User: s.User,
		Auth: []ssh.AuthMethod{
			ssh.Password(s.Password),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         30 * time.Second,
	}

	addr := fmt.Sprintf("%s:%d", s.Host, s.Port)

	// Use net.DialTimeout to retain the raw TCP connection, so we can
	// force-close it later to prevent goroutine leaks.
	tcpConn, err := net.DialTimeout("tcp", addr, sshConfig.Timeout)
	if err != nil {
		return err
	}

	sshConn, chans, reqs, err := ssh.NewClientConn(tcpConn, addr, sshConfig)
	if err != nil {
		tcpConn.Close()
		return err
	}
	s.tcpConn = tcpConn
	s.sshConn = ssh.NewClient(sshConn, chans, reqs)

	client, err := sftp.NewClient(s.sshConn)
	if err != nil {
		s.sshConn.Close()
		return err
	}
	s.client = client
	return nil
}

func (s *SFTPFileSystem) Close() error {
	// Set a short deadline on the TCP connection so that if sftp/ssh Close()
	// tries to send/receive on a dead connection, it will timeout rather
	// than block forever (which causes goroutine leaks).
	if s.tcpConn != nil {
		s.tcpConn.SetDeadline(time.Now().Add(10 * time.Second))
	}
	if s.client != nil {
		s.client.Close()
	}
	if s.sshConn != nil {
		s.sshConn.Close()
	}
	// Force-close the raw TCP connection to guarantee all SSH goroutines
	// (handshake, mux, channel handler, session wait, sftp pipe) exit.
	if s.tcpConn != nil {
		s.tcpConn.Close()
	}
	return nil
}

func (s *SFTPFileSystem) List(relPath string) ([]FileEntry, error) {
	fullPath := path.Join(s.RootPath, relPath)
	entries, err := s.client.ReadDir(fullPath)
	if err != nil {
		return nil, err
	}

	var files []FileEntry
	for _, entry := range entries {
		files = append(files, FileEntry{
			Name:    entry.Name(),
			Size:    entry.Size(),
			ModTime: entry.ModTime(),
			IsDir:   entry.IsDir(),
			Path:    path.Join(relPath, entry.Name()),
		})
	}
	return files, nil
}

func (s *SFTPFileSystem) Open(relPath string) (io.ReadCloser, error) {
	fullPath := path.Join(s.RootPath, relPath)
	return s.client.Open(fullPath)
}

func (s *SFTPFileSystem) Create(relPath string) (io.WriteCloser, error) {
	fullPath := path.Join(s.RootPath, relPath)
	return s.client.Create(fullPath)
}

func (s *SFTPFileSystem) MkdirAll(relPath string) error {
	fullPath := path.Join(s.RootPath, relPath)
	return s.client.MkdirAll(fullPath)
}

func (s *SFTPFileSystem) Stat(relPath string) (*FileEntry, error) {
	fullPath := path.Join(s.RootPath, relPath)
	info, err := s.client.Stat(fullPath)
	if err != nil {
		return nil, err
	}
	return &FileEntry{
		Name:    info.Name(),
		Size:    info.Size(),
		ModTime: info.ModTime(),
		IsDir:   info.IsDir(),
		Path:    relPath,
	}, nil
}

func (s *SFTPFileSystem) Remove(relPath string) error {
	fullPath := path.Join(s.RootPath, relPath)
	return s.client.Remove(fullPath)
}
