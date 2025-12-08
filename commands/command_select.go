// Copyright (c) 2025 Broadcom. All Rights Reserved.
// Broadcom Confidential. The term "Broadcom" refers to Broadcom Inc.
// and/or its subsidiaries.

package commands

import (
	"fmt"
	"log"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/vmware/etcd-recovery/pkg/config"
	"github.com/vmware/etcd-recovery/pkg/ssh"
)

// NewCommandSelect selects the best member for cluster recovery.
// It depends on the `etcd-diagnosis commit-index` command. When running
// `etcd-recovery select`, if `etcd-diagnosis` is not present on the
// target VM, it will be automatically uploaded to `/tmp/etcd-diagnosis`.
// In the future, both `etcd-diagnosis` and `etcd-recovery` will be
// pre-packaged on every control plane VM (including Supervisor and VKS),
// removing the need for automatic upload.
func NewCommandSelect() *cobra.Command {
	return &cobra.Command{
		Use:   "select",
		Short: "Select the best member to recover the cluster from",
		Run:   selectCommandFunc,
	}
}

func selectCommandFunc(cmd *cobra.Command, args []string) {
	hostCfg, err := config.ParseHostFromFile(configFile)
	if err != nil {
		log.Fatalf("Error parsing hosts config file: %v", err)
	}

	var (
		bestHosts      []*config.Host
		maxCommitIndex int
	)
	for _, h := range hostCfg {
		printLog("Connecting to host (%s: %s)\n", h.Name, h.Host)

		client, err := ssh.NewClient(&ssh.Config{
			User:                 h.Username,
			Host:                 h.Host,
			Password:             h.Password,
			PrivateKeyPath:       h.PrivateKey,
			PrivateKeyPassphrase: h.Passphrase,
		})
		if err != nil {
			log.Printf("Error creating ssh client to (%s: %s): %v\n", h.Name, h.Host, err)
			continue
		}

		targetPath := getTargetPath(h.Username)
		_, err = client.Run(fmt.Sprintf("%s version", targetPath))
		if err != nil {
			printLog("Uploading etcd-diagnosis to %s on host (%s: %s)\n", targetPath, h.Name, h.Host)
			if uErr := client.Upload("./etcd-diagnosis", targetPath); uErr != nil {
				log.Fatalf("Error uploading etcd-diagnosis to %s on (%v: %v): %v", targetPath, h.Name, h.Host, uErr)
			}
		}

		commitIndexCmd := fmt.Sprintf("sudo %s commit-index /var/lib/etcd", targetPath)
		resp, err := client.Run(commitIndexCmd)
		if err != nil {
			// The directory /var/lib/etcd might have already been removed.
			log.Printf("Error running etcd-diagnosis on (%v: %v), output:\n %s\n error:\n %v\n", h.Name, h.Host, string(resp), err)
			continue
		}

		commitIndex, err := strconv.Atoi(strings.TrimSpace(string(resp)))
		if err != nil {
			log.Fatalf("Error converting commit index to int (%v: %v): %v", h.Name, h.Host, err)
		}

		printLog("Member (%s: %s), Commit index: %d\n", h.Name, h.Host, commitIndex)

		if commitIndex > maxCommitIndex {
			maxCommitIndex = commitIndex
			bestHosts = []*config.Host{h}
		} else if commitIndex == maxCommitIndex {
			bestHosts = append(bestHosts, h)
		}
	}

	fmt.Printf("The following members have the highest commit index (%d): \n", maxCommitIndex)
	for _, h := range bestHosts {
		fmt.Printf("- %s: %s\n", h.Name, h.Host)
	}
}

func getTargetPath(user string) string {
	if user == "root" {
		return "/root/etcd-diagnosis"
	}
	return fmt.Sprintf("/home/%s/etcd-diagnosis", user)
}
