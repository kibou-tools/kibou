// Copyright 2026 Varun Gandhi
//
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package result

import (
	"code.kibou.tools/common/assert"
	"code.kibou.tools/common/core/option"
)

// Partial represents the outcome of an operation which can produce:
//
// - A value
// - An error
// - Both a value and an error.
//
// This is useful when callers can recover a degraded value from an error.
type Partial[T any, E any] struct {
	value option.Option[T]
	err   *E
}

// NewPartial constructs a Partial value.
//
// Pre-condition: If err is nil, then value must be some.
func NewPartial[T any, E any](value option.Option[T], err *E) Partial[T, E] {
	if err == nil && value.IsNone() {
		assert.Precondition(false, "NewPartial must get value | error | both, but got no value with nil error")
	}
	return Partial[T, E]{value: value, err: err}
}

// Get unpacks a Partial value.
//
// If the error is nil, then the value is guaranteed to be present.
// If the error is non-nil, then the value may or may not be present.
func (p Partial[T, E]) Get() (option.Option[T], *E) {
	return p.value, p.err
}

func (p Partial[T, E]) Value() option.Option[T] { return p.value }
func (p Partial[T, E]) Err() *E                 { return p.err }
