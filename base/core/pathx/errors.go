// Copyright 2026 Varun Gandhi
//
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package pathx

import (
	"code.kibou.tools/base/assert"
	"code.kibou.tools/base/errorx"
)

type AbsPathParseError struct {
	kind      AbsPathParseErrorKind
	inputPath string
}

type AbsPathParseErrorKind uint8

const (
	AbsPathParseErrorKind_Empty AbsPathParseErrorKind = iota + 1
	AbsPathParseErrorKind_NotAbsolute
)

func NewAbsPathParseError(kind AbsPathParseErrorKind, inputPath string) *AbsPathParseError {
	switch kind {
	case AbsPathParseErrorKind_Empty:
		assert.Preconditionf(inputPath == "", "input path should be empty for AbsPathParseErrorKind_Empty")
	case AbsPathParseErrorKind_NotAbsolute:
		assert.Preconditionf(inputPath != "", "input path should be non-empty for AbsPathParseErrorKind_NotAbsolute")
	default:
		assert.PanicUnknownCase[any](kind)
	}
	return &AbsPathParseError{kind: kind, inputPath: inputPath}
}

func (e *AbsPathParseError) Kind() AbsPathParseErrorKind {
	return e.kind
}

func (e *AbsPathParseError) InputPath() string {
	return e.inputPath
}

func (e *AbsPathParseError) Error() string {
	fmt := errorx.NewStableFormatter()
	e.FormatError(fmt)
	return fmt.Finish()
}

func (e *AbsPathParseError) FormatError(fmt errorx.Formatter) {
	switch e.kind {
	case AbsPathParseErrorKind_Empty:
		fmt.FormatConstMsg("empty absolute path")
	case AbsPathParseErrorKind_NotAbsolute:
		fmt.FormatConstMsg("not an absolute path")
		fmt.FormatDynamic(errorx.ValueKind_Path, "input", e.inputPath)
	default:
		assert.PanicUnknownCase[any](e.kind)
	}
}

type RelPathParseError struct {
	kind      RelPathParseErrorKind
	inputPath string
}

type RelPathParseErrorKind uint8

const (
	RelPathParseErrorKind_Empty RelPathParseErrorKind = iota + 1
	RelPathParseErrorKind_NotRelative
)

func NewRelPathParseError(kind RelPathParseErrorKind, inputPath string) *RelPathParseError {
	switch kind {
	case RelPathParseErrorKind_Empty:
		assert.Preconditionf(inputPath == "", "input path should be empty for RelPathParseErrorKind_Empty")
	case RelPathParseErrorKind_NotRelative:
		assert.Preconditionf(inputPath != "", "input path should be non-empty for RelPathParseErrorKind_NotRelative")
	default:
		assert.PanicUnknownCase[any](kind)
	}
	return &RelPathParseError{kind: kind, inputPath: inputPath}
}

func (e *RelPathParseError) Kind() RelPathParseErrorKind {
	return e.kind
}

func (e *RelPathParseError) InputPath() string {
	return e.inputPath
}

func (e *RelPathParseError) Error() string {
	fmt := errorx.NewStableFormatter()
	e.FormatError(fmt)
	return fmt.Finish()
}

func (e *RelPathParseError) FormatError(fmt errorx.Formatter) {
	switch e.kind {
	case RelPathParseErrorKind_Empty:
		fmt.FormatConstMsg("empty relative path")
	case RelPathParseErrorKind_NotRelative:
		fmt.FormatConstMsg("not a relative path")
		fmt.FormatDynamic(errorx.ValueKind_Path, "input", e.inputPath)
	default:
		assert.PanicUnknownCase[any](e.kind)
	}
}
