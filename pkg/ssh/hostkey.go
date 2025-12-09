// Copyright (c) 2025 Broadcom. All Rights Reserved.
// Broadcom Confidential. The term "Broadcom" refers to Broadcom Inc.
// and/or its subsidiaries.

package ssh

import (
	"bufio"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

// InteractiveHostKeyCallback creates a host key callback that prompts the user
// interactively when encountering an unknown host key. If the user accepts,
// the host key is added to the known_hosts file.
//
// This callback is idempotent - if a host key is already in known_hosts,
// it will be validated without prompting the user.
func InteractiveHostKeyCallback(knownHostsPath string) (ssh.HostKeyCallback, error) {
	// Create known_hosts file if it doesn't exist
	if err := ensureKnownHostsFile(knownHostsPath); err != nil {
		return nil, fmt.Errorf("failed to ensure known_hosts file exists: %w", err)
	}

	// Return a callback that handles unknown hosts interactively
	return func(hostname string, remote net.Addr, key ssh.PublicKey) error {
		// Create a fresh knownhosts callback for each connection attempt
		// This ensures it picks up any changes to the known_hosts file.
		currentKnownHostsCallback, err := knownhosts.New(knownHostsPath)
		if err != nil {
			// If we can't create the callback, treat it as an unknown host scenario
			// to allow the user to add the key.
			currentKnownHostsCallback = func(_ string, _ net.Addr, _ ssh.PublicKey) error {
				return &knownhosts.KeyError{Want: []knownhosts.KnownKey{}}
			}
		}

		// Create a version of the hostname that includes the port, for consistent lookup
		lookupHostname := hostname
		if tcpAddr, ok := remote.(*net.TCPAddr); ok {
			// If the hostname does not already contain a port, append the port from remote
			if !strings.Contains(hostname, ":") {
				lookupHostname = net.JoinHostPort(hostname, fmt.Sprint(tcpAddr.Port))
			}
		}

		// First, try the standard knownhosts validation with the lookupHostname
		err = currentKnownHostsCallback(lookupHostname, remote, key) // Pass lookupHostname
		if err == nil {
			// Host is already known, no action needed
			return nil
		}

		// Check if this is an "unknown host" error
		var keyErr *knownhosts.KeyError
		if errors.As(err, &keyErr) {
			// Host is unknown - prompt user
			return promptAndAddHostKey(hostname, remote, key, knownHostsPath, keyErr)
		}

		// Check if this is a parsing error (e.g., empty known_hosts file or format issue)
		// Treat parsing errors as unknown host to allow user to add it
		errStr := err.Error()
		if strings.Contains(errStr, "missing port") || strings.Contains(errStr, "SplitHostPort") {
			// Create a KeyError-like structure for unknown host
			keyErr = &knownhosts.KeyError{
				Want: []knownhosts.KnownKey{},
			}
			return promptAndAddHostKey(hostname, remote, key, knownHostsPath, keyErr)
		}

		// This is a different error (e.g., changed key), return it
		return err
	}, nil
}

// promptAndAddHostKey prompts the user to accept or reject an unknown host key
// and adds it to known_hosts if accepted.
func promptAndAddHostKey(hostname string, remote net.Addr, key ssh.PublicKey, knownHostsPath string, keyErr *knownhosts.KeyError) error {
	// Get the fingerprint of the host key
	fingerprint := getHostKeyFingerprint(key)

	// Display the prompt similar to OpenSSH
	fmt.Printf("\nThe authenticity of host '%s (%s)' can't be established.\n", hostname, remote.String())
	fmt.Printf("%s key fingerprint is %s.\n", key.Type(), fingerprint)
	if len(keyErr.Want) > 0 {
		fmt.Printf("This host key is known but does not match. Possible man-in-the-middle attack!\n")
	} else {
		fmt.Printf("This key is not known by any other names.\n")
	}
	fmt.Printf("Are you sure you want to continue connecting (yes/no/[fingerprint])? ")

	// Read user input
	reader := bufio.NewReader(os.Stdin)
	response, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("failed to read user input: %w", err)
	}

	response = strings.TrimSpace(strings.ToLower(response))

	// Validate user response
	if response != "yes" && response != "y" && response != strings.ToLower(fingerprint) {
		return fmt.Errorf("host key verification cancelled by user")
	}

	// Add the host key to known_hosts
	if err := addHostKeyToKnownHosts(hostname, remote, key, knownHostsPath); err != nil {
		return fmt.Errorf("failed to add host key to known_hosts: %w", err)
	}

	fmt.Printf("Warning: Permanently added '%s' (%s) to the list of known hosts.\n", hostname, key.Type())
	return nil
}

// getHostKeyFingerprint returns the SHA256 fingerprint of the host key
// in the format used by OpenSSH (SHA256:...).
func getHostKeyFingerprint(key ssh.PublicKey) string {
	hash := sha256.Sum256(key.Marshal())
	return "SHA256:" + base64.StdEncoding.EncodeToString(hash[:])
}

// addHostKeyToKnownHosts adds a host key to the known_hosts file.
func addHostKeyToKnownHosts(hostname string, remote net.Addr, key ssh.PublicKey, knownHostsPath string) error {
	// Open known_hosts file in append mode
	file, err := os.OpenFile(knownHostsPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("failed to open known_hosts file: %w", err)
	}
	defer file.Close()

	// Use knownhosts.Line to format the entry correctly.
	// It internally handles normalization and uses both hostname and remote address.
	var addresses []string
	// Always add the provided hostname
	addresses = append(addresses, hostname)

	// If remote is a TCP address, add its IP string to ensure lookup by IP also works.
	if tcpAddr, ok := remote.(*net.TCPAddr); ok {
		// Add the IP address string, ensuring it's not a duplicate of the hostname if hostname is already an IP
		if tcpAddr.IP.String() != hostname {
			addresses = append(addresses, tcpAddr.IP.String())
		}
	}

	entry := knownhosts.Line(addresses, key)

	// knownhosts.Line doesn't include newline, so add it
	if _, err := file.WriteString(entry + "\n"); err != nil {
		return fmt.Errorf("failed to write to known_hosts file: %w", err)
	}

	return nil
}

// ensureKnownHostsFile ensures the known_hosts file and its directory exist.
func ensureKnownHostsFile(knownHostsPath string) error {
	// Get the directory path
	dir := filepath.Dir(knownHostsPath)

	// Create directory if it doesn't exist
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("failed to create .ssh directory: %w", err)
	}

	// Create file if it doesn't exist (with proper permissions)
	if _, err := os.Stat(knownHostsPath); os.IsNotExist(err) {
		file, err := os.Create(knownHostsPath)
		if err != nil {
			return fmt.Errorf("failed to create known_hosts file: %w", err)
		}
		file.Close()

		// Set proper permissions (read/write for owner only)
		if err := os.Chmod(knownHostsPath, 0o600); err != nil {
			return fmt.Errorf("failed to set known_hosts file permissions: %w", err)
		}
	}

	return nil
}
