// Copyright 2026 Varun Gandhi
//
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package core

import "code.kibou.tools/base/core/result"

// Result holds either a value of type T or an error.
//
// Use [Success] or [Failure] to construct one. A Failure always carries a
// non-nil error; a Success has a nil error.
type Result[T any] = result.Result[T]

// NewResult converts a (value, error) return into a Result.
//
// The value is discarded if err != nil.
//
// NOTE: Due to special forwarding rules (https://go.dev/ref/spec#Calls),
// you can use this directly to wrap a function which returns (T, err):
//
//	data, err := someOperation(...)
//	result := NewResult(data, err)
//	// => simplify to
//	result := NewResult(someOperation(...))
func NewResult[T any](value T, err error) Result[T] {
	return result.New(value, err)
}

// Success returns a Result containing value.
func Success[T any](value T) Result[T] {
	return result.Success(value)
}

// Failure returns a Result containing err.
//
// Pre-condition: err is non-nil.
func Failure[T any](err error) Result[T] {
	return result.Failure[T](err)
}
