// Copyright (c) 2025 Broadcom. All Rights Reserved.
// Broadcom Confidential. The term "Broadcom" refers to Broadcom Inc.
// and/or its subsidiaries.

package plan

func (p *ExecutionPlan) Execute() error {
	for _, session := range p.Sessions {
		client, err := session.Host.NewClient()
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
