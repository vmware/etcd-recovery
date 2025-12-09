// Copyright (c) 2025 Broadcom. All Rights Reserved.
// Broadcom Confidential. The term "Broadcom" refers to Broadcom Inc.
// and/or its subsidiaries.

package ssh

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/pkg/sftp"

	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
)

var (
	serverPrivateBytes = []byte(`
-----BEGIN OPENSSH PRIVATE KEY-----
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
-----END OPENSSH PRIVATE KEY-----`)

	serverPublicKeyBytes = []byte(`ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAACAQDDORO9B0GszJh4Ebq+UvsqddIfssLuWtsD5gX4CXj8YQQpFPD3MxzKqdmbAPpICr8VFou1eabO2InbnfJQp0+iu2J8MZ845zvhsjYCDQcBE7pDXw74dan3wvpcCN1ydbQizzw8leidrNM3RoxT3ZPNefBf7VgJgf1pGz9UzuuwRdp9KxxbaMpyqZ3E9b/jWfW/ApQ9/T7lE8lap0ft6wt3q6TsjCvOZeS9wY9imVU8GVVkFRakBgY2MlZ7UcvLU/Hytklxf7asxKDt8SRzdYkymh/qP/d+fGsrr9iu6RAOFM6WdGz2zd40YDz+n9nRXudFgT14le0ldIQN/GrOZbJxwF2jiHkGCJZKgJ/8G1OfFEXjhNRIR1rvAnwRJfgFwq4reLg56yjgk5H6mMuLSnKNG/XpRYE24nzoj8mUN+wE60/o9wLmb2raHgac6xyISXQEtZmdEZPnkP2BZrEE9myrUoU4xpQ4ds8evivSrjCk9VMHB4wPDd8cOIjgIFg7QDQ2ywXHuKH4zejflDWthyYGvwpnyabT8uSuq4OVroQLLOrGhCRQwWfTWCxFr2hlBpsl/+x4exNo+YfXndS134Fz1LVM7XqJcRx+6/30nXxQeg87Wz29rzVUja5ZDKD+TKahKZBqxZhg7THLcfAl1IkOPlnWxqZiyIM5vZfAE/oHcw== test@broadcom.com`)
)

func TestSSHConnectionWithWrongPassword(t *testing.T) {
	hostConfig := &Config{
		User:     "testuser",
		Host:     "127.0.0.1",
		Port:     2020,
		Timeout:  30 * time.Second,
		Password: "123456", // user config with wrong password
	}

	// prepare server config
	hostPubKey, _, _, _, err := ssh.ParseAuthorizedKey(serverPublicKeyBytes)
	require.NoError(t, err)
	hostConfig.SetHostKeyCallback(ssh.FixedHostKey(hostPubKey))

	// Setup mock server
	server, err := NewServerLocal(hostConfig.User, "testpass", hostConfig.Port, "./testdata")
	require.NoError(t, err)

	// Start the server
	err = server.Start()
	require.NoError(t, err)

	defer server.Stop()

	// Give the server a moment to start
	time.Sleep(100 * time.Millisecond)

	_, err = NewClient(hostConfig)
	require.Error(t, err)
}

func TestSSHConnectionWithCorrectPassword(t *testing.T) {
	hostConfig := &Config{
		User:     "testuser",
		Host:     "127.0.0.1",
		Port:     2020,
		Timeout:  30 * time.Second,
		Password: "testpass",
	}

	// prepare server config
	hostPubKey, _, _, _, err := ssh.ParseAuthorizedKey(serverPublicKeyBytes)
	require.NoError(t, err)

	hostConfig.SetHostKeyCallback(ssh.FixedHostKey(hostPubKey))

	// Setup mock server
	server, err := NewServerLocal(hostConfig.User, hostConfig.Password, hostConfig.Port, "./testdata")
	require.NoError(t, err)

	// Start the server
	err = server.Start()
	require.NoError(t, err)

	defer server.Stop()

	// Give the server a moment to start
	time.Sleep(100 * time.Millisecond)

	client, err := NewClient(hostConfig)
	require.NoError(t, err)
	defer client.Close()
}

