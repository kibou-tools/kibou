// Copyright 2026 Varun Gandhi
//
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package uniseg

import (
	"iter"

	"code.kibou.tools/base/assert"
	rivouniseg "github.com/rivo/uniseg"

	"code.kibou.tools/base/collections"
	"code.kibou.tools/base/core/option"
	"code.kibou.tools/base/ranges"
	"code.kibou.tools/base/utf8"
)

// SegmentedText stores text together with its grapheme cluster boundaries.
type SegmentedText struct {
	text utf8.Text
	// boundaries[i] is true iff i == text.Len() or if
	// text.GetByte(i) is the start byte of a grapheme cluster.
	//
	// boundaries.Len() == text.Len() + 1
	boundaries collections.BitVec
}

func NewSegmentedText(text utf8.Text) SegmentedText {
	boundaries := collections.NewBitVec(text.Len() + 1)
	boundaries.Set(0)
	for cluster := range GraphemeClusters(text) {
		boundaries.Set(cluster.Span().End())
	}
	return SegmentedText{text: text, boundaries: boundaries}
}

// CheckSpan checks that the bounds of the span line up with
// UTF-8 codepoint boundaries and grapheme cluster boundaries.
//
// Pre-condition: span ⊆ [0, t.text.Len()].
//
// Post-condition: If the returned error has kind
// [SpanBoundaryErrorKind_NotGraphemeBoundary], then ByteOffset() is a UTF-8
// boundary strictly inside a grapheme cluster.
func (t *SegmentedText) CheckSpan(span ranges.Span[int]) *SpanBoundaryError {
	start := span.Start()
	end := span.End()
	if start != t.text.Len() && !utf8.IsPotentialStartOfRune(t.text.GetByte(start)) {
		return &SpanBoundaryError{SpanBoundaryErrorKind_NotUTF8Boundary, ranges.Bound_Start, span}
	}
	if end != t.text.Len() && !utf8.IsPotentialStartOfRune(t.text.GetByte(end)) {
		return &SpanBoundaryError{SpanBoundaryErrorKind_NotUTF8Boundary, ranges.Bound_End, span}
	}
	if !t.boundaries.Get(start) {
		return &SpanBoundaryError{SpanBoundaryErrorKind_NotGraphemeBoundary, ranges.Bound_Start, span}
	}
	if !t.boundaries.Get(end) {
		return &SpanBoundaryError{SpanBoundaryErrorKind_NotGraphemeBoundary, ranges.Bound_End, span}
	}
	return nil
}

// GraphemeClusterWindow is a possibly truncated grapheme-cluster byte span.
type GraphemeClusterWindow struct {
	span          ranges.Span[int]
	isFullCluster bool
}

func (w GraphemeClusterWindow) Span() ranges.Span[int] { return w.span }

func (w GraphemeClusterWindow) IsFullCluster() bool { return w.isFullCluster }

// FindGraphemeClusterContaining returns a codepoint-aligned window around an
// offset strictly inside a grapheme cluster.
//
// If the containing grapheme cluster has at most maxCodePoints codepoints,
// the returned window is the full cluster. Otherwise, the window contains
// maxCodePoints / 2 codepoints before offset, and the rest after offset.
//
// Preconditions:
//   - offset ∈ [0, t.text.Len()].
//   - offset == t.text.Len() or offset points to the first byte of
//     a UTF-8 code point.
//   - maxCodePoints >= 1.
//
// Postconditions: If Some(window) is returned:
//   - window.Span() contains offset
//   - window.Span() will be codepoint aligned
func (t *SegmentedText) FindGraphemeClusterContaining(offset int, maxCodePoints int) option.Option[GraphemeClusterWindow] {
	if offset < 0 || t.text.Len() < offset {
		assert.Preconditionf(false, "offset %d outside text bounds [0, %d]", offset, t.text.Len())
	}
	if maxCodePoints < 1 {
		assert.Preconditionf(false, "maxCodePoints %d below 1", maxCodePoints)
	}
	if offset != t.text.Len() && !utf8.IsPotentialStartOfRune(t.text.GetByte(offset)) {
		assert.Preconditionf(false, "offset %d is not a UTF-8 boundary", offset)
	}
	if offset == 0 || offset == t.text.Len() || t.boundaries.Get(offset) {
		return option.None[GraphemeClusterWindow]()
	}

	// Optimistic loop: try to include the entire grapheme cluster when it fits
	// within maxCodePoints, even if offset is unbalanced within the cluster.
	start, end := offset, offset
	codePoints := 0
	for codePoints < maxCodePoints && !t.boundaries.Get(start) {
		start = t.previousCodePointStart(start)
		codePoints++
	}
	for codePoints < maxCodePoints && !t.boundaries.Get(end) {
		end = t.nextCodePointEnd(end)
		codePoints++
	}
	if t.boundaries.Get(start) && t.boundaries.Get(end) {
		return option.Some(GraphemeClusterWindow{
			span:          ranges.NewSpan(start, end),
			isFullCluster: true,
		})
	}

	leftBudget := maxCodePoints / 2
	rightBudget := maxCodePoints - leftBudget

	// Fallback loop: return a balanced, bounded window around offset.
	// Invariant: start and end always point to valid codepoint boundaries
	// (including t.text.Len() for end)
	start, end = offset, offset
	for i := 0; i < rightBudget; i++ {
		if i < leftBudget && !t.boundaries.Get(start) {
			// start is not a grapheme cluster boundary, so try
			// to walk back by 1 codepoint.
			start = t.previousCodePointStart(start)
		}
		if !t.boundaries.Get(end) {
			// i < rightBudget means we can potentially look at one more
			// codepoint on the right side.
			end = t.nextCodePointEnd(end)
		}
		if t.boundaries.Get(start) && t.boundaries.Get(end) {
			// We found a full grapheme cluster within the budget!
			// We know this is exactly 1 grapheme cluster because we checked
			// t.boundaries.Get(offset) earlier, and grapheme clusters are
			// composed of 1 or more codepoints.
			break
		}
	}

	return option.Some(GraphemeClusterWindow{
		span:          ranges.NewSpan(start, end),
		isFullCluster: t.boundaries.Get(start) && t.boundaries.Get(end),
	})
}

