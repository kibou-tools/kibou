// Copyright 2026 Varun Gandhi
//
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package fsx_name

import (
	"fmt"

	"code.kibou.tools/base/assert"
	internal_pathx "code.kibou.tools/base/internal/pathx"
)

// ExtractBaseName extracts the final concrete path component as a [Name].
func ExtractBaseName(path string) (Name, *BaseNameError) {
	if path == "" {
		return Name{}, &BaseNameError{kind: BaseNameErrorKind_EmptyString, path: path}
	}
	if internal_pathx.IsPathSeparator(path[len(path)-1]) {
		return Name{}, &BaseNameError{kind: BaseNameErrorKind_EndsWithPathSeparator, path: path}
	}

	var base string
	for component := range internal_pathx.Components(path).Components() {
		base = component
	}
	if base == "." || base == ".." || base == "" {
		return Name{}, &BaseNameError{kind: BaseNameErrorKind_NoBaseName, path: path}
	}

	return Name{base}, nil
}

// MustExtractBaseName extracts the final concrete path component from path as a
// [Name].
//
// Pre-condition: path has a final concrete path component.
func MustExtractBaseName(path string) Name {
	name, err := ExtractBaseName(path)
	if err != nil {
		assert.Precondition(false, err.Error())
	}
	return name
}

// BaseNameErrorKind classifies a [BaseNameError].
type BaseNameErrorKind uint8

const (
	// BaseNameErrorKind_EmptyString indicates that the input path was empty.
	BaseNameErrorKind_EmptyString BaseNameErrorKind = iota + 1
	// BaseNameErrorKind_EndsWithPathSeparator indicates that the input path ends
	// with a path separator.
	BaseNameErrorKind_EndsWithPathSeparator
	// BaseNameErrorKind_NoBaseName indicates that the input path does not have a
	// concrete final path component, such as "." or "..".
	BaseNameErrorKind_NoBaseName
)

// BaseNameError is returned by [ExtractBaseName].
type BaseNameError struct {
	kind BaseNameErrorKind
	// path is the input path passed to [ExtractBaseName]. It is empty for
	// [BaseNameErrorKind_EmptyString].
	path string
}

// Kind returns the error kind.
func (e *BaseNameError) Kind() BaseNameErrorKind {
	return e.kind
}

// Path returns the input path passed to [ExtractBaseName]. It is empty for
// [BaseNameErrorKind_EmptyString].
func (e *BaseNameError) Path() string {
	return e.path
}

func (e *BaseNameError) Error() string {
	switch e.kind {
	case BaseNameErrorKind_EmptyString:
		return "cannot extract base name from empty path string"
	case BaseNameErrorKind_EndsWithPathSeparator:
		return fmt.Sprintf("path %q ends with a path separator; cannot extract base name", e.path)
	case BaseNameErrorKind_NoBaseName:
		return fmt.Sprintf("path %q has no concrete base name", e.path)
	default:
		return assert.PanicUnknownCase[string](e.kind)
	}
}
