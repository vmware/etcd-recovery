// Copyright (c) 2025 Broadcom. All Rights Reserved.
// Broadcom Confidential. The term "Broadcom" refers to Broadcom Inc.
// and/or its subsidiaries.

package main

import (
	"fmt"
	"log"

	"github.com/vmware/etcd-recovery/pkg/cliui"
)

func main() {
	index, choice, err := cliui.Select("Please select one of the VMs:", []string{"etcd-vm1", "etcd-vm2", "etcd-vm3"})

	if err != nil {
		log.Fatalf("Error occurred during selection: %v", err)
	}

	fmt.Printf("Index: %d, Choice: %s\n", index, choice)
}
