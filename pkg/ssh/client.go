// Copyright (c) 2025 Broadcom. All Rights Reserved.
// Broadcom Confidential. The term "Broadcom" refers to Broadcom Inc.
// and/or its subsidiaries.

package ssh

import (
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

// default constants
const (
	DefaultTimeout = 20 * time.Second
	DefaultPort    = 22
)

// Client represents ssh client.
type Client struct {
	*ssh.Client
}

type Config struct {
	User                 string
	Host                 string
	Port                 int
	Timeout              time.Duration
	Password             string
	PrivateKeyPath       string
	PrivateKeyPassphrase string
	hostKeyCallBack      ssh.HostKeyCallback
}

func (c *Config) SetHostKeyCallback(hostKeyCallBack ssh.HostKeyCallback) {
	c.hostKeyCallBack = hostKeyCallBack
}

// NewClient returns new ssh client and error if any.
func NewClient(config *Config) (*Client, error) {
	c := &Client{}
	var auth Auth
	var hostKeyCallback ssh.HostKeyCallback
	var err error

	// configure Auth as per users config
	auth, err = configureAuth(config.Password, config.PrivateKeyPath, config.PrivateKeyPassphrase)
	if err != nil {
		return nil, errors.New("failed to configure auth: " + err.Error())
	}

	// configure hostKeyCallback as per users config
	hostKeyCallback, err = configureHostKeyCallback(config.hostKeyCallBack)
	if err != nil {
		return nil, errors.New("failed to configure hostKeyCallBack: " + err.Error())
	}

	// configure default timeout
	if config.Timeout == 0 {
		config.Timeout = DefaultTimeout
	}

	// configure default port
	if config.Port == 0 {
		config.Port = DefaultPort
	}

	c.Client, err = ssh.Dial("tcp", net.JoinHostPort(config.Host, fmt.Sprint(config.Port)), &ssh.ClientConfig{
		User:            config.User,
		Auth:            auth,
		HostKeyCallback: hostKeyCallback,
		Timeout:         config.Timeout,
	})
	if err != nil {
		return nil, err
	}
	return c, nil
}

// Run starts a new SSH session and runs the cmd, it returns CombinedOutput and err if any.
func (c Client) Run(cmd string) ([]byte, error) {
	var (
		err  error
		sess *ssh.Session
	)
	if sess, err = c.NewSession(); err != nil {
		return nil, err
	}
	defer sess.Close()

	return sess.CombinedOutput(cmd)
}

// newSftp returns new sftp client and error if any.
func (c Client) newSftp(opts ...sftp.ClientOption) (*sftp.Client, error) {
	return sftp.NewClient(c.Client, opts...)
}

// Close client net connection.
func (c Client) Close() error {
	return c.Client.Close()
}

// makeTempPath generates temporary file location
func makeTempPath(basePath string) string {
	return filepath.Join("/tmp", fmt.Sprintf("etcd-recovery_%d_%s", time.Now().UnixNano(), filepath.Base(basePath)))
}

// Upload a local file to remote server!
func (c Client) Upload(localPath string, remotePath string) (err error) {
	local, err := os.Open(localPath)
	if err != nil {
		return err
	}
	defer local.Close()

	// Stat to retrieve local file permissions
	localFileInfo, err := local.Stat()
	if err != nil {
		return err
	}

	if err := c.sftpUpload(local, remotePath, localFileInfo.Mode()); err != nil {
		if isPermissionDenied(err) {
			return c.sudoUpload(localPath, remotePath, localFileInfo)
		}
		return err
	}

	return nil
}

func (c Client) sftpUpload(local *os.File, remotePath string, mode os.FileMode) error {
	// Reset file pointer
	if _, err := local.Seek(0, 0); err != nil {
		return err
	}

	ftp, err := c.newSftp()
	if err != nil {
		return err
	}
	defer ftp.Close()

	remote, err := ftp.Create(remotePath)
	if err != nil {
		return err
	}
	defer remote.Close()
	_, err = io.Copy(remote, local)
	if err != nil {
		return err
	}

	// Set remote file mode to match local file permissions
	err = remote.Chmod(mode)
	if err != nil {
		return err
	}

	return nil
}

func (c Client) sudoUpload(localPath string, remotePath string, info os.FileInfo) error {
	// To handle permission denied errors, we first upload the file to a temporary location
	// on the remote server, and then use sudo to move it to the final destination and set permissions.
	tempPath := makeTempPath(localPath)

	local, err := os.Open(localPath)
	if err != nil {
		return err
	}
	defer local.Close()

	if err := c.sftpUpload(local, tempPath, info.Mode()); err != nil {
		return fmt.Errorf("failed to upload to temp path %s: %w", tempPath, err)
	}
	// ensure temporary file is cleaned up
	defer c.Run(fmt.Sprintf("sudo rm -f %s", tempPath))

	// Move to destination with sudo
	if _, err := c.Run(fmt.Sprintf("sudo mv %s %s", tempPath, remotePath)); err != nil {
		return fmt.Errorf("failed to sudo mv from %s to %s: %w", tempPath, remotePath, err)
	}

	// Chmod
	if _, err := c.Run(fmt.Sprintf("sudo chmod %o %s", info.Mode().Perm(), remotePath)); err != nil {
		return fmt.Errorf("failed to sudo chmod on %s: %w", remotePath, err)
	}

	return nil
}

// Download file from remote server!
func (c Client) Download(remotePath string, localPath string) (err error) {
	if err := c.sftpDownload(remotePath, localPath); err != nil {
		if isPermissionDenied(err) {
			return c.sudoDownload(remotePath, localPath)
		}
		return err
	}
	return nil
}

func (c Client) sftpDownload(remotePath string, localPath string) error {
	local, err := os.Create(localPath)
	if err != nil {
		return err
	}
	defer local.Close()

	ftp, err := c.newSftp()
	if err != nil {
		return err
	}
	defer ftp.Close()

	remote, err := ftp.Open(remotePath)
	if err != nil {
		return err
	}
	defer remote.Close()

	// Stat to retrieve remote file permissions
	remoteFileInfo, err := remote.Stat()
	if err != nil {
		return err
	}

	if _, err = io.Copy(local, remote); err != nil {
		return err
	}

	// set local file permissions to match remote file
	err = local.Chmod(remoteFileInfo.Mode())
	if err != nil {
		return err
	}

	return local.Sync()
}

func (c Client) sudoDownload(remotePath string, localPath string) error {
	// To handle permission denied errors, we first copy the file to a temporary location
	// on the remote server using sudo, change its ownership to the current user,
	// then download it, and finally clean up the temporary file.
	tempPath := makeTempPath(remotePath)

	// Copy to temp path with sudo, preserving permissions
	if _, err := c.Run(fmt.Sprintf("sudo cp -p %s %s", remotePath, tempPath)); err != nil {
		return fmt.Errorf("failed to sudo cp to %s: %w", tempPath, err)
	}
	defer c.Run(fmt.Sprintf("sudo rm -f %s", tempPath))

	// Change ownership to the current user so we can download it
	if _, err := c.Run(fmt.Sprintf("sudo chown %s %s", c.Client.User(), tempPath)); err != nil {
		return fmt.Errorf("failed to sudo chown on %s: %w", tempPath, err)
	}

	// Download from temp path (sftpDownload will preserve permissions from temp file)
	return c.sftpDownload(tempPath, localPath)
}

func isPermissionDenied(err error) bool {
	if errors.Is(err, os.ErrPermission) {
		return true
	}
	var statusErr *sftp.StatusError
	if errors.As(err, &statusErr) {
		if statusErr.Code == uint32(sftp.ErrSshFxPermissionDenied) {
			return true
		}
	}
	errMsg := strings.ToLower(err.Error())
	return strings.Contains(errMsg, "permission denied") || strings.Contains(errMsg, "ssh_fx_permission_denied")
}
