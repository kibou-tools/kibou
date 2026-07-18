// Copyright 2026 Varun Gandhi
//
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

// Package wtf8 provides support for WTF-8 encoded strings.
//
// From [the specification](https://wtf-8.codeberg.page/):
//
// > WTF-8 (Wobbly Transformation Format − 8-bit) is a superset of UTF-8
// > that encodes surrogate code points if they are not in a pair.
// > It represents, in a way compatible with UTF-8, text from systems
// > such as JavaScript and Windows that use UTF-16 internally
// > but don’t enforce the well-formedness invariant that surrogates
// > must be paired.
//
// The data types in this package are meant to be used within a program,
// and not across serialization boundaries.
package wtf8

import (
	"fmt"
	"strings"

	"code.kibou.tools/base/assert"
	"code.kibou.tools/base/internal/unicode_impl"
	"code.kibou.tools/base/ranges"
	"code.kibou.tools/base/utf8"
)

// Text is a string known to contain valid WTF-8.
//
// WTF-8 has the following representation:
//
//   - Any valid UTF-8 is also valid WTF-8.
//   - Since UTF-8 does not use surrogate code points,
//     WTF-8 provides a 3-byte form for surrogates,
//     to allow losslessly representing unpaired surrogates.
//
// The presence of a high surrogate followed by a low surrogate
// in encoded form (i.e. 6 consecutive bytes) is forbidden.
// Since the pairing already corresponds to a dedicated
// code point in UTF-8, represented using 4 bytes, the
// 4 byte representation is used.
type Text struct {
	text string
}

var _ fmt.Formatter = Text{text: ""}

// FromUTF8 converts valid UTF-8 text to WTF-8.
//
// This is total and allocation-free: UTF-8 is a subset of WTF-8, so no
// validation or re-encoding is required.
func FromUTF8(t utf8.Text) Text {
	return Text{text: t.String()}
}

// ParseText parses the input as WTF-8 encoded text.
//
// CAUTION: In most cases, you shouldn't need to use this function
// directly. It is primarily meant for internal use in the base
// libraries, to convert paths obtained from the Go standard
// library (which uses WTF-8 on Windows) to a specific
// type which correctly encodes the relevant guarantees.
func ParseText(s string) (Text, *TextParseError) {
	previousWasHighSurrogate := false
	for i := 0; i < len(s); {
		r, size, ok := decodeFirstWTF8Rune(s[i:])
		if !ok {
			// Include up to three immediately following continuation bytes.
			end := i + 1
			for end < len(s) && end < i+4 && unicode_impl.IsContinuationByte(s[end]) {
				end++
			}
			return Text{}, newTextParseErrorAt(s, i, end)
		}
		if size == 3 { // unlikely branch; separate out
			if previousWasHighSurrogate && isLowSurrogate(r) {
				return Text{}, newTextParseErrorAt(s, i, i+size)
			}
			previousWasHighSurrogate = isHighSurrogate(r)
		} else {
			previousWasHighSurrogate = false
		}
		i += size
	}
	return Text{text: s}, nil
}

