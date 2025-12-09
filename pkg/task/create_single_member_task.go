// Copyright (c) 2025 Broadcom. All Rights Reserved.
// Broadcom Confidential. The term "Broadcom" refers to Broadcom Inc.
// and/or its subsidiaries.

package task

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"

	clientv3 "go.etcd.io/etcd/client/v3"
	corev1 "k8s.io/api/core/v1"

	"sigs.k8s.io/yaml"

	"github.com/vmware/etcd-recovery/pkg/ssh"
)

type CreateSingleMemberClusterTask struct {
	Description    string
	BackupManifest string
}

func (t *CreateSingleMemberClusterTask) Name() string {
	return "CreateSingleMemberCluster"
}

func (t *CreateSingleMemberClusterTask) Run(client *ssh.Client) (string, error) {
	var memberID string
	var isSingleMember bool
	// steps to create single-member etcd cluster
	// 1. Check if etcd container is running
	waitForEtcdRunningTask := &WaitForEtcdRunningTask{
		Description:      "Get etcd container ID",
		TimeoutSec:       15,
		RetryIntervalSec: 5,
	}
	oldContainerID, err := waitForEtcdRunningTask.Run(client)
	if err != nil {
		// if error, container is not running, proceed with creating single-member cluster
		// skip scenario
		log.Printf("etcd container didn't start: %v\n", err)
	}

	// update timeout and retry interval for waiting etcd to restart
	waitForEtcdRunningTask.TimeoutSec = 600
	waitForEtcdRunningTask.RetryIntervalSec = 5

	// 1.1 `oldContainerID` not empty means that the etcd container is running.
	if oldContainerID != "" {
		// verify if it is single member cluster by checking etcd member list
		// if memberList contains more than one member, skip with warning
		memberID, isSingleMember = isSingleMemberCluster(client, oldContainerID)
		if !isSingleMember {
			log.Println("WARNING: the etcd instance is part of a multi-member cluster; aborting single-member cluster creation")
			return memberID, nil
		}

		localEtcdPath := filepath.Join(os.TempDir(), filepath.Base(t.BackupManifest))
		// Download manifest from `/etc/kubernetes/manifests/etcd.yaml` to local temp path
		err = client.Download("/etc/kubernetes/manifests/etcd.yaml", localEtcdPath)
		if err != nil {
			return memberID, err
		}

		var pod corev1.Pod
		backupEtcdYaml, err := os.ReadFile(localEtcdPath)
		if err != nil {
			return memberID, err
		}
		err = yaml.Unmarshal(backupEtcdYaml, &pod)
		if err != nil {
			return memberID, err
		}

		// Remove --force-new-cluster from etcd container command if it exists
		pod, isManifestChanged, err := updateForceNewClusterCommand(pod, "etcd", false)
		if err != nil {
			return memberID, fmt.Errorf("failed to remove --force-new-cluster flag, err: %w", err)
		}

		if isManifestChanged {
			etcdYamlNoForce, err := yaml.Marshal(&pod)
			if err != nil {
				return memberID, err
			}

			tmpNoForce := filepath.Join(os.TempDir(), "etcd-no-force.yaml")
			err = os.WriteFile(tmpNoForce, etcdYamlNoForce, 0o644)
			if err != nil {
				return memberID, err
			}

			// Upload manifest without --force-new-cluster to `/etc/kubernetes/manifests/etcd.yaml`
			err = client.Upload(tmpNoForce, "/etc/kubernetes/manifests/etcd.yaml")
			if err != nil {
				return memberID, err
			}

			// update old container ID
			waitForEtcdRunningTask.OldContainerID = oldContainerID

			newContainerID, err := waitForEtcdRunningTask.Run(client)
			if err != nil {
				return memberID, fmt.Errorf("etcd did not restart: %w", err)
			}

			// final health check
			if err = waitForEtcdHealthyCommandTask(client, newContainerID); err != nil {
				return memberID, fmt.Errorf("etcd health check failed: %w", err)
			}

			//  Ensure it's a single member cluster
			memberID, isSingleMember = isSingleMemberCluster(client, newContainerID)
			if !isSingleMember {
				return memberID, fmt.Errorf("failed to create a single-member cluster")
			}
		} else {
			// final health check
			if err = waitForEtcdHealthyCommandTask(client, oldContainerID); err != nil {
				return memberID, fmt.Errorf("final etcd health check failed: %w", err)
			}

			//  Ensure it's a single member cluster
			memberID, isSingleMember = isSingleMemberCluster(client, oldContainerID)
			if !isSingleMember {
				return memberID, fmt.Errorf("failed to create a single-member cluster")
			}
		}
	} else {
		// etcd container is not running, proceed with creating single member cluster
		log.Println("etcd container is not running, proceeding with single-member cluster creation")

		// Download backup manifest to local temp path
		localEtcdPath := filepath.Join(os.TempDir(), filepath.Base(t.BackupManifest))
		if err := client.Download(t.BackupManifest, localEtcdPath); err != nil {
			return memberID, fmt.Errorf("failed to download backup manifest: %w", err)
		}

		// Parse manifest and add --force-new-cluster
		backupEtcdYaml, err := os.ReadFile(localEtcdPath)
		if err != nil {
			return memberID, fmt.Errorf("failed to read backup manifest: %w", err)
		}
		var pod corev1.Pod
		if err = yaml.Unmarshal(backupEtcdYaml, &pod); err != nil {
			return memberID, fmt.Errorf("failed to unmarshal manifest: %w", err)
		}
		// Add --force-new-cluster to etcd container command
		pod, _, err = updateForceNewClusterCommand(pod, "etcd", true)
		if err != nil {
			return memberID, fmt.Errorf("failed to add --force-new-cluster flag, err: %w", err)
		}

		// Marshal back to YAML
		etcdYamlWithForce, err := yaml.Marshal(&pod)
		if err != nil {
			return memberID, fmt.Errorf("failed to marshal manifest: %w", err)
		}
		tmpWithForce := filepath.Join(os.TempDir(), "etcd-force.yaml")
		if err = os.WriteFile(tmpWithForce, etcdYamlWithForce, 0o644); err != nil {
			return memberID, fmt.Errorf("failed to write temp manifest: %w", err)
		}

		// Upload manifest with --force-new-cluster
		if err = client.Upload(tmpWithForce, "/etc/kubernetes/manifests/etcd.yaml"); err != nil {
			return memberID, fmt.Errorf("failed to upload manifest: %w", err)
		}
		// Wait for etcd to start (container ID becomes available)
		containerID, err := waitForEtcdRunningTask.Run(client)
		if err != nil {
			return memberID, fmt.Errorf("etcd container didn't start in time: %w", err)
		}

		// Wait for etcd to become healthy
		if err := waitForEtcdHealthyCommandTask(client, containerID); err != nil {
			return memberID, fmt.Errorf("etcd did not become healthy: %w", err)
		}

		// Remove --force-new-cluster from manifest
		pod, _, err = updateForceNewClusterCommand(pod, "etcd", false)
		if err != nil {
			return memberID, fmt.Errorf("failed to remove --force-new-cluster flag, err: %w", err)
		}

		etcdYamlNoForce, err := yaml.Marshal(&pod)
		if err != nil {
			return memberID, fmt.Errorf("failed to marshal manifest: %w", err)
		}
		tmpNoForce := filepath.Join(os.TempDir(), "etcd-no-force.yaml")
		if err := os.WriteFile(tmpNoForce, etcdYamlNoForce, 0o644); err != nil {
			return memberID, fmt.Errorf("failed to write temp manifest: %w", err)
		}

		// Upload manifest without --force-new-cluster
		if err := client.Upload(tmpNoForce, "/etc/kubernetes/manifests/etcd.yaml"); err != nil {
			return memberID, fmt.Errorf("failed to upload manifest: %w", err)
		}

		// update old container ID
		waitForEtcdRunningTask.OldContainerID = containerID

		// Wait for etcd to restart (container ID changes)
		newContainerID, err := waitForEtcdRunningTask.Run(client)
		if err != nil {
			return memberID, fmt.Errorf("etcd did not restart: %w", err)
		}

		// Final health check
		if err := waitForEtcdHealthyCommandTask(client, newContainerID); err != nil {
			return memberID, fmt.Errorf("final etcd health check failed: %w", err)
		}

		// Ensure it's a single member cluster
		memberID, isSingleMember = isSingleMemberCluster(client, newContainerID)
		if !isSingleMember {
			return memberID, fmt.Errorf("failed to create a single-member cluster")
		}
	}

	return memberID, nil
}

