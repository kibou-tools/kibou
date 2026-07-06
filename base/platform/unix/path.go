// Copyright 2026 Varun Gandhi
//
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package unix

import (
	"fmt"
	"iter"

	"code.kibou.tools/base/assert"
	"code.kibou.tools/base/platform/platform_core"
)

type Path = UnixPath
type AbsPath = UnixAbsPath
type RelPath = UnixRelPath

type NormalizedPath = UnixNormalizedPath
type NormalizedAbsPath = UnixNormalizedAbsPath
type NormalizedRelPath = UnixNormalizedRelPath

// UnixPath represents a path on a Unix system.
type UnixPath struct {
	// Always non-empty.
	text string
}

// LengthLimit is the maximum byte length of a path this package will parse.
const LengthLimit = 4096

// ParsePath parses p into a UnixPath.
//
// The returned error, if any, has Kind ParsePathErrorKind_Empty or
// ParsePathErrorKind_TooLong.
func ParsePath(p string) (UnixPath, *ParsePathError) {
	if len(p) == 0 {
		return UnixPath{}, &ParsePathError{Kind: ParsePathErrorKind_Empty, Input: ""}
	}
	if len(p) > LengthLimit {
		return UnixPath{}, &ParsePathError{Kind: ParsePathErrorKind_TooLong, Input: p}
	}
	return UnixPath{p}, nil
}

// MustParsePath parses p into a UnixPath, panicking if parsing fails.
//
// Precondition: p is non-empty.
func MustParsePath(p string) UnixPath {
	up, err := ParsePath(p)
	if err != nil {
		assert.Preconditionf(false, "invalid path: %v", err)
	}
	return up
}

func (p *UnixPath) Classify() platform_core.PathClassification {
	if p.IsAbsolute() {
		return platform_core.PathClassification_Absolute
	}
	return platform_core.PathClassification_Relative
}

func (p *UnixPath) IsAbsolute() bool {
	return p.text[0] == '/'
}

func (p *UnixPath) IsRelative() bool {
	return p.text[0] != '/'
}

type UnixAbsPath struct {
	raw UnixPath
}

// ParseAbsPath parses p into a UnixAbsPath.
//
// The returned error, if any, has Kind ParsePathErrorKind_Empty,
// ParsePathErrorKind_TooLong, or ParsePathErrorKind_NotAbsolute.
func ParseAbsPath(p string) (UnixAbsPath, *ParsePathError) {
	if len(p) == 0 {
		return UnixAbsPath{}, &ParsePathError{Kind: ParsePathErrorKind_Empty, Input: ""}
	}
	if len(p) > LengthLimit {
		return UnixAbsPath{}, &ParsePathError{Kind: ParsePathErrorKind_TooLong, Input: p}
	}
	if p[0] != '/' {
		return UnixAbsPath{}, &ParsePathError{Kind: ParsePathErrorKind_NotAbsolute, Input: p}
	}
	return UnixAbsPath{UnixPath{p}}, nil
}

// Precondition: p is non-empty and starts with '/'.
func MustParseAbsPath(p string) UnixAbsPath {
	ap, err := ParseAbsPath(p)
	if err != nil {
		assert.Preconditionf(false, "invalid absolute path: %v", err)
	}
	return ap
}

// ParsePathError reports why a string could not be parsed into a path.
//
// The set of possible Kind values depends on the parsing function; each
// function documents which kinds it can return.
type ParsePathError struct {
	Kind ParsePathErrorKind
	// Input is the string that failed to parse. Empty when Kind is
	// ParsePathErrorKind_Empty.
	Input string
}

type ParsePathErrorKind uint8

const (
	// The input path was empty.
	ParsePathErrorKind_Empty ParsePathErrorKind = iota + 1
	// The input path was not absolute (did not start with '/').
	ParsePathErrorKind_NotAbsolute
	// The input path exceeded LengthLimit bytes.
	ParsePathErrorKind_TooLong
)

func (e *ParsePathError) Error() string {
	switch e.Kind {
	case ParsePathErrorKind_Empty:
		return "empty path"
	case ParsePathErrorKind_NotAbsolute:
		return fmt.Sprintf("%q does not start with '/'", e.Input)
	case ParsePathErrorKind_TooLong:
		return fmt.Sprintf("path length %d exceeds limit %d", len(e.Input), LengthLimit)
	default:
		return assert.PanicUnknownCase[string](e.Kind)
	}
}

type UnixRelPath struct {
	raw UnixPath
}

type UnixNormalizedPath struct {
	raw UnixPath
}

type UnixNormalizedAbsPath struct {
	abs UnixAbsPath
}

type UnixNormalizedRelPath struct {
	rel UnixRelPath
}

func ComponentsUnix(path string) iter.Seq[string] {
	return components(path, func(c byte) bool { return c == '/' }, 0)
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