func TestSSHConnectionWithPrivateKey(t *testing.T) {
	hostConfig := &Config{
		User:           "testuser",
		Host:           "127.0.0.1",
		Port:           2020,
		Timeout:        30 * time.Second,
		PrivateKeyPath: "testdata/id_test",
	}

	// prepare server config
	hostPubKey, _, _, _, err := ssh.ParseAuthorizedKey(serverPublicKeyBytes)
	require.NoError(t, err)

	hostConfig.SetHostKeyCallback(ssh.FixedHostKey(hostPubKey))

	// Setup mock server
	server, err := NewServerLocal(hostConfig.User, hostConfig.Password, hostConfig.Port, "./testdata")
	require.NoError(t, err)

	// Start the server
	err = server.Start()
	require.NoError(t, err)

	defer server.Stop()

	// Give the server a moment to start
	time.Sleep(100 * time.Millisecond)

	client, err := NewClient(hostConfig)
	require.NoError(t, err)
	defer client.Close()
}

func TestRunCommandOnLocalServer(t *testing.T) {
	hostConfig := &Config{
		User:           "testuser",
		Host:           "127.0.0.1",
		Port:           2020,
		Timeout:        30 * time.Second,
		PrivateKeyPath: "testdata/id_test",
	}

	// prepare server config
	hostPubKey, _, _, _, err := ssh.ParseAuthorizedKey(serverPublicKeyBytes)
	require.NoError(t, err)

	hostConfig.SetHostKeyCallback(ssh.FixedHostKey(hostPubKey))

	// Setup mock server
	server, err := NewServerLocal(hostConfig.User, hostConfig.Password, hostConfig.Port, "./testdata")
	require.NoError(t, err)

	// Start the server
	err = server.Start()
	require.NoError(t, err)

	defer server.Stop()

	// Give the server a moment to start
	time.Sleep(100 * time.Millisecond)

	client, err := NewClient(hostConfig)
	require.NoError(t, err)
	defer client.Close()

	out, err := client.Run("hey!!")
	require.NoError(t, err)
	t.Logf("output: %v", string(out))
	// verify output
	if string(out) != "HI, i am handled\n" {
		require.Fail(t, "RunCommand returned unexpected output")
	}
}

func TestUploadFileToLocalServer(t *testing.T) {
	hostConfig := &Config{
		User:           "testuser",
		Host:           "127.0.0.1",
		Port:           2020,
		Timeout:        30 * time.Second,
		PrivateKeyPath: "testdata/id_test",
	}

	// prepare server config
	hostPubKey, _, _, _, err := ssh.ParseAuthorizedKey(serverPublicKeyBytes)
	require.NoError(t, err)

	hostConfig.SetHostKeyCallback(ssh.FixedHostKey(hostPubKey))

	// Setup mock server
	server, err := NewServerLocal(hostConfig.User, hostConfig.Password, hostConfig.Port, "./testdata")
	require.NoError(t, err)

	// Start the server
	err = server.Start()
	require.NoError(t, err)

	defer server.Stop()

	// Give the server a moment to start
	time.Sleep(100 * time.Millisecond)

	client, err := NewClient(hostConfig)
	require.NoError(t, err)
	defer client.Close()

	testData := []byte("Hello, SFTP Test!")
	localfile := t.TempDir() + "/test.txt"
	remotePath := "test_upload.txt"

	err = os.WriteFile(localfile, testData, 0o600)
	require.NoError(t, err)

	err = client.Upload(localfile, remotePath)
	require.NoError(t, err)

	// Verify the file exists on the server
	localPath := filepath.Join(server.GetRootDir(), remotePath)
	content, err := os.ReadFile(localPath)
	require.NoError(t, err)
	if !bytes.Equal(content, testData) {
		require.Fail(t, fmt.Sprintf("File content mismatch. Expected: %s, Got: %s", testData, content))
	}
}

