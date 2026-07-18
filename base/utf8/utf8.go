// Copyright 2026 Varun Gandhi
//
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

// Package utf8 provides UTF-8 helpers used by Kibou code.
package utf8

import (
	stdutf8 "unicode/utf8"

	"code.kibou.tools/base/assert"
	. "code.kibou.tools/base/core/option"
	"code.kibou.tools/base/internal/unicode_impl"
	"code.kibou.tools/base/ranges"
)

// ReplacementChar is the Unicode replacement character.
const ReplacementChar = stdutf8.RuneError

// TryDecodeFirstRune tries to decode the first rune in s.
//
// The result kind is:
//   - RuneDecodingResultKind_Valid if s starts with a valid UTF-8 encoding.
//   - RuneDecodingResultKind_Invalid if s starts with invalid UTF-8.
//   - RuneDecodingResultKind_Empty if s is empty.
func TryDecodeFirstRune(s string) RuneDecodingResult {
	r, size := stdutf8.DecodeRuneInString(s)
	return RuneDecodingResult{r: r, byteLen: uint8(size)}
}

// RuneDecodingResult is the result of trying to decode a rune.
//
// This is logically a union of the form:
//
//	  Valid { byteLen int, r rune } (byteLen ∈ [1, 4])
//	| Invalid { byteLen int = 1 }
//	| Empty { byteLen int = 0 }
type RuneDecodingResult struct {
	r       rune
	byteLen uint8
}

type RuneDecodingResultKind uint8

const (
	RuneDecodingResultKind_Valid RuneDecodingResultKind = iota + 1
	RuneDecodingResultKind_Invalid
	RuneDecodingResultKind_Empty
)

// Rune returns the decoded rune.
//
// Precondition: Kind() == RuneDecodingResultKind_Valid.
func (r RuneDecodingResult) Rune() rune {
	if r.Kind() != RuneDecodingResultKind_Valid {
		assert.Precondition(false, "RuneDecodingResult.Rune requires a valid decoding result")
	}
	return r.r
}

// ByteLen returns the number of bytes read while decoding.
func (r RuneDecodingResult) ByteLen() int {
	return int(r.byteLen)
}

// Kind returns the decoding outcome.
func (r RuneDecodingResult) Kind() RuneDecodingResultKind {
	if r.byteLen == 0 {
		return RuneDecodingResultKind_Empty
	}
	if r.r == stdutf8.RuneError && r.byteLen == 1 {
		return RuneDecodingResultKind_Invalid
	}
	return RuneDecodingResultKind_Valid
}

// RuneLen returns the number of bytes required to encode r in UTF-8.
func RuneLen(r rune) int {
	return stdutf8.RuneLen(r)
}

// IsPotentialStartOfRune reports whether b could be the first byte of an encoded rune.
func IsPotentialStartOfRune(b byte) bool {
	return stdutf8.RuneStart(b)
}

// firstInvalidSpan returns the first range of bytes in the
// input text such that the range corresponds to invalid UTF-8.
//
// The Length() of the returned span will be within [1, 4].
func firstInvalidSpan(text string) Option[ranges.Span[int]] {
	for i := 0; i < len(text); {
		// 0 <= i < len(text) => len(text[i:]) is non-empty.
		// So below, we don't need to handle the size == 0 case.
		r, size := stdutf8.DecodeRuneInString(text[i:])
		if r == stdutf8.RuneError && size == 1 {
			return Some(invalidUTF8SpanAt(text, i))
		}
		i += size
	}
	return None[ranges.Span[int]]()
}

// Precondition: TryDecodeFirstRune(text[start:]).Kind() == RuneDecodingResultKind_Invalid.
func invalidUTF8SpanAt(text string, start int) ranges.Span[int] {
	b0 := text[start]
	width, secondMin, secondMax, ok := utf8LeadByteInfo(b0)
	if !ok {
		return ranges.NewSpan(start, start+1)
	}

	if len(text) <= start+1 {
		return ranges.NewSpan(start, len(text))
	}
	b1 := text[start+1]
	if b1 < secondMin || secondMax < b1 {
		return ranges.NewSpan(start, start+2)
	}
	if width == 2 {
		assert.Invariant(false, "DecodeRuneInString reported invalid UTF-8 for a complete two-byte encoding")
	}

	if len(text) <= start+2 {
		return ranges.NewSpan(start, len(text))
	}
	if !unicode_impl.IsContinuationByte(text[start+2]) {
		return ranges.NewSpan(start, start+3)
	}
	if width == 3 {
		assert.Invariant(false, "DecodeRuneInString reported invalid UTF-8 for a complete three-byte encoding")
	}

	if len(text) <= start+3 {
		return ranges.NewSpan(start, len(text))
	}
	if !unicode_impl.IsContinuationByte(text[start+3]) {
		return ranges.NewSpan(start, start+4)
	}
	assert.Invariant(false, "DecodeRuneInString reported invalid UTF-8 for a complete four-byte encoding")
	return ranges.NewSpan(start, start+4)
}

// utf8LeadByteInfo returns the UTF-8 width and valid second-byte range for a
// leading byte, following RFC 3629 §4:
// https://datatracker.ietf.org/doc/html/rfc3629#section-4
//
//	UTF8-octets = *( UTF8-char )
//	UTF8-char   = UTF8-1 / UTF8-2 / UTF8-3 / UTF8-4
//	UTF8-1      = %x00-7F
//	UTF8-2      = %xC2-DF UTF8-tail
//	UTF8-3      = %xE0 %xA0-BF UTF8-tail / %xE1-EC 2( UTF8-tail ) /
//	              %xED %x80-9F UTF8-tail / %xEE-EF 2( UTF8-tail )
//	UTF8-4      = %xF0 %x90-BF 2( UTF8-tail ) / %xF1-F3 3( UTF8-tail ) /
//	              %xF4 %x80-8F 2( UTF8-tail )
//	UTF8-tail   = %x80-BF
//
// P.S. The ranges are inclusive per https://datatracker.ietf.org/doc/html/rfc2234#section-3.4,
// which is cited for syntax from RFC 3629.
func utf8LeadByteInfo(b byte) (width int, secondMin byte, secondMax byte, ok bool) {
	switch {
	case 0xc2 <= b && b <= 0xdf:
		return 2, 0x80, 0xbf, true
	case b == 0xe0:
		return 3, 0xa0, 0xbf, true
	case 0xe1 <= b && b <= 0xec:
		return 3, 0x80, 0xbf, true
	case b == 0xed:
		return 3, 0x80, 0x9f, true
	case 0xee <= b && b <= 0xef:
		return 3, 0x80, 0xbf, true
	case b == 0xf0:
		return 4, 0x90, 0xbf, true
	case 0xf1 <= b && b <= 0xf3:
		return 4, 0x80, 0xbf, true
	case b == 0xf4:
		return 4, 0x80, 0x8f, true
	default:
		return 0, 0, 0, false
	}
}
