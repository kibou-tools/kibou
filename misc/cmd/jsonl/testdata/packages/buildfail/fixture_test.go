// Copyright 2026 Varun Gandhi
//
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package buildfail

import "testing"

// TestBuildFail intentionally references an undefined symbol so the package
// fails to compile. This exercises the renderer's build-output / build-fail
// handling: go test -json emits the compiler diagnostics on build-output
// events keyed by ImportPath. The package builds nowhere else (it is its own
// module, outside the workspace) so it only fails when this harness drives it.
func TestBuildFail(t *testing.T) {
	thisSymbolDoesNotExist(t)
}
