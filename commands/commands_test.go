// Copyright (c) 2025 Broadcom. All Rights Reserved.
// Broadcom Confidential. The term "Broadcom" refers to Broadcom Inc.
// and/or its subsidiaries.

package commands

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vmware/etcd-recovery/pkg/config"
)

// TestExecCommandHasCommandFlag verifies that --command / -e is registered on
// the exec subcommand and is NOT present on the root command.
func TestExecCommandHasCommandFlag(t *testing.T) {
	execCmd := NewCommandExecute()

	flag := execCmd.Flags().Lookup("command")
	require.NotNil(t, flag, "--command flag should be defined on exec subcommand")
	assert.Equal(t, "e", flag.Shorthand, "--command shorthand should be -e")
	assert.Equal(t, "", flag.DefValue, "--command default value should be empty string")

	rootFlag := rootCmd.PersistentFlags().Lookup("command")
	assert.Nil(t, rootFlag, "--command flag should NOT be defined on root command")
}

// TestRepairCommandHasModeFlag verifies that --mode / -m is registered on
// the repair subcommand and is NOT present on the root command.
func TestRepairCommandHasModeFlag(t *testing.T) {
	repairCmd := NewCommandRepair()

	flag := repairCmd.Flags().Lookup("mode")
	require.NotNil(t, flag, "--mode flag should be defined on repair subcommand")
	assert.Equal(t, "m", flag.Shorthand, "--mode shorthand should be -m")
	assert.Equal(t, "both", flag.DefValue, "--mode default value should be 'both'")

	rootFlag := rootCmd.PersistentFlags().Lookup("mode")
	assert.Nil(t, rootFlag, "--mode flag should NOT be defined on root command")
}

// TestRootCommandRetainsSharedFlags verifies that the shared --config and
// --verbose flags remain on the root command as persistent flags.
func TestRootCommandRetainsSharedFlags(t *testing.T) {
	configFlag := rootCmd.PersistentFlags().Lookup("config")
	require.NotNil(t, configFlag, "--config flag should be defined on root command")
	assert.Equal(t, "c", configFlag.Shorthand)
	assert.Equal(t, "hosts.json", configFlag.DefValue)

	verboseFlag := rootCmd.PersistentFlags().Lookup("verbose")
	require.NotNil(t, verboseFlag, "--verbose flag should be defined on root command")
	assert.Equal(t, "v", verboseFlag.Shorthand)
}

// TestValidateParams covers all branches of the validateParams helper.
func TestValidateParams(t *testing.T) {
	host := func(name string) *config.Host { return &config.Host{Name: name, Host: "127.0.0.1"} }

	tests := []struct {
		name    string
		hosts   []*config.Host
		mode    string
		wantErr bool
	}{
		{
			name:    "no hosts returns error",
			hosts:   []*config.Host{},
			mode:    "both",
			wantErr: true,
		},
		{
			name:    "add mode with single host returns error",
			hosts:   []*config.Host{host("etcd-vm1")},
			mode:    "add",
			wantErr: true,
		},
		{
			name:    "add mode with two hosts succeeds",
			hosts:   []*config.Host{host("etcd-vm1"), host("etcd-vm2")},
			mode:    "add",
			wantErr: false,
		},
		{
			name:    "create mode with single host succeeds",
			hosts:   []*config.Host{host("etcd-vm1")},
			mode:    "create",
			wantErr: false,
		},
		{
			name:    "both mode with multiple hosts succeeds",
			hosts:   []*config.Host{host("etcd-vm1"), host("etcd-vm2"), host("etcd-vm3")},
			mode:    "both",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateParams(tt.hosts, tt.mode)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestGetRemainingMembers verifies that the master host is excluded from the
// returned slice while all other hosts are preserved.
func TestGetRemainingMembers(t *testing.T) {
	vm1 := &config.Host{Name: "etcd-vm1", Host: "10.0.0.1"}
	vm2 := &config.Host{Name: "etcd-vm2", Host: "10.0.0.2"}
	vm3 := &config.Host{Name: "etcd-vm3", Host: "10.0.0.3"}
	all := []*config.Host{vm1, vm2, vm3}

	remaining := getRemainingMembers(all, vm1)
	require.Len(t, remaining, 2)
	assert.Equal(t, vm2, remaining[0])
	assert.Equal(t, vm3, remaining[1])
}

// TestGetRemainingMembersOnlyMaster verifies an empty slice is returned when
// the master is the only host.
func TestGetRemainingMembersOnlyMaster(t *testing.T) {
	vm1 := &config.Host{Name: "etcd-vm1", Host: "10.0.0.1"}
	remaining := getRemainingMembers([]*config.Host{vm1}, vm1)
	assert.Empty(t, remaining)
}

// TestCreateOptions verifies the human-readable option strings are formatted
// correctly.
func TestCreateOptions(t *testing.T) {
	hosts := []*config.Host{
		{Name: "etcd-vm1", Host: "10.0.0.1"},
		{Name: "etcd-vm2", Host: "10.0.0.2"},
	}

	opts := createOptions(hosts)
	require.Len(t, opts, 2)
	assert.Equal(t, "etcd-vm1 (10.0.0.1)", opts[0])
	assert.Equal(t, "etcd-vm2 (10.0.0.2)", opts[1])
}

// TestRepairCommandModeFlagAcceptsValidValues verifies that the --mode flag
// can be set to each of the documented valid values without error.
func TestRepairCommandModeFlagAcceptsValidValues(t *testing.T) {
	for _, mode := range validModes {
		t.Run(mode, func(t *testing.T) {
			repairCmd := NewCommandRepair()
			err := repairCmd.Flags().Set("mode", mode)
			assert.NoError(t, err, "setting --mode to %q should not return an error", mode)
			got, err := repairCmd.Flags().GetString("mode")
			require.NoError(t, err)
			assert.Equal(t, mode, got)
		})
	}
}

// TestExecCommandCommandFlagCanBeSet verifies the --command flag value can be
// retrieved after being set on the exec subcommand.
func TestExecCommandCommandFlagCanBeSet(t *testing.T) {
	execCmd := NewCommandExecute()
	err := execCmd.Flags().Set("command", "echo hello")
	require.NoError(t, err)
	got, err := execCmd.Flags().GetString("command")
	require.NoError(t, err)
	assert.Equal(t, "echo hello", got)
}
