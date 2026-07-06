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
	"code.kibou.tools/base/platform/unix"
	"code.kibou.tools/base/platform/windows"
)

// LexicallyContains reports whether child is lexically contained within root.
// Both paths are cleaned before comparison.
func LexicallyContains(root, child string) bool {
	rel, err := filepath.Rel(root, filepath.Join(root, child))
	return err == nil && !strings.HasPrefix(rel, "..")
}

func IsPathSeparator(c byte) bool {
	if runtime.GOOS == "windows" {
		return windows.IsPathSeparator(c)
	}
	return c == filepath.Separator
}

type PathKind uint8

const (
	PathKind_Unix PathKind = iota + 1
	PathKind_Windows
)

type PathComponents struct {
	path string
	// windows is set for PathKind_Windows.
	windows windows.WindowsPath
	kind    PathKind
}

func Components(path string) PathComponents {
	if runtime.GOOS == "windows" {
		return PathComponents{
			path:    path,
			windows: windows.ComponentsWindows(path),
			kind:    PathKind_Windows,
		}
	}
	return PathComponents{
		path:    path,
		windows: windows.WindowsPath{},
		kind:    PathKind_Unix,
	}
}

func (c PathComponents) Kind() PathKind {
	return c.kind
}

func (c PathComponents) Components() iter.Seq[string] {
	switch c.kind {
	case PathKind_Unix:
		return unix.ComponentsUnix(c.path)
	case PathKind_Windows:
		return c.windows.Components()
	default:
		return assert.PanicUnknownCase[iter.Seq[string]](c.kind)
	}
}

func (c PathComponents) Windows() windows.WindowsPath {
	switch c.kind {
	case PathKind_Windows:
		return c.windows
	case PathKind_Unix:
		assert.Preconditionf(false, "Windows() called on PathComponents kind %v", c.kind)
		return windows.WindowsPath{}
	default:
		return assert.PanicUnknownCase[windows.WindowsPath](c.kind)
	}
}
