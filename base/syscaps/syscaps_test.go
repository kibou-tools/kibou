// Copyright 2026 Varun Gandhi
//
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package syscaps_test

import (
	"fmt"
	"strings"
	"testing"

	"pgregory.net/rapid"

	"code.kibou.tools/base/assert"
	"code.kibou.tools/base/check"
	. "code.kibou.tools/base/check/prelude"
	"code.kibou.tools/base/collections"
	"code.kibou.tools/base/core/pathx"
	"code.kibou.tools/base/fsx/fsx_testkit"
	"code.kibou.tools/base/internal/constants"
	"code.kibou.tools/base/syscaps"
)

func TestTempFileNames(t *testing.T) {
	h := check.New(t)
	h.Parallel()

	h.Run("valid pattern", func(h check.Harness) {
		h.Parallel()

		var got string
		for name := range syscaps.TempFileNames(`foo\*bar-*.txt`) {
			got = name.String()
			break
		}
		h.Assertf(strings.HasPrefix(got, "foo*bar-tmp"), "name %q does not have expected prefix", got)
		h.Assertf(strings.HasSuffix(got, ".txt"), "name %q does not have expected suffix", got)
	})

	h.Run("preconditions", func(h check.Harness) {
		h.Parallel()

		type testCase struct {
			name    string
			pattern string
			want    assert.AssertionError
		}

		testCases := []testCase{
			{
				name:    "no wildcard",
				pattern: "foo.txt",
				want: assert.AssertionError{
					Fmt:  "precondition violation: temp file pattern %q does not contain a wildcard",
					Args: []any{"foo.txt"},
				},
			},
			{
				name:    "multiple wildcards",
				pattern: "***",
				want: assert.AssertionError{
					Fmt:  "precondition violation: temp file pattern %q contains more than one wildcard",
					Args: []any{"***"},
				},
			},
			{
				name:    "trailing escape",
				pattern: `foo-\`,
				want: assert.AssertionError{
					Fmt:  "precondition violation: temp file pattern %q has a trailing escape",
					Args: []any{`foo-\`},
				},
			},
			{
				name:    "invalid escape sequence",
				pattern: `foo-\x*`,
				want: assert.AssertionError{
					Fmt:  "precondition violation: temp file pattern %q contains invalid escape sequence",
					Args: []any{`foo-\x*`},
				},
			},
		}

		for _, tc := range testCases {
			h.Run(tc.name, func(h check.Harness) {
				h.Parallel()

				h.AssertPanicsWith(tc.want, func() {
					_ = syscaps.TempFileNames(tc.pattern)
				})
			})
		}
	})
}

func TestFSReadDirBatched(t *testing.T) {
	h := check.New(t)
	h.Parallel()

	repoFS := fsx_testkit.TempDirFS(h)

	rapid.Check(h.T(), func(t *rapid.T) {
		h := check.NewBasic(t)
		entryCount := rapid.IntRange(0, constants.ReadDirBatchSize*3).Draw(t, "entry_count")
		parentDir := Do(repoFS.MkdirTemp(pathx.Dot(), "entries-"))(h)

		want := collections.NewSet[string]()
		for i := range entryCount {
			name := fmt.Sprintf("file-%03d.txt", i)
			fileRel := parentDir.JoinComponents(name)
			h.NoErrorf(repoFS.WriteFile(fileRel, []byte("data"), 0o644), "WriteFile(%q)", fileRel)
			want.InsertNew(name)
		}

		got := collections.NewSet[string]()
		for entryRes := range repoFS.ReadDir(parentDir) {
			entry := Do(entryRes.Get())(h)
			name := entry.BaseName().String()
			got.InsertNew(name)

			info := Do(entry.Info())(h)
			h.Assertf(info.Name() == name, "Info(%q).Name() = %q, want %q", name, info.Name(), name)
			h.Assertf(!entry.IsDir(), "ReadDir(%q) returned directory entry %q, want file", parentDir, name)
			h.Assertf(!info.IsDir(), "Info(%q).IsDir() = true, want false", name)
		}

		check.AssertSame(h, collections.SortedValues(want), collections.SortedValues(got), "ReadDir entries")
	})
}

func TestFSReadDirOnFileReturnsError(t *testing.T) {
	h := check.New(t)
	h.Parallel()

	repoFS := fsx_testkit.TempDirFS(h)

	fileRel := pathx.MustParseRelPath("file.txt")
	h.NoErrorf(repoFS.WriteFile(fileRel, []byte("data"), 0o644), "WriteFile(%q)", fileRel)

	gotAny := false
	for entryRes := range repoFS.ReadDir(fileRel) {
		gotAny = true
		_, err := entryRes.Get()
		h.Assertf(err != nil, "ReadDir(%q) unexpectedly succeeded", fileRel)
	}
	h.Assertf(gotAny, "ReadDir(%q) produced no result", fileRel)
}

func TestFSMkdirTempRejectsEmptyPattern(t *testing.T) {
	h := check.New(t)
	h.Parallel()

	repoFS := fsx_testkit.TempDirFS(h)
	want := assert.AssertionError{Fmt: "precondition violation: pattern is empty", Args: nil}
	h.AssertPanicsWith(want, func() {
		_, _ = repoFS.MkdirTemp(pathx.Dot(), "")
	})
}
