// Copyright 2026 Varun Gandhi
//
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

// Package pathx provides filepath utilities.
package pathx

import (
	"iter"
	"path/filepath"
	"runtime"
	"strings"

	"code.kibou.tools/base/assert"
	"code.kibou.tools/base/core/option"
)

// LexicallyContains reports whether child is lexically contained within root.
// Both paths are cleaned before comparison.
func LexicallyContains(root, child string) bool {
	rel, err := filepath.Rel(root, filepath.Join(root, child))
	return err == nil && !strings.HasPrefix(rel, "..")
}

func IsPathSeparator(c byte) bool {
	if runtime.GOOS == "windows" {
		return IsWindowsPathSeparator(c)
	}
	return c == filepath.Separator
}

func IsWindowsPathSeparator(c byte) bool {
	return c == '\\' || c == '/'
}

type PathKind uint8

const (
	PathKind_Unix PathKind = iota + 1
	PathKind_Windows
)

type PathComponents struct {
	path string
	// windows is set for PathKind_Windows.
	windows WindowsPath
	kind    PathKind
}

func Components(path string) PathComponents {
	if runtime.GOOS == "windows" {
		return PathComponents{
			path:    path,
			windows: ComponentsWindows(path),
			kind:    PathKind_Windows,
		}
	}
	return PathComponents{
		path: path,
		windows: WindowsPath{
			path:            "",
			prefixStart:     0,
			prefixEnd:       0,
			componentsStart: 0,
			kind:            0,
		},
		kind: PathKind_Unix,
	}
}

func (c PathComponents) Kind() PathKind {
	return c.kind
}

func (c PathComponents) Components() iter.Seq[string] {
	switch c.kind {
	case PathKind_Unix:
		return ComponentsUnix(c.path)
	case PathKind_Windows:
		return c.windows.Components()
	default:
		return assert.PanicUnknownCase[iter.Seq[string]](c.kind)
	}
}

func (c PathComponents) Windows() WindowsPath {
	switch c.kind {
	case PathKind_Windows:
		return c.windows
	case PathKind_Unix:
		assert.Preconditionf(false, "Windows() called on PathComponents kind %v", c.kind)
		return WindowsPath{path: "", prefixStart: 0, prefixEnd: 0, componentsStart: 0, kind: 0}
	default:
		return assert.PanicUnknownCase[WindowsPath](c.kind)
	}
}

func ComponentsUnix(path string) iter.Seq[string] {
	return components(path, func(c byte) bool { return c == '/' }, 0)
}

type WindowsPathKind uint8

const (
	// A relative path of the form `abc\xyz\blah.txt`.
	//
	// This is similar to what's typically called a
	// "relative path" on Unix systems.
	WindowsPathKind_Relative WindowsPathKind = iota + 1
	// A root-relative path of the form `\abc\xyz.txt`.
	//
	// Resolved relative to the current drive, which is
	// based on the current working directory.
	WindowsPathKind_RootRelative
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
	WindowsPathKind_DriveRelative
	// A drive-absolute path like `C:\abc\xyz.txt`.
	//
	// This is similar to what's typically called an absolute
	// path on Unix systems. Interpreting it doesn't require access
	// to any ambient state.
	WindowsPathKind_DriveAbsolute
	// A UNC path like `\\server\share\abc\xyz.txt`.
	//
	// UNC paths identify a resource by server and share name.
	// Here, the prefix is `\\server\share`, and the components after it
	// are the ordinary path components.
	WindowsPathKind_UNC
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
	WindowsPathKind_VerbatimDrive
	// A verbatim UNC path like `\\?\UNC\server\share\abc\xyz.txt`.
	//
	// This is like a verbatim drive path, but for a network share.
	//
	// Note that the 'UNC' is literal; it can't be replaced
	// with some other string.
	//
	// Here, `\\?\UNC\server\share` is the prefix.
	WindowsPathKind_VerbatimUNC
	// A verbatim path like `\\?\GLOBALROOT\Device\HarddiskVolume1`.
	//
	// This is the fallback form for verbatim paths that are not verbatim
	// drive paths or verbatim UNC paths. It corresponds to Rust's
	// `Prefix::Verbatim`.
	WindowsPathKind_Verbatim
	// A Win32 device namespace path of the form
	// `\\.\COM1` or `\\.\PIPE\name`.
	//
	// The prefix includes the first component after `\\.\`. For example,
	// the prefix of `\\.\PIPE\name` is `\\.\PIPE`.
	WindowsPathKind_Device
)

