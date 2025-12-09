// Copyright (c) 2025 Broadcom. All Rights Reserved.
// Broadcom Confidential. The term "Broadcom" refers to Broadcom Inc.
// and/or its subsidiaries.

package commands

import (
	"fmt"
	"log"

	"github.com/spf13/cobra"

	"github.com/vmware/etcd-recovery/pkg/cliui"
	"github.com/vmware/etcd-recovery/pkg/config"
	"github.com/vmware/etcd-recovery/pkg/ssh"
)

// NewCommandExecute executes command against host(s)
// Runs command against single host if user selects specific host
// Runs command against all hosts if user selects all
func NewCommandExecute() *cobra.Command {
	return &cobra.Command{
		Use:   "exec",
		Short: "Execute command against host(s)",
		Run:   executeCommandFunc,
	}
}

func executeCommandFunc(cmd *cobra.Command, args []string) {
	hosts, err := config.ParseHostFromFile(configFile)
	if err != nil {
		log.Fatalf("Error parsing hosts config file: %v", err)
	}

	if len(hosts) == 0 {
		log.Fatalf("hosts.json should contain at least one Host, got: %d", len(hosts))
	}

	options := make([]string, len(hosts)+1)
	for i, h := range hosts {
		options[i] = fmt.Sprintf("%s (%s)", h.Name, h.Host)
	}

	options[len(hosts)] = "all"

	idx, _, err := cliui.Select(
		"Select the member to execute command against with:",
		options,
	)
	if err != nil {
		// user didn't select any host
		log.Fatalf("no host selected, exiting: %v", err)
	}

	if idx == len(hosts) {
		for _, host := range hosts {
			out, err := executeUserCommand(host, userCmd)
			if err != nil {
				log.Printf("Error executing command %q on host (%s: %s), output:\n %s\n error:\n %v\n", userCmd, host.Name, host.Host, string(out), err)
				continue
			}
			printLog("output:\n %s\n", string(out))
		}
	} else {
		out, err := executeUserCommand(hosts[idx], userCmd)
		if err != nil {
			log.Fatalf("Error executing command %q on host (%s: %s), output:\n %s\n error:\n %v\n", userCmd, hosts[idx].Name, hosts[idx].Host, string(out), err)
		}
		printLog("output:\n %s\n", string(out))
	}
}

func executeUserCommand(host *config.Host, command string) ([]byte, error) {
	printLog("Connecting to host (%s: %s)\n", host.Name, host.Host)

	client, err := ssh.NewClient(&ssh.Config{
		User:                 host.Username,
		Host:                 host.Host,
		Password:             host.Password,
		PrivateKeyPath:       host.PrivateKey,
		PrivateKeyPassphrase: host.Passphrase,
	})
	if err != nil {
		log.Fatalf("Error creating ssh client to (%s: %s): %v", host.Name, host.Host, err)
	}
	defer client.Close()

	printLog("Executing command %q on host (%s: %s)\n", command, host.Name, host.Host)
	return client.Run(command)
}
