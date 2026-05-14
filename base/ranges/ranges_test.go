// Copyright 2026 Varun Gandhi
//
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package ranges_test

import (
	"math"
	"slices"
	"testing"

	"github.com/google/go-cmp/cmp"
	"pgregory.net/rapid"

	"code.kibou.tools/base/assert"
	"code.kibou.tools/base/check"
	"code.kibou.tools/base/ranges"
)

func TestSpan(t *testing.T) {
	h := check.New(t)
	h.Parallel()

	h.Run("NewSpan", testNewSpan)
	h.Run("Length", testLength)
	h.Run("Contains", testContains)
	h.Run("CompareStrict", testCompareStrict)
	h.Run("Overlaps", func(h check.Harness) {
		h.Run("unit", testOverlapsUnit)
		h.Run("property", testOverlapsProperty)
	})
}

func testNewSpan(h check.Harness) {
	h.Parallel()
	h.AssertPanicsWith(
		assert.AssertionError{Fmt: "precondition violation: end %v before start %v", Args: []any{0, 5}},
		func() { _ = ranges.NewSpan(5, 0) },
	)
}

func testLength(h check.Harness) {
	h.Parallel()

	s := ranges.NewSpan(3, 10)
	check.AssertSame(h, 7, s.Length().Unwrap(), "in-range Length")

	empty := ranges.NewSpan(5, 5)
	check.AssertSame(h, 0, empty.Length().Unwrap(), "empty Length")
	check.AssertSame(h, true, empty.IsEmpty(), "IsEmpty")

	overflowed := ranges.NewSpan[int8](-128, 127)
	check.AssertSame(h, true, overflowed.Length().IsOverflowed(), "int8 overflow detected")

	for start := math.MinInt8; start <= math.MaxInt8; start++ {
		for end := start; end <= math.MaxInt8; end++ {
			length, ok := ranges.NewSpan(int8(start), int8(end)).Length().Get()
			if !ok { // overflowed
				continue
			}
			h.Assertf(length >= 0,
				"NewSpan(%d, %d).Length() = %d, want non-negative or Overflowed",
				start, end, length)
		}
	}
}

func testContains(h check.Harness) {
	h.Parallel()

	span := ranges.NewSpan(3, 7)
	check.AssertSame(h, false, span.Contains(2), "before start")
	check.AssertSame(h, true, span.Contains(3), "at start")
	check.AssertSame(h, true, span.Contains(6), "before end")
	check.AssertSame(h, false, span.Contains(7), "at end")

	empty := ranges.NewSpan(3, 3)
	check.AssertSame(h, false, empty.Contains(3), "empty span")
}

func testCompareStrict(h check.Harness) {
	h.Parallel()

	span := func(start, end int) ranges.Span[int] { return ranges.NewSpan(start, end) }

	type TestCase struct {
		Name string
		LHS  ranges.Span[int]
		RHS  ranges.Span[int]
		Want int
	}
	cases := []TestCase{
		{"equal", span(3, 7), span(3, 7), 0},
		{"smaller start", span(2, 7), span(3, 7), -1},
		{"larger start", span(4, 7), span(3, 7), 1},
		{"equal start, smaller end", span(3, 5), span(3, 7), -1},
		{"equal start, larger end", span(3, 9), span(3, 7), 1},
	}
	for _, tc := range cases {
		check.AssertSame(h, tc.Want, tc.LHS.CompareStrict(tc.RHS), tc.Name)
	}

	spans := []ranges.Span[int]{span(5, 10), span(3, 7), span(3, 9), span(3, 3), span(0, 0)}
	want := []ranges.Span[int]{span(0, 0), span(3, 3), span(3, 7), span(3, 9), span(5, 10)}
	slices.SortFunc(spans, func(a, b ranges.Span[int]) int { return a.CompareStrict(b) })
	check.AssertSame(h, want, spans, "sorted spans", cmp.AllowUnexported(ranges.Span[int]{}))
}

