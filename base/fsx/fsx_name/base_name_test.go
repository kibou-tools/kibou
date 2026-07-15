// Copyright 2026 Varun Gandhi
//
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package fsx_name_test

import (
	"fmt"
	"path/filepath"
	"testing"

	"code.kibou.tools/base/assert"
	"code.kibou.tools/base/check"
	"code.kibou.tools/base/fsx/fsx_name"
)

func TestExtractBaseName(t *testing.T) {
	h := check.New(t)
	h.Parallel()

	h.Run("Unit", testExtractBaseNameUnit)
}

func testExtractBaseNameUnit(h check.Harness) {
	h.Parallel()

	type testCase struct {
		name        string
		path        string
		wantName    string
		wantKind    fsx_name.BaseNameErrorKind
		wantMessage string
	}

	trailingSeparatorPath := "parent" + string(filepath.Separator)
	testCases := []testCase{
		{
			name:        "valid",
			path:        filepath.Join("parent", "child.txt"),
			wantName:    "child.txt",
			wantKind:    0,
			wantMessage: "",
		},
		{
			name:        "empty string",
			path:        "",
			wantName:    "",
			wantKind:    fsx_name.BaseNameErrorKind_EmptyString,
			wantMessage: "cannot extract base name from empty path string",
		},
		{
			name:     "trailing separator",
			path:     trailingSeparatorPath,
			wantName: "",
			wantKind: fsx_name.BaseNameErrorKind_EndsWithPathSeparator,
			wantMessage: fmt.Sprintf(
				"path %q ends with a path separator; cannot extract base name",
				trailingSeparatorPath,
			),
		},
		{
			name:        "current directory",
			path:        ".",
			wantName:    "",
			wantKind:    fsx_name.BaseNameErrorKind_NoBaseName,
			wantMessage: `path "." has no concrete base name`,
		},
		{
			name:        "parent directory",
			path:        "..",
			wantName:    "",
			wantKind:    fsx_name.BaseNameErrorKind_NoBaseName,
			wantMessage: `path ".." has no concrete base name`,
		},
	}

	for _, tc := range testCases {
		h.Run(tc.name, func(h check.Harness) {
			h.Parallel()

			name, err := fsx_name.ExtractBaseName(tc.path)
			if tc.wantMessage == "" {
				h.Assertf(err == nil, "ExtractBaseName returned error: %v", err)
				check.AssertSame(h, tc.wantName, name.String(), "ExtractBaseName base name")
				mustName := fsx_name.MustExtractBaseName(tc.path)
				check.AssertSame(h, tc.wantName, mustName.String(), "MustExtractBaseName base name")
				return
			}

			h.Assertf(err != nil, "ExtractBaseName(%q) succeeded unexpectedly", tc.path)
			check.AssertSame(h, tc.wantKind, err.Kind(), "error kind")
			check.AssertSame(h, tc.path, err.Path(), "error path")
			check.AssertSame(h, tc.wantMessage, err.Error(), "error message")
			h.AssertPanicsWith(
				assert.NewError("precondition violation: %s", tc.wantMessage),
				func() {
					_ = fsx_name.MustExtractBaseName(tc.path)
				},
			)
		})
	}
}
