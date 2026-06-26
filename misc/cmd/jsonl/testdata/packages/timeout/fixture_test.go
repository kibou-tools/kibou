// Copyright 2026 Varun Gandhi
//
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package timeout

import (
	"os"
	"testing"
	"time"
)

// TestTimeout sleeps far longer than the harness's -timeout, so go test kills
// the binary and emits its "test timed out" panic. It is gated on
// JSONL_FIXTURE so it stays inert under an ordinary `go test ./...`.
func TestTimeout(t *testing.T) {
	if os.Getenv("JSONL_FIXTURE") != "timeout" {
		t.Skip("set JSONL_FIXTURE=timeout to exercise the timeout path")
	}
	time.Sleep(time.Hour)
}
