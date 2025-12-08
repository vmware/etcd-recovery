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
			"username": "root",
			"password": "changeme",
			"backedup_manifest": "/root/etcd.yaml"
		},
		{
			"name": "etcd-vm2",
			"member_name": "5338fecd97945f54e50b8b5b21f88e73",
			"host": "10.100.72.8",
			"username": "root",
			"password": "changeme",
			"backedup_manifest": "/root/etcd.yaml"
		}, 
		{
			"name": "etcd-vm3",
			"member_name": "6449fecd97945f54e50b8b5b21f88e84",
			"host": "10.100.72.9",
			"username": "root",
			"password": "changeme",
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
			Username:         "root",
			Password:         "changeme",
			BackedupManifest: "/root/etcd.yaml",
		},
		{
			Name:             "etcd-vm2",
			MemberName:       "5338fecd97945f54e50b8b5b21f88e73",
			Host:             "10.100.72.8",
			Username:         "root",
			Password:         "changeme",
			BackedupManifest: "/root/etcd.yaml",
		},
		{
			Name:             "etcd-vm3",
			MemberName:       "6449fecd97945f54e50b8b5b21f88e84",
			Host:             "10.100.72.9",
			Username:         "root",
			Password:         "changeme",
			BackedupManifest: "/root/etcd.yaml",
		},
	}

	require.Equal(t, want, got)
}