// MustParseText validates s as WTF-8 text and panics if validation fails.
func MustParseText(s string) Text {
	text, err := ParseText(s)
	if err != nil {
		assert.Preconditionf(false, "%v", err)
	}
	return text
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

// Slice returns a subslice of t by byte offsets.
//
// Precondition: start and end are WTF-8 code point boundaries and
// 0 <= start <= end <= t.Len().
func (t Text) Slice(start, end int) Text {
	if start < 0 || end < start || len(t.text) < end ||
		(start < len(t.text) && unicode_impl.IsContinuationByte(t.text[start])) ||
		(end < len(t.text) && unicode_impl.IsContinuationByte(t.text[end])) {
		assert.Preconditionf(false, "invalid WTF-8 slice [%d, %d) for text of length %d", start, end, len(t.text))
	}
	return Text{text: t.text[start:end]}
}

// String returns the underlying valid WTF-8 string.
//
// NOTE: Generally, this method shouldn't need to be used outside of tests.
func (t Text) String() string {
	return t.text
}

// Format implements fmt.Formatter. For %v, unpaired surrogates are rendered as
// visible \u escapes; other verbs format the underlying WTF-8 bytes directly.
func (t Text) Format(state fmt.State, verb rune) {
	if verb != 'v' || state.Flag('#') {
		_, _ = fmt.Fprintf(state, fmt.FormatString(state, verb), t.text)
		return
	}

	const lowerHexDigits = "0123456789abcdef"
	var result strings.Builder
	escaped := false
	start := 0
	for i := 0; i < len(t.text); {
		r, size, ok := decodeFirstWTF8Rune(t.text[i:])
		assert.Invariant(ok, "wtf8.Text contains invalid WTF-8")
		if isHighSurrogate(r) || isLowSurrogate(r) {
			if !escaped {
				result.Grow(len(t.text))
			}
			escaped = true
			result.WriteString(t.text[start:i])
			result.WriteString(`\u`)
			result.WriteByte(lowerHexDigits[(r>>12)&0xf])
			result.WriteByte(lowerHexDigits[(r>>8)&0xf])
			result.WriteByte(lowerHexDigits[(r>>4)&0xf])
			result.WriteByte(lowerHexDigits[r&0xf])
			start = i + size
		}
		i += size
	}
	if !escaped {
		_, _ = fmt.Fprintf(state, fmt.FormatString(state, verb), t.text)
		return
	}
	result.WriteString(t.text[start:])
	_, _ = fmt.Fprintf(state, fmt.FormatString(state, 's'), result.String())
}

// TextBuilder incrementally assembles a Text. The zero value is ready to use.
type TextBuilder struct {
	buf []byte
}

// NewTextBuilder returns a TextBuilder with space preallocated for capacity
// bytes. Writing more than capacity bytes still works; the buffer grows.
func NewTextBuilder(capacity int) TextBuilder {
	return TextBuilder{buf: make([]byte, 0, capacity)}
}

// WriteText appends t.
//
// This follows the concatenation algorithm in
// https://wtf-8.codeberg.page/#concatenating.
func (b *TextBuilder) WriteText(t Text) {
	left := b.buf
	right := t.text
	if right == "" {
		return
	}
	// From https://wtf-8.codeberg.page/#concatenating:
	//
	// > To concatenate two WTF-8 strings, run these steps:
	// >
	// > 1. If the left input string ends with a [high] surrogate byte sequence and the right input string starts with a [low] surrogate byte sequence, run these substeps:
	// >    - Let [high] and [low] be two code points, the respective results of decoding from WTF-8 these two surrogate byte sequences.
	// >    - Let supplementary be the encoding to WTF-8 of a single code point of value 0x10000 + (([high] - 0xd800) << 10) + ([low] - 0xdc00)
	// >    - Let left be substring of the left input string that removes the three final bytes.
	// >    - Let right be substring of the right input string that removes the three initial bytes.
	// >    - Return the concatenation of left, supplementary, and right.
	// > 2. Otherwise, return the concatenation of the two input byte sequences
	//
	// A high surrogate is always the 3-byte sequence 0xed 0xa0-0xaf 0x80-0xbf,
	// and 0xed can only start a 3-byte sequence, so the final three bytes are the
	// last code point whenever the buffer ends in 0xed.
	if n := len(left); n >= 3 && left[n-3] == 0xed {
		high := decodeThreeByteRune(left[n-3], left[n-2], left[n-1])
		// right is non-empty valid WTF-8 because it comes from a Text.
		low, lowSize, ok := decodeFirstWTF8Rune(right)
		assert.Invariant(ok, "TextBuilder.WriteText input contains invalid WTF-8")
		if isHighSurrogate(high) && isLowSurrogate(low) {
			supplementary := 0x10000 + (high-0xd800)<<10 + (low - 0xdc00)
			left = append(left[:n-3],
				byte(0xf0|supplementary>>18),
				byte(0x80|(supplementary>>12)&0x3f),
				byte(0x80|(supplementary>>6)&0x3f),
				byte(0x80|supplementary&0x3f),
			)
			right = right[lowSize:]
		}
	}
	b.buf = append(left, right...)
}

// WriteASCIIByte appends a single ASCII byte.
//
// Precondition: c < 0x80. An ASCII byte is valid WTF-8 on its own and cannot be
// part of a surrogate, so it needs no boundary handling.
func (b *TextBuilder) WriteASCIIByte(c byte) {
	if 0x80 <= c {
		assert.Preconditionf(false, "wtf8.TextBuilder.WriteASCIIByte: 0x%02x is not ASCII", c)
	}
	b.buf = append(b.buf, c)
}

// Text returns the accumulated Text.
func (b *TextBuilder) Text() Text {
	return Text{text: string(b.buf)}
}

// TextParseError reports invalid WTF-8 found while parsing a Text.
type TextParseError struct {
	// firstInvalidSpan is the first non-empty byte span at which WTF-8 parsing
	// fails, after a valid WTF-8 prefix of the input. Length() ∈ [1, 4].
	firstInvalidSpan ranges.Span[int]
	// firstInvalidBytes stores the first firstInvalidSpan.Length() bytes from
	// firstInvalidSpan. Remaining entries are zero and ignored.
	firstInvalidBytes [4]byte
}

// FirstInvalidSpan returns the first span of consecutive invalid WTF-8 bytes.
func (e *TextParseError) FirstInvalidSpan() ranges.Span[int] {
	return e.firstInvalidSpan
}

func (e *TextParseError) Error() string {
	return unicode_impl.FormatTextParseError("WTF-8", e.firstInvalidSpan, e.firstInvalidBytes)
}

// decodeFirstWTF8Rune decodes one generalized UTF-8 code point.
//
// The return values are:
//   - (r, size, true) if s starts with a well-formed generalized UTF-8
//     sequence; r is the decoded code point, which may be a surrogate, and
//     size is in [1, 4].
//   - (0, 0, false) if s starts with an ill-formed or incomplete sequence.
//
// The accepted byte sequences are defined by the table in
// https://wtf-8.codeberg.page/#generalized-utf-8.
//
// Precondition: s is non-empty.
func decodeFirstWTF8Rune(s string) (rune, int, bool) {
	// See "Table 3. Well-formed byte sequences representing a single code point"
	// in the link above for the implementation.
	b0 := s[0]
	// For a code point in [U+0000, U+007f], b0 ∈ [00, 7f].
	if b0 < 0x80 {
		return rune(b0), 1, true
	}
	// For a code point in [U+0080, U+07ff], b0 ∈ [c2, df] and b1 ∈ [80, bf].
	if 0xc2 <= b0 && b0 <= 0xdf {
		if len(s) < 2 || !unicode_impl.IsContinuationByte(s[1]) {
			return 0, 0, false
		}
		return decodeTwoByteRune(s[0], s[1]), 2, true
	}
	// For a code point in [U+0800, U+0fff], b0 = e0, b1 ∈ [a0, bf], and b2 ∈ [80, bf].
	if b0 == 0xe0 {
		if len(s) < 3 || s[1] < 0xa0 || 0xbf < s[1] || !unicode_impl.IsContinuationByte(s[2]) {
			return 0, 0, false
		}
		return decodeThreeByteRune(s[0], s[1], s[2]), 3, true
	}
	// For a code point in [U+1000, U+ffff], b0 ∈ [e1, ef] and b1, b2 ∈ [80, bf].
	// Unlike UTF-8, generalized UTF-8 allows b1 ∈ [0xa0, 0xbf] when
	// b0 = 0xed. This admits high and low surrogate code points.
	if 0xe1 <= b0 && b0 <= 0xef {
		if len(s) < 3 || !unicode_impl.IsContinuationByte(s[1]) || !unicode_impl.IsContinuationByte(s[2]) {
			return 0, 0, false
		}
		return decodeThreeByteRune(s[0], s[1], s[2]), 3, true
	}
	// For a code point in [U+10000, U+3ffff], b0 = f0, b1 ∈ [90, bf], and b2, b3 ∈ [80, bf].
	if b0 == 0xf0 {
		if len(s) < 4 || s[1] < 0x90 || 0xbf < s[1] || !unicode_impl.IsContinuationByte(s[2]) || !unicode_impl.IsContinuationByte(s[3]) {
			return 0, 0, false
		}
		return decodeFourByteRune(s[0], s[1], s[2], s[3]), 4, true
	}
	// For a code point in [U+40000, U+fffff], b0 ∈ [f1, f3] and b1, b2, b3 ∈ [80, bf].
	if 0xf1 <= b0 && b0 <= 0xf3 {
		if len(s) < 4 || !unicode_impl.IsContinuationByte(s[1]) || !unicode_impl.IsContinuationByte(s[2]) || !unicode_impl.IsContinuationByte(s[3]) {
			return 0, 0, false
		}
		return decodeFourByteRune(s[0], s[1], s[2], s[3]), 4, true
	}
	// For a code point in [U+100000, U+10ffff], b0 = f4, b1 ∈ [80, 8f], and b2, b3 ∈ [80, bf].
	if b0 == 0xf4 {
		if len(s) < 4 || s[1] < 0x80 || 0x8f < s[1] || !unicode_impl.IsContinuationByte(s[2]) || !unicode_impl.IsContinuationByte(s[3]) {
			return 0, 0, false
		}
		return decodeFourByteRune(s[0], s[1], s[2], s[3]), 4, true
	}
	// No other first byte begins a well-formed generalized UTF-8 sequence.
	return 0, 0, false
}

// decodeTwoByteRune decodes a well-formed two-byte generalized UTF-8 sequence.
//
// Precondition: b0 ∈ [0xc2, 0xdf] and b1 ∈ [0x80, 0xbf].
//
// From https://wtf-8.codeberg.page/#decoding-wtf-8:
//
// > [Return] a code point of value
// > ((B & 0x1f) << 6) + (B2 & 0x3f).
func decodeTwoByteRune(b0, b1 byte) rune {
	return rune(b0&0x1f)<<6 + rune(b1&0x3f)
}

// decodeThreeByteRune decodes a well-formed three-byte generalized UTF-8 sequence.
//
// Precondition: b0 ∈ [0xe0, 0xef], b1 and b2 are in [0x80, 0xbf], and
// b1 ∈ [0xa0, 0xbf] when b0 = 0xe0.
//
// From https://wtf-8.codeberg.page/#decoding-wtf-8:
//
// > [Return] a code point of value
// > ((B & 0x0f) << 12) + ((B2 & 0x3f) << 6) + (B3 & 0x3f).
func decodeThreeByteRune(b0, b1, b2 byte) rune {
	return rune(b0&0x0f)<<12 + rune(b1&0x3f)<<6 + rune(b2&0x3f)
}

// decodeFourByteRune decodes a well-formed four-byte generalized UTF-8 sequence.
//
// Precondition: b0 ∈ [0xf0, 0xf4] and b1, b2, and b3 are in [0x80, 0xbf].
// Additionally, b1 ∈ [0x90, 0xbf] when b0 = 0xf0, and b1 ∈ [0x80, 0x8f]
// when b0 = 0xf4.
//
// From https://wtf-8.codeberg.page/#decoding-wtf-8:
//
// > [Return] a code point of value ((B & 0x07) << 18) +
// > ((B2 & 0x3f) << 12) + ((B3 & 0x3f) << 6) + (B4 & 0x3f).
func decodeFourByteRune(b0, b1, b2, b3 byte) rune {
	return rune(b0&0x07)<<18 + rune(b1&0x3f)<<12 + rune(b2&0x3f)<<6 + rune(b3&0x3f)
}

// From https://wtf-8.codeberg.page/#surrogates-code-points
//
// > A lead surrogate code point or high surrogate code point
// > is a code point in the range from U+D800 to U+DBFF.
func isHighSurrogate(r rune) bool {
	return 0xd800 <= r && r <= 0xdbff
}

// From https://wtf-8.codeberg.page/#surrogates-code-points
//
// > A trail surrogate code point or low surrogate code point
// > is a code point in the range from U+DC00 to U+DFFF.
func isLowSurrogate(r rune) bool {
	return 0xdc00 <= r && r <= 0xdfff
}

// Precondition: 0 <= start < end <= len(s).
func newTextParseErrorAt(s string, start, end int) *TextParseError {
	span := ranges.NewSpan(start, end)
	return &TextParseError{
		firstInvalidSpan:  span,
		firstInvalidBytes: unicode_impl.InvalidBytesPrefix(s, span),
	}
}
