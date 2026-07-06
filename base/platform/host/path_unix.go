// Copyright 2026 Varun Gandhi
//
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

//go:build unix

package host

import "code.kibou.tools/base/platform/unix"

type pathImpl = unix.Path
type absPathImpl = unix.AbsPath
type relPathImpl = unix.RelPath
type normalizedPathImpl = unix.NormalizedPath
type normalizedAbsPathImpl = unix.NormalizedAbsPath
type normalizedRelPathImpl = unix.NormalizedRelPath

// ParsePathError is the host's path-parsing error type. On Unix it is a
// unix.ParsePathError.
type ParsePathError = unix.ParsePathError
type ParsePathErrorKind = unix.ParsePathErrorKind

// ParsePath parses s into a host Path.
//
// The returned error, if any, has Kind ParsePathErrorKind_Empty.
func ParsePath(s string) (Path, *ParsePathError) {
	p, err := unix.ParsePath(s)
	if err != nil {
		return Path{}, err
	}
	return Path{impl: p}, nil
}

// MustParsePath parses s into a host Path, panicking on failure.
func MustParsePath(s string) Path {
	return Path{impl: unix.MustParsePath(s)}
}

// ParseAbsPath parses s into a host AbsPath.
//
// The returned error, if any, has Kind ParsePathErrorKind_Empty or
// ParsePathErrorKind_NotAbsolute.
func ParseAbsPath(s string) (AbsPath, *ParsePathError) {
	ap, err := unix.ParseAbsPath(s)
	if err != nil {
		return AbsPath{}, err
	}
	return AbsPath{impl: ap}, nil
}

// MustParseAbsPath parses s into a host AbsPath, panicking on failure.
func MustParseAbsPath(s string) AbsPath {
	return AbsPath{impl: unix.MustParseAbsPath(s)}
}