func TestDownloadFileFromLocalServer(t *testing.T) {
	hostConfig := &Config{
		User:           "testuser",
		Host:           "127.0.0.1",
		Port:           2020,
		Timeout:        30 * time.Second,
		PrivateKeyPath: "testdata/id_test",
	}

	// prepare server config
	hostPubKey, _, _, _, err := ssh.ParseAuthorizedKey(serverPublicKeyBytes)
	require.NoError(t, err)

	hostConfig.SetHostKeyCallback(ssh.FixedHostKey(hostPubKey))

	// Setup mock server
	server, err := NewServerLocal(hostConfig.User, hostConfig.Password, hostConfig.Port, "./testdata")
	require.NoError(t, err)

	// Start the server
	err = server.Start()
	require.NoError(t, err)

	defer server.Stop()

	// Give the server a moment to start
	time.Sleep(100 * time.Millisecond)

	client, err := NewClient(hostConfig)
	require.NoError(t, err)
	defer client.Close()

	// First, create a file on the server
	testData := []byte("Download test content")
	remotePath := filepath.Join(server.GetRootDir(), "test_download.txt")

	err = os.WriteFile(remotePath, testData, 0o600)
	require.NoError(t, err)

	localPath := t.TempDir() + "/test_download.txt"

	// Download the file via SFTP
	err = client.Download("test_download.txt", localPath)
	require.NoError(t, err)

	// Read the content
	n, err := os.ReadFile(localPath)
	require.NoError(t, err)
	if !bytes.Equal(n, testData) {
		require.Fail(t, fmt.Sprintf("File content mismatch. Expected: %s, Got: %s", testData, n))
	}
}

func TestUploadFileWithSudoFallback(t *testing.T) {
	hostConfig := &Config{
		User:           "testuser",
		Host:           "127.0.0.1",
		Port:           2020,
		Timeout:        30 * time.Second,
		PrivateKeyPath: "testdata/id_test",
	}

	hostPubKey, _, _, _, err := ssh.ParseAuthorizedKey(serverPublicKeyBytes)
	require.NoError(t, err)
	hostConfig.SetHostKeyCallback(ssh.FixedHostKey(hostPubKey))

	server, err := NewServerLocal(hostConfig.User, hostConfig.Password, hostConfig.Port, "./testdata")
	require.NoError(t, err)

	// Set restricted path
	restrictedPath := "restricted_upload.txt"
	server.SetRestrictedPath(restrictedPath)

	err = server.Start()
	require.NoError(t, err)
	defer server.Stop()
	time.Sleep(100 * time.Millisecond)

	client, err := NewClient(hostConfig)
	require.NoError(t, err)
	defer client.Close()

	testData := []byte("Hello, Sudo Upload!")
	localfile := t.TempDir() + "/test.txt"
	err = os.WriteFile(localfile, testData, 0o600)
	require.NoError(t, err)

	// Upload should succeed via fallback
	err = client.Upload(localfile, restrictedPath)
	require.NoError(t, err)

	// Verify file exists on server (because mock exec moved it)
	serverPath := filepath.Join(server.GetRootDir(), restrictedPath)
	content, err := os.ReadFile(serverPath)
	require.NoError(t, err)
	if !bytes.Equal(content, testData) {
		require.Fail(t, fmt.Sprintf("File content mismatch. Expected: %s, Got: %s", testData, content))
	}

	// Verify commands
	cmds := server.GetExecutedCommands()
	foundMv := false
	for _, cmd := range cmds {
		if strings.Contains(cmd, "mv") && strings.Contains(cmd, restrictedPath) {
			foundMv = true
			break
		}
	}
	if !foundMv {
		t.Errorf("Expected 'mv' command to be executed, got: %v", cmds)
	}
}

