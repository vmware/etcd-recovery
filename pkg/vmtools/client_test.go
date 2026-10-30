// Copyright (c) 2026 Broadcom. All Rights Reserved.
// Broadcom Confidential. The term "Broadcom" refers to Broadcom Inc.
// and/or its subsidiaries.

package vmtools

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/vmware/govmomi/vim25/soap"
	"github.com/vmware/govmomi/vim25/types"
)

func TestShellQuote(t *testing.T) {
	for in, want := range map[string]string{
		"echo hi":   `'echo hi'`,
		"it's":      `'it'\''s'`,
		"a; rm -rf": `'a; rm -rf'`,
	} {
		if got := shellQuote(in); got != want {
			t.Errorf("shellQuote(%q) = %s, want %s", in, got, want)
		}
	}
}

func soapFault(f types.AnyType) error {
	sf := &soap.Fault{}
	sf.Detail.Fault = f
	return soap.WrapSoapFault(sf)
}

func TestIsAccessDenied(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"plain error", errors.New("boom"), false},
		{"file fault", soapFault(types.FileFault{File: "/root/etcd.yaml"}), true},
		{"guest permission denied", soapFault(types.GuestPermissionDenied{}), true},
		{"wrapped file fault", fmt.Errorf("vmtools: download: %w", soapFault(types.FileFault{})), true},
		{"cannot access file", soapFault(types.CannotAccessFile{}), true},
		{"file not found", soapFault(types.FileNotFound{}), false},
		{"vcenter no permission", soapFault(types.NoPermission{}), false},
		{"invalid guest login", soapFault(types.InvalidGuestLogin{}), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isAccessDenied(tt.err); got != tt.want {
				t.Errorf("isAccessDenied(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

func TestExitErrorStatus(t *testing.T) {
	err := fmt.Errorf("run: %w", &ExitError{Code: 3})
	var ee interface{ ExitStatus() int }
	if !errors.As(err, &ee) || ee.ExitStatus() != 3 {
		t.Fatalf("expected exit status 3 from %v", err)
	}
}

func TestMakeTempPath(t *testing.T) {
	p := makeTempPath("/root/etcd.yaml")
	if !strings.HasPrefix(p, "/tmp/etcd-recovery_") || !strings.HasSuffix(p, "_etcd.yaml") {
		t.Errorf("unexpected temp path %q", p)
	}
}

func TestWithFaultType(t *testing.T) {
	if got := withFaultType(errors.New("boom")); got.Error() != "boom" {
		t.Errorf("non-fault error changed: %v", got)
	}
	got := withFaultType(soapFault(types.NoPermission{}))
	if !strings.Contains(got.Error(), "*types.NoPermission") {
		t.Errorf("fault type missing from %q", got)
	}
}
