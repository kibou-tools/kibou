// Copyright 2026 Varun Gandhi
//
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package pathx

import (
	"slices"
	"testing"

	"code.kibou.tools/base/check"
	"code.kibou.tools/base/core/option"
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

func TestComponentsWindows(t *testing.T) {
	h := check.New(t)
	h.Parallel()

	testCases := []struct {
		name              string
		path              string
		wantKind          WindowsPathKind
		wantPrefix        option.Option[string]
		want              []string
		wantDOSDeviceName string
	}{
		{
			name:              "relative",
			path:              `foo\bar`,
			wantKind:          WindowsPathKind_Relative,
			wantPrefix:        option.None[string](),
			want:              []string{"foo", "bar"},
			wantDOSDeviceName: "",
		},
		{
			name:              "repeated separators",
			path:              `foo\\\bar`,
			wantKind:          WindowsPathKind_Relative,
			wantPrefix:        option.None[string](),
			want:              []string{"foo", "bar"},
			wantDOSDeviceName: "",
		},
		{
			name:              "root relative",
			path:              `\foo\bar`,
			wantKind:          WindowsPathKind_RootRelative,
			wantPrefix:        option.None[string](),
			want:              []string{"foo", "bar"},
			wantDOSDeviceName: "",
		},
		{
			name:              "root",
			path:              `\`,
			wantKind:          WindowsPathKind_RootRelative,
			wantPrefix:        option.None[string](),
			want:              nil,
			wantDOSDeviceName: "",
		},
		{
			name:              "drive relative",
			path:              `C:foo\bar`,
			wantKind:          WindowsPathKind_DriveRelative,
			wantPrefix:        option.Some("C:"),
			want:              []string{"foo", "bar"},
			wantDOSDeviceName: "",
		},
		{
			name:              "drive absolute",
			path:              `C:\foo\bar`,
			wantKind:          WindowsPathKind_DriveAbsolute,
			wantPrefix:        option.Some("C:"),
			want:              []string{"foo", "bar"},
			wantDOSDeviceName: "",
		},
		{
			name:              "drive root",
			path:              `C:\`,
			wantKind:          WindowsPathKind_DriveAbsolute,
			wantPrefix:        option.Some("C:"),
			want:              nil,
			wantDOSDeviceName: "",
		},
		{
			name:              "incomplete UNC",
			path:              `\\server`,
			wantKind:          WindowsPathKind_UNC,
			wantPrefix:        option.Some(`\\server`),
			want:              nil,
			wantDOSDeviceName: "",
		},
		{
			name:              "UNC root",
			path:              `\\server\share`,
			wantKind:          WindowsPathKind_UNC,
			wantPrefix:        option.Some(`\\server\share`),
			want:              nil,
			wantDOSDeviceName: "",
		},
		{
			name:              "UNC path",
			path:              `\\server\share\foo`,
			wantKind:          WindowsPathKind_UNC,
			wantPrefix:        option.Some(`\\server\share`),
			want:              []string{"foo"},
			wantDOSDeviceName: "",
		},
		{
			name:              "verbatim drive path",
			path:              `\\?\C:\foo`,
			wantKind:          WindowsPathKind_VerbatimDrive,
			wantPrefix:        option.Some(`\\?\C:`),
			want:              []string{"foo"},
			wantDOSDeviceName: "",
		},
		{
			name:              "verbatim UNC path",
			path:              `\\?\UNC\server\share\foo`,
			wantKind:          WindowsPathKind_VerbatimUNC,
			wantPrefix:        option.Some(`\\?\UNC\server\share`),
			want:              []string{"foo"},
			wantDOSDeviceName: "",
		},
		{
			name:              "verbatim fallback path",
			path:              `\\?\GLOBALROOT\Device\HarddiskVolume1`,
			wantKind:          WindowsPathKind_Verbatim,
			wantPrefix:        option.Some(`\\?\GLOBALROOT`),
			want:              []string{"Device", "HarddiskVolume1"},
			wantDOSDeviceName: "",
		},
		{
			name:              "relative DOS device name",
			path:              `cOm1.. ..`,
			wantKind:          WindowsPathKind_Relative,
			wantPrefix:        option.None[string](),
			want:              []string{`cOm1.. ..`},
			wantDOSDeviceName: "COM1",
		},
		{
			name:              "relative nested NUL",
			path:              `foo\NUL.. ..`,
			wantKind:          WindowsPathKind_Relative,
			wantPrefix:        option.None[string](),
			want:              []string{"foo", "NUL.. .."},
			wantDOSDeviceName: "NUL",
		},
		{
			name:              "explicit device path",
			path:              `\\.\NUL`,
			wantKind:          WindowsPathKind_Device,
			wantPrefix:        option.Some(`\\.\NUL`),
			want:              nil,
			wantDOSDeviceName: "NUL",
		},
	}

	for _, tc := range testCases {
		h.Run(tc.name, func(h check.Harness) {
			h.Parallel()

			gotWindows := ComponentsWindows(tc.path)
			check.AssertSame(h, tc.wantKind, gotWindows.Kind(), "kind")
			wantPrefix, wantPrefixOK := tc.wantPrefix.Get()
			gotPrefix, gotPrefixOK := gotWindows.Prefix().Get()
			check.AssertSame(h, wantPrefixOK, gotPrefixOK, "prefix present")
			if wantPrefixOK {
				check.AssertSame(h, wantPrefix, gotPrefix, "prefix")
			}
			got := iterx.Collect(gotWindows.Components())
			check.AssertSame(h, tc.want, got, "components")
			gotBackward := iterx.Collect(gotWindows.ComponentsBackward())
			slices.Reverse(got)
			check.AssertSame(h, got, gotBackward, "backward components")
			gotDOSDeviceName, ok := gotWindows.SpecialDOSDeviceName().Get()
			if tc.wantDOSDeviceName == "" {
				h.Assertf(!ok, "SpecialDOSDeviceName() = %q, want none", gotDOSDeviceName)
			} else {
				h.Assertf(ok, "SpecialDOSDeviceName() returned none, want %q", tc.wantDOSDeviceName)
				check.AssertSame(h, tc.wantDOSDeviceName, gotDOSDeviceName, "DOS device name")
			}
		})
	}
}
