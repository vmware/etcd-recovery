// Copyright (c) 2026 Broadcom. All Rights Reserved.
// Broadcom Confidential. The term "Broadcom" refers to Broadcom Inc.
// and/or its subsidiaries.

package vmtools

import "testing"

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