func testOverlapsUnit(h check.Harness) {
	h.Parallel()

	span := func(start, end int) ranges.Span[int] { return ranges.NewSpan(start, end) }

	type TestCase struct {
		Name string
		LHS  ranges.Span[int]
		RHS  ranges.Span[int]
		Want ranges.Relation
	}
	cases := []TestCase{
		{"Equal", span(5, 15), span(5, 15), ranges.Relation_Equal},
		{"BeforeStart", span(0, 5), span(10, 15), ranges.Relation_BeforeStart},
		{"AfterEnd", span(20, 25), span(10, 15), ranges.Relation_AfterEnd},
		{"Covers", span(0, 20), span(5, 15), ranges.Relation_Covers},
		{"IsCoveredBy", span(7, 11), span(5, 15), ranges.Relation_IsCoveredBy},
		{"OverlapsStart", span(0, 8), span(5, 15), ranges.Relation_OverlapsStart},
		{"OverlapsEnd", span(10, 20), span(5, 15), ranges.Relation_OverlapsEnd},
	}
	for _, tc := range cases {
		check.AssertSame(h, tc.Want, tc.LHS.Overlaps(tc.RHS), tc.Name)
	}
}

func testOverlapsProperty(h check.Harness) {
	h.Parallel()

	h.Run("int8", func(h check.Harness) {
		genSpan := rapid.Custom(func(t *rapid.T) ranges.Span[int8] {
			start := rapid.Int8().Draw(t, "start")
			end := rapid.Int8Range(start, math.MaxInt8).Draw(t, "end")
			return ranges.NewSpan(start, end)
		})
		runOverlapsProperties(h, genSpan)
	})

	h.Run("uint8", func(h check.Harness) {
		genSpan := rapid.Custom(func(t *rapid.T) ranges.Span[uint8] {
			start := rapid.Uint8().Draw(t, "start")
			end := rapid.Uint8Range(start, math.MaxUint8).Draw(t, "end")
			return ranges.NewSpan(start, end)
		})
		runOverlapsProperties(h, genSpan)
	})
}

func runOverlapsProperties[T ranges.Numeric](h check.Harness, genSpan *rapid.Generator[ranges.Span[T]]) {
	h.Run("reflexivity", func(h check.Harness) {
		rapid.Check(h.T(), func(t *rapid.T) {
			s := genSpan.Draw(t, "s")
			if got := s.Overlaps(s); got != ranges.Relation_Equal {
				t.Fatalf("R(s, s) = %v, want Equal (s = %+v)", got, s)
			}
		})
	})

	h.Run("duality", func(h check.Harness) {
		rapid.Check(h.T(), func(t *rapid.T) {
			a := genSpan.Draw(t, "a")
			b := genSpan.Draw(t, "b")
			rAB := a.Overlaps(b)
			rBA := b.Overlaps(a)
			if want := rAB.Inverse(); rBA != want {
				t.Fatalf("R(a, b) = %v but R(b, a) = %v; want %v", rAB, rBA, want)
			}
		})
	})

	h.Run("transitivity", func(h check.Harness) {
		rapid.Check(h.T(), func(t *rapid.T) {
			s1 := genSpan.Draw(t, "s1")
			s2 := genSpan.Draw(t, "s2")
			s3 := genSpan.Draw(t, "s3")
			r12 := s1.Overlaps(s2)
			r23 := s2.Overlaps(s3)
			want, ok := transitiveRelation(r12, r23)
			if !ok {
				return
			}
			if got := s1.Overlaps(s3); got != want {
				t.Fatalf("R(s1,s2)=%v R(s2,s3)=%v R(s1,s3)=%v, want %v", r12, r23, got, want)
			}
		})
	})
}

// transitiveRelation attempts to determine based on r12 and r23,
// if there's a unique possible value for r13.
func transitiveRelation(r12, r23 ranges.Relation) (ranges.Relation, bool) {
	switch r12 {
	case ranges.Relation_Equal:
		return r23, true
	case ranges.Relation_BeforeStart,
		ranges.Relation_AfterEnd,
		ranges.Relation_Covers,
		ranges.Relation_IsCoveredBy:
		if r23 == r12 || r23 == ranges.Relation_Equal {
			return r12, true
		}
		return 0, false
	case ranges.Relation_OverlapsStart, ranges.Relation_OverlapsEnd:
		if r23 == ranges.Relation_Equal {
			return r12, true
		}
		return 0, false
	default:
		return assert.PanicUnknownCase[ranges.Relation](r12), false
	}
}
