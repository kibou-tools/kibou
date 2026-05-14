// Copyright 2026 Varun Gandhi
//
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package option

import "code.kibou.tools/common/assert"

// BoundsChecked denotes an optional value of type T,
// which potentially indicates overflow.
//
// Prefer using this over Option to indicate overflow.
type BoundsChecked[T any] struct {
	value      T
	overflowed bool
}

// InRange is a constructor for a BoundsChecked value,
// representing the result of a computation which stayed
// within the bounds of the underlying numeric type.
//
// This is the equivalent of option.Some.
func InRange[T any](t T) BoundsChecked[T] {
	return BoundsChecked[T]{value: t, overflowed: false}
}

// Overflowed is a constructor for a BoundsChecked value
// representing the result of a computation which overflowed
// the bounds of the underlying numeric type.
//
// This is the equivalent of option.None.
func Overflowed[T any]() BoundsChecked[T] {
	var zero T
	return BoundsChecked[T]{value: zero, overflowed: true}
}

// Get unpacks an InRange value, similar to how Option.Get
// unpacks a Some value.
//
// The first return value should not be consulted if the
// second return value is false.
func (b BoundsChecked[T]) Get() (_ T, inRange bool) {
	return b.value, !b.overflowed
}

// IsOverflowed checks if the computation overflowed.
func (b BoundsChecked[T]) IsOverflowed() bool {
	return b.overflowed
}

// Unwrap gets the underlying InRange value.
//
// Pre-condition: The value must not have Overflowed.
func (b BoundsChecked[T]) Unwrap() T {
	if b.overflowed {
		assert.Precondition(false, "called Unwrap on Overflowed value")
	}
	return b.value
}

func (b BoundsChecked[T]) Expect(invariantMsg string) T {
	if b.overflowed {
		assert.Invariant(false, invariantMsg)
	}
	return b.value
}
