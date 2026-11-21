// Copyright (c) 2025 Broadcom. All Rights Reserved.
// Broadcom Confidential. The term "Broadcom" refers to Broadcom Inc.
// and/or its subsidiaries.

package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/vmware/etcd-recovery/pkg/ssh"
	"github.com/vmware/etcd-recovery/pkg/transport"
	"github.com/vmware/etcd-recovery/pkg/vmtools"
)

const DefaultConfigFilename = "hosts.json"

// SSHConfig holds the settings of the ssh transport. It lives under the "ssh"
// key of a host entry.
type SSHConfig struct {
	Username   string `json:"username"`
	Password   string `json:"password,omitempty"`
	PrivateKey string `json:"private_key,omitempty"`
	Passphrase string `json:"passphrase,omitempty"`
}

// VMToolsConfig holds the settings of the vmtools transport. It lives under the
// "vmtools" key of a host entry.
type VMToolsConfig struct {
	// VCenterURL, VCenterUsername and VCenterPassword address and authenticate
	// to vCenter or ESXi.
	VCenterURL      string `json:"vcenter_url"`
	VCenterUsername string `json:"vcenter_username,omitempty"`
	VCenterPassword string `json:"vcenter_password,omitempty"`
	VCenterInsecure bool   `json:"vcenter_insecure,omitempty"`
	// VM is the inventory path (starting with "/") or UUID of the VM.
	VM string `json:"vm"`
	// GuestUsername and GuestPassword are the guest OS credentials.
	GuestUsername string `json:"guest_username"`
	GuestPassword string `json:"guest_password,omitempty"`
}

// Host describes one control plane VM. Exactly one of SSH and VMTools must be
// set, and selects the transport used to reach the VM.
type Host struct {
	Name             string `json:"name"`
	MemberName       string `json:"member_name,omitempty"`
	Host             string `json:"host"`
	BackedupManifest string `json:"backedup_manifest"`

	SSH     *SSHConfig     `json:"ssh,omitempty"`
	VMTools *VMToolsConfig `json:"vmtools,omitempty"`
}

// LoginUser returns the OS user that commands run as on the host.
func (h *Host) LoginUser() string {
	switch {
	case h.VMTools != nil:
		return h.VMTools.GuestUsername
	case h.SSH != nil:
		return h.SSH.Username
	}
	return ""
}

// Validate checks that exactly one transport is configured and that its
// required settings are set.
func (h *Host) Validate() error {
	if h.Name == "" || h.Host == "" {
		return errors.New("name and host are required")
	}
	switch {
	case h.SSH != nil && h.VMTools != nil:
		return fmt.Errorf("host %s: set only one of ssh and vmtools", h.Name)
	case h.SSH != nil:
		if h.SSH.Username == "" {
			return fmt.Errorf("host %s: ssh.username is required", h.Name)
		}
	case h.VMTools != nil:
		v := h.VMTools
		if v.VCenterURL == "" || v.VM == "" || v.GuestUsername == "" {
			return fmt.Errorf("host %s: vmtools requires vcenter_url, vm and guest_username", h.Name)
		}
	default:
		return fmt.Errorf("host %s: one of ssh and vmtools is required", h.Name)
	}
	return nil
}

// NewClient connects to the host using the transport selected by its settings.
func (h *Host) NewClient() (transport.Client, error) {
	if err := h.Validate(); err != nil {
		return nil, err
	}
	if v := h.VMTools; v != nil {
		return transport.New(&transport.Options{
			Type: transport.VMTools,
			VMTools: vmtools.Config{
				URL:           v.VCenterURL,
				Username:      v.VCenterUsername,
				Password:      v.VCenterPassword,
				Insecure:      v.VCenterInsecure,
				VM:            v.VM,
				GuestUser:     v.GuestUsername,
				GuestPassword: v.GuestPassword,
			},
		})
	}
	return transport.New(&transport.Options{
		Type: transport.SSH,
		SSH: ssh.Config{
			User:                 h.SSH.Username,
			Host:                 h.Host,
			Password:             h.SSH.Password,
			PrivateKeyPath:       h.SSH.PrivateKey,
			PrivateKeyPassphrase: h.SSH.Passphrase,
		},
	})
}

func ParseHostFromFile(path string) ([]*Host, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read file failed: %w", err)
	}

	var hosts []*Host
	// Reject unknown fields so that a pre-"ssh" config with a top-level
	// username is reported instead of silently ignored.
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&hosts); err != nil {
		return nil, fmt.Errorf("unmarshal json failed: %w", err)
	}

	for _, h := range hosts {
		if err := h.Validate(); err != nil {
			return nil, fmt.Errorf("invalid host config: %w", err)
		}
	}

	return hosts, nil
}

// FetchMemberName returns the provided MemberName if it is not empty.
// Otherwise, it retrieves the hostname of the target host at runtime and uses it as the member name.
func (h *Host) FetchMemberName() (string, error) {
	if h.MemberName != "" {
		return h.MemberName, nil
	}

	client, err := h.NewClient()
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
