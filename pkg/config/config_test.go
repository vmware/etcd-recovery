// Copyright (c) 2025 Broadcom. All Rights Reserved.
// Broadcom Confidential. The term "Broadcom" refers to Broadcom Inc.
// and/or its subsidiaries.

package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseHostFromFile(t *testing.T) {
	content := `[
		{
			"name": "etcd-vm1",
			"member_name": "4227fecd97945f54e50b8b5b21f88e62",
			"host": "10.100.72.7",
			"ssh": {"username": "root", "password": "changeme"},
			"backedup_manifest": "/root/etcd.yaml"
		},
		{
			"name": "etcd-vm2",
			"member_name": "5338fecd97945f54e50b8b5b21f88e73",
			"host": "10.100.72.8",
			"ssh": {"username": "root", "password": "changeme"},
			"backedup_manifest": "/root/etcd.yaml"
		}, 
		{
			"name": "etcd-vm3",
			"member_name": "6449fecd97945f54e50b8b5b21f88e84",
			"host": "10.100.72.9",
			"ssh": {"username": "root", "password": "changeme"},
			"backedup_manifest": "/root/etcd.yaml"
		}
	]`

	tmpFile := filepath.Join(t.TempDir(), "hosts.json")
	if err := os.WriteFile(tmpFile, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}

	got, err := ParseHostFromFile(tmpFile)
	if err != nil {
		t.Fatalf("ParseHostFromFile returned error: %v", err)
	}

	want := []*Host{
		{
			Name:             "etcd-vm1",
			MemberName:       "4227fecd97945f54e50b8b5b21f88e62",
			Host:             "10.100.72.7",
			SSH:              &SSHConfig{Username: "root", Password: "changeme"},
			BackedupManifest: "/root/etcd.yaml",
		},
		{
			Name:             "etcd-vm2",
			MemberName:       "5338fecd97945f54e50b8b5b21f88e73",
			Host:             "10.100.72.8",
			SSH:              &SSHConfig{Username: "root", Password: "changeme"},
			BackedupManifest: "/root/etcd.yaml",
		},
		{
			Name:             "etcd-vm3",
			MemberName:       "6449fecd97945f54e50b8b5b21f88e84",
			Host:             "10.100.72.9",
			SSH:              &SSHConfig{Username: "root", Password: "changeme"},
			BackedupManifest: "/root/etcd.yaml",
		},
	}

	require.Equal(t, want, got)
}

func TestParseHostFromFile_VMTools(t *testing.T) {
	content := `[{
		"name": "etcd-vm1",
		"host": "10.100.72.7",
		"backedup_manifest": "/root/etcd.yaml",
		"vmtools": {
			"vcenter_url": "https://vc.example.com/sdk",
			"vcenter_username": "administrator@vsphere.local",
			"vcenter_password": "vcpass",
			"vm": "/dc1/vm/etcd-vm1",
			"guest_username": "root",
			"guest_password": "changeme"
		}
	}]`
	tmpFile := filepath.Join(t.TempDir(), "hosts.json")
	require.NoError(t, os.WriteFile(tmpFile, []byte(content), 0o644))

	got, err := ParseHostFromFile(tmpFile)
	require.NoError(t, err)
	require.Len(t, got, 1)
	require.Equal(t, "root", got[0].LoginUser())
	require.Equal(t, "/dc1/vm/etcd-vm1", got[0].VMTools.VM)
}

func TestParseHostFromFile_LegacyFlatSSH(t *testing.T) {
	content := `[{"name": "a", "host": "1.1.1.1", "username": "root", "password": "x"}]`
	tmpFile := filepath.Join(t.TempDir(), "hosts.json")
	require.NoError(t, os.WriteFile(tmpFile, []byte(content), 0o644))

	_, err := ParseHostFromFile(tmpFile)
	require.ErrorContains(t, err, `unknown field "username"`)
}

func TestHostValidate(t *testing.T) {
	vm := &VMToolsConfig{VCenterURL: "https://vc/sdk", VM: "/dc/vm/a", GuestUsername: "root"}
	ssh := &SSHConfig{Username: "root"}
	tests := []struct {
		name    string
		host    Host
		wantErr bool
	}{
		{"ssh ok", Host{Name: "a", Host: "1.1.1.1", SSH: ssh}, false},
		{"ssh missing username", Host{Name: "a", Host: "1.1.1.1", SSH: &SSHConfig{}}, true},
		{"vmtools ok", Host{Name: "a", Host: "1.1.1.1", VMTools: vm}, false},
		{"vmtools incomplete", Host{Name: "a", Host: "1.1.1.1", VMTools: &VMToolsConfig{VCenterURL: "https://vc/sdk"}}, true},
		{"both transports", Host{Name: "a", Host: "1.1.1.1", SSH: ssh, VMTools: vm}, true},
		{"no transport", Host{Name: "a", Host: "1.1.1.1"}, true},
		{"missing host", Host{Name: "a", SSH: ssh}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.host.Validate()
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
