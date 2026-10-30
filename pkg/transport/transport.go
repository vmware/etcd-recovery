// Copyright (c) 2026 Broadcom. All Rights Reserved.
// Broadcom Confidential. The term "Broadcom" refers to Broadcom Inc.
// and/or its subsidiaries.

// Package transport abstracts how etcd-recovery reaches a remote node.
package transport

// Supported transport types.
const (
	SSH     = "ssh"
	VMTools = "vmtools"
)

// Client runs commands on, and moves files to and from, a single remote node.
type Client interface {
	// Run runs cmd and returns its combined stdout and stderr.
	Run(cmd string) ([]byte, error)
	// Upload copies a local file to the remote path.
	Upload(localPath string, remotePath string) error
	// Download copies a remote file to the local path.
	Download(remotePath string, localPath string) error
	// Close releases the underlying connection.
	Close() error
}
