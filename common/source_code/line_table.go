// Copyright 2026 Varun Gandhi
//
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package source_code

import (
	"iter"
	"sort"

	"code.kibou.tools/common/assert"
	"code.kibou.tools/common/ranges"
)

// LineTable represents a line table with the accompanying source.
//
// This allows for efficient lookup of lines based on byte offsets.
type LineTable struct {
	text            string
	startLineNumber int
	// Always non-empty.
	//
	// The first element has Start == 0, the last element has
	// End == len(text).
	lineOffsets []ranges.Span[int]
}

// NewLineTable creates a new LineTable whose first line number is 1.
//
// The input string is retained.
func NewLineTable(s string) LineTable {
	return NewLineTableAt(s, 1)
}

// NewLineTableAt creates a new LineTable whose first line number is startLineNumber.
//
// Pre-condition: startLineNumber >= 1.
//
// The input string is retained.
func NewLineTableAt(s string, startLineNumber int) LineTable {
	if startLineNumber < 1 {
		assert.Preconditionf(false, "startLineNumber (%d) should be >= 1", startLineNumber)
	}
	lineOffsets := make([]ranges.Span[int], 0, 1)
	start := 0
	for i := 0; i < len(s); {
		sepEnd := i
		switch s[i] {
		case '\r':
			sepEnd++
			if sepEnd < len(s) && s[sepEnd] == '\n' {
				sepEnd++
			}
		case '\n':
			sepEnd++
		default:
			i++
			continue
		}
		lineOffsets = append(lineOffsets, ranges.NewSpan(start, i))
		start = sepEnd
		i = sepEnd
	}
	lineOffsets = append(lineOffsets, ranges.NewSpan(start, len(s)))
	return LineTable{text: s, startLineNumber: startLineNumber, lineOffsets: lineOffsets}
}

// TextLen returns the number of bytes of text stored in this LineTable.
func (t *LineTable) TextLen() int {
	return len(t.text)
}

// LineCount returns the number of lines in this LineTable.
//
// This is equal to 1 + the number of line endings (\r\n or \n)
// in the text, even if the text ends with a line ending.
//
// Time complexity: O(1)
func (t *LineTable) LineCount() int {
	return len(t.lineOffsets)
}

// Lines returns an iterator over the underlying lines in
// the LineTable in ascending order.
func (t *LineTable) Lines() iter.Seq[Line] {
	return func(yield func(Line) bool) {
		for i, lineOffset := range t.lineOffsets {
			if !yield(t.line(i, lineOffset)) {
				return
			}
		}
	}
}

// LineAtByteOffset returns the line containing byteOffset.
//
// Pre-condition: byteOffset ∈ [0, t.TextLen()].
func (t *LineTable) LineAtByteOffset(byteOffset int) Line {
	if byteOffset < 0 {
		assert.Preconditionf(false, "negative byte offset: %d", byteOffset)
	}
	if byteOffset > len(t.text) {
		assert.Preconditionf(false, "byte offset %d past source length %d", byteOffset, len(t.text))
	}
	if byteOffset == len(t.text) {
		last := len(t.lineOffsets) - 1
		return t.line(last, t.lineOffsets[last])
	}
	nextLine := sort.Search(len(t.lineOffsets), func(i int) bool {
		return byteOffset < t.lineOffsets[i].Start()
	})
	// byteOffset ∈ [0, t.TextLen()) here, and t.lineOffsets[0].Start() == 0,
	// so for i == 0, this check above will always be false.
	//
	// sort.Search returns the first index where the predicate is true,
	// hence 1 <= nextLine <= len(t.lineOffsets).
	if nextLine == 0 {
		assert.Invariantf(false, "line table search failed for byte offset %d", byteOffset)
	}
	lineIndex := nextLine - 1
	return t.line(lineIndex, t.lineOffsets[lineIndex])
}

func (t *LineTable) line(i int, lineOffset ranges.Span[int]) Line {
	return Line{text: t.text[lineOffset.Start():lineOffset.End()], span: lineOffset, lineNo: t.startLineNumber + i}
}

// Line represents a line of source code.
type Line struct {
	text   string
	span   ranges.Span[int]
	lineNo int
}

// Text returns the text on the given line, without the line ending.
//
// The returned value may be empty.
func (l *Line) Text() string {
	return l.text
}

// Span returns the half-inclusive byte index range for the
// text of this Line in the LineTable it was created from.
//
// The byte indexes for any line endings are excluded.
func (l *Line) Span() ranges.Span[int] {
	return l.span
}

// LineNumber returns the 1-based line number.
func (l *Line) LineNumber() int {
	return l.lineNo
}