// Pre-condition: t.text[i] is the first byte of a codepoint.
func (t *SegmentedText) nextCodePointEnd(i int) int {
	decoded := utf8.TryDecodeFirstRune(t.text.String()[i:])
	assert.Invariant(decoded.Kind() == utf8.RuneDecodingResultKind_Valid, "segmented text should contain valid UTF-8")
	return i + decoded.ByteLen()
}

func (t *SegmentedText) previousCodePointStart(before int) int {
	start := before - 1
	for 0 < start && !utf8.IsPotentialStartOfRune(t.text.GetByte(start)) {
		start--
	}
	return start
}

// GraphemeCluster is one user-perceived character within a string.
type GraphemeCluster struct {
	graphemes *rivouniseg.Graphemes
}

// GraphemeClusters iterates over the grapheme clusters in s.
func GraphemeClusters(text utf8.Text) iter.Seq[GraphemeCluster] {
	return func(yield func(GraphemeCluster) bool) {
		graphemes := rivouniseg.NewGraphemes(text.String())
		cluster := GraphemeCluster{graphemes}
		for graphemes.Next() {
			if !yield(cluster) {
				return
			}
		}
	}
}

// Str returns the substring for this grapheme cluster.
func (c GraphemeCluster) Str() string { return c.graphemes.Str() }

// Span returns the byte span for this grapheme cluster in the original string.
func (c GraphemeCluster) Span() ranges.Span[int] {
	start, end := c.graphemes.Positions()
	return ranges.NewSpan(start, end)
}

// ComputeWidth returns the monospace display width of s.
func ComputeWidth(s string) int { return rivouniseg.StringWidth(s) }

type SpanBoundaryError struct {
	kind  SpanBoundaryErrorKind
	bound ranges.Bound
	span  ranges.Span[int]
}

type SpanBoundaryErrorKind uint8

const (
	SpanBoundaryErrorKind_NotUTF8Boundary SpanBoundaryErrorKind = iota + 1
	SpanBoundaryErrorKind_NotGraphemeBoundary
)

// Kind returns which boundary a particular span failed to line up with.
func (e *SpanBoundaryError) Kind() SpanBoundaryErrorKind { return e.kind }

// Bound represents the end/edge of the span which did not correctly
// line up with the codepoint/grapheme cluster boundary.
func (e *SpanBoundaryError) Bound() ranges.Bound { return e.bound }

// Span returns the input span passed to CheckSpan.
func (e *SpanBoundaryError) Span() ranges.Span[int] { return e.span }

// ByteOffset returns the offending offset which did not correctly
// line up with a codepoint/grapheme cluster boundary.
func (e *SpanBoundaryError) ByteOffset() int {
	switch e.bound {
	case ranges.Bound_Start:
		return e.span.Start()
	case ranges.Bound_End:
		return e.span.End()
	default:
		return assert.PanicUnknownCase[int](e.bound)
	}
}
