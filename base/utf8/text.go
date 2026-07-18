// Copyright 2026 Varun Gandhi
//
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package utf8

import (
	stdutf8 "unicode/utf8"

	"code.kibou.tools/base/assert"
	. "code.kibou.tools/base/core/option"
	"code.kibou.tools/base/internal/unicode_impl"
	"code.kibou.tools/base/ranges"
)

// Text is a string known to contain valid UTF-8.
type Text struct {
	text string
}

// Len returns the byte length of t.
func (t Text) Len() int {
	return len(t.text)
}

// GetByte gets the i-th byte for this Text value.
//
// Precondition: 0 <= i < t.Len().
func (t Text) GetByte(i int) byte {
	return t.text[i]
}

// ParseText validates s as UTF-8 text.
func ParseText(s string) (Text, *TextParseError) {
	if span, ok := firstInvalidSpan(s).Get(); ok {
		firstInvalidBytes := unicode_impl.InvalidBytesPrefix(s, span)
		length := span.Length().Unwrap()
		return Text{}, NewTextParseError(len(s), span, firstInvalidBytes[:length])
	}
	return Text{text: s}, nil
}

// MustParseText validates s as UTF-8 text and panics if validation fails.
func MustParseText(s string) Text {
	text, err := ParseText(s)
	if err != nil {
		assert.Preconditionf(false, "%v", err)
	}
	return text
}

// String returns the underlying valid UTF-8 string.
func (t Text) String() string {
	return t.text
}

// CodePointContaining returns the byte span of the UTF-8 codepoint containing
// offset, if offset lies strictly inside a codepoint.
//
// If offset already lies on a codepoint boundary, the result is None.
//
// Precondition: offset ∈ [0, t.Len()].
func (t Text) CodePointContaining(offset int) Option[ranges.Span[int]] {
	text := t.text
	if offset < 0 || len(text) < offset {
		assert.Preconditionf(false, "offset %d outside text bounds [0, %d]", offset, len(text))
	}
	if offset == 0 || offset == len(text) {
		return None[ranges.Span[int]]()
	}

	start := offset - 1
	for start >= 0 && !IsPotentialStartOfRune(text[start]) {
		start--
	}
	if start < 0 {
		return None[ranges.Span[int]]()
	}

	decoded := TryDecodeFirstRune(text[start:])
	end := start + decoded.ByteLen()
	if start < offset && offset < end {
		return Some(ranges.NewSpan(start, end))
	}
	return None[ranges.Span[int]]()
}

// TextParseError reports invalid UTF-8 found while parsing a Text.
type TextParseError struct {
	// firstInvalidSpan is the first non-empty byte span at which UTF-8
	// parsing fails, after a valid UTF-8 prefix of the input.
	//
	// Length() ∈ [1, 4].
	//
	// The bytes in this span are one of:
	//
	// - A malformed UTF-8 sequence: the bytes are not a valid UTF-8 encoding,
	//   and cannot be extended to one;
	// - A truncated UTF-8 sequence: the bytes are a proper prefix of a valid
	//   UTF-8 encoding, but the input ended.
	firstInvalidSpan ranges.Span[int]
	// firstInvalidBytes stores the first firstInvalidSpan.Length() bytes from
	// firstInvalidSpan. Remaining entries are zero and ignored.
	firstInvalidBytes [4]byte
}

// NewTextParseError constructs a TextParseError from the first UTF-8 parse
// failure in an input of length inputLen.
//
// Preconditions:
//   - firstInvalidSpan is within [0, inputLen]
//   - len(invalidUTF8) ∈ [1, 4]
//   - len(invalidUTF8) == firstInvalidSpan.Length()
//   - invalidUTF8 is either malformed UTF-8, or is truncated UTF-8 ending
//     at inputLen
func NewTextParseError(inputLen int, firstInvalidSpan ranges.Span[int], invalidUTF8 []byte) *TextParseError {
	start := firstInvalidSpan.Start()
	if start < 0 {
		assert.Preconditionf(false, "invalid UTF-8 span start %d before 0", start)
	}
	end := firstInvalidSpan.End()
	if inputLen < end {
		assert.Preconditionf(false, "invalid UTF-8 span end %d after input length %d", end, inputLen)
	}
	// start >= 0 => Length() cannot overflow
	length := firstInvalidSpan.Length().Unwrap()
	if len(invalidUTF8) != length {
		assert.Preconditionf(false, "invalid byte prefix length %d; expected span length %d", len(invalidUTF8), length)
	}
	if len(invalidUTF8) < 1 || 4 < len(invalidUTF8) {
		assert.Preconditionf(false, "invalid byte prefix length %d outside [1, 4]", len(invalidUTF8))
	}
	if !stdutf8.FullRune(invalidUTF8) && end != inputLen {
		assert.Preconditionf(false, "truncated UTF-8 span end %d before input length %d", end, inputLen)
	}
	var storedBytes [4]byte
	copy(storedBytes[:], invalidUTF8)
	return &TextParseError{firstInvalidSpan: firstInvalidSpan, firstInvalidBytes: storedBytes}
}

// FirstInvalidSpan returns the first span of consecutive invalid UTF-8 bytes.
func (e *TextParseError) FirstInvalidSpan() ranges.Span[int] {
	return e.firstInvalidSpan
}

func (e *TextParseError) Error() string {
	return unicode_impl.FormatTextParseError("UTF-8", e.firstInvalidSpan, e.firstInvalidBytes)
}