// Windows11DeviceNames returns the fixed list of special DOS device names
// recognized by Windows 11.
//
// Prefix-less paths can be surprising. For example, plain `NUL` is interpreted
// as `\\.\NUL`. See WindowsPath.SpecialDOSDeviceName for the lexical subset of
// those rules modeled by this package.
func Windows11DeviceNames() []string {
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

type WindowsPath struct {
	path string
	// prefixStart and prefixEnd delimit the Windows path prefix, if any.
	prefixStart     uint32
	prefixEnd       uint32
	componentsStart uint32
	kind            WindowsPathKind
}

func ComponentsWindows(path string) WindowsPath {
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

func (c WindowsPath) Prefix() option.Option[string] {
	if c.prefixStart == c.prefixEnd {
		return option.None[string]()
	}
	return option.Some(c.path[c.prefixStart:c.prefixEnd])
}

func (c WindowsPath) Components() iter.Seq[string] {
	return components(c.path, IsWindowsPathSeparator, int(c.componentsStart))
}

func (c WindowsPath) ComponentsBackward() iter.Seq[string] {
	return componentsBackward(c.path, IsWindowsPathSeparator, int(c.componentsStart))
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
func (c WindowsPath) SpecialDOSDeviceName() option.Option[string] {
	if c.kind == WindowsPathKind_Device {
		prefix := c.Prefix().Unwrap()
		const devicePrefixLen = len(`\\.\`)
		return option.NewOption(canonicalSpecialDOSDeviceName(prefix[devicePrefixLen:]))
	}

	var lastComponent string
	componentCount := 0
	for component := range c.Components() {
		lastComponent = component
		componentCount++
	}
	if componentCount == 0 {
		return option.None[string]()
	}

	if c.kind == WindowsPathKind_Relative && componentCount == 1 {
		return option.NewOption(canonicalSpecialDOSDeviceName(lastComponent))
	}
	if c.kind == WindowsPathKind_Relative || c.kind == WindowsPathKind_DriveAbsolute {
		name, ok := canonicalSpecialDOSDeviceName(lastComponent)
		if ok && name == "NUL" {
			return option.Some(name)
		}
	}
	return option.None[string]()
}

func canonicalSpecialDOSDeviceName(name string) (string, bool) {
	name = strings.TrimRight(name, ". ")
	name = strings.Map(func(r rune) rune {
		if 'a' <= r && r <= 'z' {
			return r - ('a' - 'A')
		}
		return r
	}, name)
	for _, deviceName := range Windows11DeviceNames() {
		if name == deviceName {
			return deviceName, true
		}
	}
	return "", false
}

func components(path string, isPathSeparator func(byte) bool, start int) iter.Seq[string] {
	return func(yield func(string) bool) {
		for i := start; i <= len(path); i++ {
			if i < len(path) && !isPathSeparator(path[i]) {
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

func componentsBackward(path string, isPathSeparator func(byte) bool, start int) iter.Seq[string] {
	return func(yield func(string) bool) {
		end := len(path)
		for i := len(path) - 1; i >= start; i-- {
			if !isPathSeparator(path[i]) {
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

func parseWindowsPathPrefix(path string) (WindowsPathKind, uint32, uint32, uint32) {
	if len(path) >= 2 && isWindowsDriveLetter(path[0]) && path[1] == ':' {
		if len(path) > 2 && IsWindowsPathSeparator(path[2]) {
			return WindowsPathKind_DriveAbsolute, 0, 2, 3
		}
		return WindowsPathKind_DriveRelative, 0, 2, 2
	}
	if len(path) == 0 || !IsWindowsPathSeparator(path[0]) {
		return WindowsPathKind_Relative, 0, 0, 0
	}
	if len(path) == 1 || !IsWindowsPathSeparator(path[1]) {
		return WindowsPathKind_RootRelative, 0, 0, 1
	}
	return parseWindowsDoubleSeparatorPathPrefix(path)
}

// Pre-condition: path starts with two Windows path separators.
func parseWindowsDoubleSeparatorPathPrefix(path string) (WindowsPathKind, uint32, uint32, uint32) {
	const devicePrefixLen = len(`\\?\`)
	if len(path) >= devicePrefixLen && (path[2] == '?' || path[2] == '.') && IsWindowsPathSeparator(path[3]) {
		if path[2] == '?' && hasWindowsVerbatimUNCPrefix(path) {
			prefixEnd, start := windowsUNCPrefixEnd(path, len(`\\?\UNC\`))
			return WindowsPathKind_VerbatimUNC, 0, uint32(prefixEnd), uint32(start)
		}
		if path[2] == '?' && hasWindowsVerbatimDrivePrefix(path) {
			start := devicePrefixLen + 2
			if len(path) > start && IsWindowsPathSeparator(path[start]) {
				start++
			}
			return WindowsPathKind_VerbatimDrive, 0, uint32(devicePrefixLen + 2), uint32(start)
		}
		prefixEnd, start := windowsPrefixEnd(path, devicePrefixLen)
		if path[2] == '?' {
			return WindowsPathKind_Verbatim, 0, uint32(prefixEnd), uint32(start)
		}
		return WindowsPathKind_Device, 0, uint32(prefixEnd), uint32(start)
	}
	prefixEnd, start := windowsUNCPrefixEnd(path, 2)
	return WindowsPathKind_UNC, 0, uint32(prefixEnd), uint32(start)
}

func hasWindowsVerbatimUNCPrefix(path string) bool {
	const verbatimUNCPrefixLen = len(`\\?\UNC\`)
	return len(path) >= verbatimUNCPrefixLen &&
		path[4] == 'U' && path[5] == 'N' && path[6] == 'C' &&
		IsWindowsPathSeparator(path[7])
}

func hasWindowsVerbatimDrivePrefix(path string) bool {
	const devicePrefixLen = len(`\\?\`)
	const prefixLen = devicePrefixLen + len(`C:`)
	return len(path) >= prefixLen &&
		isWindowsDriveLetter(path[devicePrefixLen]) && path[devicePrefixLen+1] == ':' &&
		(len(path) == prefixLen || IsWindowsPathSeparator(path[prefixLen]))
}

func windowsPrefixEnd(path string, start int) (int, int) {
	end, ok := windowsNextComponentEnd(path, start)
	if !ok {
		return len(path), len(path)
	}
	if end < len(path) && IsWindowsPathSeparator(path[end]) {
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
	if shareEnd < len(path) && IsWindowsPathSeparator(path[shareEnd]) {
		return shareEnd, shareEnd + 1
	}
	return shareEnd, shareEnd
}

func windowsNextComponentEnd(path string, start int) (int, bool) {
	if start >= len(path) || IsWindowsPathSeparator(path[start]) {
		return start, false
	}
	for i := start; i < len(path); i++ {
		if IsWindowsPathSeparator(path[i]) {
			return i, true
		}
	}
	return len(path), true
}

// See NOTE(id: windows-drive-letters) in docs/external/windows.md.
func isWindowsDriveLetter(c byte) bool {
	return ('a' <= c && c <= 'z') || ('A' <= c && c <= 'Z')
}
