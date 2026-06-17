// Copyright 2026 Varun Gandhi
//
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

// Package assert provides precondition and invariant checks that panic on violation.
package assert

import "fmt"

// AssertionError is the value passed to panic by assertion helpers.
// Formatting is deferred to String(), so no work is done on the happy path.
type AssertionError struct {
	Fmt  string
	Args []any
}

func NewError(format string, args ...any) AssertionError {
	return AssertionError{Fmt: format, Args: args}
}

func (e AssertionError) String() string {
	return fmt.Sprintf(e.Fmt, e.Args...)
}

func (e AssertionError) Error() string {
	return e.String()
}

//go:noinline
func panicWith[R any](msg string, args []any) R {
	panic(AssertionError{Fmt: msg, Args: args})
}

//go:noinline
func panicWithMessage[R any](format string, msg string) R {
	panic(AssertionError{Fmt: format, Args: []any{msg}})
}

// Always panics with a formatted message if b is false.
func Always(b bool, msg string, args ...any) {
	if !b {
		panicWith[int](msg, args)
	}
}

// Precondition panics if b is false, prefixing the message with
// "precondition violation: ".
func Precondition(b bool, msg string) {
	if !b {
		panicWithMessage[int]("precondition violation: %s", msg)
	}
}

// Preconditionf is like Precondition but with a formatted message.
func Preconditionf(b bool, msg string, args ...any) {
	Always(b, "precondition violation: "+msg, args...)
}

// Invariant panics if b is false, prefixing the message with
// "invariant violation: ".
func Invariant(b bool, msg string) {
	if !b {
		panicWithMessage[int]("invariant violation: %s", msg)
	}
}

// Invariantf is like Invariant, but with a formatted message.
func Invariantf(b bool, msg string, args ...any) {
	Always(b, "invariant violation: "+msg, args...)
}

// Postcondition panics if b is false, prefixing the message with
// "postcondition violation: ".
func Postcondition(b bool, msg string) {
	if !b {
		panicWithMessage[int]("postcondition violation: %s", msg)
	}
}

// Postconditionf is like Postcondition but with a formatted message.
func Postconditionf(b bool, msg string, args ...any) {
	Always(b, "postcondition violation: "+msg, args...)
}

// PanicInvariantViolation panics indicating an invariant was violated.
// The return type R allows using it in a return statement.
//
//go:noinline
func PanicInvariantViolation[R any](msg string, args ...any) R {
	return panicWith[R]("invariant violation: "+msg, args)
}

// PanicUnknownCase panics with a message indicating an unhandled enum value.
// The return type R allows using it in a return statement.
//
//go:noinline
func PanicUnknownCase[R any, T any](t T) R {
	return panicWith[R]("unknown value for type %T: %v", []any{t, t})
}
