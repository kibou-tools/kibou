// Copyright 2026 Varun Gandhi
//
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package fsx_opt_test

import (
	"testing"

	"code.kibou.tools/base/check"
	"code.kibou.tools/base/fsx/fsx_opt"
)

func TestOpenOptionsBuilder(t *testing.T) {
	h := check.New(t)
	h.Parallel()

	_ = fsx_opt.Open(fsx_opt.OpenRW_ReadOnly).MustBuild()

	_ = fsx_opt.Open(fsx_opt.OpenRW_ReadWrite).WithMode(fsx_opt.OpenMode_CreateNew).MustBuild()

	_, err := fsx_opt.Open(fsx_opt.OpenRW_ReadOnly).WithMode(fsx_opt.OpenMode_CreateNew).Build()
	h.Assertf(err != nil, "CreateNew should require a write-capable mode")
	h.Assertf(err.Kind() == fsx_opt.OpenOptionsBuildErrorKind_MissingWritePermission, "unexpected error kind: %v", err.Kind())
	h.Assertf(err.NeededBy() == fsx_opt.OpenMode_CreateNew.String(), "unexpected NeededBy: %q", err.NeededBy())

	opts := fsx_opt.Open(fsx_opt.OpenRW_WriteOnly).WithMode(fsx_opt.OpenMode_CreateNew).WithMode(fsx_opt.OpenMode_CreateOrKeep).MustBuild()
	h.Assertf(opts.Mode() == fsx_opt.OpenMode_CreateOrKeep, "last WithMode call should win")
}
