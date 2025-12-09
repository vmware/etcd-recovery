// Copyright (c) 2025 Broadcom. All Rights Reserved.
// Broadcom Confidential. The term "Broadcom" refers to Broadcom Inc.
// and/or its subsidiaries.

package commands

import (
	"fmt"

	"github.com/spf13/cobra"
)

const (
	cliName        = "etcd-recovery"
	cliDescription = "A tool to automatically recover an etcd cluster when quorum is lost"
)

var (
	configFile string
	verbose    bool
	repairMode string
	userCmd    string

	rootCmd = &cobra.Command{
		Use:   cliName,
		Short: cliDescription,
	}
)

func init() {
	rootCmd.PersistentFlags().StringVarP(&configFile, "config", "c", "hosts.json", "path to etcd cluster hosts config file")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "enable verbose output")
	rootCmd.PersistentFlags().StringVarP(&repairMode, "mode", "m", "both", fmt.Sprintf("etcd cluster repair mode, valid modes are: %v", validModes))
	rootCmd.PersistentFlags().StringVarP(&userCmd, "command", "e", "", "command to execute against target host(s)")

	rootCmd.AddCommand(
		NewCommandVersion(),
		NewCommandSelect(),
		NewCommandRepair(),
		NewCommandExecute(),
	)
}

func RootCmd() *cobra.Command {
	return rootCmd
}
