// Copyright (c) 2025 Broadcom. All Rights Reserved.
// Broadcom Confidential. The term "Broadcom" refers to Broadcom Inc.
// and/or its subsidiaries.

package task

import (
	"fmt"
	"log"
	"strings"

	"github.com/vmware/etcd-recovery/pkg/ssh"
)

// WaitForEtcdRunningTask waits for etcd container to be running
type WaitForEtcdRunningTask struct {
	Description      string
	OldContainerID   string
	TimeoutSec       int
	RetryIntervalSec int
}

func (t *WaitForEtcdRunningTask) Name() string {
	return "WaitForEtcdRunningTask"
}

func (t *WaitForEtcdRunningTask) Run(client *ssh.Client) (string, error) {
	task := &CommandTask{
		Description: "Wait for etcd container to be running",
		Command:     "sudo crictl ps --label io.kubernetes.container.name=etcd -q | head -n 1",
		Check: &Check{
			ExpectedExitCode:  0,
			NotExpectedOutput: t.OldContainerID,
			TimeoutSec:        t.TimeoutSec,
			RetryIntervalSec:  t.RetryIntervalSec,
		},
	}

	if task.Check.TimeoutSec == 0 {
		task.Check.TimeoutSec = 120
	}
	if task.Check.RetryIntervalSec == 0 {
		task.Check.RetryIntervalSec = 5
	}

	out, err := task.Run(client)
	if err != nil {
		return "", err
	}

	containerID := strings.TrimSpace(out)
	if containerID == "" {
		return "", fmt.Errorf("etcd container not running")
	}

	log.Printf("etcd container %s is running\n", containerID)
	return containerID, nil
}
