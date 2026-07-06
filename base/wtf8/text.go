// Copyright 2026 Varun Gandhi
//
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

// Package wtf8 provides WTF-8 helpers used by Kibou code.
//
// WTF-8 is a strict superset of UTF-8 that can additionally encode unpaired
// surrogate code points using the ordinary UTF-8 3-byte form. This allows
// lossless representation of potentially ill-formed UTF-16, as produced by
// Windows filesystem APIs. Every valid UTF-8 string is valid WTF-8, unchanged.
//
// See [the WTF-8 encoding](https://simonsapin.github.io/wtf-8/).
package wtf8

import (
	"code.kibou.tools/base/assert"
	"code.kibou.tools/base/ranges"
	"code.kibou.tools/base/utf8"
)

// Text is a string known to contain valid WTF-8.
type Text struct {
	text string
}

// FromUTF8 converts valid UTF-8 text to WTF-8.
//
// This is total and allocation-free: UTF-8 is a subset of WTF-8, so no
// validation or re-encoding is required.
func FromUTF8(t utf8.Text) Text {
	return Text{text: t.String()}
}

// ParseText validates s as WTF-8 text.
func ParseText(s string) (Text, *TextParseError) {
	// TODO(wtf8): implement WTF-8 validation. Unlike UTF-8, WTF-8 accepts the
	// 3-byte encoding of an unpaired surrogate (0xED, 0xA0-0xBF, 0x80-0xBF),
	// but still rejects overlong encodings and a surrogate *pair* encoded as
	// two separate 3-byte sequences (which must use the 4-byte form).
	assert.Preconditionf(false, "TODO(wtf8): ParseText not implemented")
	return Text{}, nil
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

// String returns the underlying valid WTF-8 string.
func (t Text) String() string {
	return t.text
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
	// TODO(wtf8): mirror utf8.TextParseError.Error once validation lands.
	assert.Preconditionf(false, "TODO(wtf8): TextParseError.Error not implemented")
	return ""
}
