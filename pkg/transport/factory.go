// Copyright (c) 2026 Broadcom. All Rights Reserved.
// Broadcom Confidential. The term "Broadcom" refers to Broadcom Inc.
// and/or its subsidiaries.

package transport

import (
	"fmt"

	"github.com/vmware/etcd-recovery/pkg/ssh"
	"github.com/vmware/etcd-recovery/pkg/vmtools"
)

// Options selects a transport and carries the settings for all of them.
type Options struct {
	// Type is SSH or VMTools. Empty means SSH.
	Type string

	SSH     ssh.Config
	VMTools vmtools.Config
}

// New connects to a node using the transport selected in opts.
func New(opts *Options) (Client, error) {
	switch opts.Type {
	case "", SSH:
		c, err := ssh.NewClient(&opts.SSH)
		if err != nil {
			return nil, err
		}
		return c, nil
	case VMTools:
		c, err := vmtools.NewClient(&opts.VMTools)
		if err != nil {
			return nil, err
		}
		return c, nil
	default:
		return nil, fmt.Errorf("unknown transport %q (supported: %s, %s)", opts.Type, SSH, VMTools)
	}
}
