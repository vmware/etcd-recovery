// Copyright (c) 2025 Broadcom. All Rights Reserved.
// Broadcom Confidential. The term "Broadcom" refers to Broadcom Inc.
// and/or its subsidiaries.

package plan

import (
	"github.com/vmware/etcd-recovery/pkg/config"
	"github.com/vmware/etcd-recovery/pkg/task"
)

type ExecutionPlan struct {
	Name     string
	Sessions []*RemoteSession
}

type RemoteSession struct {
	Host  *config.Host
	Tasks []task.Task
}
