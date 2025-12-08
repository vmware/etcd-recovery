// Copyright (c) 2025 Broadcom. All Rights Reserved.
// Broadcom Confidential. The term "Broadcom" refers to Broadcom Inc.
// and/or its subsidiaries.

package commands

import "log"

func printLog(format string, v ...any) {
	if verbose {
		log.Printf(format, v...)
	}
}
