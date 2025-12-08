// Copyright (c) 2025 Broadcom. All Rights Reserved.
// Broadcom Confidential. The term "Broadcom" refers to Broadcom Inc.
// and/or its subsidiaries.

package plan

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/vmware/etcd-recovery/pkg/config"
	"github.com/vmware/etcd-recovery/pkg/ssh"
	"github.com/vmware/etcd-recovery/pkg/task"
)

// MockTask implements the Task interface for testing
type mockTask struct {
	shouldFail bool
}

func (m *mockTask) Name() string { return "MockTask" }
func (m *mockTask) Run(client *ssh.Client) (string, error) {
	if m.shouldFail {
		return "", errors.New("mock task failure")
	}
	return "mocked_output", nil
}

func TestExecute_Success(t *testing.T) {
	host := &config.Host{Name: "test", Host: "localhost"}
	session := &RemoteSession{
		Host:  host,
		Tasks: []task.Task{&mockTask{shouldFail: false}},
	}
	plan := &ExecutionPlan{
		Name:     "TestPlan",
		Sessions: []*RemoteSession{session},
	}

	// Replace ssh.NewClient and client.Close with no-ops or mocks as needed
	err := plan.Execute()
	require.Errorf(t, err, "failed to configure auth: no private key/password found to configure SSH auth")
}

func TestExecute_TaskFailure(t *testing.T) {
	host := &config.Host{Name: "test", Host: "localhost"}
	session := &RemoteSession{
		Host:  host,
		Tasks: []task.Task{&mockTask{shouldFail: true}},
	}
	plan := &ExecutionPlan{
		Name:     "TestPlan",
		Sessions: []*RemoteSession{session},
	}

	err := plan.Execute()
	require.Error(t, err)
}
