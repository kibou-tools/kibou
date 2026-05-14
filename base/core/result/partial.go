// Copyright 2026 Varun Gandhi
//
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package result

import (
	"code.kibou.tools/base/assert"
	"code.kibou.tools/base/core/option"
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

// Value returns the computed value for the 'value' and 'both' cases
// of a Partial.
//
// If Err() returned nil, then this is guaranteed to be Some(v).
// Otherwise, this may be Some(v) or None.
func (p Partial[T, E]) Value() option.Option[T] { return p.value }

// Err returns the error for the 'error' and 'both' cases of a Partial.
//
// If this method returns nil, then Value() is guaranteed to return Some(v).
func (p Partial[T, E]) Err() *E { return p.err }
