// Copyright (c) 2025 Broadcom. All Rights Reserved.
// Broadcom Confidential. The term "Broadcom" refers to Broadcom Inc.
// and/or its subsidiaries.

package task

import "github.com/vmware/etcd-recovery/pkg/ssh"

type Task interface {
	Name() string
	Run(client *ssh.Client) (string, error)
}
