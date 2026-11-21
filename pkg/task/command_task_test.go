// Copyright (c) 2026 Broadcom. All Rights Reserved.
// Broadcom Confidential. The term "Broadcom" refers to Broadcom Inc.
// and/or its subsidiaries.

package task

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/vmware/etcd-recovery/pkg/vmtools"
)

type fakeClient struct {
	out []byte
	err error
}

func (f *fakeClient) Run(string) ([]byte, error) { return f.out, f.err }
func (f *fakeClient) Upload(_, _ string) error   { return nil }
func (f *fakeClient) Download(_, _ string) error { return nil }
func (f *fakeClient) Close() error               { return nil }

// A non-zero exit from any transport must be reported as an exit code
// mismatch, not as a generic execution failure.
func TestCommandTaskRecognizesTransportExitErrors(t *testing.T) {
	task := &CommandTask{
		Command: "false",
		Check:   &Check{ExpectedExitCode: 0, TimeoutSec: 1, RetryIntervalSec: 1},
	}
	_, err := task.Run(&fakeClient{err: &vmtools.ExitError{Code: 3}})
	require.ErrorContains(t, err, "expected exit code 0 but got 3")
}
