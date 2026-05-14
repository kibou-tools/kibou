// Copyright 2026 Varun Gandhi
//
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package uniseg

import (
	"iter"
	"unicode/utf8"

	"code.kibou.tools/common/assert"
	rivouniseg "github.com/rivo/uniseg"

	"code.kibou.tools/common/collections"
	"code.kibou.tools/common/core/option"
	"code.kibou.tools/common/ranges"
)

// SegmentedText stores text together with its grapheme cluster boundaries.
type SegmentedText struct {
	text string
	// boundaries[i] is true iff i == len(text) or if
	// text[i] is the start byte of a grapheme cluster.
	//
	// boundaries.Len() == len(text) + 1
	boundaries collections.BitVec
}

// Pre-condition: text is valid UTF-8.
func NewSegmentedText(text string) SegmentedText {
	boundaries := collections.NewBitVec(len(text) + 1)
	boundaries.Set(0)
	for cluster := range GraphemeClusters(text) {
		boundaries.Set(cluster.Span().End())
	}
	return SegmentedText{text: text, boundaries: boundaries}
}

// CheckSpan checks that the bounds of the span line up with
// UTF-8 codepoint boundaries and grapheme cluster boundaries.
//
// Pre-condition: span ⊆ [0, len(t.text)).
func (t *SegmentedText) CheckSpan(span ranges.Span[int]) *SpanBoundaryError {
	start := span.Start()
	end := span.End()
	if start != len(t.text) && !utf8.RuneStart(t.text[start]) {
		return &SpanBoundaryError{SpanBoundaryErrorKind_NotUTF8Boundary, ranges.Bound_Start, span, option.None[ranges.Span[int]]()}
	}
	if end != len(t.text) && !utf8.RuneStart(t.text[end]) {
		return &SpanBoundaryError{SpanBoundaryErrorKind_NotUTF8Boundary, ranges.Bound_End, span, option.None[ranges.Span[int]]()}
	}
	if !t.boundaries.Get(start) {
		return &SpanBoundaryError{SpanBoundaryErrorKind_NotGraphemeBoundary, ranges.Bound_Start, span, t.findContainingGraphemeCluster(start)}
	}
	if !t.boundaries.Get(end) {
		return &SpanBoundaryError{SpanBoundaryErrorKind_NotGraphemeBoundary, ranges.Bound_End, span, t.findContainingGraphemeCluster(end)}
	}
	return nil
}

func (t *SegmentedText) findContainingGraphemeCluster(offset int) option.Option[ranges.Span[int]] {
	if offset < 0 || t.boundaries.Len() <= offset || t.boundaries.Get(offset) {
		return option.None[ranges.Span[int]]()
	}
	start, ok := t.boundaries.FindAtOrBefore(offset).Get()
	if !ok {
		return option.None[ranges.Span[int]]()
	}
	end, ok := t.boundaries.FindAtOrAfter(offset).Get()
	if !ok {
		return option.None[ranges.Span[int]]()
	}
	return option.Some(ranges.NewSpan(start, end))
}

// GraphemeCluster is one user-perceived character within a string.
type GraphemeCluster struct {
	graphemes *rivouniseg.Graphemes
}

// GraphemeClusters iterates over the grapheme clusters in s.
func GraphemeClusters(s string) iter.Seq[GraphemeCluster] {
	return func(yield func(GraphemeCluster) bool) {
		graphemes := rivouniseg.NewGraphemes(s)
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
	kind                      SpanBoundaryErrorKind
	bound                     ranges.Bound
	span                      ranges.Span[int]
	containingGraphemeCluster option.Option[ranges.Span[int]]
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

// ContainingGraphemeCluster returns the span corresponding to
// the grapheme cluster which contains the ByteOffset.
func (e *SpanBoundaryError) ContainingGraphemeCluster() option.Option[ranges.Span[int]] {
	return e.containingGraphemeCluster
}