func TestDownloadFileWithSudoFallback(t *testing.T) {
	hostConfig := &Config{
		User:           "testuser",
		Host:           "127.0.0.1",
		Port:           2020,
		Timeout:        30 * time.Second,
		PrivateKeyPath: "testdata/id_test",
	}

	hostPubKey, _, _, _, err := ssh.ParseAuthorizedKey(serverPublicKeyBytes)
	require.NoError(t, err)
	hostConfig.SetHostKeyCallback(ssh.FixedHostKey(hostPubKey))

	server, err := NewServerLocal(hostConfig.User, hostConfig.Password, hostConfig.Port, "./testdata")
	require.NoError(t, err)

	restrictedPath := "restricted_download.txt"
	server.SetRestrictedPath(restrictedPath)

	err = server.Start()
	require.NoError(t, err)
	defer server.Stop()
	time.Sleep(100 * time.Millisecond)

	client, err := NewClient(hostConfig)
	require.NoError(t, err)
	defer client.Close()

	// Create file on server
	testData := []byte("Sudo Download Content")
	serverPath := filepath.Join(server.GetRootDir(), restrictedPath)
	err = os.WriteFile(serverPath, testData, 0o600)
	require.NoError(t, err)

	localPath := t.TempDir() + "/downloaded.txt"

	// Download should succeed via fallback
	err = client.Download(restrictedPath, localPath)
	require.NoError(t, err)

	content, err := os.ReadFile(localPath)
	require.NoError(t, err)
	if !bytes.Equal(content, testData) {
		require.Fail(t, fmt.Sprintf("File content mismatch. Expected: %s, Got: %s", testData, content))
	}

	// Verify commands
	cmds := server.GetExecutedCommands()
	foundCp := false
	for _, cmd := range cmds {
		if strings.Contains(cmd, "cp") && strings.Contains(cmd, restrictedPath) {
			foundCp = true
			break
		}
	}
	if !foundCp {
		t.Errorf("Expected 'cp' command to be executed, got: %v", cmds)
	}
}

// Server represents a local server instance
type Server struct {
	user             string
	password         string
	port             int
	rootDir          string
	listener         net.Listener
	config           *ssh.ServerConfig
	running          bool
	mu               sync.Mutex
	stopChan         chan struct{}
	executedCommands []string
	restrictedPaths  map[string]bool
}

// NewServerLocal creates a new local server instance
func NewServerLocal(user, password string, port int, rootDir string) (*Server, error) {
	// Create root directory if it doesn't exist
	if err := os.MkdirAll(rootDir, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create root directory: %w", err)
	}

	server := &Server{
		user:            user,
		password:        password,
		port:            port,
		rootDir:         rootDir,
		stopChan:        make(chan struct{}),
		restrictedPaths: make(map[string]bool),
	}

	private, err := ssh.ParsePrivateKey(serverPrivateBytes)
	if err != nil {
		log.Printf("Failed to parse private key: %v", err)
	}

	// Configure SSH server
	server.config = &ssh.ServerConfig{
		PasswordCallback: func(c ssh.ConnMetadata, pass []byte) (*ssh.Permissions, error) {
			if c.User() == server.user && string(pass) == server.password {
				return nil, nil
			}
			return nil, fmt.Errorf("authentication failed")
		},
		PublicKeyCallback: func(c ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
			if c.User() == server.user {
				return nil, nil
			}
			return nil, fmt.Errorf("public key rejected for %q", c.User())
		},
	}
	server.config.AddHostKey(private)

	return server, nil
}

func (s *Server) SetRestrictedPath(path string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.restrictedPaths[path] = true
}

func (s *Server) GetExecutedCommands() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.executedCommands...)
}

// Start starts the SFTP server
func (s *Server) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		return fmt.Errorf("server is already running")
	}

	// Start listening on the specified port
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", s.port))
	if err != nil {
		return fmt.Errorf("failed to listen on port %d: %w", s.port, err)
	}
	s.listener = listener
	s.running = true

	// Handle connections in a goroutine
	go s.acceptConnections()

	log.Printf("SFTP server started on port %d", s.port)
	return nil
}

// Stop stops the SFTP server
func (s *Server) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return fmt.Errorf("server is not running")
	}

	close(s.stopChan)
	s.running = false

	if s.listener != nil {
		return s.listener.Close()
	}

	log.Println("SFTP server stopped")
	return nil
}

