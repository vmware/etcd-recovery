// Copyright (c) 2025 Broadcom. All Rights Reserved.
// Broadcom Confidential. The term "Broadcom" refers to Broadcom Inc.
// and/or its subsidiaries.

package task

import (
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	cryptoSSH "golang.org/x/crypto/ssh"

	"github.com/vmware/etcd-recovery/pkg/ssh"
)

type CommandTask struct {
	Description string
	Command     string
	Check       *Check
}

type Check struct {
	// Type              string
	ExpectedExitCode  int
	ExpectedOutput    string
	NotExpectedOutput string
	TimeoutSec        int
	RetryIntervalSec  int
}

func (t *CommandTask) Name() string {
	return "CommandTask"
}

func (t *CommandTask) Run(client *ssh.Client) (string, error) {
	var (
		start    = time.Now()
		timeout  = 10 * time.Second // sensible default timeout
		interval = time.Second      // sensible default interval
	)
	if t.Check != nil {
		if t.Check.TimeoutSec > 0 {
			timeout = time.Duration(t.Check.TimeoutSec) * time.Second
		}
		if t.Check.RetryIntervalSec > 0 {
			interval = time.Duration(t.Check.RetryIntervalSec) * time.Second
		}
	}

	var lasterr error

	for time.Since(start) < timeout {
		out, err := client.Run(t.Command)
		exitCode := 0
		if err != nil {
			// Try to extract exit code from error if possible
			var ee *cryptoSSH.ExitError
			if !errors.As(err, &ee) {
				// Not an ExitError, treat as command execution failure
				log.Printf("command '%s' execution failed: %v\n", t.Command, err)
				lasterr = err
				time.Sleep(interval)
				continue
			}
			exitCode = ee.ExitStatus()
		}

		// validation check
		// check expected exit code
		if t.Check != nil && exitCode != t.Check.ExpectedExitCode {
			log.Printf("command '%s' validation failed: expected exit code : %d, got: %d\n", t.Command, t.Check.ExpectedExitCode, exitCode)
			lasterr = fmt.Errorf("command '%s' validation failed: expected exit code %d but got %d", t.Command, t.Check.ExpectedExitCode, exitCode)
			time.Sleep(interval)
			continue
		}

		if t.Check != nil && t.Check.ExpectedOutput != "" && !strings.Contains(string(out), t.Check.ExpectedOutput) {
			log.Printf("command '%s' validation failed: expected output : %s not found\n", t.Command, t.Check.ExpectedOutput)
			lasterr = fmt.Errorf("command '%s' validation failed: expected output : %s not found", t.Command, t.Check.ExpectedOutput)
			time.Sleep(interval)
			continue
		}
		if t.Check != nil && t.Check.NotExpectedOutput != "" && strings.Contains(string(out), t.Check.NotExpectedOutput) {
			log.Printf("command '%s' validation failed: not expected output : %s found\n", t.Command, t.Check.NotExpectedOutput)
			lasterr = fmt.Errorf("command '%s' validation failed: not expected output : %s found", t.Command, t.Check.NotExpectedOutput)
			time.Sleep(interval)
			continue
		}

		if err == nil && string(out) != "" {
			return string(out), nil
		}
	}

	if lasterr != nil {
		return "", fmt.Errorf("command '%s' failed after timed out, error: %w", t.Command, lasterr)
	}

	return "", fmt.Errorf("command '%s' failed after timed out", t.Command)
}

// Example usage:

// healthCheck := &CommandTask{
//     Description: "Check etcd health",
//     Command:     "etcdctl endpoint health",
//     Check: &Check{
//         ExpectedExitCode: 0,
//         ExpectedOutput:    "1234",
//	       NotExpectedOutput:  "1456"
//         TimeoutSec:       30,
//         RetryIntervalSec: 5,
//     },
// }
//
//
// healthCheck := &task.CommandTask{
//     Description: "Check etcd health",
//     Command:     "etcdctl endpoint health",
//     Check:       &task.Check{ExpectedExitCode: 0},
// }
//
// downloadTask := &task.CommandTask{
//     Description: "Download etcd.yaml",
//     Command:     "cp /etc/kubernetes/manifests/etcd.yaml /tmp/etcd.yaml",
// }
//
// restartTask := &task.CommandTask{
//     Description: "Restart etcd",
//     Command:     "systemctl restart etcd",
// }
