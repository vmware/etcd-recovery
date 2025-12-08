// Copyright (c) 2025 Broadcom. All Rights Reserved.
// Broadcom Confidential. The term "Broadcom" refers to Broadcom Inc.
// and/or its subsidiaries.

package ssh

import (
	"crypto/rand"
	"crypto/rsa"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
)

// TestInteractiveHostKeyCallback_UnknownHost tests the interactive callback
// when encountering an unknown host.
func TestInteractiveHostKeyCallback_UnknownHost(t *testing.T) {
	// Create a temporary known_hosts file
	tempDir := t.TempDir()
	knownHostsPath := filepath.Join(tempDir, "known_hosts")

	// Create the callback
	callback, err := InteractiveHostKeyCallback(knownHostsPath)
	require.NoError(t, err)

	// Generate a test host key
	hostKey, err := generateTestHostKey()
	require.NoError(t, err)

	// Create a mock remote address
	remoteAddr := &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 22}

	// Test with user input "yes"
	t.Run("UserAcceptsHostKey", func(t *testing.T) {
		// Simulate user input "yes"
		input := "yes\n"
		oldStdin := os.Stdin
		defer func() { os.Stdin = oldStdin }()

		r, w, pipeErr := os.Pipe()
		require.NoError(t, pipeErr)
		os.Stdin = r

		go func() {
			defer w.Close()
			_, _ = w.WriteString(input)
		}()

		// Call the callback
		err := callback("testhost", remoteAddr, hostKey)
		require.NoError(t, err)

		// Verify the host key was added to known_hosts
		content, err := os.ReadFile(knownHostsPath)
		require.NoError(t, err)
		require.Contains(t, string(content), "testhost")
		require.Contains(t, string(content), "127.0.0.1")
	})

	// Test with user input "no"
	t.Run("UserRejectsHostKey", func(t *testing.T) {
		// Create a new callback with a fresh known_hosts file
		tempDir2 := t.TempDir()
		knownHostsPath2 := filepath.Join(tempDir2, "known_hosts")
		callback2, err := InteractiveHostKeyCallback(knownHostsPath2)
		require.NoError(t, err)

		// Simulate user input "no"
		input := "no\n"
		oldStdin := os.Stdin
		defer func() { os.Stdin = oldStdin }()

		r, w, pipeErr := os.Pipe()
		require.NoError(t, pipeErr)
		os.Stdin = r

		go func() {
			defer w.Close()
			_, _ = w.WriteString(input)
		}()

		// Call the callback
		err = callback2("testhost2", remoteAddr, hostKey)
		require.Error(t, err)
		require.Contains(t, err.Error(), "cancelled by user")

		// Verify the host key was NOT added to known_hosts
		content, err := os.ReadFile(knownHostsPath2)
		if err == nil {
			require.NotContains(t, string(content), "testhost2")
		}
	})
}

// TestInteractiveHostKeyCallback_KnownHost tests the interactive callback
// when the host is already in known_hosts.
func TestInteractiveHostKeyCallback_KnownHost(t *testing.T) {
	// Create a temporary known_hosts file
	tempDir := t.TempDir()
	knownHostsPath := filepath.Join(tempDir, "known_hosts")

	// Generate a test host key
	hostKey, err := generateTestHostKey()
	require.NoError(t, err)

	// Define host details with port
	hostname := "testhost"
	port := 22
	remoteAddr := &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: port}

	// Add the host key to known_hosts manually
	err = addHostKeyToKnownHosts(hostname, remoteAddr, hostKey, knownHostsPath)
	require.NoError(t, err)

	// Create the callback
	callback, err := InteractiveHostKeyCallback(knownHostsPath)
	require.NoError(t, err)

	// Call the callback - should not prompt since host is known
	// No need to redirect os.Stdin as no input is expected here.
	err = callback(hostname, remoteAddr, hostKey)
	require.NoError(t, err)
}

// TestGetHostKeyFingerprint tests the fingerprint generation.
func TestGetHostKeyFingerprint(t *testing.T) {
	hostKey, err := generateTestHostKey()
	require.NoError(t, err)

	fingerprint := getHostKeyFingerprint(hostKey)
	require.NotEmpty(t, fingerprint)
	require.True(t, strings.HasPrefix(fingerprint, "SHA256:"))
}

// TestAddHostKeyToKnownHosts tests adding a host key to known_hosts.
func TestAddHostKeyToKnownHosts(t *testing.T) {
	tempDir := t.TempDir()
	knownHostsPath := filepath.Join(tempDir, "known_hosts")

	hostKey, err := generateTestHostKey()
	require.NoError(t, err)

	remoteAddr := &net.TCPAddr{IP: net.ParseIP("192.168.1.1"), Port: 22}

	err = addHostKeyToKnownHosts("example.com", remoteAddr, hostKey, knownHostsPath)
	require.NoError(t, err)

	// Verify the entry was added
	content, err := os.ReadFile(knownHostsPath)
	require.NoError(t, err)
	require.Contains(t, string(content), "example.com")
	require.Contains(t, string(content), "192.168.1.1")
	require.Contains(t, string(content), hostKey.Type())
}

