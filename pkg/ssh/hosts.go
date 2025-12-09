// Copyright (c) 2025 Broadcom. All Rights Reserved.
// Broadcom Confidential. The term "Broadcom" refers to Broadcom Inc.
// and/or its subsidiaries.

package ssh

import (
	"fmt"
	"os"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

// DefaultKnownHosts returns host key callback from default known hosts path, and error if any.
func DefaultKnownHosts() (ssh.HostKeyCallback, error) {
	path, err := DefaultKnownHostsPath()
	if err != nil {
		return nil, err
	}

	return knownhosts.New(path)
}

// DefaultKnownHostsPath returns default user knows hosts file.
func DefaultKnownHostsPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%s/.ssh/known_hosts", home), err
}

// configureHostKeyCallback returns an interactive host key callback by default
// that prompts the user when encountering unknown hosts. If a custom callback
// is provided, it will be used instead.
func configureHostKeyCallback(hostKeyCallback ssh.HostKeyCallback) (ssh.HostKeyCallback, error) {
	if hostKeyCallback != nil {
		return hostKeyCallback, nil
	}

	// Use interactive callback by default
	path, err := DefaultKnownHostsPath()
	if err != nil {
		return nil, err
	}

	return InteractiveHostKeyCallback(path)
}
