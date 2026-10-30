// Copyright (c) 2026 Broadcom. All Rights Reserved.
// Broadcom Confidential. The term "Broadcom" refers to Broadcom Inc.
// and/or its subsidiaries.

package transport

import (
	"strings"
	"testing"
)

func TestNewUnknownType(t *testing.T) {
	c, err := New(&Options{Type: "telnet"})
	if err == nil || !strings.Contains(err.Error(), "unknown transport") {
		t.Fatalf("expected unknown transport error, got %v", err)
	}
	if c != nil {
		t.Fatal("client must be a nil interface on error")
	}
}

func TestNewVMToolsRequiresURLAndVM(t *testing.T) {
	c, err := New(&Options{Type: VMTools})
	if err == nil {
		t.Fatal("expected error for missing url/vm")
	}
	if c != nil {
		t.Fatal("client must be a nil interface on error")
	}
}
