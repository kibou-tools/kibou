// Copyright 2026 Varun Gandhi
//
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package unix

import (
	"testing"

	"code.kibou.tools/base/check"
	"code.kibou.tools/base/iterx"
)

func TestComponentsUnix(t *testing.T) {
	h := check.New(t)
	h.Parallel()

	testCases := []struct {
		name string
		path string
		want []string
	}{
		{name: "relative", path: "foo/bar", want: []string{"foo", "bar"}},
		{name: "repeated separators", path: "foo///bar", want: []string{"foo", "bar"}},
		{name: "rooted", path: "/foo/bar", want: []string{"foo", "bar"}},
		{name: "root", path: "/", want: nil},
	}

	for _, tc := range testCases {
		h.Run(tc.name, func(h check.Harness) {
			h.Parallel()

			got := iterx.Collect(ComponentsUnix(tc.path))
			check.AssertSame(h, tc.want, got, "components")
		})
	}
}