func isSingleMemberCluster(client *ssh.Client, containerID string) (string, bool) {
	// prepare command task to check if single member cluster
	// use crictl exec to run etcdctl member list inside the etcd container
	singleMemberTask := &CommandTask{
		Description: "check if single-member cluster",
		Command:     fmt.Sprintf("sudo crictl exec %s etcdctl --endpoints=https://127.0.0.1:2379 --cert /etc/kubernetes/pki/etcd/healthcheck-client.crt --key /etc/kubernetes/pki/etcd/healthcheck-client.key --cacert /etc/kubernetes/pki/etcd/ca.crt member list -w json", strings.TrimSpace(containerID)),
		Check: &Check{
			ExpectedExitCode: 0,
			TimeoutSec:       60,
			RetryIntervalSec: 5,
		},
	}

	// verify if it is single member cluster by checking etcd member list
	var memberListResponse clientv3.MemberListResponse
	out, err := singleMemberTask.Run(client)
	if err != nil {
		return "", false
	}

	if err := json.Unmarshal([]byte(out), &memberListResponse); err != nil {
		return "", false
	}
	if len(memberListResponse.Members) == 1 {
		return strconv.FormatUint(memberListResponse.Members[0].ID, 10), true
	}
	return strconv.FormatUint(memberListResponse.Header.MemberId, 10), false
}