// TestEnsureKnownHostsFile tests the known_hosts file creation.
func TestEnsureKnownHostsFile(t *testing.T) {
	tempDir := t.TempDir()
	knownHostsPath := filepath.Join(tempDir, ".ssh", "known_hosts")

	err := ensureKnownHostsFile(knownHostsPath)
	require.NoError(t, err)

	// Verify file exists
	info, err := os.Stat(knownHostsPath)
	require.NoError(t, err)
	require.NotNil(t, info)

	// Verify permissions (should be 0600)
	require.Equal(t, os.FileMode(0o600), info.Mode().Perm())
}

// TestInteractiveHostKeyCallback_Idempotent tests that the callback is idempotent.
func TestInteractiveHostKeyCallback_Idempotent(t *testing.T) {
	tempDir := t.TempDir()
	knownHostsPath := filepath.Join(tempDir, "known_hosts")

	hostKey, err := generateTestHostKey()
	require.NoError(t, err)

	// Define host details with port
	// hostname := "testhost"
	port := 22
	// Use the IP address for remote to ensure consistent known_hosts entry
	remoteAddr := &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: port}

	// Create the callback
	callback, err := InteractiveHostKeyCallback(knownHostsPath)
	require.NoError(t, err)

	// First call - should prompt and add
	input := "yes\n"
	oldStdin := os.Stdin
	defer func() { os.Stdin = oldStdin }()

	r, w, pipeErr := os.Pipe()
	require.NoError(t, pipeErr)
	os.Stdin = r

	go func() {
		defer w.Close()
		_, _ = w.WriteString(input)
	}()

	// Pass the IP address directly for the hostname to the callback
	err = callback(remoteAddr.IP.String(), remoteAddr, hostKey)
	require.NoError(t, err)

	// Second call - should not prompt (idempotent)
	// Crucially, we do NOT redirect os.Stdin for the second call.
	// If the prompt were to appear, it would cause an EOF error.
	// The callback should now find the key in known_hosts and return nil immediately.
	err = callback(remoteAddr.IP.String(), remoteAddr, hostKey)
	require.NoError(t, err)

	// Verify only one entry in known_hosts
	content, err := os.ReadFile(knownHostsPath)
	require.NoError(t, err)
	lines := strings.Split(strings.TrimSpace(string(content)), "\n")
	// Filter out empty lines
	var nonEmptyLines []string
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			nonEmptyLines = append(nonEmptyLines, line)
		}
	}
	require.Lenf(t, nonEmptyLines, 1, "known_hosts should contain exactly one entry")
}

// generateTestHostKey generates a test RSA host key for testing.
func generateTestHostKey() (ssh.PublicKey, error) {
	// Generate a private key
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, fmt.Errorf("failed to generate private key: %w", err)
	}

	// Create SSH public key from RSA public key
	publicKey, err := ssh.NewPublicKey(&privateKey.PublicKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create SSH public key: %w", err)
	}

	return publicKey, nil
}

// TestInteractiveHostKeyCallback_WithFingerprintInput tests accepting with fingerprint.
func TestInteractiveHostKeyCallback_WithFingerprintInput(t *testing.T) {
	tempDir := t.TempDir()
	knownHostsPath := filepath.Join(tempDir, "known_hosts")

	hostKey, err := generateTestHostKey()
	require.NoError(t, err)

	fingerprint := getHostKeyFingerprint(hostKey)
	remoteAddr := &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 22}

	callback, err := InteractiveHostKeyCallback(knownHostsPath)
	require.NoError(t, err)

	// Simulate user input with fingerprint
	input := fingerprint + "\n"
	oldStdin := os.Stdin
	defer func() { os.Stdin = oldStdin }()

	r, w, pipeErr := os.Pipe()
	require.NoError(t, pipeErr)
	os.Stdin = r

	go func() {
		defer w.Close()
		_, _ = w.WriteString(input)
	}()

	err = callback("testhost", remoteAddr, hostKey)
	require.NoError(t, err)

	// Verify the host key was added
	content, err := os.ReadFile(knownHostsPath)
	require.NoError(t, err)
	require.Contains(t, string(content), "testhost")
}

// TestInteractiveHostKeyCallback_InvalidInput tests handling of invalid input.
func TestInteractiveHostKeyCallback_InvalidInput(t *testing.T) {
	tempDir := t.TempDir()
	knownHostsPath := filepath.Join(tempDir, "known_hosts")

	hostKey, err := generateTestHostKey()
	require.NoError(t, err)

	remoteAddr := &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 22}

	callback, err := InteractiveHostKeyCallback(knownHostsPath)
	require.NoError(t, err)

	// Simulate invalid user input
	input := "maybe\n"
	oldStdin := os.Stdin
	defer func() { os.Stdin = oldStdin }()

	r, w, pipeErr := os.Pipe()
	require.NoError(t, pipeErr)
	os.Stdin = r

	go func() {
		defer w.Close()
		_, _ = w.WriteString(input)
	}()

	err = callback("testhost", remoteAddr, hostKey)
	require.Error(t, err)
	require.Contains(t, err.Error(), "cancelled by user")
}
