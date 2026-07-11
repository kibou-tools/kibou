// Copyright 2026 Varun Gandhi
//
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

// Package result provides a generic Result[T] type holding either a value or
// an error.
package result

import "code.kibou.tools/base/assert"

// Result holds either a value of type T or an error.
//
// Use [Success] or [Failure] to construct one. A Failure always carries a
// non-nil error; a Success has a nil error.
type Result[T any] struct {
	value T
	err   error
}

// NewResult converts a (value, error) return into a Result.
//
// The value is discarded if err != nil.
func New[T any](value T, err error) Result[T] {
	if err != nil {
		return Failure[T](err)
	}
	return Success(value)
}

// Success returns a Result containing value.
func Success[T any](value T) Result[T] {
	return Result[T]{value: value, err: nil}
}

// Failure returns a Result containing err.
//
// Pre-condition: err is non-nil.
func Failure[T any](err error) Result[T] {
	assert.Preconditionf(err != nil, "Failure called with nil error")
	var zero T
	return Result[T]{value: zero, err: err}
}

// Get returns the contained value and error.
// If r is a Success, err is nil.
// If r is a Failure, err is non-nil and the value is the zero value of T.
func (r Result[T]) Get() (T, error) {
	return r.value, r.err
}

// ErrOrNil returns the contained error, or nil if r is a Success.
func (r Result[T]) ErrOrNil() error {
	return r.err
}

// Status is the outcome of an operation, without an associated value or error.
//
// Use it where a caller must tell a callee whether the surrounding work
// succeeded so the callee can finalize accordingly (for example, committing
// versus discarding a partially written file).
type Status uint8

const (
	Status_Success Status = iota + 1
	Status_Failure
)

func NewStatusFromError(err error) Status {
	if err != nil {
		return Status_Failure
	}
	return Status_Success
}
