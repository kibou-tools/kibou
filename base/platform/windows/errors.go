// Copyright 2026 Varun Gandhi
//
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package windows

import (
	"fmt"

	"code.kibou.tools/base/assert"
	"code.kibou.tools/base/wtf8"
)

// WindowsParsePathError reports why a string could not be parsed into a
// WindowsPath.
type WindowsParsePathError struct {
	Kind WindowsParsePathErrorKind
	// Path is the input that failed to parse. Empty when Kind is
	// WindowsParsePathErrorKind_Empty.
	Path string
	// WTF8Err carries the underlying WTF-8 validation failure.
	//
	// Non-nil if and only if Kind is WindowsParsePathErrorKind_InvalidWTF8.
	WTF8Err *wtf8.TextParseError
}

type WindowsParsePathErrorKind uint8

const (
	// The input path was empty.
	WindowsParsePathErrorKind_Empty WindowsParsePathErrorKind = iota + 1
	// The input path was not valid WTF-8. See WindowsParsePathError.WTF8Err.
	WindowsParsePathErrorKind_InvalidWTF8
	// The input path exceeded LengthLimit bytes.
	WindowsParsePathErrorKind_TooLong
)

func (e *WindowsParsePathError) Error() string {
	switch e.Kind {
	case WindowsParsePathErrorKind_Empty:
		return "empty path"
	case WindowsParsePathErrorKind_InvalidWTF8:
		return fmt.Sprintf("%q is not valid WTF-8: %v", e.Path, e.WTF8Err)
	case WindowsParsePathErrorKind_TooLong:
		return fmt.Sprintf("path length %d exceeds limit %d", len(e.Path), LengthLimit)
	default:
		return assert.PanicUnknownCase[string](e.Kind)
	}
}
