// Copyright 2026 Varun Gandhi
//
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package windows

import (
	"iter"
	"math"
	"strings"

	"code.kibou.tools/base/assert"
	. "code.kibou.tools/base/core/option"
	"code.kibou.tools/base/platform/platform_core"
	"code.kibou.tools/base/wtf8"
)

// --- Aliases for exported use ---

type Path = WindowsPath

// PathKind represents the kinds of paths for Windows.
//
// Unlike Unix systems, there are a large variety of path types on Windows.
// One notable distinction is that while on a Unix system, a path can be
// absolute xor relative, on Windows, the resolution of a path can also depend
// on other global state. For example, it can depend on the current drive,
// or the drive-specific working directory.
type PathKind = WindowsPathKind

type AbsPath = WindowsAbsPath
type RelPath = WindowsRelPath

type NormalizedPath = WindowsNormalizedPath
type NormalizedAbsPath = WindowsNormalizedAbsPath
type NormalizedRelPath = WindowsNormalizedRelPath

type WindowsPath struct {
	path wtf8.Text
	// prefixEnd delimits the Windows path prefix, if any.
	//
	// The prefix applies for the following WindowsPathKind cases:
	//
	// - PathKind_DriveRelative
	// - PathKind_DriveAbsolute
	// - PathKind_UNC
	// - PathKind_VerbatimDrive
	// - PathKind_VerbatimUNC
	// - PathKind_Verbatim
	// - PathKind_Device
	// - PathKind_RootLocalDevice
	//
	// It does not apply to PathKind_Relative or PathKind_RootRelative.
	prefixEnd uint16
	kind      WindowsPathKind
}

const LengthLimit = math.MaxUint16

// WindowsPathKind defines the kinds of path that are possible on Windows.
//
// The cases for this type are modeled after the return values of Windows's
// native path-classification function `RtlDetermineDosPathNameType_U` as well
// as the Rust standard library's `std::path::Prefix` type.
type WindowsPathKind uint8

const (
	// A relative path of the form `abc\xyz\blah.txt`.
	//
	// This is similar to what's typically called a
	// "relative path" on Unix systems.
	PathKind_Relative WindowsPathKind = iota + 1
	// A root-relative path of the form `\abc\xyz.txt`.
	//
	// Additionally, just '\' is also classified as root-relative.
	//
	// Resolved relative to the current drive, which is
	// based on the current working directory.
	PathKind_RootRelative
	// A drive-relative path of the form `C:abc\xyz.txt`.
	//
	// If the current working directory for the drive is set
	// in the environment, then this path is interpreted
	// relative to that. For example,
	//
	// ```batch
	// set "=C:=C:\Users\me"
	// cd /d D:\work
	// type C:file.txt
	// ```
	//
	// - Line 1 sets the hidden environment variable.
	// - Line 2 changes the working directory
	//   (`/d` => "allow me to change the drive")
	//   to `D:\work`.
	// - Line 3 prints the `C:\Users\me\file.txt` to the
	//   console.
	//
	// Otherwise, interpreted relative to the root of the
	// drive (equivalent to `C:\abc\xyz.txt`).
	//
	// Historical context from [What are these strange =C: environment variables?](https://devblogs.microsoft.com/oldnewthing/20100506-00/?p=14133)
	//
	// > Win32 does not have the concept of a separate current
	// > directory for each drive, but the command processor
	// > wanted to preserve the old MS-DOS behavior because
	// > people were accustomed to it (and batch files relied upon it).
	// > The solution was to store this “per-drive current directory”
	// > in the environment, using a weird-o environment variable name
	// > so it wouldn’t conflict with normal environment variables.
	PathKind_DriveRelative
	// A drive-absolute path like `C:\abc\xyz.txt`.
	//
	// This is similar to what's typically called an absolute
	// path on Unix systems. Interpreting it doesn't require access
	// to any ambient state.
	PathKind_DriveAbsolute
	// A UNC path like `\\server\share\abc\xyz.txt`.
	//
	// UNC paths identify a resource by server and share name.
	// Here, the prefix is `\\server\share`, and the components after it
	// are the ordinary path components.
	PathKind_UNC
	// A verbatim drive path like `\\?\C:\abc\xyz.txt`.
	//
	// Verbatim paths bypass parts of Win32 path normalization
	// and are commonly used to avoid legacy path-length limits
	// (NNN on Windows) or to preserve path spelling more exactly.
	//
	// Here, `\\?\C:` is the prefix.
	//
	// NOTE: Depending on the source, such a path may be called an
	// "extended length path" or a "DOS device path".
	PathKind_VerbatimDrive
	// A verbatim UNC path like `\\?\UNC\server\share\abc\xyz.txt`.
	//
	// This is like a verbatim drive path, but for a network share.
	//
	// Note that the 'UNC' is literal; it can't be replaced
	// with some other string.
	//
	// Here, `\\?\UNC\server\share` is the prefix.
	PathKind_VerbatimUNC
	// A verbatim path like `\\?\GLOBALROOT\Device\HarddiskVolume1`.
	//
	// This is the fallback form for verbatim paths that are not verbatim
	// drive paths or verbatim UNC paths. It corresponds to Rust's
	// `Prefix::Verbatim`.
	PathKind_Verbatim
	// A Win32 device namespace path of the form
	// `\\.\COM1` or `\\.\PIPE\name`.
	//
	// The prefix includes the first component after `\\.\`. For example,
	// the prefix of `\\.\PIPE\name` is `\\.\PIPE`.
	PathKind_Device
	// Only used for the exact paths `\\.` and `\\?`. By themselves, these
	// paths are not particularly useful, and should be avoided.
	PathKind_RootLocalDevice
)