// GetRootDir returns the root directory path
func (s *Server) GetRootDir() string {
	return s.rootDir
}

// GetPort returns the server port
func (s *Server) GetPort() int {
	return s.port
}

// IsRunning returns whether the server is running
func (s *Server) IsRunning() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.running
}

// acceptConnections handles incoming connections
func (s *Server) acceptConnections() {
	for {
		select {
		case <-s.stopChan:
			return
		default:
			conn, err := s.listener.Accept()
			if err != nil {
				if !s.IsRunning() {
					return
				}
				log.Printf("Failed to accept connection: %v", err)
				continue
			}

			go s.handleConnection(conn)
		}
	}
}

// handleConnection handles a single SSH/SFTP connection
func (s *Server) handleConnection(conn net.Conn) {
	defer conn.Close()

	// Perform SSH handshake
	sshConn, chans, reqs, err := ssh.NewServerConn(conn, s.config)
	if err != nil {
		log.Printf("Failed to establish SSH connection: %v", err)
		return
	}
	defer sshConn.Close()

	log.Printf("SSH connection established with %s", sshConn.RemoteAddr())

	// Discard all global requests
	go ssh.DiscardRequests(reqs)

	// Handle channels
	for newChannel := range chans {
		if newChannel.ChannelType() != "session" {
			newChannel.Reject(ssh.UnknownChannelType, "unknown channel type")
			continue
		}

		channel, requests, err := newChannel.Accept()
		if err != nil {
			log.Printf("Failed to accept channel: %v", err)
			continue
		}

		go s.handleChannel(channel, requests)
	}
}

// handleChannel handles an SSH channel for SFTP or Exec
func (s *Server) handleChannel(channel ssh.Channel, requests <-chan *ssh.Request) {
	defer channel.Close()

	for req := range requests {
		switch req.Type {
		case "exec":
			command := string(req.Payload[4:])
			log.Printf("Executing command: %v", command)

			// Consume stdin to avoid blocking/errors if client writes to it
			go io.Copy(io.Discard, channel)

			s.mu.Lock()
			s.executedCommands = append(s.executedCommands, command)
			s.mu.Unlock()

			// Simple simulation for cp/mv/rm
			parts := strings.Fields(command)
			if len(parts) > 0 && parts[0] == "sudo" {
				parts = parts[1:]
				// Consume common sudo flags used in RunSudo
				for len(parts) > 0 && strings.HasPrefix(parts[0], "-") {
					if parts[0] == "-p" && len(parts) > 1 {
						parts = parts[2:] // consume -p and its arg (e.g. '')
					} else if parts[0] == "-S" || parts[0] == "-n" {
						parts = parts[1:]
					} else {
						break // unknown flag, stop
					}
				}
			}

			if len(parts) >= 3 && parts[0] == "mv" {
				src := filepath.Join(s.rootDir, parts[1])
				dst := filepath.Join(s.rootDir, parts[2])
				_ = os.MkdirAll(filepath.Dir(dst), 0o755)
				_ = os.Rename(src, dst)
			} else if len(parts) >= 2 && parts[0] == "cp" {
				// Handle cp with optional -p flag
				srcIdx, dstIdx := 1, 2
				if len(parts) > 2 && parts[1] == "-p" {
					srcIdx, dstIdx = 2, 3
				}
				if len(parts) > dstIdx {
					src := filepath.Join(s.rootDir, parts[srcIdx])
					dst := filepath.Join(s.rootDir, parts[dstIdx])
					_ = os.MkdirAll(filepath.Dir(dst), 0o755)
					data, err := os.ReadFile(src)
					if err == nil {
						// Get source file permissions
						srcInfo, _ := os.Stat(src)
						perm := os.FileMode(0o644)
						if srcInfo != nil {
							perm = srcInfo.Mode()
						}
						_ = os.WriteFile(dst, data, perm)
					}
				}
			} else if len(parts) >= 2 && parts[0] == "rm" {
				target := filepath.Join(s.rootDir, parts[1])
				_ = os.Remove(target)
			}

			_, _ = channel.Write([]byte("HI, i am handled\n"))
			req.Reply(true, nil)
			// just return error 0.
			channel.SendRequest("exit-status", false, []byte{0, 0, 0, 0})
			return // close session after execution

		case "subsystem":
			if string(req.Payload[4:]) == "sftp" {
				req.Reply(true, nil)

				// Create SFTP server handler with custom root
				s.mu.Lock()
				restricted := s.restrictedPaths
				s.mu.Unlock()

				handlers := &customHandlers{rootDir: s.rootDir, restrictedPaths: restricted}
				server := sftp.NewRequestServer(channel, sftp.Handlers{
					FileGet:  handlers,
					FilePut:  handlers,
					FileList: handlers,
					FileCmd:  handlers,
				})

				if err := server.Serve(); err != nil && err != io.EOF { //nolint:errorlint
					log.Printf("SFTP server error: %v", err)
				}
				return
			}
			req.Reply(false, nil)
		default:
			req.Reply(false, nil)
		}
	}
}

