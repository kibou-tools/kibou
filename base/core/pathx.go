// Copyright 2026 Varun Gandhi
//
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package core

import "code.kibou.tools/base/core/pathx"

// Path types for manipulating host platform paths.
// See the pathx package for details.

type AbsPath = pathx.AbsPath

type RelPath = pathx.RelPath

// MustParseAbsPath creates an AbsPath from an already-absolute path string.
//
// Pre-condition: path is non-empty and absolute per [filepath.IsAbs].
func MustParseAbsPath(path string) AbsPath {
	return pathx.MustParseAbsPath(path)
}

// MustParseRelPath creates a RelPath from a relative path string.
//
// Pre-condition: path is non-empty and not absolute per [filepath.IsAbs].
func MustParseRelPath(path string) RelPath {
	return pathx.MustParseRelPath(path)
}