func waitForEtcdHealthyCommandTask(client *ssh.Client, containerID string) error {
	waitForEtcdToBeHealthyCommandTask := CommandTask{
		Description: "Wait for etcd to be healthy",
		Command:     fmt.Sprintf("sudo crictl exec %s etcdctl --endpoints=127.0.0.1:2379 --cert /etc/kubernetes/pki/etcd/healthcheck-client.crt --key /etc/kubernetes/pki/etcd/healthcheck-client.key --cacert /etc/kubernetes/pki/etcd/ca.crt endpoint health --cluster", strings.TrimSpace(containerID)),
		Check: &Check{
			ExpectedExitCode: 0,
			ExpectedOutput:   "is healthy",
			TimeoutSec:       600,
			RetryIntervalSec: 10,
		},
	}
	_, err := waitForEtcdToBeHealthyCommandTask.Run(client)
	return err
}

func updateForceNewClusterCommand(pod corev1.Pod, containerName string, add bool) (updatePod corev1.Pod, changed bool, err error) {
	var (
		containerFound = false
		configChanged  = false
	)
	for i, container := range pod.Spec.Containers {
		if strings.TrimSpace(container.Name) != containerName {
			continue
		}
		containerFound = true
		var newCmd []string
		for _, cmd := range container.Command {
			// always remove `--force-new-cluster` if present
			if !strings.HasPrefix(cmd, "--force-new-cluster") {
				newCmd = append(newCmd, cmd)
			}
		}

		if add {
			newCmd = append(newCmd, "--force-new-cluster")
		}

		configChanged = !compareSlice(pod.Spec.Containers[i].Command, newCmd)
		pod.Spec.Containers[i].Command = newCmd

		break
	}
	if !containerFound {
		return pod, false, fmt.Errorf("failed to find container name: %s", containerName)
	}

	return pod, configChanged, nil
}

func compareSlice(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}

	aCopy := append([]string(nil), a...)
	bCopy := append([]string(nil), b...)
	sort.Strings(aCopy)
	sort.Strings(bCopy)

	return slices.Equal(aCopy, bCopy)
}