func (p *WindowsPath) Classify() platform_core.PathClassification {
	return p.kind.Classify()
}

func (pk WindowsPathKind) Classify() platform_core.PathClassification {
	switch pk {
	case PathKind_Relative:
		return platform_core.PathClassification_Relative
	case PathKind_RootRelative, PathKind_DriveRelative:
		return platform_core.PathClassification_Neither
	case PathKind_DriveAbsolute,
		PathKind_UNC, PathKind_VerbatimDrive,
		PathKind_VerbatimUNC, PathKind_Verbatim,
		PathKind_Device, PathKind_RootLocalDevice:
		return platform_core.PathClassification_Absolute
	default:
		return assert.PanicUnknownCase[platform_core.PathClassification](pk)
	}
}

type WindowsAbsPath struct {
	raw WindowsPath
}

type WindowsRelPath struct {
	raw WindowsPath
}

type WindowsNormalizedPath struct {
	raw WindowsPath
}

type WindowsNormalizedAbsPath struct {
	abs WindowsAbsPath
}

type WindowsNormalizedRelPath struct {
	rel WindowsRelPath
}

func IsPathSeparator(c byte) bool {
	return c == '\\' || c == '/'
}

// SpecialDOSDeviceNames returns the fixed list of special DOS device names
// recognized by Windows 11.
//
// Prefix-less paths can be surprising. For example, plain `NUL` is interpreted
// as `\\.\NUL`. See WindowsPath.SpecialDOSDeviceName for the lexical subset of
// those rules modeled by this package.
func SpecialDOSDeviceNames() []string {
	return []string{
		"AUX",
		"CON",
		"CONIN$",
		"CONOUT$",
		"COM1",
		"COM2",
		"COM3",
		"COM4",
		"COM5",
		"COM6",
		"COM7",
		"COM8",
		"COM9",
		"COM²",
		"COM³",
		"COM¹",
		"LPT1",
		"LPT2",
		"LPT3",
		"LPT4",
		"LPT5",
		"LPT6",
		"LPT7",
		"LPT8",
		"LPT9",
		"LPT²",
		"LPT³",
		"LPT¹",
		"NUL",
		"PRN",
	}
}

func ParsePath(path string) (WindowsPath, *WindowsParsePathError) {
	if len(path) == 0 {
		return WindowsPath{}, &WindowsParsePathError{
			Kind: WindowsParsePathErrorKind_Empty,
			Path: "",
		}
	}
	if len(path) > LengthLimit {
		return WindowsPath{}, &WindowsParsePathError{
			Kind: WindowsParsePathErrorKind_TooLong,
			Path: path,
		}
	}
	t, err := wtf8.ParseText(path)
	if err != nil {
		return WindowsPath{}, &WindowsParsePathError{
			Kind:    WindowsParsePathErrorKind_InvalidWTF8,
			Path:    path,
			WTF8Err: err,
		}
	}
	return ParsePathFromWTF8(t)
}

