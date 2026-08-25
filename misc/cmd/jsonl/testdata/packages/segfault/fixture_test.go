// Copyright 2026 Varun Gandhi
//
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package segfault

import (
	"os"
	"testing"
)

// sink keeps the nil dereference observable so the compiler cannot elide it.
var sink int

// TestSegfault dereferences a nil pointer, which the runtime delivers as
// SIGSEGV and prints as a crash traceback in go test -json. It is gated on
// JSONL_FIXTURE so it stays inert under an ordinary `go test ./...`.
func TestSegfault(t *testing.T) {
	if os.Getenv("JSONL_FIXTURE") != "segfault" {
		t.Skip("set JSONL_FIXTURE=segfault to exercise the crash path")
	}
	var p *int
	sink = *p
}
