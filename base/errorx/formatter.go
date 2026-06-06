// Copyright 2026 Varun Gandhi
//
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package errorx

// Precondition for all methods: key should be a constant string,
// not a dynamically generated string.
type Formatter interface {
	// Requirement: If the Formatter only supports a limited set of ValueKinds,
	// FormatValue should assert that the kind is one of the supported ValueKinds
	// instead of silently picking some default (redaction/hashing/exposure).
	FormatDynamic(kind ValueKind, key string, value string)
	// Requirement: FormatBool must not panic.
	FormatBool(key string, value bool)
	// Requirement: FormatUint64 must not panic.
	FormatUint64(key string, value uint64)
	// Requirement: FormatUintptr must not panic.
	FormatUintptr(key string, value uintptr)
	// Requirement: FormatInt64 must not panic.
	FormatInt64(key string, value int64)
	// Requirement: FormatFloat64 must not panic.
	FormatFloat64(key string, value float64)
	// Precondition: value must be a constant string, not a dynamically generated
	// string.
	//
	// Requirement: FormatConstMsg must not panic.
	FormatConstMsg(value string)
	// Precondition: value must be a constant string, not a dynamically generated
	// string.
	//
	// Requirement: FormatConstString must not panic.
	FormatConstString(key string, value string)
	// Precondition: forEach must be non-nil.
	FormatGroup(key string, forEach func(Formatter))
	// Requirement: If Err() returns a non-nil value, subsequent calls to
	// Err() must return the same logical value.
	Err() *FormatterError
}

// ValueKind represents an enum which covers the different kinds
// of values which may be formatted. This is to allow preserving
// coarse-grained type information for letting a formatter
// decide on redaction on a per kind basis.
//
// Applications may introduce custom ValueKind constants with
// values from -1, -2, ...
//
// Libraries should generally not introduce their own constants,
// because these may collide with the constants introduced by
// other libraries. The existing constants should suffice for
// the most common cases.
type ValueKind int32

const (
	ValueKind_Bool ValueKind = iota + 1
	ValueKind_Uint64
	ValueKind_Uintptr
	ValueKind_Int64
	ValueKind_Float64
	ValueKind_ConstString
	ValueKind_Path

	// Insert new values here!

	ValueKind_StdMax           = ValueKind_Path
	ValueKind_StdMin ValueKind = 1
)

type FormatterError struct {
	kind  FormatterErrorKind
	inner error
}

func NewFormatterError(kind FormatterErrorKind, inner error) *FormatterError {
	return &FormatterError{kind, inner}
}

func (e *FormatterError) Kind() FormatterErrorKind {
	return e.kind
}

func (e *FormatterError) Unwrap() error {
	return e.inner
}

type FormatterErrorKind uint8

const (
	FormatterErrorKind_BufferFull FormatterErrorKind = iota + 1
	FormatterErrorKind_IOFailed
)
