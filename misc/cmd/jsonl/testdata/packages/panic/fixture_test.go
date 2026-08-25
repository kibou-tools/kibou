// Copyright 2026 Varun Gandhi
//
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package panicfix

import (
	"os"
	"testing"
)

// TestPanic panics so the renderer harness can capture how an uncaught panic
// appears in go test -json. It is gated on JSONL_FIXTURE so it stays inert
// under an ordinary `go test ./...`.
func TestPanic(t *testing.T) {
	if os.Getenv("JSONL_FIXTURE") != "panic" {
		t.Skip("set JSONL_FIXTURE=panic to exercise the panic path")
	}
	panic("boom")
}
