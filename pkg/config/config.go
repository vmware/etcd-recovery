// Copyright (c) 2025 Broadcom. All Rights Reserved.
// Broadcom Confidential. The term "Broadcom" refers to Broadcom Inc.
// and/or its subsidiaries.

package config

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/vmware/etcd-recovery/pkg/ssh"
)

const DefaultConfigFilename = "hosts.json"

type Host struct {
	Name             string `json:"name"`
	MemberName       string `json:"member_name,omitempty"`
	Host             string `json:"host"`
	Username         string `json:"username"`
	Password         string `json:"password,omitempty"`
	PrivateKey       string `json:"private_key,omitempty"`
	Passphrase       string `json:"passphrase,omitempty"`
	BackedupManifest string `json:"backedup_manifest"`
}

func ParseHostFromFile(path string) ([]*Host, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read file failed: %w", err)
	}

	var hosts []*Host
	if err := json.Unmarshal(data, &hosts); err != nil {
		return nil, fmt.Errorf("unmarshal json failed: %w", err)
	}

	return hosts, nil
}

// FetchMemberName returns the provided MemberName if it is not empty.
// Otherwise, it retrieves the hostname of the target host at runtime and uses it as the member name.
func (h *Host) FetchMemberName() (string, error) {
	if h.MemberName != "" {
		return h.MemberName, nil
	}

	client, err := ssh.NewClient(&ssh.Config{
		User:                 h.Username,
		Host:                 h.Host,
		Password:             h.Password,
		PrivateKeyPath:       h.PrivateKey,
		PrivateKeyPassphrase: h.Passphrase,
	})
	if err != nil {
		return "", fmt.Errorf("failed to connect to host %s to fetch hostname: %w", h.Host, err)
	}
	defer client.Close()

	out, err := client.Run("hostname")
	if err != nil {
		return "", fmt.Errorf("failed to fetch hostname of remote machine: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}
