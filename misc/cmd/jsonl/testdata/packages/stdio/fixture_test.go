// Copyright 2026 Varun Gandhi
//
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package stdio

import (
	"fmt"
	"os"
	"testing"
)

func TestStdoutStderr(t *testing.T) {
	fmt.Fprintln(os.Stdout, "stdout line")
	fmt.Fprintln(os.Stderr, "stderr line")
}