// ParsePathFromWTF8 classifies an already-validated WTF-8 path.
func ParsePathFromWTF8(path wtf8.Text) (WindowsPath, *WindowsParsePathError) {
	p := path.String()
	if len(p) == 0 {
		return WindowsPath{}, &WindowsParsePathError{
			Kind: WindowsParsePathErrorKind_Empty,
			Path: "",
		}
	}
	if len(p) >= 2 && p[1] == ':' && isWindowsDriveLetter(p[0]) {
		if len(p) >= 3 && IsPathSeparator(p[3]) {
			return WindowsPath{path, 2, PathKind_DriveAbsolute}, nil
		}
		// C: -> DriveRelative to match RtlDetermineDosPathNameType_U
		return WindowsPath{path, 2, PathKind_DriveRelative}, nil
	}
	if IsPathSeparator(p[0]) { // OK as len(p) > 0
		if len(p) >= 2 && IsPathSeparator(p[1]) {
			if len(p) >= 3 && (p[2] == '.' || p[2] == '?') {
				if len(p) == 3 {
					return WindowsPath{path, 3, PathKind_RootLocalDevice}, nil
				}
				if IsPathSeparator(p[3]) { // len(p) >= 4 here already
					if p[2] == '.' {
						return WindowsPath{path, 3, PathKind_Device}, nil
					}
					return WindowsPath{path, 3, PathKind_Verbatim}, nil
				}
			}

		}
		// NOTE: Just '\' is also root-relative to match RtlDetermineDosPathNameType_U
		return WindowsPath{path, 1, PathKind_RootRelative}, nil
	}
	return WindowsPath{path, 0, PathKind_Relative}, nil
}

// Precondition: path must be non-empty.
func ComponentsWindows(path string) WindowsPath {
	if len(path) == 0 {
		assert.Preconditionf(false, "ComponentsWindows called with an empty path")
	}
	kind, prefixStart, prefixEnd, componentsStart := parseWindowsPathPrefix(path)
	return WindowsPath{
		path:            path,
		prefixStart:     prefixStart,
		prefixEnd:       prefixEnd,
		componentsStart: componentsStart,
		kind:            kind,
	}
}

func (c WindowsPath) Kind() WindowsPathKind {
	return c.kind
}

// Returns a non-empty prefix string for the following path kinds:
//
// - PathKind_DriveRelative
// - PathKind_DriveAbsolute
// - PathKind_UNC
// - PathKind_VerbatimDrive
// - PathKind_VerbatimUNC
// - PathKind_Verbatim
// - PathKind_Device
// - PathKind_RootLocalDevice
//
// Returns None for PathKind_Relative and PathKind_RootRelative
func (c WindowsPath) Prefix() Option[string] {
	if c.prefixStart == c.prefixEnd {
		return None[string]()
	}
	return Some(c.path[c.prefixStart:c.prefixEnd])
}

func (c WindowsPath) Components() iter.Seq[string] {
	return components(c.path, int(c.componentsStart))
}

func (c WindowsPath) ComponentsBackward() iter.Seq[string] {
	return componentsBackward(c.path, int(c.componentsStart))
}

