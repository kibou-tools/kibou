// Copyright 2026 Varun Gandhi
//
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package collections

import (
	"iter"
	"math"

	"code.kibou.tools/common/assert"
	"code.kibou.tools/common/core/option"
)

// Deque is a double-ended queue implemented as a growable
// ring buffer.
type Deque[T any] struct {
	// The logical element at offset i is stored at (head+i)%cap(values)
	values []T
	// cap(values) == 0 && head == 0 or 0 <= head < cap(values)
	head int
	// 0 <= len <= cap(values)
	len int
}

// NewDeque creates an empty Deque.
//
// Post-condition: the deque has length 0.
func NewDeque[T any]() Deque[T] {
	return Deque[T]{values: nil, head: 0, len: 0}
}

// Len returns the number of elements in the buffer.
func (b *Deque[T]) Len() int {
	return b.len
}

// IsEmpty reports whether the deque has no elements.
func (b *Deque[T]) IsEmpty() bool {
	return b.Len() == 0
}

// Iter returns the deque's elements in order from front to back.
func (b *Deque[T]) Iter() iter.Seq[T] {
	return func(yield func(T) bool) {
		first, second := b.splitContiguous()
		for _, value := range first {
			if !yield(value) {
				return
			}
		}
		for _, value := range second {
			if !yield(value) {
				return
			}
		}
	}
}

// Code generation note:
//
// Using assertions under a branch instead of using them directly,
// because the Go 1.25 compiler does not sink heap allocations
// into conditional branches/across blocks.

// ReserveMore ensures space for n more elements.
//
// Pre-condition: 0 <= n <= math.MaxInt - Len.
// Post-condition: Len is unchanged, existing values keep their order, and at
// least n values can be pushed without growing the backing storage again.
func (b *Deque[T]) ReserveMore(n int) {
	if n < 0 || n > math.MaxInt-b.len {
		assert.Preconditionf(
			false,
			"ReserveMore needs 0 <= n <= math.MaxInt - Len, got n=%d Len=%d",
			n,
			b.len,
		)
	}
	b.reserveCapacity(b.len + n) // check earlier => no overflow
}

// PushFront adds value to the front of the buffer.
//
// Pre-condition: Len < math.MaxInt.
// Post-condition: Len increases by 1 and all previous values keep their order.
func (b *Deque[T]) PushFront(value T) {
	if b.len == math.MaxInt {
		assert.Preconditionf(false, "PushFront needs Len < math.MaxInt, got Len=%d", b.len)
	}
	if b.len == cap(b.values) {
		b.grow()
	}
	b.head--
	if b.head < 0 {
		b.head = cap(b.values) - 1
	}
	b.values[b.head] = value
	b.len++
}

// PushBack adds value to the back of the buffer.
//
// Pre-condition: Len < math.MaxInt.
// Post-condition: Len increases by 1 and all previous values keep their order.
func (b *Deque[T]) PushBack(value T) {
	if b.len == math.MaxInt {
		assert.Preconditionf(false, "PushBack needs Len < math.MaxInt, got Len=%d", b.len)
	}
	if b.len == cap(b.values) {
		b.grow()
	}
	idx := b.nextBackIndex()
	b.values[idx] = value
	b.len++
}

func (b *Deque[T]) TryPopFront() option.Option[T] {
	if b.len == 0 {
		return option.None[T]()
	}
	return option.Some(b.PopFront())
}

// PopFront removes and returns the front element.
//
// Pre-condition: Len() > 0.
// Post-condition: Len() decreases by 1 and backing storage capacity is unchanged.
func (b *Deque[T]) PopFront() T {
	if b.len == 0 {
		assert.Preconditionf(false, "PopFront on empty deque")
	}

	value := b.values[b.head]
	var zero T
	b.values[b.head] = zero // eagerly drop reference for GC
	b.len--
	if b.len == 0 {
		b.head = 0
		return value
	}
	b.head++
	if b.head == cap(b.values) {
		b.head = 0
	}
	return value
}

// PopBack removes and returns the back element.
//
// Pre-condition: the buffer is non-empty.
// Post-condition: Len() decreases by 1 and backing storage capacity is unchanged.
func (b *Deque[T]) PopBack() T {
	if b.len <= 0 {
		assert.Preconditionf(false, "PopBack on empty deque: %p", b)
	}

	idx := b.lastIndex()
	value := b.values[idx]
	var zero T
	b.values[idx] = zero // eagerly drop reference for GC
	b.len--
	if b.len == 0 {
		b.head = 0
	}
	return value
}

// grow increases capacity enough to push one more value.
//
// Pre-condition: the deque is full.
// Post-condition: Len() is unchanged, capacity is at least old Len() + 1, and head is 0.
func (b *Deque[T]) grow() {
	if b.len != cap(b.values) {
		assert.Precondition(false, "grow on non-full deque")
	}
	b.reserveCapacity(b.len + 1)
}

// reserveCapacity ensures capacity for wantCap total elements.
//
// Pre-condition: wantCap >= Len.
// Post-condition: Len() is unchanged, capacity is at least wantCap, existing
// values keep their order, and head is 0 if backing storage was reallocated.
func (b *Deque[T]) reserveCapacity(wantCap int) {
	if wantCap < b.len {
		assert.Preconditionf(
			false,
			"reserveCapacity needs wantCap >= len, got %d < %d",
			wantCap,
			b.len,
		)
	}
	oldCap := cap(b.values)
	if wantCap <= oldCap {
		return
	}
	newCap := dequeCapacityFor(wantCap, oldCap)
	newValues := make([]T, newCap)
	first, second := b.splitContiguous()
	copy(newValues, first)
	copy(newValues[len(first):], second)
	b.values = newValues
	b.head = 0
}

// splitContiguous returns the deque's contents as one or two contiguous slices
// in logical order. The returned slices alias the deque's backing storage.
func (b *Deque[T]) splitContiguous() ([]T, []T) {
	firstLen := min(b.len, cap(b.values)-b.head)
	return b.values[b.head : b.head+firstLen], b.values[:b.len-firstLen]
}

// dequeCapacityFor returns the capacity to use for a deque requiring at least
// wantCap elements.
//
// Pre-condition: wantCap > oldCap >= 0.
// Post-condition: returns capacity >= max(4, wantCap, min(oldCap * 2, math.MaxInt)).
func dequeCapacityFor(wantCap int, oldCap int) int {
	if wantCap <= oldCap || oldCap < 0 {
		assert.Preconditionf(
			false,
			"dequeCapacityFor needs wantCap > oldCap >= 0, got wantCap=%d oldCap=%d",
			wantCap,
			oldCap,
		)
	}

	newCap := max(wantCap, 4)
	if oldCap <= math.MaxInt/2 {
		newCap = max(newCap, 2*oldCap)
	}
	return newCap
}

// nextBackIndex returns the physical index for the next PushBack value.
//
// Pre-condition: Len < cap(values)
func (b *Deque[T]) nextBackIndex() int {
	return b.physicalIndex(b.len)
}

// lastIndex returns the physical index of the last element.
//
// Pre-condition: Len() >= 1.
func (b *Deque[T]) lastIndex() int {
	return b.physicalIndex(b.len - 1)
}

// physicalIndex returns the physical index for a logical offset from head.
//
// Pre-condition: offset >= 0.
func (b *Deque[T]) physicalIndex(offset int) int {
	// Checked that this compiles to branchless code for arm64 and
	// x86_64 with Go 1.25.
	tailLen := cap(b.values) - b.head
	if offset < tailLen {
		return b.head + offset
	}
	return offset - tailLen
}
