// Copyright 2026 Varun Gandhi
//
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package fsx_test

import (
	"io"
	"strings"
	"testing"

	"code.kibou.tools/base/check"
	. "code.kibou.tools/base/check/prelude"
	"code.kibou.tools/base/core/pathx"
	"code.kibou.tools/base/fsx"
	"code.kibou.tools/base/fsx/fsx_testkit"
)

func TestFSOpenOptions(t *testing.T) {
	h := check.New(t)
	h.Parallel()

	repoFS := fsx_testkit.TempDirFS(h)
	rel := pathx.MustParseRelPath("capture.jsonl")

	createNewOpts := fsx.NewOpenOptions(fsx.OpenRW_WriteOnly).
		WithMode(fsx.OpenMode_CreateNew).
		MustBuild()
	f := Do(repoFS.OpenFile(rel, createNewOpts))(h)
	_, err := io.WriteString(f, "first\n")
	h.NoErrorf(err, "write created file")
	h.Close(f)

	_, err = repoFS.OpenFile(rel, createNewOpts)
	h.Assertf(err != nil, "CreateNew should fail when the file already exists")

	truncateOpts := fsx.NewOpenOptions(fsx.OpenRW_WriteOnly).
		WithTruncateIfPresent(true).
		MustBuild()
	f = Do(repoFS.OpenFile(rel, truncateOpts))(h)
	h.Close(f)
	data := Do(repoFS.ReadFile(rel))(h)
	h.Assertf(len(data) == 0, "TruncateIfPresent should empty the file, got %q", data)

	appendOpts := fsx.NewOpenOptions(fsx.OpenRW_WriteOnly).
		WithAppend(true).
		MustBuild()
	f = Do(repoFS.OpenFile(rel, appendOpts))(h)
	_, err = io.WriteString(f, "appended\n")
	h.NoErrorf(err, "append")
	h.Close(f)
	data = Do(repoFS.ReadFile(rel))(h)
	h.Assertf(strings.HasSuffix(string(data), "appended\n"), "Append should write at the end, got %q", data)
}