// SpecialDOSDeviceName reports whether this path lexically names a special
// DOS device under the Windows 11 rules described by Chris Denton.
//
// Prefix-less paths are surprising. Plain `NUL` is interpreted as `\\.\NUL`.
// More generally, if a path matches the fixed list of DOS device names after
// the following transformations, it is treated as a special device name:
//
//  1. ASCII letters are uppercased.
//  2. Trailing dots (`.`) and spaces (` `) are removed.
//
// For example, `cOm1.. ..` is interpreted as `\\.\COM1`, but `.\COM1` is
// not. `NUL` has one additional rule: if the final component of a relative path
// or drive-absolute path matches `NUL` after those transformations, Windows
// may interpret the path as `\\.\NUL`. That resolution also depends on whether
// the parent directory exists, so this lexical parser can only report the
// potential special-device match.
func (c WindowsPath) SpecialDOSDeviceName() Option[string] {
	if c.kind == PathKind_Device {
		prefix := c.Prefix().Unwrap()
		const devicePrefixLen = len(`\\.\`)
		return NewOption(canonicalSpecialDOSDeviceName(prefix[devicePrefixLen:]))
	}

	var lastComponent string
	componentCount := 0
	for component := range c.Components() {
		lastComponent = component
		componentCount++
	}
	if componentCount == 0 {
		return None[string]()
	}

	if c.kind == PathKind_Relative && componentCount == 1 {
		return NewOption(canonicalSpecialDOSDeviceName(lastComponent))
	}
	if c.kind == PathKind_Relative || c.kind == PathKind_DriveAbsolute {
		name, ok := canonicalSpecialDOSDeviceName(lastComponent)
		if ok && name == "NUL" {
			return Some(name)
		}
	}
	return None[string]()
}

func canonicalSpecialDOSDeviceName(name string) (string, bool) {
	name = strings.TrimRight(name, ". ")
	name = strings.Map(func(r rune) rune {
		if 'a' <= r && r <= 'z' {
			return r - ('a' - 'A')
		}
		return r
	}, name)
	for _, deviceName := range SpecialDOSDeviceNames() {
		if name == deviceName {
			return deviceName, true
		}
	}
	return "", false
}

func components(path string, start int) iter.Seq[string] {
	return func(yield func(string) bool) {
		for i := start; i <= len(path); i++ {
			if i < len(path) && !IsPathSeparator(path[i]) {
				continue
			}
			if start < i {
				if !yield(path[start:i]) {
					return
				}
			}
			start = i + 1
		}
	}
}

func componentsBackward(path string, start int) iter.Seq[string] {
	return func(yield func(string) bool) {
		end := len(path)
		for i := len(path) - 1; i >= start; i-- {
			if !IsPathSeparator(path[i]) {
				continue
			}
			if i+1 < end {
				if !yield(path[i+1 : end]) {
					return
				}
			}
			end = i
		}
		if start < end {
			yield(path[start:end])
		}
	}
}

func parseWindowsPathPrefix(path string) (WindowsPathKind, uint16, uint16, uint16) {
	if len(path) >= 2 && isWindowsDriveLetter(path[0]) && path[1] == ':' {
		if len(path) > 2 && IsPathSeparator(path[2]) {
			return PathKind_DriveAbsolute, 0, 2, 3
		}
		return PathKind_DriveRelative, 0, 2, 2
	}
	if len(path) == 0 || !IsPathSeparator(path[0]) {
		return PathKind_Relative, 0, 0, 0
	}
	if len(path) == 1 || !IsPathSeparator(path[1]) {
		return PathKind_RootRelative, 0, 0, 1
	}

	// OK, now we have two path separators at the start.
	const devicePrefixLen = len(`\\?\`)
	if len(path) >= devicePrefixLen && (path[2] == '?' || path[2] == '.') && IsPathSeparator(path[3]) {
		const verbatimUNCPrefixLen = len(`\\?\UNC\`)
		if path[2] == '?' && (len(path) >= verbatimUNCPrefixLen &&
			path[4] == 'U' && path[5] == 'N' && path[6] == 'C' &&
			IsPathSeparator(path[7])) {
			prefixEnd, start := windowsUNCPrefixEnd(path, len(`\\?\UNC\`))
			return PathKind_VerbatimUNC, 0, uint16(prefixEnd), uint16(start)
		}
		const prefixLen = devicePrefixLen + len(`C:`)
		if path[2] == '?' && (len(path) >= prefixLen &&
			isWindowsDriveLetter(path[devicePrefixLen]) && path[devicePrefixLen+1] == ':' &&
			(len(path) == prefixLen || IsPathSeparator(path[prefixLen]))) {
			start := devicePrefixLen + 2
			if len(path) > start && IsPathSeparator(path[start]) {
				start++
			}
			return PathKind_VerbatimDrive, 0, uint16(devicePrefixLen + 2), uint16(start)
		}
		prefixEnd, start := windowsPrefixEnd(path, devicePrefixLen)
		if path[2] == '?' {
			return PathKind_Verbatim, 0, uint16(prefixEnd), uint16(start)
		}
		return PathKind_Device, 0, uint16(prefixEnd), uint16(start)
	}
	prefixEnd, start := windowsUNCPrefixEnd(path, 2)
	return PathKind_UNC, 0, uint16(prefixEnd), uint16(start)
}

func windowsPrefixEnd(path string, start int) (int, int) {
	end, ok := windowsNextComponentEnd(path, start)
	if !ok {
		return len(path), len(path)
	}
	if end < len(path) && IsPathSeparator(path[end]) {
		return end, end + 1
	}
	return end, end
}

func windowsUNCPrefixEnd(path string, start int) (int, int) {
	firstEnd, ok := windowsNextComponentEnd(path, start)
	if !ok {
		return len(path), len(path)
	}
	shareStart := firstEnd + 1
	shareEnd, ok := windowsNextComponentEnd(path, shareStart)
	if !ok {
		return len(path), len(path)
	}
	if shareEnd < len(path) && IsPathSeparator(path[shareEnd]) {
		return shareEnd, shareEnd + 1
	}
	return shareEnd, shareEnd
}

func windowsNextComponentEnd(path string, start int) (int, bool) {
	if start >= len(path) || IsPathSeparator(path[start]) {
		return start, false
	}
	for i := start; i < len(path); i++ {
		if IsPathSeparator(path[i]) {
			return i, true
		}
	}
	return len(path), true
}

// See NOTE(id: windows-drive-letters) in docs/external/windows.md.
func isWindowsDriveLetter(c byte) bool {
	return ('a' <= c && c <= 'z') || ('A' <= c && c <= 'Z')
}
