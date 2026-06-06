// Copyright 2026 Varun Gandhi
//
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package ranges

import (
	"cmp"

	"code.kibou.tools/base/assert"
	"code.kibou.tools/base/core/option"
)

type Numeric interface {
	~int | ~int8 | ~int16 | ~int32 | ~int64 |
		~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64
}

// Span represents a half-open interval [start, end).
//
// The Span may be empty if start == end.
type Span[T Numeric] struct {
	start T
	end   T
}

// NewSpan constructs a new Span from a start and end value.
//
// Pre-condition: end >= start.
func NewSpan[T Numeric](start, end T) Span[T] {
	if end < start {
		assert.Preconditionf(false, "end %v before start %v", end, start)
	}
	return Span[T]{start: start, end: end}
}

func (s Span[T]) Start() T      { return s.start }
func (s Span[T]) End() T        { return s.end }
func (s Span[T]) IsEmpty() bool { return s.start == s.end }

// Length attempts to compute the length of this span using the
// span's numeric type.
//
// Post-condition: If the return value is [option.InRange], it will be
// non-negative.
func (s Span[T]) Length() option.BoundsChecked[T] {
	length := s.end - s.start
	// By construction, s.start <= s.end. Suppose we have N bits.
	//
	// For unsigned integer types, s.end - s.start ∈ [0, 2^N - 1];
	// there's no possibility of overflow.
	//
	// For signed integer types, s.end - s.start ∈ [0, 2^N - 1].
	// If s.end - s.start ∈ [0, 2^(N-1) - 1], then length >= 0.
	// Else s.end - s.start ∈ [2^(N-1), 2^N - 1].
	//   - Wrapping on overflow is equivalent to subtracting
	//     2^N. So length ∈ [-2^(N-1), -1].
	//
	// Hence, length < 0 is sufficient to detect overflow.
	if length < 0 {
		return option.Overflowed[T]()
	}
	return option.InRange(length)
}

// CompareStrict defines a total lexicographic order across ranges.
func (s Span[T]) CompareStrict(other Span[T]) int {
	if c := cmp.Compare(s.start, other.start); c != 0 {
		return c
	}
	return cmp.Compare(s.end, other.end)
}

// Overlaps computes the relationship between the receiver and the argument span.
func (s Span[T]) Overlaps(other Span[T]) Relation {
	switch {
	case s.start == other.start && s.end == other.end:
		return Relation_Equal
	case s.end <= other.start:
		return Relation_BeforeStart
	case s.start >= other.end:
		return Relation_AfterEnd
	case s.start <= other.start && s.end >= other.end:
		return Relation_Covers
	case s.start >= other.start && s.end <= other.end:
		return Relation_IsCoveredBy
	case s.start < other.start:
		return Relation_OverlapsStart
	default:
		return Relation_OverlapsEnd
	}
}

// Relation describes the relationship between two ranges.
type Relation uint8

const (
	Relation_BeforeStart Relation = iota + 1
	Relation_OverlapsStart
	Relation_IsCoveredBy
	Relation_Equal
	Relation_Covers
	Relation_OverlapsEnd
	Relation_AfterEnd
)

func (r Relation) Inverse() Relation {
	switch r {
	case Relation_BeforeStart:
		return Relation_AfterEnd
	case Relation_AfterEnd:
		return Relation_BeforeStart
	case Relation_Covers:
		return Relation_IsCoveredBy
	case Relation_IsCoveredBy:
		return Relation_Covers
	case Relation_OverlapsStart:
		return Relation_OverlapsEnd
	case Relation_OverlapsEnd:
		return Relation_OverlapsStart
	case Relation_Equal:
		return Relation_Equal
	default:
		return assert.PanicUnknownCase[Relation](r)
	}
}

func (r Relation) String() string {
	switch r {
	case Relation_BeforeStart:
		return "before-start"
	case Relation_OverlapsStart:
		return "overlaps-start"
	case Relation_IsCoveredBy:
		return "is-covered-by"
	case Relation_Equal:
		return "equal"
	case Relation_Covers:
		return "covers"
	case Relation_OverlapsEnd:
		return "overlaps-end"
	case Relation_AfterEnd:
		return "after-end"
	default:
		return assert.PanicUnknownCase[string](r)
	}
}
