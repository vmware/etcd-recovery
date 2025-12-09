// Copyright (c) 2025 Broadcom. All Rights Reserved.
// Broadcom Confidential. The term "Broadcom" refers to Broadcom Inc.
// and/or its subsidiaries.

package plan

import (
	"github.com/vmware/etcd-recovery/pkg/ssh"
)

func (p *ExecutionPlan) Execute() error {
	for _, session := range p.Sessions {
		client, err := ssh.NewClient(&ssh.Config{
			User:                 session.Host.Username,
			Host:                 session.Host.Host,
			Password:             session.Host.Password,
			PrivateKeyPath:       session.Host.PrivateKey,
			PrivateKeyPassphrase: session.Host.Passphrase,
		})
		if err != nil {
			return err
		}
		defer client.Close()

		for _, task := range session.Tasks {
			// Run task
			if _, err := task.Run(client); err != nil {
				return err
			}
		}
	}
	return nil
}
