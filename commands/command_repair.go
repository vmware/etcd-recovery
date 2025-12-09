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
	"github.com/vmware/etcd-recovery/pkg/plan"
	"github.com/vmware/etcd-recovery/pkg/task"
)

var validModes = []string{"add", "create", "both"}

func NewCommandRepair() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "repair",
		Short: "Perform etcd repair operations",
		Long: `Perform etcd repair operations on target VMs.
Supported Modes:
  - add: Add a new member to an existing cluster
  - create: Creates a single-member etcd cluster
  - both: Run both create and add actions sequentially
`,
		Args: cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			hosts, err := config.ParseHostFromFile(configFile)
			if err != nil {
				log.Fatalf("failed to parse hosts file: %v", err)
			}
			if err = validateParams(hosts, repairMode); err != nil {
				log.Fatalf("failed to validate params: %v", err)
			}

			printLog("Repair with mode %s, all hosts: %v", repairMode, createOptions(hosts))
			switch repairMode {
			case "add":
				masterMember := mustSelectMember(hosts, "Select the initial member used to create the single-member cluster:")
				memberToAdd := mustSelectMember(getRemainingMembers(hosts, masterMember), "Select a learner member to add to the cluster:")
				mustAddMemberToCluster(hosts, masterMember, memberToAdd)
			case "create":
				masterMember := mustSelectMember(hosts, "Select the member with the highest commit index to recover the cluster:")
				mustCreateSingleMemberCluster(masterMember)
			case "both":
				masterMember := mustSelectMember(hosts, "Select the member with the highest commit index to recover the cluster:")
				mustCreateSingleMemberCluster(masterMember)

				remainingHosts := getRemainingMembers(hosts, masterMember)
				for i, h := range remainingHosts {
					printLog("Adding member %d/%d: %s (%s)", i+1, len(remainingHosts), h.Name, h.Host)
					mustAddMemberToCluster(hosts, masterMember, h)
				}
			default:
				log.Fatalf("Invalid repair mode: %s, , valid modes are %v", repairMode, validModes)
			}
		},
	}

	return cmd
}

func validateParams(hosts []*config.Host, mode string) error {
	if len(hosts) == 0 {
		return fmt.Errorf("hosts.json should contain at least one Host, got: %d", len(hosts))
	}

	if mode == "add" {
		if len(hosts) == 1 {
			return fmt.Errorf("hosts.json should contain at least two Host in 'add' mode, got: %d", len(hosts))
		}
	}

	return nil
}

func mustCreateSingleMemberCluster(selectedHost *config.Host) {
	printLog("Creating a single-member cluster from %s (%s)", selectedHost.Name, selectedHost.Host)

	session := &plan.RemoteSession{
		Host: selectedHost,
		Tasks: []task.Task{
			&task.CreateSingleMemberClusterTask{
				Description:    "CreateSingleMemberCluster",
				BackupManifest: selectedHost.BackedupManifest,
			},
		},
	}
	p := &plan.ExecutionPlan{
		Name:     "CreateSingleMemberCluster",
		Sessions: []*plan.RemoteSession{session},
	}

	if err := p.Execute(); err != nil {
		log.Fatalf("Failed to create single-member cluster: %v", err)
	}

	printLog("Single-member cluster created successfully.")
}

func createOptions(hosts []*config.Host) []string {
	options := make([]string, 0)
	for _, h := range hosts {
		options = append(options, fmt.Sprintf("%s (%s)", h.Name, h.Host))
	}
	return options
}

// mustSelectMember selects a member from a list of hosts. Use cases:
//   - Select the member with the highest commit index to recover the cluster, for mode "create" and "both"
//   - Select the initial member used to create the single-member cluster, for mode "add"
//   - Select a member to add into the cluster, for mode "add"
func mustSelectMember(hosts []*config.Host, msg string) *config.Host {
	options := createOptions(hosts)
	if len(options) == 0 {
		log.Fatal("No any hosts to select")
	}

	learnerIdx, _, err := cliui.Select(
		msg,
		options,
	)
	if err != nil {
		log.Fatalf("Failed to select member: %v", err)
	}

	return hosts[learnerIdx]
}

func mustAddMemberToCluster(allHosts []*config.Host, master, learner *config.Host) {
	printLog("Adding learner member %s (%s) to cluster via %s (%s)", learner.Name, learner.Host, master.Name, master.Host)

	// Execute workflow on master host to add the learner
	session := &plan.RemoteSession{
		Host: master,
		Tasks: []task.Task{
			&task.AddMemberTask{
				Description: "Add member workflow",
				Master:      master,
				Learner:     learner,
				AllHosts:    allHosts,
			},
		},
	}

	p := &plan.ExecutionPlan{
		Name:     "AddMember",
		Sessions: []*plan.RemoteSession{session},
	}

	if err := p.Execute(); err != nil {
		log.Fatalf("Failed to add member %s (%s) to cluster: %v", learner.Name, learner.Host, err)
	}

	printLog("Member added to cluster successfully.")
}

func getRemainingMembers(hosts []*config.Host, masterHost *config.Host) []*config.Host {
	var remainingHosts []*config.Host
	for _, h := range hosts {
		if h.Name != masterHost.Name {
			remainingHosts = append(remainingHosts, h)
		}
	}
	return remainingHosts
}
