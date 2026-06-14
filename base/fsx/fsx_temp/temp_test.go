// Copyright 2026 Varun Gandhi
//
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package fsx_temp_test

import (
	"iter"
	"testing"

	"code.kibou.tools/base/check"
	"code.kibou.tools/base/core/pathx"
	"code.kibou.tools/base/core/pathx/pathx_testkit"
	"code.kibou.tools/base/fsx"
	"code.kibou.tools/base/fsx/fsx_temp"
	"code.kibou.tools/base/fsx/fsx_testkit"
	"code.kibou.tools/base/iterx"
)

func TestNames(t *testing.T) {
	h := check.New(t)
	h.Parallel()

	h.Run("Unit", func(h check.Harness) {
		h.Parallel()

		nameSeq := fsx_temp.Names([]byte("capture-"), fragments("tmp0001", "tmp0002"), []byte(".jsonl"))
		names := iterx.Map(nameSeq, fsx.Name.String)
		check.AssertSame(h,
			[]string{"capture-tmp0001.jsonl", "capture-tmp0002.jsonl"},
			iterx.Collect(names),
			"names")
	})
}

func TestCreateFile(t *testing.T) {
	h := check.New(t)
	h.Parallel()

	h.Run("Unit", func(h check.Harness) {
		h.Parallel()

		h.Run("creates first available candidate", func(h check.Harness) {
			h.Parallel()

			repoFS := fsx_testkit.TempDirFS(h)
			fsx_testkit.WriteFile(h, repoFS, pathx.MustParseRelPath("capture-tmp0001.jsonl"), "taken")

			nameSeq := fsx_temp.Names(
				[]byte("capture-"),
				fragments("tmp0001", "tmp0002"),
				[]byte(".jsonl"))
			opts := fsx.NewOpenOptions(fsx.OpenRW_WriteOnly)
			f, err := fsx_temp.CreateFile(repoFS, pathx.Dot(), nameSeq, opts)
			h.Assertf(err == nil, "CreateFile returned error: %v", err)
			defer h.Close(f)

			check.AssertSame(h, "capture-tmp0002.jsonl", f.Name().String(), "created name")
		})

		h.Run("out of names", func(h check.Harness) {
			h.Parallel()

			repoFS := fsx_testkit.TempDirFS(h)
			fsx_testkit.WriteFile(h, repoFS, pathx.MustParseRelPath("capture-tmp0001.jsonl"), "taken")

			nameSeq := fsx_temp.Names([]byte("capture-"), fragments("tmp0001"), []byte(".jsonl"))
			opts := fsx.NewOpenOptions(fsx.OpenRW_WriteOnly)
			_, err := fsx_temp.CreateFile(repoFS, pathx.Dot(), nameSeq, opts)
			h.Assertf(err != nil, "CreateFile should fail when all candidates exist")
			check.AssertSame(h, fsx_temp.CreateFileErrorKind_OutOfNames, err.Kind(), "error kind")
		})

		h.Run("I/O error", func(h check.Harness) {
			h.Parallel()

			repoFS := fsx_testkit.TempDirFS(h)
			dir := pathx.MustParseRelPath("missing")

			nameSeq := fsx_temp.Names([]byte("capture-"), fragments("tmp0001"), []byte(".jsonl"))
			opts := fsx.NewOpenOptions(fsx.OpenRW_WriteOnly)
			_, err := fsx_temp.CreateFile(repoFS, dir, nameSeq, opts)
			h.Assertf(err != nil, "CreateFile should fail when parent directory is missing")
			check.AssertSame(h, fsx_temp.CreateFileErrorKind_IOError, err.Kind(), "error kind")
			check.AssertSame(h, dir.JoinOne(fsx.NewName("capture-tmp0001.jsonl")), err.Path(), "error path", pathx_testkit.CompareOptions()...)
			h.Assertf(err.Unwrap() != nil, "IOError should wrap the underlying error")
		})
	})
}

func fragments(values ...string) iter.Seq[[]byte] {
	return func(yield func([]byte) bool) {
		for _, value := range values {
			if !yield([]byte(value)) {
				return
			}
		}
	}
}