// customHandlers implements sftp.Handlers with a custom root directory
type customHandlers struct {
	rootDir         string
	restrictedPaths map[string]bool
}

func (h *customHandlers) Fileread(r *sftp.Request) (io.ReaderAt, error) {
	// check if path is restricted
	// r.Filepath is the path requested by client.
	// If client requests "restricted_download.txt", r.Filepath is that.
	// We verify against restrictedPaths.
	if h.restrictedPaths[r.Filepath] || h.restrictedPaths[strings.TrimPrefix(r.Filepath, "/")] {
		return nil, &sftp.StatusError{Code: uint32(sftp.ErrSshFxPermissionDenied)}
	}

	path := filepath.Join(h.rootDir, r.Filepath)
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	return file, nil
}

func (h *customHandlers) Filewrite(r *sftp.Request) (io.WriterAt, error) {
	if h.restrictedPaths[r.Filepath] || h.restrictedPaths[strings.TrimPrefix(r.Filepath, "/")] {
		return nil, &sftp.StatusError{Code: uint32(sftp.ErrSshFxPermissionDenied)}
	}

	path := filepath.Join(h.rootDir, r.Filepath)

	// Create directory if it doesn't exist
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}

	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return nil, err
	}
	return file, nil
}

func (h *customHandlers) Filelist(r *sftp.Request) (sftp.ListerAt, error) {
	path := filepath.Join(h.rootDir, r.Filepath)

	switch r.Method {
	case "List":
		entries, err := os.ReadDir(path)
		if err != nil {
			return nil, err
		}

		var fileInfos []os.FileInfo
		for _, entry := range entries {
			info, err := entry.Info()
			if err != nil {
				continue
			}
			fileInfos = append(fileInfos, info)
		}
		return listerat(fileInfos), nil

	case "Stat":
		info, err := os.Stat(path)
		if err != nil {
			return nil, err
		}
		return listerat([]os.FileInfo{info}), nil

	default:
		return nil, fmt.Errorf("unsupported list command: %s", r.Method)
	}
}

// listerat implements sftp.ListerAt for a slice of os.FileInfo
type listerat []os.FileInfo

func (l listerat) ListAt(ls []os.FileInfo, offset int64) (int, error) {
	if offset >= int64(len(l)) {
		return 0, io.EOF
	}

	n := copy(ls, l[offset:])
	if n < len(ls) {
		return n, io.EOF
	}
	return n, nil
}

func (h *customHandlers) Filecmd(r *sftp.Request) error {
	path := filepath.Join(h.rootDir, r.Filepath)

	switch r.Method {
	case "Remove":
		return os.Remove(path)
	case "Rename":
		newPath := filepath.Join(h.rootDir, r.Target)
		return os.Rename(path, newPath)
	case "Mkdir":
		return os.Mkdir(path, 0o755)
	case "Rmdir":
		return os.Remove(path)
	case "Setstat":
		return nil // ignore: handling stats
	default:
		return fmt.Errorf("unsupported file command: %s", r.Method)
	}
}
